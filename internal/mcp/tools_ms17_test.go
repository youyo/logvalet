package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
)

// newMultiSpaceRegistry は multi-space 対応の ToolRegistry と MCPServer を返すテストヘルパー。
func newMultiSpaceRegistry(store space.Store) (*mcpserver.MCPServer, *mcpinternal.ToolRegistry) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		mock := backlog.NewMockClient()
		return mock, nil
	}
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)
	return s, reg
}

// newMultiSpaceRegistryWithFactory は factory 付きの multi-space ToolRegistry を返す。
func newMultiSpaceRegistryWithFactory(
	store space.Store,
	factory func(ctx context.Context) (backlog.Client, error),
	spaceFactory space.ClientFactory,
) (*mcpserver.MCPServer, *mcpinternal.ToolRegistry) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, factory, "", resolver, spaceFactory)
	return s, reg
}

// echoTool は引数に "echo_key" を含み、それをそのまま返すテスト用 ToolFunc。
var echoTool = gomcp.NewTool("echo_tool",
	gomcp.WithDescription("echo tool for testing"),
	gomcp.WithString("echo_key", gomcp.Description("value to echo")),
)

func echoFn(_ context.Context, _ backlog.Client, args map[string]any) (any, error) {
	v, _ := args["echo_key"]
	return map[string]any{"echoed": v}, nil
}

// decodeTextJSON は TextContent の JSON を map に変換するヘルパー。
func decodeTextJSON(t *testing.T, result *gomcp.CallToolResult) map[string]any {
	t.Helper()
	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	return out
}

// ============================================================================
// MS17-1: resolver=nil → 通常の Register と同等
// ============================================================================

func TestRegisterWithSpaces_NilResolver(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", nil, nil)

	reg.RegisterWithSpaces(echoTool, echoFn)

	// resolver=nil でも spaces 未指定で通常動作すること
	result := callToolWithCtx(t, s, context.Background(), "echo_tool", map[string]any{"echo_key": "hello"})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	out := decodeTextJSON(t, result)
	if out["echoed"] != "hello" {
		t.Errorf("expected echoed=hello, got %v", out["echoed"])
	}
}

// ============================================================================
// MS17-2: spaces 未指定 + all_spaces=false + resolver あり → 単一スペースを resolver 経由で解決
// ============================================================================

func TestRegisterWithSpaces_NoSpaces(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "foo", BaseURL: "https://foo.backlog.com", Status: space.SpaceStatusOK,
	})

	s, reg := newMultiSpaceRegistry(store)
	fnCalled := false
	tool := gomcp.NewTool("check_tool", gomcp.WithDescription("check"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		fnCalled = true
		return map[string]any{"ok": true}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "check_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if !fnCalled {
		t.Error("expected fn to be called")
	}
}

// ============================================================================
// MS17-3: spaces=["foo"] → 1スペース fan-out
// ============================================================================

