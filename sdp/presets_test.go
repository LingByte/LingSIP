package sdp

import "testing"

func TestDefaultOutboundOfferCodecs(t *testing.T) {
	codecs := DefaultOutboundOfferCodecs()
	if len(codecs) < 4 {
		t.Fatalf("len=%d", len(codecs))
	}
	if codecs[0].Name != "pcma" {
		t.Fatalf("first=%q", codecs[0].Name)
	}
	last := codecs[len(codecs)-1]
	if last.Name != "telephone-event" {
		t.Fatalf("last=%q", last.Name)
	}
}

func TestTransferAgentBridgeOfferCodecs(t *testing.T) {
	codecs := TransferAgentBridgeOfferCodecs()
	if len(codecs) != 3 {
		t.Fatalf("len=%d", len(codecs))
	}
	for _, c := range codecs {
		if c.ClockRate != 8000 {
			t.Fatalf("clock %q", c.Name)
		}
	}
}
