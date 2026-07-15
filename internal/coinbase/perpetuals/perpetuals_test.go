// SPDX-License-Identifier: MIT

package perpetuals

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rangertaha/coinbase-mcp/internal/client"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

const testPortfolioID = "1f4b2b9e-2f47-4c9a-8f0e-2f0f9d1c1234"

// Fixtures shaped per the Advanced Trade INTX endpoints, including fields the
// trimmed structs intentionally drop (decoding must ignore them).
const (
	// The endpoint returns an ARRAY under "portfolios" (per the official
	// Python SDK), even though it addresses a single portfolio.
	portfolioFixture = `{
  "portfolios": [
    {
      "portfolio_uuid": "1f4b2b9e-2f47-4c9a-8f0e-2f0f9d1c1234",
      "collateral": "10000.00",
      "position_notional": "2500.00",
      "open_position_notional": "2500.00",
      "pending_fees": "1.25",
      "borrow": "0",
      "accrued_interest": "0",
      "rolling_debt": "0",
      "portfolio_initial_margin": 0.2,
      "portfolio_im_notional": {"value": "500.00", "currency": "USDC"},
      "portfolio_maintenance_margin": 0.1,
      "portfolio_mm_notional": {"value": "250.00", "currency": "USDC"},
      "liquidation_percentage": "35.5",
      "liquidation_buffer": "9750.00",
      "margin_type": "MARGIN_TYPE_CROSS",
      "margin_flags": "PORTFOLIO_MARGIN_FLAGS_UNSPECIFIED",
      "liquidation_status": "NOT_LIQUIDATING",
      "unrealized_pnl": {"value": "12.34", "currency": "USDC"},
      "total_balance": {"value": "10012.34", "currency": "USDC"}
    }
  ]
}`
	positionsFixture = `{
  "positions": [
    {
      "product_id": "BTC-PERP-INTX",
      "symbol": "BTC-PERP-INTX",
      "position_side": "POSITION_SIDE_LONG",
      "net_size": "0.05",
      "buy_order_size": "0",
      "sell_order_size": "0",
      "im_contribution": "0.2",
      "leverage": "5",
      "margin_type": "MARGIN_TYPE_CROSS",
      "unrealized_pnl": {"value": "25.50", "currency": "USDC"},
      "mark_price": {"value": "64500.00", "currency": "USDC"},
      "liquidation_price": {"value": "51600.00", "currency": "USDC"}
    },
    {
      "product_id": "ETH-PERP-INTX",
      "symbol": "ETH-PERP-INTX",
      "position_side": "POSITION_SIDE_SHORT",
      "net_size": "-1.5",
      "leverage": "3",
      "margin_type": "MARGIN_TYPE_ISOLATED",
      "unrealized_pnl": {"value": "-4.20", "currency": "USDC"},
      "mark_price": {"value": "3456.78", "currency": "USDC"},
      "liquidation_price": {"value": "4600.00", "currency": "USDC"}
    }
  ]
}`
	positionFixture = `{
  "position": {
    "product_id": "BTC-PERP-INTX",
    "symbol": "BTC-PERP-INTX",
    "position_side": "POSITION_SIDE_LONG",
    "net_size": "0.05",
    "leverage": "5",
    "margin_type": "MARGIN_TYPE_CROSS",
    "unrealized_pnl": {"value": "25.50", "currency": "USDC"},
    "mark_price": {"value": "64500.00", "currency": "USDC"},
    "liquidation_price": {"value": "51600.00", "currency": "USDC"}
  }
}`
	balancesFixture = `{
  "portfolio_balances": [
    {
      "portfolio_uuid": "1f4b2b9e-2f47-4c9a-8f0e-2f0f9d1c1234",
      "balances": [
        {
          "asset": {"asset_id": "usdc-id", "asset_name": "USDC", "asset_uuid": "ignored"},
          "quantity": "10000.00",
          "hold": "500.00",
          "collateral_value": "10000.00",
          "max_withdraw_amount": "9500.00",
          "loan": "0"
        },
        {
          "asset": {"asset_id": "eth-id", "asset_name": "ETH"},
          "quantity": "2.5"
        }
      ],
      "is_margin_limit_reached": true
    }
  ]
}`
	errorFixture = `{"error": "INVALID_ARGUMENT", "message": "bad request"}`
)

// newTestService returns a perpetuals service backed by a httptest server.
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

