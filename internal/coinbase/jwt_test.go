// SPDX-License-Identifier: MIT

package coinbase

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

const testKeyName = "organizations/test-org/apiKeys/test-key"

// genRSAPKCS8 returns a PKCS#8 DER encoding of a small test-only RSA key.
func genRSAPKCS8(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal rsa pkcs8: %v", err)
	}
	return der
}

// genECKeyPEM generates a P-256 key and returns it with its SEC1 PEM encoding.
func genECKeyPEM(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

// decodeSegment decodes one base64url JWT segment into dst.
func decodeSegment(t *testing.T, seg string, dst any) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("decode segment: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("unmarshal segment: %v", err)
	}
}

// authorizeAndSplit runs Authorize on a GET request and returns the JWT parts.
func authorizeAndSplit(t *testing.T, a *jwtAuthorizer) (header, claims map[string]any, parts []string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://api.coinbase.com/api/v3/brokerage/accounts?limit=1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := a.Authorize(req); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	bearer := req.Header.Get("Authorization")
	if !strings.HasPrefix(bearer, "Bearer ") {
		t.Fatalf("Authorization = %q, want Bearer token", bearer)
	}
	parts = strings.Split(strings.TrimPrefix(bearer, "Bearer "), ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d segments, want 3", len(parts))
	}
	header, claims = map[string]any{}, map[string]any{}
	decodeSegment(t, parts[0], &header)
	decodeSegment(t, parts[1], &claims)
	return header, claims, parts
}

func TestJWT_ES256(t *testing.T) {
	key, pemStr := genECKeyPEM(t)
	a, err := NewJWTAuthorizer(testKeyName, pemStr)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer: %v", err)
	}
	base := time.Unix(1_770_000_000, 0)
	a.now = func() time.Time { return base }

	header, claims, parts := authorizeAndSplit(t, a)

	if header["alg"] != "ES256" || header["typ"] != "JWT" || header["kid"] != testKeyName {
		t.Errorf("header = %v", header)
	}
	if nonce, _ := header["nonce"].(string); len(nonce) != 32 {
		t.Errorf("nonce = %q, want 32 hex chars", header["nonce"])
	}
	if claims["iss"] != "cdp" || claims["sub"] != testKeyName {
		t.Errorf("claims = %v", claims)
	}
	// The uri claim binds the token to method + host + path (no query).
	if claims["uri"] != "GET api.coinbase.com/api/v3/brokerage/accounts" {
		t.Errorf("uri = %v", claims["uri"])
	}
	if nbf, exp := claims["nbf"].(float64), claims["exp"].(float64); int64(nbf) != base.Unix() || int64(exp-nbf) != int64(jwtTTL.Seconds()) {
		t.Errorf("nbf/exp = %v/%v, want %d ttl", nbf, exp, int64(jwtTTL.Seconds()))
	}

	// Verify the raw r||s signature against the public key.
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length = %d, want 64 (raw r||s)", len(sig))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
		t.Error("ES256 signature does not verify")
	}
}

func TestJWT_ES256_PKCS8AndEscapedNewlines(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	// Environment variables often carry the PEM with literal \n sequences.
	escaped := strings.ReplaceAll(pemStr, "\n", `\n`)

	a, err := NewJWTAuthorizer(testKeyName, escaped)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer(escaped pkcs8): %v", err)
	}
	if a.alg != "ES256" {
		t.Errorf("alg = %q, want ES256", a.alg)
	}
	if _, _, parts := authorizeAndSplit(t, a); len(parts) != 3 {
		t.Error("signing failed")
	}
}

func TestJWT_EdDSA_Base64(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	secret := base64.StdEncoding.EncodeToString(priv)

	a, err := NewJWTAuthorizer(testKeyName, secret)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer: %v", err)
	}
	header, _, parts := authorizeAndSplit(t, a)
	if header["alg"] != "EdDSA" {
		t.Errorf("alg = %v, want EdDSA", header["alg"])
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		t.Error("EdDSA signature does not verify")
	}
}

