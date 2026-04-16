package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

func TestBacklogProvider_Name(t *testing.T) {
	p, err := NewBacklogOAuthProvider("test-space", "client-id", "client-secret")
	if err != nil {
		t.Fatalf("NewBacklogOAuthProvider() error = %v", err)
	}

	if got := p.Name(); got != "backlog" {
		t.Errorf("Name() = %q, want %q", got, "backlog")
	}
}

func TestBacklogProvider_NewWithEmptySpace(t *testing.T) {
	_, err := NewBacklogOAuthProvider("", "client-id", "client-secret")
	if err == nil {
		t.Fatal("NewBacklogOAuthProvider() with empty space should return error")
	}
	if !errors.Is(err, auth.ErrInvalidTenant) {
		t.Errorf("error = %v, want ErrInvalidTenant", err)
	}
}

func TestBacklogProvider_BuildAuthorizationURL(t *testing.T) {
	p, err := NewBacklogOAuthProvider("my-space", "my-client-id", "my-secret")
	if err != nil {
		t.Fatalf("NewBacklogOAuthProvider() error = %v", err)
	}

	got, err := p.BuildAuthorizationURL("test-state", "http://localhost/callback")
	if err != nil {
		t.Fatalf("BuildAuthorizationURL() error = %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("URL parse error: %v", err)
	}

	// ホスト確認
	if u.Host != "my-space.backlog.com" {
		t.Errorf("host = %q, want %q", u.Host, "my-space.backlog.com")
	}

	// パラメータ確認
	params := u.Query()
	checks := map[string]string{
		"response_type": "code",
		"client_id":     "my-client-id",
		"redirect_uri":  "http://localhost/callback",
		"state":         "test-state",
	}
	for key, want := range checks {
		if got := params.Get(key); got != want {
			t.Errorf("param %q = %q, want %q", key, got, want)
		}
	}
}

func TestBacklogProvider_ExchangeCode(t *testing.T) {
	// httptest サーバーでトークンエンドポイントをモック
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// パス確認
		if r.URL.Path != "/api/v2/oauth2/token" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/v2/oauth2/token")
		}

		// POST メソッド確認
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		// Content-Type 確認
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %q, want application/x-www-form-urlencoded", ct)
		}

		// フォームパラメータ確認
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))

		if form.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", form.Get("grant_type"))
		}
		if form.Get("code") != "test-code" {
			t.Errorf("code = %q, want test-code", form.Get("code"))
		}
		if form.Get("client_id") != "my-client-id" {
			t.Errorf("client_id = %q, want my-client-id", form.Get("client_id"))
		}
		if form.Get("client_secret") != "my-secret" {
			t.Errorf("client_secret = %q, want my-secret", form.Get("client_secret"))
		}
		if form.Get("redirect_uri") != "http://localhost/callback" {
			t.Errorf("redirect_uri = %q, want http://localhost/callback", form.Get("redirect_uri"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	before := time.Now()
	record, err := p.ExchangeCode(context.Background(), "test-code", "http://localhost/callback")
	after := time.Now()

	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}

	if record.AccessToken != "test-access-token" {
		t.Errorf("AccessToken = %q, want %q", record.AccessToken, "test-access-token")
	}
	if record.RefreshToken != "test-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", record.RefreshToken, "test-refresh-token")
	}
	if record.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", record.TokenType, "Bearer")
	}
	if record.Provider != "backlog" {
		t.Errorf("Provider = %q, want %q", record.Provider, "backlog")
	}
	if record.Tenant != "test-space" {
		t.Errorf("Tenant = %q, want %q", record.Tenant, "test-space")
	}

	// Expiry は now + 3600 秒の範囲内であること
	expectedExpiry := before.Add(3600 * time.Second)
	if record.Expiry.Before(expectedExpiry.Add(-2*time.Second)) || record.Expiry.After(after.Add(3600*time.Second+2*time.Second)) {
		t.Errorf("Expiry = %v, expected around %v", record.Expiry, expectedExpiry)
	}

	// UserID と ProviderUserID は空のままであること（caller が設定する）
	if record.UserID != "" {
		t.Errorf("UserID = %q, want empty", record.UserID)
	}
	if record.ProviderUserID != "" {
		t.Errorf("ProviderUserID = %q, want empty", record.ProviderUserID)
	}
}