// errorService returns a service whose API always answers with the given
// status and the standard error envelope.
func errorService(t *testing.T, status int) *service {
	t.Helper()
	return newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, errorFixture)
	})
}

// wantAPIError asserts err is a *client.APIError with the given status.
func wantAPIError(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
		t.Fatalf("err = %v, want *APIError with %d", err, status)
	}
	if apiErr.Message != "bad request" {
		t.Errorf("Message = %q, want API message", apiErr.Message)
	}
}

func TestGetPortfolio(t *testing.T) {
	var gotPath, gotMethod string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_, _ = io.WriteString(w, portfolioFixture)
	})

	out, err := svc.GetPortfolio(context.Background(), "  "+testPortfolioID+"  ")
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v3/brokerage/intx/portfolio/"+testPortfolioID {
		t.Errorf("request = %s %s, want trimmed ID in path", gotMethod, gotPath)
	}
	if out.PortfolioUUID != testPortfolioID || out.Collateral != "10000.00" ||
		out.PositionNotional != "2500.00" || out.OpenPositionNotional != "2500.00" ||
		out.PendingFees != "1.25" || out.Borrow != "0" || out.AccruedInterest != "0" ||
		out.RollingDebt != "0" || out.LiquidationPercentage != "35.5" ||
		out.LiquidationBuffer != "9750.00" || out.MarginType != "MARGIN_TYPE_CROSS" ||
		out.LiquidationStatus != "NOT_LIQUIDATING" {
		t.Errorf("portfolio decoded wrong: %+v", out)
	}
	if out.InitialMargin != "0.2" || out.MaintenanceMargin != "0.1" {
		t.Errorf("margin ratios = %v/%v, want 0.2/0.1", out.InitialMargin, out.MaintenanceMargin)
	}
	if out.IMNotional != (Amount{Value: "500.00", Currency: "USDC"}) ||
		out.MMNotional != (Amount{Value: "250.00", Currency: "USDC"}) ||
		out.UnrealizedPnL != (Amount{Value: "12.34", Currency: "USDC"}) ||
		out.TotalBalance != (Amount{Value: "10012.34", Currency: "USDC"}) {
		t.Errorf("amounts decoded wrong: %+v", out)
	}
}

func TestGetPortfolio_EmptyID(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, id := range []string{"", "   "} {
		if _, err := svc.GetPortfolio(context.Background(), id); err == nil {
			t.Errorf("GetPortfolio(%q): expected error", id)
		}
	}
	if called {
		t.Error("empty ID must not reach the API")
	}
}

func TestGetPortfolio_APIError(t *testing.T) {
	svc := errorService(t, http.StatusNotFound)
	_, err := svc.GetPortfolio(context.Background(), testPortfolioID)
	wantAPIError(t, err, http.StatusNotFound)
}

func TestListPositions(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, positionsFixture)
	})

	out, err := svc.ListPositions(context.Background(), testPortfolioID)
	if err != nil {
		t.Fatalf("ListPositions: %v", err)
	}
	if gotPath != "/api/v3/brokerage/intx/positions/"+testPortfolioID {
		t.Errorf("path = %q", gotPath)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	p := out[0]
	if p.ProductID != "BTC-PERP-INTX" || p.Symbol != "BTC-PERP-INTX" ||
		p.PositionSide != "POSITION_SIDE_LONG" || p.NetSize != "0.05" || p.Leverage != "5" ||
		p.MarginType != "MARGIN_TYPE_CROSS" ||
		p.UnrealizedPnL != (Amount{Value: "25.50", Currency: "USDC"}) ||
		p.MarkPrice != (Amount{Value: "64500.00", Currency: "USDC"}) ||
		p.LiquidationPrice != (Amount{Value: "51600.00", Currency: "USDC"}) {
		t.Errorf("position decoded wrong: %+v", p)
	}
	if out[1].Symbol != "ETH-PERP-INTX" || out[1].NetSize != "-1.5" {
		t.Errorf("second position decoded wrong: %+v", out[1])
	}
}

func TestListPositions_EmptyID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	if _, err := svc.ListPositions(context.Background(), " "); err == nil {
		t.Fatal("expected error for empty portfolioId")
	}
}

func TestListPositions_APIError(t *testing.T) {
	svc := errorService(t, http.StatusUnauthorized)
	_, err := svc.ListPositions(context.Background(), testPortfolioID)
	wantAPIError(t, err, http.StatusUnauthorized)
}

