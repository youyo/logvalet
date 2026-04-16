package tokenstore

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

// newTestSQLiteStore は t.TempDir() を使ったテスト用 SQLiteStore を作成する。
func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteStore_PutGet(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	rec := &auth.TokenRecord{
		UserID:         "user1",
		Provider:       "backlog",
		Tenant:         "example.backlog.com",
		AccessToken:    "access-token-12345678",
		RefreshToken:   "refresh-token-12345678",
		TokenType:      "Bearer",
		Scope:          "read write",
		Expiry:         now.Add(1 * time.Hour),
		ProviderUserID: "backlog-user-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected record, got nil")
	}

	// 各フィールドを検証
	if got.UserID != rec.UserID {
		t.Errorf("UserID: want %q, got %q", rec.UserID, got.UserID)
	}
	if got.Provider != rec.Provider {
		t.Errorf("Provider: want %q, got %q", rec.Provider, got.Provider)
	}
	if got.Tenant != rec.Tenant {
		t.Errorf("Tenant: want %q, got %q", rec.Tenant, got.Tenant)
	}
	if got.AccessToken != rec.AccessToken {
		t.Errorf("AccessToken: want %q, got %q", rec.AccessToken, got.AccessToken)
	}
	if got.RefreshToken != rec.RefreshToken {
		t.Errorf("RefreshToken: want %q, got %q", rec.RefreshToken, got.RefreshToken)
	}
	if got.TokenType != rec.TokenType {
		t.Errorf("TokenType: want %q, got %q", rec.TokenType, got.TokenType)
	}
	if got.Scope != rec.Scope {
		t.Errorf("Scope: want %q, got %q", rec.Scope, got.Scope)
	}
	if !got.Expiry.Equal(rec.Expiry) {
		t.Errorf("Expiry: want %v, got %v", rec.Expiry, got.Expiry)
	}
	if got.ProviderUserID != rec.ProviderUserID {
		t.Errorf("ProviderUserID: want %q, got %q", rec.ProviderUserID, got.ProviderUserID)
	}
	if !got.CreatedAt.Equal(rec.CreatedAt) {
		t.Errorf("CreatedAt: want %v, got %v", rec.CreatedAt, got.CreatedAt)
	}
	if !got.UpdatedAt.Equal(rec.UpdatedAt) {
		t.Errorf("UpdatedAt: want %v, got %v", rec.UpdatedAt, got.UpdatedAt)
	}
}

func TestSQLiteStore_GetNotFound(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	got, err := store.Get(ctx, "nonexistent", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("Get: expected nil, got %v", got)
	}
}

func TestSQLiteStore_PutUpsert(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	rec := &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "old-token-12345678",
		CreatedAt:   now,
		UpdatedAt:   now,
		Expiry:      now.Add(1 * time.Hour),
	}

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put (first): %v", err)
	}

	// 更新
	later := now.Add(10 * time.Minute)
	rec2 := &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "new-token-12345678",
		CreatedAt:   now,
		UpdatedAt:   later,
		Expiry:      now.Add(2 * time.Hour),
	}

	if err := store.Put(ctx, rec2); err != nil {
		t.Fatalf("Put (upsert): %v", err)
	}

	got, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected record, got nil")
	}
	if got.AccessToken != "new-token-12345678" {
		t.Errorf("AccessToken: want %q, got %q", "new-token-12345678", got.AccessToken)
	}
	if !got.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt: want %v, got %v", later, got.UpdatedAt)
	}
	// CreatedAt は初回のまま保持される
	if !got.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: want %v (original), got %v", now, got.CreatedAt)
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	rec := &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "access-token-12345678",
		Expiry:      time.Now().UTC().Add(1 * time.Hour),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := store.Delete(ctx, "user1", "backlog", "example.backlog.com"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if got != nil {
		t.Errorf("Get after Delete: expected nil, got %v", got)
	}
}

func TestSQLiteStore_DeleteNotFound(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	// 存在しないキーの Delete はエラーにならない
	err := store.Delete(ctx, "nonexistent", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Delete (not found): unexpected error: %v", err)
	}
}

func TestSQLiteStore_CloseIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close (second): %v", err)
	}
}

func TestSQLiteStore_OperationsAfterClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx := context.Background()

	// Close 後の Get はエラー
	_, err = store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err == nil {
		t.Error("Get after Close: expected error, got nil")
	}

	// Close 後の Put はエラー
	err = store.Put(ctx, &auth.TokenRecord{
		UserID:   "user1",
		Provider: "backlog",
		Tenant:   "example.backlog.com",
		Expiry:   time.Now().UTC().Add(1 * time.Hour),
	})
	if err == nil {
		t.Error("Put after Close: expected error, got nil")
	}

	// Close 後の Delete はエラー
	err = store.Delete(ctx, "user1", "backlog", "example.backlog.com")
	if err == nil {
		t.Error("Delete after Close: expected error, got nil")
	}
}

func TestSQLiteStore_UserIsolation(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	rec := &auth.TokenRecord{
		UserID:      "userA",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "token-A-12345678",
		Expiry:      time.Now().UTC().Add(1 * time.Hour),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put(userA): %v", err)
	}

	// userB で Get しても nil
	got, err := store.Get(ctx, "userB", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get(userB): %v", err)
	}
	if got != nil {
		t.Errorf("Get(userB): expected nil, got %v", got)
	}

	// 同じ provider/tenant でも別ユーザー
	got2, err := store.Get(ctx, "userA", "backlog", "other.backlog.com")
	if err != nil {
		t.Fatalf("Get(userA, other tenant): %v", err)
	}
	if got2 != nil {
		t.Errorf("Get(userA, other tenant): expected nil, got %v", got2)
	}
}

func TestSQLiteStore_ConcurrentWrites(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			rec := &auth.TokenRecord{
				UserID:      "user1",
				Provider:    "backlog",
				Tenant:      "example.backlog.com",
				AccessToken: "token-" + string(rune('A'+n)),
				Expiry:      time.Now().UTC().Add(1 * time.Hour),
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}
			// 並行書き込みでパニックやデータ競合が起きないことを確認
			_ = store.Put(ctx, rec)
		}(i)
	}

	wg.Wait()

	// 最終的に1レコードが存在する
	got, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected record after concurrent writes, got nil")
	}
}

func TestSQLiteStore_AutoCreateTable(t *testing.T) {
	// NewSQLiteStore が新しいDBファイルに対してテーブルを自動作成する
	dbPath := filepath.Join(t.TempDir(), "new.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// テーブルが存在するので Put/Get が動作する
	rec := &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "access-token-12345678",
		Expiry:      time.Now().UTC().Add(1 * time.Hour),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected record, got nil")
	}
}
