package uas

import (
	"errors"
	"testing"

	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/transaction"
)

func TestSentinelErrors(t *testing.T) {
	cases := []error{
		ErrNilEndpoint,
		ErrNeedRequest,
		ErrInvalidStatus,
		ErrTransactionSendRequired,
	}
	for _, err := range cases {
		if !errors.Is(err, err) {
			t.Fatalf("not self-equal: %v", err)
		}
		if err.Error() == "" {
			t.Fatal("empty Error() string")
		}
	}
}

func TestNewResponseErrors(t *testing.T) {
	if _, err := NewResponse(nil, stack.StatusOK, stack.ReasonOK, "", ""); !errors.Is(err, ErrNeedRequest) {
		t.Fatalf("nil req: %v", err)
	}
	resp, _ := stack.Parse("SIP/2.0 100 Trying\r\n\r\n")
	if _, err := NewResponse(resp, stack.StatusOK, stack.ReasonOK, "", ""); !errors.Is(err, ErrNeedRequest) {
		t.Fatalf("not request: %v", err)
	}
	if _, err := NewResponse(reqOPTIONS(), 99, "x", "", ""); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("low status: %v", err)
	}
}

func TestHandlersAttachErrors(t *testing.T) {
	var h Handlers
	if err := h.Attach(nil); !errors.Is(err, ErrNilEndpoint) {
		t.Fatalf("Attach nil ep: %v", err)
	}
}

func TestAttachWithTransactionErrors(t *testing.T) {
	h := Handlers{}
	if err := h.AttachWithTransaction(nil, TransactionBinding{}); !errors.Is(err, ErrNilEndpoint) {
		t.Fatalf("nil ep: %v", err)
	}
	ep := stack.NewEndpoint(stack.EndpointConfig{Host: "127.0.0.1", Port: 0})
	if err := ep.Open(); err != nil {
		t.Fatal(err)
	}
	defer ep.Close()
	if err := h.AttachWithTransaction(ep, TransactionBinding{Mgr: transaction.NewManager()}); !errors.Is(err, ErrTransactionSendRequired) {
		t.Fatalf("missing send: %v", err)
	}
}

func TestErrInvalidStatusWrap(t *testing.T) {
	err := errInvalidStatus(700)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("wrap: %v", err)
	}
}
