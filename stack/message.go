package stack

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Message is a parsed SIP message (request or response).
type Message struct {
	Method       string
	RequestURI   string
	StatusCode   int
	StatusText   string
	Version      string
	Headers      map[string]string   // first value per canonical header name
	HeadersMulti map[string][]string // all values per canonical header name (e.g. Via)
	Body         string
	IsRequest    bool
}

func canonicalHeaderKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Parse parses a raw SIP message into a Message.
// Header lines use ":"; body follows the first empty line. CRLF and bare LF are accepted.
//
// Header folding (RFC 3261 continuation lines) is not implemented; folded headers may parse incorrectly.
func Parse(raw string) (*Message, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrEmptyMessage
	}

	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return nil, ErrEmptyMessageLines
	}

	firstLine := strings.TrimSpace(lines[0])
	if firstLine == "" {
		return nil, ErrEmptyFirstLine
	}

	msg := &Message{
		Headers:      make(map[string]string),
		HeadersMulti: make(map[string][]string),
	}

	if strings.HasPrefix(firstLine, VersionPrefix) {
		msg.IsRequest = false
		parts := strings.SplitN(firstLine, " ", 3)
		if len(parts) < 2 {
			return nil, errInvalidResponseLine(firstLine)
		}
		msg.Version = strings.TrimSpace(parts[0])
		code, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, errInvalidStatusCode(firstLine, err)
		}
		msg.StatusCode = code
		if len(parts) >= 3 {
			msg.StatusText = strings.TrimSpace(parts[2])
		}
	} else {
		msg.IsRequest = true
		parts := strings.SplitN(firstLine, " ", 3)
		if len(parts) < 2 {
			return nil, errInvalidRequestLine(firstLine)
		}
		msg.Method = strings.ToUpper(strings.TrimSpace(parts[0]))
		msg.RequestURI = strings.TrimSpace(parts[1])
		if len(parts) >= 3 {
			msg.Version = strings.TrimSpace(parts[2])
		} else {
			msg.Version = VersionSIP20
		}
	}

	bodyStart := -1
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			bodyStart = i + 1
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key != "" {
				ck := canonicalHeaderKey(key)
				if _, exists := msg.Headers[ck]; !exists {
					msg.Headers[ck] = val
				}
				msg.HeadersMulti[ck] = append(msg.HeadersMulti[ck], val)
			}
		}
	}

	if bodyStart > 0 && bodyStart < len(lines) {
		msg.Body = strings.Join(lines[bodyStart:], "\n")
	}

	return msg, nil
}

// String serializes the message to SIP wire format (CRLF line endings).
func (m *Message) String() string {
	if m == nil {
		return ""
	}

	var b strings.Builder
	if m.IsRequest {
		b.WriteString(fmt.Sprintf("%s %s %s\r\n", m.Method, m.RequestURI, m.Version))
	} else {
		b.WriteString(fmt.Sprintf("%s %d %s\r\n", m.Version, m.StatusCode, m.StatusText))
	}

	preferred := preferredHeaderOrder

	emitted := make(map[string]struct{}, 32)
	for _, k := range preferred {
		vals := m.HeadersMulti[k]
		if len(vals) == 0 {
			if v, ok := m.Headers[k]; ok && v != "" {
				vals = []string{v}
			}
		}
		if len(vals) == 0 {
			continue
		}
		for _, v := range vals {
			b.WriteString(fmt.Sprintf("%s: %s\r\n", prettyHeaderName(k), v))
		}
		emitted[k] = struct{}{}
	}

	restKeys := make([]string, 0, len(m.HeadersMulti))
	for k := range m.HeadersMulti {
		if _, ok := emitted[k]; ok {
			continue
		}
		restKeys = append(restKeys, k)
	}
	for k := range m.Headers {
		if _, ok := emitted[k]; ok {
			continue
		}
		found := false
		for _, rk := range restKeys {
			if rk == k {
				found = true
				break
			}
		}
		if !found {
			restKeys = append(restKeys, k)
		}
	}
	sort.Strings(restKeys)
	for _, k := range restKeys {
		vals := m.HeadersMulti[k]
		if len(vals) == 0 {
			if v, ok := m.Headers[k]; ok && v != "" {
				vals = []string{v}
			}
		}
		for _, v := range vals {
			b.WriteString(fmt.Sprintf("%s: %s\r\n", prettyHeaderName(k), v))
		}
	}

	b.WriteString("\r\n")
	if m.Body != "" {
		b.WriteString(normalizeCRLF(m.Body))
	}

	return b.String()
}

