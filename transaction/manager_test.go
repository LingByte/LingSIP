package transaction

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LingByte/lingsip/stack"
)

func TestRouteHeadersForDialog(t *testing.T) {
	if RouteHeadersForDialog(nil) != nil {
		t.Fatal("nil resp")
	}
	resp := &stack.Message{IsRequest: false, StatusCode: 200}
	if got := RouteHeadersForDialog(resp); got != nil {
		t.Fatalf("no RR: %v", got)
	}
	resp.AddHeader("Record-Route", "<sip:proxy1;lr>")
	resp.AddHeader("Record-Route", "<sip:proxy2;lr>")
	got := RouteHeadersForDialog(resp)
	if len(got) != 2 || got[0] != "<sip:proxy2;lr>" || got[1] != "<sip:proxy1;lr>" {
		t.Fatalf("reverse order: %v", got)
	}
}

func TestNewManager_SetTimers(t *testing.T) {
	m := NewManager()
	m.SetT1(0)
	m.SetT2(-1)
	if m.t1Duration() != 500*time.Millisecond {
		t.Fatal("t1 default")
	}
	m.SetT1(100 * time.Millisecond)
	m.SetT2(2 * time.Second)
	if m.t1Duration() != 100*time.Millisecond {
		t.Fatal("t1 set")
	}
	if m.t2Duration() != 2*time.Second {
		t.Fatal("t2 set")
	}
}

func TestManager_HandleResponseNil(t *testing.T) {
	var m *Manager
	if m.HandleResponse(nil, nil) {
		t.Fatal("nil manager")
	}
	m = NewManager()
	if m.HandleResponse(nil, nil) {
		t.Fatal("nil resp")
	}
	req := &stack.Message{IsRequest: true, Method: stack.MethodInvite}
	if m.HandleResponse(req, nil) {
		t.Fatal("request not handled")
	}
}

func TestSetTransactionTimeoutHook(t *testing.T) {
	var called string
	SetTransactionTimeoutHook(func(method string) { called = method })
	onTransactionTimeout("BYE")
	if called != "BYE" {
		t.Fatalf("hook: %q", called)
	}
	SetTransactionTimeoutHook(nil)
	onTransactionTimeout("CANCEL")
}

func TestNonInviteClientKey(t *testing.T) {
	k := nonInviteClientKey("br", "cid", 5)
	if !strings.Contains(k, "br") || !strings.Contains(k, "cid") {
		t.Fatalf("key: %q", k)
	}
}

func TestRunNonInviteClient_Validation(t *testing.T) {
	var m *Manager
	_, err := m.RunNonInviteClient(context.Background(), nil, nil, nil)
	if !errors.Is(err, ErrNilManager) {
		t.Fatalf("expected ErrNilManager, got %v", err)
	}
}
