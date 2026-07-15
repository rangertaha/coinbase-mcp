// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoIn struct {
	Msg string `json:"msg" jsonschema:"message to echo"`
}

type echoOut struct {
	Echo string `json:"echo"`
}

func echoHandler(_ context.Context, _ *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, echoOut, error) {
	return nil, echoOut{Echo: in.Msg}, nil
}

// mustAnnotations fails the test if the tool has no annotations.
func mustAnnotations(t *testing.T, tool *mcp.Tool) *mcp.ToolAnnotations {
	t.Helper()
	if tool.Annotations == nil {
		t.Fatal("annotations missing")
	}
	return tool.Annotations
}

// connect wires the server to an in-memory MCP client session.
func connect(t *testing.T, s *Server) *mcp.ClientSession {
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

func TestNew(t *testing.T) {
	s := New("test-server", "v1.0.0", true)
	if !s.ReadOnly() {
		t.Error("ReadOnly() = false, want true")
	}
	if s.ToolCount() != 0 || s.PromptCount() != 0 {
		t.Error("fresh server should have no tools or prompts")
	}
	if len(s.Toolsets()) != 0 {
		t.Error("fresh server should have no toolsets")
	}
}

func TestNoteToolset(t *testing.T) {
	s := New("t", "v", false)
	s.NoteToolset("products")
	s.NoteToolset("orders")
	if got := s.Toolsets(); !reflect.DeepEqual(got, []string{"products", "orders"}) {
		t.Errorf("Toolsets() = %v", got)
	}
}

func TestRegister_CountsTools(t *testing.T) {
	s := New("t", "v", false)
	Register(s, ToolDef{Name: "echo", Description: "echoes"}, echoHandler)
	if s.ToolCount() != 1 {
		t.Errorf("ToolCount = %d, want 1", s.ToolCount())
	}
}

func TestRegister_ReadOnlySkipsWriteTools(t *testing.T) {
	s := New("t", "v", true)
	Register(s, ToolDef{Name: "read_tool", Description: "reads"}, echoHandler)
	Register(s, ToolDef{Name: "write_tool", Description: "writes", Write: true}, echoHandler)
	if s.ToolCount() != 1 {
		t.Errorf("ToolCount = %d, want 1 (write tool skipped)", s.ToolCount())
	}

	cs := connect(t, s)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 1 || res.Tools[0].Name != "read_tool" {
		t.Errorf("tools = %v, want only read_tool", res.Tools)
	}
}

func TestRegister_WriteToolAnnotations(t *testing.T) {
	s := New("t", "v", false)
	Register(s, ToolDef{
		Name: "destroyer", Title: "Destroyer", Description: "deletes things",
		Write: true, Destructive: true, Idempotent: true,
	}, echoHandler)

	cs := connect(t, s)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(res.Tools))
	}
	ann := mustAnnotations(t, res.Tools[0])
	if ann.Title != "Destroyer" {
		t.Errorf("Title = %q", ann.Title)
	}
	if ann.ReadOnlyHint {
		t.Error("ReadOnlyHint = true, want false for write tool")
	}
	if !ann.IdempotentHint {
		t.Error("IdempotentHint = false, want true")
	}
	if ann.DestructiveHint == nil || !*ann.DestructiveHint {
		t.Error("DestructiveHint should be true")
	}
}

func TestRegister_ReadOnlyToolAnnotations(t *testing.T) {
	s := New("t", "v", false)
	Register(s, ToolDef{Name: "reader", Description: "reads"}, echoHandler)

	cs := connect(t, s)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	ann := res.Tools[0].Annotations
	if ann == nil || !ann.ReadOnlyHint {
		t.Error("read tool should have ReadOnlyHint=true")
	}
}

func TestCallTool_RoundTrip(t *testing.T) {
	s := New("t", "v", false)
	Register(s, ToolDef{Name: "echo", Description: "echoes"}, echoHandler)

	cs := connect(t, s)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hello"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out echoOut
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Echo != "hello" {
		t.Errorf("echo = %q, want hello", out.Echo)
	}
}

