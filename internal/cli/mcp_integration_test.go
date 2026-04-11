//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	idproxy "github.com/youyo/idproxy"
	"github.com/youyo/idproxy/store"
	"github.com/youyo/idproxy/testutil"
	"github.com/youyo/logvalet/internal/cli"
)

// cookieSecret はテスト用の 32 バイト Cookie 暗号化キー。
var testCookieSecret = []byte("01234567890123456789012345678901")

// newNoRedirectClient はリダイレクトを追跡しない HTTP クライアントを返す。
func newNoRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// performBrowserLogin はブラウザ認証フローを実行し、セッション Cookie を返す。
func performBrowserLogin(t *testing.T, authSrv *httptest.Server, mockIdP *testutil.MockIdP) []*http.Cookie {
	t.Helper()
	client := newNoRedirectClient()

	// 1. GET /login → IdP にリダイレクト
	resp, err := client.Get(authSrv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 from /login, got %d", resp.StatusCode)
	}

	// 2. GET IdP /authorize → callback にリダイレクト
	resp, err = client.Get(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("GET IdP authorize failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 from IdP, got %d", resp.StatusCode)
	}

	// 3. GET /callback → セッション Cookie 発行
	resp, err = client.Get(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("GET /callback failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 from /callback, got %d", resp.StatusCode)
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies from /callback")
	}
	return cookies
}

