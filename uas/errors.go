package uas

import (
	"errors"
	"fmt"
)

// Package-level sentinel errors for stable errors.Is checks.
var (
	ErrNilEndpoint              = errors.New("sip/uas: nil endpoint")
	ErrNeedRequest              = errors.New("sip/uas: need a SIP request")
	ErrInvalidStatus            = errors.New("sip/uas: invalid status code")
	ErrTransactionSendRequired  = errors.New("sip/uas: TransactionBinding.Send required when Manager is set")
)

func errInvalidStatus(code int) error {
	return fmt.Errorf("sip/uas: invalid status %d: %w", code, ErrInvalidStatus)
}
