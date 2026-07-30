package historyinfo_test

import (
	"testing"

	"github.com/LingByte/lingsip/historyinfo"
)

func TestBuildRetargetHeadersBasic(t *testing.T) {
	hi, dv := historyinfo.BuildRetargetHeaders(
		"<sip:alice@example.com>",
		"",
		"",
		"sip:bob@example.com",
		"SIP;cause=302",
		"unconditional",
	)
	if len(hi) != 2 {
		t.Fatalf("history len=%d", len(hi))
	}
	if hi[0].URI != "sip:alice@example.com" || hi[1].URI != "sip:bob@example.com" {
		t.Fatalf("history=%+v", hi)
	}
	if len(dv) != 1 || dv[0].URI != "sip:alice@example.com" {
		t.Fatalf("diversion=%+v", dv)
	}
}

func TestBuildRetargetHeadersEmptyTarget(t *testing.T) {
	hi, dv := historyinfo.BuildRetargetHeaders("<sip:a@b>", "", "", "", "", "")
	if hi != nil || dv != nil {
		t.Fatalf("hi=%v dv=%v", hi, dv)
	}
}

func TestBuildRetargetHeadersExtendsExistingChain(t *testing.T) {
	inbound := []historyinfo.Entry{{URI: "sip:orig@x", Index: "1"}}
	hi, dv := historyinfo.BuildRetargetHeaders(
		"<sip:orig@x>",
		historyinfo.FormatChain(inbound),
		"",
		"sip:target@y",
		"SIP;cause=302",
		"deflection",
	)
	if len(hi) != 2 || hi[1].URI != "sip:target@y" {
		t.Fatalf("history=%+v", hi)
	}
	if len(dv) != 1 {
		t.Fatalf("diversion=%+v", dv)
	}
}

func TestBuildRetargetHeadersNoOriginalNoChains(t *testing.T) {
	hi, dv := historyinfo.BuildRetargetHeaders("", "", "", "sip:only@x", "", "")
	if hi != nil || dv != nil {
		t.Fatalf("hi=%v dv=%v", hi, dv)
	}
}
