// SPDX-License-Identifier: MIT

package orders

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/rangertaha/coinbase-mcp/internal/client"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Fixtures shaped per the Advanced Trade API spec, including fields the
// trimmed structs intentionally drop (decoding must ignore them).
const (
	// createFixture is per POST /api/v3/brokerage/orders (success).
	createFixture = `{
  "success": true,
  "success_response": {
    "order_id": "ord-123",
    "product_id": "BTC-USD",
    "side": "BUY",
    "client_order_id": "cli-abc"
  },
  "order_configuration": {
    "market_market_ioc": {"quote_size": "100"}
  }
}`

	// createRejectedFixture is per POST /api/v3/brokerage/orders (rejection).
	createRejectedFixture = `{
  "success": false,
  "error_response": {
    "error": "INSUFFICIENT_FUND",
    "message": "Insufficient balance in source account",
    "error_details": "need 100 USD, have 5 USD",
    "preview_failure_reason": "PREVIEW_INSUFFICIENT_FUND"
  },
  "order_configuration": {
    "market_market_ioc": {"quote_size": "100"}
  }
}`

	// previewFixture is per POST /api/v3/brokerage/orders/preview.
	previewFixture = `{
  "order_total": "100.25",
  "commission_total": "0.25",
  "errs": ["PREVIEW_WARNING_A"],
  "warning": ["W1", "W2"],
  "quote_size": "100",
  "base_size": "0.0015",
  "best_bid": "64650.01",
  "best_ask": "64655.99",
  "is_max": true,
  "slippage": "0.001",
  "leverage": "1",
  "long_leverage": "1"
}`

	// editFixture is per POST /api/v3/brokerage/orders/edit (success).
	editFixture = `{"success": true, "errors": []}`

	// editRejectedFixture is per POST /api/v3/brokerage/orders/edit (rejection).
	editRejectedFixture = `{
  "success": false,
  "errors": [
    {"edit_failure_reason": "ONLY_LIMIT_ORDER_EDITS_SUPPORTED", "preview_failure_reason": "PREVIEW_INVALID_PRICE"}
  ]
}`

	// editPreviewFixture is per POST /api/v3/brokerage/orders/edit_preview.
	editPreviewFixture = `{
  "errors": [{"edit_failure_reason": "", "preview_failure_reason": "PREVIEW_INVALID_SIZE"}],
  "slippage": "0.002",
  "order_total": "200.5",
  "commission_total": "0.5",
  "quote_size": "200",
  "base_size": "0.003",
  "average_filled_price": "64000.1"
}`

	// cancelFixture is per POST /api/v3/brokerage/orders/batch_cancel.
	cancelFixture = `{
  "results": [
    {"success": true, "failure_reason": "UNKNOWN_CANCEL_FAILURE_REASON", "order_id": "ord-1"},
    {"success": false, "failure_reason": "UNKNOWN_CANCEL_ORDER", "order_id": "ord-2"}
  ]
}`

	// listFixture is per GET /api/v3/brokerage/orders/historical/batch.
	listFixture = `{
  "orders": [
    {
      "order_id": "ord-1",
      "client_order_id": "cli-1",
      "product_id": "BTC-USD",
      "user_id": "u-1",
      "side": "BUY",
      "status": "FILLED",
      "order_type": "LIMIT",
      "time_in_force": "GOOD_UNTIL_CANCELLED",
      "created_time": "2026-07-14T10:00:00Z",
      "completion_percentage": "100",
      "filled_size": "0.001",
      "average_filled_price": "64000.5",
      "filled_value": "64.0005",
      "number_of_fills": "2",
      "fee": "",
      "total_fees": "0.32",
      "total_value_after_fees": "63.68",
      "settled": true,
      "reject_reason": "REJECT_REASON_UNSPECIFIED",
      "cancel_message": "",
      "pending_cancel": false,
      "size_in_quote": false
    },
    {
      "order_id": "ord-2",
      "product_id": "ETH-USD",
      "side": "SELL",
      "status": "CANCELLED",
      "cancel_message": "User requested cancel"
    }
  ],
  "has_next": true,
  "cursor": "cur-789"
}`

	// getFixture is per GET /api/v3/brokerage/orders/historical/{order_id}.
	getFixture = `{
  "order": {
    "order_id": "ord-1",
    "client_order_id": "cli-1",
    "product_id": "BTC-USD",
    "side": "BUY",
    "status": "OPEN",
    "order_type": "LIMIT",
    "time_in_force": "GOOD_UNTIL_DATE_TIME",
    "created_time": "2026-07-14T10:00:00Z",
    "completion_percentage": "50",
    "filled_size": "0.0005",
    "average_filled_price": "64000.5",
    "filled_value": "32.0",
    "number_of_fills": "1",
    "total_fees": "0.16",
    "total_value_after_fees": "31.84",
    "settled": false,
    "reject_reason": "",
    "cancel_message": ""
  }
}`

	// fillsFixture is per GET /api/v3/brokerage/orders/historical/fills.
	fillsFixture = `{
  "fills": [
    {
      "entry_id": "e-1",
      "trade_id": "t-1",
      "order_id": "ord-1",
      "trade_time": "2026-07-14T10:00:01Z",
      "trade_type": "FILL",
      "price": "64000.5",
      "size": "0.0005",
      "commission": "0.16",
      "product_id": "BTC-USD",
      "sequence_timestamp": "2026-07-14T10:00:01Z",
      "liquidity_indicator": "MAKER",
      "size_in_quote": false,
      "user_id": "u-1",
      "side": "BUY"
    }
  ],
  "cursor": "fill-cur-1"
}`
)

