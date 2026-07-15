// SPDX-License-Identifier: MIT

package perpetuals

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

// newToolSession registers the perpetuals toolset against a stub Coinbase API
// and returns an MCP client session for end-to-end tool calls.
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
	if s.ToolCount() != 6 {
		t.Fatalf("ToolCount = %d, want 6", s.ToolCount())
	}
	if got := s.Toolsets(); len(got) != 1 || got[0] != Name {
		t.Fatalf("Toolsets = %v, want [%s]", got, Name)
	}

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

// callTool invokes an MCP tool and returns its result.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

// writeTools are the mutating perpetuals tools.
var writeTools = map[string]bool{
	"perpetuals_allocate":               true,
	"perpetuals_multi_asset_collateral": true,
}

func TestToolsListed(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		if tool.Annotations == nil {
			t.Errorf("tool %s has no annotations", tool.Name)
			continue
		}
		if tool.Annotations.ReadOnlyHint == writeTools[tool.Name] {
			t.Errorf("tool %s: ReadOnlyHint = %v", tool.Name, tool.Annotations.ReadOnlyHint)
		}
		if writeTools[tool.Name] &&
			(tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint) {
			t.Errorf("tool %s should hint non-destructive write", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema", tool.Name)
		}
	}
	for _, want := range []string{
		"perpetuals_portfolio", "perpetuals_positions", "perpetuals_position",
		"perpetuals_balances", "perpetuals_allocate", "perpetuals_multi_asset_collateral",
	} {
		if !names[want] {
			t.Errorf("tools = %v, missing %s", names, want)
		}
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
	if s.ToolCount() != 4 {
		t.Fatalf("ToolCount = %d, want 4 read-only tools", s.ToolCount())
	}

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

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if writeTools[tool.Name] {
			t.Errorf("write tool %s exposed on read-only server", tool.Name)
		}
	}
	if len(res.Tools) != 4 {
		t.Errorf("len(tools) = %d, want 4", len(res.Tools))
	}
}

func TestPerpetualsPortfolioTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/intx/portfolio/"+testPortfolioID) {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, portfolioFixture)
	})

	res := callTool(t, cs, "perpetuals_portfolio", map[string]any{"portfolioId": testPortfolioID})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out Portfolio
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.PortfolioUUID != testPortfolioID || out.InitialMargin != "0.2" ||
		out.TotalBalance.Value != "10012.34" {
		t.Errorf("portfolio = %+v", out)
	}
}

func TestPerpetualsPositionsTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, positionsFixture)
	})

	res := callTool(t, cs, "perpetuals_positions", map[string]any{"portfolioId": testPortfolioID})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out server.ListResult[Position]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.Count != 2 || len(out.Items) != 2 || out.Items[0].Symbol != "BTC-PERP-INTX" {
		t.Errorf("positions = %+v", out)
	}
}

func TestPerpetualsAllocateTool(t *testing.T) {
	var gotBody map[string]any
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/intx/allocate") {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{}`)
	})

	res := callTool(t, cs, "perpetuals_allocate", map[string]any{
		"portfolioId": testPortfolioID,
		"symbol":      "BTC-PERP-INTX",
		"amount":      "100.00",
		"currency":    "USDC",
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if gotBody["portfolio_uuid"] != testPortfolioID || gotBody["symbol"] != "BTC-PERP-INTX" ||
		gotBody["amount"] != "100.00" || gotBody["currency"] != "USDC" {
		t.Errorf("body = %v", gotBody)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out Allocated
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if !out.Allocated || out.Symbol != "BTC-PERP-INTX" {
		t.Errorf("result = %+v", out)
	}
}

func TestPerpetualsMultiAssetCollateralTool(t *testing.T) {
	var gotBody map[string]any
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/intx/multi_asset_collateral") {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"multi_asset_collateral_enabled": true}`)
	})

	res := callTool(t, cs, "perpetuals_multi_asset_collateral", map[string]any{
		"portfolioId": testPortfolioID,
		"enabled":     true,
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if gotBody["portfolio_uuid"] != testPortfolioID || gotBody["multi_asset_collateral_enabled"] != true {
		t.Errorf("body = %v", gotBody)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out MultiAssetCollateral
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if !out.Enabled {
		t.Errorf("result = %+v", out)
	}
}

func TestPerpetualsPortfolioTool_ValidationErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})

	res := callTool(t, cs, "perpetuals_portfolio", map[string]any{"portfolioId": "  "})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for empty portfolioId")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "portfolioId is required") {
		t.Errorf("error text = %q, want validation message", text)
	}
}

func TestPerpetualsPortfolioTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"portfolio not found"}`)
	})

	res := callTool(t, cs, "perpetuals_portfolio", map[string]any{"portfolioId": testPortfolioID})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for 404")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "portfolio not found") {
		t.Errorf("error text = %q, want API message included", text)
	}
}

func TestPerpetualsReadToolsRegisteredAndCallable(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/intx/portfolio/"):
			_, _ = io.WriteString(w, portfolioFixture)
		case strings.HasSuffix(r.URL.Path, "/BTC-PERP-INTX"):
			_, _ = io.WriteString(w, positionFixture)
		case strings.Contains(r.URL.Path, "/intx/positions/"):
			_, _ = io.WriteString(w, positionsFixture)
		case strings.Contains(r.URL.Path, "/intx/balances/"):
			_, _ = io.WriteString(w, balancesFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	calls := []struct {
		tool string
		args map[string]any
	}{
		{"perpetuals_portfolio", map[string]any{"portfolioId": testPortfolioID}},
		{"perpetuals_positions", map[string]any{"portfolioId": testPortfolioID}},
		{"perpetuals_position", map[string]any{"portfolioId": testPortfolioID, "symbol": "BTC-PERP-INTX"}},
		{"perpetuals_balances", map[string]any{"portfolioId": testPortfolioID}},
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
