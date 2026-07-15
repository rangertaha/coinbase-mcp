// SPDX-License-Identifier: MIT

package accounts

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rangertaha/coinbase-mcp/internal/client"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// listFixture is shaped per the Advanced Trade API spec for
// GET /api/v3/brokerage/accounts, including fields the Account struct
// intentionally drops (decoding must ignore them).
const listFixture = `{
  "accounts": [
    {
      "uuid": "8bfc20d7-f7c6-4422-bf07-8243ca4169fe",
      "name": "BTC Wallet",
      "currency": "BTC",
      "available_balance": {"value": "1.23", "currency": "BTC"},
      "default": true,
      "active": true,
      "created_at": "2021-05-31T09:59:59Z",
      "updated_at": "2021-05-31T10:59:59Z",
      "deleted_at": null,
      "type": "ACCOUNT_TYPE_CRYPTO",
      "ready": true,
      "hold": {"value": "0.5", "currency": "BTC"},
      "retail_portfolio_id": "b87cf1e6-9a1a-4a3b-9a26-27d0a9f4c8e1",
      "platform": "ACCOUNT_PLATFORM_CONSUMER"
    },
    {
      "uuid": "b6ee36f9-2b34-40ba-8a05-cfba2b3a2c9c",
      "name": "USD Wallet",
      "currency": "USD",
      "available_balance": {"value": "1000.00", "currency": "USD"},
      "default": false,
      "active": false,
      "type": "ACCOUNT_TYPE_FIAT",
      "ready": false,
      "hold": {"value": "0", "currency": "USD"}
    }
  ],
  "has_next": true,
  "cursor": "789100",
  "size": 2
}`

// getFixture is shaped per GET /api/v3/brokerage/accounts/{account_uuid}.
const getFixture = `{
  "account": {
    "uuid": "8bfc20d7-f7c6-4422-bf07-8243ca4169fe",
    "name": "BTC Wallet",
    "currency": "BTC",
    "available_balance": {"value": "1.23", "currency": "BTC"},
    "default": true,
    "active": true,
    "created_at": "2021-05-31T09:59:59Z",
    "updated_at": "2021-05-31T10:59:59Z",
    "type": "ACCOUNT_TYPE_CRYPTO",
    "ready": true,
    "hold": {"value": "0.5", "currency": "BTC"},
    "retail_portfolio_id": "b87cf1e6-9a1a-4a3b-9a26-27d0a9f4c8e1"
  }
}`

// newTestService returns an accounts service backed by a httptest server.
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

func TestListAccounts(t *testing.T) {
	var gotMethod, gotPath string
	var gotQuery map[string][]string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, listFixture)
	})

	out, err := svc.ListAccounts(context.Background(), 2, "abc123")
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/accounts" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["limit"]; len(got) != 1 || got[0] != "2" {
		t.Errorf("limit = %v, want [2]", got)
	}
	if got := gotQuery["cursor"]; len(got) != 1 || got[0] != "abc123" {
		t.Errorf("cursor = %v, want [abc123]", got)
	}

	if !out.HasNext || out.Cursor != "789100" || out.Size != 2 {
		t.Errorf("page state = has_next=%v cursor=%q size=%d", out.HasNext, out.Cursor, out.Size)
	}
	if len(out.Accounts) != 2 {
		t.Fatalf("len = %d, want 2", len(out.Accounts))
	}
	btc := out.Accounts[0]
	if btc.UUID != "8bfc20d7-f7c6-4422-bf07-8243ca4169fe" || btc.Name != "BTC Wallet" ||
		btc.Currency != "BTC" || !btc.Default || !btc.Active || !btc.Ready ||
		btc.CreatedAt != "2021-05-31T09:59:59Z" || btc.UpdatedAt != "2021-05-31T10:59:59Z" ||
		btc.Type != "ACCOUNT_TYPE_CRYPTO" ||
		btc.RetailPortfolioID != "b87cf1e6-9a1a-4a3b-9a26-27d0a9f4c8e1" {
		t.Errorf("BTC account decoded wrong: %+v", btc)
	}
	if btc.AvailableBalance != (Amount{Value: "1.23", Currency: "BTC"}) {
		t.Errorf("available_balance = %+v", btc.AvailableBalance)
	}
	if btc.Hold != (Amount{Value: "0.5", Currency: "BTC"}) {
		t.Errorf("hold = %+v", btc.Hold)
	}
	if usd := out.Accounts[1]; usd.UUID != "b6ee36f9-2b34-40ba-8a05-cfba2b3a2c9c" ||
		usd.Default || usd.Active || usd.Ready ||
		usd.AvailableBalance != (Amount{Value: "1000.00", Currency: "USD"}) {
		t.Errorf("USD account decoded wrong: %+v", usd)
	}
}

