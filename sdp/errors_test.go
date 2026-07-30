package sdp

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	cases := []error{
		ErrEmptyBody,
		ErrNoCodec,
		ErrEmptyCryptoKeyParams,
		ErrCryptoMissingInline,
		ErrEmptySuite,
		ErrUnsupportedSRTPSuite,
		ErrInvalidSRTPInline,
		ErrSRTPAnswerCrypto,
		ErrDTLSFingerprintMismatch,
	}
	for _, err := range cases {
		if !errors.Is(err, err) {
			t.Fatalf("not self-equal: %v", err)
		}
		if err.Error() == "" {
			t.Fatal("empty Error() string")
		}
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse(""); !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("empty body: %v", err)
	}
	if _, err := Parse("v=0\r\no=- 1 1 IN IP4 1.1.1.1\r\ns=x\r\nc=IN IP4 1.1.1.1\r\nt=0 0\r\nm=audio 999999999999999999999 RTP/AVP 0\r\n"); err == nil || !strings.Contains(err.Error(), "invalid m=audio port") {
		t.Fatalf("bad port: %v", err)
	}
	body := strings.Join([]string{
		"v=0",
		"o=- 1 1 IN IP4 1.1.1.1",
		"s=x",
		"c=IN IP4 1.1.1.1",
		"t=0 0",
		"m=audio 49170 RTP/AVP 99",
	}, "\r\n")
	if _, err := Parse(body); !errors.Is(err, ErrNoCodec) {
		t.Fatalf("no codec: %v", err)
	}
}

func TestCryptoErrors(t *testing.T) {
	if _, _, err := DecodeSDESInline(""); !errors.Is(err, ErrEmptyCryptoKeyParams) {
		t.Fatalf("empty key-params: %v", err)
	}
	if _, _, err := DecodeSDESInline("foo bar"); !errors.Is(err, ErrCryptoMissingInline) {
		t.Fatalf("missing inline: %v", err)
	}
	if _, err := FormatCryptoLine(1, "", nil, nil); !errors.Is(err, ErrEmptySuite) {
		t.Fatalf("empty suite: %v", err)
	}
	_, err := FormatCryptoLine(1, SuiteAESCM128HMACSHA180, []byte{1}, []byte{1})
	if err == nil || !strings.Contains(err.Error(), "key/salt length mismatch") {
		t.Fatalf("length mismatch: %v", err)
	}
}

func TestWrappedErrors(t *testing.T) {
	root := io.ErrUnexpectedEOF
	if !errors.Is(errInlineBase64(root), root) {
		t.Fatal("inline base64 wrap")
	}
	if !errors.Is(errInvalidAudioPort(root), root) {
		t.Fatal("audio port wrap")
	}
	var numErr *strconv.NumError
	body := "v=0\r\no=- 1 1 IN IP4 1.1.1.1\r\ns=x\r\nc=IN IP4 1.1.1.1\r\nt=0 0\r\nm=audio 999999999999999999999 RTP/AVP 0\r\n"
	_, err := Parse(body)
	if err == nil {
		t.Fatal("expected port error")
	}
	if !errors.As(err, &numErr) {
		t.Fatalf("expected NumError wrap, got %T %v", err, err)
	}
}
