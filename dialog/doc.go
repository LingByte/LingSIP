// Package dialog tracks minimal SIP dialog state (Call-ID, tags, early/confirmed).
//
// Tag helpers (TagFromHeader, AppendTagAfterNameAddr) are used by pkg/sip/server
// for To-tag generation and by Dialog for ACK matching. Wire Dialog via
// server.rememberUASDialog on inbound 200 OK; MatchACK validates inbound ACKs.
//
// It aligns with transaction keys via InviteTransactionKey (branch + Call-ID).
// It does not parse SDP or touch RTP; use pkg/sip/session for media.
package dialog
