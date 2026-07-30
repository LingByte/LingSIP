package sdp

import (
	"errors"
	"fmt"
)

// Package-level sentinel errors for stable errors.Is checks.
var (
	ErrEmptyBody               = errors.New("sip/sdp: empty body")
	ErrNoCodec                 = errors.New("sip/sdp: no codec found")
	ErrEmptyCryptoKeyParams    = errors.New("sip/sdp: empty crypto key-params")
	ErrCryptoMissingInline     = errors.New("sip/sdp: crypto key-params missing inline")
	ErrEmptySuite              = errors.New("sip/sdp: empty suite")
	ErrUnsupportedSRTPSuite    = errors.New("sip/sdp: unsupported SRTP suite")
	ErrInvalidSRTPInline       = errors.New("sip/sdp: invalid SRTP inline")
	ErrSRTPAnswerCrypto        = errors.New("sip/sdp: SRTP answer crypto")
	ErrDTLSFingerprintMismatch = errors.New("sdp/dtls: no advertised fingerprint matched peer cert")
)

func errInvalidAudioPort(cause error) error {
	return fmt.Errorf("sip/sdp: invalid m=audio port: %w", cause)
}

func errInlineBase64(cause error) error {
	return fmt.Errorf("sip/sdp: inline base64: %w", cause)
}

func errInlineMaterialTooShort(got, need int) error {
	return fmt.Errorf("sip/sdp: inline material too short: got %d need %d", got, need)
}

func errKeySaltLengthMismatch(suite string) error {
	return fmt.Errorf("sip/sdp: key/salt length mismatch for %s", suite)
}

func errUnsupportedSRTPSuite(suite string) error {
	return fmt.Errorf("sip/sdp: unsupported SRTP suite in answer: %s: %w", suite, ErrUnsupportedSRTPSuite)
}

func errInvalidSRTPInline(cause error) error {
	return fmt.Errorf("sip/sdp: invalid SRTP inline: %w: %w", cause, ErrInvalidSRTPInline)
}

func errSRTPAnswerCrypto(cause error) error {
	return fmt.Errorf("sip/sdp: SRTP answer crypto: %w: %w", cause, ErrSRTPAnswerCrypto)
}

func errDTLSFingerprintMismatch(algs []string) error {
	return fmt.Errorf("sdp/dtls: no advertised fingerprint matched peer cert (tried %v): %w", algs, ErrDTLSFingerprintMismatch)
}
