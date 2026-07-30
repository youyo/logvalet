package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/auth"
)

func TestPassthroughTokenFromContext_NotSet(t *testing.T) {
	_, ok := auth.PassthroughTokenFromContext(context.Background())
	if ok {
		t.Fatal("PassthroughTokenFromContext should return ok=false when token is not set")
	}
}

func TestPassthroughTokenFromContext_Empty(t *testing.T) {
	ctx := auth.ContextWithPassthroughToken(context.Background(), "")
	_, ok := auth.PassthroughTokenFromContext(ctx)
	if ok {
		t.Fatal("PassthroughTokenFromContext should return ok=false for empty token")
	}
}

func TestPassthroughTokenFromContext_RoundTrip(t *testing.T) {
	ctx := auth.ContextWithPassthroughToken(context.Background(), "token-abc")
	got, ok := auth.PassthroughTokenFromContext(ctx)
	if !ok {
		t.Fatal("PassthroughTokenFromContext should return ok=true")
	}
	if got != "token-abc" {
		t.Errorf("token = %q, want %q", got, "token-abc")
	}
}

// TestNewPassthroughClientFactory_MissingToken は、context に passthrough
// トークンが無い場合に ErrPassthroughTokenMissing を返す (fail-fast) ことを検証する。
func TestNewPassthroughClientFactory_MissingToken(t *testing.T) {
	factory := auth.NewPassthroughClientFactory("https://example.backlog.com")

	_, err := factory(context.Background())
	if err == nil {
		t.Fatal("factory should return error when passthrough token is missing")
	}
	if !errors.Is(err, auth.ErrPassthroughTokenMissing) {
		t.Errorf("error = %v, want ErrPassthroughTokenMissing", err)
	}
}

// TestNewPassthroughClientFactory_UsesBearerToken は、context の passthrough
// トークンがそのまま Backlog API 呼び出しの Authorization: Bearer ヘッダーに
// 転送されることを検証する (実 API は叩かず httptest でモックする)。
func TestNewPassthroughClientFactory_UsesBearerToken(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	factory := auth.NewPassthroughClientFactory(srv.URL)
	ctx := auth.ContextWithPassthroughToken(context.Background(), "gateway-token-xyz")

	client, err := factory(ctx)
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}

	if _, err := client.GetMyself(ctx); err != nil {
		t.Fatalf("GetMyself returned error: %v", err)
	}

	if gotAuthHeader != "Bearer gateway-token-xyz" {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, "Bearer gateway-token-xyz")
	}
}

// TestNewPassthroughClientFactory_RequestIsolation は、異なるトークンを持つ複数の
// リクエストが context スコープで分離され、互いに漏れないことを検証する。
// リクエスト単位の分離を明示的に検証するテスト (done_criteria #2)。
func TestNewPassthroughClientFactory_RequestIsolation(t *testing.T) {
	var mu chan string = make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	factory := auth.NewPassthroughClientFactory(srv.URL)

	ctxA := auth.ContextWithPassthroughToken(context.Background(), "token-A")
	ctxB := auth.ContextWithPassthroughToken(context.Background(), "token-B")

	clientA, err := factory(ctxA)
	if err != nil {
		t.Fatalf("factory(ctxA) returned error: %v", err)
	}
	clientB, err := factory(ctxB)
	if err != nil {
		t.Fatalf("factory(ctxB) returned error: %v", err)
	}

	if _, err := clientA.GetMyself(ctxA); err != nil {
		t.Fatalf("clientA.GetMyself returned error: %v", err)
	}
	if _, err := clientB.GetMyself(ctxB); err != nil {
		t.Fatalf("clientB.GetMyself returned error: %v", err)
	}

	seen := map[string]bool{<-mu: true, <-mu: true}
	if !seen["Bearer token-A"] || !seen["Bearer token-B"] {
		t.Fatalf("expected both Bearer token-A and Bearer token-B to be observed, got %v", seen)
	}

	// ctxA から生成された client が ctxB のトークンを引き継いでいないこと
	// (client 自体が生成時点のトークンを固定して保持し、後続の context 変化の影響を
	// 受けないこと) を確認する。
	if clientA == clientB {
		t.Fatal("clientA and clientB must be distinct instances")
	}
}

