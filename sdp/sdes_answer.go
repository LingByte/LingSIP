package sdp

import (
	"crypto/rand"
	"strings"

	"github.com/pion/srtp/v2"
)

// SDESAnswer bundles negotiated SDES-SRTP material for a UAS answer.
type SDESAnswer struct {
	CryptoLine string
	Profile    srtp.ProtectionProfile
	PeerKey    []byte
	PeerSalt   []byte
	LocalKey   []byte
	LocalSalt  []byte
}

// BuildSDESAnswerFromOffer picks a supported peer offer, mints local keying, and
// returns the a=crypto line plus key material for EnableSDESSRTPWithProfile.
func BuildSDESAnswerFromOffer(offer *Info) (SDESAnswer, bool, error) {
	var empty SDESAnswer
	if offer == nil {
		return empty, false, nil
	}
	if !strings.Contains(strings.ToUpper(offer.Proto), "SAVP") || IsDTLSTransport(offer.Proto) {
		return empty, false, nil
	}
	co, ok := PickSupportedSDESOffer(offer.CryptoOffers)
	if !ok {
		return empty, false, nil
	}
	prof, profOK := PionProfileForSuite(co.Suite)
	if !profOK {
		return empty, true, errUnsupportedSRTPSuite(co.Suite)
	}
	rk, rsalt, err := DecodeSDESInline(co.KeyParams)
	if err != nil {
		return empty, true, errInvalidSRTPInline(err)
	}
	lk := make([]byte, 16)
	lsalt := make([]byte, 14)
	if _, err := rand.Read(lk); err != nil {
		return empty, true, err
	}
	if _, err := rand.Read(lsalt); err != nil {
		return empty, true, err
	}
	cryptoLine, err := FormatCryptoLine(co.Tag, co.Suite, lk, lsalt)
	if err != nil {
		return empty, true, errSRTPAnswerCrypto(err)
	}
	return SDESAnswer{
		CryptoLine: cryptoLine,
		Profile:    prof,
		PeerKey:    rk,
		PeerSalt:   rsalt,
		LocalKey:   lk,
		LocalSalt:  lsalt,
	}, true, nil
}