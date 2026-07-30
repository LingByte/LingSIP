package signaling_test

import (
	"testing"

	"github.com/LingByte/lingsip/signaling"
	"github.com/LingByte/lingsip/stack"
)

func TestParseRFC3326Reason(t *testing.T) {
	proto, cause, text := signaling.ParseRFC3326Reason(`Q.850;cause=16;text="Normal call clearing"`)
	if proto != "Q.850" || cause != 16 || text != "Normal call clearing" {
		t.Fatalf("proto=%q cause=%d text=%q", proto, cause, text)
	}
}

func TestClassifyBYEReasonNil(t *testing.T) {
	cls, raw := signaling.ClassifyBYEReason(nil)
	if cls != signaling.ByeReasonNormal || raw != "" {
		t.Fatalf("cls=%q raw=%q", cls, raw)
	}
}

func reasonMsg(reason string) *stack.Message {
	msg := &stack.Message{IsRequest: true, Method: stack.MethodBye}
	msg.SetHeader("Reason", reason)
	return msg
}

func TestClassifyBYEReasonQ850Normal(t *testing.T) {
	cls, raw := signaling.ClassifyBYEReason(reasonMsg(`Q.850;cause=16;text="cleared"`))
	if cls != signaling.ByeReasonNormal || raw != "cleared" {
		t.Fatalf("cls=%q raw=%q", cls, raw)
	}
}

func TestClassifyBYEReasonSIP408(t *testing.T) {
	cls, _ := signaling.ClassifyBYEReason(reasonMsg(`SIP;cause=408;text="Timeout"`))
	if cls != signaling.ByeReasonTimerExpired {
		t.Fatalf("cls=%q", cls)
	}
}

func TestClassifyBYEReasonSIPError(t *testing.T) {
	cls, _ := signaling.ClassifyBYEReason(reasonMsg(`SIP;cause=503;text="Unavailable"`))
	if cls != signaling.ByeReasonError {
		t.Fatalf("cls=%q", cls)
	}
}

func TestClassifyBYEReasonUserHangup(t *testing.T) {
	cls, _ := signaling.ClassifyBYEReason(reasonMsg(`Q.850;cause=21;text="User busy"`))
	if cls != signaling.ByeReasonUserHangup {
		t.Fatalf("cls=%q", cls)
	}
}