// newTestService returns an orders service backed by a httptest server.
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

// capture records the request the API stub received.
type capture struct {
	method string
	path   string
	query  map[string][]string
	body   map[string]any
}

// stub returns a handler that records the request into cap and writes fixture.
func stub(t *testing.T, cap *capture, fixture string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.query = r.URL.Query()
		if r.Body != nil {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading request body: %v", err)
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &cap.body); err != nil {
					t.Errorf("decoding request body %q: %v", raw, err)
				}
			}
		}
		_, _ = io.WriteString(w, fixture)
	}
}

// cfgOf digs the order_configuration object out of a captured body.
func cfgOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	cfg, ok := body["order_configuration"].(map[string]any)
	if !ok {
		t.Fatalf("order_configuration missing or wrong type: %v", body["order_configuration"])
	}
	return cfg
}

// --- CreateOrder ---

func TestCreateOrder_MarketBuy(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, createFixture))

	out, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams:  ConfigParams{OrderType: "market", QuoteSize: "100"},
		ProductID:     "BTC-USD",
		Side:          "BUY",
		ClientOrderID: "cli-abc",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/api/v3/brokerage/orders" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body["client_order_id"] != "cli-abc" || cap.body["product_id"] != "BTC-USD" || cap.body["side"] != "BUY" {
		t.Errorf("body = %v", cap.body)
	}
	cfg := cfgOf(t, cap.body)
	ioc, ok := cfg["market_market_ioc"].(map[string]any)
	if !ok {
		t.Fatalf("market_market_ioc missing: %v", cfg)
	}
	if ioc["quote_size"] != "100" {
		t.Errorf("quote_size = %v", ioc["quote_size"])
	}
	if _, has := ioc["base_size"]; has {
		t.Errorf("base_size must be omitted for market buy: %v", ioc)
	}
	for _, key := range []string{"limit_limit_gtc", "limit_limit_gtd"} {
		if _, has := cfg[key]; has {
			t.Errorf("%s must be omitted for market orders", key)
		}
	}
	if out.OrderID != "ord-123" || out.ProductID != "BTC-USD" || out.Side != "BUY" || out.ClientOrderID != "cli-abc" {
		t.Errorf("result = %+v", out)
	}
}

