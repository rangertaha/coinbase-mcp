// SPDX-License-Identifier: MIT

package convert

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

// writeTools are the convert tools that mutate state.
var writeTools = map[string]bool{
	"convert_quote":  true,
	"convert_commit": true,
}

// newSession registers the convert toolset against a stub Coinbase API and
// returns an MCP client session for end-to-end tool calls.
func newSession(t *testing.T, readOnly bool, handler http.HandlerFunc) (*server.Server, *mcp.ClientSession) {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	clients, err := coinbase.NewClients(api.URL, "", "")
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}

	s := server.New("test", "v0", readOnly)
	Register(s, clients)
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
	return s, cs
}

// newToolSession is the read-write variant used by most tests.
func newToolSession(t *testing.T, handler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	s, cs := newSession(t, false, handler)
	if s.ToolCount() != 3 {
		t.Fatalf("ToolCount = %d, want 3", s.ToolCount())
	}
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

// decodeTrade unmarshals a tool result's structured content into a Trade.
func decodeTrade(t *testing.T, res *mcp.CallToolResult) *Trade {
	t.Helper()
	raw, _ := json.Marshal(res.StructuredContent)
	var tr Trade
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	return &tr
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
		if want := !writeTools[tool.Name]; tool.Annotations.ReadOnlyHint != want {
			t.Errorf("tool %s ReadOnlyHint = %v, want %v", tool.Name, tool.Annotations.ReadOnlyHint, want)
		}
		if writeTools[tool.Name] {
			if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Errorf("tool %s should have DestructiveHint=false", tool.Name)
			}
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema", tool.Name)
		}
	}
	for _, want := range []string{"convert_quote", "convert_commit", "convert_get"} {
		if !names[want] {
			t.Errorf("tools = %v, missing %s", names, want)
		}
	}
}

func TestReadOnlyServerHidesWriteTools(t *testing.T) {
	s, cs := newSession(t, true, func(w http.ResponseWriter, r *http.Request) {})
	if s.ToolCount() != 1 {
		t.Fatalf("ToolCount = %d, want 1 (read-only tools only)", s.ToolCount())
	}
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		if writeTools[tool.Name] {
			t.Errorf("write tool %s exposed on read-only server", tool.Name)
		}
	}
	if !names["convert_get"] {
		t.Errorf("tools = %v, missing read-only tool convert_get", names)
	}
}

func TestConvertQuoteTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/convert/quote") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, tradeFixture)
	})

	res := callTool(t, cs, "convert_quote", map[string]any{
		"fromAccount": "USD", "toAccount": "USDC", "amount": "100",
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	checkTrade(t, decodeTrade(t, res))
}

func TestConvertCommitTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/convert/trade/trade-1234") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, tradeFixture)
	})

	res := callTool(t, cs, "convert_commit", map[string]any{
		"tradeId": "trade-1234", "fromAccount": "USD", "toAccount": "USDC",
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	checkTrade(t, decodeTrade(t, res))
}

func TestConvertGetTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/convert/trade/trade-1234") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("from_account") != "USD" || r.URL.Query().Get("to_account") != "USDC" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, tradeFixture)
	})

	res := callTool(t, cs, "convert_get", map[string]any{
		"tradeId": "trade-1234", "fromAccount": "USD", "toAccount": "USDC",
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	checkTrade(t, decodeTrade(t, res))
}

func TestConvertGetTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"trade not found"}`)
	})

	res := callTool(t, cs, "convert_get", map[string]any{
		"tradeId": "nope", "fromAccount": "USD", "toAccount": "USDC",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for 404")
	}
	// The error content should carry the API's message so the model can act.
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "trade not found") {
		t.Errorf("error text = %q, want API message included", text)
	}
}

func TestConvertQuoteTool_ValidationErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	res := callTool(t, cs, "convert_quote", map[string]any{
		"fromAccount": "USD", "toAccount": "USDC", "amount": "   ",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for blank amount")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "amount is required") {
		t.Errorf("error text = %q, want validation message", text)
	}
}
