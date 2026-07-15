// SPDX-License-Identifier: MIT

package orders

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

// readTools and writeTools partition the toolset by mutation.
var (
	readTools  = []string{"orders_preview", "orders_edit_preview", "orders_list", "orders_get", "orders_fills"}
	writeTools = []string{"orders_create", "orders_edit", "orders_cancel", "orders_close_position"}
)

// connect wires an MCP client session to a registered server.
func connect(t *testing.T, s *server.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Wait() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// newToolSession registers the orders toolset (writable server) against a stub
// Coinbase API and returns an MCP client session for end-to-end tool calls.
func newToolSession(t *testing.T, handler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	clients, err := coinbase.NewClients(api.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}

	s := server.New("test", "v0", false)
	Register(s, clients)
	if s.ToolCount() != 9 {
		t.Fatalf("ToolCount = %d, want 9", s.ToolCount())
	}
	if got := s.Toolsets(); len(got) != 1 || got[0] != Name {
		t.Fatalf("Toolsets = %v, want [%s]", got, Name)
	}
	return connect(t, s)
}

// callTool invokes an MCP tool and returns its result.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

// errorText concatenates a tool result's text content.
func errorText(res *mcp.CallToolResult) string {
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text
}

func TestToolsListed(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		tools[tool.Name] = tool
		if tool.Annotations == nil {
			t.Errorf("tool %s has no annotations", tool.Name)
			continue
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema", tool.Name)
		}
	}
	for _, want := range readTools {
		tool := tools[want]
		if tool == nil {
			t.Errorf("missing read tool %s", want)
			continue
		}
		if !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s should be read-only", want)
		}
	}
	destructive := map[string]bool{"orders_cancel": true, "orders_close_position": true}
	for _, want := range writeTools {
		tool := tools[want]
		if tool == nil {
			t.Errorf("missing write tool %s", want)
			continue
		}
		if tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s should not be read-only", want)
		}
		if tool.Annotations.DestructiveHint == nil {
			t.Errorf("tool %s should carry a destructive hint", want)
		} else if *tool.Annotations.DestructiveHint != destructive[want] {
			t.Errorf("tool %s DestructiveHint = %v, want %v", want, *tool.Annotations.DestructiveHint, destructive[want])
		}
	}
	if len(tools) != 9 {
		t.Errorf("tools = %d, want 9", len(tools))
	}
}

func TestReadOnlyServerHidesWriteTools(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(api.Close)
	clients, err := coinbase.NewClients(api.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}

	s := server.New("test", "v0", true)
	Register(s, clients)
	if s.ToolCount() != len(readTools) {
		t.Fatalf("ToolCount = %d, want %d", s.ToolCount(), len(readTools))
	}

	cs := connect(t, s)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	for _, want := range readTools {
		if !names[want] {
			t.Errorf("read tool %s missing from read-only server", want)
		}
	}
	for _, hidden := range writeTools {
		if names[hidden] {
			t.Errorf("write tool %s exposed on read-only server", hidden)
		}
	}
}

func TestOrdersCreateTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/brokerage/orders" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if body["product_id"] != "BTC-USD" || body["side"] != "BUY" || body["client_order_id"] != "cli-abc" {
			t.Errorf("body = %v", body)
		}
		cfg, _ := body["order_configuration"].(map[string]any)
		if ioc, _ := cfg["market_market_ioc"].(map[string]any); ioc == nil || ioc["quote_size"] != "100" {
			t.Errorf("order_configuration = %v", cfg)
		}
		_, _ = io.WriteString(w, createFixture)
	})

	res := callTool(t, cs, "orders_create", map[string]any{
		"productId":     "BTC-USD",
		"side":          "BUY",
		"orderType":     "market",
		"quoteSize":     "100",
		"clientOrderId": "cli-abc",
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out CreateResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.OrderID != "ord-123" || out.ProductID != "BTC-USD" || out.Side != "BUY" || out.ClientOrderID != "cli-abc" {
		t.Errorf("result = %+v", out)
	}
}

func TestOrdersCreateTool_RejectionSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, createRejectedFixture)
	})
	res := callTool(t, cs, "orders_create", map[string]any{
		"productId": "BTC-USD",
		"side":      "BUY",
		"orderType": "market",
		"quoteSize": "100",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for rejected order")
	}
	text := errorText(res)
	for _, want := range []string{"INSUFFICIENT_FUND", "Insufficient balance in source account"} {
		if !strings.Contains(text, want) {
			t.Errorf("error text = %q, want containing %q", text, want)
		}
	}
}