func TestCreateOrder_MarketSell(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, createFixture))

	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams:  ConfigParams{OrderType: "market", BaseSize: "0.001"},
		ProductID:     "BTC-USD",
		Side:          "sell", // lowercase must be normalized
		ClientOrderID: "cli-abc",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if cap.body["side"] != "SELL" {
		t.Errorf("side = %v, want SELL", cap.body["side"])
	}
	ioc := cfgOf(t, cap.body)["market_market_ioc"].(map[string]any)
	if ioc["base_size"] != "0.001" {
		t.Errorf("base_size = %v", ioc["base_size"])
	}
	if _, has := ioc["quote_size"]; has {
		t.Errorf("quote_size must be omitted for market sell: %v", ioc)
	}
}

func TestCreateOrder_LimitGTC(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, createFixture))

	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams:  ConfigParams{OrderType: "limit", BaseSize: "0.001", LimitPrice: "60000", PostOnly: true},
		ProductID:     "BTC-USD",
		Side:          "BUY",
		ClientOrderID: "cli-abc",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	cfg := cfgOf(t, cap.body)
	gtc, ok := cfg["limit_limit_gtc"].(map[string]any)
	if !ok {
		t.Fatalf("limit_limit_gtc missing: %v", cfg)
	}
	if gtc["base_size"] != "0.001" || gtc["limit_price"] != "60000" || gtc["post_only"] != true {
		t.Errorf("limit_limit_gtc = %v", gtc)
	}
	if _, has := gtc["end_time"]; has {
		t.Errorf("end_time must be absent for GTC: %v", gtc)
	}
	if _, has := cfg["limit_limit_gtd"]; has {
		t.Errorf("limit_limit_gtd must be omitted without endTime")
	}
}

func TestCreateOrder_LimitGTD(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, createFixture))

	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams: ConfigParams{
			OrderType: "LIMIT", BaseSize: "0.001", LimitPrice: "60000",
			EndTime: "2026-08-01T00:00:00Z",
		},
		ProductID:     "BTC-USD",
		Side:          "SELL",
		ClientOrderID: "cli-abc",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	cfg := cfgOf(t, cap.body)
	gtd, ok := cfg["limit_limit_gtd"].(map[string]any)
	if !ok {
		t.Fatalf("limit_limit_gtd missing: %v", cfg)
	}
	if gtd["base_size"] != "0.001" || gtd["limit_price"] != "60000" ||
		gtd["end_time"] != "2026-08-01T00:00:00Z" || gtd["post_only"] != false {
		t.Errorf("limit_limit_gtd = %v", gtd)
	}
	if _, has := cfg["limit_limit_gtc"]; has {
		t.Errorf("limit_limit_gtc must be omitted when endTime is set")
	}
}

func TestCreateOrder_GeneratesClientOrderID(t *testing.T) {
	var ids []string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["client_order_id"].(string)
		ids = append(ids, id)
		_, _ = io.WriteString(w, createFixture)
	})

	p := CreateParams{
		ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "100"},
		ProductID:    "BTC-USD",
		Side:         "BUY",
	}
	for range 2 {
		if _, err := svc.CreateOrder(context.Background(), p); err != nil {
			t.Fatalf("CreateOrder: %v", err)
		}
	}
	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, id := range ids {
		if !hex32.MatchString(id) {
			t.Errorf("client_order_id = %q, want 32 hex chars", id)
		}
	}
	if len(ids) == 2 && ids[0] == ids[1] {
		t.Errorf("generated IDs must differ, both %q", ids[0])
	}
}

