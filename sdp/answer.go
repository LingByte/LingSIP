package sdp

import "strings"

// DefaultAnswerCodecPreference is the UAS answer preference order when intersecting an offer.
// Narrowband first for carrier / softphone interoperability; Opus last among audio codecs.
var DefaultAnswerCodecPreference = []string{
	CodecPCMA,
	CodecPCMU,
	CodecG722,
	CodecOpus,
}

// FallbackAnswerCodecs is used when the offer is missing or has no intersecting audio codecs.
func FallbackAnswerCodecs() []Codec {
	return []Codec{
		{PayloadType: 8, Name: CodecPCMA, ClockRate: 8000, Channels: 1},
		{PayloadType: 101, Name: CodecTelephoneEvent, ClockRate: 8000, Channels: 1},
	}
}

// SelectAnswerCodecs picks answer codecs from an offer (intersection with DefaultAnswerCodecPreference),
// always appending telephone-event when the offer advertises it. Nil/empty offer → FallbackAnswerCodecs.
func SelectAnswerCodecs(offer *Info) []Codec {
	if offer == nil || len(offer.Codecs) == 0 {
		return FallbackAnswerCodecs()
	}

	byName := map[string]Codec{}
	for _, c := range offer.Codecs {
		name := strings.ToLower(strings.TrimSpace(c.Name))
		if name == "" {
			continue
		}
		if _, ok := byName[name]; ok {
			continue // keep first (usually preferred PT from offer)
		}
		byName[name] = c
	}

	out := make([]Codec, 0, 4)
	for _, want := range DefaultAnswerCodecPreference {
		if c, ok := byName[want]; ok {
			out = append(out, c)
		}
	}
	if te, ok := PickTelephoneEventFromOffer(offer.Codecs, 8000); ok {
		out = append(out, te)
	}
	if len(out) == 0 {
		return FallbackAnswerCodecs()
	}
	return out
}

// BuildAudioAnswer builds a minimal audio SDP answer for localIP:localPort.
//
// Codec list is SelectAnswerCodecs(offer). Media proto prefers the offer's m= proto
// (e.g. RTP/AVP, RTP/SAVP) when present; otherwise RTP/AVP.
//
// This does not allocate RTP sockets — callers still bind media separately.
func BuildAudioAnswer(localIP string, localPort int, offer *Info) string {
	codecs := SelectAnswerCodecs(offer)
	proto := "RTP/AVP"
	if offer != nil {
		if p := strings.TrimSpace(offer.Proto); p != "" {
			proto = p
		}
	}
	return GenerateWithProto(localIP, localPort, proto, codecs)
}