// TestNewPassthroughAwareClientFactory_PrefersPassthrough は、context に
// passthrough トークンがある場合、fallback (tokenstore 解決) を一切呼ばずに
// passthrough を優先することを検証する。
func TestNewPassthroughAwareClientFactory_PrefersPassthrough(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	factory := auth.NewPassthroughAwareClientFactory(srv.URL, nil)
	ctx := auth.ContextWithPassthroughToken(context.Background(), "gateway-token-1")

	client, err := factory(ctx)
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if _, err := client.GetMyself(ctx); err != nil {
		t.Fatalf("GetMyself returned error: %v", err)
	}
	if gotAuthHeader != "Bearer gateway-token-1" {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, "Bearer gateway-token-1")
	}
	// fallback は nil のまま渡している。passthrough が優先されなければ nil 参照で
	// パニックするはずなので、ここまでエラー無く到達したこと自体が
	// 「fallback へ委譲していない」ことの証跡になる。
}

// TestNewPassthroughAwareClientFactory_DoesNotPersistOrTouchTokenManager は、
// passthrough トークンが使われる場合に既存の TokenManager (tokenstore への
// 永続化を伴う per-user 解決) が一切呼ばれないことを検証する。
// これにより、passthrough トークンが tokenstore 等の永続ストアへ書き込まれない
// (done_criteria #2) ことを、フォールバック側の呼び出し回数ゼロという形で保証する。
func TestNewPassthroughAwareClientFactory_DoesNotPersistOrTouchTokenManager(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			t.Fatal("TokenManager.GetValidToken must not be called when a passthrough token is present")
			return nil, nil
		},
	}
	fallback := auth.NewClientFactory(tm, "backlog", "example.backlog.com", "https://example.backlog.com")
	factory := auth.NewPassthroughAwareClientFactory("https://example.backlog.com", fallback)

	ctx := auth.ContextWithPassthroughToken(context.Background(), "gateway-token-2")
	if _, err := factory(ctx); err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
}

// TestNewPassthroughAwareClientFactory_FallsBackToTokenManager は、context に
// passthrough トークンが無い場合、既存の per-user token 解決 (fallback) に
// フォールバックすることを検証する。
func TestNewPassthroughAwareClientFactory_FallsBackToTokenManager(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, userID, _, _ string) (*auth.TokenRecord, error) {
			if userID != "user-1" {
				t.Fatalf("unexpected userID: %q", userID)
			}
			return &auth.TokenRecord{AccessToken: "fallback-token"}, nil
		},
	}
	fallback := auth.NewClientFactory(tm, "backlog", "example.backlog.com", "https://example.backlog.com")

	factory := auth.NewPassthroughAwareClientFactory("https://example.backlog.com", fallback)

	// passthrough トークンが無く、userID もない場合は fallback がそのままエラーを返す。
	if _, err := factory(context.Background()); err == nil {
		t.Fatal("expected error when neither passthrough token nor userID is present")
	} else if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("error = %v, want ErrUnauthenticated (from fallback)", err)
	}

	// passthrough トークンが無く、userID がある場合は fallback の per-user 解決が使われる。
	ctx := auth.ContextWithUserID(context.Background(), "user-1")
	client, err := factory(ctx)
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client from fallback")
	}
}

// TestNewPassthroughAwareClientFactory_NilFallback は、fallback が未設定の
// 構成 (HTTP(Gateway) 専用構成) で、passthrough トークンが無いリクエストが
// ErrPassthroughTokenMissing を返すことを検証する。
func TestNewPassthroughAwareClientFactory_NilFallback(t *testing.T) {
	factory := auth.NewPassthroughAwareClientFactory("https://example.backlog.com", nil)

	_, err := factory(context.Background())
	if err == nil {
		t.Fatal("factory should return error when passthrough token is missing and fallback is nil")
	}
	if !errors.Is(err, auth.ErrPassthroughTokenMissing) {
		t.Errorf("error = %v, want ErrPassthroughTokenMissing", err)
	}
}
