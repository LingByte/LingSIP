package stack

// Protocol version and transport tokens (RFC 3261).
const (
	VersionSIP20    = "SIP/2.0"
	VersionPrefix   = "SIP/" // first line of SIP responses
	TransportUDP    = "UDP"
	TransportPrefix = VersionSIP20 + "/" + TransportUDP // e.g. Via: SIP/2.0/UDP host:port;branch=...
)

// Common SIP header names (wire form as used in SetHeader / GetHeader).
const (
	HeaderVia           = "Via"
	HeaderFrom          = "From"
	HeaderTo            = "To"
	HeaderCallID        = "Call-ID"
	HeaderCSeq          = "CSeq"
	HeaderContact       = "Contact"
	HeaderContentLength = "Content-Length"
	HeaderContentType   = "Content-Type"
	HeaderAllow         = "Allow"
	HeaderAccept        = "Accept"
	HeaderSupported     = "Supported"
	HeaderMaxForwards   = "Max-Forwards"
	HeaderUserAgent     = "User-Agent"
	HeaderRequire       = "Require"
	HeaderRSeq          = "RSeq"
	HeaderRAck          = "RAck"
	HeaderRecordRoute   = "Record-Route"
	HeaderRoute         = "Route"
	HeaderReason        = "Reason"
	HeaderExpires       = "Expires"
)

// Content-Type values used in this codebase.
const (
	ContentTypeSDP = "application/sdp"
)

// Common SIP response status codes.
const (
	StatusTrying                      = 100
	StatusRinging                     = 180
	StatusSessionProgress             = 183
	StatusOK                          = 200
	StatusAccepted                    = 202
	StatusBadRequest                  = 400
	StatusUnauthorized                = 401
	StatusForbidden                   = 403
	StatusNotFound                    = 404
	StatusRequestTimeout              = 408
	StatusBadEvent                    = 489
	StatusRequestTerminated           = 487
	StatusSessionIntervalTooSmall     = 422
	StatusCallTransactionDoesNotExist = 481
	StatusServiceUnavailable          = 503
	StatusServerTimeout               = 504
)

// Standard reason phrases paired with the status codes above.
const (
	ReasonTrying                      = "Trying"
	ReasonRinging                     = "Ringing"
	ReasonSessionProgress             = "Session Progress"
	ReasonOK                          = "OK"
	ReasonAccepted                    = "Accepted"
	ReasonBadRequest                  = "Bad Request"
	ReasonUnauthorized                = "Unauthorized"
	ReasonForbidden                   = "Forbidden"
	ReasonNotFound                    = "Not Found"
	ReasonRequestTimeout              = "Request Timeout"
	ReasonBadEvent                    = "Bad Event"
	ReasonRequestTerminated           = "Request Terminated"
	ReasonSessionIntervalTooSmall     = "Session Interval Too Small"
	ReasonCallTransactionDoesNotExist = "Call/Transaction Does Not Exist"
	ReasonServiceUnavailable          = "Service Unavailable"
	ReasonServerTimeout               = "Server Time-out"
)

// preferredHeaderOrder is the canonical (lowercase) header key order for Message.String().
var preferredHeaderOrder = []string{
	canonicalHeaderKey(HeaderVia),
	canonicalHeaderKey(HeaderMaxForwards),
	canonicalHeaderKey(HeaderFrom),
	canonicalHeaderKey(HeaderTo),
	canonicalHeaderKey(HeaderCallID),
	canonicalHeaderKey(HeaderCSeq),
	canonicalHeaderKey(HeaderContact),
	canonicalHeaderKey(HeaderAllow),
	canonicalHeaderKey(HeaderSupported),
	canonicalHeaderKey(HeaderUserAgent),
	canonicalHeaderKey(HeaderContentType),
	canonicalHeaderKey(HeaderContentLength),
}
