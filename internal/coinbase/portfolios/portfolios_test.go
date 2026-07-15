// SPDX-License-Identifier: MIT

package portfolios

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

// listFixture is shaped per the Advanced Trade API spec for
// GET /api/v3/brokerage/portfolios.
const listFixture = `{
  "portfolios": [
    {"name": "Default", "uuid": "11111111-1111-1111-1111-111111111111", "type": "DEFAULT", "deleted": false},
    {"name": "Trading", "uuid": "22222222-2222-2222-2222-222222222222", "type": "CONSUMER", "deleted": true}
  ]
}`

// portfolioFixture is the envelope returned by the create and edit endpoints.
const portfolioFixture = `{
  "portfolio": {"name": "Trading", "uuid": "22222222-2222-2222-2222-222222222222", "type": "CONSUMER", "deleted": false}
}`

// breakdownFixture is shaped per GET /api/v3/brokerage/portfolios/{uuid},
// including the perp/futures position arrays the Breakdown struct
// intentionally drops (decoding must ignore them). Spot position balance
// fields are JSON numbers per the API spec.
const breakdownFixture = `{
  "breakdown": {
    "portfolio": {"name": "Default", "uuid": "11111111-1111-1111-1111-111111111111", "type": "DEFAULT", "deleted": false},
    "portfolio_balances": {
      "total_balance": {"value": "1500.25", "currency": "USD"},
      "total_futures_balance": {"value": "0", "currency": "USD"},
      "total_cash_equivalent_balance": {"value": "500.25", "currency": "USD"},
      "total_crypto_balance": {"value": "1000.00", "currency": "USD"},
      "futures_unrealized_pnl": {"value": "0", "currency": "USD"},
      "perp_unrealized_pnl": {"value": "-12.5", "currency": "USD"}
    },
    "spot_positions": [
      {
        "asset": "BTC",
        "account_uuid": "33333333-3333-3333-3333-333333333333",
        "total_balance_fiat": 1000.0,
        "total_balance_crypto": 0.0155,
        "available_to_trade_fiat": 950.5,
        "allocation": 0.6666,
        "cost_basis": {"value": "800.00", "currency": "USD"},
        "asset_img_url": "https://example.com/btc.png",
        "is_cash": false
      },
      {
        "asset": "USD",
        "account_uuid": "44444444-4444-4444-4444-444444444444",
        "total_balance_fiat": 500.25,
        "total_balance_crypto": 500.25,
        "available_to_trade_fiat": 500.25,
        "allocation": 0.3334,
        "cost_basis": {"value": "500.25", "currency": "USD"},
        "is_cash": true
      }
    ],
    "perp_positions": [{"product_id": "BTC-PERP-INTX"}],
    "futures_positions": [{"product_id": "BIT-31JUL26-CDE"}]
  }
}`

// moveFundsFixture is shaped per POST /api/v3/brokerage/portfolios/move_funds.
const moveFundsFixture = `{
  "source_portfolio_uuid": "11111111-1111-1111-1111-111111111111",
  "target_portfolio_uuid": "22222222-2222-2222-2222-222222222222"
}`

// newTestService returns a portfolios service backed by a httptest server.
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

func TestListPortfolios(t *testing.T) {
	var gotMethod, gotPath string
	var gotQuery map[string][]string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, listFixture)
	})

	out, err := svc.ListPortfolios(context.Background(), "DEFAULT")
	if err != nil {
		t.Fatalf("ListPortfolios: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/portfolios" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["portfolio_type"]; len(got) != 1 || got[0] != "DEFAULT" {
		t.Errorf("portfolio_type = %v, want [DEFAULT]", got)
	}

	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	p := out[0]
	if p.Name != "Default" || p.UUID != "11111111-1111-1111-1111-111111111111" ||
		p.Type != "DEFAULT" || p.Deleted {
		t.Errorf("portfolio decoded wrong: %+v", p)
	}
	if q := out[1]; q.Name != "Trading" || q.Type != "CONSUMER" || !q.Deleted {
		t.Errorf("second portfolio decoded wrong: %+v", q)
	}
}

func TestListPortfolios_NoFilter(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"portfolios":[]}`)
	})

	out, err := svc.ListPortfolios(context.Background(), "  ")
	if err != nil {
		t.Fatalf("ListPortfolios: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if len(out) != 0 {
		t.Errorf("len = %d, want 0", len(out))
	}
}

func TestListPortfolios_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"invalid portfolio_type"}`)
	})
	_, err := svc.ListPortfolios(context.Background(), "BOGUS")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError with 400", err)
	}
	if apiErr.Message != "invalid portfolio_type" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestCreatePortfolio(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody struct {
		Name string `json:"name"`
	}
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, portfolioFixture)
	})

	p, err := svc.CreatePortfolio(context.Background(), "  Trading  ")
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/portfolios" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.Name != "Trading" {
		t.Errorf("body name = %q, want trimmed %q", gotBody.Name, "Trading")
	}
	if p.Name != "Trading" || p.UUID != "22222222-2222-2222-2222-222222222222" ||
		p.Type != "CONSUMER" || p.Deleted {
		t.Errorf("portfolio decoded wrong: %+v", p)
	}
}

