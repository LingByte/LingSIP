package main

// Minimal UAS example: listen on UDP (+ optional TCP), answer REGISTER / INVITE,
// handle OPTIONS / ACK / BYE / CANCEL with RFC 3261 transactions.
//
//	cd lingsip && go run ./examples/uas-server -listen 0.0.0.0:5060
//
// Optional digest:
//
//	go run ./examples/uas-server -listen :5060 -realm example.com -user alice -pass secret
//
// Optional TCP:
//
//	go run ./examples/uas-server -listen :5060 -tcp :5060

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LingByte/lingsip/auth"
	"github.com/LingByte/lingsip/dialog"
	"github.com/LingByte/lingsip/dtmf"
	"github.com/LingByte/lingsip/sdp"
	"github.com/LingByte/lingsip/signaling"
	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/transaction"
	"github.com/LingByte/lingsip/uas"
)

func main() {
	listen := flag.String("listen", "0.0.0.0:5060", "UDP listen host:port")
	tcpAddr := flag.String("tcp", "", "optional TCP listen address (e.g. :5060)")
	publicIP := flag.String("public-ip", "", "IP advertised in Contact / SDP (default: UDP bind host)")
	rtpPort := flag.Int("rtp-port", 10000, "UDP port advertised in answer SDP (media not served)")
	realm := flag.String("realm", "", "digest realm (empty = no auth)")
	user := flag.String("user", "", "digest username")
	pass := flag.String("pass", "", "digest password")
	flag.Parse()

	host, portStr, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		log.Fatalf("invalid port in -listen")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		if strings.TrimSpace(*publicIP) == "" {
			*publicIP = "127.0.0.1"
		}
	} else if strings.TrimSpace(*publicIP) == "" {
		*publicIP = host
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := &server{
		publicIP: *publicIP,
		sipPort:  port,
		rtpPort:  *rtpPort,
		digest:   auth.NewDigestAuth(*realm, *user, *pass),
		dialogs:  make(map[string]*dialog.Dialog),
		bindings: make(map[string]string),
	}

	ep := stack.NewEndpoint(stack.EndpointConfig{
		Host: host,
		Port: port,
		OnRequest: func(req *stack.Message, addr *net.UDPAddr) {
			log.Printf("← %s %s from %s Call-ID=%s", req.Method, req.RequestURI, addr, req.GetHeader("Call-ID"))
		},
		OnResponseSent: func(req, resp *stack.Message, addr *net.UDPAddr) {
			log.Printf("→ %d to %s Call-ID=%s", resp.StatusCode, addr, req.GetHeader("Call-ID"))
		},
		OnParseErr: func(raw []byte, addr *net.UDPAddr, err error) {
			log.Printf("parse error from %s: %v", addr, err)
		},
	})
	if err := ep.Open(); err != nil {
		log.Fatalf("open udp: %v", err)
	}
	defer func() { _ = ep.Close() }()

	mgr := transaction.NewManager()
	send := func(msg *stack.Message, addr *net.UDPAddr) error { return ep.Send(msg, addr) }
	h := uas.Handlers{
		Invite:   srv.onInvite,
		Ack:      srv.onAck,
		Bye:      srv.onBye,
		Cancel:   srv.onCancel,
		Info:     srv.onInfo,
		Register: srv.onRegister,
		Message:  srv.onMessage,
		Options:  nil, // default Allow list
	}
	if err := h.AttachWithTransaction(ep, uas.TransactionBinding{
		Mgr:  mgr,
		Send: send,
		Ctx:  ctx,
	}); err != nil {
		log.Fatalf("attach handlers: %v", err)
	}

	go func() {
		log.Printf("lingsip UAS listening udp/%s (Contact/SDP IP=%s)", *listen, *publicIP)
		if err := ep.Serve(ctx); err != nil && ctx.Err() == nil {
			log.Printf("udp serve: %v", err)
			cancel()
		}
	}()

	if ta := strings.TrimSpace(*tcpAddr); ta != "" {
		go func() {
			log.Printf("lingsip UAS listening tcp/%s", ta)
			if err := stack.ListenAndServeTCP(ctx, ta, ep); err != nil && ctx.Err() == nil {
				log.Printf("tcp serve: %v", err)
			}
		}()
	}

	<-ctx.Done()
	log.Printf("shutting down")
}

type server struct {
	publicIP string
	sipPort  int
	rtpPort  int
	digest   *auth.DigestAuth

	mu       sync.Mutex
	dialogs  map[string]*dialog.Dialog
	bindings map[string]string // AOR (To URI) → Contact
}

func (s *server) onRegister(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
	if s.digest != nil && !s.digest.VerifyRequest(req) {
		return s.digest.Challenge401(req)
	}

	contact := strings.TrimSpace(req.GetHeader(stack.HeaderContact))
	expires := strings.TrimSpace(req.GetHeader(stack.HeaderExpires))
	if expires == "" {
		expires = "3600"
	}
	// Contact: * ;expires=0 → unregister all (simplified: clear this AOR)
	aor := extractURI(req.GetHeader(stack.HeaderTo))
	if aor == "" {
		aor = extractURI(req.GetHeader(stack.HeaderFrom))
	}

	s.mu.Lock()
	if contact == "*" || strings.HasPrefix(contact, "*") || expires == "0" {
		delete(s.bindings, aor)
		log.Printf("REGISTER unbound AOR=%s from %s", aor, addr)
	} else if contact != "" {
		s.bindings[aor] = contact
		log.Printf("REGISTER bound AOR=%s → %s expires=%s from %s", aor, contact, expires, addr)
	}
	n := len(s.bindings)
	s.mu.Unlock()

	to := dialog.AppendTagAfterNameAddr(req.GetHeader(stack.HeaderTo), newTag())
	resp, err := uas.NewResponseWithTo(req, stack.StatusOK, stack.ReasonOK, "", "", to)
	if err != nil {
		return nil, err
	}
	if contact != "" && contact != "*" && !strings.HasPrefix(contact, "*") {
		resp.SetHeader(stack.HeaderContact, contact)
	}
	resp.SetHeader(stack.HeaderExpires, expires)
	resp.SetHeader(stack.HeaderAllow, "INVITE, ACK, BYE, CANCEL, OPTIONS, REGISTER, INFO, MESSAGE")
	log.Printf("REGISTER ok AOR=%s bindings=%d", aor, n)
	return resp, nil
}

