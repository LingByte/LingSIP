package sdp

// Canonical RTP codec names (lowercase, as used in a=rtpmap and Codec.Name).
const (
	CodecPCMU           = "pcmu"
	CodecPCMA           = "pcma"
	CodecG722           = "g722"
	CodecOpus           = "opus"
	CodecTelephoneEvent = "telephone-event"
)

// DefaultOutboundOfferCodecs is the standard outbound INVITE audio preference list:
// PCMA first for carrier interoperability, then PCMU, G.722, Opus, telephone-event.
func DefaultOutboundOfferCodecs() []Codec {
	return []Codec{
		{PayloadType: 8, Name: CodecPCMA, ClockRate: 8000, Channels: 1},
		{PayloadType: 0, Name: CodecPCMU, ClockRate: 8000, Channels: 1},
		{PayloadType: 9, Name: CodecG722, ClockRate: 8000, Channels: 1},
		{PayloadType: 111, Name: CodecOpus, ClockRate: 48000, Channels: 1},
		{PayloadType: 101, Name: CodecTelephoneEvent, ClockRate: 8000, Channels: 1},
	}
}

// TransferAgentBridgeOfferCodecs is the INVITE offer for the human/agent leg after transfer
// (narrowband-first to simplify PCM bridging).
func TransferAgentBridgeOfferCodecs() []Codec {
	return []Codec{
		{PayloadType: 8, Name: CodecPCMA, ClockRate: 8000, Channels: 1},
		{PayloadType: 0, Name: CodecPCMU, ClockRate: 8000, Channels: 1},
		{PayloadType: 101, Name: CodecTelephoneEvent, ClockRate: 8000, Channels: 1},
	}
}
