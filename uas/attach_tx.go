package uas

import (
	"context"
	"net"

	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/transaction"
)

// TransactionBinding binds a transaction Manager and UDP send function
// to provide reliable server transaction behavior for the UAS.
type TransactionBinding struct {
	Mgr *transaction.Manager // SIP transaction layer manager

	// Send function must use the same UDP socket as the endpoint to ensure
	// correct source IP/port for symmetric response routing.
	Send transaction.SendFunc

	// Ctx controls the lifecycle of background transaction timers.
	// If nil, context.Background() is used automatically.
	Ctx context.Context
}

// WrapHandlersWithTransaction returns a copy of the input handlers with
// transaction-layer middleware applied for INVITE, non-INVITE, CANCEL, and ACK.
// If Manager or Send is nil, the original handlers are returned unmodified.
func WrapHandlersWithTransaction(h Handlers, b TransactionBinding) Handlers {
	if b.Mgr == nil || b.Send == nil {
		return h
	}

	out := h

	// Helper to wrap non-INVITE methods with server transaction handling
	wrapNonInvite := func(inner SimpleHandler) SimpleHandler {
		if inner == nil {
			return nil
		}
		return ChainNonInviteServerTx(b.Mgr, inner)
	}

	// Wrap INVITE handler with transaction retransmission protection
	if out.Invite != nil {
		out.Invite = ChainInviteServerTx(b.Mgr, h.Invite)
	}

	// Wrap and enhance CANCEL handler with transaction logic
	originalCancel := h.Cancel
	out.Cancel = func(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
		// Absorb retransmitted CANCELs
		if b.Mgr.HandleNonInviteRequest(req, addr) {
			return nil, nil
		}

		// Attempt to match and handle the CANCEL in the transaction layer
		matched := b.Mgr.HandleCancelRequest(req, addr, b.Send)
		if !matched {
			// Return 481 Call/Transaction Does Not Exist if no matching INVITE
			return ErrorResponse(req, stack.StatusCallTransactionDoesNotExist, stack.ReasonCallTransactionDoesNotExist)
		}

		// Call original application handler if provided
		if originalCancel != nil {
			return originalCancel(req, addr)
		}

		return nil, nil
	}

	// Wrap ACK handler to stop 2xx retransmissions in the transaction layer
	if h.Ack != nil {
		out.Ack = ChainAckServerTx(b.Mgr, h.Ack)
	} else {
		// Default ACK handler: only notify transaction layer
		out.Ack = func(req *stack.Message, addr *net.UDPAddr) error {
			_ = b.Mgr.HandleAck(req, addr)
			return nil
		}
	}

	// Wrap all standard non-INVITE methods
	if out.Bye != nil {
		out.Bye = wrapNonInvite(h.Bye)
	}
	if h.Options != nil {
		out.Options = wrapNonInvite(h.Options)
	} else {
		out.Options = wrapNonInvite(defaultOptions)
	}
	if out.Register != nil {
		out.Register = wrapNonInvite(h.Register)
	}
	if out.Info != nil {
		out.Info = wrapNonInvite(h.Info)
	}
	if out.Prack != nil {
		out.Prack = wrapNonInvite(h.Prack)
	}
	if out.Subscribe != nil {
		out.Subscribe = wrapNonInvite(h.Subscribe)
	}
	if out.Notify != nil {
		out.Notify = wrapNonInvite(h.Notify)
	}
	if out.Publish != nil {
		out.Publish = wrapNonInvite(h.Publish)
	}
	if out.Refer != nil {
		out.Refer = wrapNonInvite(h.Refer)
	}
	if out.Message != nil {
		out.Message = wrapNonInvite(h.Message)
	}
	if out.Update != nil {
		out.Update = wrapNonInvite(h.Update)
	}

	return out
}

// AttachWithTransaction configures the endpoint with transaction support
// and attaches the wrapped UAS handlers. It adds the necessary post-response
// hook to start server transactions after sending final responses.
func (h Handlers) AttachWithTransaction(ep *stack.Endpoint, b TransactionBinding) error {
	if ep == nil {
		return ErrNilEndpoint
	}
	if b.Mgr != nil && b.Send == nil {
		return ErrTransactionSendRequired
	}

	// Install hook to create server transactions AFTER responses are sent
	if b.Mgr != nil && b.Send != nil {
		ep.AppendOnResponseSent(AfterResponseSentBeginServerTx(b.Mgr, b.Ctx, b.Send))
	}

	// Wrap handlers with transaction middleware and attach to endpoint
	handlersWrapped := WrapHandlersWithTransaction(h, b)
	return handlersWrapped.Attach(ep)
}
