package stack

import (
	"errors"
	"fmt"
)

// Package-level sentinel errors for stable errors.Is checks.
var (
	ErrNilEndpoint            = errors.New("sip/stack: nil endpoint")
	ErrEndpointNotOpen        = errors.New("sip/stack: endpoint not open")
	ErrNilMessage             = errors.New("sip/stack: nil message")
	ErrUDPTransportNotStarted = errors.New("sip/stack: udp transport not started")

	ErrEmptyMessage        = errors.New("sip/stack: empty message")
	ErrEmptyMessageLines   = errors.New("sip/stack: empty message lines")
	ErrEmptyFirstLine      = errors.New("sip/stack: empty first line")
	ErrEmptyMessageHeaders = errors.New("sip/stack: empty message headers")

	ErrEmptyRAck       = errors.New("sip/stack: empty RAck")
	ErrRAckNeedsFields = errors.New("sip/stack: RAck needs rseq cseq method")
	ErrRAckCSeq        = errors.New("sip/stack: RAck cseq")
)

func errInvalidResponseLine(line string) error {
	return fmt.Errorf("sip/stack: invalid response line: %s", line)
}

func errInvalidStatusCode(line string, cause error) error {
	return fmt.Errorf("sip/stack: invalid status code in %q: %w", line, cause)
}

func errInvalidRequestLine(line string) error {
	return fmt.Errorf("sip/stack: invalid request line: %s", line)
}

func errListenUDP(cause error) error {
	return fmt.Errorf("sip/stack: listen udp: %w", cause)
}

func errReadUDP(cause error) error {
	return fmt.Errorf("sip/stack: read udp: %w", cause)
}

func errSendResponse(cause error) error {
	return fmt.Errorf("sip/stack: send response: %w", cause)
}

func errRAckRSeq(cause error) error {
	return fmt.Errorf("sip/stack: RAck rseq: %w", cause)
}
