// Package credentials_test — OAuth フロー コンポーネントテスト。
package credentials_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/credentials"
)

// ---- BuildAuthorizeURL ----

func TestBuildAuthorizeURL_Basic(t *testing.T) {
	t.Parallel()
	got := credentials.BuildAuthorizeURL("example-space", "CLIENT_ID", "http://localhost:12345/callback", "RANDOM_STATE")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("BuildAuthorizeURL() returned invalid URL: %v", err)
	}
	if parsed.Scheme != "https" {
		t.Errorf("scheme = %q, want %q", parsed.Scheme, "https")
	}
	if parsed.Host != "example-space.backlog.com" {
		t.Errorf("host = %q, want %q", parsed.Host, "example-space.backlog.com")
	}
	if !strings.Contains(parsed.Path, "OAuth2AccessRequest") {
		t.Errorf("path %q does not contain %q", parsed.Path, "OAuth2AccessRequest")
	}
	q := parsed.Query()
	if q.Get("client_id") != "CLIENT_ID" {
		t.Errorf("client_id = %q, want %q", q.Get("client_id"), "CLIENT_ID")
	}
	if q.Get("redirect_uri") != "http://localhost:12345/callback" {
		t.Errorf("redirect_uri = %q, want %q", q.Get("redirect_uri"), "http://localhost:12345/callback")
	}
	if q.Get("state") != "RANDOM_STATE" {
		t.Errorf("state = %q, want %q", q.Get("state"), "RANDOM_STATE")
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want %q", q.Get("response_type"), "code")
	}
}

// ---- ExchangeCode ----

func TestExchangeCode_Success(t *testing.T) {
	t.Parallel()
	// httptest.Server でトークンエンドポイントをモック
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "AUTH_CODE" {
			t.Errorf("code = %q, want AUTH_CODE", r.FormValue("code"))
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"access_token":  "ACCESS_TOKEN_RESP",
			"refresh_token": "REFRESH_TOKEN_RESP",
			"token_type":    "Bearer",
			"expires_in":    3600,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("json.Encode() error: %v", err)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	result, err := credentials.ExchangeCode(ctx, server.URL, "CLIENT_ID", "CLIENT_SECRET", "AUTH_CODE", "http://localhost/callback")
	if err != nil {
		t.Fatalf("ExchangeCode() returned unexpected error: %v", err)
	}
	if result.AccessToken != "ACCESS_TOKEN_RESP" {
		t.Errorf("result.AccessToken = %q, want %q", result.AccessToken, "ACCESS_TOKEN_RESP")
	}
	if result.RefreshToken != "REFRESH_TOKEN_RESP" {
		t.Errorf("result.RefreshToken = %q, want %q", result.RefreshToken, "REFRESH_TOKEN_RESP")
	}
	if result.ExpiresIn != 3600 {
		t.Errorf("result.ExpiresIn = %d, want %d", result.ExpiresIn, 3600)
	}
}

func TestExchangeCode_ServerError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid code"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := credentials.ExchangeCode(ctx, server.URL, "CLIENT_ID", "CLIENT_SECRET", "BAD_CODE", "http://localhost/callback")
	if err == nil {
		t.Fatal("ExchangeCode() returned nil error for 400 response, want error")
	}
}

func TestExchangeCode_ContextTimeout(t *testing.T) {
	t.Parallel()
	// レスポンスを遅延するサーバー
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := credentials.ExchangeCode(ctx, server.URL, "CLIENT_ID", "CLIENT_SECRET", "CODE", "http://localhost/callback")
	if err == nil {
		t.Fatal("ExchangeCode() returned nil error for timeout, want error")
	}
}

// ---- StartCallbackServer ----

func TestStartCallbackServer_ReceivesCode(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	codeCh, redirectURI, err := credentials.StartCallbackServer(ctx)
	if err != nil {
		t.Fatalf("StartCallbackServer() returned unexpected error: %v", err)
	}
	if redirectURI == "" {
		t.Fatal("StartCallbackServer() returned empty redirectURI")
	}

	// redirectURI にコードを送信
	go func() {
		callbackURL := redirectURI + "?code=CALLBACK_CODE&state=SOME_STATE"
		resp, err := http.Get(callbackURL)
		if err != nil {
			return
		}
		resp.Body.Close()
	}()

	select {
	case result := <-codeCh:
		if result.Code != "CALLBACK_CODE" {
			t.Errorf("received code = %q, want %q", result.Code, "CALLBACK_CODE")
		}
		if result.State != "SOME_STATE" {
			t.Errorf("received state = %q, want %q", result.State, "SOME_STATE")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for callback code")
	}
}

func TestStartCallbackServer_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	codeCh, _, err := credentials.StartCallbackServer(ctx)
	if err != nil {
		t.Fatalf("StartCallbackServer() returned unexpected error: %v", err)
	}

	// コールバックを送らずにコンテキストをキャンセル
	cancel()

	select {
	case result := <-codeCh:
		if result.Err == nil {
			t.Error("expected error from context cancellation, got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for channel to close after context cancellation")
	}
}

// ---- GenerateState ----

func TestGenerateState_Unique(t *testing.T) {
	t.Parallel()
	s1, err1 := credentials.GenerateState()
	s2, err2 := credentials.GenerateState()
	if err1 != nil {
		t.Fatalf("GenerateState() error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("GenerateState() error: %v", err2)
	}
	if s1 == s2 {
		t.Error("GenerateState() returned same value twice, want unique values")
	}
	if len(s1) < 16 {
		t.Errorf("GenerateState() = %q (len %d), want at least 16 chars", s1, len(s1))
	}
}