func TestCreatePortfolio_EmptyName(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	for _, name := range []string{"", "   "} {
		if _, err := svc.CreatePortfolio(context.Background(), name); err == nil {
			t.Errorf("CreatePortfolio(%q): expected error", name)
		}
	}
}

func TestCreatePortfolio_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"PERMISSION_DENIED","message":"portfolio limit reached"}`)
	})
	_, err := svc.CreatePortfolio(context.Background(), "One Too Many")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v, want *APIError with 403", err)
	}
}

func TestGetPortfolio(t *testing.T) {
	var gotMethod, gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, breakdownFixture)
	})

	b, err := svc.GetPortfolio(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/portfolios/11111111-1111-1111-1111-111111111111" {
		t.Errorf("path = %q", gotPath)
	}

	if b.Portfolio.Name != "Default" || b.Portfolio.UUID != "11111111-1111-1111-1111-111111111111" ||
		b.Portfolio.Type != "DEFAULT" || b.Portfolio.Deleted {
		t.Errorf("portfolio decoded wrong: %+v", b.Portfolio)
	}

	bal := b.Balances
	if bal.TotalBalance != (Amount{"1500.25", "USD"}) ||
		bal.TotalFuturesBalance != (Amount{"0", "USD"}) ||
		bal.TotalCashEquivalentBalance != (Amount{"500.25", "USD"}) ||
		bal.TotalCryptoBalance != (Amount{"1000.00", "USD"}) ||
		bal.FuturesUnrealizedPNL != (Amount{"0", "USD"}) ||
		bal.PerpUnrealizedPNL != (Amount{"-12.5", "USD"}) {
		t.Errorf("balances decoded wrong: %+v", bal)
	}

	if len(b.SpotPositions) != 2 {
		t.Fatalf("spot positions = %d, want 2", len(b.SpotPositions))
	}
	btc := b.SpotPositions[0]
	if btc.Asset != "BTC" || btc.AccountUUID != "33333333-3333-3333-3333-333333333333" ||
		btc.TotalBalanceFiat != 1000.0 || btc.TotalBalanceCrypto != 0.0155 ||
		btc.AvailableToTradeFiat != 950.5 || btc.Allocation != 0.6666 ||
		btc.CostBasis != (Amount{"800.00", "USD"}) || btc.IsCash {
		t.Errorf("BTC position decoded wrong: %+v", btc)
	}
	if usd := b.SpotPositions[1]; usd.Asset != "USD" || !usd.IsCash || usd.TotalBalanceFiat != 500.25 {
		t.Errorf("USD position decoded wrong: %+v", usd)
	}
}

func TestGetPortfolio_TrimsAndEscapesID(t *testing.T) {
	var gotPath, gotRawPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, breakdownFixture)
	})
	if _, err := svc.GetPortfolio(context.Background(), "  a/b  "); err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if gotPath != "/api/v3/brokerage/portfolios/a/b" {
		t.Errorf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/api/v3/brokerage/portfolios/a%2Fb" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestGetPortfolio_EmptyID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	for _, id := range []string{"", "   "} {
		if _, err := svc.GetPortfolio(context.Background(), id); err == nil {
			t.Errorf("GetPortfolio(%q): expected error", id)
		}
	}
}

func TestGetPortfolio_NotFound(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"portfolio not found"}`)
	})
	_, err := svc.GetPortfolio(context.Background(), "nope")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
	if apiErr.Message != "portfolio not found" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestEditPortfolio(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody struct {
		Name string `json:"name"`
	}
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, portfolioFixture)
	})

	p, err := svc.EditPortfolio(context.Background(), "22222222-2222-2222-2222-222222222222", "Trading")
	if err != nil {
		t.Fatalf("EditPortfolio: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/portfolios/22222222-2222-2222-2222-222222222222" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.Name != "Trading" {
		t.Errorf("body name = %q", gotBody.Name)
	}
	if p.Name != "Trading" || p.UUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("portfolio decoded wrong: %+v", p)
	}
}

