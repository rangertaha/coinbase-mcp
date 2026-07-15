// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/rangertaha/coinbase-mcp/internal/config"
)

// permissionsAPI returns a stub Coinbase API URL whose key_permissions
// endpoint reports the given trade capability.
func permissionsAPI(t *testing.T, canTrade bool) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/brokerage/key_permissions" {
			t.Errorf("unexpected startup request: %s", r.URL.Path)
		}
		body := `{"can_view": true, "can_trade": false, "can_transfer": false}`
		if canTrade {
			body = `{"can_view": true, "can_trade": true, "can_transfer": true}`
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// testCredentials returns a CDP-style key name and a valid EC private key PEM,
// so Assemble can build the JWT authorizer.
func testCredentials(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
	return "organizations/test/apiKeys/test", pemStr
}

func TestAssemble_UnauthenticatedRegistersPublicOnly(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.coinbase.com"}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if got := srv.Toolsets(); !slices.Equal(got, []string{"products"}) {
		t.Errorf("Toolsets = %v, want [products] without credentials", got)
	}
	if srv.ToolCount() != 6 {
		t.Errorf("ToolCount = %d, want 6 public tools", srv.ToolCount())
	}
	if srv.PromptCount() != 1 {
		t.Errorf("PromptCount = %d, want 1", srv.PromptCount())
	}
}

func TestAssemble_AuthenticatedRegistersEverything(t *testing.T) {
	key, secret := testCredentials(t)
	cfg := &config.Config{BaseURL: permissionsAPI(t, true), APIKey: key, APISecret: secret}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	got := srv.Toolsets()
	want := make([]string, 0)
	for _, ts := range toolsets() {
		if !slices.Contains(want, ts.Name) { // products registers in two halves
			want = append(want, ts.Name)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("Toolsets = %v, want all of %v", got, want)
	}
	if srv.ToolCount() <= 6 {
		t.Errorf("ToolCount = %d, want more than the 6 public tools", srv.ToolCount())
	}
}

func TestAssemble_InvalidCredentials(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.coinbase.com", APIKey: "key", APISecret: "not-a-real-key"}
	if _, _, err := Assemble(context.Background(), cfg, "v-test"); err == nil {
		t.Fatal("expected error for unparseable API secret")
	}
}

func TestAssemble_ToolsetFiltering(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.coinbase.com", Toolsets: []string{"nonexistent"}}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if srv.ToolCount() != 0 {
		t.Errorf("ToolCount = %d, want 0 with no matching toolsets", srv.ToolCount())
	}
	if len(srv.Toolsets()) != 0 {
		t.Errorf("Toolsets = %v, want none", srv.Toolsets())
	}
}

func TestAssemble_SingleAuthToolset(t *testing.T) {
	key, secret := testCredentials(t)
	cfg := &config.Config{
		BaseURL: permissionsAPI(t, true),
		APIKey:  key, APISecret: secret,
		Toolsets: []string{"accounts"},
	}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if got := srv.Toolsets(); !slices.Equal(got, []string{"accounts"}) {
		t.Errorf("Toolsets = %v, want [accounts]", got)
	}
}

func TestAssemble_ReadOnly(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.coinbase.com", ReadOnly: true}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if !srv.ReadOnly() {
		t.Error("server should be read-only")
	}
	// The products toolset is all read tools, so they all survive.
	if srv.ToolCount() != 6 {
		t.Errorf("ToolCount = %d, want 6", srv.ToolCount())
	}
}

func TestAssemble_InvalidBaseURL(t *testing.T) {
	cfg := &config.Config{BaseURL: "://bad"}
	if _, _, err := Assemble(context.Background(), cfg, "v-test"); err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestToolsets(t *testing.T) {
	ts := toolsets()
	if len(ts) == 0 || ts[0].Name != "products" || ts[0].Auth {
		t.Fatalf("first toolset must be the public products set, got %+v", ts)
	}
	// Uniqueness is per (name, auth) pair: "products" intentionally has a
	// public half and an authenticated half.
	type key struct {
		name string
		auth bool
	}
	seen := map[key]bool{}
	for _, t2 := range ts {
		if t2.Register == nil {
			t.Errorf("toolset %q has nil Register", t2.Name)
		}
		k := key{t2.Name, t2.Auth}
		if seen[k] {
			t.Errorf("duplicate toolset entry %+v", k)
		}
		seen[k] = true
		if t2.Name != "products" && !t2.Auth {
			t.Errorf("toolset %q must be marked Auth", t2.Name)
		}
	}
}

func TestAssemble_ScopeFilteringForcesReadOnly(t *testing.T) {
	key, secret := testCredentials(t)
	cfg := &config.Config{BaseURL: permissionsAPI(t, false), APIKey: key, APISecret: secret}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if !srv.ReadOnly() {
		t.Error("a can_trade=false key must force read-only")
	}
}

func TestAssemble_ScopeLookupFailureKeepsConfiguredPolicy(t *testing.T) {
	key, secret := testCredentials(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(api.Close)
	cfg := &config.Config{BaseURL: api.URL, APIKey: key, APISecret: secret}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if srv.ReadOnly() {
		t.Error("lookup failure must not force read-only")
	}
}

func TestAssemble_ReadOnlySkipsScopeLookup(t *testing.T) {
	key, secret := testCredentials(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("read-only server must not call the API at startup")
	}))
	t.Cleanup(api.Close)
	cfg := &config.Config{BaseURL: api.URL, APIKey: key, APISecret: secret, ReadOnly: true}
	if _, cleanup, err := Assemble(context.Background(), cfg, "v-test"); err != nil {
		t.Fatalf("Assemble: %v", err)
	} else {
		cleanup()
	}
}

func TestAssemble_ToolAllowlist(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "https://api.coinbase.com",
		Tools:   []string{"products_get", "products_time"},
	}
	srv, cleanup, err := Assemble(context.Background(), cfg, "v-test")
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	defer cleanup()

	if srv.ToolCount() != 2 {
		t.Errorf("ToolCount = %d, want 2 allowlisted tools", srv.ToolCount())
	}
}
