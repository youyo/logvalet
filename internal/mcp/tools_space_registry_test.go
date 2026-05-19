package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
)

// newSpaceRegistryServer は space 管理 tools を登録した MCPServer を返すテストヘルパー。
func newSpaceRegistryServer(store space.Store, authBaseURL string) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistry(s, nil, "")
	resolver := space.NewResolver(store)
	mcpinternal.RegisterSpaceRegistryTools(reg, store, resolver, authBaseURL)
	return s
}

// ctxWithUser は userID を含む context を返すテストヘルパー。
func ctxWithUser(userID string) context.Context {
	return auth.ContextWithUserID(context.Background(), userID)
}

// T1: logvalet_space_list は現在ユーザーのスペース一覧を返す
func TestLogvaletSpaceList_ReturnsUserSpaces(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: "https://foo.backlog.com",
		Status:  space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "bar",
		Tenant:  "bar",
		BaseURL: "https://bar.backlog.com",
		Status:  space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "u2",
		Alias:   "baz",
		Tenant:  "baz",
		BaseURL: "https://baz.backlog.com",
		Status:  space.SpaceStatusOK,
	})

	s := newSpaceRegistryServer(store, "https://mcp.example.com")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_list", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	spaces, ok := out["spaces"].([]any)
	if !ok {
		t.Fatalf("expected spaces array, got %T: %v", out["spaces"], out["spaces"])
	}
	if len(spaces) != 2 {
		t.Errorf("expected 2 spaces for u1, got %d", len(spaces))
	}

	// u2 のスペースが含まれていないことを確認
	raw, _ := json.Marshal(spaces)
	if strings.Contains(string(raw), "baz") {
		t.Error("u2's space 'baz' must not appear in u1's result")
	}
}

// T1b: userID が context にない場合はエラー
func TestLogvaletSpaceList_NoUserID_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, context.Background(), "logvalet_space_list", map[string]any{})
	if !result.IsError {
		t.Error("expected IsError=true when userID not in context")
	}
}

// T2: logvalet_space_use は default space を更新する
func TestLogvaletSpaceUse_ChangesDefault(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := ctxWithUser("u1")

	_ = store.Upsert(context.Background(), &space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: "https://foo.backlog.com",
		Status:  space.SpaceStatusOK,
	})

	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, ctx, "logvalet_space_use", map[string]any{"alias": "foo"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	pref, err := store.GetPreference(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if pref == nil || pref.DefaultSpaceAlias != "foo" {
		t.Errorf("expected DefaultSpaceAlias=foo, got %v", pref)
	}
}

// T2b: alias が未指定の場合はエラー
func TestLogvaletSpaceUse_MissingAlias_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_use", map[string]any{})
	if !result.IsError {
		t.Error("expected IsError=true for missing alias")
	}
}

// T3: logvalet_space_verify は各スペースの接続状態を確認する
func TestLogvaletSpaceVerify_AllSpaces(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: "https://foo.backlog.com",
		Status:  space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "bar",
		Tenant:  "bar",
		BaseURL: "https://bar.backlog.com",
		Status:  space.SpaceStatusNotConnected,
	})

	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_verify", map[string]any{"all_spaces": true})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	results, ok := out["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", out["results"])
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// T4: logvalet_space_connect_url は authorization_url を返す
func TestLogvaletSpaceConnectUrl_ReturnsAuthURL(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServer(store, "https://mcp.example.com")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://foo.backlog.com",
		"alias":    "foo",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	authURL, ok := out["authorization_url"].(string)
	if !ok || authURL == "" {
		t.Fatalf("expected authorization_url string, got %v", out["authorization_url"])
	}
	if !strings.HasPrefix(authURL, "https://mcp.example.com") {
		t.Errorf("authorization_url should start with base URL, got %q", authURL)
	}
	if !strings.Contains(authURL, "base_url=") {
		t.Errorf("authorization_url should contain base_url param, got %q", authURL)
	}
}

// T4b: base_url が未指定の場合はエラー
func TestLogvaletSpaceConnectUrl_MissingBaseURL_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServer(store, "https://mcp.example.com")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_connect_url", map[string]any{})
	if !result.IsError {
		t.Error("expected IsError=true for missing base_url")
	}
}

// T4c: authBaseURL が設定されていない場合はエラー
func TestLogvaletSpaceConnectUrl_NoAuthBaseURL_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://foo.backlog.com",
	})
	if !result.IsError {
		t.Error("expected IsError=true when authBaseURL is not configured")
	}
}

// T5: logvalet_space_disconnect はスペースを削除する
func TestLogvaletSpaceDisconnect_RemovesSpace(t *testing.T) {
	store := space.NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: "https://foo.backlog.com",
		Status:  space.SpaceStatusOK,
	})

	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_disconnect", map[string]any{"alias": "foo"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	reg, err := store.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reg != nil {
		t.Error("expected space to be deleted, but it still exists")
	}
}

// T5b: alias が未指定の場合はエラー
func TestLogvaletSpaceDisconnect_MissingAlias_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServer(store, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_disconnect", map[string]any{})
	if !result.IsError {
		t.Error("expected IsError=true for missing alias")
	}
}

// mustTextContent は *gomcp.CallToolResult の最初の TextContent.Text を返すヘルパー。
func mustTextContent(t *testing.T, result *gomcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}
	tc, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}
