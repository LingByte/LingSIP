package sdp_test

import (
	"strings"
	"testing"

	"github.com/LingByte/lingsip/sdp"
)

func TestBuildSDESAnswerFromOfferNil(t *testing.T) {
	_, ok, err := sdp.BuildSDESAnswerFromOffer(nil)
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestBuildSDESAnswerFromOfferPlainRTP(t *testing.T) {
	_, ok, err := sdp.BuildSDESAnswerFromOffer(&sdp.Info{Proto: "RTP/AVP"})
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestBuildSDESAnswerFromOfferSAVP(t *testing.T) {
	key := make([]byte, 16)
	salt := make([]byte, 14)
	for i := range key {
		key[i] = byte(i + 1)
	}
	for i := range salt {
		salt[i] = byte(i + 10)
	}
	inline, err := sdp.FormatCryptoLine(1, "AES_CM_128_HMAC_SHA1_80", key, salt)
	if err != nil {
		t.Fatal(err)
	}
	offer := &sdp.Info{
		Proto: "RTP/SAVP",
		CryptoOffers: []sdp.CryptoOffer{{
			Tag:       1,
			Suite:     "AES_CM_128_HMAC_SHA1_80",
			KeyParams: strings.TrimPrefix(inline, "a=crypto:1 AES_CM_128_HMAC_SHA1_80 "),
		}},
	}
	ans, ok, err := sdp.BuildSDESAnswerFromOffer(offer)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !strings.HasPrefix(ans.CryptoLine, "a=crypto:") {
		t.Fatalf("crypto=%q", ans.CryptoLine)
	}
}

func TestBuildSDESAnswerFromOfferNoSupportedCrypto(t *testing.T) {
	offer := &sdp.Info{
		Proto: "RTP/SAVP",
		CryptoOffers: []sdp.CryptoOffer{{
			Tag:       1,
			Suite:     "AEAD_AES_128_GCM",
			KeyParams: "inline:ABCDEFGHIJKLMNOPQRSTUVWX/Y",
		}},
	}
	_, ok, err := sdp.BuildSDESAnswerFromOffer(offer)
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
