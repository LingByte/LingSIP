package auth

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LingByte/lingsip/stack"
	"github.com/LingByte/lingsip/uas"
)

// NonceTTL is how long an issued nonce stays valid before garbage collection.
const NonceTTL = 10 * time.Minute

type digestNonce struct {
	expires time.Time
}

// DigestAuth authenticates requests against a single realm/user/password credential.
// The zero value is not usable; build one with NewDigestAuth. A nil *DigestAuth is safe to
// call on and always fails verification.
type DigestAuth struct {
	realm  string
	user   string
	pass   string
	mu     sync.Mutex
	nonces map[string]digestNonce
}

// NewDigestAuth returns a digest authenticator, or nil if any credential field is blank.
func NewDigestAuth(realm, user, pass string) *DigestAuth {
	realm = strings.TrimSpace(realm)
	user = strings.TrimSpace(user)
	pass = strings.TrimSpace(pass)
	if realm == "" || user == "" || pass == "" {
		return nil
	}
	return &DigestAuth{
		realm:  realm,
		user:   user,
		pass:   pass,
		nonces: make(map[string]digestNonce),
	}
}

// Realm returns the configured realm.
func (d *DigestAuth) Realm() string {
	if d == nil {
		return ""
	}
	return d.realm
}

// GC drops expired nonces. Challenge401 calls it, so explicit use is only needed for
// long-lived authenticators that rarely challenge.
func (d *DigestAuth) GC() {
	if d == nil {
		return
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, v := range d.nonces {
		if now.After(v.expires) {
			delete(d.nonces, k)
		}
	}
}

func (d *DigestAuth) issueNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	n := hex.EncodeToString(b[:])
	d.mu.Lock()
	d.nonces[n] = digestNonce{expires: time.Now().Add(NonceTTL)}
	d.mu.Unlock()
	return n
}

// Challenge401 builds a 401 Unauthorized response carrying a fresh WWW-Authenticate nonce.
func (d *DigestAuth) Challenge401(req *stack.Message) (*stack.Message, error) {
	if d == nil || req == nil {
		return nil, fmt.Errorf("auth: digest not configured")
	}
	d.GC()
	nonce := d.issueNonce()
	www := fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5, qop="auth"`, d.realm, nonce)
	resp, err := uas.NewResponse(req, 401, "Unauthorized", "", "")
	if err != nil {
		return nil, err
	}
	resp.SetHeader("WWW-Authenticate", www)
	return resp, nil
}

// MD5Hex returns the lowercase hex MD5 digest of s.
func MD5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// DigestHA1 returns MD5(user:realm:pass).
func DigestHA1(user, realm, pass string) string {
	return MD5Hex(fmt.Sprintf("%s:%s:%s", user, realm, pass))
}

// DigestExpectResponse computes the expected digest response for the given credential
// parameters, request method and URI. qop=auth and the legacy no-qop form are supported.
func DigestExpectResponse(auth map[string]string, method, uri string, ha1 string) string {
	nonce := auth["nonce"]
	qop := strings.Trim(auth["qop"], `"`)
	if qop == "auth" {
		nc := auth["nc"]
		cnonce := strings.Trim(auth["cnonce"], `"`)
		ha2 := MD5Hex(fmt.Sprintf("%s:%s", method, uri))
		return MD5Hex(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))
	}
	ha2 := MD5Hex(fmt.Sprintf("%s:%s", method, uri))
	return MD5Hex(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
}

// ParseDigestAuth splits a Digest challenge or credential header into lowercase keys with
// surrounding quotes stripped from the values.
func ParseDigestAuth(h string) map[string]string {
	out := make(map[string]string)
	h = strings.TrimPrefix(strings.TrimSpace(h), "Digest")
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(strings.ToLower(kv[0]))
		v := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		out[k] = v
	}
	return out
}

// VerifyRequest checks the Authorization (or Proxy-Authorization) header of req against the
// configured credential. Nonces are single-use: a successful verification consumes them.
func (d *DigestAuth) VerifyRequest(req *stack.Message) bool {
	if d == nil || req == nil {
		return false
	}
	raw := req.GetHeader("Authorization")
	if raw == "" {
		raw = req.GetHeader("Proxy-Authorization")
	}
	if raw == "" || !strings.HasPrefix(strings.TrimSpace(strings.ToLower(raw)), "digest") {
		return false
	}
	auth := ParseDigestAuth(raw)
	if !strings.EqualFold(strings.TrimSpace(auth["username"]), d.user) ||
		!strings.EqualFold(strings.TrimSpace(auth["realm"]), d.realm) {
		return false
	}
	nonce := auth["nonce"]
	d.mu.Lock()
	_, ok := d.nonces[nonce]
	if ok {
		delete(d.nonces, nonce)
	}
	d.mu.Unlock()
	if !ok {
		return false
	}
	uri := auth["uri"]
	if uri == "" {
		uri = req.RequestURI
	}
	response := strings.Trim(auth["response"], `"`)
	if response == "" {
		return false
	}
	ha1 := DigestHA1(d.user, d.realm, d.pass)
	expect := DigestExpectResponse(auth, req.Method, uri, ha1)
	return strings.EqualFold(expect, response)
}
