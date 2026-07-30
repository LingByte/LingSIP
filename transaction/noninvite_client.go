package transaction

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LingByte/lingsip/stack"
)

// nonInviteClientTx implements RFC 3261 Non-INVITE Client Transaction (UAC role).
// It handles request transmission, retransmission timers (Timer E),
// response matching, and transaction lifecycle management.
type nonInviteClientTx struct {
	key       string              // Unique transaction identifier
	ctx       context.Context     // Context for transaction cancellation
	send      SendFunc            // Function to send SIP messages over UDP
	remote    *net.UDPAddr        // Remote destination address
	frozen    *stack.Message      // Cloned, unmodifiable copy of the original request
	t1        time.Duration       // Base SIP timer T1 (default 500ms)
	t2        time.Duration       // Maximum retransmission interval (default 4s)
	mu        sync.Mutex          // Guards access to finalSeen
	finalSeen bool                // True if a final response (2xx-699) has been received
	stopOnce  sync.Once           // Ensures stop signal is triggered only once
	stopCh    chan struct{}       // Channel to stop retransmit loop
	finalCh   chan *stack.Message // Delivers final response to the caller
	respSrcMu sync.Mutex          // Guards access to respSrc
	respSrc   *net.UDPAddr        // Source address of the received response
	wg        sync.WaitGroup      // Waits for retransmit goroutine to exit
}

// nonInviteClientKey generates a unique key for Non-INVITE client transaction matching.
// Key composition: branch + Call-ID + CSeq number.
func nonInviteClientKey(branch, callID string, cseqNum int) string {
	return strings.TrimSpace(branch) + "\x00" + strings.TrimSpace(callID) + "\x00" + strconv.Itoa(cseqNum)
}

// stop signals the transaction to stop all operations safely.
// Idempotent and thread-safe.
func (tx *nonInviteClientTx) stop() {
	if tx == nil {
		return
	}
	tx.stopOnce.Do(func() { close(tx.stopCh) })
}

// sendFrozen transmits the stored original request.
// Used for initial transmission and retransmissions.
func (tx *nonInviteClientTx) sendFrozen() error {
	if tx.frozen == nil {
		return ErrNilFrozenRequest
	}
	return tx.send(tx.frozen, tx.remote)
}

// retransmitLoop runs the RFC 3261 Timer E retransmission logic.
// Retransmits the request with exponential backoff until stopped or T2 is reached.
func (tx *nonInviteClientTx) retransmitLoop() {
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
		case <-tx.stopCh:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}

		// Perform request retransmission
		_ = tx.sendFrozen()

		// Exponential backoff, capped at T2
		if next < tx.t2 {
			next *= 2
			if next > tx.t2 {
				next = tx.t2
			}
		}
	}
}

// noteRespSrc saves the source address of the incoming response.
func (tx *nonInviteClientTx) noteRespSrc(src *net.UDPAddr) {
	if tx == nil || src == nil {
		return
	}
	tx.respSrcMu.Lock()
	tx.respSrc = src
	tx.respSrcMu.Unlock()
}

// handleResponse processes incoming responses for this transaction.
// Provisional responses (1xx) are ignored.
// Final responses (2xx-699) stop the transaction and return the result.
func (tx *nonInviteClientTx) handleResponse(resp *stack.Message, src *net.UDPAddr) bool {
	if tx == nil || resp == nil {
		return false
	}

	tx.noteRespSrc(src)
	st := resp.StatusCode

	// Ignore provisional responses
	if st >= 100 && st < 200 {
		return true
	}

	// Process final response
	if st >= 200 && st <= 699 {
		tx.mu.Lock()
		if tx.finalSeen {
			tx.mu.Unlock()
			tx.stop()
			return true
		}
		tx.finalSeen = true
		tx.mu.Unlock()

		tx.stop()

		// Send response to caller (non-blocking)
		select {
		case tx.finalCh <- resp:
		default:
		}
		return true
	}

	return false
}

// NonInviteClientResult contains the result of a completed Non-INVITE client transaction.
type NonInviteClientResult struct {
	Final  *stack.Message // Final SIP response (2xx-699)
	Remote *net.UDPAddr   // Source address of the response
}

// RunNonInviteClient executes a full RFC 3261 Non-INVITE client transaction over UDP
// (Timer E retransmissions until a final response or timeout).
func (m *Manager) RunNonInviteClient(
	ctx context.Context,
	req *stack.Message,
	remote *net.UDPAddr,
	send SendFunc,
) (*NonInviteClientResult, error) {
	return m.runNonInviteClient(ctx, req, remote, send, true)
}

