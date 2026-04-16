package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/auth/provider"
	httptransport "github.com/youyo/logvalet/internal/transport/http"
)

// テスト定数
var (
	testSecret      = []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	testTenant      = "test-space"
	testRedirectURI = "https://example.com/oauth/backlog/callback"
	testTTL         = 10 * time.Minute
	testUserID      = "user-123"
)

// fakeProvider は provider.OAuthProvider のテスト用モック。
type fakeProvider struct {
	name    string
	buildFn func(state, redirectURI string) (string, error)
}

func (f *fakeProvider) Name() string {
	if f.name == "" {
		return "backlog"
	}
	return f.name
}

func (f *fakeProvider) BuildAuthorizationURL(state, redirectURI string) (string, error) {
	if f.buildFn != nil {
		return f.buildFn(state, redirectURI)
	}
	// デフォルト: 正常系のモック URL
	u := &url.URL{
		Scheme: "https",
		Host:   "test-space.backlog.com",
		Path:   "/OAuth2AccessRequest.action",
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", "test-client-id")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (f *fakeProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.TokenRecord, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeProvider) GetCurrentUser(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
	return nil, errors.New("not implemented")
}

// 型アサーションで interface 実装を検証
var _ provider.OAuthProvider = (*fakeProvider)(nil)

// newTestHandler はテスト用の OAuthHandler を構築する（正常値）。
func newTestHandler(t *testing.T, logger *slog.Logger) *httptransport.OAuthHandler {
	t.Helper()
	h, err := httptransport.NewOAuthHandler(
		&fakeProvider{},
		testTenant,
		testRedirectURI,
		testSecret,
		testTTL,
		logger,
	)
	if err != nil {
		t.Fatalf("NewOAuthHandler() error = %v", err)
	}
	return h
}

// ============================================================================
// NewOAuthHandler のテスト
// ============================================================================

func TestNewOAuthHandler_NilProvider_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewOAuthHandler(nil provider) did not panic")
		}
	}()
	_, _ = httptransport.NewOAuthHandler(nil, testTenant, testRedirectURI, testSecret, testTTL, nil)
}

func TestNewOAuthHandler_EmptyTenant(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, "", testRedirectURI, testSecret, testTTL, nil)
	if !errors.Is(err, auth.ErrInvalidTenant) {
		t.Errorf("error = %v, want ErrInvalidTenant", err)
	}
}

func TestNewOAuthHandler_EmptyRedirectURI(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, "", testSecret, testTTL, nil)
	if !errors.Is(err, auth.ErrInvalidRedirectURI) {
		t.Errorf("error = %v, want ErrInvalidRedirectURI", err)
	}
}

func TestNewOAuthHandler_NilStateSecret(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, nil, testTTL, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_EmptyStateSecret(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, []byte{}, testTTL, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_ZeroTTL(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, testSecret, 0, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_NegativeTTL(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, testSecret, -1*time.Minute, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_NilLogger_UsesDefault(t *testing.T) {
	h, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, testSecret, testTTL, nil)
	if err != nil {
		t.Fatalf("NewOAuthHandler() error = %v", err)
	}
	if h == nil {
		t.Fatal("NewOAuthHandler() returned nil handler")
	}
}

func TestNewOAuthHandler_Valid(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, testSecret, testTTL, logger)
	if err != nil {
		t.Fatalf("NewOAuthHandler() error = %v", err)
	}
	if h == nil {
		t.Fatal("NewOAuthHandler() returned nil handler")
	}
}

// ============================================================================
// HandleAuthorize のテスト
// ============================================================================

func TestHandleAuthorize_MethodNotAllowed(t *testing.T) {
	methods := []string{stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodDelete, stdhttp.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
			ctx := auth.ContextWithUserID(context.Background(), testUserID)
			req := httptest.NewRequest(method, "/oauth/backlog/authorize", nil).WithContext(ctx)
			rec := httptest.NewRecorder()

			h.HandleAuthorize(rec, req)

			if rec.Code != stdhttp.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusMethodNotAllowed)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body unmarshal: %v", err)
			}
			if body["error"] != "method_not_allowed" {
				t.Errorf("error = %q, want method_not_allowed", body["error"])
			}
		})
	}
}

