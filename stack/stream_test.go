package stack

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

const streamTestOPTIONS = "OPTIONS sip:probe@example.com SIP/2.0\r\n" +
	"Via: SIP/2.0/TCP 127.0.0.1:5060;branch=z9hG4bK-stream\r\n" +
	"From: <sip:probe@example.com>;tag=abc\r\n" +
	"To: <sip:probe@example.com>\r\n" +
	"Call-ID: stream-test\r\n" +
	"CSeq: 1 OPTIONS\r\n" +
	"Content-Length: 0\r\n\r\n"

const streamTest200 = "SIP/2.0 200 OK\r\n" +
	"Via: SIP/2.0/TCP 127.0.0.1:5060;branch=z9hG4bK-stream\r\n" +
	"Call-ID: stream-resp\r\n" +
	"CSeq: 1 OPTIONS\r\n" +
	"Content-Length: 0\r\n\r\n"

func newStreamTestEndpoint(gotResp chan<- *Message) *Endpoint {
	ep := NewEndpoint(EndpointConfig{
		OnSIPResponse: func(resp *Message, _ *net.UDPAddr) {
			select {
			case gotResp <- resp:
			default:
			}
		},
	})
	ep.RegisterHandler(MethodOptions, func(msg *Message, _ *net.UDPAddr) *Message {
		resp := NewResponse(StatusOK, "OK")
		resp.SetHeader("Call-ID", msg.GetHeader("Call-ID"))
		return resp
	})
	return ep
}

func TestServeStreamConn_RequestAndResponse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gotResp := make(chan *Message, 1)
	ep := newStreamTestEndpoint(gotResp)

	served := make(chan struct{})
	go func() {
		defer close(served)
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		ServeStreamConn(ctx, conn, ep)
	}()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte(streamTestOPTIONS)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := ReadMessage(bufio.NewReader(conn))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != StatusOK || resp.GetHeader("Call-ID") != "stream-test" {
		t.Fatalf("status=%d call-id=%q", resp.StatusCode, resp.GetHeader("Call-ID"))
	}

	if _, err := conn.Write([]byte(streamTest200)); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-gotResp:
		if got.GetHeader("Call-ID") != "stream-resp" {
			t.Fatalf("call-id=%q", got.GetHeader("Call-ID"))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnSIPResponse not invoked")
	}

	cancel()
	_ = conn.Close()
	select {
	case <-served:
	case <-time.After(3 * time.Second):
		t.Fatal("ServeStreamConn did not return")
	}
}

func TestServeStreamConn_NilAndNonTCPConn(t *testing.T) {
	ServeStreamConn(context.Background(), nil, nil)

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeStreamConn(context.Background(), server, nil)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("non-TCP conn should be rejected immediately")
	}
}

func TestListenAndServeTCP(t *testing.T) {
	addr := freeLocalAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- ListenAndServeTCP(ctx, addr, newStreamTestEndpoint(nil)) }()

	conn := dialWithRetry(t, addr, nil)
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte(streamTestOPTIONS)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := ReadMessage(bufio.NewReader(conn))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected ctx error on shutdown")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("ListenAndServeTCP did not stop")
	}
}

func TestListenAndServeTLS(t *testing.T) {
	addr := freeLocalAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cert := selfSignedCert(t)
	go func() {
		_ = ListenAndServeTLS(ctx, addr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}, newStreamTestEndpoint(nil))
	}()

	conn := dialWithRetry(t, addr, &tls.Config{InsecureSkipVerify: true})
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte(streamTestOPTIONS)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := ReadMessage(bufio.NewReader(conn))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestListenAndServeBadAddr(t *testing.T) {
	if err := ListenAndServeTCP(context.Background(), "127.0.0.1:-1", nil); err == nil {
		t.Fatal("expected listen error")
	}
	if err := ListenAndServeTLS(context.Background(), "127.0.0.1:-1", nil, nil); err == nil {
		t.Fatal("expected listen error")
	}
}

func freeLocalAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// dialWithRetry retries because the listener goroutine may not have bound yet.
func dialWithRetry(t *testing.T, addr string, tlsCfg *tls.Config) net.Conn {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var conn net.Conn
		var err error
		if tlsCfg != nil {
			conn, err = tls.Dial("tcp", addr, tlsCfg)
		} else {
			conn, err = net.DialTimeout("tcp", addr, time.Second)
		}
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial %s: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
