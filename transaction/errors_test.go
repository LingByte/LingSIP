package transaction

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/LingByte/lingsip/stack"
)

func TestSentinelErrors(t *testing.T) {
	cases := []error{
		ErrNilManager,
		ErrNilInviteFinalOrSend,
		ErrNotInviteRequest,
		ErrInviteCSeqNotInvite,
		ErrInviteMissingViaBranch,
		ErrInviteMissingCallID,
		ErrInvalidInviteCSeq,
		ErrInviteServerTxExists,
		ErrNilMessage,
		ErrInvalidCSeqOnInvite,
		ErrInviteMissingVia,
		ErrNilRequestFinalOrSend,
		ErrNotRequest,
		ErrUseBeginInviteServerForInvite,
		ErrMissingViaBranchOrCSeq,
		ErrNonInviteServerTxExists,
		ErrNilFrozenRequest,
		ErrNilRequestOrSend,
		ErrRequestMissingViaBranch,
		ErrRequestMissingCallID,
		ErrInvalidCSeq,
		ErrNeedInvite,
		ErrMissingCallID,
		ErrMissingViaBranch,
		ErrBadInviteCSeq,
		ErrNilFrozenInvite,
		ErrNilInviteOrSend,
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

func TestWrappedTransactionErrors(t *testing.T) {
	if !errors.Is(errNonInviteServerTxExists("k"), ErrNonInviteServerTxExists) {
		t.Fatal("non-invite server tx exists wrap")
	}
	if err := errFinalStatusNotFinal(180); err == nil || !strings.Contains(err.Error(), "180") {
		t.Fatalf("final status not final: %v", err)
	}
	if err := errFinalNotFinalResponse(100); err == nil || !strings.Contains(err.Error(), "100") {
		t.Fatalf("final not final response: %v", err)
	}
}

func TestRunInviteClientValidationErrors(t *testing.T) {
	var m *Manager
	if _, err := m.RunInviteClient(context.Background(), nil, nil, nil, nil); !errors.Is(err, ErrNilManager) {
		t.Fatalf("nil manager: %v", err)
	}
	m = NewManager()
	if _, err := m.RunInviteClient(context.Background(), nil, nil, nil, nil); !errors.Is(err, ErrNilInviteOrSend) {
		t.Fatalf("nil invite: %v", err)
	}
	inv := &stack.Message{
		IsRequest: true, Method: stack.MethodInvite, RequestURI: "sip:a@b",
		Headers: map[string]string{"Call-ID": "c", "CSeq": "1 INVITE"},
	}
	if _, err := m.RunInviteClient(context.Background(), inv, nil, nil, nil); !errors.Is(err, ErrNilInviteOrSend) {
		t.Fatalf("nil send: %v", err)
	}
	if _, err := m.RunInviteClient(context.Background(), inv, nil, func(*stack.Message, *net.UDPAddr) error { return nil }, nil); !errors.Is(err, ErrInviteMissingViaBranch) {
		t.Fatalf("missing branch: %v", err)
	}
}

func TestRunNonInviteClientValidationErrors(t *testing.T) {
	var m *Manager
	if _, err := m.RunNonInviteClient(context.Background(), nil, nil, nil); !errors.Is(err, ErrNilManager) {
		t.Fatalf("nil manager: %v", err)
	}
	m = NewManager()
	if _, err := m.RunNonInviteClient(context.Background(), nil, nil, nil); !errors.Is(err, ErrNilRequestOrSend) {
		t.Fatalf("nil request: %v", err)
	}
	req := &stack.Message{
		IsRequest: true, Method: stack.MethodBye, RequestURI: "sip:a@b",
		Headers: map[string]string{"Call-ID": "c", "CSeq": "2 BYE"},
	}
	if _, err := m.RunNonInviteClient(context.Background(), req, nil, nil); !errors.Is(err, ErrNilRequestOrSend) {
		t.Fatalf("nil send: %v", err)
	}
	if _, err := m.RunNonInviteClient(context.Background(), req, nil, func(*stack.Message, *net.UDPAddr) error { return nil }); !errors.Is(err, ErrRequestMissingViaBranch) {
		t.Fatalf("missing branch: %v", err)
	}
}

func TestBeginInviteServerValidationErrors(t *testing.T) {
	var m *Manager
	if err := m.BeginInviteServer(context.Background(), nil, nil, nil, nil); !errors.Is(err, ErrNilManager) {
		t.Fatalf("nil manager: %v", err)
	}
	m = NewManager()
	if err := m.BeginInviteServer(context.Background(), nil, nil, nil, nil); !errors.Is(err, ErrNilInviteFinalOrSend) {
		t.Fatalf("nil args: %v", err)
	}
	bye := &stack.Message{IsRequest: true, Method: stack.MethodBye}
	final := &stack.Message{IsRequest: false, StatusCode: 200, StatusText: "OK"}
	if err := m.BeginInviteServer(context.Background(), bye, nil, final, func(*stack.Message, *net.UDPAddr) error { return nil }); !errors.Is(err, ErrNotInviteRequest) {
		t.Fatalf("not invite: %v", err)
	}
}

func TestBeginNonInviteServerValidationErrors(t *testing.T) {
	m := NewManager()
	inv := &stack.Message{IsRequest: true, Method: stack.MethodInvite}
	final := &stack.Message{IsRequest: false, StatusCode: 200, StatusText: "OK"}
	if err := m.BeginNonInviteServer(context.Background(), inv, nil, final, func(*stack.Message, *net.UDPAddr) error { return nil }); !errors.Is(err, ErrUseBeginInviteServerForInvite) {
		t.Fatalf("invite on non-invite path: %v", err)
	}
}

func TestRegisterPendingInviteServerErrors(t *testing.T) {
	m := NewManager()
	if err := m.RegisterPendingInviteServer(nil); !errors.Is(err, ErrNeedInvite) {
		t.Fatalf("nil: %v", err)
	}
	if err := m.RegisterPendingInviteServer(&stack.Message{IsRequest: true, Method: stack.MethodBye}); !errors.Is(err, ErrNeedInvite) {
		t.Fatalf("not invite: %v", err)
	}
}

func TestBuildAckForInviteErrors(t *testing.T) {
	if _, err := BuildAckForInvite(nil, nil, ""); !errors.Is(err, ErrNilMessage) {
		t.Fatalf("nil message: %v", err)
	}
	inv := &stack.Message{IsRequest: true, Method: stack.MethodInvite, Headers: map[string]string{}}
	inv.SetHeader("CSeq", "1 BYE")
	final := &stack.Message{IsRequest: false, StatusCode: 200, StatusText: "OK"}
	if _, err := BuildAckForInvite(inv, final, ""); !errors.Is(err, ErrInviteCSeqNotInvite) {
		t.Fatalf("cseq not invite: %v", err)
	}
	inv.SetHeader("CSeq", "1 INVITE")
	prov := &stack.Message{IsRequest: false, StatusCode: 180, StatusText: "Ringing"}
	if _, err := BuildAckForInvite(inv, prov, ""); err == nil || !strings.Contains(err.Error(), "not a final response") {
		t.Fatalf("provisional final: %v", err)
	}
}
