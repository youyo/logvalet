package tokenstore_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/auth/tokenstore"
)

// newTestRecord はテスト用の TokenRecord を生成するヘルパー。
func newTestRecord(userID, provider, tenant, accessToken string) *auth.TokenRecord {
	now := time.Now().Truncate(time.Second)
	return &auth.TokenRecord{
		UserID:         userID,
		Provider:       provider,
		Tenant:         tenant,
		AccessToken:    accessToken,
		RefreshToken:   "refresh-" + accessToken,
		TokenType:      "Bearer",
		Scope:          "read write",
		Expiry:         now.Add(1 * time.Hour),
		ProviderUserID: "provider-user-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestMemoryStore_PutAndGet(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	rec := newTestRecord("userA", "backlog", "example.backlog.com", "access-token-123")

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, err := store.Get(ctx, "userA", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil, want non-nil")
	}
	if got.UserID != rec.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, rec.UserID)
	}
	if got.Provider != rec.Provider {
		t.Errorf("Provider = %q, want %q", got.Provider, rec.Provider)
	}
	if got.Tenant != rec.Tenant {
		t.Errorf("Tenant = %q, want %q", got.Tenant, rec.Tenant)
	}
	if got.AccessToken != rec.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, rec.AccessToken)
	}
	if got.RefreshToken != rec.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, rec.RefreshToken)
	}
	if got.TokenType != rec.TokenType {
		t.Errorf("TokenType = %q, want %q", got.TokenType, rec.TokenType)
	}
	if got.Scope != rec.Scope {
		t.Errorf("Scope = %q, want %q", got.Scope, rec.Scope)
	}
	if !got.Expiry.Equal(rec.Expiry) {
		t.Errorf("Expiry = %v, want %v", got.Expiry, rec.Expiry)
	}
	if got.ProviderUserID != rec.ProviderUserID {
		t.Errorf("ProviderUserID = %q, want %q", got.ProviderUserID, rec.ProviderUserID)
	}
}

func TestMemoryStore_GetNotFound(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	got, err := store.Get(ctx, "nonexistent", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get returned error for missing key: %v", err)
	}
	if got != nil {
		t.Fatalf("Get returned %v, want nil", got)
	}
}

func TestMemoryStore_PutOverwrite(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	rec1 := newTestRecord("userA", "backlog", "example.backlog.com", "old-token")
	rec2 := newTestRecord("userA", "backlog", "example.backlog.com", "new-token")

	if err := store.Put(ctx, rec1); err != nil {
		t.Fatalf("Put(rec1) failed: %v", err)
	}
	if err := store.Put(ctx, rec2); err != nil {
		t.Fatalf("Put(rec2) failed: %v", err)
	}

	got, err := store.Get(ctx, "userA", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil, want non-nil")
	}
	if got.AccessToken != "new-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "new-token")
	}
}

func TestMemoryStore_DeleteAndGet(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	rec := newTestRecord("userA", "backlog", "example.backlog.com", "access-token-123")

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if err := store.Delete(ctx, "userA", "backlog", "example.backlog.com"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	got, err := store.Get(ctx, "userA", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get after Delete returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get after Delete returned %v, want nil", got)
	}
}

func TestMemoryStore_DeleteNotFound(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	err := store.Delete(ctx, "nonexistent", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Delete on missing key returned error: %v", err)
	}
}

func TestMemoryStore_CloseIdempotent(t *testing.T) {
	store := tokenstore.NewMemoryStore()

	if err := store.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

func TestMemoryStore_UserIsolation(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	recA := newTestRecord("userA", "backlog", "example.backlog.com", "token-A")

	if err := store.Put(ctx, recA); err != nil {
		t.Fatalf("Put(userA) failed: %v", err)
	}

	// userB で同じ provider/tenant を指定しても userA のトークンは取得できない
	got, err := store.Get(ctx, "userB", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get(userB) returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get(userB) returned %v, want nil — user isolation violated", got)
	}
}

func TestMemoryStore_GetReturnsCopy(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	rec := newTestRecord("userA", "backlog", "example.backlog.com", "original-token")

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get で取得したレコードを変更
	got, _ := store.Get(ctx, "userA", "backlog", "example.backlog.com")
	got.AccessToken = "mutated-token"

	// 再度 Get してストア内のデータが変更されていないことを確認
	got2, _ := store.Get(ctx, "userA", "backlog", "example.backlog.com")
	if got2.AccessToken != "original-token" {
		t.Errorf("store data was mutated via returned pointer: AccessToken = %q, want %q",
			got2.AccessToken, "original-token")
	}
}

func TestMemoryStore_PutStoresCopy(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	rec := newTestRecord("userA", "backlog", "example.backlog.com", "original-token")

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Put に渡したオリジナルを変更
	rec.AccessToken = "mutated-after-put"

	// ストア内のデータが変更されていないことを確認
	got, _ := store.Get(ctx, "userA", "backlog", "example.backlog.com")
	if got.AccessToken != "original-token" {
		t.Errorf("store data was mutated via original pointer: AccessToken = %q, want %q",
			got.AccessToken, "original-token")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := tokenstore.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			userID := "user"
			provider := "backlog"
			tenant := "example.backlog.com"

			for j := range iterations {
				rec := newTestRecord(userID, provider, tenant, "token")
				_ = store.Put(ctx, rec)
				_, _ = store.Get(ctx, userID, provider, tenant)
				if j%3 == 0 {
					_ = store.Delete(ctx, userID, provider, tenant)
				}
			}
		}(i)
	}

	wg.Wait()
}
