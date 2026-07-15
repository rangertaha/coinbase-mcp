// SPDX-License-Identifier: MIT

package fees

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

// newToolSession registers the fees toolset against a stub Coinbase API and
// returns an MCP client session for end-to-end tool calls.
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
	if s.ToolCount() != 1 {
		t.Fatalf("ToolCount = %d, want 1", s.ToolCount())
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

func TestToolsListed(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s should be read-only", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema", tool.Name)
		}
	}
	if !names["fees_summary"] {
		t.Errorf("tools = %v, missing fees_summary", names)
	}
}

func TestFeesSummaryTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("product_type") != "SPOT" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, summaryFixture)
	})

	res := callTool(t, cs, "fees_summary", map[string]any{"productType": "SPOT"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var out Summary
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.TotalVolume != 1000.5 || out.TotalFees != 25.5 {
		t.Errorf("totals = %v/%v", out.TotalVolume, out.TotalFees)
	}
	if out.FeeTier.TakerFeeRate != "0.008" || out.FeeTier.MakerFeeRate != "0.006" {
		t.Errorf("fee_tier = %+v", out.FeeTier)
	}
}

func TestFeesSummaryTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"UNAUTHENTICATED","message":"missing credentials"}`)
	})

	res := callTool(t, cs, "fees_summary", map[string]any{})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for 401")
	}
	// The error content should carry the API's message so the model can act.
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
