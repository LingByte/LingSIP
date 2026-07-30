package uas

import (
	"strconv"
	"strings"

	"github.com/LingByte/lingsip/stack"
)

// Reason phrases for common UAS error responses not defined in pkg/sip/stack.
const (
	ReasonMethodNotAllowed    = "Method Not Allowed"
	ReasonBusyHere            = "Busy Here"
	ReasonNotAcceptableHere   = "Not Acceptable Here"
	ReasonServerInternalError = "Server Internal Error"
	ReasonGenericError        = "Error"
)

// NewResponse builds a SIP response with common headers copied from the request
// (Via, From, To, Call-ID, CSeq). body and contentType may be empty (Content-Length: 0).
//
// The response shell is created via stack.NewResponse; this function adds UAS-specific
// header copying and body handling on top of that base.
func NewResponse(req *stack.Message, status int, reason, body, contentType string) (*stack.Message, error) {
	if req == nil || !req.IsRequest {
		return nil, ErrNeedRequest
	}
	if status < 100 || status > 699 {
		return nil, errInvalidStatus(status)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = stack.ReasonOK
	}
	resp := stack.NewResponse(status, reason)
	if vias := req.GetHeaders(stack.HeaderVia); len(vias) > 0 {
		resp.SetHeader(stack.HeaderVia, vias[0])
		for i := 1; i < len(vias); i++ {
			resp.AddHeader(stack.HeaderVia, vias[i])
		}
	}
	for _, h := range []string{
		stack.HeaderFrom,
		stack.HeaderTo,
		stack.HeaderCallID,
		stack.HeaderCSeq,
	} {
		if v := req.GetHeader(h); v != "" {
			resp.SetHeader(h, v)
		}
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	resp.Body = body
	if strings.TrimSpace(contentType) != "" {
		resp.SetHeader(stack.HeaderContentType, strings.TrimSpace(contentType))
	}
	resp.SetHeader(stack.HeaderContentLength, strconv.Itoa(stack.BodyBytesLen(body)))
	return resp, nil
}

// NewResponseWithTo is NewResponse with an optional To header override (e.g. tag on 200 OK).
func NewResponseWithTo(req *stack.Message, status int, reason, body, contentType, toOverride string) (*stack.Message, error) {
	resp, err := NewResponse(req, status, reason, body, contentType)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(toOverride) != "" {
		resp.SetHeader(stack.HeaderTo, strings.TrimSpace(toOverride))
	}
	return resp, nil
}

// ErrorResponse returns a minimal final error response (3xx–6xx) with a default reason phrase when empty.
func ErrorResponse(req *stack.Message, status int, reason string) (*stack.Message, error) {
	if reason == "" {
		switch status {
		case stack.StatusBadRequest:
			reason = stack.ReasonBadRequest
		case stack.StatusForbidden:
			reason = stack.ReasonForbidden
		case stack.StatusNotFound:
			reason = stack.ReasonNotFound
		case 405:
			reason = ReasonMethodNotAllowed
		case 486:
			reason = ReasonBusyHere
		case 488:
			reason = ReasonNotAcceptableHere
		case 500:
			reason = ReasonServerInternalError
		case stack.StatusServiceUnavailable:
			reason = stack.ReasonServiceUnavailable
		default:
			reason = ReasonGenericError
		}
	}
	return NewResponse(req, status, reason, "", "")
}
