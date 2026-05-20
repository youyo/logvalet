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

// ============================================================================
// M22: multi-space metadata fix — context-aware SpaceRegistration 伝搬
// ============================================================================

// decodeTextJSONArray は TextContent の JSON 配列を []map[string]any に変換するヘルパー。
func decodeTextJSONArray(t *testing.T, result *gomcp.CallToolResult) []map[string]any {
	t.Helper()
	var text string
	for _, c := range result.Content {
		if tc, ok := c.(gomcp.TextContent); ok {
			text = tc.Text
			break
		}
	}
	if text == "" {
		t.Fatalf("no TextContent in result")
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal array: %v", err)
	}
	return out
}

// assertInnerSpaceMetadata は fan-out Result の inner Value（map）が期待する
// space / base_url を持つことを検証する。
func assertInnerSpaceMetadata(t *testing.T, r map[string]any, wantAlias, wantBaseURL string) {
	t.Helper()
	inner, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result.result to be map, got %T", r["result"])
	}
	if got, _ := inner["alias"].(string); got != wantAlias {
		t.Errorf("inner alias: want %q, got %q", wantAlias, got)
	}
	if got, _ := inner["base_url"].(string); got != wantBaseURL {
		t.Errorf("inner base_url: want %q, got %q", wantBaseURL, got)
	}
}

// ----------------------------------------------------------------------------
// T2: 単一スペースパス（callWithSpaceClient）で context にスペースが注入される
// ----------------------------------------------------------------------------

func TestRegisterWithSpaces_SingleSpace_BuilderSeesRegistrationMetadata(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp",
		Status: space.SpaceStatusOK,
	})

	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	var seenAlias, seenBaseURL string
	tool := gomcp.NewTool("m22_single", gomcp.WithDescription("m22 single"))
	reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		seenAlias, seenBaseURL = mcpinternal.SpaceInfoFromContextForTest(ctx, "fallback", "https://fallback.example")
		return map[string]any{"ok": true}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "m22_single", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if seenAlias != "megumilog" || seenBaseURL != "https://megumilog.backlog.jp" {
		t.Errorf("expected reg from context, got alias=%q baseURL=%q", seenAlias, seenBaseURL)
	}
}

// ----------------------------------------------------------------------------
// T3-1: fan-out (spaces=[...]) で各クロージャが独立した reg を context から取得
// ----------------------------------------------------------------------------

func TestRegisterWithSpaces_FanOut_EachClosureSeesItsOwnRegistration(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "heptagon", BaseURL: "https://heptagon.backlog.com",
		Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp",
		Status: space.SpaceStatusOK,
	})

	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("m22_fanout", gomcp.WithDescription("m22 fanout"))
	reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		alias, baseURL := mcpinternal.SpaceInfoFromContextForTest(ctx, "fallback", "https://fallback")
		return map[string]any{"alias": alias, "base_url": baseURL}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "m22_fanout", map[string]any{
		"spaces": []any{"megumilog", "heptagon"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	raw := decodeTextJSONArray(t, result)
	if len(raw) != 2 {
		t.Fatalf("expected 2 results, got %d", len(raw))
	}

	// 入力順保持を仮定せず、alias → result の対応を検証（T3-2 と同じパターン）
	gotByAlias := map[string]map[string]any{}
	for _, r := range raw {
		alias, _ := r["space"].(string)
		gotByAlias[alias] = r
	}

	want := map[string]string{
		"megumilog": "https://megumilog.backlog.jp",
		"heptagon":  "https://heptagon.backlog.com",
	}
	for alias, baseURL := range want {
		r, ok := gotByAlias[alias]
		if !ok {
			t.Fatalf("missing result for space %q", alias)
		}
		// outer
		if got, _ := r["base_url"].(string); got != baseURL {
			t.Errorf("outer[%s].base_url: want %q, got %q", alias, baseURL, got)
		}
		// inner Value: result[i].result.alias / base_url（context から取得した値）
		assertInnerSpaceMetadata(t, r, alias, baseURL)
	}
}

// ----------------------------------------------------------------------------
// T3-2: fan-out (all_spaces=true) パスでも各クロージャに独立した reg が伝搬
// ----------------------------------------------------------------------------

func TestRegisterWithSpaces_AllSpaces_EachClosureSeesItsOwnRegistration(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "alpha", BaseURL: "https://alpha.backlog.com",
		Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "bravo", BaseURL: "https://bravo.backlog.jp",
		Status: space.SpaceStatusOK,
	})

	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	tool := gomcp.NewTool("m22_allspaces", gomcp.WithDescription("m22 allspaces"))
	reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		alias, baseURL := mcpinternal.SpaceInfoFromContextForTest(ctx, "fb", "https://fb")
		return map[string]any{"alias": alias, "base_url": baseURL}, nil
	})

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "m22_allspaces", map[string]any{
		"all_spaces": true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	raw := decodeTextJSONArray(t, result)
	if len(raw) != 2 {
		t.Fatalf("expected 2 results, got %d", len(raw))
	}

	// 入力順保持を仮定せず、alias → inner alias/base_url の対応を検証
	gotByAlias := map[string]map[string]any{}
	for _, r := range raw {
		alias, _ := r["space"].(string)
		gotByAlias[alias] = r
	}

	want := map[string]string{
		"alpha": "https://alpha.backlog.com",
		"bravo": "https://bravo.backlog.jp",
	}
	for alias, baseURL := range want {
		r, ok := gotByAlias[alias]
		if !ok {
			t.Fatalf("missing result for space %q", alias)
		}
		// outer
		if got, _ := r["base_url"].(string); got != baseURL {
			t.Errorf("outer[%s].base_url: want %q, got %q", alias, baseURL, got)
		}
		// inner
		assertInnerSpaceMetadata(t, r, alias, baseURL)
	}
}

// ----------------------------------------------------------------------------
// T5: 後方互換 — resolver=nil の場合は context にスペースなしで fallback が効く
// ----------------------------------------------------------------------------

func TestRegister_NoMultiSpace_FallsBackToCfgSpace(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistry(s, backlog.NewMockClient(), "")

	var seenAlias, seenBaseURL string
	tool := gomcp.NewTool("m22_legacy", gomcp.WithDescription("legacy"))
	reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		seenAlias, seenBaseURL = mcpinternal.SpaceInfoFromContextForTest(ctx, "cfg-space", "https://cfg.example")
		return map[string]any{"ok": true}, nil
	})

	result := callToolWithCtx(t, s, context.Background(), "m22_legacy", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if seenAlias != "cfg-space" || seenBaseURL != "https://cfg.example" {
		t.Errorf("expected fallback to cfg values, got alias=%q baseURL=%q", seenAlias, seenBaseURL)
	}
}
