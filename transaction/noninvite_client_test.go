package transaction

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LingByte/lingsip/stack"
)

func TestRunNonInviteClient_Success(t *testing.T) {
	m := NewManager()
	m.SetT1(10 * time.Millisecond)
	m.SetT2(20 * time.Millisecond)

	raw := strings.Join([]string{
		"BYE sip:a@b SIP/2.0",
		"Via: SIP/2.0/UDP 10.0.0.1:5060;branch=z9hG4bKbye1",
		"From: <sip:a@b>;tag=f1",
		"To: <sip:x@y>;tag=t1",
		"Call-ID: bye-cid",
		"CSeq: 2 BYE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	req, err := stack.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	remote := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5060}
	send := func(msg *stack.Message, addr *net.UDPAddr) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *NonInviteClientResult, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := m.RunNonInviteClient(ctx, req, remote, send)
		if err != nil {
			errCh <- err
			done <- nil
			return
		}
		done <- res
	}()

	// Wait until the client tx is registered.
	time.Sleep(20 * time.Millisecond)

	resp200 := &stack.Message{IsRequest: false, StatusCode: 200, Version: "SIP/2.0"}
	resp200.SetHeader("Via", "SIP/2.0/UDP 10.0.0.1:5060;branch=z9hG4bKbye1")
	resp200.SetHeader("Call-ID", "bye-cid")
	resp200.SetHeader("CSeq", "2 BYE")
	src := &net.UDPAddr{IP: net.ParseIP("10.0.0.2"), Port: 5060}
	if !m.HandleResponse(resp200, src) {
		t.Fatal("HandleResponse should consume BYE 200")
	}
	select {
	case res := <-done:
		if res == nil || res.Final == nil || res.Final.StatusCode != 200 {
			t.Fatalf("result: %+v", res)
		}
		if res.Remote == nil || res.Remote.IP.String() != "10.0.0.2" {
			t.Fatalf("remote: %+v", res.Remote)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestDispatchNonInviteClientResponse_NoMatch(t *testing.T) {
	m := NewManager()
	resp := &stack.Message{IsRequest: false, StatusCode: 200}
	resp.SetHeader("Via", "SIP/2.0/UDP x;branch=b")
	resp.SetHeader("Call-ID", "unknown")
	resp.SetHeader("CSeq", "1 BYE")
	if m.dispatchNonInviteClientResponse(resp, nil) {
		t.Fatal("no tx registered")
	}
}

func TestNonInviteClientTx_handleResponse(t *testing.T) {
	tx := &nonInviteClientTx{
		stopCh:  make(chan struct{}),
		finalCh: make(chan *stack.Message, 1),
	}
	provisional := &stack.Message{IsRequest: false, StatusCode: 100}
	if !tx.handleResponse(provisional, nil) {
		t.Fatal("1xx consumed")
	}
	final := &stack.Message{IsRequest: false, StatusCode: 200}
	if !tx.handleResponse(final, nil) {
		t.Fatal("2xx consumed")
	}
	if !tx.handleResponse(final, nil) {
		t.Fatal("duplicate 2xx consumed")
	}
	if tx.handleResponse(nil, nil) {
		t.Fatal("nil resp")
	}
}

func TestRunNonInviteClientReliable_NoRetransmit(t *testing.T) {
	m := NewManager()
	m.SetT1(50 * time.Millisecond)
	remote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}
	req := testNonInviteRequest(t, stack.MethodBye)

	var sends atomic.Int32
	send := func(*stack.Message, *net.UDPAddr) error {
		sends.Add(1)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = m.RunNonInviteClientReliable(ctx, req, remote, send)
	}()

	time.Sleep(180 * time.Millisecond)
	if sends.Load() != 1 {
		t.Fatalf("reliable mode sends=%d want 1", sends.Load())
	}
	<-done
}

func testNonInviteRequest(t *testing.T, method string) *stack.Message {
	t.Helper()
	raw := method + " sip:c@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP 192.168.1.10:5060;branch=z9hG4bKnoninv\r\n" +
		"From: <sip:a@local>;tag=1\r\n" +
		"To: <sip:c@example.com>;tag=2\r\n" +
		"Call-ID: noninv-1\r\n" +
		"CSeq: 2 " + method + "\r\n" +
		"Content-Length: 0\r\n\r\n"
	msg, err := stack.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}
