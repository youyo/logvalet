package tokenstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

func TestNewTokenStore_Memory(t *testing.T) {
	cfg := &auth.OAuthEnvConfig{
		TokenStoreType: auth.StoreTypeMemory,
	}

	store, err := NewTokenStore(cfg)
	if err != nil {
		t.Fatalf("NewTokenStore(memory): unexpected error: %v", err)
	}
	defer store.Close()

	// MemoryStore が返されたことを Put/Get ラウンドトリップで検証
	ctx := context.Background()
	rec := &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example",
		AccessToken: "access-token-123",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	if err := store.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get(ctx, "user1", "backlog", "example")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected record, got nil")
	}
	if got.AccessToken != "access-token-123" {
		t.Errorf("AccessToken: want %q, got %q", "access-token-123", got.AccessToken)
	}
}

func TestNewTokenStore_MemoryDefault(t *testing.T) {
	// 空文字列（デフォルト）でも MemoryStore が返される
	cfg := &auth.OAuthEnvConfig{
		TokenStoreType: "", // LoadOAuthEnvConfig はデフォルトで StoreTypeMemory を設定するが、空文字列のケースも対応
	}

	store, err := NewTokenStore(cfg)
	if err != nil {
		t.Fatalf("NewTokenStore(empty): unexpected error: %v", err)
	}
	defer store.Close()

	// 型アサーションで MemoryStore であることを確認（同一パッケージ内）
	if _, ok := store.(*MemoryStore); !ok {
		t.Errorf("expected *MemoryStore, got %T", store)
	}
}

func TestNewTokenStore_SQLite_NotImplemented(t *testing.T) {
	cfg := &auth.OAuthEnvConfig{
		TokenStoreType: auth.StoreTypeSQLite,
		SQLitePath:     "/tmp/test.db",
	}

	store, err := NewTokenStore(cfg)
	if store != nil {
		t.Errorf("expected nil store, got %v", store)
	}
	if !errors.Is(err, auth.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

func TestNewTokenStore_DynamoDB_NotImplemented(t *testing.T) {
	cfg := &auth.OAuthEnvConfig{
		TokenStoreType: auth.StoreTypeDynamoDB,
		DynamoDBTable:  "test-table",
		DynamoDBRegion: "ap-northeast-1",
	}

	store, err := NewTokenStore(cfg)
	if store != nil {
		t.Errorf("expected nil store, got %v", store)
	}
	if !errors.Is(err, auth.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

func TestNewTokenStore_Unknown(t *testing.T) {
	cfg := &auth.OAuthEnvConfig{
		TokenStoreType: auth.StoreType("redis"),
	}

	store, err := NewTokenStore(cfg)
	if store != nil {
		t.Errorf("expected nil store, got %v", store)
	}
	if !errors.Is(err, auth.ErrInvalidStoreType) {
		t.Errorf("expected ErrInvalidStoreType, got: %v", err)
	}
}

func TestNewTokenStore_NilConfig(t *testing.T) {
	store, err := NewTokenStore(nil)
	if err != nil {
		t.Fatalf("NewTokenStore(nil): unexpected error: %v", err)
	}
	defer store.Close()

	// nil config でもデフォルト（memory）が返される
	if _, ok := store.(*MemoryStore); !ok {
		t.Errorf("expected *MemoryStore, got %T", store)
	}
}