func TestCallTool_HandlerErrorIsToolError(t *testing.T) {
	s := New("t", "v", false)
	failing := func(_ context.Context, _ *mcp.CallToolRequest, _ echoIn) (*mcp.CallToolResult, echoOut, error) {
		return nil, echoOut{}, errors.New("backend exploded")
	}
	Register(s, ToolDef{Name: "boom", Description: "fails"}, failing)

	cs := connect(t, s)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "boom",
		Arguments: map[string]any{"msg": "x"},
	})
	if err != nil {
		t.Fatalf("CallTool should surface handler failure as tool error, got protocol error: %v", err)
	}
	if !res.IsError {
		t.Error("IsError = false, want true")
	}
}

func TestAddPrompt_RoundTrip(t *testing.T) {
	s := New("t", "v", false)
	s.AddPrompt("greet", "Greets someone.",
		[]PromptArg{{Name: "name", Description: "who to greet", Required: true}},
		func(args map[string]string) string { return "Hello, " + args["name"] + "!" },
	)
	if s.PromptCount() != 1 {
		t.Errorf("PromptCount = %d, want 1", s.PromptCount())
	}

	cs := connect(t, s)
	ctx := context.Background()

	list, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(list.Prompts) != 1 || list.Prompts[0].Name != "greet" {
		t.Fatalf("prompts = %v", list.Prompts)
	}
	pa := list.Prompts[0].Arguments
	if len(pa) != 1 || pa[0].Name != "name" || !pa[0].Required || pa[0].Description != "who to greet" {
		t.Errorf("prompt arguments = %+v", pa)
	}

	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "greet",
		Arguments: map[string]string{"name": "Ada"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(got.Messages))
	}
	tc, ok := got.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *TextContent", got.Messages[0].Content)
	}
	if tc.Text != "Hello, Ada!" {
		t.Errorf("prompt text = %q", tc.Text)
	}
	if got.Messages[0].Role != "user" {
		t.Errorf("role = %q, want user", got.Messages[0].Role)
	}
}

func TestAddPrompt_MissingRequiredArgument(t *testing.T) {
	s := New("t", "v", false)
	s.AddPrompt("greet", "Greets someone.",
		[]PromptArg{
			{Name: "name", Description: "who to greet", Required: true},
			{Name: "tone", Description: "optional tone", Required: false},
		},
		func(args map[string]string) string { return "Hello, " + args["name"] + "!" },
	)

	cs := connect(t, s)
	ctx := context.Background()

	// Entirely absent arguments.
	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "greet"}); err == nil {
		t.Error("expected error for missing required argument")
	}
	// Present but empty counts as missing.
	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "greet", Arguments: map[string]string{"name": ""},
	}); err == nil {
		t.Error("expected error for empty required argument")
	}
	// Optional argument may be omitted.
	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "greet", Arguments: map[string]string{"name": "Ada"},
	}); err != nil {
		t.Errorf("optional argument should be omittable: %v", err)
	}
}

func TestAddPrompt_NoArguments(t *testing.T) {
	s := New("t", "v", false)
	s.AddPrompt("static", "No args.", nil, func(args map[string]string) string {
		if args == nil {
			return "nil-args"
		}
		return "has-args"
	})

	cs := connect(t, s)
	got, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "static"})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	tc := got.Messages[0].Content.(*mcp.TextContent)
	if tc.Text != "nil-args" && tc.Text != "has-args" {
		t.Errorf("unexpected text %q", tc.Text)
	}
}

func TestList(t *testing.T) {
	r := List([]int{1, 2, 3})
	if r.Count != 3 || !reflect.DeepEqual(r.Items, []int{1, 2, 3}) {
		t.Errorf("List = %+v", r)
	}
	empty := List[string](nil)
	if empty.Count != 0 || empty.Items != nil {
		t.Errorf("List(nil) = %+v", empty)
	}
}

