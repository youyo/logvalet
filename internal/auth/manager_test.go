package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

// --- Mock implementations ---

// mockStore は TokenStore のテスト用モック。
type mockStore struct {
	records map[string]*auth.TokenRecord
	putErr  error
	getErr  error
	delErr  error
}

func newMockStore() *mockStore {
	return &mockStore{records: make(map[string]*auth.TokenRecord)}
}

func (m *mockStore) Get(_ context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	key := auth.StoreKey(userID, provider, tenant)
	rec, ok := m.records[key]
	if !ok {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}

func (m *mockStore) Put(_ context.Context, record *auth.TokenRecord) error {
	if m.putErr != nil {
		return m.putErr
	}
	key := auth.StoreKey(record.UserID, record.Provider, record.Tenant)
	cp := *record
	m.records[key] = &cp
	return nil
}

func (m *mockStore) Delete(_ context.Context, userID, provider, tenant string) error {
	if m.delErr != nil {
		return m.delErr
	}
	key := auth.StoreKey(userID, provider, tenant)
	delete(m.records, key)
	return nil
}

func (m *mockStore) Close() error { return nil }

// mockRefresher は TokenRefresher のテスト用モック。
type mockRefresher struct {
	name       string
	record     *auth.TokenRecord
	err        error
	calledWith string
}

func (m *mockRefresher) Name() string { return m.name }

func (m *mockRefresher) RefreshToken(_ context.Context, refreshToken string) (*auth.TokenRecord, error) {
	m.calledWith = refreshToken
	if m.err != nil {
		return nil, m.err
	}
	cp := *m.record
	return &cp, nil
}

// --- Tests ---

func TestGetValidToken_ValidToken(t *testing.T) {
	store := newMockStore()
	rec := &auth.TokenRecord{
		UserID:         "user1",
		Provider:       "backlog",
		Tenant:         "example.backlog.com",
		AccessToken:    "valid-access-token",
		RefreshToken:   "valid-refresh-token",
		TokenType:      "Bearer",
		Expiry:         time.Now().Add(1 * time.Hour),
		ProviderUserID: "provider-user-1",
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		UpdatedAt:      time.Now().Add(-1 * time.Hour),
	}
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = rec

	refresher := &mockRefresher{name: "backlog"}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers)

	got, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("GetValidToken returned error: %v", err)
	}
	if got.AccessToken != "valid-access-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "valid-access-token")
	}
	if refresher.calledWith != "" {
		t.Error("RefreshToken should not have been called for a valid token")
	}
}

func TestGetValidToken_AutoRefresh(t *testing.T) {
	store := newMockStore()
	rec := &auth.TokenRecord{
		UserID:         "user1",
		Provider:       "backlog",
		Tenant:         "example.backlog.com",
		AccessToken:    "old-access-token",
		RefreshToken:   "old-refresh-token",
		TokenType:      "Bearer",
		Expiry:         time.Now().Add(2 * time.Minute), // 5min マージン以内 → リフレッシュ対象
		ProviderUserID: "provider-user-1",
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		UpdatedAt:      time.Now().Add(-1 * time.Hour),
	}
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = rec

	refreshedRec := &auth.TokenRecord{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
		// UserID, ProviderUserID は設定されない（M05 仕様）
	}
	refresher := &mockRefresher{name: "backlog", record: refreshedRec}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers)

	got, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("GetValidToken returned error: %v", err)
	}
	if got.AccessToken != "new-access-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "new-access-token")
	}
	if refresher.calledWith != "old-refresh-token" {
		t.Errorf("RefreshToken called with %q, want %q", refresher.calledWith, "old-refresh-token")
	}

	// store が更新されていることを確認
	stored, _ := store.Get(context.Background(), "user1", "backlog", "example.backlog.com")
	if stored == nil {
		t.Fatal("store should contain the refreshed record")
	}
	if stored.AccessToken != "new-access-token" {
		t.Errorf("stored AccessToken = %q, want %q", stored.AccessToken, "new-access-token")
	}
}

func TestGetValidToken_IdentityFieldsPreserved(t *testing.T) {
	store := newMockStore()
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rec := &auth.TokenRecord{
		UserID:         "user1",
		Provider:       "backlog",
		Tenant:         "example.backlog.com",
		AccessToken:    "old-token",
		RefreshToken:   "old-refresh",
		TokenType:      "Bearer",
		Scope:          "read write",
		Expiry:         time.Now().Add(1 * time.Minute), // リフレッシュ対象
		ProviderUserID: "backlog-uid-42",
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = rec

	refreshedRec := &auth.TokenRecord{
		AccessToken:  "new-token",
		RefreshToken: "new-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
		// UserID, ProviderUserID, Provider, Tenant, CreatedAt は未設定
	}
	refresher := &mockRefresher{name: "backlog", record: refreshedRec}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	beforeRefresh := time.Now()
	mgr := auth.NewTokenManager(store, providers)
	got, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("GetValidToken returned error: %v", err)
	}

	// identity fields がコピーされていること
	if got.UserID != "user1" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user1")
	}
	if got.Provider != "backlog" {
		t.Errorf("Provider = %q, want %q", got.Provider, "backlog")
	}
	if got.Tenant != "example.backlog.com" {
		t.Errorf("Tenant = %q, want %q", got.Tenant, "example.backlog.com")
	}
	if got.ProviderUserID != "backlog-uid-42" {
		t.Errorf("ProviderUserID = %q, want %q", got.ProviderUserID, "backlog-uid-42")
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, createdAt)
	}
	// UpdatedAt はリフレッシュ後に更新されていること
	if got.UpdatedAt.Before(beforeRefresh) {
		t.Error("UpdatedAt should be updated after refresh")
	}
}