// obtainBearerToken は DCR → ブラウザログイン → authorize(PKCE) → token exchange のフルフローで
// Bearer トークンを取得する。
func obtainBearerToken(t *testing.T, authSrv *httptest.Server, mockIdP *testutil.MockIdP) string {
	t.Helper()
	client := newNoRedirectClient()

	// 1. DCR
	dcrBody, _ := json.Marshal(map[string]any{
		"redirect_uris": []string{"http://localhost:9999/callback"},
		"client_name":   "e2e-test",
	})
	resp, err := client.Post(authSrv.URL+"/register", "application/json", bytes.NewReader(dcrBody))
	if err != nil {
		t.Fatalf("POST /register failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("DCR failed: %d %s", resp.StatusCode, body)
	}
	var dcrResp map[string]any
	json.NewDecoder(resp.Body).Decode(&dcrResp)
	clientID := dcrResp["client_id"].(string)

	// 2. ブラウザログインでセッション取得
	cookies := performBrowserLogin(t, authSrv, mockIdP)

	// 3. Authorize with PKCE
	codeVerifier := "test-code-verifier-that-is-long-enough-for-pkce-validation"
	codeChallenge := idproxy.S256Challenge(codeVerifier)
	authorizeURL := fmt.Sprintf(
		"%s/authorize?response_type=code&client_id=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=S256&state=test-state&scope=openid+email",
		authSrv.URL, clientID, url.QueryEscape("http://localhost:9999/callback"), codeChallenge,
	)
	req, _ := http.NewRequest("GET", authorizeURL, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("GET /authorize failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 from /authorize, got %d", resp.StatusCode)
	}
	parsedRedirect, _ := url.Parse(resp.Header.Get("Location"))
	authCode := parsedRedirect.Query().Get("code")
	if authCode == "" {
		t.Fatal("no code in authorize redirect")
	}

	// 4. Token exchange
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode},
		"redirect_uri":  {"http://localhost:9999/callback"},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
	resp, err = client.PostForm(authSrv.URL+"/token", tokenForm)
	if err != nil {
		t.Fatalf("POST /token failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("token exchange failed: %d %s", resp.StatusCode, body)
	}
	var tokenResp map[string]any
	json.NewDecoder(resp.Body).Decode(&tokenResp)
	accessToken, ok := tokenResp["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatal("no access_token in token response")
	}
	return accessToken
}

// echoMCPHandler は簡易 MCP エコーハンドラー。認証が通ったことを確認するため 200 + JSON を返す。
func echoMCPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"jsonrpc":"2.0","result":"ok"}`))
}

// setupTestAuthServer は認証付きテストサーバーを構築する。
// logvalet の McpCmd.Run() と同じ Handler Topology を再現:
//
//	topMux: /healthz → healthHandler, / → auth.Wrap(mcpMux)
//	mcpMux: /mcp → echoMCPHandler
func setupTestAuthServer(t *testing.T) (*httptest.Server, *testutil.MockIdP, *ecdsa.PrivateKey) {
	t.Helper()

	mockIdP := testutil.NewMockIdP(t)

	memStore := store.NewMemoryStore()
	t.Cleanup(func() { _ = memStore.Close() })

	signingKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate signing key: %v", err)
	}

	// dummy server を先に起動して ExternalURL を取得
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(dummyHandler)

	cfg := idproxy.Config{
		Providers: []idproxy.OIDCProvider{
			{
				Issuer:       mockIdP.Issuer(),
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
		},
		ExternalURL:  srv.URL,
		CookieSecret: testCookieSecret,
		Store:        memStore,
		OAuth: &idproxy.OAuthConfig{
			SigningKey: signingKey,
			AllowedRedirectURIs: []string{
				srv.URL + "/callback",
				"http://localhost:9999/callback",
			},
		},
	}

	auth, err := idproxy.New(context.Background(), cfg)
	if err != nil {
		srv.Close()
		t.Fatalf("failed to create auth: %v", err)
	}

	// logvalet と同じ Handler Topology を構築
	mcpMux := http.NewServeMux()
	mcpMux.HandleFunc("/mcp", echoMCPHandler)

	topMux := http.NewServeMux()
	topMux.HandleFunc("/healthz", cli.HealthHandler)
	topMux.Handle("/", auth.Wrap(mcpMux))

	srv.Config.Handler = topMux
	t.Cleanup(srv.Close)

	return srv, mockIdP, signingKey
}

// --- E2E テスト ---

func TestIntegration_HealthzBypassesAuth(t *testing.T) {
	srv, _, _ := setupTestAuthServer(t)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", string(body), `{"status":"ok"}`)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestIntegration_UnauthenticatedMCPReturns401(t *testing.T) {
	srv, _, _ := setupTestAuthServer(t)
	client := newNoRedirectClient()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestIntegration_ValidBearerTokenAccessesMCP(t *testing.T) {
	srv, mockIdP, _ := setupTestAuthServer(t)

	// フル OAuth フロー（DCR → ブラウザログイン → authorize → token）で Bearer 取得
	accessToken := obtainBearerToken(t, srv, mockIdP)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp with bearer failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d, body = %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestIntegration_InvalidBearerTokenReturns401(t *testing.T) {
	srv, _, _ := setupTestAuthServer(t)
	client := newNoRedirectClient()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token-here")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp with invalid bearer failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestIntegration_ExpiredBearerTokenReturns401(t *testing.T) {
	srv, mockIdP, _ := setupTestAuthServer(t)
	client := newNoRedirectClient()

	// フル OAuth フローでトークンを取得後、手動で期限切れトークンを生成はできない。
	// 代わりに無効なトークン文字列でテスト（期限切れと同等の拒否を確認）。
	// MockIdP の鍵で署名（OAuthServer の鍵と異なる）→ 署名不一致で拒否される
	token, err := mockIdP.IssueAccessToken(
		srv.URL, srv.URL, "test-subject", "user@example.com", "Test User",
		uuid.New().String(), time.Now().Add(-1*time.Hour),
	)
	if err != nil {
		t.Fatalf("failed to issue expired token: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp with expired bearer failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestIntegration_OAuthMetadataDiscovery(t *testing.T) {
	srv, _, _ := setupTestAuthServer(t)

	resp, err := http.Get(srv.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("GET metadata failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var meta map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}

	// 必須フィールドの存在確認
	for _, field := range []string{"issuer", "authorization_endpoint", "token_endpoint", "registration_endpoint"} {
		if _, ok := meta[field]; !ok {
			t.Errorf("metadata missing field: %s", field)
		}
	}
}

func TestIntegration_DynamicClientRegistration(t *testing.T) {
	srv, _, _ := setupTestAuthServer(t)

	body, _ := json.Marshal(map[string]any{
		"redirect_uris": []string{srv.URL + "/callback"},
		"client_name":   "test-mcp-client",
	})

	resp, err := http.Post(srv.URL+"/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /register failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d, body = %s", resp.StatusCode, http.StatusCreated, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode register response: %v", err)
	}

	if _, ok := result["client_id"]; !ok {
		t.Error("register response missing client_id")
	}
}

func TestIntegration_BrowserLoginRedirectsToIdP(t *testing.T) {
	srv, mockIdP, _ := setupTestAuthServer(t)
	client := newNoRedirectClient()

	resp, err := client.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("no Location header from /login")
	}

	// MockIdP の authorize エンドポイントにリダイレクトされることを確認
	if !strings.HasPrefix(location, mockIdP.Issuer()+"/authorize") {
		t.Errorf("Location = %q, want prefix %q", location, mockIdP.Issuer()+"/authorize")
	}
}
