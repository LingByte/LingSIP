package signaling

import (
	"strconv"
	"strings"

	"github.com/LingByte/lingsip/stack"
)

const (
	ByeReasonNormal       = "normal"
	ByeReasonUserHangup   = "user_hangup"
	ByeReasonTimerExpired = "timer_expired"
	ByeReasonError        = "error"
)

// ClassifyBYEReason returns a bounded reason class and raw Reason header text.
func ClassifyBYEReason(msg *stack.Message) (reasonClass, rawText string) {
	if msg == nil {
		return ByeReasonNormal, ""
	}
	reasonHdr := strings.TrimSpace(msg.GetHeader("Reason"))
	if reasonHdr == "" {
		return ByeReasonNormal, ""
	}
	proto, cause, text := ParseRFC3326Reason(reasonHdr)
	rawText = strings.TrimSpace(text)

	switch strings.ToUpper(proto) {
	case "Q.850":
		switch cause {
		case 16, 17, 18, 19:
			return ByeReasonNormal, rawText
		case 21:
			return ByeReasonUserHangup, rawText
		case 102:
			return ByeReasonTimerExpired, rawText
		case 34, 38, 41, 42, 47:
			return ByeReasonError, rawText
		}
	case "SIP":
		switch cause {
		case 200:
			return ByeReasonNormal, rawText
		case 408:
			return ByeReasonTimerExpired, rawText
		default:
			if cause >= 400 {
				return ByeReasonError, rawText
			}
		}
	}
	return ByeReasonNormal, rawText
}

// ParseRFC3326Reason extracts protocol, cause, and text from a Reason header.
func ParseRFC3326Reason(hdr string) (proto string, cause int, text string) {
	parts := strings.SplitN(hdr, ";", 2)
	proto = strings.TrimSpace(parts[0])
	if len(parts) < 2 {
		return proto, 0, ""
	}
	for _, kv := range strings.Split(parts[1], ";") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[:eq]))
		val := strings.TrimSpace(kv[eq+1:])
		switch key {
		case "cause":
			val = strings.Trim(val, `"`)
			if n, err := strconv.Atoi(val); err == nil {
				cause = n
			}
		case "text":
			text = strings.Trim(val, `"`)
		}
	}
	return proto, cause, text
}