// RunNonInviteClientReliable runs a non-INVITE client transaction without UDP
// retransmissions (TCP/TLS). Still registers the tx and waits for a final via HandleResponse.
func (m *Manager) RunNonInviteClientReliable(
	ctx context.Context,
	req *stack.Message,
	remote *net.UDPAddr,
	send SendFunc,
) (*NonInviteClientResult, error) {
	return m.runNonInviteClient(ctx, req, remote, send, false)
}

func (m *Manager) runNonInviteClient(
	ctx context.Context,
	req *stack.Message,
	remote *net.UDPAddr,
	send SendFunc,
	retransmit bool,
) (*NonInviteClientResult, error) {
	if m == nil {
		return nil, ErrNilManager
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil || send == nil {
		return nil, ErrNilRequestOrSend
	}

	// Validate mandatory transaction identifiers
	br := stack.TopBranch(req)
	if br == "" {
		return nil, ErrRequestMissingViaBranch
	}
	callID := strings.TrimSpace(req.GetHeader(stack.HeaderCallID))
	if callID == "" {
		return nil, ErrRequestMissingCallID
	}
	n := stack.ParseCSeqNum(req.GetHeader(stack.HeaderCSeq))
	if n <= 0 {
		return nil, ErrInvalidCSeq
	}

	// Create a deep copy of the request to prevent external modification
	frozen, err := stack.Parse(req.String())
	if err != nil {
		return nil, err
	}

	// Create transaction instance
	key := nonInviteClientKey(br, callID, n)
	tx := &nonInviteClientTx{
		key:     key,
		ctx:     ctx,
		send:    send,
		remote:  remote,
		frozen:  frozen,
		t1:      m.t1Duration(),
		t2:      m.t2Duration(),
		stopCh:  make(chan struct{}),
		finalCh: make(chan *stack.Message, 1),
	}

	// Register transaction in the manager
	m.registerNonInviteClientTx(key, tx)

	retransmitStarted := false
	defer func() {
		tx.stop()
		if retransmitStarted {
			tx.wg.Wait()
		}
		m.unregisterNonInviteClientTx(key)
	}()

	// Send initial request
	if err := send(frozen, remote); err != nil {
		return nil, err
	}

	// Start retransmission loop in background (UDP only).
	if retransmit {
		retransmitStarted = true
		tx.wg.Add(1)
		go tx.retransmitLoop()
	}

	// Wait for completion: timeout or final response
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			onTransactionTimeout(strings.ToUpper(req.Method))
		}
		return nil, ctx.Err()

	case r := <-tx.finalCh:
		tx.respSrcMu.Lock()
		src := tx.respSrc
		tx.respSrcMu.Unlock()

		if src == nil {
			src = remote
		}
		return &NonInviteClientResult{Final: r, Remote: src}, nil
	}
}

// registerNonInviteClientTx adds a client transaction to the manager map.
func (m *Manager) registerNonInviteClientTx(key string, tx *nonInviteClientTx) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nonInviteClient == nil {
		m.nonInviteClient = make(map[string]*nonInviteClientTx)
	}
	m.nonInviteClient[key] = tx
}

// unregisterNonInviteClientTx removes a transaction from the manager map.
func (m *Manager) unregisterNonInviteClientTx(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nonInviteClient, key)
}

// dispatchNonInviteClientResponse matches an incoming response to a client transaction.
// Returns true if the response was consumed by a transaction.
func (m *Manager) dispatchNonInviteClientResponse(resp *stack.Message, src *net.UDPAddr) bool {
	if m == nil || resp == nil {
		return false
	}
	if stack.IsInviteCSeq(resp) {
		return false
	}

	br := stack.TopBranch(resp)
	callID := strings.TrimSpace(resp.GetHeader(stack.HeaderCallID))
	n := stack.ParseCSeqNum(resp.GetHeader(stack.HeaderCSeq))

	if br == "" || callID == "" || n <= 0 {
		return false
	}

	key := nonInviteClientKey(br, callID, n)

	m.mu.Lock()
	tx := m.nonInviteClient[key]
	m.mu.Unlock()

	if tx == nil {
		return false
	}

	return tx.handleResponse(resp, src)
}