func TestEditPortfolio_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	cases := []struct{ id, name, wantErr string }{
		{"", "New Name", "portfolioId is required"},
		{"   ", "New Name", "portfolioId is required"},
		{"uuid-1", "", "name is required"},
		{"uuid-1", "   ", "name is required"},
	}
	for _, c := range cases {
		_, err := svc.EditPortfolio(context.Background(), c.id, c.name)
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("EditPortfolio(%q, %q): err = %v, want %q", c.id, c.name, err, c.wantErr)
		}
	}
}

func TestEditPortfolio_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"portfolio not found"}`)
	})
	_, err := svc.EditPortfolio(context.Background(), "nope", "New Name")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
}

func TestDeletePortfolio(t *testing.T) {
	var gotMethod, gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{}`)
	})

	res, err := svc.DeletePortfolio(context.Background(), "  22222222-2222-2222-2222-222222222222  ")
	if err != nil {
		t.Fatalf("DeletePortfolio: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/portfolios/22222222-2222-2222-2222-222222222222" {
		t.Errorf("path = %q", gotPath)
	}
	if !res.Deleted || res.PortfolioUUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("result = %+v", res)
	}
}

func TestDeletePortfolio_EmptyID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	for _, id := range []string{"", "   "} {
		if _, err := svc.DeletePortfolio(context.Background(), id); err == nil {
			t.Errorf("DeletePortfolio(%q): expected error", id)
		}
	}
}

func TestDeletePortfolio_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"FAILED_PRECONDITION","message":"cannot delete default portfolio"}`)
	})
	res, err := svc.DeletePortfolio(context.Background(), "default-uuid")
	if res != nil {
		t.Errorf("result = %+v, want nil on error", res)
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v, want *APIError 403", err)
	}
	if apiErr.Message != "cannot delete default portfolio" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestMoveFunds(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody struct {
		Funds struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"funds"`
		SourcePortfolioUUID string `json:"source_portfolio_uuid"`
		TargetPortfolioUUID string `json:"target_portfolio_uuid"`
	}
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, moveFundsFixture)
	})

	res, err := svc.MoveFunds(context.Background(), " 100.50 ", " USD ",
		" 11111111-1111-1111-1111-111111111111 ", " 22222222-2222-2222-2222-222222222222 ")
	if err != nil {
		t.Fatalf("MoveFunds: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/portfolios/move_funds" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.Funds.Value != "100.50" || gotBody.Funds.Currency != "USD" {
		t.Errorf("funds = %+v, want trimmed 100.50 USD", gotBody.Funds)
	}
	if gotBody.SourcePortfolioUUID != "11111111-1111-1111-1111-111111111111" ||
		gotBody.TargetPortfolioUUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("body uuids = %+v", gotBody)
	}
	if res.SourcePortfolioUUID != "11111111-1111-1111-1111-111111111111" ||
		res.TargetPortfolioUUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("result decoded wrong: %+v", res)
	}
}

func TestMoveFunds_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	cases := []struct{ value, currency, source, target, wantErr string }{
		{"", "USD", "src", "dst", "value is required"},
		{"  ", "USD", "src", "dst", "value is required"},
		{"100", "", "src", "dst", "currency is required"},
		{"100", "  ", "src", "dst", "currency is required"},
		{"100", "USD", "", "dst", "sourcePortfolioId is required"},
		{"100", "USD", "  ", "dst", "sourcePortfolioId is required"},
		{"100", "USD", "src", "", "targetPortfolioId is required"},
		{"100", "USD", "src", "  ", "targetPortfolioId is required"},
	}
	for _, c := range cases {
		_, err := svc.MoveFunds(context.Background(), c.value, c.currency, c.source, c.target)
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("MoveFunds(%q,%q,%q,%q): err = %v, want %q",
				c.value, c.currency, c.source, c.target, err, c.wantErr)
		}
	}
}

func TestMoveFunds_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"insufficient funds"}`)
	})
	_, err := svc.MoveFunds(context.Background(), "999999", "USD", "src", "dst")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError 400", err)
	}
	if apiErr.Message != "insufficient funds" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}
