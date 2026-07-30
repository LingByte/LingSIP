package transaction

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/LingByte/lingsip/stack"
)

// inviteServerTx implements RFC 3261 INVITE Server Transaction (UAS role).
// It manages retransmissions of the final response (2xx/6xx) and handles
// Timer G (for 2xx) and Timer I (for 3xx-6xx) logic.
type inviteServerTx struct {
	mgr    *Manager        // Parent transaction manager
	key    string          // Unique transaction key (branch + Call-ID)
	ctx    context.Context // Lifecycle context
	send   SendFunc        // Function to send SIP messages
	remote *net.UDPAddr    // Remote address of the UAC

	mu sync.Mutex
	// finalResp is a deep copy of the last final response (2xx–6xx)
	// used for retransmitting to the UAC.
	finalResp *stack.Message

	inviteCSeq int // CSeq number from the original INVITE

	stopOnce sync.Once      // Ensures stop is triggered only once
	stopCh   chan struct{}  // Signals transaction to stop
	wg       sync.WaitGroup // Waits for timer goroutines to exit
}

// signalStop closes the stop channel to terminate timers and retransmissions.
// Safe for concurrent calls.
func (tx *inviteServerTx) signalStop() {
	if tx == nil {
		return
	}
	tx.stopOnce.Do(func() {
		close(tx.stopCh)
	})
}

// retransmitFinal sends the cached final response to the given address.
// Uses the original remote address if addr is nil.
func (tx *inviteServerTx) retransmitFinal(addr *net.UDPAddr) error {
	if tx == nil {
		return nil
	}

	tx.mu.Lock()
	fr := tx.finalResp
	tx.mu.Unlock()

	if fr == nil {
		return nil
	}

	dst := addr
	if dst == nil {
		dst = tx.remote
	}
	return tx.send(fr, dst)
}

// runTimerG handles retransmissions of 2xx responses (RFC 3261 §17.2.3).
// Retransmits with exponential backoff (capped at T2) until stopped by ACK or context.
func (tx *inviteServerTx) runTimerG() {
	defer func() {
		tx.mgr.unregisterInviteServerTx(tx.key)
		tx.wg.Done()
	}()

	next := tx.mgr.t1Duration()
	t2 := tx.mgr.t2Duration()

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

		// Retransmit the 2xx final response
		_ = tx.retransmitFinal(nil)

		// Exponential backoff up to T2
		if next < t2 {
			next *= 2
			if next > t2 {
				next = t2
			}
		}
	}
}

// runTimerI handles the timeout for non-2xx final responses (3xx-6xx).
// Waits 64*T1 before cleaning up the transaction state (RFC 3261 §17.2.1).
func (tx *inviteServerTx) runTimerI() {
	defer func() {
		tx.mgr.unregisterInviteServerTx(tx.key)
		tx.wg.Done()
	}()

	duration := 64 * tx.mgr.t1Duration()
	timer := time.NewTimer(duration)

	select {
	case <-tx.ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	case <-tx.stopCh:
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
	}
}

// unregisterInviteServerTx removes the INVITE server transaction from the manager.
func (m *Manager) unregisterInviteServerTx(key string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inviteServer == nil {
		return
	}
	delete(m.inviteServer, key)
}

// BeginInviteServer creates and starts an INVITE server transaction after
// sending a final response (2xx–6xx) to the UAC.
// - For 2xx: starts Timer G (retransmits until ACK)
// - For 3xx-6xx: starts Timer I (waits 64*T1 then cleans up)
func (m *Manager) BeginInviteServer(
	ctx context.Context,
	invite *stack.Message,
	remote *net.UDPAddr,
	final *stack.Message,
	send SendFunc,
) error {
	if m == nil {
		return ErrNilManager
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if invite == nil || final == nil || send == nil {
		return ErrNilInviteFinalOrSend
	}
	if !invite.IsRequest || invite.Method != stack.MethodInvite {
		return ErrNotInviteRequest
	}
	if !stack.IsInviteCSeq(invite) {
		return ErrInviteCSeqNotInvite
	}

	// Validate final response status
	st := final.StatusCode
	if st < 200 || st > 699 {
		return errFinalStatusNotFinal(st)
	}

	// Extract mandatory transaction identifiers
	br := stack.TopBranch(invite)
	if br == "" {
		return ErrInviteMissingViaBranch
	}
	callID := strings.TrimSpace(invite.GetHeader(stack.HeaderCallID))
	if callID == "" {
		return ErrInviteMissingCallID
	}

	// Create a deep copy of the final response to avoid external modification
	fr, err := stack.Parse(final.String())
	if err != nil {
		return err
	}

	key := stack.InviteClientKey(br, callID)
	cseqNum := stack.ParseCSeqNum(invite.GetHeader(stack.HeaderCSeq))
	if cseqNum <= 0 {
		return ErrInvalidInviteCSeq
	}

	// Initialize transaction
	tx := &inviteServerTx{
		mgr:        m,
		key:        key,
		ctx:        ctx,
		send:       send,
		remote:     remote,
		finalResp:  fr,
		inviteCSeq: cseqNum,
		stopCh:     make(chan struct{}),
	}

	// Register transaction
	m.mu.Lock()
	if m.inviteServer == nil {
		m.inviteServer = make(map[string]*inviteServerTx)
	}
	if _, exists := m.inviteServer[key]; exists {
		m.mu.Unlock()
		return ErrInviteServerTxExists
	}
	m.inviteServer[key] = tx
	m.mu.Unlock()

	// Clean up pending INVITE tracking for CANCEL
	m.ClearPendingInviteServer(callID)

	// Start appropriate timer based on final response type
	if st >= 200 && st < 300 {
		// 2xx: use Timer G for retransmissions until ACK
		tx.wg.Add(1)
		go tx.runTimerG()
	} else {
		// 3xx-6xx: use Timer I for cleanup
		tx.wg.Add(1)
		go tx.runTimerI()
	}

	return nil
}

// HandleInviteRequest processes retransmitted INVITEs.
// If a transaction exists, resends the cached final response.
func (m *Manager) HandleInviteRequest(req *stack.Message, addr *net.UDPAddr) bool {
	if m == nil || req == nil || !req.IsRequest || req.Method != stack.MethodInvite {
		return false
	}
	if !stack.IsInviteCSeq(req) {
		return false
	}

	key := stack.InviteClientKey(stack.TopBranch(req), req.GetHeader(stack.HeaderCallID))

	m.mu.Lock()
	tx := m.inviteServer[key]
	m.mu.Unlock()

	if tx == nil {
		return false
	}

	_ = tx.retransmitFinal(addr)
	return true
}

// HandleAck matches an incoming ACK to an INVITE server transaction.
// Stops retransmissions (Timer G) if a matching transaction is found.
func (m *Manager) HandleAck(ack *stack.Message, _ *net.UDPAddr) bool {
	if m == nil || ack == nil || !ack.IsRequest || ack.Method != stack.MethodAck {
		return false
	}
	if !stack.IsAckCSeq(ack) {
		return false
	}

	key := stack.InviteClientKey(stack.TopBranch(ack), ack.GetHeader(stack.HeaderCallID))
	ackCSeq := stack.ParseCSeqNum(ack.GetHeader(stack.HeaderCSeq))
	if ackCSeq <= 0 {
		return false
	}

	m.mu.Lock()
	tx := m.inviteServer[key]
	m.mu.Unlock()

	if tx == nil {
		return false
	}
	if ackCSeq != tx.inviteCSeq {
		return false
	}

	// Stop retransmissions (Timer G)
	tx.signalStop()
	return true
}
