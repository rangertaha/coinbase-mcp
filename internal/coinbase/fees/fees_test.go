// SPDX-License-Identifier: MIT

package fees

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

// summaryFixture is shaped per the Advanced Trade API spec for
// GET /api/v3/brokerage/transaction_summary, including fields the Summary
// struct intentionally drops (decoding must ignore them). Volume/fee totals
// are JSON numbers; fee-tier fields are strings.
const summaryFixture = `{
  "total_volume": 1000.5,
  "total_fees": 25.5,
  "fee_tier": {
    "pricing_tier": "Advanced 1",
    "usd_from": "0",
    "usd_to": "1000",
    "taker_fee_rate": "0.008",
    "maker_fee_rate": "0.006",
    "aop_from": "0",
    "aop_to": "10000"
  },
  "margin_rate": {"value": "0.02"},
  "goods_and_services_tax": {"rate": "0.1", "type": "INCLUSIVE"},
  "advanced_trade_only_volume": 800.25,
  "advanced_trade_only_fees": 20.5,
  "coinbase_pro_volume": 200.25,
  "coinbase_pro_fees": 5.0,
  "total_balance": "5000",
  "has_cost_plus_commission": false
}`

// newTestService returns a fees service backed by a httptest server.
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

func TestGetSummary(t *testing.T) {
	var gotMethod, gotPath string
	var gotQuery map[string][]string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, summaryFixture)
	})

	out, err := svc.GetSummary(context.Background(), "FUTURE", "FCM", "PERPETUAL")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/transaction_summary" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["product_type"]; len(got) != 1 || got[0] != "FUTURE" {
		t.Errorf("product_type = %v, want [FUTURE]", got)
	}
	if got := gotQuery["product_venue"]; len(got) != 1 || got[0] != "FCM" {
		t.Errorf("product_venue = %v, want [FCM]", got)
	}
	if got := gotQuery["contract_expiry_type"]; len(got) != 1 || got[0] != "PERPETUAL" {
		t.Errorf("contract_expiry_type = %v, want [PERPETUAL]", got)
	}

	if out.TotalVolume != 1000.5 || out.TotalFees != 25.5 {
		t.Errorf("totals = %v/%v, want 1000.5/25.5", out.TotalVolume, out.TotalFees)
	}
	ft := out.FeeTier
	if ft.PricingTier != "Advanced 1" || ft.USDFrom != "0" || ft.USDTo != "1000" ||
		ft.TakerFeeRate != "0.008" || ft.MakerFeeRate != "0.006" ||
		ft.AOPFrom != "0" || ft.AOPTo != "10000" {
		t.Errorf("fee_tier decoded wrong: %+v", ft)
	}
	if out.MarginRate == nil || out.MarginRate.Value != "0.02" {
		t.Errorf("margin_rate = %+v, want value 0.02", out.MarginRate)
	}
	if out.GoodsAndServicesTax == nil || out.GoodsAndServicesTax.Rate != "0.1" ||
		out.GoodsAndServicesTax.Type != "INCLUSIVE" {
		t.Errorf("goods_and_services_tax = %+v", out.GoodsAndServicesTax)
	}
	if out.AdvancedTradeOnlyVolume != 800.25 || out.AdvancedTradeOnlyFees != 20.5 ||
		out.CoinbaseProVolume != 200.25 || out.CoinbaseProFees != 5.0 {
		t.Errorf("breakdown decoded wrong: %+v", out)
	}
}

func TestGetSummary_NoFilters(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"total_volume":0,"total_fees":0,"fee_tier":{}}`)
	})

	out, err := svc.GetSummary(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if out.TotalVolume != 0 || out.MarginRate != nil || out.GoodsAndServicesTax != nil {
		t.Errorf("summary = %+v, want zero values", out)
	}
}

func TestGetSummary_BlankFiltersIgnored(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"total_volume":0,"total_fees":0,"fee_tier":{}}`)
	})
	if _, err := svc.GetSummary(context.Background(), "  ", " ", "\t"); err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("blank filters sent to API: %q", gotRawQuery)
	}
}

func TestGetSummary_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"invalid product_type"}`)
	})
	_, err := svc.GetSummary(context.Background(), "BOGUS", "", "")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError with 400", err)
	}
	if apiErr.Message != "invalid product_type" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}