func TestHandleAuthorize_Unauthenticated(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	// context に userID を注入しない
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body unmarshal: %v", err)
	}
	if body["error"] != "unauthenticated" {
		t.Errorf("error = %q, want unauthenticated", body["error"])
	}
}

func TestHandleAuthorize_Redirects(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusFound)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	if u.Host != "test-space.backlog.com" {
		t.Errorf("Location host = %q, want test-space.backlog.com", u.Host)
	}
	if u.Query().Get("state") == "" {
		t.Error("state query parameter is empty")
	}
	if u.Query().Get("redirect_uri") != testRedirectURI {
		t.Errorf("redirect_uri = %q, want %q", u.Query().Get("redirect_uri"), testRedirectURI)
	}
}

func TestHandleAuthorize_StateInURL(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("state query parameter is empty")
	}

	claims, err := auth.ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	if claims.UserID != testUserID {
		t.Errorf("claims.UserID = %q, want %q", claims.UserID, testUserID)
	}
}

func TestHandleAuthorize_StateClaimsMatch(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	claims, err := auth.ValidateState(u.Query().Get("state"), testSecret)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	if claims.UserID != testUserID {
		t.Errorf("claims.UserID = %q, want %q", claims.UserID, testUserID)
	}
	if claims.Tenant != testTenant {
		t.Errorf("claims.Tenant = %q, want %q", claims.Tenant, testTenant)
	}
	if claims.Nonce == "" {
		t.Error("claims.Nonce is empty")
	}
}

func TestHandleAuthorize_ProviderError(t *testing.T) {
	fp := &fakeProvider{
		buildFn: func(state, redirectURI string) (string, error) {
			return "", errors.New("provider failure")
		},
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h, err := httptransport.NewOAuthHandler(fp, testTenant, testRedirectURI, testSecret, testTTL, logger)
	if err != nil {
		t.Fatalf("NewOAuthHandler: %v", err)
	}

	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusInternalServerError)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if body["error"] != "internal_error" {
		t.Errorf("error = %q, want internal_error", body["error"])
	}
}

func TestHandleAuthorize_Nonce_Unique(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)

	do := func() string {
		req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		h.HandleAuthorize(rec, req)
		u, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatalf("Location parse: %v", err)
		}
		claims, err := auth.ValidateState(u.Query().Get("state"), testSecret)
		if err != nil {
			t.Fatalf("ValidateState: %v", err)
		}
		return claims.Nonce
	}

	nonce1 := do()
	nonce2 := do()
	if nonce1 == nonce2 {
		t.Errorf("nonces are identical: %q (expected unique per request)", nonce1)
	}
}

func TestHandleAuthorize_LogsSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h, err := httptransport.NewOAuthHandler(&fakeProvider{name: "backlog"}, testTenant, testRedirectURI, testSecret, testTTL, logger)
	if err != nil {
		t.Fatalf("NewOAuthHandler: %v", err)
	}
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusFound)
	}

	// ログをパース
	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("log unmarshal: %v (body=%q)", err, buf.String())
	}
	if logEntry["msg"] != "oauth authorize started" {
		t.Errorf("log msg = %v, want 'oauth authorize started'", logEntry["msg"])
	}
	if logEntry["user_id"] != testUserID {
		t.Errorf("log user_id = %v, want %q", logEntry["user_id"], testUserID)
	}
	if logEntry["provider"] != "backlog" {
		t.Errorf("log provider = %v, want 'backlog'", logEntry["provider"])
	}
	if logEntry["tenant"] != testTenant {
		t.Errorf("log tenant = %v, want %q", logEntry["tenant"], testTenant)
	}
}

func TestHandleAuthorize_DoesNotLogSecret(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h, err := httptransport.NewOAuthHandler(&fakeProvider{}, testTenant, testRedirectURI, testSecret, testTTL, logger)
	if err != nil {
		t.Fatalf("NewOAuthHandler: %v", err)
	}
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	logs := buf.String()
	secretHex := string(testSecret)
	if strings.Contains(logs, secretHex) {
		t.Errorf("logs contain state secret: %s", logs)
	}

	// state JWT 生値もログに含まれないこと
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	stateJWT := u.Query().Get("state")
	if stateJWT != "" && strings.Contains(logs, stateJWT) {
		t.Errorf("logs contain state JWT: %s", logs)
	}
}
