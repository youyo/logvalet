package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

// mockTokenManager は TokenManager のテスト用モック。
type mockTokenManager struct {
	getValidTokenFunc func(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error)
}

func (m *mockTokenManager) GetValidToken(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
	return m.getValidTokenFunc(ctx, userID, provider, tenant)
}

func (m *mockTokenManager) SaveToken(_ context.Context, _ *auth.TokenRecord) error {
	return nil
}

func (m *mockTokenManager) RevokeToken(_ context.Context, _, _, _ string) error {
	return nil
}

func TestClientFactory_NoUserID(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			t.Fatal("GetValidToken should not be called when userID is missing")
			return nil, nil
		},
	}

	factory := auth.NewClientFactory(tm, "backlog", "example.backlog.com", "https://example.backlog.com")

	// userID が context にない場合
	_, err := factory(context.Background())
	if err == nil {
		t.Fatal("factory should return error when userID is missing from context")
	}
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("error = %v, want ErrUnauthenticated", err)
	}
}

func TestClientFactory_ProviderNotConnected(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			return nil, auth.ErrProviderNotConnected
		},
	}

	factory := auth.NewClientFactory(tm, "backlog", "example.backlog.com", "https://example.backlog.com")

	ctx := auth.ContextWithUserID(context.Background(), "user-1")
	_, err := factory(ctx)
	if err == nil {
		t.Fatal("factory should return error when provider is not connected")
	}
	if !errors.Is(err, auth.ErrProviderNotConnected) {
		t.Errorf("error = %v, want ErrProviderNotConnected", err)
	}
}

func TestClientFactory_BearerHeader(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"admin","name":"Admin User"}`))
	}))
	defer ts.Close()

	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, userID, _, _ string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				UserID:      userID,
				Provider:    "backlog",
				Tenant:      "example.backlog.com",
				AccessToken: "test-access-token-12345678",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	factory := auth.NewClientFactory(tm, "backlog", "example.backlog.com", ts.URL)

	ctx := auth.ContextWithUserID(context.Background(), "user-1")
	client, err := factory(ctx)
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if client == nil {
		t.Fatal("factory returned nil client")
	}

	// クライアントを使って API コールし、Bearer ヘッダが正しく設定されることを検証
	_, err = client.GetMyself(ctx)
	if err != nil {
		t.Fatalf("GetMyself returned error: %v", err)
	}
	if gotAuth != "Bearer test-access-token-12345678" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-access-token-12345678")
	}
}

func TestClientFactory_UserIsolation(t *testing.T) {
	var gotAuths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuths = append(gotAuths, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"test","name":"Test"}`))
	}))
	defer ts.Close()

	tokens := map[string]string{
		"alice": "alice-token-abcdefgh",
		"bob":   "bob-token-12345678",
	}

	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, userID, _, _ string) (*auth.TokenRecord, error) {
			tok, ok := tokens[userID]
			if !ok {
				return nil, auth.ErrProviderNotConnected
			}
			return &auth.TokenRecord{
				UserID:      userID,
				Provider:    "backlog",
				Tenant:      "example.backlog.com",
				AccessToken: tok,
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	factory := auth.NewClientFactory(tm, "backlog", "example.backlog.com", ts.URL)

	// Alice のリクエスト
	ctxAlice := auth.ContextWithUserID(context.Background(), "alice")
	clientAlice, err := factory(ctxAlice)
	if err != nil {
		t.Fatalf("factory for alice returned error: %v", err)
	}
	_, _ = clientAlice.GetMyself(ctxAlice)

	// Bob のリクエスト
	ctxBob := auth.ContextWithUserID(context.Background(), "bob")
	clientBob, err := factory(ctxBob)
	if err != nil {
		t.Fatalf("factory for bob returned error: %v", err)
	}
	_, _ = clientBob.GetMyself(ctxBob)

	if len(gotAuths) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(gotAuths))
	}
	if gotAuths[0] != "Bearer alice-token-abcdefgh" {
		t.Errorf("alice auth = %q, want %q", gotAuths[0], "Bearer alice-token-abcdefgh")
	}
	if gotAuths[1] != "Bearer bob-token-12345678" {
		t.Errorf("bob auth = %q, want %q", gotAuths[1], "Bearer bob-token-12345678")
	}
}
