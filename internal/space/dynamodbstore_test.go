package space

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// mockDynamoDBSpaceClient は DynamoDBSpaceAPI を満たすテスト用モック。
// 複合キー (pk, sk) でアイテムを管理する。
type mockDynamoDBSpaceClient struct {
	// items[pk][sk] = attribute map
	items map[string]map[string]map[string]types.AttributeValue

	// エラー注入用
	getItemErr    error
	putItemErr    error
	deleteItemErr error
	queryErr      error

	// ConditionalCheckFailed をシミュレートするフラグ
	condCheckFail bool
}

func newMockDynamoDBSpaceClient() *mockDynamoDBSpaceClient {
	return &mockDynamoDBSpaceClient{
		items: make(map[string]map[string]map[string]types.AttributeValue),
	}
}

func (m *mockDynamoDBSpaceClient) getItem(pk, sk string) (map[string]types.AttributeValue, bool) {
	sks, ok := m.items[pk]
	if !ok {
		return nil, false
	}
	item, ok := sks[sk]
	return item, ok
}

func (m *mockDynamoDBSpaceClient) setItem(pk, sk string, item map[string]types.AttributeValue) {
	if m.items[pk] == nil {
		m.items[pk] = make(map[string]map[string]types.AttributeValue)
	}
	m.items[pk][sk] = item
}

func (m *mockDynamoDBSpaceClient) GetItem(_ context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	pkAttr, ok := input.Key["pk"]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	skAttr, ok := input.Key["sk"]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	pk := pkAttr.(*types.AttributeValueMemberS).Value
	sk := skAttr.(*types.AttributeValueMemberS).Value
	item, found := m.getItem(pk, sk)
	if !found {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (m *mockDynamoDBSpaceClient) PutItem(_ context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if m.putItemErr != nil {
		return nil, m.putItemErr
	}
	pkAttr, ok := input.Item["pk"]
	if !ok {
		return nil, errors.New("mock: pk not found in item")
	}
	skAttr, ok := input.Item["sk"]
	if !ok {
		return nil, errors.New("mock: sk not found in item")
	}
	pk := pkAttr.(*types.AttributeValueMemberS).Value
	sk := skAttr.(*types.AttributeValueMemberS).Value
	m.setItem(pk, sk, input.Item)
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDynamoDBSpaceClient) DeleteItem(_ context.Context, input *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemErr != nil {
		return nil, m.deleteItemErr
	}
	pkAttr, ok := input.Key["pk"]
	if !ok {
		return &dynamodb.DeleteItemOutput{}, nil
	}
	skAttr, ok := input.Key["sk"]
	if !ok {
		return &dynamodb.DeleteItemOutput{}, nil
	}
	pk := pkAttr.(*types.AttributeValueMemberS).Value
	sk := skAttr.(*types.AttributeValueMemberS).Value

	// ConditionExpression がある場合は存在チェックを行う
	if input.ConditionExpression != nil {
		if m.condCheckFail {
			return nil, &types.ConditionalCheckFailedException{}
		}
		_, exists := m.getItem(pk, sk)
		if !exists {
			return nil, &types.ConditionalCheckFailedException{}
		}
	}

	if sks, ok := m.items[pk]; ok {
		delete(sks, sk)
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *mockDynamoDBSpaceClient) Query(_ context.Context, input *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	// pk 値を ExpressionAttributeValues から取得
	pkVal, ok := input.ExpressionAttributeValues[":pk"]
	if !ok {
		return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil
	}
	pk := pkVal.(*types.AttributeValueMemberS).Value

	sks, ok := m.items[pk]
	if !ok {
		return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil
	}

	// SK prefix フィルタ（:sk_prefix があれば適用）
	prefix := ""
	if skPrefix, ok := input.ExpressionAttributeValues[":sk_prefix"]; ok {
		prefix = skPrefix.(*types.AttributeValueMemberS).Value
	}

	var items []map[string]types.AttributeValue
	for sk, item := range sks {
		if prefix == "" || len(sk) >= len(prefix) && sk[:len(prefix)] == prefix {
			items = append(items, item)
		}
	}
	return &dynamodb.QueryOutput{Items: items}, nil
}

func (m *mockDynamoDBSpaceClient) TransactWriteItems(_ context.Context, _ *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	return &dynamodb.TransactWriteItemsOutput{}, nil
}

// newTestDynamoDBSpaceStore はモック付きの DynamoDBStore を作成するヘルパー。
func newTestDynamoDBSpaceStore(t *testing.T) (*DynamoDBStore, *mockDynamoDBSpaceClient) {
	t.Helper()
	mock := newMockDynamoDBSpaceClient()
	store := NewDynamoDBStoreWithClient("test-table", mock)
	t.Cleanup(func() { store.Close() })
	return store, mock
}

// --- Store interface テスト ---

func TestDynamoDBStore_UpsertAndGet(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	reg := &SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "example",
		BaseURL: "https://example.backlog.com",
	}
	if err := store.Upsert(ctx, reg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := store.Get(ctx, "u1", "foo")
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

func TestDynamoDBStore_UpsertOverwrite(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "old"})
	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "new"})

	got, err := store.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Tenant != "new" {
		t.Errorf("expected Tenant=new, got %s", got.Tenant)
	}
}