func TestCreateOrder_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input must not reach the API")
	})

	valid := CreateParams{
		ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "100"},
		ProductID:    "BTC-USD",
		Side:         "BUY",
	}
	cases := []struct {
		name    string
		mutate  func(*CreateParams)
		wantErr string
	}{
		{"missing productId", func(p *CreateParams) { p.ProductID = "  " }, "productId is required"},
		{"missing side", func(p *CreateParams) { p.Side = "" }, "side is required"},
		{"invalid side", func(p *CreateParams) { p.Side = "HOLD" }, `side must be "BUY" or "SELL"`},
		{"missing orderType", func(p *CreateParams) { p.OrderType = " " }, "orderType is required"},
		{"invalid orderType", func(p *CreateParams) { p.OrderType = "stop" }, `orderType must be one of`},
		{"market with both sizes", func(p *CreateParams) { p.BaseSize = "0.001" }, "exactly one of baseSize or quoteSize"},
		{"market with neither size", func(p *CreateParams) { p.QuoteSize = "" }, "exactly one of baseSize or quoteSize"},
		{"limit without limitPrice", func(p *CreateParams) {
			p.OrderType, p.QuoteSize, p.BaseSize = "limit", "", "0.001"
		}, "limitPrice is required"},
		{"limit without baseSize", func(p *CreateParams) {
			p.OrderType, p.QuoteSize, p.LimitPrice = "limit", "", "60000"
		}, "baseSize is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.mutate(&p)
			_, err := svc.CreateOrder(context.Background(), p)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestCreateOrder_Rejected(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, createRejectedFixture)
	})
	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "100"},
		ProductID:    "BTC-USD",
		Side:         "BUY",
	})
	if err == nil {
		t.Fatal("expected error for success=false")
	}
	for _, want := range []string{"INSUFFICIENT_FUND", "Insufficient balance in source account", "need 100 USD", "PREVIEW_INSUFFICIENT_FUND"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want containing %q", err, want)
		}
	}
}

func TestCreateOrder_RejectedWithoutDetails(t *testing.T) {
	fixtures := []string{
		`{"success": false}`,
		`{"success": false, "error_response": {}}`,
	}
	for _, fx := range fixtures {
		svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, fx)
		})
		_, err := svc.CreateOrder(context.Background(), CreateParams{
			ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "100"},
			ProductID:    "BTC-USD",
			Side:         "BUY",
		})
		if err == nil || !strings.Contains(err.Error(), "no error details") {
			t.Errorf("fixture %q: err = %v, want generic rejection", fx, err)
		}
	}
}

func TestCreateOrder_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})
	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "100"},
		ProductID:    "BTC-USD",
		Side:         "BUY",
	})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want *APIError 401", err)
	}
	if apiErr.Message != "missing credentials" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

// --- PreviewOrder ---

