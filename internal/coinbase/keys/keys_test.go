// SPDX-License-Identifier: MIT

package keys

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rangertaha/coinbase-mcp/internal/client"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// permissionsFixture is shaped per the Advanced Trade API spec for
// GET /api/v3/brokerage/key_permissions, including a field the Permissions
// struct intentionally drops (decoding must ignore it).
const permissionsFixture = `{
  "can_view": true,
  "can_trade": true,
  "can_transfer": false,
  "can_receive": true,
  "portfolio_uuid": "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d",
  "portfolio_type": "DEFAULT"
}`

// newTestService returns a keys service backed by a httptest server.
func newTestService(t *testing.T, handler http.HandlerFunc) *service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := coinbase.NewClients(srv.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	return &service{c: c}
}

func TestGetPermissions(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, permissionsFixture)
	})

	p, err := svc.GetPermissions(context.Background())
	if err != nil {
		t.Fatalf("GetPermissions: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/key_permissions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if !p.CanView || !p.CanTrade || p.CanTransfer ||
		p.PortfolioUUID != "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" ||
		p.PortfolioType != "DEFAULT" {
		t.Errorf("permissions decoded wrong: %+v", p)
	}
}

func TestGetPermissions_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})
	_, err := svc.GetPermissions(context.Background())
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want *APIError with 401", err)
	}
	if apiErr.Message != "missing credentials" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}