func TestDynamoDBStore_List_UserIsolation(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})
	store.Upsert(ctx, &SpaceRegistration{UserID: "u2", Alias: "foo"})

	u1List, err := store.List(ctx, "u1")
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

	u2List, err := store.List(ctx, "u2")
	if err != nil {
		t.Fatalf("List u2: %v", err)
	}
	if len(u2List) != 1 {
		t.Errorf("expected 1 for u2, got %d", len(u2List))
	}
}

func TestDynamoDBStore_List_Empty(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	result, err := store.List(ctx, "nobody")
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

func TestDynamoDBStore_Delete_TargetOnly(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})

	if err := store.Delete(ctx, "u1", "foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get foo: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete, got value")
	}

	got, err = store.Get(ctx, "u1", "bar")
	if err != nil {
		t.Fatalf("Get bar: %v", err)
	}
	if got == nil {
		t.Error("bar should still exist")
	}
}

func TestDynamoDBStore_Delete_DifferentUserSameAlias(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	store.Upsert(ctx, &SpaceRegistration{UserID: "u2", Alias: "foo"})

	if err := store.Delete(ctx, "u1", "foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get(ctx, "u2", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Error("u2/foo should still exist after deleting u1/foo")
	}
}

func TestDynamoDBStore_Delete_NotExist(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	if err := store.Delete(ctx, "u1", "nonexistent"); err != nil {
		t.Errorf("Delete of nonexistent should not error, got: %v", err)
	}
}

func TestDynamoDBStore_Preference_GetPut(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	pref := &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"}
	if err := store.PutPreference(ctx, pref); err != nil {
		t.Fatalf("PutPreference: %v", err)
	}

	got, err := store.GetPreference(ctx, "u1")
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

func TestDynamoDBStore_Preference_GetNotExist(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	got, err := store.GetPreference(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetPreference should not error for unset user, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unset user, got %+v", got)
	}
}

func TestDynamoDBStore_Preference_UserIsolation(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	store.PutPreference(ctx, &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"})
	store.PutPreference(ctx, &UserPreference{UserID: "u2", DefaultSpaceAlias: "bar"})

	p1, _ := store.GetPreference(ctx, "u1")
	p2, _ := store.GetPreference(ctx, "u2")

	if p1.DefaultSpaceAlias != "foo" {
		t.Errorf("u1: expected foo, got %s", p1.DefaultSpaceAlias)
	}
	if p2.DefaultSpaceAlias != "bar" {
		t.Errorf("u2: expected bar, got %s", p2.DefaultSpaceAlias)
	}
}

func TestDynamoDBStore_AllFields_RoundTrip(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	reg := &SpaceRegistration{
		UserID:         "u1",
		Alias:          "myspace",
		Tenant:         "myorg.backlog.com",
		BaseURL:        "https://myorg.backlog.com",
		AuthType:       AuthTypeOAuth,
		AuthProfile:    "default",
		Provider:       "backlog",
		Status:         SpaceStatusOK,
		LastVerifiedAt: now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Disabled:       false,
	}

	if err := store.Upsert(ctx, reg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := store.Get(ctx, "u1", "myspace")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Tenant != "myorg.backlog.com" {
		t.Errorf("Tenant: want %q got %q", "myorg.backlog.com", got.Tenant)
	}
	if got.BaseURL != "https://myorg.backlog.com" {
		t.Errorf("BaseURL: want %q got %q", "https://myorg.backlog.com", got.BaseURL)
	}
	if got.AuthType != AuthTypeOAuth {
		t.Errorf("AuthType: want %q got %q", AuthTypeOAuth, got.AuthType)
	}
	if got.Status != SpaceStatusOK {
		t.Errorf("Status: want %q got %q", SpaceStatusOK, got.Status)
	}
	if !got.LastVerifiedAt.Equal(now) {
		t.Errorf("LastVerifiedAt: want %v got %v", now, got.LastVerifiedAt)
	}
	if !got.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: want %v got %v", now, got.CreatedAt)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt: want %v got %v", now, got.UpdatedAt)
	}
}

func TestDynamoDBStore_GetItem_Error(t *testing.T) {
	store, mock := newTestDynamoDBSpaceStore(t)
	mock.getItemErr = errors.New("dynamo: simulated error")
	ctx := context.Background()

	_, err := store.Get(ctx, "u1", "foo")
	if err == nil {
		t.Fatal("Get: expected error, got nil")
	}
}

func TestDynamoDBStore_PutItem_Error(t *testing.T) {
	store, mock := newTestDynamoDBSpaceStore(t)
	mock.putItemErr = errors.New("dynamo: simulated error")
	ctx := context.Background()

	err := store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	if err == nil {
		t.Fatal("Upsert: expected error, got nil")
	}
}

func TestDynamoDBStore_DeleteItem_Error(t *testing.T) {
	store, mock := newTestDynamoDBSpaceStore(t)
	mock.deleteItemErr = errors.New("dynamo: simulated error")
	ctx := context.Background()

	err := store.Delete(ctx, "u1", "foo")
	if err == nil {
		t.Fatal("Delete: expected error, got nil")
	}
}

func TestDynamoDBStore_OperationsAfterClose(t *testing.T) {
	mock := newMockDynamoDBSpaceClient()
	store := NewDynamoDBStoreWithClient("test-table", mock)

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx := context.Background()

	if _, err := store.Get(ctx, "u1", "foo"); err == nil {
		t.Error("Get after Close: expected error, got nil")
	}
	if err := store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"}); err == nil {
		t.Error("Upsert after Close: expected error, got nil")
	}
	if err := store.Delete(ctx, "u1", "foo"); err == nil {
		t.Error("Delete after Close: expected error, got nil")
	}
	if _, err := store.List(ctx, "u1"); err == nil {
		t.Error("List after Close: expected error, got nil")
	}
	if _, err := store.GetPreference(ctx, "u1"); err == nil {
		t.Error("GetPreference after Close: expected error, got nil")
	}
	if err := store.PutPreference(ctx, &UserPreference{UserID: "u1"}); err == nil {
		t.Error("PutPreference after Close: expected error, got nil")
	}
}

