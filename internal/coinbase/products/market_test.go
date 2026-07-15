// SPDX-License-Identifier: MIT

package products

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Fixtures shaped per the Advanced Trade public market-data endpoints.
const (
	candlesFixture = `{
  "candles": [
    {"start": "1752470400", "low": "63800.1", "high": "64510.9", "open": "64000.0", "close": "64450.2", "volume": "812.5"},
    {"start": "1752474000", "low": "64300.0", "high": "64890.4", "open": "64450.2", "close": "64655.2", "volume": "633.1"}
  ]
}`
	tickerFixture = `{
  "trades": [
    {"trade_id": "t-1001", "product_id": "BTC-USD", "price": "64655.21", "size": "0.01", "time": "2026-07-14T12:00:00Z", "side": "BUY"},
    {"trade_id": "t-1002", "product_id": "BTC-USD", "price": "64650.00", "size": "0.25", "time": "2026-07-14T11:59:58Z", "side": "SELL"}
  ],
  "best_bid": "64650.01",
  "best_ask": "64655.99"
}`
	bookFixture = `{
  "pricebook": {
    "product_id": "BTC-USD",
    "bids": [{"price": "64650.01", "size": "0.5"}, {"price": "64649.50", "size": "1.2"}],
    "asks": [{"price": "64655.99", "size": "0.3"}],
    "time": "2026-07-14T12:00:00Z"
  }
}`
	timeFixture = `{"iso": "2026-07-14T12:00:00Z", "epochSeconds": "1784030400", "epochMillis": "1784030400000"}`
)

func TestGetCandles(t *testing.T) {
	var gotPath, gotQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, candlesFixture)
	})

	out, err := svc.GetCandles(context.Background(), "BTC-USD", "1752470400", "1752477600", "ONE_HOUR", 2)
	if err != nil {
		t.Fatalf("GetCandles: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products/BTC-USD/candles" {
		t.Errorf("path = %q", gotPath)
	}
	for _, part := range []string{"start=1752470400", "end=1752477600", "granularity=ONE_HOUR", "limit=2"} {
		if !strings.Contains(gotQuery, part) {
			t.Errorf("query %q missing %q", gotQuery, part)
		}
	}
	if len(out) != 2 {
		t.Fatalf("candles = %d, want 2", len(out))
	}
	c := out[0]
	if c.Start != "1752470400" || c.Low != "63800.1" || c.High != "64510.9" ||
		c.Open != "64000.0" || c.Close != "64450.2" || c.Volume != "812.5" {
		t.Errorf("candle decoded wrong: %+v", c)
	}
}

func TestGetCandles_NoLimit(t *testing.T) {
	var gotQuery string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"candles":[]}`)
	})
	if _, err := svc.GetCandles(context.Background(), "BTC-USD", "1", "2", "ONE_DAY", 0); err != nil {
		t.Fatalf("GetCandles: %v", err)
	}
	if strings.Contains(gotQuery, "limit") {
		t.Errorf("limit sent when zero: %q", gotQuery)
	}
}

func TestGetCandles_EmptyProductID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	if _, err := svc.GetCandles(context.Background(), " ", "1", "2", "ONE_HOUR", 0); err == nil {
		t.Fatal("expected error for empty productId")
	}
}

func TestGetTicker(t *testing.T) {
	var gotPath, gotLimit string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotLimit = r.URL.Query().Get("limit")
		_, _ = io.WriteString(w, tickerFixture)
	})

	out, err := svc.GetTicker(context.Background(), "BTC-USD", 2)
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products/BTC-USD/ticker" {
		t.Errorf("path = %q", gotPath)
	}
	if gotLimit != "2" {
		t.Errorf("limit = %q, want 2", gotLimit)
	}
	if out.BestBid != "64650.01" || out.BestAsk != "64655.99" {
		t.Errorf("bid/ask = %q/%q", out.BestBid, out.BestAsk)
	}
	if len(out.Trades) != 2 {
		t.Fatalf("trades = %d, want 2", len(out.Trades))
	}
	tr := out.Trades[0]
	if tr.TradeID != "t-1001" || tr.Price != "64655.21" || tr.Side != "BUY" || tr.Size != "0.01" {
		t.Errorf("trade decoded wrong: %+v", tr)
	}
}

func TestGetTicker_DefaultLimit(t *testing.T) {
	var gotLimit string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		_, _ = io.WriteString(w, `{"trades":[]}`)
	})
	if _, err := svc.GetTicker(context.Background(), "BTC-USD", 0); err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if gotLimit != "10" {
		t.Errorf("default limit = %q, want 10", gotLimit)
	}
}

func TestGetTicker_EmptyProductID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	if _, err := svc.GetTicker(context.Background(), "", 0); err == nil {
		t.Fatal("expected error for empty productId")
	}
}

func TestGetBook(t *testing.T) {
	var gotPath string
	var gotQuery map[string][]string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, bookFixture)
	})

	out, err := svc.GetBook(context.Background(), "BTC-USD", 2)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/product_book" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["product_id"]; len(got) != 1 || got[0] != "BTC-USD" {
		t.Errorf("product_id = %v", got)
	}
	if got := gotQuery["limit"]; len(got) != 1 || got[0] != "2" {
		t.Errorf("limit = %v", got)
	}
	if out.ProductID != "BTC-USD" || len(out.Bids) != 2 || len(out.Asks) != 1 {
		t.Errorf("book decoded wrong: %+v", out)
	}
	if out.Bids[0].Price != "64650.01" || out.Bids[0].Size != "0.5" {
		t.Errorf("bid level decoded wrong: %+v", out.Bids[0])
	}
}

func TestGetBook_EmptyProductID(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	if _, err := svc.GetBook(context.Background(), "", 0); err == nil {
		t.Fatal("expected error for empty productId")
	}
}

func TestGetTime(t *testing.T) {
	var gotPath string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, timeFixture)
	})

	out, err := svc.GetTime(context.Background())
	if err != nil {
		t.Fatalf("GetTime: %v", err)
	}
	if gotPath != "/api/v3/brokerage/time" {
		t.Errorf("path = %q", gotPath)
	}
	if out.ISO != "2026-07-14T12:00:00Z" || out.EpochSeconds != "1784030400" || out.EpochMillis != "1784030400000" {
		t.Errorf("time decoded wrong: %+v", out)
	}
}

func TestGetTime_Error(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"internal error"}`)
	})
	if _, err := svc.GetTime(context.Background()); err == nil {
		t.Fatal("expected error from 500")
	}
}