func (s *server) onInvite(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
	if s.digest != nil && !s.digest.VerifyRequest(req) {
		return s.digest.Challenge401(req)
	}

	offer, err := sdp.Parse(req.Body)
	if err != nil {
		log.Printf("INVITE without usable SDP (%v); answering with fallback codecs", err)
		offer = nil
	} else {
		log.Printf("offer from %s:%d codecs=%v", offer.IP, offer.Port, codecNames(offer.Codecs))
	}

	d, err := dialog.NewUASFromINVITE(req)
	if err != nil {
		return uas.ErrorResponse(req, stack.StatusBadRequest, stack.ReasonBadRequest)
	}
	localTag := newTag()
	to := dialog.AppendTagAfterNameAddr(req.GetHeader(stack.HeaderTo), localTag)
	d.SetLocalTag(localTag)

	body := sdp.BuildAudioAnswer(s.publicIP, s.rtpPort, offer)
	log.Printf("answer codecs=%v", codecNames(sdp.SelectAnswerCodecs(offer)))
	resp, err := uas.NewResponseWithTo(req, stack.StatusOK, stack.ReasonOK, body, stack.ContentTypeSDP, to)
	if err != nil {
		return nil, err
	}
	resp.SetHeader(stack.HeaderContact, fmt.Sprintf("<sip:uas@%s:%d>", s.publicIP, s.sipPort))
	resp.SetHeader(stack.HeaderAllow, "INVITE, ACK, BYE, CANCEL, OPTIONS, REGISTER, INFO, MESSAGE")

	s.mu.Lock()
	s.dialogs[d.CallID] = d
	s.mu.Unlock()
	return resp, nil
}

func (s *server) onAck(req *stack.Message, _ *net.UDPAddr) error {
	callID := strings.TrimSpace(req.GetHeader(stack.HeaderCallID))
	s.mu.Lock()
	d := s.dialogs[callID]
	s.mu.Unlock()
	if d != nil && d.MatchACK(req) {
		d.Confirm()
		log.Printf("dialog confirmed Call-ID=%s", callID)
	}
	return nil
}

func (s *server) onBye(req *stack.Message, _ *net.UDPAddr) (*stack.Message, error) {
	cls, text := signaling.ClassifyBYEReason(req)
	callID := strings.TrimSpace(req.GetHeader(stack.HeaderCallID))
	log.Printf("BYE Call-ID=%s class=%s text=%q", callID, cls, text)
	s.mu.Lock()
	delete(s.dialogs, callID)
	s.mu.Unlock()
	return uas.NewResponse(req, stack.StatusOK, stack.ReasonOK, "", "")
}

func (s *server) onCancel(req *stack.Message, _ *net.UDPAddr) (*stack.Message, error) {
	callID := strings.TrimSpace(req.GetHeader(stack.HeaderCallID))
	log.Printf("CANCEL Call-ID=%s", callID)
	s.mu.Lock()
	delete(s.dialogs, callID)
	s.mu.Unlock()
	return uas.NewResponse(req, stack.StatusOK, stack.ReasonOK, "", "")
}

func (s *server) onInfo(req *stack.Message, _ *net.UDPAddr) (*stack.Message, error) {
	if digit, ok := dtmf.DigitFromSIPINFO(req.GetHeader(stack.HeaderContentType), req.Body); ok {
		log.Printf("DTMF via INFO: %s Call-ID=%s", digit, req.GetHeader(stack.HeaderCallID))
	}
	return uas.NewResponse(req, stack.StatusOK, stack.ReasonOK, "", "")
}

func (s *server) onMessage(req *stack.Message, addr *net.UDPAddr) (*stack.Message, error) {
	if s.digest != nil && !s.digest.VerifyRequest(req) {
		return s.digest.Challenge401(req)
	}
	ct := req.GetHeader(stack.HeaderContentType)
	body := strings.TrimSpace(req.Body)
	if len(body) > 200 {
		body = body[:200] + "…"
	}
	log.Printf("MESSAGE from %s ct=%s body=%q", addr, ct, body)
	return uas.NewResponse(req, stack.StatusOK, stack.ReasonOK, "", "")
}

func codecNames(cs []sdp.Codec) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func newTag() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func extractURI(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if i := strings.IndexByte(header, '<'); i >= 0 {
		if j := strings.IndexByte(header[i:], '>'); j > 0 {
			return strings.TrimSpace(header[i+1 : i+j])
		}
	}
	if i := strings.IndexByte(header, ';'); i >= 0 {
		header = header[:i]
	}
	return strings.TrimSpace(header)
}
