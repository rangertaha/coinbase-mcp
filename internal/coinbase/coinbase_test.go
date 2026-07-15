// SPDX-License-Identifier: MIT

package coinbase

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClients_Unauthenticated(t *testing.T) {
	c, err := NewClients("https://api.coinbase.com", "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if c.API == nil {
		t.Fatal("API client is nil")
	}
}

func TestNewClients_InvalidBaseURL(t *testing.T) {
	if _, err := NewClients("://bad", "", ""); err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestCheck(t *testing.T) {
	var gotPath, gotRawQuery, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		gotUA = r.Header.Get("User-Agent")
		// Response shape per GET /api/v3/brokerage/market/products.
		_, _ = io.WriteString(w, `{"products":[{"product_id":"BTC-USD"}],"num_products":742}`)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClients(srv.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	n, err := Check(context.Background(), c)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if n != 742 {
		t.Errorf("num products = %d, want 742", n)
	}
	if gotPath != "/api/v3/brokerage/market/products" {
		t.Errorf("path = %q", gotPath)
	}
	// num_products counts products in the response, so the check must request
	// the unfiltered list — any limit would be echoed back as the "total".
	if gotRawQuery != "" {
		t.Errorf("query = %q, want unfiltered request", gotRawQuery)
	}
	if gotUA != "coinbase-mcp" {
		t.Errorf("User-Agent = %q, want coinbase-mcp", gotUA)
	}
}

func TestCheck_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"message":"service unavailable"}`)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClients(srv.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if _, err := Check(context.Background(), c); err == nil {
		t.Fatal("expected error from 503")
	}
}
