package tokenstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/youyo/logvalet/internal/auth"
)

// mockDynamoDBClient は DynamoDBAPI を満たすテスト用モック。
// items は PK をキーとする属性マップを保持する。
type mockDynamoDBClient struct {
	items map[string]map[string]types.AttributeValue

	// エラー注入用
	getItemErr    error
	putItemErr    error
	deleteItemErr error
}

func newMockDynamoDBClient() *mockDynamoDBClient {
	return &mockDynamoDBClient{
		items: make(map[string]map[string]types.AttributeValue),
	}
}

func (m *mockDynamoDBClient) GetItem(_ context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	pk, ok := input.Key["pk"]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	pkStr := pk.(*types.AttributeValueMemberS).Value
	item, found := m.items[pkStr]
	if !found {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (m *mockDynamoDBClient) PutItem(_ context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if m.putItemErr != nil {
		return nil, m.putItemErr
	}
	pk, ok := input.Item["pk"]
	if !ok {
		return nil, errors.New("mock: pk not found in item")
	}
	pkStr := pk.(*types.AttributeValueMemberS).Value
	m.items[pkStr] = input.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDynamoDBClient) DeleteItem(_ context.Context, input *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemErr != nil {
		return nil, m.deleteItemErr
	}
	pk, ok := input.Key["pk"]
	if !ok {
		return &dynamodb.DeleteItemOutput{}, nil
	}
	pkStr := pk.(*types.AttributeValueMemberS).Value
	delete(m.items, pkStr)
	return &dynamodb.DeleteItemOutput{}, nil
}

// newTestDynamoDBStore はモック付きの DynamoDBStore を作成するヘルパー。
func newTestDynamoDBStore(t *testing.T) (*DynamoDBStore, *mockDynamoDBClient) {
	t.Helper()
	mock := newMockDynamoDBClient()
	store := NewDynamoDBStoreWithClient("test-table", mock)
	t.Cleanup(func() { store.Close() })
	return store, mock
}

func TestDynamoDBStore_PutGet(t *testing.T) {
	store, _ := newTestDynamoDBStore(t)
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

func TestDynamoDBStore_GetNotFound(t *testing.T) {
	store, _ := newTestDynamoDBStore(t)
	ctx := context.Background()

	got, err := store.Get(ctx, "nonexistent", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("Get: expected nil, got %v", got)
	}
}

func TestDynamoDBStore_PutUpsert(t *testing.T) {
	store, _ := newTestDynamoDBStore(t)
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
}

func TestDynamoDBStore_Delete(t *testing.T) {
	store, _ := newTestDynamoDBStore(t)
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

func TestDynamoDBStore_DeleteNotFound(t *testing.T) {
	store, _ := newTestDynamoDBStore(t)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("Delete (not found): unexpected error: %v", err)
	}
}

func TestDynamoDBStore_CloseIdempotent(t *testing.T) {
	mock := newMockDynamoDBClient()
	store := NewDynamoDBStoreWithClient("test-table", mock)

	if err := store.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close (second): %v", err)
	}
}

func TestDynamoDBStore_OperationsAfterClose(t *testing.T) {
	mock := newMockDynamoDBClient()
	store := NewDynamoDBStoreWithClient("test-table", mock)

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx := context.Background()

	// Close 後の Get はエラー
	_, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
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

func TestDynamoDBStore_UserIsolation(t *testing.T) {
	store, _ := newTestDynamoDBStore(t)
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
		t.Errorf("Get(userB): expected nil, got %v — user isolation violated", got)
	}

	// 同じユーザーでも別 tenant
	got2, err := store.Get(ctx, "userA", "backlog", "other.backlog.com")
	if err != nil {
		t.Fatalf("Get(userA, other tenant): %v", err)
	}
	if got2 != nil {
		t.Errorf("Get(userA, other tenant): expected nil, got %v", got2)
	}
}

func TestDynamoDBStore_GetItemError(t *testing.T) {
	store, mock := newTestDynamoDBStore(t)
	mock.getItemErr = errors.New("dynamo: simulated error")
	ctx := context.Background()

	_, err := store.Get(ctx, "user1", "backlog", "example.backlog.com")
	if err == nil {
		t.Fatal("Get: expected error, got nil")
	}
}

func TestDynamoDBStore_PutItemError(t *testing.T) {
	store, mock := newTestDynamoDBStore(t)
	mock.putItemErr = errors.New("dynamo: simulated error")
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

	err := store.Put(ctx, rec)
	if err == nil {
		t.Fatal("Put: expected error, got nil")
	}
}

func TestDynamoDBStore_DeleteItemError(t *testing.T) {
	store, mock := newTestDynamoDBStore(t)
	mock.deleteItemErr = errors.New("dynamo: simulated error")
	ctx := context.Background()

	err := store.Delete(ctx, "user1", "backlog", "example.backlog.com")
	if err == nil {
		t.Fatal("Delete: expected error, got nil")
	}
}
