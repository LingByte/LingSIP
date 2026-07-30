package session_timer

import (
	"strings"

	"github.com/LingByte/lingsip/stack"
)

// NegotiateUASFromINVITE runs UAS-side session-timer negotiation on an inbound INVITE.
func NegotiateUASFromINVITE(msg *stack.Message) Decision {
	if msg == nil {
		return Decision{}
	}
	peerSE, peerRefresher, _ := ParseSessionExpires(msg.GetHeader("Session-Expires"))
	peerMinSE := ParseMinSE(msg.GetHeader("Min-SE"))
	supported := ParseTokenList(msg.GetHeader("Supported"))
	require := ParseTokenList(msg.GetHeader("Require"))
	return NegotiateUAS(
		peerSE,
		peerRefresher,
		peerMinSE,
		HasToken(supported, SupportedTokenTimer),
		HasToken(require, SupportedTokenTimer),
		DefaultMinSE,
		DefaultSE,
	)
}

// MergeSupportedToken appends tok to msg's Supported header if not already present.
func MergeSupportedToken(msg *stack.Message, tok string) {
	tok = strings.TrimSpace(tok)
	if msg == nil || tok == "" {
		return
	}
	existing := ParseTokenList(msg.GetHeader("Supported"))
	lower := strings.ToLower(tok)
	for _, t := range existing {
		if t == lower {
			return
		}
	}
	existing = append(existing, lower)
	msg.SetHeader("Supported", strings.Join(existing, ", "))
}
