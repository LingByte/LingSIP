package auth

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LingByte/lingsip/stack"
)

func TestDigestAuthChallengeAndVerify(t *testing.T) {
	d := NewDigestAuth("testrealm", "alice", "secret")
	if d == nil {
		t.Fatal("digest auth nil")
	}
	req, err := stack.Parse("INVITE sip:bob@example.com SIP/2.0\r\nCall-ID: d1\r\nContent-Length: 0\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := d.Challenge401(req)
	if err != nil {
		t.Fatal(err)
	}
	www := resp.GetHeader("WWW-Authenticate")
	if !strings.Contains(www, "nonce=") {
		t.Fatalf("www=%s", www)
	}
	nonce := parseDigestParam(www, "nonce")
	uri := "sip:bob@example.com"
	ha1 := DigestHA1("alice", "testrealm", "secret")
	auth := map[string]string{
		"username": "alice",
		"realm":    "testrealm",
		"nonce":    nonce,
		"uri":      uri,
		"qop":      "auth",
		"nc":       "00000001",
		"cnonce":   "abc",
		"response": DigestExpectResponse(map[string]string{
			"nonce":  nonce,
			"qop":    "auth",
			"nc":     "00000001",
			"cnonce": "abc",
		}, "INVITE", uri, ha1),
	}
	req.SetHeader("Authorization", formatDigestAuth(auth))
	if !d.VerifyRequest(req) {
		t.Fatal("verify failed")
	}
	if d.VerifyRequest(req) {
		t.Fatal("nonce single use")
	}
}

func TestDigestAuthNilAndInvalid(t *testing.T) {
	if NewDigestAuth("", "u", "p") != nil {
		t.Fatal("empty realm")
	}
	var d *DigestAuth
	if d.VerifyRequest(nil) {
		t.Fatal("nil verify")
	}
	if d.Realm() != "" {
		t.Fatal("nil realm")
	}
	if _, err := d.Challenge401(nil); err == nil {
		t.Fatal("nil challenge")
	}
	req, _ := stack.Parse("INVITE sip:x SIP/2.0\r\nContent-Length: 0\r\n\r\n")
	if d.VerifyRequest(req) {
		t.Fatal("nil digest verify")
	}
	req.SetHeader("Authorization", "Basic xyz")
	d = NewDigestAuth("r", "u", "p")
	if d.VerifyRequest(req) {
		t.Fatal("basic auth rejected")
	}
	if d.Realm() != "r" {
		t.Fatalf("realm=%q", d.Realm())
	}
	d.GC()
}

func TestParseDigestAuthAndMD5(t *testing.T) {
	m := ParseDigestAuth(`Digest username="u", realm="r", nonce="n"`)
	if m["username"] != "u" || m["realm"] != "r" {
		t.Fatalf("parse=%v", m)
	}
	if MD5Hex("test") == "" {
		t.Fatal("MD5Hex")
	}
	ha1 := DigestHA1("u", "r", "p")
	if ha1 == "" {
		t.Fatal("ha1")
	}
	expect := DigestExpectResponse(map[string]string{"nonce": "n"}, "INVITE", "sip:x", ha1)
	if expect == "" {
		t.Fatal("expect no qop")
	}
}

func parseDigestParam(www, key string) string {
	m := ParseDigestAuth(www)
	return m[strings.ToLower(key)]
}

func formatDigestAuth(m map[string]string) string {
	return fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", "+
		"response=\"%s\", algorithm=MD5, qop=auth, nc=%s, cnonce=\"%s\"",
		m["username"], m["realm"], m["nonce"], m["uri"], m["response"], m["nc"], m["cnonce"])
}