func TestPreviewOrder(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, previewFixture))

	out, err := svc.PreviewOrder(context.Background(), PreviewParams{
		ConfigParams: ConfigParams{OrderType: "limit", BaseSize: "0.0015", LimitPrice: "64000", PostOnly: true},
		ProductID:    "BTC-USD",
		Side:         "BUY",
	})
	if err != nil {
		t.Fatalf("PreviewOrder: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/api/v3/brokerage/orders/preview" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if _, has := cap.body["client_order_id"]; has {
		t.Errorf("preview body must not contain client_order_id: %v", cap.body)
	}
	if cap.body["product_id"] != "BTC-USD" || cap.body["side"] != "BUY" {
		t.Errorf("body = %v", cap.body)
	}
	gtc := cfgOf(t, cap.body)["limit_limit_gtc"].(map[string]any)
	if gtc["limit_price"] != "64000" || gtc["post_only"] != true {
		t.Errorf("limit_limit_gtc = %v", gtc)
	}
	if out.OrderTotal != "100.25" || out.CommissionTotal != "0.25" || out.QuoteSize != "100" ||
		out.BaseSize != "0.0015" || out.BestBid != "64650.01" || out.BestAsk != "64655.99" ||
		!out.IsMax || out.Slippage != "0.001" {
		t.Errorf("preview decoded wrong: %+v", out)
	}
	if len(out.Errs) != 1 || out.Errs[0] != "PREVIEW_WARNING_A" {
		t.Errorf("errs = %v", out.Errs)
	}
	if len(out.Warning) != 2 || out.Warning[0] != "W1" {
		t.Errorf("warning = %v", out.Warning)
	}
}

func TestPreviewOrder_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input must not reach the API")
	})
	cases := []struct {
		name    string
		params  PreviewParams
		wantErr string
	}{
		{"missing productId", PreviewParams{ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "1"}, Side: "BUY"}, "productId is required"},
		{"missing side", PreviewParams{ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "1"}, ProductID: "BTC-USD"}, "side is required"},
		{"bad config", PreviewParams{ConfigParams: ConfigParams{OrderType: "market"}, ProductID: "BTC-USD", Side: "BUY"}, "exactly one of baseSize or quoteSize"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.PreviewOrder(context.Background(), tc.params)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestPreviewOrder_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"invalid size"}`)
	})
	_, err := svc.PreviewOrder(context.Background(), PreviewParams{
		ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "1"},
		ProductID:    "BTC-USD",
		Side:         "BUY",
	})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError 400", err)
	}
}

// --- EditOrder / PreviewEditOrder ---

func TestEditOrder(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, editFixture))

	out, err := svc.EditOrder(context.Background(), " ord-1 ", "61000", "0.002")
	if err != nil {
		t.Fatalf("EditOrder: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/api/v3/brokerage/orders/edit" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body["order_id"] != "ord-1" || cap.body["price"] != "61000" || cap.body["size"] != "0.002" {
		t.Errorf("body = %v", cap.body)
	}
	if !out.Success {
		t.Errorf("result = %+v", out)
	}
}

func TestEditOrder_Rejected(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, editRejectedFixture)
	})
	_, err := svc.EditOrder(context.Background(), "ord-1", "61000", "0.002")
	if err == nil {
		t.Fatal("expected error for success=false")
	}
	for _, want := range []string{"ONLY_LIMIT_ORDER_EDITS_SUPPORTED", "PREVIEW_INVALID_PRICE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want containing %q", err, want)
		}
	}
}

func TestEditOrder_RejectedWithoutDetails(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"success": false, "errors": [{}]}`)
	})
	_, err := svc.EditOrder(context.Background(), "ord-1", "61000", "0.002")
	if err == nil || !strings.Contains(err.Error(), "no error details") {
		t.Errorf("err = %v, want generic rejection", err)
	}
}

func TestEditOrder_Validation(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input must not reach the API")
	})
	cases := []struct {
		name                 string
		orderID, price, size string
		wantErr              string
	}{
		{"missing orderId", "  ", "1", "1", "orderId is required"},
		{"missing price", "ord-1", "", "1", "price is required"},
		{"missing size", "ord-1", "1", " ", "size is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.EditOrder(context.Background(), tc.orderID, tc.price, tc.size); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("EditOrder err = %v, want %q", err, tc.wantErr)
			}
			if _, err := svc.PreviewEditOrder(context.Background(), tc.orderID, tc.price, tc.size); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("PreviewEditOrder err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestEditOrder_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"order not found"}`)
	})
	_, err := svc.EditOrder(context.Background(), "ord-x", "1", "1")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
}

