package stack

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// HandlerFunc defines a callback function to process an incoming SIP request.
// It receives the parsed SIP message and remote UDP address, then returns a SIP response message.
// Return nil to skip sending any response.
type HandlerFunc func(msg *Message, addr *net.UDPAddr) *Message

// EventType enumerates types of telemetry events emitted by the Endpoint.
type EventType int

// Enumeration of Endpoint event types
const (
	EventDatagramReceived EventType = iota // A raw UDP datagram was received
	EventParseError                        // Failed to parse received datagram into a valid SIP message
	EventRequestReceived                   // A valid SIP request was received and parsed
	EventResponseReceived                  // A valid SIP response was received and parsed
	EventResponseSent                      // A SIP response was successfully sent to the remote peer
)

// Event represents a lightweight observation/telemetry record from the Endpoint's read loop.
// It carries metadata about received datagrams, parsed messages, parsing errors, and sent responses.
type Event struct {
	Type     EventType    // Type of the event
	Addr     *net.UDPAddr // Remote network address associated with the event
	Raw      []byte       // Raw binary data of the received UDP datagram
	Request  *Message     // Parsed SIP request (populated for request-related events)
	Response *Message     // Parsed SIP response (populated for response-related events)
	Err      error        // Error details (populated for error-related events)
}

// RequestMethod returns the SIP request method for request-related events.
func (e Event) RequestMethod() string {
	if e.Request == nil {
		return ""
	}
	return e.Request.Method
}

// ResponseStatus returns the SIP status code for response-related events.
func (e Event) ResponseStatus() int {
	if e.Response == nil {
		return 0
	}
	return e.Response.StatusCode
}

// EndpointConfig contains configuration parameters for initializing a UDP-based SIP Endpoint.
type EndpointConfig struct {
	// Host and Port specify the IPv4 UDP listen address for the Endpoint
	Host string
	Port int

	ReadBufSize  int           // Size of the read buffer for incoming UDP datagrams; defaults to 65535
	ReadDeadline time.Duration // Timeout for each read operation to enable poll-based shutdown; defaults to 1s

	// OnReadError is invoked when a non-timeout read error occurs on the UDP socket
	OnReadError func(err error)
	// OnDatagram is invoked when a raw UDP datagram is received
	OnDatagram func(raw []byte, addr *net.UDPAddr)
	// OnParseErr is invoked when parsing a received datagram into a SIP message fails
	OnParseErr func(raw []byte, addr *net.UDPAddr, err error)
	// OnRequest is invoked when a valid SIP request is received and parsed
	OnRequest func(req *Message, addr *net.UDPAddr)
	// OnResponse is invoked before sending a generated SIP response
	OnResponse func(req *Message, resp *Message, addr *net.UDPAddr)
	// OnResponseSent is invoked after a SIP response is successfully transmitted
	OnResponseSent func(req *Message, resp *Message, addr *net.UDPAddr)
	// OnSIPResponse is invoked when a valid SIP response is received from the network
	OnSIPResponse func(resp *Message, addr *net.UDPAddr)
	// OnMessageSent is invoked after any SIP message (request/response) is sent
	OnMessageSent func(msg *Message, addr *net.UDPAddr)
	// OnEvent is a generic callback for all Endpoint events
	OnEvent func(e Event)
	// NoRouteHandler is the fallback handler for SIP requests with unregistered methods
	NoRouteHandler HandlerFunc
}

// Endpoint implements a UDP-based SIP signaling endpoint.
// It handles socket binding, datagram reception, SIP message parsing, request routing, and response transmission.
// Safe for concurrent use by multiple goroutines.
type Endpoint struct {
	cfg      EndpointConfig         // Configuration of the endpoint
	mu       sync.RWMutex           // Mutex to protect concurrent access to internal state
	handlers map[string]HandlerFunc // Registered SIP method handlers (method -> handler)
	tr       *UDPTransport          // Underlying UDP transport connection
}

// NewEndpoint creates and initializes a new SIP Endpoint with the given configuration.
// Default values are applied for unspecified optional parameters.
// Returns a pointer to the created Endpoint.
func NewEndpoint(cfg EndpointConfig) *Endpoint {
	if cfg.ReadBufSize <= 0 {
		cfg.ReadBufSize = 65535
	}
	if cfg.ReadDeadline <= 0 {
		cfg.ReadDeadline = time.Second
	}
	return &Endpoint{
		cfg:      cfg,
		handlers: make(map[string]HandlerFunc),
	}
}

