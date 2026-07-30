package stir

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPFetcher_FetchSelfSigned(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "stir-leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBody := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(pemBody)
	}))
	defer srv.Close()

	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())

	f := &HTTPFetcher{
		HTTPClient:      srv.Client(),
		RootCAs:         pool,
		SkipChainVerify: true,
	}
	cert, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Subject.CommonName != "stir-leaf" {
		t.Fatalf("cn=%q", cert.Subject.CommonName)
	}
	// Cached hit
	if _, err := f.Fetch(context.Background(), srv.URL); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPFetcher_InvalidURL(t *testing.T) {
	f := &HTTPFetcher{HTTPClient: http.DefaultClient}
	if _, err := f.Fetch(context.Background(), "://bad"); err == nil {
		t.Fatal("expected error")
	}
}
