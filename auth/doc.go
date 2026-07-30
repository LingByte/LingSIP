// Package auth implements HTTP Digest authentication (RFC 2617 / RFC 3261 §22) for SIP UAS.
//
// DigestAuth is a single-credential server: it issues single-use nonces via Challenge401 and
// validates Authorization / Proxy-Authorization headers via VerifyRequest. The digest math
// (MD5Hex, DigestHA1, DigestExpectResponse) is exported for callers that need to build client
// credentials or run their own credential lookup.
package auth