func TestPreviewEditOrder(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, editPreviewFixture))

	out, err := svc.PreviewEditOrder(context.Background(), "ord-1", "61000", "0.002")
	if err != nil {
		t.Fatalf("PreviewEditOrder: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/api/v3/brokerage/orders/edit_preview" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body["order_id"] != "ord-1" || cap.body["price"] != "61000" || cap.body["size"] != "0.002" {
		t.Errorf("body = %v", cap.body)
	}
	if out.Slippage != "0.002" || out.OrderTotal != "200.5" || out.CommissionTotal != "0.5" ||
		out.QuoteSize != "200" || out.BaseSize != "0.003" || out.AverageFilledPrice != "64000.1" {
		t.Errorf("edit preview decoded wrong: %+v", out)
	}
	if len(out.Errors) != 1 || out.Errors[0].PreviewFailureReason != "PREVIEW_INVALID_SIZE" {
		t.Errorf("errors = %+v", out.Errors)
	}
}

func TestPreviewEditOrder_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"internal error"}`)
	})
	if _, err := svc.PreviewEditOrder(context.Background(), "ord-1", "1", "1"); err == nil {
		t.Fatal("expected error from 500")
	}
}

// --- CancelOrders ---

func TestCancelOrders(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, cancelFixture))

	out, err := svc.CancelOrders(context.Background(), []string{" ord-1 ", "", "ord-2"})
	if err != nil {
		t.Fatalf("CancelOrders: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/api/v3/brokerage/orders/batch_cancel" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	ids, ok := cap.body["order_ids"].([]any)
	if !ok || len(ids) != 2 || ids[0] != "ord-1" || ids[1] != "ord-2" {
		t.Errorf("order_ids = %v, want trimmed [ord-1 ord-2]", cap.body["order_ids"])
	}
	if len(out) != 2 {
		t.Fatalf("results = %d, want 2", len(out))
	}
	if !out[0].Success || out[0].OrderID != "ord-1" {
		t.Errorf("result[0] = %+v", out[0])
	}
	if out[1].Success || out[1].FailureReason != "UNKNOWN_CANCEL_ORDER" || out[1].OrderID != "ord-2" {
		t.Errorf("result[1] = %+v", out[1])
	}
}

func TestCancelOrders_Empty(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("empty orderIds must not reach the API")
	})
	for _, ids := range [][]string{nil, {}, {"", "   "}} {
		if _, err := svc.CancelOrders(context.Background(), ids); err == nil || !strings.Contains(err.Error(), "orderIds is required") {
			t.Errorf("CancelOrders(%v) err = %v, want orderIds required", ids, err)
		}
	}
}

func TestCancelOrders_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"PERMISSION_DENIED","message":"missing trade permission"}`)
	})
	_, err := svc.CancelOrders(context.Background(), []string{"ord-1"})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v, want *APIError 403", err)
	}
}

// --- ClosePosition ---

func TestClosePosition(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, createFixture))

	out, err := svc.ClosePosition(context.Background(), "cli-abc", "BTC-PERP-INTX", " 0.5 ")
	if err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}
	if cap.method != http.MethodPost || cap.path != "/api/v3/brokerage/orders/close_position" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.body["client_order_id"] != "cli-abc" || cap.body["product_id"] != "BTC-PERP-INTX" || cap.body["size"] != "0.5" {
		t.Errorf("body = %v", cap.body)
	}
	if out.OrderID != "ord-123" {
		t.Errorf("result = %+v", out)
	}
}

func TestClosePosition_GeneratesClientOrderIDAndOmitsSize(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, createFixture))

	if _, err := svc.ClosePosition(context.Background(), "", "BTC-PERP-INTX", ""); err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}
	id, _ := cap.body["client_order_id"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
		t.Errorf("client_order_id = %q, want generated 32 hex chars", id)
	}
	if _, has := cap.body["size"]; has {
		t.Errorf("size must be omitted when empty: %v", cap.body)
	}
}

func TestClosePosition_EmptyProductID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("empty productId must not reach the API")
	})
	if _, err := svc.ClosePosition(context.Background(), "", "  ", ""); err == nil || !strings.Contains(err.Error(), "productId is required") {
		t.Errorf("err = %v, want productId required", err)
	}
}

func TestClosePosition_Rejected(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, createRejectedFixture)
	})
	_, err := svc.ClosePosition(context.Background(), "", "BTC-PERP-INTX", "")
	if err == nil || !strings.Contains(err.Error(), "INSUFFICIENT_FUND") {
		t.Errorf("err = %v, want rejection details", err)
	}
}

