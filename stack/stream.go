package stack

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"time"
)

// StreamReadTimeout is how long a stream connection may stay idle before it is dropped.
const StreamReadTimeout = 90 * time.Second

// streamAcceptTimeout bounds Accept so listeners notice context cancellation promptly.
const streamAcceptTimeout = 2 * time.Second

// ServeStreamConn reads SIP messages from a connection-oriented transport (TCP or TLS) and
// dispatches them through ep. Requests are answered on the same connection; responses are
// handed to the endpoint's OnSIPResponse hook.
//
// The connection is closed on return. ServeStreamConn returns when ctx is cancelled, the peer
// closes, or a read fails (including the StreamReadTimeout idle deadline).
func ServeStreamConn(ctx context.Context, conn net.Conn, ep *Endpoint) {
	if conn == nil {
		return
	}
	defer func() { _ = conn.Close() }()
	ra, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return
	}
	udpAddr := &net.UDPAddr{IP: ra.IP, Port: ra.Port}
	br := bufio.NewReader(conn)
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		_ = conn.SetReadDeadline(time.Now().Add(StreamReadTimeout))
		msg, err := ReadMessage(br)
		if err != nil {
			return
		}
		if !msg.IsRequest {
			ep.InvokeOnSIPResponse(msg, udpAddr)
			continue
		}
		resp := ep.DispatchRequest(msg, udpAddr)
		if resp != nil {
			if _, err := conn.Write([]byte(resp.String())); err != nil {
				return
			}
		}
	}
}

// ListenAndServeTCP accepts SIP-over-TCP connections on addr until ctx is cancelled, serving
// each with ServeStreamConn. It returns only if the listener cannot be created or ctx ends.
func ListenAndServeTCP(ctx context.Context, addr string, ep *Endpoint) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return serveStreamListener(ctx, ln, ln, ep)
}

// ListenAndServeTLS accepts SIP-over-TLS connections on addr until ctx is cancelled, serving
// each with ServeStreamConn.
func ListenAndServeTLS(ctx context.Context, addr string, tlsCfg *tls.Config, ep *Endpoint) error {
	plain, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return serveStreamListener(ctx, tls.NewListener(plain, tlsCfg), plain, ep)
}

// serveStreamListener runs the accept loop. deadliner is the underlying (pre-TLS) listener,
// used to bound Accept so ctx cancellation is observed even without inbound traffic.
func serveStreamListener(ctx context.Context, ln net.Listener, deadliner net.Listener, ep *Endpoint) error {
	defer func() { _ = ln.Close() }()
	if ctx != nil {
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-ctx.Done():
				_ = ln.Close()
			case <-done:
			}
		}()
	}
	tcpLn, _ := deadliner.(*net.TCPListener)
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		if tcpLn != nil {
			_ = tcpLn.SetDeadline(time.Now().Add(streamAcceptTimeout))
		}
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go ServeStreamConn(ctx, conn, ep)
	}
}
