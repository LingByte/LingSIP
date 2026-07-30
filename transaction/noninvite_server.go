package transaction

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/LingByte/lingsip/stack"
)

// nonInviteServerTx implements RFC 3261 Non-INVITE Server Transaction (UAS role).
// It stores a final response and handles retransmissions during the Timer J window.
// Used for requests like OPTIONS, REGISTER, BYE, NOTIFY, etc.
type nonInviteServerTx struct {
	mgr      *Manager        // Parent transaction manager
	key      string          // Unique transaction key (branch + method)
	ctx      context.Context // Context for lifecycle management
	final    *stack.Message  // Cached final response (2xx-6xx) to retransmit
	remote   *net.UDPAddr    // Remote address of the client
	send     SendFunc        // Function to send SIP messages
	stopOnce sync.Once       // Ensures stop signal is sent only once
	stopCh   chan struct{}   // Channel to stop the timer early
	wg       sync.WaitGroup  // Waits for timer goroutine to exit
}

// signalStop closes the stop channel to terminate Timer J early.
// Safe for concurrent calls.
func (tx *nonInviteServerTx) signalStop() {
	tx.stopOnce.Do(func() { close(tx.stopCh) })
}

// retransmit sends the cached final response to the destination address.
// Uses the remote addr from transaction creation if the provided addr is nil.
func (tx *nonInviteServerTx) retransmit(addr *net.UDPAddr) error {
	dst := addr
	if dst == nil {
		dst = tx.remote
	}
	return tx.send(tx.final, dst)
}

// runTimerJ runs the RFC 3261 Timer J (64*T1) for Non-INVITE server transactions.
// After timeout, the transaction is automatically unregistered and cleaned up.
func (tx *nonInviteServerTx) runTimerJ() {
	defer tx.mgr.unregisterNonInviteTx(tx.key)
	defer tx.wg.Done()

	// Timer J: wait 64*T1 before destroying transaction state
	timer := time.NewTimer(64 * tx.mgr.t1Duration())
	select {
	case <-tx.ctx.Done():
		// Canceled by parent context
		if !timer.Stop() {
			<-timer.C
		}
	case <-tx.stopCh:
		// Stopped manually
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
		// Timer J expired, transaction ends
	}
}

// unregisterNonInviteTx removes the transaction from the manager's map.
// Safe for concurrent access.
func (m *Manager) unregisterNonInviteTx(key string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nonInvite == nil {
		return
	}
	delete(m.nonInvite, key)
}

// BeginNonInviteServer creates and starts a Non-INVITE Server Transaction (UAS).
// Must be called AFTER sending a final response (2xx-6xx).
// It stores the final response to handle retransmissions during Timer J window.
func (m *Manager) BeginNonInviteServer(
	ctx context.Context,
	req *stack.Message,
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
	if req == nil || final == nil || send == nil {
		return ErrNilRequestFinalOrSend
	}
	if !req.IsRequest {
		return ErrNotRequest
	}
	if req.Method == stack.MethodInvite {
		return ErrUseBeginInviteServerForInvite
	}

	// Validate final response is in 200-699 range
	st := final.StatusCode
	if st < 200 || st > 699 {
		return errFinalStatusNotFinal(st)
	}

	// Compute unique transaction key
	key := stack.NonInviteServerKey(req)
	if key == "" || stack.ParseCSeqNum(req.GetHeader("CSeq")) <= 0 || stack.TopBranch(req) == "" {
		return ErrMissingViaBranchOrCSeq
	}

	// Clone final response to avoid external modification
	fr, err := stack.Parse(final.String())
	if err != nil {
		return err
	}

	// Create transaction state
	tx := &nonInviteServerTx{
		mgr:    m,
		key:    key,
		ctx:    ctx,
		final:  fr,
		remote: remote,
		send:   send,
		stopCh: make(chan struct{}),
	}

	// Store transaction in manager
	m.mu.Lock()
	if m.nonInvite == nil {
		m.nonInvite = make(map[string]*nonInviteServerTx)
	}
	if _, exists := m.nonInvite[key]; exists {
		m.mu.Unlock()
		return errNonInviteServerTxExists(key)
	}
	m.nonInvite[key] = tx
	m.mu.Unlock()

	// Start Timer J in background
	tx.wg.Add(1)
	go tx.runTimerJ()

	return nil
}

// HandleNonInviteRequest checks if the incoming request is a retransmission.
// If a transaction exists (Timer J running), it resends the final response.
// Returns true if the request was handled as a retransmission.
func (m *Manager) HandleNonInviteRequest(req *stack.Message, addr *net.UDPAddr) bool {
	if m == nil || req == nil || !req.IsRequest {
		return false
	}
	if req.Method == stack.MethodInvite {
		return false
	}

	key := stack.NonInviteServerKey(req)
	if key == "" {
		return false
	}

	// Look up existing transaction
	m.mu.Lock()
	tx := m.nonInvite[key]
	m.mu.Unlock()

	if tx == nil {
		return false
	}

	// Retransmit final response to the sender
	_ = tx.retransmit(addr)
	return true
}