// RegisterHandler registers a HandlerFunc for a specific SIP request method.
// The method name is case-insensitive and will be normalized to uppercase.
// Safe for concurrent calls.
func (e *Endpoint) RegisterHandler(method string, h HandlerFunc) {
	if e == nil {
		return
	}
	method = strings.ToUpper(strings.TrimSpace(method))

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.handlers == nil {
		e.handlers = make(map[string]HandlerFunc)
	}
	e.handlers[method] = h
}

// Open binds and opens the underlying UDP listen socket using the configured Host and Port.
// Must be called before Serve or Send.
// Returns an error if binding fails.
func (e *Endpoint) Open() error {
	if e == nil {
		return ErrNilEndpoint
	}

	addr := &net.UDPAddr{
		IP:   net.ParseIP(strings.TrimSpace(e.cfg.Host)),
		Port: e.cfg.Port,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return errListenUDP(err)
	}

	e.mu.Lock()
	e.tr = NewUDPTransport(conn)
	e.mu.Unlock()

	return nil
}

// Transport returns the underlying DatagramTransport interface after the Endpoint is opened.
// Returns nil if the Endpoint has not been opened.
func (e *Endpoint) Transport() DatagramTransport {
	if e == nil {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.tr
}

// ListenAddr returns the local network address that the Endpoint is bound to.
// Returns nil if the Endpoint is not open or not bound.
func (e *Endpoint) ListenAddr() net.Addr {
	if e == nil {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.tr == nil || e.tr.conn == nil {
		return nil
	}
	return e.tr.conn.LocalAddr()
}

// Send transmits a SIP message to the specified remote UDP address.
// Returns an error if the Endpoint is closed, message is nil, or transmission fails.
// Triggers OnMessageSent callback on successful transmission.
func (e *Endpoint) Send(msg *Message, addr *net.UDPAddr) error {
	if e == nil {
		return ErrNilEndpoint
	}

	e.mu.RLock()
	tr := e.tr
	e.mu.RUnlock()

	if tr == nil || tr.conn == nil {
		return ErrEndpointNotOpen
	}
	if msg == nil {
		return ErrNilMessage
	}

	raw := msg.String()
	_, err := tr.conn.WriteToUDP([]byte(raw), addr)
	if err == nil && e.cfg.OnMessageSent != nil {
		e.cfg.OnMessageSent(msg, addr)
	}

	return err
}

// Close shuts down the UDP listen socket and stops the Serve loop.
// Safe for concurrent calls and idempotent.
func (e *Endpoint) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	tr := e.tr
	e.tr = nil
	e.mu.Unlock()

	if tr == nil {
		return nil
	}
	return tr.Close()
}

// AppendOnResponseSent chains a new callback after the existing OnResponseSent handler.
// The original callback (if any) is executed first, followed by the new function.
// Must be called before Serve starts to avoid race conditions.
func (e *Endpoint) AppendOnResponseSent(fn func(*Message, *Message, *net.UDPAddr)) {
	if e == nil || fn == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	prev := e.cfg.OnResponseSent
	e.cfg.OnResponseSent = func(req, resp *Message, addr *net.UDPAddr) {
		if prev != nil {
			prev(req, resp, addr)
		}
		fn(req, resp, addr)
	}
}

// Serve runs the main read-parse-dispatch loop for incoming SIP datagrams.
// It runs until the context is canceled, Close is called, or a fatal read error occurs.
// Returns ctx.Err() on context cancellation, or a wrapped error on I/O failures.
func (e *Endpoint) Serve(ctx context.Context) error {
	if e == nil {
		return ErrNilEndpoint
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.mu.RLock()
	tr := e.tr
	e.mu.RUnlock()

	if tr == nil || tr.conn == nil {
		return ErrEndpointNotOpen
	}

	buf := make([]byte, e.cfg.ReadBufSize)
	for {
		// Exit loop if context is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Set read timeout to enable periodic context cancellation checks
		_ = tr.conn.SetReadDeadline(time.Now().Add(e.cfg.ReadDeadline))
		n, addr, err := tr.conn.ReadFromUDP(buf)
		if err != nil {
			// Ignore timeout errors and continue loop
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			// Handle fatal read error
			if e.cfg.OnReadError != nil {
				e.cfg.OnReadError(err)
			}
			return errReadUDP(err)
		}

		// Copy received data to avoid buffer overwriting
		rawBytes := append([]byte(nil), buf[:n]...)
		if e.cfg.OnDatagram != nil {
			e.cfg.OnDatagram(rawBytes, addr)
		}

		// Skip non-SIP keepalive/noise datagrams
		if IsSignalingNoiseDatagram(rawBytes) {
			continue
		}
		if e.cfg.OnEvent != nil {
			e.cfg.OnEvent(Event{Type: EventDatagramReceived, Raw: rawBytes, Addr: addr})
		}

		// Parse raw data into SIP message
		raw := string(rawBytes)
		msg, err := Parse(raw)
		if err != nil {
			if e.cfg.OnParseErr != nil {
				e.cfg.OnParseErr(rawBytes, addr, err)
			}
			if e.cfg.OnEvent != nil {
				e.cfg.OnEvent(Event{Type: EventParseError, Raw: rawBytes, Addr: addr, Err: err})
			}
			continue
		}
		if msg == nil {
			continue
		}

		// Handle received SIP response
		if !msg.IsRequest {
			if e.cfg.OnSIPResponse != nil {
				e.cfg.OnSIPResponse(msg, addr)
			}
			if e.cfg.OnEvent != nil {
				e.cfg.OnEvent(Event{
					Type:     EventResponseReceived,
					Addr:     addr,
					Raw:      rawBytes,
					Response: msg,
				})
			}
			continue
		}

		// Handle received SIP request
		if e.cfg.OnRequest != nil {
			e.cfg.OnRequest(msg, addr)
		}
		if e.cfg.OnEvent != nil {
			e.cfg.OnEvent(Event{
				Type:    EventRequestReceived,
				Addr:    addr,
				Raw:     rawBytes,
				Request: msg,
			})
		}

		// Route request to registered handler
		method := strings.ToUpper(msg.Method)
		e.mu.RLock()
		h := e.handlers[method]
		if h == nil {
			h = e.cfg.NoRouteHandler
		}
		e.mu.RUnlock()

		if h == nil {
			continue
		}

		// Execute handler and send response if provided
		resp := h(msg, addr)
		if resp == nil {
			continue
		}

		if e.cfg.OnResponse != nil {
			e.cfg.OnResponse(msg, resp, addr)
		}
		if e.cfg.OnEvent != nil {
			e.cfg.OnEvent(Event{
				Type:     EventResponseSent,
				Addr:     addr,
				Request:  msg,
				Response: resp,
			})
		}

		// Send generated response
		if err := e.Send(resp, addr); err != nil {
			return errSendResponse(err)
		}

		// Trigger post-send callback
		e.mu.RLock()
		onSent := e.cfg.OnResponseSent
		e.mu.RUnlock()

		if onSent != nil {
			onSent(msg, resp, addr)
		}
	}
}

// SetNoRouteHandler sets the fallback handler for SIP requests with no registered method handler.
// Safe for concurrent calls.
func (e *Endpoint) SetNoRouteHandler(h HandlerFunc) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.NoRouteHandler = h
}

// NotifyResponseSent invokes OnResponseSent without sending on the wire.
// Used when tests or alternate transports dispatch handlers directly.
func (e *Endpoint) NotifyResponseSent(req, resp *Message, addr *net.UDPAddr) {
	if e == nil || req == nil || resp == nil {
		return
	}
	e.mu.RLock()
	fn := e.cfg.OnResponseSent
	e.mu.RUnlock()
	if fn != nil {
		fn(req, resp, addr)
	}
}

// DispatchRequest routes a SIP request to the registered handler without network I/O.
// Used by non-UDP transports (e.g., TCP/TLS) that handle transmission independently.
// Returns the generated response message or nil.
func (e *Endpoint) DispatchRequest(req *Message, addr *net.UDPAddr) *Message {
	if e == nil || req == nil {
		return nil
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	e.mu.RLock()
	h := e.handlers[method]
	if h == nil {
		h = e.cfg.NoRouteHandler
	}
	e.mu.RUnlock()

	if h == nil {
		return nil
	}
	return h(req, addr)
}

// InvokeOnSIPResponse triggers the OnSIPResponse callback with the given response and address.
// Used for responses received over alternate transports (e.g., TCP/TLS).
func (e *Endpoint) InvokeOnSIPResponse(resp *Message, addr *net.UDPAddr) {
	if e == nil {
		return
	}

	e.mu.RLock()
	fn := e.cfg.OnSIPResponse
	e.mu.RUnlock()

	if fn != nil {
		fn(resp, addr)
	}
}

// IsSignalingNoiseDatagram checks if a datagram is non-SIP signaling noise (e.g., NAT keepalives).
// Valid noise consists solely of whitespace, CR, LF, or tab characters within a small size limit.
// Such datagrams can be safely ignored to reduce parsing overhead.
func IsSignalingNoiseDatagram(b []byte) bool {
	if len(b) == 0 || len(b) > 64 {
		return false
	}
	for _, c := range b {
		if c != '\r' && c != '\n' && c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}
