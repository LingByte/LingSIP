package transaction

import (
	"net"
	"sync"
	"time"

	"github.com/LingByte/lingsip/stack"
)

// Manager manages all SIP transactions (client + server, INVITE + non-INVITE).
// It routes incoming responses to the correct client transaction,
// and tracks server transaction state for retransmissions.
// Acts as the central coordinator for RFC 3261 transaction layer logic.
type Manager struct {
	mu              sync.Mutex
	inviteTx        map[string]*inviteClientTx    // Active INVITE client transactions (UAC)
	inviteServer    map[string]*inviteServerTx    // Active INVITE server transactions (UAS)
	nonInvite       map[string]*nonInviteServerTx // Active non-INVITE server transactions (UAS)
	nonInviteClient map[string]*nonInviteClientTx // Active non-INVITE client transactions (UAC)

	// pendingInviteByCall tracks INVITEs that have not received a final response,
	// used to match CANCEL requests by Call-ID + CSeq.
	pendingInviteByCall map[string]*pendingInvite

	t1 time.Duration // Base retransmission timer (RFC default: 500ms)
	t2 time.Duration // Max retransmission interval (RFC default: 4s)
}

// pendingInvite tracks an active INVITE awaiting a final response,
// used for CANCEL request matching.
type pendingInvite struct {
	branch string // Via branch ID
	cseq   int    // CSeq number
}

// NewManager creates a new transaction Manager with RFC 3261 default timers:
// T1 = 500ms, T2 = 4s.
func NewManager() *Manager {
	return &Manager{
		inviteTx:            make(map[string]*inviteClientTx),
		inviteServer:        make(map[string]*inviteServerTx),
		nonInvite:           make(map[string]*nonInviteServerTx),
		nonInviteClient:     make(map[string]*nonInviteClientTx),
		pendingInviteByCall: make(map[string]*pendingInvite),
		t1:                  500 * time.Millisecond,
		t2:                  4 * time.Second,
	}
}

// SetT1 overrides the T1 timer for testing purposes.
// T1 is the initial round-trip time estimate for INVITE retransmissions.
func (m *Manager) SetT1(d time.Duration) {
	if m == nil || d <= 0 {
		return
	}
	m.mu.Lock()
	m.t1 = d
	m.mu.Unlock()
}

// SetT2 overrides the T2 timer for testing purposes.
// T2 is the maximum interval for INVITE 2xx retransmissions.
func (m *Manager) SetT2(d time.Duration) {
	if m == nil || d <= 0 {
		return
	}
	m.mu.Lock()
	m.t2 = d
	m.mu.Unlock()
}

// t1Duration returns the current T1 duration (thread-safe).
func (m *Manager) t1Duration() time.Duration {
	m.mu.Lock()
	d := m.t1
	m.mu.Unlock()
	if d <= 0 {
		return 500 * time.Millisecond
	}
	return d
}

// t2Duration returns the current T2 duration (thread-safe).
func (m *Manager) t2Duration() time.Duration {
	m.mu.Lock()
	d := m.t2
	m.mu.Unlock()
	if d <= 0 {
		return 4 * time.Second
	}
	return d
}

// registerInviteTx adds an INVITE client transaction to the manager.
func (m *Manager) registerInviteTx(key string, tx *inviteClientTx) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inviteTx == nil {
		m.inviteTx = make(map[string]*inviteClientTx)
	}
	m.inviteTx[key] = tx
}

// unregisterInviteTx removes an INVITE client transaction from the manager.
func (m *Manager) unregisterInviteTx(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inviteTx, key)
}

// HandleResponse routes an incoming SIP response to the corresponding client transaction.
// Returns true if the response was handled by a transaction.
// src is the UDP address from which the response was received (for symmetric routing).
func (m *Manager) HandleResponse(resp *stack.Message, src *net.UDPAddr) bool {
	if m == nil || resp == nil || resp.IsRequest {
		return false
	}

	// Handle INVITE responses
	if stack.IsInviteCSeq(resp) {
		key := stack.InviteClientKey(stack.TopBranch(resp), resp.GetHeader("Call-ID"))
		m.mu.Lock()
		tx := m.inviteTx[key]
		m.mu.Unlock()
		if tx == nil {
			return false
		}
		return tx.handleResponse(resp, src)
	}

	// Handle non-INVITE responses
	return m.dispatchNonInviteClientResponse(resp, src)
}