func TestRegisterWithSpaces_SingleSpace(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "foo", BaseURL: "https://foo.backlog.com", Status: space.SpaceStatusOK,
	})

	calledWith := []string{}
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		calledWith = append(calledWith, reg.Alias)
		return backlog.NewMockClient(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("fanout_tool", gomcp.WithDescription("fanout"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return map[string]any{"result": "ok"}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "fanout_tool", map[string]any{
		"spaces": []any{"foo"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	// 結果は []space.Result[any] 形式
	text := mustTextContent(t, result)
	var results []map[string]any
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("failed to unmarshal results: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0]["space"] != "foo" {
		t.Errorf("expected space=foo, got %v", results[0]["space"])
	}
	if ok, _ := results[0]["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got %v", results[0]["ok"])
	}
	if len(calledWith) != 1 || calledWith[0] != "foo" {
		t.Errorf("expected spaceFactory called with foo, got %v", calledWith)
	}
}

// ============================================================================
// MS17-4: spaces=["foo","bar"] → 2スペース fan-out
// ============================================================================

func TestRegisterWithSpaces_MultiSpace(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "foo", BaseURL: "https://foo.backlog.com", Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "bar", BaseURL: "https://bar.backlog.com", Status: space.SpaceStatusOK,
	})

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("multi_tool", gomcp.WithDescription("multi"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return map[string]any{"result": "ok"}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "multi_tool", map[string]any{
		"spaces": []any{"foo", "bar"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var results []map[string]any
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("failed to unmarshal results: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// ============================================================================
// MS17-5: all_spaces=true → 全スペース fan-out
// ============================================================================

func TestRegisterWithSpaces_AllSpaces(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "foo", BaseURL: "https://foo.backlog.com", Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "bar", BaseURL: "https://bar.backlog.com", Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "baz", BaseURL: "https://baz.backlog.com", Status: space.SpaceStatusOK,
	})

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("all_tool", gomcp.WithDescription("all spaces"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return map[string]any{"result": "ok"}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "all_tool", map[string]any{
		"all_spaces": true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var results []map[string]any
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("failed to unmarshal results: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results (all spaces), got %d", len(results))
	}
}

// ============================================================================
// MS17-6: spaces + all_spaces 同時指定 → エラー
// ============================================================================

func TestRegisterWithSpaces_Conflict(t *testing.T) {
	store := space.NewMemoryStore()
	s, reg := newMultiSpaceRegistry(store)

	tool := gomcp.NewTool("conflict_tool", gomcp.WithDescription("conflict"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "conflict_tool", map[string]any{
		"spaces":     []any{"foo"},
		"all_spaces": true,
	})
	if !result.IsError {
		t.Error("expected IsError=true for spaces + all_spaces conflict")
	}
}

// ============================================================================
// MS17-7: RegisterWithSpacesWrite: spaces 未指定 + userID なし → default client fallback
// ============================================================================

func TestRegisterWithSpacesWrite_NoSpaces(t *testing.T) {
	store := space.NewMemoryStore()
	s, reg := newMultiSpaceRegistry(store)

	fnCalled := false
	tool := gomcp.NewTool("write_nospace_tool", gomcp.WithDescription("write no space"))
	reg.RegisterWithSpacesWrite(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		fnCalled = true
		return map[string]any{"written": true}, nil
	})

	result := callToolWithCtx(t, s, context.Background(), "write_nospace_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if !fnCalled {
		t.Error("expected fn to be called via fallback")
	}
}

// ============================================================================
// MS17-8: RegisterWithSpacesWrite: spaces=["foo"]（1件）→ OK
// ============================================================================

func TestRegisterWithSpacesWrite_SingleOK(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "foo", BaseURL: "https://foo.backlog.com", Status: space.SpaceStatusOK,
	})

	factoryAlias := ""
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		factoryAlias = reg.Alias
		return backlog.NewMockClient(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("write_single_tool", gomcp.WithDescription("write single"))
	reg.RegisterWithSpacesWrite(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return map[string]any{"written": true}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "write_single_tool", map[string]any{
		"spaces": []any{"foo"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if factoryAlias != "foo" {
		t.Errorf("expected spaceFactory called with foo, got %q", factoryAlias)
	}
	out := decodeTextJSON(t, result)
	if written, _ := out["written"].(bool); !written {
		t.Errorf("expected written=true, got %v", out["written"])
	}
}

// ============================================================================
// MS17-9: RegisterWithSpacesWrite: spaces=["foo","bar"]（複数）→ エラー
// ============================================================================

func TestRegisterWithSpacesWrite_MultiError(t *testing.T) {
	store := space.NewMemoryStore()
	s, reg := newMultiSpaceRegistry(store)

	tool := gomcp.NewTool("write_multi_tool", gomcp.WithDescription("write multi"))
	reg.RegisterWithSpacesWrite(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "write_multi_tool", map[string]any{
		"spaces": []any{"foo", "bar"},
	})
	if !result.IsError {
		t.Error("expected IsError=true for multi-space write")
	}
}

// ============================================================================
// MS17-10: RegisterWithSpacesWrite: all_spaces=true → エラー
// ============================================================================

func TestRegisterWithSpacesWrite_AllSpacesError(t *testing.T) {
	store := space.NewMemoryStore()
	s, reg := newMultiSpaceRegistry(store)

	tool := gomcp.NewTool("write_allspaces_tool", gomcp.WithDescription("write all spaces"))
	reg.RegisterWithSpacesWrite(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "write_allspaces_tool", map[string]any{
		"all_spaces": true,
	})
	if !result.IsError {
		t.Error("expected IsError=true for all_spaces in write operation")
	}
}
