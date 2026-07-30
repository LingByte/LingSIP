package stack

import (
	"strconv"
	"strings"
)

// TopVia returns the first Via header field-value (SIP message order: top-most Via).
func TopVia(m *Message) string {
	if m == nil {
		return ""
	}
	vs := m.GetHeaders(HeaderVia)
	if len(vs) == 0 {
		return ""
	}
	return strings.TrimSpace(vs[0])
}

// BranchParam extracts the branch parameter from one Via field-value (case-insensitive "branch=").
func BranchParam(viaLine string) string {
	if viaLine == "" {
		return ""
	}
	lower := strings.ToLower(viaLine)
	idx := strings.Index(lower, "branch=")
	if idx < 0 {
		return ""
	}
	v := strings.TrimSpace(viaLine[idx+len("branch="):])
	if cut := strings.IndexByte(v, ';'); cut >= 0 {
		v = v[:cut]
	}
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "\"")
	v = strings.TrimSuffix(v, "\"")
	return v
}

// TopBranch returns BranchParam(TopVia(m)).
func TopBranch(m *Message) string {
	return BranchParam(TopVia(m))
}

// IsInviteCSeq reports whether the CSeq header refers to method INVITE.
func IsInviteCSeq(m *Message) bool {
	if m == nil {
		return false
	}
	return strings.Contains(strings.ToUpper(strings.TrimSpace(m.GetHeader(HeaderCSeq))), MethodInvite)
}

// IsAckCSeq reports whether the CSeq header refers to method ACK.
func IsAckCSeq(m *Message) bool {
	if m == nil {
		return false
	}
	return strings.Contains(strings.ToUpper(strings.TrimSpace(m.GetHeader(HeaderCSeq))), MethodAck)
}

// IsCancelCSeq reports whether the CSeq header refers to method CANCEL.
func IsCancelCSeq(m *Message) bool {
	if m == nil {
		return false
	}
	return strings.Contains(strings.ToUpper(strings.TrimSpace(m.GetHeader(HeaderCSeq))), MethodCancel)
}

// NonInviteServerKey builds a stable key for a non-INVITE request (branch + Call-ID + method + CSeq number).
func NonInviteServerKey(req *Message) string {
	if req == nil {
		return ""
	}
	return InviteClientKey(TopBranch(req), req.GetHeader(HeaderCallID)) + "\x01" +
		strings.ToUpper(strings.TrimSpace(req.Method)) + "\x01" +
		strconv.Itoa(ParseCSeqNum(req.GetHeader(HeaderCSeq)))
}

func InviteClientKey(branch, callID string) string {
	return strings.TrimSpace(branch) + "\x00" + strings.TrimSpace(callID)
}

// InviteTransactionKey is the INVITE transaction map key (top Via branch + Call-ID).
func InviteTransactionKey(branch, callID string) string {
	return InviteClientKey(branch, callID)
}
