package mcp_test

import (
	"context"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
)

// ============================================================================
// M20: RegisterWithSpaces が DynamoDB UserPreference を尊重すること
// ============================================================================

// TestRegisterWithSpaces_DefaultSpaceFromPreference は、spaces/all_spaces 未指定でも
// DynamoDB の UserPreference.DefaultSpaceAlias が使われることを検証する。
func TestRegisterWithSpaces_DefaultSpaceFromPreference(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "heptagon", BaseURL: "https://heptagon.backlog.com", Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp", Status: space.SpaceStatusOK,
	})
	_ = store.PutPreference(ctx, &space.UserPreference{UserID: "u1", DefaultSpaceAlias: "megumilog"})

	calledWith := ""
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		calledWith = reg.Alias
		return backlog.NewMockClient(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("pref_tool", gomcp.WithDescription("pref test"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return map[string]any{"ok": true}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "pref_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if calledWith != "megumilog" {
		t.Errorf("expected spaceFactory called with megumilog, got %q", calledWith)
	}
	// 単一スペース preference → 配列ではなく通常のオブジェクトレスポンス
	out := decodeTextJSON(t, result)
	if ok, _ := out["ok"].(bool); !ok {
		t.Errorf("expected ok=true in response, got %v", out)
	}
}

// TestRegisterWithSpacesWrite_DefaultSpaceFromPreference は、Write 操作においても
// spaces 未指定時に DynamoDB preference が尊重されることを検証する。
func TestRegisterWithSpacesWrite_DefaultSpaceFromPreference(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "heptagon", BaseURL: "https://heptagon.backlog.com", Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp", Status: space.SpaceStatusOK,
	})
	_ = store.PutPreference(ctx, &space.UserPreference{UserID: "u1", DefaultSpaceAlias: "megumilog"})

	calledWith := ""
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		calledWith = reg.Alias
		return backlog.NewMockClient(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("write_pref_tool", gomcp.WithDescription("write pref test"))
	reg.RegisterWithSpacesWrite(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return map[string]any{"written": true}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "write_pref_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if calledWith != "megumilog" {
		t.Errorf("expected spaceFactory called with megumilog, got %q", calledWith)
	}
	out := decodeTextJSON(t, result)
	if written, _ := out["written"].(bool); !written {
		t.Errorf("expected written=true in response, got %v", out)
	}
}

// TestRegisterWithSpaces_NoUserIDFallback は userID がコンテキストにない場合に
// fallback client（デフォルト動作）を使うことを検証する。
func TestRegisterWithSpaces_NoUserIDFallback(t *testing.T) {
	store := space.NewMemoryStore()
	_ = store.Upsert(context.Background(), &space.SpaceRegistration{
		UserID: "u1", Alias: "foo", BaseURL: "https://foo.backlog.com", Status: space.SpaceStatusOK,
	})

	s, reg := newMultiSpaceRegistry(store)
	fnCalled := false
	tool := gomcp.NewTool("nouserid_tool", gomcp.WithDescription("no user id"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		fnCalled = true
		return map[string]any{"ok": true}, nil
	})

	// userID なしのコンテキスト → defaultClient fallback
	result := callToolWithCtx(t, s, context.Background(), "nouserid_tool", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if !fnCalled {
		t.Error("expected fn to be called via fallback")
	}
}
