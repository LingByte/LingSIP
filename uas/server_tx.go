package uas

import (
	"context"
	"net"

	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/transaction"
)

// ChainInviteServerTx wraps an InviteHandler with server transaction logic.
// It intercepts retransmitted INVITEs and resends the cached final response.
// Only calls the inner handler if this is a new, non-retransmitted INVITE.
func ChainInviteServerTx(mgr *transaction.Manager, inner InviteHandler) InviteHandler {
	if inner == nil {
		return nil
	}
	if mgr == nil {
		return inner
	}

	return func(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
		// If this is a retransmission, absorb it and return nothing
		if mgr.HandleInviteRequest(req, addr) {
			return nil, nil
		}
		// Otherwise, pass to the actual application handler
		return inner(req, addr)
	}
}

// AfterResponseSentBeginServerTx returns a callback that creates a UAS server transaction
// AFTER a final response (2xx-6xx) has been sent. This starts the appropriate timers
// (Timer G/I for INVITE, Timer J for non-INVITE) to handle retransmissions.
func AfterResponseSentBeginServerTx(
	mgr *transaction.Manager,
	srvCtx context.Context,
	send transaction.SendFunc,
) func(*stack.Message, *stack.Message, *net.UDPAddr) {
	return func(req, resp *stack.Message, addr *net.UDPAddr) {
		if mgr == nil || send == nil || req == nil || resp == nil {
			return
		}

		// Only process final responses
		st := resp.StatusCode
		if st < 200 || st > 699 {
			return
		}

		ctx := srvCtx
		if ctx == nil {
			ctx = context.Background()
		}

		// Start INVITE server transaction
		if req.Method == stack.MethodInvite && stack.IsInviteCSeq(req) {
			_ = mgr.BeginInviteServer(ctx, req, addr, resp, send)
			return
		}

		// Skip if it's an INVITE with mismatched CSeq
		if req.Method == stack.MethodInvite {
			return
		}

		// Start non-INVITE server transaction (OPTIONS, BYE, REGISTER, etc.)
		_ = mgr.BeginNonInviteServer(ctx, req, addr, resp, send)
	}
}

// AfterResponseSentBeginInviteServer is a convenience alias for INVITE-only UAS.
func AfterResponseSentBeginInviteServer(
	mgr *transaction.Manager,
	srvCtx context.Context,
	send transaction.SendFunc,
) func(*stack.Message, *stack.Message, *net.UDPAddr) {
	return AfterResponseSentBeginServerTx(mgr, srvCtx, send)
}

// ChainNonInviteServerTx wraps non-INVITE handlers (BYE, OPTIONS, REGISTER, etc.)
// to absorb retransmissions before passing new requests to the inner handler.
func ChainNonInviteServerTx(mgr *transaction.Manager, inner SimpleHandler) SimpleHandler {
	if inner == nil {
		return nil
	}
	if mgr == nil {
		return inner
	}

	return func(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
		// Handle retransmissions by resending cached response
		if mgr.HandleNonInviteRequest(req, addr) {
			return nil, nil
		}
		// Process new request
		return inner(req, addr)
	}
}

// WithOnResponseSentAppended chains a new OnResponseSent callback after the existing one.
// Used to add transaction hooks without overwriting existing endpoint behavior.
func WithOnResponseSentAppended(
	cfg stack.EndpointConfig,
	fn func(*stack.Message, *stack.Message, *net.UDPAddr),
) stack.EndpointConfig {
	if fn == nil {
		return cfg
	}

	prev := cfg.OnResponseSent
	cfg.OnResponseSent = func(req *stack.Message, resp *stack.Message, addr *net.UDPAddr) {
		if prev != nil {
			prev(req, resp, addr)
		}
		if fn != nil {
			fn(req, resp, addr)
		}
	}

	return cfg
}

// ChainAckServerTx wraps an ACK handler to notify the transaction layer first.
// This stops Timer G (2xx retransmissions) when a valid ACK is received.
func ChainAckServerTx(mgr *transaction.Manager, inner AckHandler) AckHandler {
	if inner == nil && mgr == nil {
		return nil
	}

	return func(req *stack.Message, addr *net.UDPAddr) error {
		// Stop INVITE 2xx retransmissions
		if mgr != nil {
			_ = mgr.HandleAck(req, addr)
		}
		// Pass to application handler for dialog cleanup
		if inner != nil {
			return inner(req, addr)
		}
		return nil
	}
}
