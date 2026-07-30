package transaction

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/LingByte/lingsip/stack"
)

// SendFunc sends a SIP message to the specified UDP address.
type SendFunc func(msg *stack.Message, addr *net.UDPAddr) error

// inviteClientTx implements RFC 3261 INVITE Client Transaction (UAC role).
// It handles INVITE retransmissions, provisional/final response handling,
// and transaction lifecycle management.
type inviteClientTx struct {
	key string // Unique transaction key (branch + Call-ID)

	ctx    context.Context // Context for lifecycle control
	send   SendFunc        // Function to transmit SIP messages
	remote *net.UDPAddr    // Destination address for the INVITE
	frozen *stack.Message  // Immutable copy of the original INVITE

	t1 time.Duration // Base retransmission timer (T1)

	mu           sync.Mutex
	finalSeen    bool          // True if a final response (2xx-699) was received
	provStopOnce sync.Once     // Ensures provisional handling runs once
	stopRetxOnce sync.Once     // Ensures retransmission stop runs once
	stopRetxCh   chan struct{} // Stops the retransmission loop

	finalCh chan *stack.Message // Delivers final response to caller

	onProvisional func(*stack.Message) // Callback for first 1xx response

	respSrcMu sync.Mutex
	respSrc   *net.UDPAddr // Source address of the received response

	wg sync.WaitGroup // Waits for retransmit goroutine to exit
}

// noteRespSrc records the network address from which the response was received.
func (tx *inviteClientTx) noteRespSrc(src *net.UDPAddr) {
	if tx == nil || src == nil {
		return
	}
	tx.respSrcMu.Lock()
	tx.respSrc = src
	tx.respSrcMu.Unlock()
}

// stopRetransmit signals the retransmission loop to stop.
// Safe for concurrent calls.
func (tx *inviteClientTx) stopRetransmit() {
	if tx == nil {
		return
	}
	tx.stopRetxOnce.Do(func() {
		close(tx.stopRetxCh)
	})
}

// sendFrozen transmits the stored, immutable INVITE message.
func (tx *inviteClientTx) sendFrozen() error {
	if tx.frozen == nil {
		return ErrNilFrozenInvite
	}
	return tx.send(tx.frozen, tx.remote)
}

// retransmitLoop runs the INVITE retransmission logic (Timer B).
// Retransmits with exponential backoff up to 32s until stopped.
func (tx *inviteClientTx) retransmitLoop() {
	defer tx.wg.Done()
	next := tx.t1

	for {
		timer := time.NewTimer(next)
		select {
		case <-tx.ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-tx.stopRetxCh:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}

		// Perform retransmission
		_ = tx.sendFrozen()

		// Exponential backoff, capped at 32 seconds
		if next < 32*time.Second {
			next *= 2
		}
	}
}

// handleResponse processes incoming responses for this INVITE transaction.
// - 1xx (provisional): stops retransmissions, triggers callback once
// - 2xx-6xx (final): stops transaction, delivers response to caller
func (tx *inviteClientTx) handleResponse(resp *stack.Message, src *net.UDPAddr) bool {
	if tx == nil || resp == nil {
		return false
	}

	tx.noteRespSrc(src)
	st := resp.StatusCode

	// Handle provisional response (1xx)
	if st >= 100 && st < 200 {
		tx.provStopOnce.Do(func() {
			tx.stopRetransmit()
			if tx.onProvisional != nil {
				tx.onProvisional(resp)
			}
		})
		return true
	}

	// Handle final response (2xx-6xx)
	if st >= 200 && st <= 699 {
		tx.mu.Lock()
		if tx.finalSeen {
			tx.mu.Unlock()
			tx.stopRetransmit()
			return true
		}
		tx.finalSeen = true
		tx.mu.Unlock()

		tx.stopRetransmit()

		// Send final response to caller (non-blocking)
		select {
		case tx.finalCh <- resp:
		default:
		}
		return true
	}

	return false
}

// InviteClientResult contains the result of a completed INVITE client transaction.
type InviteClientResult struct {
	Final  *stack.Message // Final response (2xx-6xx)
	Remote *net.UDPAddr   // Source address for ACK/in-dialog requests
}

// RunInviteClient executes a full RFC 3261 INVITE client transaction over UDP
// (Timer A retransmissions until 1xx or final response).
func (m *Manager) RunInviteClient(
	ctx context.Context,
	invite *stack.Message,
	remote *net.UDPAddr,
	send SendFunc,
	onProvisional func(*stack.Message),
) (*InviteClientResult, error) {
	return m.runInviteClient(ctx, invite, remote, send, onProvisional, true)
}

// RunInviteClientReliable runs an INVITE client transaction without UDP
// retransmissions (TCP/TLS). Still registers the tx and waits for a final via HandleResponse.
func (m *Manager) RunInviteClientReliable(
	ctx context.Context,
	invite *stack.Message,
	remote *net.UDPAddr,
	send SendFunc,
	onProvisional func(*stack.Message),
) (*InviteClientResult, error) {
	return m.runInviteClient(ctx, invite, remote, send, onProvisional, false)
}

func (m *Manager) runInviteClient(
	ctx context.Context,
	invite *stack.Message,
	remote *net.UDPAddr,
	send SendFunc,
	onProvisional func(*stack.Message),
	retransmit bool,
) (*InviteClientResult, error) {
	if m == nil {
		return nil, ErrNilManager
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if invite == nil || send == nil {
		return nil, ErrNilInviteOrSend
	}

	br := stack.TopBranch(invite)
	if br == "" {
		return nil, ErrInviteMissingViaBranch
	}
	callID := strings.TrimSpace(invite.GetHeader("Call-ID"))
	if callID == "" {
		return nil, ErrInviteMissingCallID
	}

	frozen, err := stack.Parse(invite.String())
	if err != nil {
		return nil, err
	}

	key := stack.InviteClientKey(br, callID)
	tx := &inviteClientTx{
		key:           key,
		ctx:           ctx,
		send:          send,
		remote:        remote,
		frozen:        frozen,
		t1:            m.t1Duration(),
		stopRetxCh:    make(chan struct{}),
		finalCh:       make(chan *stack.Message, 1),
		onProvisional: onProvisional,
	}

	m.registerInviteTx(key, tx)

	retransmitStarted := false
	defer func() {
		tx.stopRetransmit()
		if retransmitStarted {
			tx.wg.Wait()
		}
		m.unregisterInviteTx(key)
	}()

	if err := send(frozen, remote); err != nil {
		return nil, err
	}

	if retransmit {
		retransmitStarted = true
		tx.wg.Add(1)
		go tx.retransmitLoop()
	}

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			onTransactionTimeout(stack.MethodInvite)
		}
		return nil, ctx.Err()

	case r := <-tx.finalCh:
		tx.respSrcMu.Lock()
		src := tx.respSrc
		tx.respSrcMu.Unlock()

		if src == nil {
			src = remote
		}
		return &InviteClientResult{Final: r, Remote: src}, nil
	}
}
