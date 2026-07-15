// SPDX-License-Identifier: MIT

package payments

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

// newToolSession registers the payments toolset against a stub Coinbase API
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
	if s.ToolCount() != 2 {
		t.Fatalf("ToolCount = %d, want 2", s.ToolCount())
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
	for _, want := range []string{"payments_list", "payments_get"} {
		if !names[want] {
			t.Errorf("tools = %v, missing %s", names, want)
		}
	}
}

func TestPaymentsListTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/brokerage/payment_methods" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, listFixture)
	})

	res := callTool(t, cs, "payments_list", map[string]any{})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var out server.ListResult[PaymentMethod]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if out.Count != 2 || len(out.Items) != 2 {
		t.Fatalf("count = %d items = %d, want 2/2", out.Count, len(out.Items))
	}
	if out.Items[0].ID != "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" {
		t.Errorf("first payment method = %+v", out.Items[0])
	}
}

func TestPaymentsGetTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/payment_methods/1c9d2e26-3158-4f18-a76b-4d2f56be6a3d") {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, getFixture)
	})

	res := callTool(t, cs, "payments_get", map[string]any{"paymentMethodId": "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var pm PaymentMethod
	if err := json.Unmarshal(raw, &pm); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pm.ID != "1c9d2e26-3158-4f18-a76b-4d2f56be6a3d" || pm.Type != "ACH" || !pm.Verified {
		t.Errorf("payment method = %+v", pm)
	}
}

func TestPaymentsGetTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"payment method not found"}`)
	})

	res := callTool(t, cs, "payments_get", map[string]any{"paymentMethodId": "83562370-3e5c-51db-87da-752af5ab9559"})
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
	if !strings.Contains(text, "payment method not found") {
		t.Errorf("error text = %q, want API message included", text)
	}
}
