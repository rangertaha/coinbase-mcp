// SPDX-License-Identifier: MIT

package products

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
// GET /api/v3/brokerage/market/products, including fields the Product struct
// intentionally drops (decoding must ignore them).
const listFixture = `{
  "products": [
    {
      "product_id": "BTC-USD",
      "price": "67123.45",
      "price_percentage_change_24h": "1.25",
      "volume_24h": "12345.678",
      "volume_percentage_change_24h": "-2.5",
      "base_increment": "0.00000001",
      "quote_increment": "0.01",
      "quote_min_size": "1",
      "quote_max_size": "150000000",
      "base_name": "Bitcoin",
      "quote_name": "US Dollar",
      "watched": false,
      "is_disabled": false,
      "new": false,
      "status": "online",
      "cancel_only": false,
      "limit_only": false,
      "post_only": false,
      "trading_disabled": false,
      "auction_mode": false,
      "product_type": "SPOT",
      "quote_currency_id": "USD",
      "base_currency_id": "BTC"
    },
    {
      "product_id": "ETH-USD",
      "price": "3456.78",
      "price_percentage_change_24h": "-0.5",
      "volume_24h": "98765.4",
      "base_name": "Ethereum",
      "quote_name": "US Dollar",
      "status": "online",
      "trading_disabled": true,
      "product_type": "SPOT"
    }
  ],
  "num_products": 2
}`

// getFixture is shaped per GET /api/v3/brokerage/market/products/{product_id}.
const getFixture = `{
  "product_id": "BTC-USD",
  "price": "67123.45",
  "price_percentage_change_24h": "1.25",
  "volume_24h": "12345.678",
  "base_name": "Bitcoin",
  "quote_name": "US Dollar",
  "status": "online",
  "trading_disabled": false,
  "product_type": "SPOT"
}`

// newTestService returns a products service backed by a httptest server.
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

func TestListProducts(t *testing.T) {
	var gotPath string
	var gotQuery map[string][]string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, listFixture)
	})

	out, err := svc.ListProducts(context.Background(), "SPOT", 2)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["product_type"]; len(got) != 1 || got[0] != "SPOT" {
		t.Errorf("product_type = %v, want [SPOT]", got)
	}
	if got := gotQuery["limit"]; len(got) != 1 || got[0] != "2" {
		t.Errorf("limit = %v, want [2]", got)
	}

	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	btc := out[0]
	if btc.ProductID != "BTC-USD" || btc.Price != "67123.45" || btc.PricePctChg24 != "1.25" ||
		btc.Volume24h != "12345.678" || btc.BaseName != "Bitcoin" || btc.QuoteName != "US Dollar" ||
		btc.Status != "online" || btc.TradingDisabled {
		t.Errorf("BTC product decoded wrong: %+v", btc)
	}
	if eth := out[1]; eth.ProductID != "ETH-USD" || !eth.TradingDisabled {
		t.Errorf("ETH product decoded wrong: %+v", eth)
	}
}

func TestListProducts_NoFilters(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"products":[],"num_products":0}`)
	})

	out, err := svc.ListProducts(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if len(out) != 0 {
		t.Errorf("len = %d, want 0", len(out))
	}
}

func TestListProducts_NegativeLimitIgnored(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"products":[],"num_products":0}`)
	})
	if _, err := svc.ListProducts(context.Background(), "", -5); err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if strings.Contains(gotRawQuery, "limit") {
		t.Errorf("negative limit sent to API: %q", gotRawQuery)
	}
}

func TestListProducts_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"invalid product_type"}`)
	})
	_, err := svc.ListProducts(context.Background(), "BOGUS", 0)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError with 400", err)
	}
	if apiErr.Message != "invalid product_type" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestGetProduct(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, getFixture)
	})

	p, err := svc.GetProduct(context.Background(), "BTC-USD")
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products/BTC-USD" {
		t.Errorf("path = %q", gotPath)
	}
	if p.ProductID != "BTC-USD" || p.Price != "67123.45" || p.BaseName != "Bitcoin" {
		t.Errorf("product decoded wrong: %+v", p)
	}
}

func TestGetProduct_TrimsWhitespace(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetProduct(context.Background(), "  BTC-USD  "); err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products/BTC-USD" {
		t.Errorf("path = %q, want trimmed ID", gotPath)
	}
}

func TestGetProduct_EmptyID(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, id := range []string{"", "   "} {
		if _, err := svc.GetProduct(context.Background(), id); err == nil {
			t.Errorf("GetProduct(%q): expected error", id)
		}
	}
	if called {
		t.Error("empty ID must not reach the API")
	}
}

func TestGetProduct_IDWithSpecialCharsIsEscapedOnce(t *testing.T) {
	var gotRawPath, gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetProduct(context.Background(), "BTC/USD"); err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products/BTC/USD" {
		t.Errorf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/api/v3/brokerage/market/products/BTC%2FUSD" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestGetProduct_NotFound(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"product not found"}`)
	})
	_, err := svc.GetProduct(context.Background(), "NOPE-USD")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
}
