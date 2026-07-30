// Package stack is the SIP/2.0 signaling layer for lingsip.
//
// It provides:
//   - Message parsing and serialization (RFC 3261–style text format)
//   - Protocol constants (VersionSIP20, Header*, Status*, Reason*) in constants.go
//   - UDP datagram transport abstraction
//   - A minimal signaling Endpoint (listen, parse, dispatch requests; pass responses to a hook;
//     optional OnResponseSent after a successful response send for UAS server-tx binding)
//
// This package intentionally does not implement full SIP transaction state machines
// or digest authentication; those live in sibling packages (transaction, auth).
// TCP/TLS stream serving is in stream.go (ServeStreamConn / ListenAndServeTCP|TLS).
//
// Sentinel and wrapped errors live in errors.go (use errors.Is).
// Product runtime wiring lives in LingEchoX pkg/sipnx/kernel.
//
// Design goals:
//   - No process-wide mutable defaults; configure via structs and context.
//   - Read loop returns on non-timeout transport errors (after optional OnReadError logging).
//   - SIP wire parsing (Parse/String/BodyBytesLen) and UDP signaling live in this package.
//
// Audio codecs for RTP live in lingllm/media/encoder (CreateEncode/CreateDecode); stack does not implement codecs.
package stack
