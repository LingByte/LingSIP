package main

// UAC MESSAGE — send a short IM to the example UAS (RFC 3428).
//
//	# terminal 1
//	go run ./examples/uas-server -listen 127.0.0.1:6050
//	# terminal 2
//	go run ./examples/uac-message -target 127.0.0.1:6050 -body "hello from lingsip"

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/transaction"
)

func main() {
	target := flag.String("target", "127.0.0.1:5060", "remote host:port")
	fromUser := flag.String("from", "uac", "From user")
	toUser := flag.String("to", "uas", "To user")
	body := flag.String("body", "hello", "message body")
	contentType := flag.String("ct", "text/plain", "Content-Type")
	timeout := flag.Duration("timeout", 5*time.Second, "wait for final response")
	flag.Parse()

	host, portStr, err := net.SplitHostPort(*target)
	if err != nil {
		log.Fatalf("target: %v", err)
	}
	port, _ := strconv.Atoi(portStr)
	remote := &net.UDPAddr{IP: net.ParseIP(host), Port: port}
	if remote.IP == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			log.Fatalf("resolve %s: %v", host, err)
		}
		remote.IP = ips[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	mgr := transaction.NewManager()
	ep := stack.NewEndpoint(stack.EndpointConfig{
		Host: "0.0.0.0",
		Port: 0,
		OnSIPResponse: func(resp *stack.Message, addr *net.UDPAddr) {
			_ = mgr.HandleResponse(resp, addr)
		},
	})
	if err := ep.Open(); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = ep.Close() }()

	localHost, localPort := "127.0.0.1", 0
	if ua, ok := ep.ListenAddr().(*net.UDPAddr); ok && ua != nil {
		if ua.IP != nil && !ua.IP.IsUnspecified() {
			localHost = ua.IP.String()
		}
		localPort = ua.Port
	}

	branch := "z9hG4bK" + randHex(8)
	callID := randHex(12)
	fromTag := randHex(6)
	reqURI := fmt.Sprintf("sip:%s@%s:%d", *toUser, host, port)
	payload := *body

	raw := strings.Join([]string{
		fmt.Sprintf("MESSAGE %s SIP/2.0", reqURI),
		fmt.Sprintf("Via: SIP/2.0/UDP %s:%d;branch=%s;rport", localHost, localPort, branch),
		fmt.Sprintf("From: <sip:%s@%s>;tag=%s", *fromUser, localHost, fromTag),
		fmt.Sprintf("To: <sip:%s@%s>", *toUser, host),
		fmt.Sprintf("Call-ID: %s", callID),
		"CSeq: 1 MESSAGE",
		"Max-Forwards: 70",
		fmt.Sprintf("Content-Type: %s", *contentType),
		fmt.Sprintf("Content-Length: %d", len(payload)),
		"",
		payload,
	}, "\r\n")
	req, err := stack.Parse(raw)
	if err != nil {
		log.Fatal(err)
	}

	send := func(msg *stack.Message, addr *net.UDPAddr) error {
		log.Printf("→ MESSAGE to %s", addr)
		return ep.Send(msg, addr)
	}

	go func() { _ = ep.Serve(ctx) }()

	res, err := mgr.RunNonInviteClient(ctx, req, remote, send)
	if err != nil {
		log.Printf("MESSAGE failed: %v", err)
		os.Exit(1)
	}
	fmt.Printf("MESSAGE → %d %s\n", res.Final.StatusCode, res.Final.StatusText)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