func TestOrdersCreateTool_ValidationErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input must not reach the API")
	})
	res := callTool(t, cs, "orders_create", map[string]any{
		"productId": "BTC-USD",
		"side":      "BUY",
		"orderType": "limit",
		"baseSize":  "0.001",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for missing limitPrice")
	}
	if text := errorText(res); !strings.Contains(text, "limitPrice is required") {
		t.Errorf("error text = %q, want limitPrice validation message", text)
	}
}

func TestOrdersCancelTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/brokerage/orders/batch_cancel" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if ids, _ := body["order_ids"].([]any); len(ids) != 2 || ids[0] != "ord-1" || ids[1] != "ord-2" {
			t.Errorf("order_ids = %v", body["order_ids"])
		}
		_, _ = io.WriteString(w, cancelFixture)
	})

	res := callTool(t, cs, "orders_cancel", map[string]any{"orderIds": []string{"ord-1", "ord-2"}})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out server.ListResult[CancelResult]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.Count != 2 || len(out.Items) != 2 || !out.Items[0].Success || out.Items[1].Success {
		t.Errorf("result = %+v", out)
	}
}

func TestOrdersToolsRegisteredAndCallable(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/orders/preview"):
			_, _ = io.WriteString(w, previewFixture)
		case strings.HasSuffix(r.URL.Path, "/orders/edit"):
			_, _ = io.WriteString(w, editFixture)
		case strings.HasSuffix(r.URL.Path, "/orders/edit_preview"):
			_, _ = io.WriteString(w, editPreviewFixture)
		case strings.HasSuffix(r.URL.Path, "/orders/close_position"):
			_, _ = io.WriteString(w, createFixture)
		case strings.HasSuffix(r.URL.Path, "/orders/historical/batch"):
			_, _ = io.WriteString(w, listFixture)
		case strings.HasSuffix(r.URL.Path, "/orders/historical/fills"):
			_, _ = io.WriteString(w, fillsFixture)
		case strings.Contains(r.URL.Path, "/orders/historical/"):
			_, _ = io.WriteString(w, getFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	calls := []struct {
		tool string
		args map[string]any
	}{
		{"orders_preview", map[string]any{"productId": "BTC-USD", "side": "BUY", "orderType": "limit", "baseSize": "0.0015", "limitPrice": "64000"}},
		{"orders_edit", map[string]any{"orderId": "ord-1", "price": "61000", "size": "0.002"}},
		{"orders_edit_preview", map[string]any{"orderId": "ord-1", "price": "61000", "size": "0.002"}},
		{"orders_close_position", map[string]any{"productId": "BTC-PERP-INTX", "size": "0.5"}},
		{"orders_list", map[string]any{"productId": "BTC-USD", "status": "FILLED", "side": "BUY", "limit": 25}},
		{"orders_get", map[string]any{"orderId": "ord-1"}},
		{"orders_fills", map[string]any{"orderId": "ord-1", "productId": "BTC-USD", "limit": 5}},
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

func TestOrdersGetTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"order not found"}`)
	})
	res := callTool(t, cs, "orders_get", map[string]any{"orderId": "ord-x"})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for 404")
	}
	if text := errorText(res); !strings.Contains(text, "order not found") {
		t.Errorf("error text = %q, want API message included", text)
	}
}
