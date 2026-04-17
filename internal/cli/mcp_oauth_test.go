package cli_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	tokenstore "github.com/youyo/logvalet/internal/auth/tokenstore"
	"github.com/youyo/logvalet/internal/cli"
)

// ---------------------------------------------------------------------
// ヘルパー
// ---------------------------------------------------------------------

// newValidOAuthCfg は buildOAuthDeps に通せる最小構成の OAuthEnvConfig を返す。
// hex 32 chars = 16 bytes で最小要件を満たす。
func newValidOAuthCfg() *auth.OAuthEnvConfig {
	return &auth.OAuthEnvConfig{
		TokenStoreType:      auth.StoreTypeMemory,
		BacklogClientID:     "test-client-id",
		BacklogClientSecret: "test-client-secret",
		BacklogRedirectURL:  "https://example.com/oauth/backlog/callback",
		OAuthStateSecret:    strings.Repeat("ab", 16), // 32 hex chars = 16 bytes
	}
}

// fakeTokenManager は auth.TokenManager のテスト用モック。
type fakeTokenManager struct {
	getFn    func(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error)
	saveFn   func(ctx context.Context, record *auth.TokenRecord) error
	revokeFn func(ctx context.Context, userID, provider, tenant string) error
}

func (f *fakeTokenManager) GetValidToken(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
	if f.getFn != nil {
		return f.getFn(ctx, userID, provider, tenant)
	}
	return nil, auth.ErrProviderNotConnected
}

func (f *fakeTokenManager) SaveToken(ctx context.Context, record *auth.TokenRecord) error {
	if f.saveFn != nil {
		return f.saveFn(ctx, record)
	}
	return nil
}

func (f *fakeTokenManager) RevokeToken(ctx context.Context, userID, provider, tenant string) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, userID, provider, tenant)
	}
	return nil
}

// ---------------------------------------------------------------------
// A. buildOAuthDeps テスト
// ---------------------------------------------------------------------

func TestBuildOAuthDeps_NilConfig(t *testing.T) {
	_, err := cli.BuildOAuthDeps(nil, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
}

func TestBuildOAuthDeps_EmptySpace(t *testing.T) {
	cfg := newValidOAuthCfg()
	_, err := cli.BuildOAuthDeps(cfg, "", "https://test-space.backlog.com", "https://example.com", nil)
	if err == nil {
		t.Fatal("expected error for empty space, got nil")
	}
}

func TestBuildOAuthDeps_InvalidHex(t *testing.T) {
	cfg := newValidOAuthCfg()
	cfg.OAuthStateSecret = "not-hex!!"
	_, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err == nil {
		t.Fatal("expected error for invalid hex secret, got nil")
	}
}

func TestBuildOAuthDeps_EmptyExternalURL(t *testing.T) {
	cfg := newValidOAuthCfg()
	_, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "", nil)
	if err == nil {
		t.Fatal("expected error for empty externalURL, got nil")
	}
}

func TestBuildOAuthDeps_AuthorizeURL(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com/", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps returned error: %v", err)
	}
	want := "https://example.com/oauth/backlog/authorize"
	if deps.AuthorizeURL != want {
		t.Errorf("deps.AuthorizeURL = %q, want %q", deps.AuthorizeURL, want)
	}
}

func TestBuildOAuthDeps_MemoryStore(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps returned error: %v", err)
	}
	if deps == nil {
		t.Fatal("deps is nil")
	}
	if deps.Store == nil {
		t.Error("deps.Store is nil")
	}
	if deps.Handler == nil {
		t.Error("deps.Handler is nil")
	}
	if deps.Factory == nil {
		t.Error("deps.Factory is nil")
	}
	if deps.TokenManager == nil {
		t.Error("deps.TokenManager is nil")
	}
	// MemoryStore が返っていること（型 assert）
	if _, ok := deps.Store.(*tokenstore.MemoryStore); !ok {
		t.Errorf("deps.Store is not *tokenstore.MemoryStore, got %T", deps.Store)
	}
}

func TestBuildOAuthDeps_ProviderRegistered(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps returned error: %v", err)
	}
	if deps.Provider == nil {
		t.Fatal("deps.Provider is nil")
	}
	if got := deps.Provider.Name(); got != "backlog" {
		t.Errorf("deps.Provider.Name() = %q, want %q", got, "backlog")
	}
}

func TestBuildOAuthDeps_Close_MemoryStore(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps returned error: %v", err)
	}
	if err := deps.Close(); err != nil {
		t.Errorf("deps.Close() returned error: %v", err)
	}
	// 冪等 (2 回目の Close もエラーにしない)
	if err := deps.Close(); err != nil {
		t.Errorf("second deps.Close() returned error: %v", err)
	}
}

func TestOAuthDeps_Close_NilReceiver(t *testing.T) {
	var deps *cli.OAuthDeps
	if err := deps.Close(); err != nil {
		t.Errorf("nil deps.Close() returned error: %v", err)
	}
}

// ---------------------------------------------------------------------
// B. installOAuthRoutes テスト
// ---------------------------------------------------------------------

func TestInstallOAuthRoutes_AuthorizeRouted(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps: %v", err)
	}
	defer deps.Close()

	mux := http.NewServeMux()
	cli.InstallOAuthRoutes(mux, deps.Handler)

	req := httptest.NewRequest(http.MethodGet, "/oauth/backlog/authorize", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "test-space.backlog.com") {
		t.Errorf("Location = %q, want to contain backlog OAuth URL", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("Location = %q, want to contain state", loc)
	}
}