func TestNormalizedSchema(t *testing.T) {
	type withAny struct {
		Anything any    `json:"anything,omitempty"`
		Name     string `json:"name"`
	}
	raw := normalizedSchema(reflect.TypeFor[*withAny]())
	if raw == nil {
		t.Fatal("normalizedSchema returned nil")
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	props, _ := node["properties"].(map[string]any)
	if props == nil {
		t.Fatalf("schema has no properties: %s", raw)
	}
	// The `any` field must be an object schema, not the boolean `true`.
	if _, ok := props["anything"].(map[string]any); !ok {
		t.Errorf("anything schema = %T (%v), want object", props["anything"], props["anything"])
	}
}

func TestNormalizedSchema_UnsupportedType(t *testing.T) {
	// Channels cannot be represented in JSON schema; expect nil fallback.
	if raw := normalizedSchema(reflect.TypeFor[chan int]()); raw != nil {
		t.Errorf("normalizedSchema(chan) = %s, want nil", raw)
	}
}

func TestNormalizeSchemaNode(t *testing.T) {
	in := map[string]any{
		"true-schema":  true,
		"false-schema": false,
		"nested": map[string]any{
			"items": []any{true, map[string]any{"x": false}},
		},
		"plain": "string",
		"num":   1.5,
	}
	out := normalizeSchemaNode(in).(map[string]any)
	if !reflect.DeepEqual(out["true-schema"], map[string]any{}) {
		t.Errorf("true -> %v, want {}", out["true-schema"])
	}
	if !reflect.DeepEqual(out["false-schema"], map[string]any{"not": map[string]any{}}) {
		t.Errorf("false -> %v, want {not:{}}", out["false-schema"])
	}
	nested := out["nested"].(map[string]any)["items"].([]any)
	if !reflect.DeepEqual(nested[0], map[string]any{}) {
		t.Errorf("nested true -> %v", nested[0])
	}
	inner := nested[1].(map[string]any)["x"]
	if !reflect.DeepEqual(inner, map[string]any{"not": map[string]any{}}) {
		t.Errorf("nested false -> %v", inner)
	}
	if out["plain"] != "string" || out["num"] != 1.5 {
		t.Error("scalars must pass through unchanged")
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	s := New("t", "v", false)
	ctx, cancel := context.WithCancel(context.Background())
	_, st := mcp.NewInMemoryTransports()

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx, st) }()
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Run returned %v, want nil or context.Canceled", err)
	}
}

func TestAllowTools(t *testing.T) {
	s := New("t", "v", false)
	s.AllowTools([]string{"kept"})
	Register(s, ToolDef{Name: "kept", Description: "stays"}, echoHandler)
	Register(s, ToolDef{Name: "dropped", Description: "filtered out"}, echoHandler)
	if s.ToolCount() != 1 {
		t.Errorf("ToolCount = %d, want 1", s.ToolCount())
	}

	cs := connect(t, s)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 1 || res.Tools[0].Name != "kept" {
		t.Errorf("tools = %v, want only kept", res.Tools)
	}
}

func TestAllowTools_EmptyListMeansAll(t *testing.T) {
	s := New("t", "v", false)
	s.AllowTools(nil)
	Register(s, ToolDef{Name: "a", Description: "x"}, echoHandler)
	Register(s, ToolDef{Name: "b", Description: "y"}, echoHandler)
	if s.ToolCount() != 2 {
		t.Errorf("ToolCount = %d, want 2", s.ToolCount())
	}
}

func TestAllowTools_ReadOnlyStillWins(t *testing.T) {
	s := New("t", "v", true)
	s.AllowTools([]string{"writer"})
	Register(s, ToolDef{Name: "writer", Description: "writes", Write: true}, echoHandler)
	if s.ToolCount() != 0 {
		t.Error("read-only must hide a write tool even when explicitly allowed")
	}
}

func TestNew_Instructions(t *testing.T) {
	s := New("t", "v", false, "Use the tools wisely.")
	cs := connect(t, s)
	if got := cs.InitializeResult().Instructions; got != "Use the tools wisely." {
		t.Errorf("Instructions = %q", got)
	}
}
