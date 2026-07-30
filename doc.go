// Package lingsip is a pure SIP signaling library extracted from LingEchoX.
//
// Packages:
//
//   - stack          — SIP/2.0 message parse/serialize + UDP endpoint + TCP/TLS stream serving
//   - transaction    — RFC 3261 client/server transactions
//   - uas            — UAS handler registration + server-tx middleware
//   - dialog         — dialog identifiers (Call-ID, tags, CSeq)
//   - sdp            — SDP offer/answer helpers
//   - session_timer  — RFC 4028 Session-Expires
//   - auth           — RFC 2617 / RFC 3261 §22 digest authentication
//   - identity       — RFC 3325 P-Asserted-Identity / Privacy
//   - historyinfo    — RFC 7044 History-Info + Diversion retargeting
//   - stir           — RFC 8224 STIR/SHAKEN PASSporT sign/verify
//   - signaling      — RFC 3326 Reason header parsing and BYE reason classes
//   - dtmf           — RFC 2833/4733 telephone-event and SIP INFO digit parsing
//
// Product B2BUA, RTP media, persistence, and contact-center logic stay in
// LingEchoX (pkg/sipnx, pkg/contactcenter).
package lingsip
