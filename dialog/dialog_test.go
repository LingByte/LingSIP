package dialog

import (
	"strings"
	"testing"

	"github.com/LingByte/lingsip/stack"
)

func TestTagFromHeader(t *testing.T) {
	if g := TagFromHeader(`<sip:a@b>;tag=abc7`); g != "abc7" {
		t.Fatalf("got %q", g)
	}
}

func TestNewUASFromINVITE_AndMatchACK(t *testing.T) {
	raw := strings.Join([]string{
		"INVITE sip:x@y SIP/2.0",
		"Via: SIP/2.0/UDP 10.0.0.1;branch=z9hG4bKd1",
		"From: <sip:a@b>;tag=rem1",
		"To: <sip:x@y>",
		"Call-ID: cid-d",
		"CSeq: 3 INVITE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	inv, err := stack.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewUASFromINVITE(inv)
	if err != nil {
		t.Fatal(err)
	}
	if d.InviteTransactionKey() != stack.InviteTransactionKey(stack.TopBranch(inv), "cid-d") {
		t.Fatal("tx key mismatch")
	}
	d.SetLocalTag("loc1")
	ackRaw := strings.Join([]string{
		"ACK sip:x@y SIP/2.0",
		"Via: SIP/2.0/UDP 10.0.0.1;branch=z9hG4bKd1",
		"From: <sip:a@b>;tag=rem1",
		"To: <sip:x@y>;tag=loc1",
		"Call-ID: cid-d",
		"CSeq: 3 ACK",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	ack, err := stack.Parse(ackRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !d.MatchACK(ack) {
		t.Fatal("expected ACK match")
	}
}

func TestTagFromHeader_EdgeCases(t *testing.T) {
	if TagFromHeader("") != "" {
		t.Fatal("empty")
	}
	if TagFromHeader(`<sip:a@b>;TAG=AbC`) != "AbC" {
		t.Fatal("case insensitive key")
	}
	if TagFromHeader(`<sip:a@b>;tag="quoted"`) != "quoted" {
		t.Fatal("quoted tag")
	}
	if TagFromHeader(`<sip:a@b>;tag=abc;user=phone`) != "abc" {
		t.Fatal("stop at semicolon")
	}
}

func TestAppendTagAfterNameAddr_EdgeCases(t *testing.T) {
	if AppendTagAfterNameAddr("", "t") != "" {
		t.Fatal("empty header")
	}
	if AppendTagAfterNameAddr("hdr", "") != "hdr" {
		t.Fatal("empty tag")
	}
	plain := "sip:user@host"
	got := AppendTagAfterNameAddr(plain, "t1")
	if TagFromHeader(got) != "t1" {
		t.Fatalf("plain uri: %q", got)
	}
}

func TestDialog_StateAndNil(t *testing.T) {
	var d *Dialog
	if d.State() != StateNone {
		t.Fatal("nil state")
	}
	d.Confirm()
	d.Terminate()
	d.SetLocalTag("x")
	d.SetLocalTagFromToHeader("")
	if d.GetLocalTag() != "" || d.GetRemoteTag() != "" || d.InviteCSeqNum() != 0 {
		t.Fatal("nil getters")
	}
	if d.InviteTransactionKey() != "" {
		t.Fatal("nil tx key")
	}
	if d.MatchACK(nil) {
		t.Fatal("nil ack")
	}
}

func TestNewUASFromINVITE_Errors(t *testing.T) {
	if _, err := NewUASFromINVITE(nil); err == nil {
		t.Fatal("nil inv")
	}
	bye := &stack.Message{IsRequest: true, Method: stack.MethodBye}
	if _, err := NewUASFromINVITE(bye); err == nil {
		t.Fatal("not invite")
	}
}

func TestDialog_ConfirmAfterTerminate(t *testing.T) {
	raw := strings.Join([]string{
		"INVITE sip:x@y SIP/2.0",
		"Via: SIP/2.0/UDP 10.0.0.1;branch=z9hG4bKd1",
		"From: <sip:a@b>;tag=rem1",
		"To: <sip:x@y>",
		"Call-ID: cid-term",
		"CSeq: 1 INVITE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	inv, _ := stack.Parse(raw)
	d, err := NewUASFromINVITE(inv)
	if err != nil {
		t.Fatal(err)
	}
	if d.State() != StateEarly {
		t.Fatal("early")
	}
	d.Terminate()
	if d.State() != StateTerminated {
		t.Fatal("terminated")
	}
	d.Confirm()
	if d.State() != StateTerminated {
		t.Fatal("confirm after terminate is no-op")
	}
	d = &Dialog{state: StateEarly}
	d.Confirm()
	if d.State() != StateConfirmed {
		t.Fatal("confirmed")
	}
}

func TestMatchACK_TagMismatch(t *testing.T) {
	raw := strings.Join([]string{
		"INVITE sip:x@y SIP/2.0",
		"Via: SIP/2.0/UDP 10.0.0.1;branch=z9hG4bKd1",
		"From: <sip:a@b>;tag=rem1",
		"To: <sip:x@y>",
		"Call-ID: cid-m",
		"CSeq: 1 INVITE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	inv, _ := stack.Parse(raw)
	d, _ := NewUASFromINVITE(inv)
	d.SetLocalTag("loc1")
	badAck := strings.Join([]string{
		"ACK sip:x@y SIP/2.0",
		"From: <sip:a@b>;tag=wrong",
		"To: <sip:x@y>;tag=loc1",
		"Call-ID: cid-m",
		"CSeq: 1 ACK",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	ack, _ := stack.Parse(badAck)
	if d.MatchACK(ack) {
		t.Fatal("tag mismatch should fail")
	}
}