func TestGetPosition(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, positionFixture)
	})

	p, err := svc.GetPosition(context.Background(), testPortfolioID, " BTC-PERP-INTX ")
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if gotPath != "/api/v3/brokerage/intx/positions/"+testPortfolioID+"/BTC-PERP-INTX" {
		t.Errorf("path = %q, want trimmed symbol", gotPath)
	}
	if p.ProductID != "BTC-PERP-INTX" || p.PositionSide != "POSITION_SIDE_LONG" ||
		p.NetSize != "0.05" || p.MarkPrice.Value != "64500.00" {
		t.Errorf("position decoded wrong: %+v", p)
	}
}

func TestGetPosition_SymbolIsPathEscaped(t *testing.T) {
	var gotRawPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, positionFixture)
	})
	if _, err := svc.GetPosition(context.Background(), testPortfolioID, "BTC/PERP"); err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	want := "/api/v3/brokerage/intx/positions/" + testPortfolioID + "/BTC%2FPERP"
	if gotRawPath != want {
		t.Errorf("raw path = %q, want %q", gotRawPath, want)
	}
}

func TestGetPosition_MissingInputs(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	cases := []struct{ portfolioID, symbol string }{
		{"", "BTC-PERP-INTX"},
		{"  ", "BTC-PERP-INTX"},
		{testPortfolioID, ""},
		{testPortfolioID, "  "},
	}
	for _, c := range cases {
		if _, err := svc.GetPosition(context.Background(), c.portfolioID, c.symbol); err == nil {
			t.Errorf("GetPosition(%q, %q): expected error", c.portfolioID, c.symbol)
		}
	}
	if called {
		t.Error("invalid inputs must not reach the API")
	}
}

func TestGetPosition_APIError(t *testing.T) {
	svc := errorService(t, http.StatusNotFound)
	_, err := svc.GetPosition(context.Background(), testPortfolioID, "NOPE-PERP-INTX")
	wantAPIError(t, err, http.StatusNotFound)
}

func TestGetBalances(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, balancesFixture)
	})

	out, err := svc.GetBalances(context.Background(), testPortfolioID)
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}
	if gotPath != "/api/v3/brokerage/intx/balances/"+testPortfolioID {
		t.Errorf("path = %q", gotPath)
	}
	if out.PortfolioUUID != testPortfolioID || !out.IsMarginLimitReached {
		t.Errorf("balances envelope decoded wrong: %+v", out)
	}
	if len(out.Balances) != 2 {
		t.Fatalf("len(balances) = %d, want 2", len(out.Balances))
	}
	b := out.Balances[0]
	if b.Asset != (Asset{AssetID: "usdc-id", AssetName: "USDC"}) ||
		b.Quantity != "10000.00" || b.Hold != "500.00" || b.CollateralValue != "10000.00" ||
		b.MaxWithdrawAmount != "9500.00" || b.Loan != "0" {
		t.Errorf("balance decoded wrong: %+v", b)
	}
	if out.Balances[1].Asset.AssetName != "ETH" || out.Balances[1].Quantity != "2.5" {
		t.Errorf("second balance decoded wrong: %+v", out.Balances[1])
	}
}

func TestGetBalances_EmptyID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	if _, err := svc.GetBalances(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty portfolioId")
	}
}

func TestGetBalances_APIError(t *testing.T) {
	svc := errorService(t, http.StatusForbidden)
	_, err := svc.GetBalances(context.Background(), testPortfolioID)
	wantAPIError(t, err, http.StatusForbidden)
}