func TestClosePosition_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"no open position"}`)
	})
	_, err := svc.ClosePosition(context.Background(), "", "BTC-PERP-INTX", "")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError 400", err)
	}
}

// --- ListOrders ---

func TestListOrders(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, listFixture))

	out, err := svc.ListOrders(context.Background(), "BTC-USD", "FILLED", "BUY", 25, "cur-123")
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if cap.method != http.MethodGet || cap.path != "/api/v3/brokerage/orders/historical/batch" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	want := map[string]string{
		"product_ids":  "BTC-USD",
		"order_status": "FILLED",
		"order_side":   "BUY",
		"limit":        "25",
		"cursor":       "cur-123",
	}
	for k, v := range want {
		if got := cap.query[k]; len(got) != 1 || got[0] != v {
			t.Errorf("query %s = %v, want [%s]", k, got, v)
		}
	}
	if len(out.Orders) != 2 || !out.HasNext || out.Cursor != "cur-789" {
		t.Fatalf("page = %+v", out)
	}
	o := out.Orders[0]
	if o.OrderID != "ord-1" || o.ClientOrderID != "cli-1" || o.ProductID != "BTC-USD" ||
		o.Side != "BUY" || o.Status != "FILLED" || o.OrderType != "LIMIT" ||
		o.TimeInForce != "GOOD_UNTIL_CANCELLED" || o.CreatedTime != "2026-07-14T10:00:00Z" ||
		o.CompletionPercentage != "100" || o.FilledSize != "0.001" ||
		o.AverageFilledPrice != "64000.5" || o.FilledValue != "64.0005" ||
		o.NumberOfFills != "2" || o.TotalFees != "0.32" || o.TotalValueAfterFees != "63.68" ||
		!o.Settled || o.RejectReason != "REJECT_REASON_UNSPECIFIED" {
		t.Errorf("order decoded wrong: %+v", o)
	}
	if o2 := out.Orders[1]; o2.OrderID != "ord-2" || o2.CancelMessage != "User requested cancel" || o2.Settled {
		t.Errorf("second order decoded wrong: %+v", o2)
	}
}

func TestListOrders_NoFilters(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"orders":[],"has_next":false}`)
	})
	out, err := svc.ListOrders(context.Background(), "", "", "", 0, "")
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if len(out.Orders) != 0 || out.HasNext {
		t.Errorf("page = %+v", out)
	}
}

func TestListOrders_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})
	_, err := svc.ListOrders(context.Background(), "", "", "", 0, "")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want *APIError 401", err)
	}
}

// --- GetOrder ---

func TestGetOrder(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, getFixture))

	out, err := svc.GetOrder(context.Background(), " ord-1 ")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if cap.method != http.MethodGet || cap.path != "/api/v3/brokerage/orders/historical/ord-1" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if out.OrderID != "ord-1" || out.Status != "OPEN" || out.TimeInForce != "GOOD_UNTIL_DATE_TIME" ||
		out.CompletionPercentage != "50" || out.Settled {
		t.Errorf("order decoded wrong: %+v", out)
	}
}

func TestGetOrder_EscapesID(t *testing.T) {
	var gotRawPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.RawPath
		_, _ = io.WriteString(w, getFixture)
	})
	if _, err := svc.GetOrder(context.Background(), "ord/1"); err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if gotRawPath != "/api/v3/brokerage/orders/historical/ord%2F1" {
		t.Errorf("raw path = %q, want single-encoded", gotRawPath)
	}
}