func TestBacklogProvider_ExchangeCode_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors":[{"message":"invalid code"}]}`))
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	_, err := p.ExchangeCode(context.Background(), "bad-code", "http://localhost/callback")
	if err == nil {
		t.Fatal("ExchangeCode() with bad code should return error")
	}
}

func TestBacklogProvider_RefreshToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/oauth2/token" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/v2/oauth2/token")
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))

		if form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", form.Get("grant_type"))
		}
		if form.Get("refresh_token") != "old-refresh-token" {
			t.Errorf("refresh_token = %q, want old-refresh-token", form.Get("refresh_token"))
		}
		if form.Get("client_id") != "my-client-id" {
			t.Errorf("client_id = %q, want my-client-id", form.Get("client_id"))
		}
		if form.Get("client_secret") != "my-secret" {
			t.Errorf("client_secret = %q, want my-secret", form.Get("client_secret"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	before := time.Now()
	record, err := p.RefreshToken(context.Background(), "old-refresh-token")
	after := time.Now()

	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	if record.AccessToken != "new-access-token" {
		t.Errorf("AccessToken = %q, want %q", record.AccessToken, "new-access-token")
	}
	if record.RefreshToken != "new-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", record.RefreshToken, "new-refresh-token")
	}
	if record.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", record.TokenType, "Bearer")
	}
	if record.Provider != "backlog" {
		t.Errorf("Provider = %q, want %q", record.Provider, "backlog")
	}
	if record.Tenant != "test-space" {
		t.Errorf("Tenant = %q, want %q", record.Tenant, "test-space")
	}

	// Expiry 確認
	expectedExpiry := before.Add(3600 * time.Second)
	if record.Expiry.Before(expectedExpiry.Add(-2*time.Second)) || record.Expiry.After(after.Add(3600*time.Second+2*time.Second)) {
		t.Errorf("Expiry = %v, expected around %v", record.Expiry, expectedExpiry)
	}
}

func TestBacklogProvider_RefreshToken_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors":[{"message":"invalid refresh token"}]}`))
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	_, err := p.RefreshToken(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("RefreshToken() with bad token should return error")
	}
}

func TestBacklogProvider_GetCurrentUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/users/myself" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/v2/users/myself")
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		// Authorization ヘッダ確認
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want %q", authHeader, "Bearer test-access-token")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          12345,
			"userId":      "example-user",
			"name":        "Example User",
			"mailAddress": "user@example.com",
		})
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	user, err := p.GetCurrentUser(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("GetCurrentUser() error = %v", err)
	}

	if user.ID != strconv.Itoa(12345) {
		t.Errorf("ID = %q, want %q", user.ID, "12345")
	}
	if user.Name != "Example User" {
		t.Errorf("Name = %q, want %q", user.Name, "Example User")
	}
	if user.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "user@example.com")
	}
}

func TestBacklogProvider_GetCurrentUser_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":[{"message":"internal error"}]}`))
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	_, err := p.GetCurrentUser(context.Background(), "test-access-token")
	if err == nil {
		t.Fatal("GetCurrentUser() with server error should return error")
	}
}

func TestBacklogProvider_GetCurrentUser_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer ts.Close()

	p, _ := NewBacklogOAuthProvider("test-space", "my-client-id", "my-secret")
	p.baseURL = ts.URL

	_, err := p.GetCurrentUser(context.Background(), "expired-token")
	if err == nil {
		t.Fatal("GetCurrentUser() with 401 should return error")
	}
}

// TestBacklogProvider_Interface はコンパイル時の interface 適合チェック。
var _ OAuthProvider = (*BacklogOAuthProvider)(nil)
