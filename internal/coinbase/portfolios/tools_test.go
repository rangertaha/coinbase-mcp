// SPDX-License-Identifier: MIT

package portfolios

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

// writeTools are the portfolios tools that mutate state.
var writeTools = map[string]bool{
	"portfolios_create":     true,
	"portfolios_edit":       true,
	"portfolios_delete":     true,
	"portfolios_move_funds": true,
}

// newSession registers the portfolios toolset against a stub Coinbase API and
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
	if s.ToolCount() != 6 {
		t.Fatalf("ToolCount = %d, want 6", s.ToolCount())
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
		if tool.Name == "portfolios_delete" {
			if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Error("portfolios_delete should have DestructiveHint=true")
			}
		} else if writeTools[tool.Name] {
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
	for _, want := range []string{"portfolios_list", "portfolios_create", "portfolios_get",
		"portfolios_edit", "portfolios_delete", "portfolios_move_funds"} {
		if !names[want] {
			t.Errorf("tools = %v, missing %s", names, want)
		}
	}
}

func TestReadOnlyServerHidesWriteTools(t *testing.T) {
	s, cs := newSession(t, true, func(w http.ResponseWriter, r *http.Request) {})
	if s.ToolCount() != 2 {
		t.Fatalf("ToolCount = %d, want 2 (read-only tools only)", s.ToolCount())
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
	for _, want := range []string{"portfolios_list", "portfolios_get"} {
		if !names[want] {
			t.Errorf("tools = %v, missing read-only tool %s", names, want)
		}
	}
}

func TestPortfoliosListTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("portfolio_type") != "DEFAULT" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, listFixture)
	})

	res := callTool(t, cs, "portfolios_list", map[string]any{"type": "DEFAULT"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var out server.ListResult[Portfolio]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.Count != 2 || len(out.Items) != 2 {
		t.Fatalf("count = %d items = %d, want 2/2", out.Count, len(out.Items))
	}
	if out.Items[0].Name != "Default" || out.Items[0].Type != "DEFAULT" {
		t.Errorf("first portfolio = %+v", out.Items[0])
	}
}

func TestPortfoliosGetTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/portfolios/11111111-1111-1111-1111-111111111111") {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, breakdownFixture)
	})

	res := callTool(t, cs, "portfolios_get", map[string]any{"portfolioId": "11111111-1111-1111-1111-111111111111"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var b Breakdown
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.Portfolio.UUID != "11111111-1111-1111-1111-111111111111" ||
		b.Balances.TotalBalance != (Amount{"1500.25", "USD"}) || len(b.SpotPositions) != 2 {
		t.Errorf("breakdown = %+v", b)
	}
}

func TestPortfoliosCreateTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_, _ = io.WriteString(w, portfolioFixture)
	})

	res := callTool(t, cs, "portfolios_create", map[string]any{"name": "Trading"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var p Portfolio
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "Trading" || p.UUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("portfolio = %+v", p)
	}
}

func TestWriteToolsCallable(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			_, _ = io.WriteString(w, portfolioFixture)
		case r.Method == http.MethodDelete:
			_, _ = io.WriteString(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/move_funds"):
			_, _ = io.WriteString(w, moveFundsFixture)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	calls := []struct {
		tool string
		args map[string]any
	}{
		{"portfolios_edit", map[string]any{"portfolioId": "uuid-2", "name": "Trading"}},
		{"portfolios_delete", map[string]any{"portfolioId": "uuid-2"}},
		{"portfolios_move_funds", map[string]any{
			"value": "100.50", "currency": "USD",
			"sourcePortfolioId": "uuid-1", "targetPortfolioId": "uuid-2",
		}},
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

func TestPortfoliosDeleteTool_ReturnsConfirmation(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	res := callTool(t, cs, "portfolios_delete", map[string]any{"portfolioId": "uuid-2"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var d DeleteResult
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !d.Deleted || d.PortfolioUUID != "uuid-2" {
		t.Errorf("delete result = %+v", d)
	}
}

func TestPortfoliosGetTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"portfolio not found"}`)
	})

	res := callTool(t, cs, "portfolios_get", map[string]any{"portfolioId": "nope"})
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
	if !strings.Contains(text, "portfolio not found") {
		t.Errorf("error text = %q, want API message included", text)
	}
}

func TestPortfoliosCreateTool_ValidationErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})
	res := callTool(t, cs, "portfolios_create", map[string]any{"name": "   "})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for blank name")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "name is required") {
		t.Errorf("error text = %q, want validation message", text)
	}
}