func TestGetOrder_EmptyID(t *testing.T) {
	called := false
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	for _, id := range []string{"", "   "} {
		if _, err := svc.GetOrder(context.Background(), id); err == nil || !strings.Contains(err.Error(), "orderId is required") {
			t.Errorf("GetOrder(%q) err = %v, want orderId required", id, err)
		}
	}
	if called {
		t.Error("empty ID must not reach the API")
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"order not found"}`)
	})
	_, err := svc.GetOrder(context.Background(), "ord-x")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError 404", err)
	}
}

// --- ListFills ---

func TestListFills(t *testing.T) {
	var cap capture
	svc := newTestService(t, stub(t, &cap, fillsFixture))

	out, err := svc.ListFills(context.Background(), "ord-1", "BTC-USD", 5, "fill-cur-0")
	if err != nil {
		t.Fatalf("ListFills: %v", err)
	}
	if cap.method != http.MethodGet || cap.path != "/api/v3/brokerage/orders/historical/fills" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	want := map[string]string{
		"order_ids":   "ord-1",
		"product_ids": "BTC-USD",
		"limit":       "5",
		"cursor":      "fill-cur-0",
	}
	for k, v := range want {
		if got := cap.query[k]; len(got) != 1 || got[0] != v {
			t.Errorf("query %s = %v, want [%s]", k, got, v)
		}
	}
	if len(out.Fills) != 1 || out.Cursor != "fill-cur-1" {
		t.Fatalf("page = %+v", out)
	}
	f := out.Fills[0]
	if f.EntryID != "e-1" || f.TradeID != "t-1" || f.OrderID != "ord-1" ||
		f.TradeTime != "2026-07-14T10:00:01Z" || f.TradeType != "FILL" ||
		f.Price != "64000.5" || f.Size != "0.0005" || f.Commission != "0.16" ||
		f.ProductID != "BTC-USD" || f.Side != "BUY" || f.LiquidityIndicator != "MAKER" {
		t.Errorf("fill decoded wrong: %+v", f)
	}
}

func TestListFills_NoFilters(t *testing.T) {
	var gotRawQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"fills":[]}`)
	})
	out, err := svc.ListFills(context.Background(), "", "", 0, "")
	if err != nil {
		t.Fatalf("ListFills: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("query = %q, want empty", gotRawQuery)
	}
	if len(out.Fills) != 0 || out.Cursor != "" {
		t.Errorf("page = %+v", out)
	}
}

func TestListFills_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"internal error"}`)
	})
	if _, err := svc.ListFills(context.Background(), "", "", 0, ""); err == nil {
		t.Fatal("expected error from 500")
	}
}

func TestCreateOrder_AttachedTPSL(t *testing.T) {
	var gotBody map[string]any
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"success":true,"success_response":{"order_id":"o-1","product_id":"BTC-USD","side":"BUY","client_order_id":"c-1"}}`)
	})

	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams:    ConfigParams{OrderType: "limit", BaseSize: "0.01", LimitPrice: "60000"},
		ProductID:       "BTC-USD",
		Side:            "BUY",
		TakeProfitPrice: "70000",
		StopLossPrice:   "55000",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	attached, _ := gotBody["attached_order_configuration"].(map[string]any)
	if attached == nil {
		t.Fatalf("attached_order_configuration missing from body: %v", gotBody)
	}
	bracket, _ := attached["trigger_bracket_gtc"].(map[string]any)
	if bracket == nil || bracket["limit_price"] != "70000" || bracket["stop_trigger_price"] != "55000" {
		t.Errorf("trigger_bracket_gtc = %v", bracket)
	}
}

func TestCreateOrder_AttachedTPSLOmittedWhenUnset(t *testing.T) {
	var gotRaw []byte
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotRaw, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"success":true,"success_response":{"order_id":"o-1"}}`)
	})
	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams: ConfigParams{OrderType: "market", QuoteSize: "10"},
		ProductID:    "BTC-USD", Side: "BUY",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if strings.Contains(string(gotRaw), "attached_order_configuration") {
		t.Errorf("attached config sent when unset: %s", gotRaw)
	}
}

func TestCreateOrder_AttachedTPSLHalfSet(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	_, err := svc.CreateOrder(context.Background(), CreateParams{
		ConfigParams:    ConfigParams{OrderType: "market", QuoteSize: "10"},
		ProductID:       "BTC-USD",
		Side:            "BUY",
		TakeProfitPrice: "70000",
	})
	if err == nil || !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("err = %v, want half-set bracket error", err)
	}
}
