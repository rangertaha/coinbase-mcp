// SPDX-License-Identifier: MIT

package accounts

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

// newToolSession registers the accounts toolset against a stub Coinbase API
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
	for _, want := range []string{"accounts_list", "accounts_get"} {
		if !names[want] {
			t.Errorf("tools = %v, missing %s", names, want)
		}
	}
}

func TestAccountsListTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("cursor") != "abc123" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, listFixture)
	})

	res := callTool(t, cs, "accounts_list", map[string]any{"limit": 2, "cursor": "abc123"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var out AccountsPage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if len(out.Accounts) != 2 || !out.HasNext || out.Cursor != "789100" || out.Size != 2 {
		t.Fatalf("page = %+v, want 2 accounts with has_next/cursor/size", out)
	}
	if out.Accounts[0].UUID != "8bfc20d7-f7c6-4422-bf07-8243ca4169fe" {
		t.Errorf("first account = %+v", out.Accounts[0])
	}
}

func TestAccountsGetTool(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/accounts/8bfc20d7-f7c6-4422-bf07-8243ca4169fe") {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, getFixture)
	})

	res := callTool(t, cs, "accounts_get", map[string]any{"accountId": "8bfc20d7-f7c6-4422-bf07-8243ca4169fe"})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var a Account
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.UUID != "8bfc20d7-f7c6-4422-bf07-8243ca4169fe" || a.AvailableBalance.Value != "1.23" {
		t.Errorf("account = %+v", a)
	}
}

func TestAccountsGetTool_APIErrorSurfacesAsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"NOT_FOUND","message":"account not found"}`)
	})

	res := callTool(t, cs, "accounts_get", map[string]any{"accountId": "b6ee36f9-2b34-40ba-8a05-cfba2b3a2c9c"})
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
	if !strings.Contains(text, "account not found") {
		t.Errorf("error text = %q, want API message included", text)
	}
}

func TestAccountsGetTool_EmptyIDIsToolError(t *testing.T) {
	cs := newToolSession(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not reach the API")
	})

	res := callTool(t, cs, "accounts_get", map[string]any{"accountId": "  "})
	if !res.IsError {
		t.Fatal("IsError = false, want tool error for empty accountId")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "accountId is required") {
		t.Errorf("error text = %q, want validation message", text)
	}
}
