package session_timer_test

import (
	"strings"
	"testing"

	"github.com/LingByte/lingsip/session_timer"
	"github.com/LingByte/lingsip/stack"
)

func TestNegotiateUASFromINVITE(t *testing.T) {
	msg := &stack.Message{IsRequest: true, Method: stack.MethodInvite}
	msg.SetHeader("Session-Expires", "1800;refresher=uac")
	msg.SetHeader("Min-SE", "900")
	msg.SetHeader("Supported", "timer")

	dec := session_timer.NegotiateUASFromINVITE(msg)
	if !dec.IsActive() || dec.ChosenSE != 1800 {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestNegotiateUASFromINVITENil(t *testing.T) {
	dec := session_timer.NegotiateUASFromINVITE(nil)
	if dec.IsActive() {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestMergeSupportedToken(t *testing.T) {
	msg := &stack.Message{}
	msg.SetHeader("Supported", "100rel")
	session_timer.MergeSupportedToken(msg, session_timer.SupportedTokenTimer)
	got := msg.GetHeader("Supported")
	if !strings.Contains(got, "timer") || !strings.Contains(got, "100rel") {
		t.Fatalf("supported=%q", got)
	}
}

func TestMergeSupportedTokenNoDuplicate(t *testing.T) {
	msg := &stack.Message{}
	msg.SetHeader("Supported", "timer")
	session_timer.MergeSupportedToken(msg, session_timer.SupportedTokenTimer)
	if strings.Count(strings.ToLower(msg.GetHeader("Supported")), "timer") != 1 {
		t.Fatalf("supported=%q", msg.GetHeader("Supported"))
	}
}

func TestMergeSupportedTokenNilMsg(t *testing.T) {
	session_timer.MergeSupportedToken(nil, session_timer.SupportedTokenTimer)
}