func TestMarketEndpoints_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"INVALID_ARGUMENT","message":"bad request"}`)
	})
	ctx := context.Background()

	if _, err := svc.GetCandles(ctx, "BTC-USD", "1", "2", "ONE_HOUR", 0); err == nil {
		t.Error("GetCandles: expected error from 400")
	}
	if _, err := svc.GetTicker(ctx, "BTC-USD", 1); err == nil {
		t.Error("GetTicker: expected error from 400")
	}
	if _, err := svc.GetBook(ctx, "BTC-USD", 1); err == nil {
		t.Error("GetBook: expected error from 400")
	}
}

func TestBasePath_SwitchesOnAuthentication(t *testing.T) {
	var gotPath string
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"products":[],"num_products":0}`)
	}
	// Unauthenticated: public /market endpoints.
	pub := newTestService(t, handler)
	if _, err := pub.ListProducts(context.Background(), "", 0); err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if gotPath != "/api/v3/brokerage/market/products" {
		t.Errorf("unauthenticated path = %q", gotPath)
	}
	// Authenticated: key-scoped endpoints.
	auth := newTestService(t, handler)
	auth.c.Authenticated = true
	if _, err := auth.ListProducts(context.Background(), "", 0); err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if gotPath != "/api/v3/brokerage/products" {
		t.Errorf("authenticated path = %q", gotPath)
	}
}

