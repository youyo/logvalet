package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/space"
)

// mockCredResolver は credentials.Resolver のテスト用モック。
type mockCredResolver struct {
	resolveFunc func(authRef string, flags credentials.CredentialFlags, getenv func(string) string) (*credentials.ResolvedCredential, error)
}

func (m *mockCredResolver) Resolve(authRef string, flags credentials.CredentialFlags, getenv func(string) string) (*credentials.ResolvedCredential, error) {
	return m.resolveFunc(authRef, flags, getenv)
}

// T1: OAuth 認証成功 — Bearer トークンが正しく送られる
func TestSpaceAwareClientFactory_OAuth_Success(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"admin","name":"Admin User"}`))
	}))
	defer ts.Close()

	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				UserID:      userID,
				Provider:    provider,
				Tenant:      tenant,
				AccessToken: "oauth-token-success",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			t.Fatal("Resolve should not be called for OAuth auth type")
			return nil, nil
		},
	}

	reg := space.SpaceRegistration{
		Alias:    "my-space",
		Tenant:   "foo",
		BaseURL:  ts.URL,
		AuthType: space.AuthTypeOAuth,
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)
	ctx := auth.ContextWithUserID(context.Background(), "u1")

	client, err := factory(ctx, reg)
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if client == nil {
		t.Fatal("factory returned nil client")
	}

	_, err = client.GetMyself(ctx)
	if err != nil {
		t.Fatalf("GetMyself returned error: %v", err)
	}
	if gotAuth != "Bearer oauth-token-success" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer oauth-token-success")
	}
}

// T2: OAuth 認証 — context に userID がない場合 ErrUnauthenticated
func TestSpaceAwareClientFactory_OAuth_NoUserID(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			t.Fatal("GetValidToken should not be called when userID is missing")
			return nil, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			return nil, nil
		},
	}

	reg := space.SpaceRegistration{
		Alias:    "my-space",
		Tenant:   "foo",
		BaseURL:  "https://foo.backlog.com",
		AuthType: space.AuthTypeOAuth,
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)

	_, err := factory(context.Background(), reg)
	if err == nil {
		t.Fatal("factory should return error when userID is missing")
	}
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("error = %v, want ErrUnauthenticated", err)
	}
}

// T3: OAuth 認証 — トークンが存在しない場合 ErrProviderNotConnected
func TestSpaceAwareClientFactory_OAuth_NoToken(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			return nil, auth.ErrProviderNotConnected
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			return nil, nil
		},
	}

	reg := space.SpaceRegistration{
		Alias:    "my-space",
		Tenant:   "foo",
		BaseURL:  "https://foo.backlog.com",
		AuthType: space.AuthTypeOAuth,
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)
	ctx := auth.ContextWithUserID(context.Background(), "u1")

	_, err := factory(ctx, reg)
	if err == nil {
		t.Fatal("factory should return error when token is not found")
	}
	if !errors.Is(err, auth.ErrProviderNotConnected) {
		t.Errorf("error = %v, want ErrProviderNotConnected", err)
	}
}

// T4: OAuth 認証 — ユーザー分離（userA/userB が別の Bearer トークンを使う）
func TestSpaceAwareClientFactory_OAuth_UserIsolation(t *testing.T) {
	var gotAuths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuths = append(gotAuths, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"test","name":"Test"}`))
	}))
	defer ts.Close()

	tokens := map[string]string{
		"userA": "token-A-abcdefgh",
		"userB": "token-B-12345678",
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
				Tenant:      "foo",
				AccessToken: tok,
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			return nil, nil
		},
	}

	reg := space.SpaceRegistration{
		Alias:    "shared-space",
		Tenant:   "foo",
		BaseURL:  ts.URL,
		AuthType: space.AuthTypeOAuth,
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)

	ctxA := auth.ContextWithUserID(context.Background(), "userA")
	clientA, err := factory(ctxA, reg)
	if err != nil {
		t.Fatalf("factory for userA returned error: %v", err)
	}
	_, _ = clientA.GetMyself(ctxA)

	ctxB := auth.ContextWithUserID(context.Background(), "userB")
	clientB, err := factory(ctxB, reg)
	if err != nil {
		t.Fatalf("factory for userB returned error: %v", err)
	}
	_, _ = clientB.GetMyself(ctxB)

	if len(gotAuths) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(gotAuths))
	}
	if gotAuths[0] != "Bearer token-A-abcdefgh" {
		t.Errorf("userA auth = %q, want %q", gotAuths[0], "Bearer token-A-abcdefgh")
	}
	if gotAuths[1] != "Bearer token-B-12345678" {
		t.Errorf("userB auth = %q, want %q", gotAuths[1], "Bearer token-B-12345678")
	}
}