func TestJWT_EdDSA_SeedAndWhitespace(t *testing.T) {
	// A bare 32-byte seed (official Python SDK parity) with wrapped base64.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(priv.Seed())
	wrapped := b64[:20] + "\n  " + b64[20:] // interior whitespace must be tolerated

	a, err := NewJWTAuthorizer(testKeyName, wrapped)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer(seed): %v", err)
	}
	_, _, parts := authorizeAndSplit(t, a)
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		t.Error("EdDSA signature from seed does not verify")
	}
}

func TestJWT_EdDSA_PKCS8(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	a, err := NewJWTAuthorizer(testKeyName, pemStr)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer: %v", err)
	}
	_, _, parts := authorizeAndSplit(t, a)
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		t.Error("EdDSA signature does not verify")
	}
}

func TestJWT_FreshTokenPerRequest(t *testing.T) {
	_, pemStr := genECKeyPEM(t)
	a, err := NewJWTAuthorizer(testKeyName, pemStr)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer: %v", err)
	}
	_, _, first := authorizeAndSplit(t, a)
	_, _, second := authorizeAndSplit(t, a)
	if first[0] == second[0] {
		t.Error("nonce not refreshed between requests")
	}
}

func TestJWT_URIBoundToRequest(t *testing.T) {
	_, pemStr := genECKeyPEM(t)
	a, err := NewJWTAuthorizer(testKeyName, pemStr)
	if err != nil {
		t.Fatalf("NewJWTAuthorizer: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, "https://api.coinbase.com/api/v3/brokerage/orders", nil)
	if err := a.Authorize(req); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	parts := strings.Split(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "), ".")
	claims := map[string]any{}
	decodeSegment(t, parts[1], &claims)
	if claims["uri"] != "POST api.coinbase.com/api/v3/brokerage/orders" {
		t.Errorf("uri = %v", claims["uri"])
	}
}

func TestJWT_UnsupportedAlgorithm(t *testing.T) {
	// Guards against internal misuse: an authorizer must refuse to sign with
	// an algorithm it has no key for.
	a := &jwtAuthorizer{keyName: testKeyName, alg: "HS256", now: time.Now}
	req, _ := http.NewRequest(http.MethodGet, "https://api.coinbase.com/x", nil)
	if err := a.Authorize(req); err == nil || !strings.Contains(err.Error(), "unsupported JWT algorithm") {
		t.Fatalf("err = %v, want unsupported algorithm", err)
	}
}

func TestNewJWTAuthorizer_InvalidSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{"garbage", "not-a-key!!!"},
		{"base64 wrong length", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"PEM garbage body", "-----BEGIN EC PRIVATE KEY-----\nZ2FyYmFnZQ==\n-----END EC PRIVATE KEY-----"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewJWTAuthorizer(testKeyName, tt.secret); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestNewJWTAuthorizer_UnsupportedKeyType(t *testing.T) {
	// PKCS#8 RSA keys parse but are not a supported CDP key type. Build one
	// via x509 with a small (test-only) RSA key.
	der := genRSAPKCS8(t)
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	_, err := NewJWTAuthorizer(testKeyName, pemStr)
	if err == nil || !strings.Contains(err.Error(), "unsupported private key type") {
		t.Errorf("err = %v, want unsupported key type", err)
	}
}

func TestNewClients_InvalidCredentials(t *testing.T) {
	if _, err := NewClients("https://api.coinbase.com", "key-name", "bogus-secret"); err == nil {
		t.Fatal("expected error for unparseable API secret")
	}
}

func TestNewClients_ValidCredentialsUseJWT(t *testing.T) {
	_, pemStr := genECKeyPEM(t)
	c, err := NewClients("https://api.coinbase.com", testKeyName, pemStr)
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if c.API == nil {
		t.Fatal("API client is nil")
	}
}