func TestBestBidAsk(t *testing.T) {
	var gotPath string
	var gotIDs []string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIDs = r.URL.Query()["product_ids"]
		_, _ = io.WriteString(w, `{
  "pricebooks": [
    {"product_id": "BTC-USD", "bids": [{"price": "64650.01", "size": "0.5"}], "asks": [{"price": "64655.99", "size": "0.3"}], "time": "2026-07-14T12:00:00Z"},
    {"product_id": "ETH-USD", "bids": [{"price": "1885.00", "size": "2"}], "asks": [{"price": "1885.50", "size": "1"}], "time": "2026-07-14T12:00:00Z"}
  ]
}`)
	})

	out, err := svc.BestBidAsk(context.Background(), []string{"BTC-USD", " ETH-USD ", ""})
	if err != nil {
		t.Fatalf("BestBidAsk: %v", err)
	}
	if gotPath != "/api/v3/brokerage/best_bid_ask" {
		t.Errorf("path = %q", gotPath)
	}
	if len(gotIDs) != 2 || gotIDs[0] != "BTC-USD" || gotIDs[1] != "ETH-USD" {
		t.Errorf("product_ids = %v, want trimmed [BTC-USD ETH-USD]", gotIDs)
	}
	if len(out) != 2 || out[0].ProductID != "BTC-USD" || out[0].Bids[0].Price != "64650.01" {
		t.Errorf("pricebooks decoded wrong: %+v", out)
	}
}

func TestBestBidAsk_NoIDs(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	for _, ids := range [][]string{nil, {}, {"", "  "}} {
		if _, err := svc.BestBidAsk(context.Background(), ids); err == nil {
			t.Errorf("BestBidAsk(%v): expected error", ids)
		}
	}
}

func TestBestBidAsk_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"Unauthorized"}`)
	})
	if _, err := svc.BestBidAsk(context.Background(), []string{"BTC-USD"}); err == nil {
		t.Fatal("expected error from 401")
	}
}

func TestRegisterAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	clients, err := coinbase.NewClients(srv.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	s := server.New("test", "v0", false)
	RegisterAuth(s, clients)
	if s.ToolCount() != 1 {
		t.Errorf("ToolCount = %d, want 1 (best_bid_ask)", s.ToolCount())
	}
	// RegisterAuth is the second half of the products toolset; it must not
	// re-note the toolset name.
	if len(s.Toolsets()) != 0 {
		t.Errorf("Toolsets = %v, want none noted by RegisterAuth", s.Toolsets())
	}
}

func TestBestBidAskTool(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"pricebooks":[{"product_id":"BTC-USD","bids":[{"price":"1","size":"1"}],"asks":[{"price":"2","size":"1"}]}]}`)
	}))
	t.Cleanup(api.Close)
	clients, err := coinbase.NewClients(api.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	s := server.New("test", "v0", false)
	Register(s, clients)
	RegisterAuth(s, clients)

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Wait() })
	client := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res := callTool(t, cs, "products_best_bid_ask", map[string]any{"productIds": []string{"BTC-USD"}})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out server.ListResult[Book]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 1 || out.Items[0].ProductID != "BTC-USD" {
		t.Errorf("result = %+v", out)
	}
}

func TestMarketToolsRegisteredAndCallable(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/candles"):
			_, _ = io.WriteString(w, candlesFixture)
		case strings.HasSuffix(r.URL.Path, "/ticker"):
			_, _ = io.WriteString(w, tickerFixture)
		case strings.HasSuffix(r.URL.Path, "/product_book"):
			_, _ = io.WriteString(w, bookFixture)
		case strings.HasSuffix(r.URL.Path, "/time"):
			_, _ = io.WriteString(w, timeFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	calls := []struct {
		tool string
		args map[string]any
	}{
		{"products_candles", map[string]any{"productId": "BTC-USD", "start": "1", "end": "2", "granularity": "ONE_HOUR"}},
		{"products_ticker", map[string]any{"productId": "BTC-USD"}},
		{"products_book", map[string]any{"productId": "BTC-USD"}},
		{"products_time", map[string]any{}},
	}
	for _, c := range calls {
		t.Run(c.tool, func(t *testing.T) {
			res := callTool(t, cs, c.tool, c.args)
			if res.IsError {
				t.Fatalf("%s returned tool error: %+v", c.tool, res.Content)
			}
			if res.StructuredContent == nil {
				t.Errorf("%s returned no structured content", c.tool)
			}
		})
	}
}
