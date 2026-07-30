package sdp

import (
	"strings"
	"testing"
)

func TestSelectAnswerCodecs_Intersect(t *testing.T) {
	offer := &Info{Codecs: []Codec{
		{PayloadType: 111, Name: CodecOpus, ClockRate: 48000},
		{PayloadType: 0, Name: CodecPCMU, ClockRate: 8000},
		{PayloadType: 8, Name: CodecPCMA, ClockRate: 8000},
		{PayloadType: 101, Name: CodecTelephoneEvent, ClockRate: 8000},
	}}
	got := SelectAnswerCodecs(offer)
	if len(got) < 3 {
		t.Fatalf("codecs=%v", got)
	}
	// Preference: PCMA before PCMU before Opus
	if !strings.EqualFold(got[0].Name, CodecPCMA) {
		t.Fatalf("want PCMA first, got %#v", got[0])
	}
	if !strings.EqualFold(got[len(got)-1].Name, CodecTelephoneEvent) {
		t.Fatalf("want telephone-event last, got %#v", got[len(got)-1])
	}
}

func TestSelectAnswerCodecs_EmptyOffer(t *testing.T) {
	got := SelectAnswerCodecs(nil)
	if len(got) != 2 || !strings.EqualFold(got[0].Name, CodecPCMA) {
		t.Fatalf("%#v", got)
	}
}

func TestBuildAudioAnswer_RoundTrip(t *testing.T) {
	offerBody := Generate("203.0.113.10", 4000, []Codec{
		{PayloadType: 111, Name: CodecOpus, ClockRate: 48000},
		{PayloadType: 0, Name: CodecPCMU, ClockRate: 8000},
		{PayloadType: 101, Name: CodecTelephoneEvent, ClockRate: 8000},
	})
	offer, err := Parse(offerBody)
	if err != nil {
		t.Fatal(err)
	}
	ans := BuildAudioAnswer("198.51.100.1", 10000, offer)
	info, err := Parse(ans)
	if err != nil {
		t.Fatal(err)
	}
	if info.IP != "198.51.100.1" || info.Port != 10000 {
		t.Fatalf("ip/port=%s:%d", info.IP, info.Port)
	}
	names := map[string]bool{}
	for _, c := range info.Codecs {
		names[strings.ToLower(c.Name)] = true
	}
	if !names[CodecPCMU] || !names[CodecTelephoneEvent] {
		t.Fatalf("codecs=%v", info.Codecs)
	}
}
