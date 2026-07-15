// SPDX-License-Identifier: MIT

package coinbase

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// jwtTTL is the validity window Coinbase requires for request JWTs.
const jwtTTL = 2 * time.Minute

// jwtAuthorizer signs each request with a short-lived CDP JWT, the scheme the
// Coinbase Advanced Trade API uses for authenticated endpoints. keyName is the
// CDP API key resource name ("organizations/{org}/apiKeys/{key}"); the private
// key is either ECDSA P-256 (ES256) or Ed25519 (EdDSA), depending on how the
// key was created in the Coinbase Developer Platform.
type jwtAuthorizer struct {
	keyName string
	alg     string // "ES256" or "EdDSA"
	ecKey   *ecdsa.PrivateKey
	edKey   ed25519.PrivateKey
	now     func() time.Time // injectable for tests
}

// NewJWTAuthorizer builds a per-request JWT signer from a CDP API key.
//
// secret accepts the private key in any form Coinbase hands out: an EC PRIVATE
// KEY or PRIVATE KEY (PKCS#8) PEM block — with real or literal "\n" newlines —
// or a base64-encoded Ed25519 private key.
func NewJWTAuthorizer(keyName, secret string) (*jwtAuthorizer, error) {
	a := &jwtAuthorizer{keyName: keyName, now: time.Now}

	secret = strings.TrimSpace(strings.ReplaceAll(secret, `\n`, "\n"))
	if block, _ := pem.Decode([]byte(secret)); block != nil {
		key, err := parsePEMKey(block)
		if err != nil {
			return nil, err
		}
		switch k := key.(type) {
		case *ecdsa.PrivateKey:
			a.alg, a.ecKey = "ES256", k
		case ed25519.PrivateKey:
			a.alg, a.edKey = "EdDSA", k
		}
		return a, nil
	}

	// Not PEM: Coinbase distributes Ed25519 keys as base64 of the raw 64-byte
	// private key (seed || public key); a bare 32-byte seed is also accepted,
	// matching the official Python SDK. Interior whitespace is stripped since
	// env plumbing sometimes wraps long values.
	raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(secret), ""))
	if err != nil {
		return nil, errors.New("API secret is neither a PEM private key nor base64: check COINBASE_API_SECRET")
	}
	switch len(raw) {
	case ed25519.PrivateKeySize:
		a.edKey = ed25519.PrivateKey(raw)
	case ed25519.SeedSize:
		a.edKey = ed25519.NewKeyFromSeed(raw)
	default:
		return nil, fmt.Errorf("base64 API secret decodes to %d bytes, want %d or %d (Ed25519 private key or seed)", len(raw), ed25519.PrivateKeySize, ed25519.SeedSize)
	}
	a.alg = "EdDSA"
	return a, nil
}

// parsePEMKey extracts a supported private key from a PEM block.
func parsePEMKey(block *pem.Block) (any, error) {
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PEM private key: %w", err)
	}
	switch key.(type) {
	case *ecdsa.PrivateKey, ed25519.PrivateKey:
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T: CDP keys are ECDSA P-256 or Ed25519", key)
	}
}

// Authorize signs a JWT for this specific request and sets it as a bearer
// token. Coinbase binds the token to the request via the "uri" claim
// (METHOD host/path), so a fresh token is minted per call.
func (a *jwtAuthorizer) Authorize(r *http.Request) error {
	uri := fmt.Sprintf("%s %s%s", r.Method, r.URL.Host, r.URL.Path)
	token, err := a.sign(uri)
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// sign builds and signs the JWT for the given uri claim.
func (a *jwtAuthorizer) sign(uri string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	header, err := json.Marshal(map[string]any{
		"alg":   a.alg,
		"kid":   a.keyName,
		"nonce": hex.EncodeToString(nonce),
		"typ":   "JWT",
	})
	if err != nil {
		return "", err
	}
	now := a.now()
	claims, err := json.Marshal(map[string]any{
		"iss": "cdp",
		"sub": a.keyName,
		"nbf": now.Unix(),
		"exp": now.Add(jwtTTL).Unix(),
		"uri": uri,
	})
	if err != nil {
		return "", err
	}

	b64 := base64.RawURLEncoding
	signingInput := b64.EncodeToString(header) + "." + b64.EncodeToString(claims)

	var sig []byte
	switch a.alg {
	case "ES256":
		digest := sha256.Sum256([]byte(signingInput))
		rr, ss, err := ecdsa.Sign(rand.Reader, a.ecKey, digest[:])
		if err != nil {
			return "", fmt.Errorf("signing JWT: %w", err)
		}
		// JOSE ES256 signatures are the raw r||s values, each left-padded to
		// the 32-byte curve size (not ASN.1 DER).
		sig = make([]byte, 64)
		rr.FillBytes(sig[:32])
		ss.FillBytes(sig[32:])
	case "EdDSA":
		sig = ed25519.Sign(a.edKey, []byte(signingInput))
	default:
		return "", fmt.Errorf("unsupported JWT algorithm %q", a.alg)
	}
	return signingInput + "." + b64.EncodeToString(sig), nil
}