func TestInstallOAuthRoutes_CallbackRouted(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps: %v", err)
	}
	defer deps.Close()

	mux := http.NewServeMux()
	cli.InstallOAuthRoutes(mux, deps.Handler)

	// state/code なしなので 400 invalid_request が返る
	req := httptest.NewRequest(http.MethodGet, "/oauth/backlog/callback", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestInstallOAuthRoutes_StatusRouted(t *testing.T) {
	// handler を fakeTM で差し替えた deps を直接手組みする
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps: %v", err)
	}
	defer deps.Close()

	mux := http.NewServeMux()
	cli.InstallOAuthRoutes(mux, deps.Handler)

	req := httptest.NewRequest(http.MethodGet, "/oauth/backlog/status", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// memory store 未設定なので connected:false
	if connected, _ := body["connected"].(bool); connected {
		t.Errorf("body[connected] = true, want false (store is empty)")
	}
}

func TestInstallOAuthRoutes_DisconnectRouted(t *testing.T) {
	cfg := newValidOAuthCfg()
	deps, err := cli.BuildOAuthDeps(cfg, "test-space", "https://test-space.backlog.com", "https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildOAuthDeps: %v", err)
	}
	defer deps.Close()

	mux := http.NewServeMux()
	cli.InstallOAuthRoutes(mux, deps.Handler)

	req := httptest.NewRequest(http.MethodDelete, "/oauth/backlog/disconnect", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if status, _ := body["status"].(string); status != "disconnected" {
		t.Errorf("body[status] = %q, want %q", status, "disconnected")
	}
}

// ---------------------------------------------------------------------
// C. bridgeFromUserIDFn テスト（bridge 単体）
// ---------------------------------------------------------------------

func TestBridgeFromUserIDFn_InjectsUserID(t *testing.T) {
	mw := cli.BridgeFromUserIDFn(func(ctx context.Context) string { return "alice" })

	var capturedUserID string
	var ok bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID, ok = auth.UserIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mw(inner).ServeHTTP(rec, req)

	if !ok {
		t.Fatal("userID was not injected into context")
	}
	if capturedUserID != "alice" {
		t.Errorf("userID = %q, want %q", capturedUserID, "alice")
	}
}

func TestBridgeFromUserIDFn_NilFn_PassThrough(t *testing.T) {
	mw := cli.BridgeFromUserIDFn(nil)

	var ok bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = auth.UserIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mw(inner).ServeHTTP(rec, req)

	if ok {
		t.Error("userID should not be injected when fn is nil")
	}
}

func TestBridgeFromUserIDFn_EmptyString_PassThrough(t *testing.T) {
	mw := cli.BridgeFromUserIDFn(func(ctx context.Context) string { return "" })

	var ok bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = auth.UserIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mw(inner).ServeHTTP(rec, req)

	if ok {
		t.Error("userID should not be injected when fn returns empty string")
	}
}

// ---------------------------------------------------------------------
// C-bis. bridge → innerMux → factory 直結テスト（M16 最重要動作）
// ---------------------------------------------------------------------

// TestBridge_PropagatesUserIDToInnerHandler は bridge → innerMux → dummyHandler の
// 順で userID が伝播することを検証する。
// bridge を外側に置いた場合に検知できないバグを防ぐ専用テスト。
func TestBridge_PropagatesUserIDToInnerHandler(t *testing.T) {
	mw := cli.BridgeFromUserIDFn(func(ctx context.Context) string { return "alice" })

	innerMux := http.NewServeMux()
	innerMux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		uid, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "no userID", http.StatusUnauthorized)
			return
		}
		w.Header().Set("X-User-ID", uid)
		w.WriteHeader(http.StatusOK)
	})

	// bridge は innerMux を wrap する
	wrapped := mw(innerMux)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("X-User-ID"); got != "alice" {
		t.Errorf("X-User-ID = %q, want %q", got, "alice")
	}
}

// TestBridge_PropagatesUserIDToClientFactory は bridge → ハンドラー → ClientFactory
// の全経路で userID が伝播することを検証する。
// fakeTokenManager の getFn 引数を capture して userID が正しく渡っているかを確認。
func TestBridge_PropagatesUserIDToClientFactory(t *testing.T) {
	var capturedUserID string
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			capturedUserID = userID
			return &auth.TokenRecord{
				UserID:      userID,
				Provider:    providerName,
				Tenant:      tenant,
				AccessToken: "test-access-token",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	factory := auth.NewClientFactory(tm, "backlog", "test-space", "https://test-space.backlog.com")

	// bridge が userID を inject → ハンドラー内で factory(ctx) を呼ぶ
	mw := cli.BridgeFromUserIDFn(func(ctx context.Context) string { return "bob" })
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := factory(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if capturedUserID != "bob" {
		t.Errorf("captured userID = %q, want %q", capturedUserID, "bob")
	}
}

// TestBridge_WithoutBridge_NoUserID は bridge を外すと userID が届かないことを確認する
// ネガティブテスト。「bridge 必須」の性質を明示的に保証する。
func TestBridge_WithoutBridge_NoUserID(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := auth.UserIDFromContext(r.Context())
		if ok {
			http.Error(w, "userID unexpectedly present", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// bridge を通さず直接呼ぶ
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	inner.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}