func TestGetValidToken_RecordNotFound(t *testing.T) {
	store := newMockStore()
	refresher := &mockRefresher{name: "backlog"}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers)

	_, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err == nil {
		t.Fatal("GetValidToken should return error for missing record")
	}
	if !errors.Is(err, auth.ErrProviderNotConnected) {
		t.Errorf("error = %v, want ErrProviderNotConnected", err)
	}
}

func TestGetValidToken_RefreshFailed(t *testing.T) {
	store := newMockStore()
	rec := &auth.TokenRecord{
		UserID:       "user1",
		Provider:     "backlog",
		Tenant:       "example.backlog.com",
		AccessToken:  "old-token",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(1 * time.Minute), // リフレッシュ対象
	}
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = rec

	refresher := &mockRefresher{
		name: "backlog",
		err:  errors.New("network error"),
	}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers)

	_, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err == nil {
		t.Fatal("GetValidToken should return error when refresh fails")
	}
	if !errors.Is(err, auth.ErrTokenRefreshFailed) {
		t.Errorf("error = %v, want ErrTokenRefreshFailed", err)
	}
}

func TestGetValidToken_ProviderNotRegistered(t *testing.T) {
	store := newMockStore()
	rec := &auth.TokenRecord{
		UserID:       "user1",
		Provider:     "github", // providers map に "github" はない
		Tenant:       "example.com",
		AccessToken:  "token",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(1 * time.Minute), // リフレッシュ対象
	}
	key := auth.StoreKey("user1", "github", "example.com")
	store.records[key] = rec

	// providers には "backlog" だけ登録
	refresher := &mockRefresher{name: "backlog"}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers)

	_, err := mgr.GetValidToken(context.Background(), "user1", "github", "example.com")
	if err == nil {
		t.Fatal("GetValidToken should return error for unregistered provider")
	}
	if !errors.Is(err, auth.ErrProviderNotConnected) {
		t.Errorf("error = %v, want ErrProviderNotConnected", err)
	}
}

func TestGetValidToken_DefaultMargin(t *testing.T) {
	store := newMockStore()
	// 6分後に期限切れ → デフォルトマージン5分 → リフレッシュしない
	rec := &auth.TokenRecord{
		UserID:       "user1",
		Provider:     "backlog",
		Tenant:       "example.backlog.com",
		AccessToken:  "valid-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(6 * time.Minute),
	}
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = rec

	refresher := &mockRefresher{name: "backlog"}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers)

	got, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("GetValidToken returned error: %v", err)
	}
	if got.AccessToken != "valid-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "valid-token")
	}
	if refresher.calledWith != "" {
		t.Error("RefreshToken should not have been called — token is outside 5min margin")
	}
}

func TestGetValidToken_CustomMargin(t *testing.T) {
	store := newMockStore()
	// 6分後に期限切れ → カスタムマージン10分 → リフレッシュ対象
	rec := &auth.TokenRecord{
		UserID:       "user1",
		Provider:     "backlog",
		Tenant:       "example.backlog.com",
		AccessToken:  "old-token",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(6 * time.Minute),
	}
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = rec

	refreshedRec := &auth.TokenRecord{
		AccessToken:  "new-token",
		RefreshToken: "new-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	refresher := &mockRefresher{name: "backlog", record: refreshedRec}
	providers := map[string]auth.TokenRefresher{"backlog": refresher}

	mgr := auth.NewTokenManager(store, providers, auth.WithRefreshMargin(10*time.Minute))

	got, err := mgr.GetValidToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("GetValidToken returned error: %v", err)
	}
	if got.AccessToken != "new-token" {
		t.Errorf("AccessToken = %q, want %q — custom margin 10min should trigger refresh", got.AccessToken, "new-token")
	}
}

func TestSaveToken(t *testing.T) {
	store := newMockStore()
	providers := map[string]auth.TokenRefresher{}

	mgr := auth.NewTokenManager(store, providers)

	rec := &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "token-123",
	}
	err := mgr.SaveToken(context.Background(), rec)
	if err != nil {
		t.Fatalf("SaveToken returned error: %v", err)
	}

	// store に保存されていることを確認
	stored, _ := store.Get(context.Background(), "user1", "backlog", "example.backlog.com")
	if stored == nil {
		t.Fatal("store should contain the saved record")
	}
	if stored.AccessToken != "token-123" {
		t.Errorf("stored AccessToken = %q, want %q", stored.AccessToken, "token-123")
	}
}

func TestRevokeToken(t *testing.T) {
	store := newMockStore()
	key := auth.StoreKey("user1", "backlog", "example.backlog.com")
	store.records[key] = &auth.TokenRecord{
		UserID:      "user1",
		Provider:    "backlog",
		Tenant:      "example.backlog.com",
		AccessToken: "token-to-delete",
	}
	providers := map[string]auth.TokenRefresher{}

	mgr := auth.NewTokenManager(store, providers)

	err := mgr.RevokeToken(context.Background(), "user1", "backlog", "example.backlog.com")
	if err != nil {
		t.Fatalf("RevokeToken returned error: %v", err)
	}

	// store から削除されていることを確認
	stored, _ := store.Get(context.Background(), "user1", "backlog", "example.backlog.com")
	if stored != nil {
		t.Error("store should not contain the record after revoke")
	}
}
