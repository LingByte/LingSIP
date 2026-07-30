package transaction

import (
	"strings"

	"github.com/LingByte/lingsip/stack"
)

// AckRequestURIFor2xx returns the Request-URI to use for an ACK sent after receiving a 2xx response to INVITE.
// Follows RFC 3261 rule: use the Contact header address from the 2xx response for dialog establishment.
// Falls back to the original INVITE Request-URI if Contact is unavailable or invalid.
func AckRequestURIFor2xx(resp *stack.Message, inviteRequestURI string) string {
	if resp == nil {
		return strings.TrimSpace(inviteRequestURI)
	}

	// Extract and clean Contact header
	c := strings.TrimSpace(resp.GetHeader(stack.HeaderContact))
	if c == "" {
		return strings.TrimSpace(inviteRequestURI)
	}

	// Remove angle brackets <uri>
	c = strings.TrimPrefix(c, "<")
	c = strings.TrimSuffix(c, ">")

	// Strip URI parameters (after semicolon)
	if idx := strings.Index(c, ";"); idx > 0 {
		c = c[:idx]
	}
	c = strings.TrimSpace(c)

	// Use Contact if it's a valid sip URI
	if strings.HasPrefix(strings.ToLower(c), "sip:") {
		return c
	}

	// Fallback to original INVITE Request-URI
	return strings.TrimSpace(inviteRequestURI)
}

// BuildAckForInvite constructs a valid ACK request for a completed INVITE transaction.
//
// Usage rules (RFC 3261):
//   - For 2xx responses: use AckRequestURIFor2xx() to get the Request-URI from Contact
//   - For 300-699 responses: use the same Request-URI as the original INVITE
//
// Copies essential dialog identifiers (Call-ID, From, To, Via, CSeq) from the INVITE and response.
func BuildAckForInvite(invite *stack.Message, final *stack.Message, requestURI string) (*stack.Message, error) {
	if invite == nil || final == nil {
		return nil, ErrNilMessage
	}

	// Validate original message is an INVITE
	if !stack.IsInviteCSeq(invite) {
		return nil, ErrInviteCSeqNotInvite
	}

	// Validate we received a final response (200-699)
	st := final.StatusCode
	if st < 200 || st > 699 {
		return nil, errFinalNotFinalResponse(st)
	}

	// Set Request-URI, fallback to INVITE if empty
	reqURI := strings.TrimSpace(requestURI)
	if reqURI == "" {
		reqURI = strings.TrimSpace(invite.RequestURI)
	}

	// Parse valid sequence number from INVITE CSeq
	n := stack.ParseCSeqNum(invite.GetHeader("CSeq"))
	if n <= 0 {
		return nil, ErrInvalidCSeqOnInvite
	}

	// Create ACK request message
	ack := &stack.Message{
		IsRequest:    true,
		Method:       stack.MethodAck,
		RequestURI:   reqURI,
		Version:      stack.VersionSIP20,
		Headers:      map[string]string{},
		HeadersMulti: map[string][]string{},
	}

	// Copy top Via header from INVITE (required for routing back to UAS)
	if v := stack.TopVia(invite); v != "" {
		ack.SetHeader("Via", v)
	} else {
		return nil, ErrInviteMissingVia
	}

	// Set mandatory SIP headers
	ack.SetHeader("Max-Forwards", "70")

	// Copy From tag from INVITE
	if f := strings.TrimSpace(invite.GetHeader("From")); f != "" {
		ack.SetHeader("From", f)
	}

	// Copy To header (prefer from final response to include UAS tag)
	if to := strings.TrimSpace(final.GetHeader("To")); to != "" {
		ack.SetHeader("To", to)
	} else if to := strings.TrimSpace(invite.GetHeader("To")); to != "" {
		ack.SetHeader("To", to)
	}

	// Copy dialog identifiers
	ack.SetHeader("Call-ID", strings.TrimSpace(invite.GetHeader("Call-ID")))
	ack.SetHeader("CSeq", stack.WithCSeqACK(n))
	ack.SetHeader("Content-Length", "0")

	return ack, nil
}