func normalizeCRLF(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// BodyBytesLen returns the byte length of the body after CRLF normalization.
func BodyBytesLen(body string) int {
	return len([]byte(normalizeCRLF(body)))
}

func prettyHeaderName(canonical string) string {
	switch strings.ToLower(strings.TrimSpace(canonical)) {
	case canonicalHeaderKey(HeaderVia):
		return HeaderVia
	case canonicalHeaderKey(HeaderMaxForwards):
		return HeaderMaxForwards
	case canonicalHeaderKey(HeaderFrom):
		return HeaderFrom
	case canonicalHeaderKey(HeaderTo):
		return HeaderTo
	case canonicalHeaderKey(HeaderCallID):
		return HeaderCallID
	case canonicalHeaderKey(HeaderCSeq):
		return HeaderCSeq
	case canonicalHeaderKey(HeaderContact):
		return HeaderContact
	case canonicalHeaderKey(HeaderAllow):
		return HeaderAllow
	case canonicalHeaderKey(HeaderSupported):
		return HeaderSupported
	case canonicalHeaderKey(HeaderUserAgent):
		return HeaderUserAgent
	case canonicalHeaderKey(HeaderContentType):
		return HeaderContentType
	case canonicalHeaderKey(HeaderContentLength):
		return HeaderContentLength
	default:
		parts := strings.Split(canonical, "-")
		for i := range parts {
			if parts[i] == "" {
				continue
			}
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
		return strings.Join(parts, "-")
	}
}

// GetHeader returns the first header value for name (case-insensitive).
func (m *Message) GetHeader(name string) string {
	if m == nil {
		return ""
	}
	return m.Headers[canonicalHeaderKey(name)]
}

// GetHeaders returns all values for a header name (case-insensitive).
func (m *Message) GetHeaders(name string) []string {
	if m == nil {
		return nil
	}
	return m.HeadersMulti[canonicalHeaderKey(name)]
}

// SetHeader replaces a header with a single value.
func (m *Message) SetHeader(name, value string) {
	if m == nil {
		return
	}
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	if m.HeadersMulti == nil {
		m.HeadersMulti = make(map[string][]string)
	}
	ck := canonicalHeaderKey(name)
	m.Headers[ck] = value
	m.HeadersMulti[ck] = []string{value}
}

// AddHeader appends a header value (multi-value headers such as Via).
func (m *Message) AddHeader(name, value string) {
	if m == nil {
		return
	}
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	if m.HeadersMulti == nil {
		m.HeadersMulti = make(map[string][]string)
	}
	ck := canonicalHeaderKey(name)
	if _, exists := m.Headers[ck]; !exists {
		m.Headers[ck] = value
	}
	m.HeadersMulti[ck] = append(m.HeadersMulti[ck], value)
}

// ReadMessage reads one SIP message from r using CRLF framing (Content-Length when body present).
func ReadMessage(r *bufio.Reader) (*Message, error) {
	var hdrLines []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		hdrLines = append(hdrLines, line)
	}
	if len(hdrLines) == 0 {
		return nil, ErrEmptyMessageHeaders
	}
	raw := strings.Join(hdrLines, "\n") + "\n\n"
	cl := 0
	for _, ln := range hdrLines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ln)), "content-length:") {
			parts := strings.SplitN(ln, ":", 2)
			if len(parts) == 2 {
				cl, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
			break
		}
	}
	if cl > 0 {
		body := make([]byte, cl)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}
		raw += string(body)
	}
	return Parse(raw)
}

// ParseRAck parses RAck header value (RFC 3262): "<rseq> <cseq-num> <method>"
func ParseRAck(h string) (rseq uint32, cseqNum int, method string, err error) {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0, 0, "", ErrEmptyRAck
	}
	parts := strings.Fields(h)
	if len(parts) < 3 {
		return 0, 0, "", ErrRAckNeedsFields
	}
	rs, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, 0, "", errRAckRSeq(err)
	}
	cs, err := strconv.Atoi(parts[1])
	if err != nil || cs < 0 {
		return 0, 0, "", ErrRAckCSeq
	}
	return uint32(rs), cs, strings.ToUpper(strings.TrimSpace(parts[2])), nil
}

// NewResponse builds an empty SIP response with VersionSIP20 and initialized header maps.
func NewResponse(code int, reason string) *Message {
	return &Message{
		IsRequest:    false,
		Version:      VersionSIP20,
		StatusCode:   code,
		StatusText:   reason,
		Headers:      make(map[string]string),
		HeadersMulti: make(map[string][]string),
	}
}

// ResponseStatusLine returns the SIP status line prefix, e.g. "SIP/2.0 200 OK".
func ResponseStatusLine(code int, reason string) string {
	return VersionSIP20 + " " + strconv.Itoa(code) + " " + reason
}
