package stir

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestLoadES256PrivateKey_InvalidPEM(t *testing.T) {
	if _, err := LoadES256PrivateKey([]byte("not pem")); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadES256PrivateKey_PKCS8(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	got, err := LoadES256PrivateKey(pemBytes)
	if err != nil || got == nil {
		t.Fatalf("LoadES256PrivateKey: %v", err)
	}
}

func TestLoadES256Certificate_SelfSigned(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-stir"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cert, err := LoadES256Certificate(pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	if PublicKeyFromCert(cert) == nil {
		t.Fatal("public key")
	}
}
