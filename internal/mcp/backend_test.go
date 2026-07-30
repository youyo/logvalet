package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// fakeBackendEntry は fakeBackend.RegisterTool に渡された ToolDef/ToolHandler の組。
type fakeBackendEntry struct {
	tool    mcpinternal.ToolDef
	handler mcpinternal.ToolHandler
}

// fakeBackend は SDK に依存しない ServerBackend の test double。
// RegisterTool の呼び出しをそのまま記録し、テストから直接 handler を呼び出せるようにする。
type fakeBackend struct {
	registered map[string]fakeBackendEntry
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{registered: map[string]fakeBackendEntry{}}
}

func (b *fakeBackend) RegisterTool(tool mcpinternal.ToolDef, handler mcpinternal.ToolHandler) {
	b.registered[tool.Name] = fakeBackendEntry{tool: tool, handler: handler}
}

func (b *fakeBackend) call(t *testing.T, name string, args map[string]any) mcpinternal.ToolResult {
	t.Helper()
	entry, ok := b.registered[name]
	if !ok {
		t.Fatalf("tool %q not registered on backend", name)
	}
	result, err := entry.handler(context.Background(), args)
	if err != nil {
		t.Fatalf("handler for %q returned error: %v", name, err)
	}
	return result
}

// B01: Register は backend.RegisterTool に ToolDef をそのまま渡し、
// backend 経由での呼び出しがツール本体 (ToolFunc) を実行することを検証する。
// MCP SDK の型を一切経由しない。
func TestRegister_DelegatesToBackend(t *testing.T) {
	mock := backlog.NewMockClient()
	backend := newFakeBackend()
	reg := mcpinternal.NewToolRegistryWithBackend(backend, mock, "")

	tool := mcpinternal.NewToolDef("backend_test_tool",
		mcpinternal.WithDesc("backend test"),
		mcpinternal.WithStringParam("value", true, "value"),
	)
	called := false
	reg.Register(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		called = true
		return map[string]string{"echo": args["value"].(string)}, nil
	})

	entry, ok := backend.registered["backend_test_tool"]
	if !ok {
		t.Fatal("expected tool to be registered on backend")
	}
	if entry.tool.Description != "backend test" {
		t.Errorf("unexpected description: %q", entry.tool.Description)
	}

	result := backend.call(t, "backend_test_tool", map[string]any{"value": "hello"})
	if !called {
		t.Error("expected tool function to be called")
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %+v", result)
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["echo"] != "hello" {
		t.Errorf("echo = %q, want hello", decoded["echo"])
	}
}

// B02: RegisterWithSpaces は resolver 未設定時、Register と同じく backend 経由で
// ツールを登録・実行し、spaces/all_spaces パラメータを注入する。
func TestRegisterWithSpaces_DelegatesToBackend_NoResolver(t *testing.T) {
	mock := backlog.NewMockClient()
	backend := newFakeBackend()
	reg := mcpinternal.NewToolRegistryWithBackend(backend, mock, "")

	tool := mcpinternal.NewToolDef("backend_space_tool", mcpinternal.WithDesc("space test"))
	reg.RegisterWithSpaces(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return "ok", nil
	})

	entry, ok := backend.registered["backend_space_tool"]
	if !ok {
		t.Fatal("expected tool to be registered on backend")
	}
	foundSpaces, foundAllSpaces := false, false
	for _, p := range entry.tool.Params {
		switch p.Name {
		case mcpinternal.ParamNameSpaces:
			foundSpaces = true
		case mcpinternal.ParamNameAllSpaces:
			foundAllSpaces = true
		}
	}
	if !foundSpaces || !foundAllSpaces {
		t.Errorf("expected spaces/all_spaces params injected, got %+v", entry.tool.Params)
	}

	result := backend.call(t, "backend_space_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error result: %+v", result)
	}
}

// B03: RegisterWithSpacesWrite も同様に backend 経由で登録・実行され、
// spaces パラメータ (単一指定用) を注入する。
func TestRegisterWithSpacesWrite_DelegatesToBackend_NoResolver(t *testing.T) {
	mock := backlog.NewMockClient()
	backend := newFakeBackend()
	reg := mcpinternal.NewToolRegistryWithBackend(backend, mock, "")

	tool := mcpinternal.NewToolDef("backend_space_write_tool", mcpinternal.WithDesc("space write test"))
	reg.RegisterWithSpacesWrite(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return "ok", nil
	})

	entry, ok := backend.registered["backend_space_write_tool"]
	if !ok {
		t.Fatal("expected tool to be registered on backend")
	}
	foundSpaces := false
	for _, p := range entry.tool.Params {
		if p.Name == mcpinternal.ParamNameSpaces {
			foundSpaces = true
		}
	}
	if !foundSpaces {
		t.Errorf("expected spaces param injected, got %+v", entry.tool.Params)
	}

	result := backend.call(t, "backend_space_write_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error result: %+v", result)
	}
}
