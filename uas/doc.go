// Package uas registers inbound (UAS-side) SIP method handlers on stack.Endpoint using typed callbacks.
//
// Response building:
//   - stack.NewResponse — empty response shell (VersionSIP20, status, reason).
//   - uas.NewResponse — copies dialog headers from a request + optional body (uses stack.NewResponse internally).
//   - uas.ErrorResponse — common 4xx/5xx with default reason phrases.
//
// Transaction wiring:
//   - AttachWithTransaction registers all method handlers and chains transaction.Manager
//     (INVITE/non-INVITE retransmit absorption, CANCEL matching, ACK → HandleAck,
//     AfterResponseSentBeginServerTx on final responses).
//   - Production SIPServer (pkg/sip/server) uses AttachWithTransaction exclusively via uas_wire.go.
//
// Validation errors are sentinels in errors.go (errors.Is).
//
// See also: pkg/sip/dialog, pkg/sip/session, pkg/sip/transaction, pkg/sip/stack.
package uas
