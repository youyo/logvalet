package space

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// T1: Upsert と Get
func TestSQLiteStore_UpsertAndGet(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	reg := &SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "example",
		BaseURL: "https://example.backlog.com",
	}
	if err := s.Upsert(ctx, reg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.UserID != "u1" || got.Alias != "foo" || got.Tenant != "example" {
		t.Errorf("unexpected value: %+v", got)
	}
}

// T2: Upsert が上書きになる
func TestSQLiteStore_UpsertOverwrite(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "old"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "new"})

	got, err := s.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Tenant != "new" {
		t.Errorf("expected Tenant=new, got %s", got.Tenant)
	}
}

// T3: List のユーザー分離
func TestSQLiteStore_List_UserIsolation(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u2", Alias: "foo"})

	u1List, err := s.List(ctx, "u1")
	if err != nil {
		t.Fatalf("List u1: %v", err)
	}
	if len(u1List) != 2 {
		t.Errorf("expected 2 for u1, got %d", len(u1List))
	}
	for _, r := range u1List {
		if r.UserID != "u1" {
			t.Errorf("u2 data leaked into u1 List: %+v", r)
		}
	}

	u2List, err := s.List(ctx, "u2")
	if err != nil {
		t.Fatalf("List u2: %v", err)
	}
	if len(u2List) != 1 {
		t.Errorf("expected 1 for u2, got %d", len(u2List))
	}
}

// T4: List が空のとき nil でなく空スライスを返す
func TestSQLiteStore_List_Empty(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	result, err := s.List(ctx, "nobody")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %d items", len(result))
	}
}

// T5: Delete が対象のみ削除する
func TestSQLiteStore_Delete_TargetOnly(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})

	if err := s.Delete(ctx, "u1", "foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get foo: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete, got value")
	}

	got, err = s.Get(ctx, "u1", "bar")
	if err != nil {
		t.Fatalf("Get bar: %v", err)
	}
	if got == nil {
		t.Error("bar should still exist")
	}
}

// T6: Delete が異なるユーザーの同名エイリアスを削除しない
func TestSQLiteStore_Delete_DifferentUserSameAlias(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u2", Alias: "foo"})

	if err := s.Delete(ctx, "u1", "foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "u2", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Error("u2/foo should still exist after deleting u1/foo")
	}
}

// T7: Delete が存在しないキーでエラーにならない
func TestSQLiteStore_Delete_NotExist(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := s.Delete(ctx, "u1", "nonexistent"); err != nil {
		t.Errorf("Delete of nonexistent should not error, got: %v", err)
	}
}

// T8: Preference の GetPut
func TestSQLiteStore_Preference_GetPut(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	pref := &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"}
	if err := s.PutPreference(ctx, pref); err != nil {
		t.Fatalf("PutPreference: %v", err)
	}

	got, err := s.GetPreference(ctx, "u1")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if got == nil {
		t.Fatal("GetPreference returned nil")
	}
	if got.DefaultSpaceAlias != "foo" {
		t.Errorf("expected foo, got %s", got.DefaultSpaceAlias)
	}
}

// T9: Preference が未設定のとき nil を返す
func TestSQLiteStore_Preference_GetNotExist(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	got, err := s.GetPreference(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetPreference should not error for unset user, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unset user, got %+v", got)
	}
}

// T10: Preference のユーザー分離
func TestSQLiteStore_Preference_UserIsolation(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	s.PutPreference(ctx, &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"})
	s.PutPreference(ctx, &UserPreference{UserID: "u2", DefaultSpaceAlias: "bar"})

	p1, _ := s.GetPreference(ctx, "u1")
	p2, _ := s.GetPreference(ctx, "u2")

	if p1.DefaultSpaceAlias != "foo" {
		t.Errorf("u1: expected foo, got %s", p1.DefaultSpaceAlias)
	}
	if p2.DefaultSpaceAlias != "bar" {
		t.Errorf("u2: expected bar, got %s", p2.DefaultSpaceAlias)
	}
}

// T11: Close が冪等
func TestSQLiteStore_Close_Idempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close 1st: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close 2nd: %v", err)
	}
}

// T12: 並行 Upsert がレースしない
func TestSQLiteStore_Concurrent_Upsert(t *testing.T) {
	t.Parallel()
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		alias := string(rune('a' + i))
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: a})
		}(alias)
	}
	wg.Wait()

	list, err := s.List(ctx, "u1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 10 {
		t.Errorf("expected 10, got %d", len(list))
	}
}

// T13: NonceStore の Store & Consume（1回限り）
func TestSQLiteStore_NonceStore_StoreAndConsume(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := s.Store(ctx, "u1", "nonce1", 1*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := s.Consume(ctx, "u1", "nonce1"); err != nil {
		t.Fatalf("Consume 1st: %v", err)
	}

	if err := s.Consume(ctx, "u1", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("Consume 2nd: expected ErrNonceAlreadyUsed, got %v", err)
	}
}

// T14: NonceStore で存在しない nonce は ErrNonceAlreadyUsed
func TestSQLiteStore_NonceStore_ConsumeNotExist(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := s.Consume(ctx, "u1", "ghost"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for nonexistent nonce, got %v", err)
	}
}

// T15: NonceStore のユーザー分離
func TestSQLiteStore_NonceStore_UserIsolation(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := s.Store(ctx, "u1", "nonce1", 1*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := s.Consume(ctx, "u2", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for different user, got %v", err)
	}
}

// T16: 自動マイグレーション（新規 DB でテーブルが自動作成される）
func TestSQLiteStore_AutoMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	// テーブルが作成されていれば操作できるはず
	reg := &SpaceRegistration{UserID: "u1", Alias: "test", Tenant: "t"}
	if err := s.Upsert(ctx, reg); err != nil {
		t.Fatalf("Upsert after auto migration: %v", err)
	}
	if err := s.Store(ctx, "u1", "nonce", 1*time.Minute); err != nil {
		t.Fatalf("NonceStore.Store after auto migration: %v", err)
	}
}

// T17: 永続化テスト（Close して再 Open してもデータが残る）
func TestSQLiteStore_Persistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")

	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore 1st: %v", err)
	}
	ctx := context.Background()
	if err := s1.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "example"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close 1st: %v", err)
	}

	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore 2nd: %v", err)
	}
	defer s2.Close()

	got, err := s2.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("expected data to persist after reopen, got nil")
	}
	if got.Tenant != "example" {
		t.Errorf("expected Tenant=example, got %s", got.Tenant)
	}
}

// T18: Nonce 期限切れは ErrNonceAlreadyUsed
func TestSQLiteStore_Nonce_Expiry(t *testing.T) {
	s := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := s.Store(ctx, "u1", "nonce1", 1*time.Millisecond); err != nil {
		t.Fatalf("Store: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := s.Consume(ctx, "u1", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for expired nonce, got %v", err)
	}
}

// T19: Close 後の操作はエラーを返す
func TestSQLiteStore_Close_RejectsOps(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "closed.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx := context.Background()
	if err := s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"}); err == nil {
		t.Error("expected error after Close, got nil")
	}
}