func TestAllocate(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{}`)
	})

	out, err := svc.Allocate(context.Background(), " "+testPortfolioID+" ", " BTC-PERP-INTX ", " 100.00 ", " USDC ")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v3/brokerage/intx/allocate" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	want := map[string]any{
		"portfolio_uuid": testPortfolioID,
		"symbol":         "BTC-PERP-INTX",
		"amount":         "100.00",
		"currency":       "USDC",
	}
	if len(gotBody) != len(want) {
		t.Errorf("body = %v, want %v", gotBody, want)
	}
	for k, v := range want {
		if gotBody[k] != v {
			t.Errorf("body[%s] = %v, want %v", k, gotBody[k], v)
		}
	}
	if !out.Allocated || out.Symbol != "BTC-PERP-INTX" || out.Amount != "100.00" || out.Currency != "USDC" {
		t.Errorf("result = %+v, want allocated confirmation", out)
	}
}

func TestAllocate_MissingInputs(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	cases := []struct {
		name                                  string
		portfolioID, symbol, amount, currency string
	}{
		{"portfolioId", "", "BTC-PERP-INTX", "100.00", "USDC"},
		{"symbol", testPortfolioID, "  ", "100.00", "USDC"},
		{"amount", testPortfolioID, "BTC-PERP-INTX", "", "USDC"},
		{"currency", testPortfolioID, "BTC-PERP-INTX", "100.00", "  "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Allocate(context.Background(), c.portfolioID, c.symbol, c.amount, c.currency)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if called {
		t.Error("invalid inputs must not reach the API")
	}
}

func TestAllocate_APIError(t *testing.T) {
	svc := errorService(t, http.StatusBadRequest)
	_, err := svc.Allocate(context.Background(), testPortfolioID, "BTC-PERP-INTX", "100.00", "USDC")
	wantAPIError(t, err, http.StatusBadRequest)
}

func TestSetMultiAssetCollateral(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"multi_asset_collateral_enabled": true}`)
	})

	out, err := svc.SetMultiAssetCollateral(context.Background(), testPortfolioID, true)
	if err != nil {
		t.Fatalf("SetMultiAssetCollateral: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v3/brokerage/intx/multi_asset_collateral" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["portfolio_uuid"] != testPortfolioID || gotBody["multi_asset_collateral_enabled"] != true {
		t.Errorf("body = %v", gotBody)
	}
	if !out.Enabled {
		t.Errorf("result = %+v, want enabled=true from API response", out)
	}
}

func TestSetMultiAssetCollateral_Disable(t *testing.T) {
	var gotBody map[string]any
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"multi_asset_collateral_enabled": false}`)
	})

	out, err := svc.SetMultiAssetCollateral(context.Background(), testPortfolioID, false)
	if err != nil {
		t.Fatalf("SetMultiAssetCollateral: %v", err)
	}
	if gotBody["multi_asset_collateral_enabled"] != false {
		t.Errorf("body = %v, want enabled=false", gotBody)
	}
	if out.Enabled {
		t.Errorf("result = %+v, want enabled=false from API response", out)
	}
}

func TestSetMultiAssetCollateral_EmptyID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	if _, err := svc.SetMultiAssetCollateral(context.Background(), "  ", true); err == nil {
		t.Fatal("expected error for empty portfolioId")
	}
}

func TestSetMultiAssetCollateral_APIError(t *testing.T) {
	svc := errorService(t, http.StatusBadRequest)
	_, err := svc.SetMultiAssetCollateral(context.Background(), testPortfolioID, true)
	wantAPIError(t, err, http.StatusBadRequest)
}

func TestNumber_TolerantDecoding(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Number
		err  bool
	}{
		{"json number", `0.25`, "0.25", false},
		{"integer", `3`, "3", false},
		{"json string", `"0.25"`, "0.25", false},
		{"null", `null`, "", false},
		{"object rejected", `{"v":1}`, "", true},
		{"array rejected", `[1]`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n Number
			err := n.UnmarshalJSON([]byte(tt.in))
			if (err != nil) != tt.err {
				t.Fatalf("err = %v, want error=%v", err, tt.err)
			}
			if !tt.err && n != tt.want {
				t.Errorf("Number = %q, want %q", n, tt.want)
			}
		})
	}
}

func TestPortfolio_MarginFieldsAcceptStrings(t *testing.T) {
	// The docs are ambiguous between number and string for the margin ratios;
	// both encodings must decode.
	var p Portfolio
	if err := json.Unmarshal([]byte(`{"portfolio_initial_margin":"0.2","portfolio_maintenance_margin":0.1}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.InitialMargin != "0.2" || p.MaintenanceMargin != "0.1" {
		t.Errorf("margins = %q/%q, want 0.2/0.1", p.InitialMargin, p.MaintenanceMargin)
	}
}

func TestGetPortfolio_EmptyArray(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"portfolios": []}`)
	})
	if _, err := svc.GetPortfolio(context.Background(), testPortfolioID); err == nil {
		t.Fatal("expected error for empty portfolios array")
	}
}

func TestGetBalances_EmptyArray(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"portfolio_balances": []}`)
	})
	if _, err := svc.GetBalances(context.Background(), testPortfolioID); err == nil {
		t.Fatal("expected error for empty balances array")
	}
}
