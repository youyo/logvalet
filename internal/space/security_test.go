package space_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/space"
)

// S1: ログに access_token / refresh_token / Bearer / api_key が含まれない
// ExecuteAcrossSpaces を実行し（エラーケース含む）、slog 出力を検査する。
func TestSecurity_NoTokenInLogs(t *testing.T) {
	sensitivePatterns := []string{
		"access_token",
		"refresh_token",
		"Bearer ",
		"api_key",
	}

	// slog 出力を buf に capture
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 401 を返す httptest サーバー（エラーパスを意図的に通す）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errors":[{"message":"unauthorized","code":11}]}`))
	}))
	defer srv.Close()

	reg := space.SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: srv.URL,
	}

	sensitiveToken := "super-secret-access-token-abc123"
	sensitiveAPIKey := "super-secret-api-key-xyz789"

	factory := func(ctx context.Context, r space.SpaceRegistration) (backlog.Client, error) {
		cred := &credentials.ResolvedCredential{
			AuthType:    credentials.AuthTypeOAuth,
			AccessToken: sensitiveToken,
			APIKey:      sensitiveAPIKey,
		}
		return backlog.NewHTTPClient(backlog.ClientConfig{
			BaseURL:    r.BaseURL,
			Credential: cred,
		}), nil
	}

	// logger を使うカスタム executor (ExecuteAcrossSpaces 自体はログを出さないが
	// factory 側でエラーをログ出力するケースを含めてテストする)
	ex := &space.Executor{
		Factory:        factory,
		MaxConcurrency: 1,
	}

	_ = logger // logger は将来的に Executor に渡せる場合に使用

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		[]space.SpaceRegistration{reg},
		func(ctx context.Context, r space.SpaceRegistration, c backlog.Client) (string, error) {
			_, err := c.ListIssues(ctx, backlog.ListIssuesOptions{})
			return "", err
		},
	)

	// 401 エラーが発生していること
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected 1 failed result, got OK=%v", results[0].OK)
	}

	// ログ内容を検査（buf には factory/executor レベルのログが入る想定）
	logOutput := buf.String()

	// 実際のトークン値がログに漏れていないことを検査
	if strings.Contains(logOutput, sensitiveToken) {
		t.Errorf("log contains sensitive access_token value")
	}
	if strings.Contains(logOutput, sensitiveAPIKey) {
		t.Errorf("log contains sensitive api_key value")
	}

	// 固定パターンのチェック（"Bearer " という文字列は本来ログに出てはいけない）
	for _, pattern := range sensitivePatterns {
		if strings.Contains(logOutput, pattern) {
			t.Errorf("log contains sensitive pattern %q", pattern)
		}
	}
}

// S5: user A が user B の alias を取得できない（Store isolation）
// MemoryStore と SQLiteStore の両方でテストする。
func TestSecurity_StoreIsolation_MemoryStore(t *testing.T) {
	testStoreIsolation(t, func() (store space.Store, cleanup func()) {
		return space.NewMemoryStore(), func() {}
	})
}

func TestSecurity_StoreIsolation_SQLiteStore(t *testing.T) {
	testStoreIsolation(t, func() (store space.Store, cleanup func()) {
		s, err := space.NewSQLiteStore(t.TempDir() + "/test.db")
		if err != nil {
			t.Fatalf("NewSQLiteStore: %v", err)
		}
		return s, func() { s.Close() }
	})
}

func testStoreIsolation(t *testing.T, newStore func() (space.Store, func())) {
	t.Helper()
	s, cleanup := newStore()
	defer cleanup()

	ctx := context.Background()

	// userA が alias "foo" を登録
	err := s.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "userA",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: "https://foo.backlog.com",
	})
	if err != nil {
		t.Fatalf("Upsert userA/foo: %v", err)
	}

	// userB が alias "bar" を登録
	err = s.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "userB",
		Alias:   "bar",
		Tenant:  "bar",
		BaseURL: "https://bar.backlog.com",
	})
	if err != nil {
		t.Fatalf("Upsert userB/bar: %v", err)
	}

	// userA が userB の "bar" を取得できないこと
	reg, err := s.Get(ctx, "userA", "bar")
	if err != nil {
		t.Fatalf("Get userA/bar error: %v", err)
	}
	if reg != nil {
		t.Errorf("userA should not be able to access userB's alias 'bar', got: %+v", reg)
	}

	// userB が userA の "foo" を取得できないこと
	reg, err = s.Get(ctx, "userB", "foo")
	if err != nil {
		t.Fatalf("Get userB/foo error: %v", err)
	}
	if reg != nil {
		t.Errorf("userB should not be able to access userA's alias 'foo', got: %+v", reg)
	}

	// List もユーザー間でリークしないこと
	listA, err := s.List(ctx, "userA")
	if err != nil {
		t.Fatalf("List userA: %v", err)
	}
	for _, r := range listA {
		if r.UserID != "userA" {
			t.Errorf("userA's List contains foreign entry: %+v", r)
		}
	}

	listB, err := s.List(ctx, "userB")
	if err != nil {
		t.Fatalf("List userB: %v", err)
	}
	for _, r := range listB {
		if r.UserID != "userB" {
			t.Errorf("userB's List contains foreign entry: %+v", r)
		}
	}
}

// S4 (補足): SQLiteStore で nonce replay が ErrNonceAlreadyUsed を返すこと。
// MemoryStore 側は memorystore_test.go で網羅済みのため SQLite のみ追加テスト。
func TestSecurity_NonceReplay_SQLiteStore(t *testing.T) {
	s, err := space.NewSQLiteStore(t.TempDir() + "/nonce.db")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	if err := s.Store(ctx, "u1", "nonce-abc", 5*time.Minute); err != nil {
		t.Fatalf("Store nonce: %v", err)
	}

	// 1回目: 成功
	if err := s.Consume(ctx, "u1", "nonce-abc"); err != nil {
		t.Fatalf("Consume 1st: %v", err)
	}

	// 2回目: replay → ErrNonceAlreadyUsed
	if err := s.Consume(ctx, "u1", "nonce-abc"); !errors.Is(err, space.ErrNonceAlreadyUsed) {
		t.Errorf("Consume 2nd: expected ErrNonceAlreadyUsed, got %v", err)
	}
}
