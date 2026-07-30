package stack

import (
	"context"
	"net"
)

// DatagramTransport defines an interface for connectionless SIP message transport using datagrams (e.g., UDP).
// Implementations must be safe for one concurrent reader and multiple concurrent writers.
type DatagramTransport interface {
	// ReadFrom reads a single datagram into the provided buffer.
	// It returns the number of bytes read, the source UDP address, and any error encountered.
	// If the context is canceled, ReadFrom returns ctx.Err() without blocking indefinitely.
	ReadFrom(ctx context.Context, buf []byte) (n int, addr *net.UDPAddr, err error)

	// WriteTo sends a datagram to the specified remote UDP address.
	// It returns the number of bytes written and any error encountered.
	// If the context is canceled, WriteTo returns ctx.Err() immediately.
	WriteTo(ctx context.Context, p []byte, addr *net.UDPAddr) (n int, err error)

	// Close terminates the transport and closes the underlying connection.
	Close() error

	// LocalAddr returns the local network address the transport is bound to.
	LocalAddr() net.Addr

	// String returns a human-readable identifier for the transport implementation.
	String() string
}

// UDPTransport implements the DatagramTransport interface using a UDP network connection.
// It wraps the standard *net.UDPConn to provide context-aware datagram I/O for SIP signaling.
type UDPTransport struct {
	conn *net.UDPConn // Underlying UDP network connection
}

// NewUDPTransport creates a new UDPTransport instance by wrapping an existing *net.UDPConn.
// The caller is responsible for managing the lifecycle of the provided UDP connection.
func NewUDPTransport(conn *net.UDPConn) *UDPTransport {
	return &UDPTransport{conn: conn}
}

// String returns the type name of the transport for logging and debugging purposes.
func (t *UDPTransport) String() string { return "UDPTransport" }

// LocalAddr returns the local network address that the UDP transport is bound to.
// Returns nil if the transport or underlying connection is not initialized.
func (t *UDPTransport) LocalAddr() net.Addr {
	if t == nil || t.conn == nil {
		return nil
	}
	return t.conn.LocalAddr()
}

// ReadFrom reads an incoming UDP datagram into the provided buffer.
// It checks for context cancellation before performing the read to avoid blocking indefinitely.
// Returns the number of bytes read, the sender's UDP address, and any error (including context cancellation).
func (t *UDPTransport) ReadFrom(ctx context.Context, buf []byte) (int, *net.UDPAddr, error) {
	if t == nil || t.conn == nil {
		return 0, nil, ErrUDPTransportNotStarted
	}

	// Return immediately if context is cancelled
	if ctx != nil {
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		default:
		}
	}

	return t.conn.ReadFromUDP(buf)
}

// WriteTo transmits a buffer as a UDP datagram to the specified remote address.
// It checks for context cancellation before performing the write operation.
// Returns the number of bytes written and any error (including context cancellation).
func (t *UDPTransport) WriteTo(ctx context.Context, p []byte, addr *net.UDPAddr) (int, error) {
	if t == nil || t.conn == nil {
		return 0, ErrUDPTransportNotStarted
	}

	// Return immediately if context is cancelled
	if ctx != nil {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
	}

	return t.conn.WriteToUDP(p, addr)
}

// Close shuts down the underlying UDP connection and releases associated resources.
// It is safe to call multiple times and returns nil if the connection is already closed.
func (t *UDPTransport) Close() error {
	if t == nil || t.conn == nil {
		return nil
	}
	return t.conn.Close()
}
