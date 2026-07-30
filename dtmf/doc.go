// Package dtmf decodes out-of-band SIP DTMF digits.
//
// Two transports are covered:
//
//   - RTP telephone-event (RFC 2833 / RFC 4733) via EventFromRFC2833. The remote SDP must
//     offer a=rtpmap:PT telephone-event/8000 (or /48000) and the caller must feed payloads
//     of that payload type. Key-up is signalled by the E (end) bit; in-band tone detection
//     is out of scope.
//   - SIP INFO bodies (application/dtmf-relay, application/dtmf) via DigitFromSIPINFO, used
//     by clients such as Linphone.
//
// Both functions are pure parsers: media plumbing and event dispatch belong to the caller.
package dtmf
