// SPDX-License-Identifier: MIT

package convert

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

// tradeFixture is shaped per the Advanced Trade convert endpoints, including
// fields the Trade struct intentionally drops (decoding must ignore them).
const tradeFixture = `{
  "trade": {
    "id": "trade-1234",
    "status": "TRADE_STATUS_CREATED",
    "user_entered_amount": {"value": "100", "currency": "USD"},
    "amount": {"value": "100", "currency": "USD"},
    "subtotal": {"value": "99.40", "currency": "USD"},
    "total": {"value": "100", "currency": "USD"},
    "exchange_rate": {"value": "1.0000", "currency": "USDC"},
    "fees": [{"title": "Coinbase fee", "amount": {"value": "0.60", "currency": "USD"}}],
    "total_fee": {"title": "total fee", "amount": {"value": "0.60", "currency": "USD"}},
    "source_currency": "USD",
    "target_currency": "USDC",
    "unit_price": {"target_to_fiat": {"amount": {"value": "1", "currency": "USD"}}}
  }
}`

// checkTrade verifies every mapped Trade field against tradeFixture.
func checkTrade(t *testing.T, tr *Trade) {
	t.Helper()
	if tr.ID != "trade-1234" || tr.Status != "TRADE_STATUS_CREATED" {
		t.Errorf("id/status = %q/%q", tr.ID, tr.Status)
	}
	if tr.UserEnteredAmount != (Amount{"100", "USD"}) ||
		tr.Amount != (Amount{"100", "USD"}) ||
		tr.Subtotal != (Amount{"99.40", "USD"}) ||
		tr.Total != (Amount{"100", "USD"}) ||
		tr.ExchangeRate != (Amount{"1.0000", "USDC"}) {
		t.Errorf("trade amounts decoded wrong: %+v", tr)
	}
}

// newTestService returns a convert service backed by a httptest server.
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

func TestCreateQuote(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody struct {
		FromAccount string `json:"from_account"`
		ToAccount   string `json:"to_account"`
		Amount      string `json:"amount"`
	}
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, tradeFixture)
	})

	tr, err := svc.CreateQuote(context.Background(), " USD ", " USDC ", " 100 ")
	if err != nil {
		t.Fatalf("CreateQuote: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/convert/quote" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.FromAccount != "USD" || gotBody.ToAccount != "USDC" || gotBody.Amount != "100" {
		t.Errorf("body = %+v, want trimmed USD/USDC/100", gotBody)
	}
	checkTrade(t, tr)
}

func TestCreateQuote_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	cases := []struct{ from, to, amount, wantErr string }{
		{"", "USDC", "100", "fromAccount is required"},
		{"  ", "USDC", "100", "fromAccount is required"},
		{"USD", "", "100", "toAccount is required"},
		{"USD", "  ", "100", "toAccount is required"},
		{"USD", "USDC", "", "amount is required"},
		{"USD", "USDC", "  ", "amount is required"},
	}
	for _, c := range cases {
		_, err := svc.CreateQuote(context.Background(), c.from, c.to, c.amount)
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("CreateQuote(%q,%q,%q): err = %v, want %q", c.from, c.to, c.amount, err, c.wantErr)
		}
	}
}

func TestCreateQuote_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"insufficient balance"}`)
	})
	_, err := svc.CreateQuote(context.Background(), "USD", "USDC", "999999")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError with 400", err)
	}
	if apiErr.Message != "insufficient balance" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestCommitTrade(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody struct {
		FromAccount string `json:"from_account"`
		ToAccount   string `json:"to_account"`
	}
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, tradeFixture)
	})

	tr, err := svc.CommitTrade(context.Background(), " trade-1234 ", "USD", "USDC")
	if err != nil {
		t.Fatalf("CommitTrade: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/convert/trade/trade-1234" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.FromAccount != "USD" || gotBody.ToAccount != "USDC" {
		t.Errorf("body = %+v", gotBody)
	}
	checkTrade(t, tr)
}

func TestCommitTrade_EscapesTradeID(t *testing.T) {
	var gotPath, gotRawPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, tradeFixture)
	})
	if _, err := svc.CommitTrade(context.Background(), "a/b", "USD", "USDC"); err != nil {
		t.Fatalf("CommitTrade: %v", err)
	}
	if gotPath != "/api/v3/brokerage/convert/trade/a/b" {
		t.Errorf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/api/v3/brokerage/convert/trade/a%2Fb" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestCommitTrade_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	cases := []struct{ id, from, to, wantErr string }{
		{"", "USD", "USDC", "tradeId is required"},
		{"  ", "USD", "USDC", "tradeId is required"},
		{"trade-1234", "", "USDC", "fromAccount is required"},
		{"trade-1234", "  ", "USDC", "fromAccount is required"},
		{"trade-1234", "USD", "", "toAccount is required"},
		{"trade-1234", "USD", "  ", "toAccount is required"},
	}
	for _, c := range cases {
		_, err := svc.CommitTrade(context.Background(), c.id, c.from, c.to)
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("CommitTrade(%q,%q,%q): err = %v, want %q", c.id, c.from, c.to, err, c.wantErr)
		}
	}
}

func TestCommitTrade_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"quote expired"}`)
	})
	_, err := svc.CommitTrade(context.Background(), "trade-1234", "USD", "USDC")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
	if apiErr.Message != "quote expired" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestGetTrade(t *testing.T) {
	var gotMethod, gotPath string
	var gotQuery map[string][]string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, tradeFixture)
	})

	tr, err := svc.GetTrade(context.Background(), " trade-1234 ", " USD ", " USDC ")
	if err != nil {
		t.Fatalf("GetTrade: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v3/brokerage/convert/trade/trade-1234" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["from_account"]; len(got) != 1 || got[0] != "USD" {
		t.Errorf("from_account = %v, want [USD]", got)
	}
	if got := gotQuery["to_account"]; len(got) != 1 || got[0] != "USDC" {
		t.Errorf("to_account = %v, want [USDC]", got)
	}
	checkTrade(t, tr)
}

func TestGetTrade_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	cases := []struct{ id, from, to, wantErr string }{
		{"", "USD", "USDC", "tradeId is required"},
		{"  ", "USD", "USDC", "tradeId is required"},
		{"trade-1234", "", "USDC", "fromAccount is required"},
		{"trade-1234", "  ", "USDC", "fromAccount is required"},
		{"trade-1234", "USD", "", "toAccount is required"},
		{"trade-1234", "USD", "  ", "toAccount is required"},
	}
	for _, c := range cases {
		_, err := svc.GetTrade(context.Background(), c.id, c.from, c.to)
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("GetTrade(%q,%q,%q): err = %v, want %q", c.id, c.from, c.to, err, c.wantErr)
		}
	}
}

func TestGetTrade_NotFound(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"trade not found"}`)
	})
	_, err := svc.GetTrade(context.Background(), "nope", "USD", "USDC")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
	if apiErr.Message != "trade not found" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}
