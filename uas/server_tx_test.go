package uas

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/transaction"
)

func TestChainInviteServerTx(t *testing.T) {
	mgr := transaction.NewManager()
	var innerCalls atomic.Int32
	inner := func(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
		innerCalls.Add(1)
		return uasFinalForTx(t, req, 100, "Trying"), nil
	}
	chained := ChainInviteServerTx(mgr, inner)
	inv := uasInviteForTx(t)
	remote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
	final := uasFinalForTx(t, inv, 486, "Busy")
	_ = mgr.BeginInviteServer(context.Background(), inv, remote, final, func(*stack.Message, *net.UDPAddr) error { return nil })
	if _, err := chained(inv, remote); err != nil {
		t.Fatal(err)
	}
	if innerCalls.Load() != 0 {
		t.Fatalf("inner should not run on retransmit, got %d", innerCalls.Load())
	}
}

func uasInviteForTx(t *testing.T) *stack.Message {
	t.Helper()
	raw := strings.Join([]string{
		"INVITE sip:x@y SIP/2.0",
		"Via: SIP/2.0/UDP 10.0.0.1;branch=z9hG4bKchain",
		"From: <sip:a@b>",
		"To: <sip:x@y>",
		"Call-ID: chain-1",
		"CSeq: 1 INVITE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	m, err := stack.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func uasFinalForTx(t *testing.T, inv *stack.Message, status int, reason string) *stack.Message {
	t.Helper()
	r := &stack.Message{
		IsRequest:    false,
		Version:      "SIP/2.0",
		StatusCode:   status,
		StatusText:   reason,
		Headers:      map[string]string{},
		HeadersMulti: map[string][]string{},
	}
	r.SetHeader("Via", stack.TopVia(inv))
	r.SetHeader("From", inv.GetHeader("From"))
	r.SetHeader("To", inv.GetHeader("To"))
	r.SetHeader("Call-ID", inv.GetHeader("Call-ID"))
	r.SetHeader("CSeq", inv.GetHeader("CSeq"))
	r.SetHeader("Content-Length", "0")
	return r
}

func TestWithOnResponseSentAppended(t *testing.T) {
	var a, b atomic.Int32
	cfg := stack.EndpointConfig{}
	cfg = WithOnResponseSentAppended(cfg, func(*stack.Message, *stack.Message, *net.UDPAddr) { a.Add(1) })
	cfg = WithOnResponseSentAppended(cfg, func(*stack.Message, *stack.Message, *net.UDPAddr) { b.Add(1) })
	cfg.OnResponseSent(nil, nil, nil)
	if a.Load() != 1 || b.Load() != 1 {
		t.Fatalf("a=%d b=%d", a.Load(), b.Load())
	}
}

func TestChainInviteServerTx_NewInviteHitsInner(t *testing.T) {
	mgr := transaction.NewManager()
	var innerCalls atomic.Int32
	inner := func(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
		innerCalls.Add(1)
		return uasFinalForTx(t, req, 100, "Trying"), nil
	}
	chained := ChainInviteServerTx(mgr, inner)
	inv := uasInviteForTx(t)
	if _, err := chained(inv, &net.UDPAddr{}); err != nil {
		t.Fatal(err)
	}
	if innerCalls.Load() != 1 {
		t.Fatalf("inner calls=%d", innerCalls.Load())
	}
}

func TestAfterResponseSentBeginServerTx(t *testing.T) {
	mgr := transaction.NewManager()
	send := func(*stack.Message, *net.UDPAddr) error { return nil }
	fn := AfterResponseSentBeginServerTx(mgr, context.Background(), send)
	fn(nil, nil, nil) // nil-safe
	req := &stack.Message{IsRequest: true, Method: stack.MethodOptions, Version: "SIP/2.0"}
	req.SetHeader("Via", "SIP/2.0/UDP x;branch=b")
	req.SetHeader("Call-ID", "c")
	req.SetHeader("CSeq", "1 OPTIONS")
	resp := &stack.Message{IsRequest: false, StatusCode: 200, Version: "SIP/2.0"}
	fn(req, resp, &net.UDPAddr{IP: net.IPv4zero, Port: 5060})
}

func TestChainAckServerTx(t *testing.T) {
	mgr := transaction.NewManager()
	var ackHandled bool
	h := ChainAckServerTx(mgr, func(*stack.Message, *net.UDPAddr) error {
		ackHandled = true
		return nil
	})
	req := &stack.Message{IsRequest: true, Method: stack.MethodAck, Version: "SIP/2.0"}
	_ = h(req, nil)
	if !ackHandled {
		t.Fatal("inner ack handler")
	}
}

func TestChainNonInviteServerTx(t *testing.T) {
	if ChainNonInviteServerTx(nil, nil) != nil {
		t.Fatal("nil inner")
	}
	mgr := transaction.NewManager()
	var n int
	h := ChainNonInviteServerTx(mgr, func(*stack.Message, *net.UDPAddr) (*stack.Message, error) {
		n++
		return ErrorResponse(&stack.Message{IsRequest: true, Method: stack.MethodOptions, Version: "SIP/2.0"}, 200, "OK")
	})
	req := &stack.Message{IsRequest: true, Method: stack.MethodOptions, Version: "SIP/2.0"}
	req.SetHeader("Via", "SIP/2.0/UDP x;branch=opt1")
	req.SetHeader("Call-ID", "opt-cid")
	req.SetHeader("CSeq", "1 OPTIONS")
	if _, err := h(req, &net.UDPAddr{}); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("calls=%d", n)
	}
}

func TestChainInviteServerTx_NilInner(t *testing.T) {
	if ChainInviteServerTx(transaction.NewManager(), nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestAfterResponseSentBeginInviteServer_Alias(t *testing.T) {
	fn := AfterResponseSentBeginInviteServer(transaction.NewManager(), context.Background(), nil)
	fn(nil, nil, nil)
}
