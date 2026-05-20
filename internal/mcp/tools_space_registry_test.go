package mcp_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
)

// newSpaceRegistryServer は space 管理 tools を登録した MCPServer を返すテストヘルパー。
func newSpaceRegistryServer(store space.Store, authBaseURL string) *mcpserver.MCPServer {
	return newSpaceRegistryServerFull(store, authBaseURL, nil, 0, "")
}

// newSpaceRegistryServerFull は bootstrap_token 設定付きでサーバーを構築するテストヘルパー。
func newSpaceRegistryServerFull(store space.Store, multiAuthURL string, bootstrapKey []byte, ttl time.Duration, _ string) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistry(s, nil, "")
	resolver := space.NewResolver(store)
	var ns space.NonceStore
	if store != nil {
		if nss, ok := store.(space.NonceStore); ok {
			ns = nss
		}
	}
	mcpinternal.RegisterSpaceRegistryTools(reg, store, resolver, multiAuthURL, bootstrapKey, ttl, ns)
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

// T4: logvalet_space_connect_url は bootstrap_token 付き authorization_url を返す
func TestLogvaletSpaceConnectUrl_ReturnsAuthURL(t *testing.T) {
	store := space.NewMemoryStore()
	multiAuthURL := "https://mcp.example.com/oauth/backlog/multi/authorize"
	s := newSpaceRegistryServerFull(store, multiAuthURL, btTestKey, 3*time.Minute, "")
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
	if !strings.Contains(authURL, "bootstrap_token=") {
		t.Errorf("authorization_url should contain bootstrap_token param, got %q", authURL)
	}
}

// T4b: base_url が未指定の場合はエラー
func TestLogvaletSpaceConnectUrl_MissingBaseURL_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServerFull(store, "https://mcp.example.com/oauth/backlog/multi/authorize", btTestKey, 3*time.Minute, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_connect_url", map[string]any{})
	if !result.IsError {
		t.Error("expected IsError=true for missing base_url")
	}
}

// T4c: multiAuthURL が設定されていない場合はエラー
func TestLogvaletSpaceConnectUrl_NoAuthBaseURL_Error(t *testing.T) {
	store := space.NewMemoryStore()
	s := newSpaceRegistryServerFull(store, "", btTestKey, 3*time.Minute, "")
	result := callToolWithCtx(t, s, ctxWithUser("u1"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://foo.backlog.com",
	})
	if !result.IsError {
		t.Error("expected IsError=true when multiAuthURL is not configured")
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

// -- Step 4 テスト: bootstrap_token 統合 --

var btTestKey = func() []byte {
	k, _ := auth.DeriveBootstrapKey(hex.EncodeToString([]byte("test-state-secret-for-bootstrap!!")))
	return k
}()

// TestSpaceConnectURL_IncludesBootstrapToken: bootstrap_token パラメータが URL に含まれること
func TestSpaceConnectURL_IncludesBootstrapToken(t *testing.T) {
	store := space.NewMemoryStore()
	multiAuthURL := "https://mcp.example.com/oauth/backlog/multi/authorize"
	s := newSpaceRegistryServerFull(store, multiAuthURL, btTestKey, 3*time.Minute, "")

	result := callToolWithCtx(t, s, ctxWithUser("user1"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://foo.backlog.com",
		"alias":    "foo",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	authURL, ok := out["authorization_url"].(string)
	if !ok || authURL == "" {
		t.Fatalf("expected authorization_url, got %v", out)
	}
	if !strings.Contains(authURL, "bootstrap_token=") {
		t.Errorf("authorization_url should contain bootstrap_token param, got %q", authURL)
	}
}

// TestSpaceConnectURL_BootstrapTokenValid: 生成された bootstrap_token が正当であること
func TestSpaceConnectURL_BootstrapTokenValid(t *testing.T) {
	store := space.NewMemoryStore()
	multiAuthURL := "https://mcp.example.com/oauth/backlog/multi/authorize"
	s := newSpaceRegistryServerFull(store, multiAuthURL, btTestKey, 3*time.Minute, "")

	result := callToolWithCtx(t, s, ctxWithUser("user2"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://bar.backlog.com",
		"alias":    "bar",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	authURL := out["authorization_url"].(string)
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	token := parsed.Query().Get("bootstrap_token")
	if token == "" {
		t.Fatal("bootstrap_token not found in URL")
	}

	userID, jti, err := auth.ValidateBootstrapToken(token, "https://bar.backlog.com", "bar", btTestKey)
	if err != nil {
		t.Fatalf("ValidateBootstrapToken: %v", err)
	}
	if userID != "user2" {
		t.Errorf("userID = %q, want %q", userID, "user2")
	}
	if jti == "" {
		t.Error("jti is empty")
	}

	// jti が NonceStore に Store されていること (Consume して確認)
	if err := store.Consume(context.Background(), "user2", "bs:"+jti); err != nil {
		t.Errorf("NonceStore.Consume: %v (jti should have been stored by spaceConnectURL)", err)
	}
}

// TestSpaceConnectURL_GenerateFailure_PropagatesError: bootstrapKey が nil の場合もエラーにならない（fail-safe）
func TestSpaceConnectURL_GenerateFailure_PropagatesError(t *testing.T) {
	store := space.NewMemoryStore()
	// bootstrapKey=nil でも fail-safe（bootstrap_token なしで URL を返す）
	s := newSpaceRegistryServerFull(store, "https://mcp.example.com/oauth/backlog/multi/authorize", nil, 3*time.Minute, "")

	result := callToolWithCtx(t, s, ctxWithUser("user3"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://foo.backlog.com",
		"alias":    "foo",
	})
	// bootstrapKey が未設定の場合はエラーとなることを確認（fail-safe）
	if !result.IsError {
		t.Error("expected IsError=true when bootstrapKey is not configured")
	}
}

// TestSpaceConnectURL_BaseURLNormalization: trailing slash を除去した base_url でトークンが検証できること
func TestSpaceConnectURL_BaseURLNormalization(t *testing.T) {
	store := space.NewMemoryStore()
	multiAuthURL := "https://mcp.example.com/oauth/backlog/multi/authorize"
	s := newSpaceRegistryServerFull(store, multiAuthURL, btTestKey, 3*time.Minute, "")

	// trailing slash 付きで送信
	result := callToolWithCtx(t, s, ctxWithUser("user4"), "logvalet_space_connect_url", map[string]any{
		"base_url": "https://baz.backlog.com/",
		"alias":    "baz",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	text := mustTextContent(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	authURL := out["authorization_url"].(string)
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	token := parsed.Query().Get("bootstrap_token")
	if token == "" {
		t.Fatal("bootstrap_token not found")
	}

	// trailing slash を除去した URL でトークン検証が通ること
	_, _, err = auth.ValidateBootstrapToken(token, "https://baz.backlog.com", "baz", btTestKey)
	if err != nil {
		t.Errorf("ValidateBootstrapToken with normalized URL: %v", err)
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