// T5: APIKey 認証成功 — apiKey クエリパラメータが送られる
func TestSpaceAwareClientFactory_APIKey_Success(t *testing.T) {
	var gotAPIKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.URL.Query().Get("apiKey")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"admin","name":"Admin User"}`))
	}))
	defer ts.Close()

	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			t.Fatal("GetValidToken should not be called for APIKey auth type")
			return nil, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(authRef string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			if authRef == "foo-profile" {
				return &credentials.ResolvedCredential{
					AuthType: credentials.AuthTypeAPIKey,
					APIKey:   "my-api-key-12345",
					Source:   "tokens_json",
				}, nil
			}
			return nil, errors.New("credentials: profile not found")
		},
	}

	reg := space.SpaceRegistration{
		Alias:       "api-space",
		Tenant:      "foo",
		BaseURL:     ts.URL,
		AuthType:    space.AuthTypeAPIKey,
		AuthProfile: "foo-profile",
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)
	ctx := context.Background()

	client, err := factory(ctx, reg)
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if client == nil {
		t.Fatal("factory returned nil client")
	}

	_, err = client.GetMyself(ctx)
	if err != nil {
		t.Fatalf("GetMyself returned error: %v", err)
	}
	if gotAPIKey != "my-api-key-12345" {
		t.Errorf("apiKey = %q, want %q", gotAPIKey, "my-api-key-12345")
	}
}

// T6: APIKey 認証 — プロファイルが存在しない場合エラー
func TestSpaceAwareClientFactory_APIKey_NoCredential(t *testing.T) {
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, _ string) (*auth.TokenRecord, error) {
			return nil, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			return nil, errors.New("credentials: no credentials found for auth_ref \"missing-profile\" in tokens.json")
		},
	}

	reg := space.SpaceRegistration{
		Alias:       "api-space",
		Tenant:      "foo",
		BaseURL:     "https://foo.backlog.com",
		AuthType:    space.AuthTypeAPIKey,
		AuthProfile: "missing-profile",
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)

	_, err := factory(context.Background(), reg)
	if err == nil {
		t.Fatal("factory should return error when credential is not found")
	}
}

// T7: 同一 tenant・異なる alias — 同じトークンが使われる
func TestSpaceAwareClientFactory_SameTenant_DifferentAlias_SameToken(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"admin","name":"Admin User"}`))
	}))
	defer ts.Close()

	getTokenCalls := 0
	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, tenant string) (*auth.TokenRecord, error) {
			getTokenCalls++
			return &auth.TokenRecord{
				UserID:      "u1",
				Provider:    "backlog",
				Tenant:      tenant,
				AccessToken: "shared-token-myorg",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			return nil, nil
		},
	}

	reg1 := space.SpaceRegistration{
		Alias:    "prod-ro",
		Tenant:   "myorg",
		BaseURL:  ts.URL,
		AuthType: space.AuthTypeOAuth,
	}
	reg2 := space.SpaceRegistration{
		Alias:    "prod-rw",
		Tenant:   "myorg",
		BaseURL:  ts.URL,
		AuthType: space.AuthTypeOAuth,
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)
	ctx := auth.ContextWithUserID(context.Background(), "u1")

	client1, err := factory(ctx, reg1)
	if err != nil {
		t.Fatalf("factory for reg1 returned error: %v", err)
	}
	client2, err := factory(ctx, reg2)
	if err != nil {
		t.Fatalf("factory for reg2 returned error: %v", err)
	}

	_, _ = client1.GetMyself(ctx)
	_, _ = client2.GetMyself(ctx)

	// 同一 tenant であっても factory は2回呼ばれる（singleflight は TokenManager 側）
	if getTokenCalls != 2 {
		t.Errorf("expected 2 GetValidToken calls (one per factory invocation), got %d", getTokenCalls)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", callCount)
	}
}

// T7: マルチスペース — reg.BaseURL ごとに異なるサーバーへルーティングされる
// M22: cli/mcp.go が oauthDeps.Factory (heptagon固定) を使っていたバグの回帰防止テスト
func TestSpaceAwareClientFactory_OAuth_RoutesToRegBaseURL(t *testing.T) {
	var heptagonCalls, megumilogCalls int

	heptagonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		heptagonCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"admin","name":"Heptagon User"}`))
	}))
	defer heptagonServer.Close()

	megumilogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		megumilogCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":2,"userId":"meg","name":"Megumilog User"}`))
	}))
	defer megumilogServer.Close()

	tm := &mockTokenManager{
		getValidTokenFunc: func(_ context.Context, _, _, tenant string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				UserID:      "u1",
				Provider:    "backlog",
				Tenant:      tenant,
				AccessToken: "token-" + tenant,
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}
	credResolver := &mockCredResolver{
		resolveFunc: func(_ string, _ credentials.CredentialFlags, _ func(string) string) (*credentials.ResolvedCredential, error) {
			return nil, nil
		},
	}

	heptagonReg := space.SpaceRegistration{
		Alias:    "heptagon",
		Tenant:   "heptagon",
		BaseURL:  heptagonServer.URL,
		AuthType: space.AuthTypeOAuth,
	}
	megumilogReg := space.SpaceRegistration{
		Alias:    "megumilog",
		Tenant:   "megumilog",
		BaseURL:  megumilogServer.URL,
		AuthType: space.AuthTypeOAuth,
	}

	factory := auth.NewSpaceAwareClientFactory(tm, credResolver)
	ctx := auth.ContextWithUserID(context.Background(), "u1")

	// heptagon 用クライアントは heptagonServer を呼ぶ
	heptagonClient, err := factory(ctx, heptagonReg)
	if err != nil {
		t.Fatalf("factory(heptagon) error: %v", err)
	}
	if _, err := heptagonClient.GetMyself(ctx); err != nil {
		t.Fatalf("heptagonClient.GetMyself error: %v", err)
	}

	// megumilog 用クライアントは megumilogServer を呼ぶ
	megumilogClient, err := factory(ctx, megumilogReg)
	if err != nil {
		t.Fatalf("factory(megumilog) error: %v", err)
	}
	if _, err := megumilogClient.GetMyself(ctx); err != nil {
		t.Fatalf("megumilogClient.GetMyself error: %v", err)
	}

	if heptagonCalls != 1 {
		t.Errorf("heptagonServer calls = %d, want 1", heptagonCalls)
	}
	if megumilogCalls != 1 {
		t.Errorf("megumilogServer calls = %d, want 1", megumilogCalls)
	}
}
