package main

// Digest challenge / verify — offline demo of lingsip/auth (no network).
//
//	cd lingsip && go run ./examples/digest-challenge

import (
	"fmt"
	"strings"

	"github.com/LingByte/lingsip/auth"
	"github.com/LingByte/lingsip/stack"
)

func main() {
	da := auth.NewDigestAuth("example.com", "alice", "secret")

	req, err := stack.Parse(strings.Join([]string{
		"REGISTER sip:example.com SIP/2.0",
		"Via: SIP/2.0/UDP 127.0.0.1:5060;branch=z9hG4bKdemo",
		"From: <sip:alice@example.com>;tag=from1",
		"To: <sip:alice@example.com>",
		"Call-ID: digest-demo",
		"CSeq: 1 REGISTER",
		"Contact: <sip:alice@127.0.0.1:5060>",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n"))
	if err != nil {
		panic(err)
	}

	fmt.Printf("unauthenticated: VerifyRequest=%v\n", da.VerifyRequest(req))

	chal, err := da.Challenge401(req)
	if err != nil {
		panic(err)
	}
	www := chal.GetHeader("WWW-Authenticate")
	fmt.Printf("401 WWW-Authenticate: %s\n", www)

	parsed := auth.ParseDigestAuth(www)
	uri := "sip:example.com"
	ha1 := auth.DigestHA1("alice", "example.com", "secret")
	nc, cnonce := "00000001", "0a4f113b"
	response := auth.DigestExpectResponse(map[string]string{
		"nonce":  parsed["nonce"],
		"qop":    "auth",
		"nc":     nc,
		"cnonce": cnonce,
	}, "REGISTER", uri, ha1)
	authz := fmt.Sprintf(
		`Digest username="alice", realm="example.com", nonce="%s", uri="%s", response="%s", algorithm=MD5, qop=auth, nc=%s, cnonce="%s"`,
		parsed["nonce"], uri, response, nc, cnonce,
	)
	req.SetHeader("Authorization", authz)
	fmt.Printf("Authorization: %s\n", authz)
	fmt.Printf("after Authorization: VerifyRequest=%v\n", da.VerifyRequest(req))
	fmt.Printf("replay (nonce single-use): VerifyRequest=%v\n", da.VerifyRequest(req))
}
