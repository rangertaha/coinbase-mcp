// SPDX-License-Identifier: MIT

package payments

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
// GET /api/v3/brokerage/payment_methods.
const listFixture = `{
  "payment_methods": [
    {
      "id": "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d",
      "type": "ACH",
      "name": "Chase ****4442",
      "currency": "USD",
      "verified": true,
      "allow_buy": true,
      "allow_sell": true,
      "allow_deposit": true,
      "allow_withdraw": false,
      "created_at": "2021-05-31T09:59:59Z",
      "updated_at": "2021-05-31T10:59:59Z"
    },
    {
      "id": "83562370-3e5c-51db-87da-752af5ab9559",
      "type": "COINBASE_FIAT_ACCOUNT",
      "name": "Cash (USD)",
      "currency": "USD",
      "verified": false,
      "allow_buy": true,
      "allow_sell": false,
      "allow_deposit": false,
      "allow_withdraw": false
    }
  ]
}`

// getFixture is shaped per
// GET /api/v3/brokerage/payment_methods/{payment_method_id}.
const getFixture = `{
  "payment_method": {
    "id": "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d",
    "type": "ACH",
    "name": "Chase ****4442",
    "currency": "USD",
    "verified": true,
    "allow_buy": true,
    "allow_sell": true,
    "allow_deposit": true,
    "allow_withdraw": false,
    "created_at": "2021-05-31T09:59:59Z",
    "updated_at": "2021-05-31T10:59:59Z"
  }
}`

// newTestService returns a payments service backed by a httptest server.
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

func TestListPaymentMethods(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, listFixture)
	})

	out, err := svc.ListPaymentMethods(context.Background())
	if err != nil {
		t.Fatalf("ListPaymentMethods: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/payment_methods" {
		t.Errorf("path = %q", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}

	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	ach := out[0]
	if ach.ID != "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" || ach.Type != "ACH" ||
		ach.Name != "Chase ****4442" || ach.Currency != "USD" || !ach.Verified ||
		!ach.AllowBuy || !ach.AllowSell || !ach.AllowDeposit || ach.AllowWithdraw ||
		ach.CreatedAt != "2021-05-31T09:59:59Z" || ach.UpdatedAt != "2021-05-31T10:59:59Z" {
		t.Errorf("ACH method decoded wrong: %+v", ach)
	}
	if fiat := out[1]; fiat.ID != "83562370-3e5c-51db-87da-752af5ab9559" ||
		fiat.Verified || !fiat.AllowBuy || fiat.AllowSell {
		t.Errorf("fiat method decoded wrong: %+v", fiat)
	}
}

func TestListPaymentMethods_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})
	_, err := svc.ListPaymentMethods(context.Background())
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want *APIError with 401", err)
	}
	if apiErr.Message != "missing credentials" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestGetPaymentMethod(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, getFixture)
	})

	pm, err := svc.GetPaymentMethod(context.Background(), "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d")
	if err != nil {
		t.Fatalf("GetPaymentMethod: %v", err)
	}
	if gotPath != "/api/v3/brokerage/payment_methods/1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" {
		t.Errorf("path = %q", gotPath)
	}
	if pm.ID != "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" || pm.Type != "ACH" ||
		pm.Name != "Chase ****4442" || !pm.Verified || pm.AllowWithdraw {
		t.Errorf("payment method decoded wrong: %+v", pm)
	}
}

func TestGetPaymentMethod_TrimsWhitespace(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetPaymentMethod(context.Background(), "  1c9d2e26-3158-4f18-a76b-4d2f56be6a3d  "); err != nil {
		t.Fatalf("GetPaymentMethod: %v", err)
	}
	if gotPath != "/api/v3/brokerage/payment_methods/1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" {
		t.Errorf("path = %q, want trimmed ID", gotPath)
	}
}

func TestGetPaymentMethod_EmptyID(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, id := range []string{"", "   "} {
		_, err := svc.GetPaymentMethod(context.Background(), id)
		if err == nil {
			t.Errorf("GetPaymentMethod(%q): expected error", id)
			continue
		}
		if !strings.Contains(err.Error(), "paymentMethodId is required") {
			t.Errorf("GetPaymentMethod(%q) error = %q, want paymentMethodId is required", id, err)
		}
	}
	if called {
		t.Error("empty ID must not reach the API")
	}
}

func TestGetPaymentMethod_IDWithSpecialCharsIsEscapedOnce(t *testing.T) {
	var gotRawPath, gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetPaymentMethod(context.Background(), "abc/def"); err != nil {
		t.Fatalf("GetPaymentMethod: %v", err)
	}
	if gotPath != "/api/v3/brokerage/payment_methods/abc/def" {
		t.Errorf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/api/v3/brokerage/payment_methods/abc%2Fdef" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestGetPaymentMethod_NotFound(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"payment method not found"}`)
	})
	_, err := svc.GetPaymentMethod(context.Background(), "83562370-3e5c-51db-87da-752af5ab9559")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
	if apiErr.Message != "payment method not found" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}
