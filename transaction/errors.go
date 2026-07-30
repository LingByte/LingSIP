package transaction

import (
	"errors"
	"fmt"
)

// Package-level sentinel errors for stable errors.Is checks.
var (
	ErrNilManager                 = errors.New("sip/transaction: nil manager")
	ErrNilInviteFinalOrSend       = errors.New("sip/transaction: nil invite, final, or send")
	ErrNotInviteRequest           = errors.New("sip/transaction: not an INVITE request")
	ErrInviteCSeqNotInvite        = errors.New("sip/transaction: invite CSeq is not INVITE")
	ErrInviteMissingViaBranch     = errors.New("sip/transaction: invite missing Via branch")
	ErrInviteMissingCallID        = errors.New("sip/transaction: invite missing Call-ID")
	ErrInvalidInviteCSeq          = errors.New("sip/transaction: invalid INVITE CSeq")
	ErrInviteServerTxExists       = errors.New("sip/transaction: invite server tx already exists for branch/call-id")
	ErrNilMessage                 = errors.New("sip/transaction: nil message")
	ErrInvalidCSeqOnInvite        = errors.New("sip/transaction: invalid CSeq on INVITE")
	ErrInviteMissingVia           = errors.New("sip/transaction: invite missing Via")
	ErrNilRequestFinalOrSend      = errors.New("sip/transaction: nil request, final, or send")
	ErrNotRequest                 = errors.New("sip/transaction: not a request")
	ErrUseBeginInviteServerForInvite = errors.New("sip/transaction: use BeginInviteServer for INVITE")
	ErrMissingViaBranchOrCSeq     = errors.New("sip/transaction: missing Via branch or CSeq")
	ErrNonInviteServerTxExists    = errors.New("sip/transaction: non-invite server tx already exists")
	ErrNilFrozenRequest           = errors.New("sip/transaction: nil frozen request")
	ErrNilRequestOrSend           = errors.New("sip/transaction: nil request or send")
	ErrRequestMissingViaBranch    = errors.New("sip/transaction: request missing Via branch")
	ErrRequestMissingCallID       = errors.New("sip/transaction: request missing Call-ID")
	ErrInvalidCSeq                = errors.New("sip/transaction: invalid CSeq")
	ErrNeedInvite                 = errors.New("sip/transaction: need INVITE")
	ErrMissingCallID              = errors.New("sip/transaction: missing Call-ID")
	ErrMissingViaBranch           = errors.New("sip/transaction: missing Via branch")
	ErrBadInviteCSeq              = errors.New("sip/transaction: bad INVITE CSeq")
	ErrNilFrozenInvite            = errors.New("sip/transaction: nil frozen invite")
	ErrNilInviteOrSend            = errors.New("sip/transaction: nil invite or send")
)

func errFinalStatusNotFinal(code int) error {
	return fmt.Errorf("sip/transaction: final status %d is not final", code)
}

func errFinalNotFinalResponse(code int) error {
	return fmt.Errorf("sip/transaction: final status %d is not a final response", code)
}

func errNonInviteServerTxExists(key string) error {
	return fmt.Errorf("sip/transaction: non-invite server tx already exists for %s: %w", key, ErrNonInviteServerTxExists)
}