// --- NonceStore テスト ---

func TestDynamoDBStore_NonceStore_StoreAndConsume(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	if err := store.Store(ctx, "u1", "nonce1", 1*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := store.Consume(ctx, "u1", "nonce1"); err != nil {
		t.Fatalf("Consume 1st: %v", err)
	}

	if err := store.Consume(ctx, "u1", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("Consume 2nd: expected ErrNonceAlreadyUsed, got %v", err)
	}
}

func TestDynamoDBStore_NonceStore_ConsumeNotExist(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	if err := store.Consume(ctx, "u1", "ghost"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for nonexistent nonce, got %v", err)
	}
}

func TestDynamoDBStore_NonceStore_UserIsolation(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	if err := store.Store(ctx, "u1", "nonce1", 1*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// u2 で消費しようとしてもエラー（u1 の nonce は u2 からは見えない）
	if err := store.Consume(ctx, "u2", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for different user, got %v", err)
	}

	// u1 の nonce はまだ消費可能
	if err := store.Consume(ctx, "u1", "nonce1"); err != nil {
		t.Errorf("u1 nonce should still be consumable: %v", err)
	}
}

// T16: ConditionExpression による consume-once の検証
func TestDynamoDBStore_Nonce_ConsumeWithConditionExpression(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	// Store nonce
	if err := store.Store(ctx, "u1", "nonce1", 5*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// 1回目 Consume → 成功
	if err := store.Consume(ctx, "u1", "nonce1"); err != nil {
		t.Fatalf("Consume 1st: %v", err)
	}

	// 2回目 Consume → ErrNonceAlreadyUsed（既に削除済み = ConditionalCheck 失敗）
	if err := store.Consume(ctx, "u1", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("Consume 2nd (replay): expected ErrNonceAlreadyUsed, got %v", err)
	}
}

// T17: TTL 属性が設定されることを確認
func TestDynamoDBStore_Nonce_TTL(t *testing.T) {
	store, mock := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	if err := store.Store(ctx, "u1", "nonce1", 10*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// DynamoDB アイテムに expires_at 属性が存在することを確認
	pk := "USER#u1"
	sk := "NONCE#nonce1"
	item, exists := mock.getItem(pk, sk)
	if !exists {
		t.Fatal("nonce item not found in mock store")
	}
	expAttr, ok := item["expires_at"]
	if !ok {
		t.Fatal("expires_at attribute not set")
	}
	_, ok = expAttr.(*types.AttributeValueMemberN)
	if !ok {
		t.Errorf("expires_at should be a Number type, got %T", expAttr)
	}
}

// T18: 同一 userID + tenant が複数件存在することを List で確認できる
// （重複検出は application layer の責務: Upsert 前に caller が確認する）
func TestDynamoDBStore_TenantDuplicateCheck(t *testing.T) {
	store, _ := newTestDynamoDBSpaceStore(t)
	ctx := context.Background()

	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "myorg"})
	store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar", Tenant: "myorg"})

	list, err := store.List(ctx, "u1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var count int
	for _, r := range list {
		if r.Tenant == "myorg" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 items with tenant=myorg, got %d", count)
	}
}

// T19: Close 冪等性
func TestDynamoDBStore_Close_Idempotent(t *testing.T) {
	mock := newMockDynamoDBSpaceClient()
	store := NewDynamoDBStoreWithClient("test-table", mock)

	if err := store.Close(); err != nil {
		t.Fatalf("Close 1st: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close 2nd: %v", err)
	}
}
