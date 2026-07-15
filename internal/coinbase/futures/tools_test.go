// SPDX-License-Identifier: MIT

package futures

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

// newToolSession registers the futures toolset against a stub Coinbase API
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
	if s.ToolCount() != 9 {
		t.Fatalf("ToolCount = %d, want 9", s.ToolCount())
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

// writeTools are the mutating futures tools.
var writeTools = map[string]bool{
	"futures_sweep_schedule":     true,
	"futures_sweep_cancel":       true,
	"futures_margin_setting_set": true,
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
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema", tool.Name)
		}
		if tool.Name == "futures_sweep_cancel" &&
			(tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Error("futures_sweep_cancel should hint destructive")
		}
	}
	for _, want := range []string{
		"futures_balance", "futures_positions", "futures_position", "futures_sweeps",
		"futures_sweep_schedule", "futures_sweep_cancel",
		"futures_margin_setting", "futures_margin_setting_set", "futures_margin_window",
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
	if s.ToolCount() != 6 {
		t.Fatalf("ToolCount = %d, want 6 read-only tools", s.ToolCount())
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
	if len(res.Tools) != 6 {
		t.Errorf("len(tools) = %d, want 6", len(res.Tools))
	}
}

func TestFuturesBalanceTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/brokerage/cfm/balance_summary" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, balanceFixture)
	})

	res := callTool(t, cs, "futures_balance", map[string]any{})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out BalanceSummary
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.FuturesBuyingPower.Value != "5000.00" || out.LiquidationBufferPct != "288.9" {
		t.Errorf("balance = %+v", out)
	}
}

func TestFuturesPositionsTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, positionsFixture)
	})

	res := callTool(t, cs, "futures_positions", map[string]any{})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out server.ListResult[Position]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.Count != 2 || len(out.Items) != 2 || out.Items[0].ProductID != "BIT-31OCT25-CDE" {
		t.Errorf("positions = %+v", out)
	}
}

func TestFuturesSweepScheduleTool(t *testing.T) {
	var gotBody map[string]any
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/cfm/sweeps/schedule") {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"success": true}`)
	})

	res := callTool(t, cs, "futures_sweep_schedule", map[string]any{"usdAmount": "250.00"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if gotBody["usd_amount"] != "250.00" {
		t.Errorf("body = %v", gotBody)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out SweepScheduled
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if !out.Scheduled || out.USDAmount != "250.00" {
		t.Errorf("result = %+v", out)
	}
}

func TestFuturesSweepCancelTool(t *testing.T) {
	var gotMethod string
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_, _ = io.WriteString(w, `{"success": true}`)
	})

	res := callTool(t, cs, "futures_sweep_cancel", map[string]any{})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out SweepCancelled
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if !out.Cancelled {
		t.Errorf("result = %+v", out)
	}
}

func TestFuturesPositionTool_ValidationErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})

	res := callTool(t, cs, "futures_position", map[string]any{"productId": "  "})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for empty productId")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "productId is required") {
		t.Errorf("error text = %q, want validation message", text)
	}
}

func TestFuturesBalanceTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})

	res := callTool(t, cs, "futures_balance", map[string]any{})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for 401")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "missing credentials") {
		t.Errorf("error text = %q, want API message included", text)
	}
}

func TestFuturesReadToolsRegisteredAndCallable(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/balance_summary"):
			_, _ = io.WriteString(w, balanceFixture)
		case strings.HasSuffix(r.URL.Path, "/positions/BIT-31OCT25-CDE"):
			_, _ = io.WriteString(w, positionFixture)
		case strings.HasSuffix(r.URL.Path, "/positions"):
			_, _ = io.WriteString(w, positionsFixture)
		case strings.HasSuffix(r.URL.Path, "/sweeps"):
			_, _ = io.WriteString(w, sweepsFixture)
		case strings.HasSuffix(r.URL.Path, "/margin_setting"):
			_, _ = io.WriteString(w, marginSettingFixture)
		case strings.HasSuffix(r.URL.Path, "/current_margin_window"):
			_, _ = io.WriteString(w, marginWindowFixture)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	calls := []struct {
		tool string
		args map[string]any
	}{
		{"futures_balance", map[string]any{}},
		{"futures_positions", map[string]any{}},
		{"futures_position", map[string]any{"productId": "BIT-31OCT25-CDE"}},
		{"futures_sweeps", map[string]any{}},
		{"futures_margin_setting", map[string]any{}},
		{"futures_margin_window", map[string]any{}},
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

func TestFuturesMarginSettingSetTool(t *testing.T) {
	var gotBody map[string]any
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/intraday/margin_setting") {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{}`)
	})

	res := callTool(t, cs, "futures_margin_setting_set", map[string]any{"setting": "INTRADAY_MARGIN_SETTING_INTRADAY"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if gotBody["setting"] != "INTRADAY_MARGIN_SETTING_INTRADAY" {
		t.Errorf("body = %v", gotBody)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out MarginSettingUpdated
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if !out.Updated || out.Setting != "INTRADAY_MARGIN_SETTING_INTRADAY" {
		t.Errorf("result = %+v", out)
	}
}
