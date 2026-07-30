package stack

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEndpoint_OPTIONS_handler(t *testing.T) {
	ep := NewEndpoint(EndpointConfig{
		Host:         "127.0.0.1",
		Port:         0,
		ReadDeadline: 200 * time.Millisecond,
	})
	if err := ep.Open(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ep.Close() }()

	local := ep.ListenAddr().(*net.UDPAddr)

	ep.RegisterHandler(MethodOptions, func(msg *Message, addr *net.UDPAddr) *Message {
		resp := NewResponse(StatusOK, ReasonOK)
		resp.SetHeader(HeaderCallID, msg.GetHeader(HeaderCallID))
		resp.SetHeader(HeaderCSeq, msg.GetHeader(HeaderCSeq))
		resp.SetHeader(HeaderContentLength, "0")
		return resp
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ep.Serve(ctx)
	}()

	c, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	req := strings.Join([]string{
		"OPTIONS sip:u@" + local.String() + " " + VersionSIP20,
		HeaderVia + ": " + TransportPrefix + " 127.0.0.1;branch=z9hG4bKtest",
		"From: <sip:a@b>;tag=1",
		"To: <sip:a@b>",
		HeaderCallID + ": test-call",
		HeaderCSeq + ": 1 " + MethodOptions,
		HeaderContentLength + ": 0",
		"",
		"",
	}, "\r\n")
	if _, err := c.Write([]byte(req)); err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}

	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	resp, err := Parse(string(buf[:n]))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	cancel()
	wg.Wait()
}

func TestEndpoint_OnResponseSent(t *testing.T) {
	var sent atomic.Int32
	ep := NewEndpoint(EndpointConfig{
		Host:           "127.0.0.1",
		Port:           0,
		ReadDeadline:   200 * time.Millisecond,
		OnResponseSent: func(req, resp *Message, addr *net.UDPAddr) { sent.Add(1) },
	})
	if err := ep.Open(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ep.Close() }()
	local := ep.ListenAddr().(*net.UDPAddr)
	ep.RegisterHandler(MethodOptions, func(msg *Message, addr *net.UDPAddr) *Message {
		resp := NewResponse(StatusOK, ReasonOK)
		resp.SetHeader(HeaderCallID, msg.GetHeader(HeaderCallID))
		resp.SetHeader(HeaderCSeq, msg.GetHeader(HeaderCSeq))
		resp.SetHeader(HeaderContentLength, "0")
		return resp
	})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = ep.Serve(ctx) }()
	c, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	req := strings.Join([]string{
		"OPTIONS sip:x " + VersionSIP20,
		HeaderVia + ": " + TransportPrefix + " 1.1.1.1;branch=z9hG4bKx",
		HeaderFrom + ": f",
		HeaderTo + ": t",
		HeaderCallID + ": c",
		HeaderCSeq + ": 1 " + MethodOptions,
		HeaderContentLength + ": 0",
		"", "",
	}, "\r\n")
	if _, err := c.Write([]byte(req)); err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	if _, err := c.Read(buf); err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	cancel()
	wg.Wait()
	if sent.Load() != 1 {
		t.Fatalf("OnResponseSent calls=%d", sent.Load())
	}
}

func TestEndpoint_OnSIPResponseViaUDP(t *testing.T) {
	var saw atomic.Bool
	ep := NewEndpoint(EndpointConfig{
		Host:          "127.0.0.1",
		Port:          0,
		ReadDeadline:  200 * time.Millisecond,
		OnSIPResponse: func(resp *Message, _ *net.UDPAddr) { saw.Store(resp != nil && resp.StatusCode == StatusOK) },
	})
	if err := ep.Open(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ep.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ep.Serve(ctx)
	}()
	defer func() {
		cancel()
		wg.Wait()
	}()

	la := ep.ListenAddr().(*net.UDPAddr)
	raw := ResponseStatusLine(StatusOK, ReasonOK) + "\r\n" +
		HeaderVia + ": " + TransportPrefix + " 127.0.0.1:9;branch=z9hG4bKtest\r\n" +
		HeaderFrom + ": <sip:a@b>;tag=1\r\n" +
		HeaderTo + ": <sip:a@b>;tag=2\r\n" +
		HeaderCallID + ": cid-1\r\n" +
		HeaderCSeq + ": 1 " + MethodInvite + "\r\n" +
		HeaderContentLength + ": 0\r\n\r\n"
	addr, _ := net.ResolveUDPAddr("udp", la.String())
	c, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte(raw)); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if saw.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("OnSIPResponse not invoked for 200 OK")
}

func TestEvent_RequestMethod_ResponseStatus(t *testing.T) {
	var e Event
	if e.RequestMethod() != "" || e.ResponseStatus() != 0 {
		t.Fatal("empty event")
	}
	e.Request = &Message{Method: MethodInvite, IsRequest: true}
	if e.RequestMethod() != MethodInvite {
		t.Fatalf("got %q", e.RequestMethod())
	}
	e.Response = &Message{IsRequest: false, StatusCode: StatusRinging, StatusText: ReasonRinging}
	if e.ResponseStatus() != StatusRinging {
		t.Fatalf("got %d", e.ResponseStatus())
	}
}

func TestEndpoint_OnEventTelemetry(t *testing.T) {
	var events []EventType
	ep := NewEndpoint(EndpointConfig{
		Host:         "127.0.0.1",
		Port:         0,
		ReadDeadline: 200 * time.Millisecond,
		OnEvent: func(e Event) {
			events = append(events, e.Type)
			if e.Type == EventRequestReceived && e.RequestMethod() != MethodOptions {
				t.Errorf("unexpected method %q", e.RequestMethod())
			}
			if e.Type == EventResponseSent && e.ResponseStatus() != StatusOK {
				t.Errorf("unexpected status %d", e.ResponseStatus())
			}
		},
	})
	if err := ep.Open(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ep.Close() }()
	local := ep.ListenAddr().(*net.UDPAddr)
	ep.RegisterHandler(MethodOptions, func(msg *Message, addr *net.UDPAddr) *Message {
		resp := NewResponse(StatusOK, ReasonOK)
		resp.SetHeader(HeaderCallID, msg.GetHeader(HeaderCallID))
		resp.SetHeader(HeaderCSeq, msg.GetHeader(HeaderCSeq))
		resp.SetHeader(HeaderContentLength, "0")
		return resp
	})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = ep.Serve(ctx) }()
	c, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	defer c.Close()
	req := strings.Join([]string{
		"OPTIONS sip:x " + VersionSIP20,
		HeaderVia + ": " + TransportPrefix + " 1.1.1.1;branch=z9hG4bKx",
		HeaderFrom + ": f",
		HeaderTo + ": t",
		HeaderCallID + ": ev",
		HeaderCSeq + ": 1 " + MethodOptions,
		HeaderContentLength + ": 0",
		"", "",
	}, "\r\n")
	if _, err := c.Write([]byte(req)); err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	if _, err := c.Read(buf); err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	cancel()
	wg.Wait()
	want := map[EventType]bool{
		EventDatagramReceived: true,
		EventRequestReceived:  true,
		EventResponseSent:     true,
	}
	for _, typ := range events {
		delete(want, typ)
	}
	if len(want) != 0 {
		t.Fatalf("missing events: %v got %v", want, events)
	}
}
