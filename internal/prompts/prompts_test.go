// SPDX-License-Identifier: MIT

package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

func TestRegister(t *testing.T) {
	s := server.New("test", "v0", false)
	Register(s)
	if s.PromptCount() != 1 {
		t.Fatalf("PromptCount = %d, want 1", s.PromptCount())
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

	list, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(list.Prompts) != 1 || list.Prompts[0].Name != "market_snapshot" {
		t.Fatalf("prompts = %v", list.Prompts)
	}
	args := list.Prompts[0].Arguments
	if len(args) != 1 || args[0].Name != "product" || !args[0].Required {
		t.Errorf("arguments = %+v", args)
	}

	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "market_snapshot",
		Arguments: map[string]string{"product": "BTC-USD"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	tc, ok := got.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %T", got.Messages[0].Content)
	}
	if !strings.Contains(tc.Text, `"BTC-USD"`) {
		t.Errorf("prompt text missing product: %q", tc.Text)
	}
	if !strings.Contains(tc.Text, "products_get") {
		t.Errorf("prompt text should reference products_get tool: %q", tc.Text)
	}
}
