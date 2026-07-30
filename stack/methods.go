package stack

// SIP method constants defined by RFC specifications and IANA registry.
// All methods are uppercase as required by the SIP protocol (RFC 3261).
// These constants are used for request handling, message construction, and method registration.
const (

	// MethodInvite initiates a call/session establishment (RFC 3261)
	MethodInvite = "INVITE"

	// MethodAck acknowledges receipt of final response to an INVITE (RFC 3261)
	MethodAck = "ACK"

	// MethodBye terminates an existing dialog/call (RFC 3261)
	MethodBye = "BYE"

	// MethodCancel cancels a pending INVITE request (RFC 3261)
	MethodCancel = "CANCEL"

	// MethodOptions queries server capabilities (RFC 3261)
	MethodOptions = "OPTIONS"

	// MethodRegister binds contact address with location service (RFC 3261)
	MethodRegister = "REGISTER"

	// MethodPrack acknowledges provisional responses (1xx) reliably (RFC 3262)
	MethodPrack = "PRACK"

	// MethodSubscribe requests event state notifications (RFC 3265, RFC 6665)
	MethodSubscribe = "SUBSCRIBE"

	// MethodNotify sends event state notifications to subscribers (RFC 3265, RFC 6665)
	MethodNotify = "NOTIFY"

	// MethodPublish publishes event state to a notifier (RFC 3903)
	MethodPublish = "PUBLISH"

	// MethodInfo carries mid-session signaling information (RFC 6086)
	MethodInfo = "INFO"

	// MethodRefer requests peer to contact another URI (call transfer, RFC 3515)
	MethodRefer = "REFER"

	// MethodMessage sends instant messages (RFC 3428)
	MethodMessage = "MESSAGE"

	// MethodUpdate updates session parameters without restarting dialog (RFC 3311)
	MethodUpdate = "UPDATE"
)