func TestListAccounts_NoParams(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"accounts":[],"has_next":false,"cursor":"","size":0}`)
	})

	out, err := svc.ListAccounts(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if len(out.Accounts) != 0 || out.HasNext || out.Cursor != "" {
		t.Errorf("page = %+v, want empty", out)
	}
}

func TestListAccounts_NegativeLimitAndBlankCursorIgnored(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"accounts":[],"has_next":false,"cursor":"","size":0}`)
	})
	if _, err := svc.ListAccounts(context.Background(), -5, "   "); err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("negative limit or blank cursor sent to API: %q", gotRawQuery)
	}
}

func TestListAccounts_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})
	_, err := svc.ListAccounts(context.Background(), 0, "")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want *APIError with 401", err)
	}
	if apiErr.Message != "missing credentials" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestGetAccount(t *testing.T) {
	var gotMethod, gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, getFixture)
	})

	a, err := svc.GetAccount(context.Background(), "8bfc20d7-f7c6-4422-bf07-8243ca4169fe")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/accounts/8bfc20d7-f7c6-4422-bf07-8243ca4169fe" {
		t.Errorf("path = %q", gotPath)
	}
	if a.UUID != "8bfc20d7-f7c6-4422-bf07-8243ca4169fe" || a.Name != "BTC Wallet" ||
		a.Currency != "BTC" || !a.Default || !a.Active || !a.Ready ||
		a.Type != "ACCOUNT_TYPE_CRYPTO" ||
		a.AvailableBalance != (Amount{Value: "1.23", Currency: "BTC"}) ||
		a.Hold != (Amount{Value: "0.5", Currency: "BTC"}) {
		t.Errorf("account decoded wrong: %+v", a)
	}
}

func TestGetAccount_TrimsWhitespace(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetAccount(context.Background(), "  8bfc20d7-f7c6-4422-bf07-8243ca4169fe  "); err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if gotPath != "/api/v3/brokerage/accounts/8bfc20d7-f7c6-4422-bf07-8243ca4169fe" {
		t.Errorf("path = %q, want trimmed UUID", gotPath)
	}
}

func TestGetAccount_EmptyID(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, id := range []string{"", "   "} {
		_, err := svc.GetAccount(context.Background(), id)
		if err == nil {
			t.Errorf("GetAccount(%q): expected error", id)
			continue
		}
		if !strings.Contains(err.Error(), "accountId is required") {
			t.Errorf("GetAccount(%q) error = %q, want accountId is required", id, err)
		}
	}
	if called {
		t.Error("empty ID must not reach the API")
	}
}

func TestGetAccount_IDWithSpecialCharsIsEscapedOnce(t *testing.T) {
	var gotRawPath, gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetAccount(context.Background(), "abc/def"); err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if gotPath != "/api/v3/brokerage/accounts/abc/def" {
		t.Errorf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/api/v3/brokerage/accounts/abc%2Fdef" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestGetAccount_NotFound(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"account not found"}`)
	})
	_, err := svc.GetAccount(context.Background(), "b6ee36f9-2b34-40ba-8a05-cfba2b3a2c9c")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
	if apiErr.Message != "account not found" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}
