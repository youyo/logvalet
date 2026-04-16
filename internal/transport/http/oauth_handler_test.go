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

// ============================================================================
// モック: fakeProvider (provider.OAuthProvider)
// ============================================================================

// fakeProvider は provider.OAuthProvider のテスト用モック。
// buildFn / exchangeFn / userFn でメソッドを差し替えられる。
type fakeProvider struct {
	name       string
	buildFn    func(state, redirectURI string) (string, error)
	exchangeFn func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error)
	userFn     func(ctx context.Context, accessToken string) (*auth.ProviderUser, error)
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
	if f.exchangeFn != nil {
		return f.exchangeFn(ctx, code, redirectURI)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.TokenRecord, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeProvider) GetCurrentUser(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
	if f.userFn != nil {
		return f.userFn(ctx, accessToken)
	}
	return nil, errors.New("not implemented")
}

// 型アサーションで interface 実装を検証
var _ provider.OAuthProvider = (*fakeProvider)(nil)

// ============================================================================
// モック: fakeTokenManager (auth.TokenManager)
// ============================================================================

// fakeTokenManager は auth.TokenManager のテスト用モック。
type fakeTokenManager struct {
	saveFn   func(ctx context.Context, record *auth.TokenRecord) error
	getFn    func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error)
	revokeFn func(ctx context.Context, userID, providerName, tenant string) error
}

func (f *fakeTokenManager) GetValidToken(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
	if f.getFn != nil {
		return f.getFn(ctx, userID, providerName, tenant)
	}
	return nil, nil
}

func (f *fakeTokenManager) SaveToken(ctx context.Context, record *auth.TokenRecord) error {
	if f.saveFn != nil {
		return f.saveFn(ctx, record)
	}
	return nil
}

func (f *fakeTokenManager) RevokeToken(ctx context.Context, userID, providerName, tenant string) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, userID, providerName, tenant)
	}
	return nil
}

var _ auth.TokenManager = (*fakeTokenManager)(nil)

// ============================================================================
// ヘルパー
// ============================================================================

// newTestHandler はテスト用の OAuthHandler を構築する（デフォルトモック）。
func newTestHandler(t *testing.T, logger *slog.Logger) *httptransport.OAuthHandler {
	t.Helper()
	return newTestHandlerWithDeps(t, logger, &fakeProvider{}, &fakeTokenManager{})
}

// newTestHandlerWithDeps は依存性を指定して OAuthHandler を構築する。
func newTestHandlerWithDeps(t *testing.T, logger *slog.Logger, p provider.OAuthProvider, tm auth.TokenManager) *httptransport.OAuthHandler {
	t.Helper()
	h, err := httptransport.NewOAuthHandler(p, tm, testTenant, testRedirectURI, testSecret, testTTL, logger)
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
	_, _ = httptransport.NewOAuthHandler(nil, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, testTTL, nil)
}

func TestNewOAuthHandler_NilTokenManager_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewOAuthHandler(nil tokenManager) did not panic")
		}
	}()
	_, _ = httptransport.NewOAuthHandler(&fakeProvider{}, nil, testTenant, testRedirectURI, testSecret, testTTL, nil)
}

func TestNewOAuthHandler_EmptyTenant(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, "", testRedirectURI, testSecret, testTTL, nil)
	if !errors.Is(err, auth.ErrInvalidTenant) {
		t.Errorf("error = %v, want ErrInvalidTenant", err)
	}
}

func TestNewOAuthHandler_EmptyRedirectURI(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, "", testSecret, testTTL, nil)
	if !errors.Is(err, auth.ErrInvalidRedirectURI) {
		t.Errorf("error = %v, want ErrInvalidRedirectURI", err)
	}
}

func TestNewOAuthHandler_NilStateSecret(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, nil, testTTL, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_EmptyStateSecret(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, []byte{}, testTTL, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_ZeroTTL(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, 0, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_NegativeTTL(t *testing.T) {
	_, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, -1*time.Minute, nil)
	if !errors.Is(err, auth.ErrStateInvalid) {
		t.Errorf("error = %v, want ErrStateInvalid", err)
	}
}

func TestNewOAuthHandler_NilLogger_UsesDefault(t *testing.T) {
	h, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, testTTL, nil)
	if err != nil {
		t.Fatalf("NewOAuthHandler() error = %v", err)
	}
	if h == nil {
		t.Fatal("NewOAuthHandler() returned nil handler")
	}
}

func TestNewOAuthHandler_Valid(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, testTTL, logger)
	if err != nil {
		t.Fatalf("NewOAuthHandler() error = %v", err)
	}
	if h == nil {
		t.Fatal("NewOAuthHandler() returned nil handler")
	}
}

// ============================================================================
// HandleAuthorize のテスト（M13 — tokenManager 引数追加に伴い更新済み）
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
	h, err := httptransport.NewOAuthHandler(fp, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, testTTL, logger)
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
	h, err := httptransport.NewOAuthHandler(&fakeProvider{name: "backlog"}, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, testTTL, logger)
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
	h, err := httptransport.NewOAuthHandler(&fakeProvider{}, &fakeTokenManager{}, testTenant, testRedirectURI, testSecret, testTTL, logger)
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

// ============================================================================
// HandleCallback のテスト（M14 新規）
// ============================================================================

// successCallbackSetup は callback 正常系のセットアップを行う。
// fakeProvider と fakeTokenManager を正常応答で構築し、state を生成して返す。
type successCallbackDeps struct {
	handler     *httptransport.OAuthHandler
	provider    *fakeProvider
	tokenMgr    *fakeTokenManager
	state       string
	savedRecord *auth.TokenRecord
	saveCalls   int
}

func setupCallbackSuccess(t *testing.T, logger *slog.Logger) *successCallbackDeps {
	t.Helper()
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	deps := &successCallbackDeps{}
	deps.tokenMgr = &fakeTokenManager{
		saveFn: func(ctx context.Context, rec *auth.TokenRecord) error {
			deps.saveCalls++
			deps.savedRecord = rec
			return nil
		},
	}
	deps.provider = &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				Provider:     "backlog",
				Tenant:       testTenant,
				AccessToken:  "at-xxx",
				RefreshToken: "rt-yyy",
				TokenType:    "Bearer",
				Expiry:       time.Now().Add(time.Hour),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}, nil
		},
		userFn: func(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
			return &auth.ProviderUser{ID: "12345", Name: "Taro Yamada", Email: "taro@example.com"}, nil
		},
	}
	deps.handler = newTestHandlerWithDeps(t, logger, deps.provider, deps.tokenMgr)

	state, err := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	deps.state = state
	return deps
}

// newCallbackRequest は userID を context に注入した GET リクエストを作成する。
func newCallbackRequest(userID string, query url.Values) *stdhttp.Request {
	u := "/oauth/backlog/callback"
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req := httptest.NewRequest(stdhttp.MethodGet, u, nil)
	if userID != "" {
		ctx := auth.ContextWithUserID(req.Context(), userID)
		req = req.WithContext(ctx)
	}
	return req
}

// ---- 2. メソッドチェック ----
func TestHandleCallback_MethodNotAllowed(t *testing.T) {
	methods := []string{stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodDelete, stdhttp.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
			req := httptest.NewRequest(method, "/oauth/backlog/callback", nil)
			rec := httptest.NewRecorder()

			h.HandleCallback(rec, req)

			if rec.Code != stdhttp.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusMethodNotAllowed)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body unmarshal: %v", err)
			}
			if body["error"] != "method_not_allowed" {
				t.Errorf("error = %q, want method_not_allowed", body["error"])
			}
		})
	}
}

// ---- 3. error クエリ優先 ----
func TestHandleCallback_ErrorQuery(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	q := url.Values{}
	q.Set("error", "access_denied")
	q.Set("error_description", "User denied the request")
	// code / state は意図的に空
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "provider_denied" {
		t.Errorf("error = %q, want provider_denied", body["error"])
	}
}

// ---- 4,5,6. code / state 空値チェック ----
func TestHandleCallback_MissingCode(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	q := url.Values{}
	q.Set("state", "some-state")
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_request" {
		t.Errorf("error = %q, want invalid_request", body["error"])
	}
}

func TestHandleCallback_MissingState(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	q := url.Values{}
	q.Set("code", "abc")
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_request" {
		t.Errorf("error = %q, want invalid_request", body["error"])
	}
}

func TestHandleCallback_BothMissing(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	req := newCallbackRequest(testUserID, url.Values{})
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_request" {
		t.Errorf("error = %q, want invalid_request", body["error"])
	}
}

// ---- 7. state 期限切れ ----
func TestHandleCallback_InvalidState_Expired(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	// 短い TTL で生成後、時間が経過したふりで検証させる → TTL = 1ms → sleep
	state, err := auth.GenerateState(testUserID, testTenant, testSecret, time.Millisecond)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // 期限切れ

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "state_expired" {
		t.Errorf("error = %q, want state_expired", body["error"])
	}
}

// ---- 8. state 改竄 ----
func TestHandleCallback_InvalidState_Tampered(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	// 別 secret で署名した state
	otherSecret := []byte("ffffffffffffffffffffffffffffffff")
	state, err := auth.GenerateState(testUserID, testTenant, otherSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "state_invalid" {
		t.Errorf("error = %q, want state_invalid", body["error"])
	}
}

// ---- 9. テナント不一致 ----
func TestHandleCallback_TenantMismatch(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	// handler の tenant は testTenant、state は別テナントで生成
	state, err := auth.GenerateState(testUserID, "other-space", testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_tenant" {
		t.Errorf("error = %q, want invalid_tenant", body["error"])
	}
}

// ---- 10. ctx userID 未設定 ----
func TestHandleCallback_Unauthenticated(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	state, err := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	// ctx に userID を注入しない
	req := newCallbackRequest("", q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "unauthenticated" {
		t.Errorf("error = %q, want unauthenticated", body["error"])
	}
}

// ---- 11. ctx userID と state.UserID 不一致 ----
func TestHandleCallback_UserMismatch(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	// state は testUserID で生成
	state, err := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	// ctx は別ユーザー
	req := newCallbackRequest("attacker-999", q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "user_mismatch" {
		t.Errorf("error = %q, want user_mismatch", body["error"])
	}
}

// ---- 12. ExchangeCode 失敗 ----
func TestHandleCallback_ExchangeCodeFailure(t *testing.T) {
	fp := &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			return nil, errors.New("token endpoint unreachable")
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), fp, &fakeTokenManager{})
	state, err := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadGateway)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "provider_error" {
		t.Errorf("error = %q, want provider_error", body["error"])
	}
}

// ---- 13. GetCurrentUser 失敗 ----
func TestHandleCallback_GetCurrentUserFailure(t *testing.T) {
	fp := &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				Provider: "backlog", Tenant: testTenant,
				AccessToken: "at", RefreshToken: "rt",
				Expiry: time.Now().Add(time.Hour),
			}, nil
		},
		userFn: func(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
			return nil, errors.New("user endpoint 403")
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), fp, &fakeTokenManager{})
	state, _ := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadGateway)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "provider_error" {
		t.Errorf("error = %q, want provider_error", body["error"])
	}
}

// ---- 14. SaveToken 失敗 ----
func TestHandleCallback_SaveTokenFailure(t *testing.T) {
	deps := setupCallbackSuccess(t, nil)
	deps.tokenMgr.saveFn = func(ctx context.Context, rec *auth.TokenRecord) error {
		return errors.New("store write failed")
	}

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", deps.state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusInternalServerError)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "internal_error" {
		t.Errorf("error = %q, want internal_error", body["error"])
	}
}

// ---- 15. 正常系 JSON 200 ----
func TestHandleCallback_Success_200JSON(t *testing.T) {
	deps := setupCallbackSuccess(t, nil)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", deps.state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if body["status"] != "connected" {
		t.Errorf("status = %q, want connected", body["status"])
	}
	if body["provider"] != "backlog" {
		t.Errorf("provider = %q, want backlog", body["provider"])
	}
	if body["tenant"] != testTenant {
		t.Errorf("tenant = %q, want %q", body["tenant"], testTenant)
	}
	if body["provider_user_id"] != "12345" {
		t.Errorf("provider_user_id = %q, want 12345", body["provider_user_id"])
	}
	if body["provider_user_name"] != "Taro Yamada" {
		t.Errorf("provider_user_name = %q, want 'Taro Yamada'", body["provider_user_name"])
	}
}

// ---- 16. 正常系: TokenRecord の identity fields が補完されて保存される ----
func TestHandleCallback_Success_SavesTokenRecord(t *testing.T) {
	deps := setupCallbackSuccess(t, nil)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", deps.state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	if deps.saveCalls != 1 {
		t.Fatalf("SaveToken was called %d times, want 1", deps.saveCalls)
	}
	got := deps.savedRecord
	if got == nil {
		t.Fatal("savedRecord is nil")
	}
	if got.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", got.UserID, testUserID)
	}
	if got.ProviderUserID != "12345" {
		t.Errorf("ProviderUserID = %q, want 12345", got.ProviderUserID)
	}
	if got.Provider != "backlog" {
		t.Errorf("Provider = %q, want backlog", got.Provider)
	}
	if got.Tenant != testTenant {
		t.Errorf("Tenant = %q, want %q", got.Tenant, testTenant)
	}
	if got.AccessToken != "at-xxx" {
		t.Errorf("AccessToken = %q, want at-xxx", got.AccessToken)
	}
	if got.RefreshToken != "rt-yyy" {
		t.Errorf("RefreshToken = %q, want rt-yyy", got.RefreshToken)
	}
}

// ---- 17. 正常系: exchangeFn / userFn / saveFn の ctx が request ctx 派生 ----
func TestHandleCallback_Success_ContextPropagated(t *testing.T) {
	var (
		exchangeCtxUser  string
		exchangeCtxFound bool
		userCtxUser      string
		userCtxFound     bool
		saveCtxUser      string
		saveCtxFound     bool
	)
	tm := &fakeTokenManager{
		saveFn: func(ctx context.Context, rec *auth.TokenRecord) error {
			saveCtxUser, saveCtxFound = auth.UserIDFromContext(ctx)
			return nil
		},
	}
	fp := &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			exchangeCtxUser, exchangeCtxFound = auth.UserIDFromContext(ctx)
			return &auth.TokenRecord{Provider: "backlog", Tenant: testTenant, AccessToken: "a", RefreshToken: "b", Expiry: time.Now().Add(time.Hour)}, nil
		},
		userFn: func(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
			userCtxUser, userCtxFound = auth.UserIDFromContext(ctx)
			return &auth.ProviderUser{ID: "12345"}, nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), fp, tm)
	state, _ := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if !exchangeCtxFound || exchangeCtxUser != testUserID {
		t.Errorf("exchangeCtx userID = %q / found=%v, want %q / true", exchangeCtxUser, exchangeCtxFound, testUserID)
	}
	if !userCtxFound || userCtxUser != testUserID {
		t.Errorf("userCtx userID = %q / found=%v, want %q / true", userCtxUser, userCtxFound, testUserID)
	}
	if !saveCtxFound || saveCtxUser != testUserID {
		t.Errorf("saveCtx userID = %q / found=%v, want %q / true", saveCtxUser, saveCtxFound, testUserID)
	}
}

// ---- 18. 成功ログ ----
func TestHandleCallback_LogsSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	deps := setupCallbackSuccess(t, logger)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", deps.state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}

	// ログを行単位でパース
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth callback success" {
			found = true
			if entry["user_id"] != testUserID {
				t.Errorf("log user_id = %v, want %q", entry["user_id"], testUserID)
			}
			if entry["provider"] != "backlog" {
				t.Errorf("log provider = %v, want 'backlog'", entry["provider"])
			}
			if entry["tenant"] != testTenant {
				t.Errorf("log tenant = %v, want %q", entry["tenant"], testTenant)
			}
			if entry["provider_user_id"] != "12345" {
				t.Errorf("log provider_user_id = %v, want '12345'", entry["provider_user_id"])
			}
		}
	}
	if !found {
		t.Errorf("log 'oauth callback success' not found; logs=%s", buf.String())
	}
}

// ---- 19. 失敗ログ ----
func TestHandleCallback_LogsFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	fp := &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			return nil, errors.New("upstream 500")
		},
	}
	h := newTestHandlerWithDeps(t, logger, fp, &fakeTokenManager{})
	state, _ := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusBadGateway)
	}

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth callback failed" {
			found = true
			if entry["reason"] != "exchange_failed" {
				t.Errorf("log reason = %v, want 'exchange_failed'", entry["reason"])
			}
			if entry["user_id"] != testUserID {
				t.Errorf("log user_id = %v, want %q", entry["user_id"], testUserID)
			}
		}
	}
	if !found {
		t.Errorf("log 'oauth callback failed' not found; logs=%s", buf.String())
	}
}

// ---- 20. 成功時ログに機微値が含まれない ----
func TestHandleCallback_DoesNotLogSensitive(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	deps := setupCallbackSuccess(t, logger)

	q := url.Values{}
	q.Set("code", "super-secret-code-XYZ")
	q.Set("state", deps.state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}

	logs := buf.String()
	for _, secret := range []string{
		"super-secret-code-XYZ", // code 生値
		deps.state,              // state JWT 生値
		"at-xxx",                // access token
		"rt-yyy",                // refresh token
		string(testSecret),      // state secret
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain sensitive value %q; logs=%s", secret, logs)
		}
	}
}

// ---- 21. provider_denied のログに description が含まれない ----
func TestHandleCallback_ProviderDenied_LogsReason(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, &fakeTokenManager{})

	q := url.Values{}
	q.Set("error", "access_denied")
	q.Set("error_description", "piidata-ABCDE-should-not-log")
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}

	logs := buf.String()
	// reason は出てよい
	if !strings.Contains(logs, "provider_denied") {
		t.Errorf("log should contain reason 'provider_denied'; logs=%s", logs)
	}
	// error_description は出してはいけない
	if strings.Contains(logs, "piidata-ABCDE-should-not-log") {
		t.Errorf("log contains error_description (PII risk); logs=%s", logs)
	}
}

// ---- 22. user_mismatch で SaveToken が呼ばれない ----
func TestHandleCallback_UserMismatch_DoesNotSave(t *testing.T) {
	saveCalled := 0
	tm := &fakeTokenManager{
		saveFn: func(ctx context.Context, rec *auth.TokenRecord) error {
			saveCalled++
			return nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)

	state, _ := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	// attacker context
	req := newCallbackRequest("attacker-x", q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
	if saveCalled != 0 {
		t.Errorf("SaveToken was called %d times, want 0", saveCalled)
	}
}

// ---- 23. ExchangeError の err.Error() がログに漏れないこと ----
func TestHandleCallback_ExchangeError_NoLeakInLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// 合成エラー: 実運用で upstream が token を echo するケースをシミュレート
	fp := &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			return nil, errors.New("upstream error: code=real-secret-code access_token=leaked-AAAA")
		},
	}
	h := newTestHandlerWithDeps(t, logger, fp, &fakeTokenManager{})
	state, _ := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)

	q := url.Values{}
	q.Set("code", "real-secret-code")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusBadGateway)
	}

	logs := buf.String()
	for _, leaky := range []string{
		"real-secret-code",
		"leaked-AAAA",
	} {
		if strings.Contains(logs, leaky) {
			t.Errorf("logs leak sensitive substring %q; logs=%s", leaky, logs)
		}
	}
}

// ---- 24. 成功レスポンスの Content-Type ----
func TestHandleCallback_Success_ContentTypeJSON(t *testing.T) {
	deps := setupCallbackSuccess(t, nil)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", deps.state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ============================================================================
// HandleStatus / HandleDisconnect のテスト（M15 新規）
// ============================================================================

// newStatusRequest は userID context 付きで GET /oauth/backlog/status リクエストを作成する。
func newStatusRequest(userID string) *stdhttp.Request {
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/status", nil)
	if userID != "" {
		req = req.WithContext(auth.ContextWithUserID(req.Context(), userID))
	}
	return req
}

// newDisconnectRequest は userID context 付きで DELETE /oauth/backlog/disconnect リクエストを作成する。
func newDisconnectRequest(userID string) *stdhttp.Request {
	req := httptest.NewRequest(stdhttp.MethodDelete, "/oauth/backlog/disconnect", nil)
	if userID != "" {
		req = req.WithContext(auth.ContextWithUserID(req.Context(), userID))
	}
	return req
}

// ---- 1. HandleStatus メソッドチェック ----
func TestHandleStatus_MethodNotAllowed(t *testing.T) {
	methods := []string{stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodDelete, stdhttp.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
			req := httptest.NewRequest(method, "/oauth/backlog/status", nil)
			req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
			rec := httptest.NewRecorder()

			h.HandleStatus(rec, req)

			if rec.Code != stdhttp.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusMethodNotAllowed)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body unmarshal: %v", err)
			}
			if body["error"] != "method_not_allowed" {
				t.Errorf("error = %q, want method_not_allowed", body["error"])
			}
		})
	}
}

// ---- 2. HandleStatus 未認証 ----
func TestHandleStatus_Unauthenticated(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	req := newStatusRequest("")
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "unauthenticated" {
		t.Errorf("error = %q, want unauthenticated", body["error"])
	}
}

// ---- 3. HandleStatus 未接続 ----
func TestHandleStatus_NotConnected(t *testing.T) {
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, auth.ErrProviderNotConnected
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if connected, ok := body["connected"].(bool); !ok || connected != false {
		t.Errorf("connected = %v (ok=%v), want false", body["connected"], ok)
	}
	if _, ok := body["needs_reauth"]; ok {
		t.Errorf("needs_reauth should be omitted when not connected; got %v", body["needs_reauth"])
	}
	if _, ok := body["provider"]; ok {
		t.Errorf("provider should be omitted when not connected; got %v", body["provider"])
	}
	if _, ok := body["tenant"]; ok {
		t.Errorf("tenant should be omitted when not connected; got %v", body["tenant"])
	}
	if _, ok := body["provider_user_id"]; ok {
		t.Errorf("provider_user_id should be omitted when not connected; got %v", body["provider_user_id"])
	}
}

// ---- 4. HandleStatus refresh 失敗 → needs_reauth ----
func TestHandleStatus_NeedsReauth_RefreshFailed(t *testing.T) {
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, auth.ErrTokenRefreshFailed
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if connected, ok := body["connected"].(bool); !ok || connected != true {
		t.Errorf("connected = %v (ok=%v), want true", body["connected"], ok)
	}
	if needs, ok := body["needs_reauth"].(bool); !ok || needs != true {
		t.Errorf("needs_reauth = %v (ok=%v), want true", body["needs_reauth"], ok)
	}
	if body["provider"] != "backlog" {
		t.Errorf("provider = %v, want backlog", body["provider"])
	}
	if body["tenant"] != testTenant {
		t.Errorf("tenant = %v, want %q", body["tenant"], testTenant)
	}
	if _, ok := body["provider_user_id"]; ok {
		t.Errorf("provider_user_id should be omitted on needs_reauth; got %v", body["provider_user_id"])
	}
}

// ---- 5. HandleStatus token expired (防御テスト) ----
func TestHandleStatus_NeedsReauth_TokenExpired(t *testing.T) {
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, auth.ErrTokenExpired
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if connected, _ := body["connected"].(bool); connected != true {
		t.Errorf("connected = %v, want true", body["connected"])
	}
	if needs, _ := body["needs_reauth"].(bool); needs != true {
		t.Errorf("needs_reauth = %v, want true", body["needs_reauth"])
	}
}

// ---- 6. HandleStatus 正常接続 ----
func TestHandleStatus_Connected(t *testing.T) {
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				UserID:         userID,
				Provider:       "backlog",
				Tenant:         tenant,
				AccessToken:    "at-secret",
				RefreshToken:   "rt-secret",
				TokenType:      "Bearer",
				Expiry:         time.Now().Add(time.Hour),
				ProviderUserID: "12345",
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}, nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if connected, _ := body["connected"].(bool); connected != true {
		t.Errorf("connected = %v, want true", body["connected"])
	}
	if _, ok := body["needs_reauth"]; ok {
		t.Errorf("needs_reauth should be omitted on healthy connection; got %v", body["needs_reauth"])
	}
	if body["provider"] != "backlog" {
		t.Errorf("provider = %v, want backlog", body["provider"])
	}
	if body["tenant"] != testTenant {
		t.Errorf("tenant = %v, want %q", body["tenant"], testTenant)
	}
	if body["provider_user_id"] != "12345" {
		t.Errorf("provider_user_id = %v, want 12345", body["provider_user_id"])
	}
}

// ---- 7. HandleStatus 内部エラー ----
func TestHandleStatus_InternalError(t *testing.T) {
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, errors.New("store down")
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusInternalServerError)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "internal_error" {
		t.Errorf("error = %q, want internal_error", body["error"])
	}
}

// ---- 8. HandleStatus Content-Type ----
func TestHandleStatus_ContentTypeJSON(t *testing.T) {
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, auth.ErrProviderNotConnected
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ---- 9. HandleStatus トークン漏洩なし ----
func TestHandleStatus_DoesNotLeakToken(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				UserID: userID, Provider: "backlog", Tenant: tenant,
				AccessToken: "super-secret-access-TOKEN",
				RefreshToken: "super-secret-refresh-TOKEN",
				Expiry: time.Now().Add(time.Hour),
				ProviderUserID: "12345",
			}, nil
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	// レスポンスボディにトークンが含まれない
	bodyStr := rec.Body.String()
	for _, secret := range []string{"super-secret-access-TOKEN", "super-secret-refresh-TOKEN", "access_token", "refresh_token"} {
		if strings.Contains(bodyStr, secret) {
			t.Errorf("response body contains sensitive value %q; body=%s", secret, bodyStr)
		}
	}

	// ログにトークンが含まれない
	logs := buf.String()
	for _, secret := range []string{"super-secret-access-TOKEN", "super-secret-refresh-TOKEN"} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain sensitive value %q; logs=%s", secret, logs)
		}
	}
}

// ---- 10. HandleStatus GetValidToken 引数 ----
func TestHandleStatus_PassesCorrectArgsToGetValidToken(t *testing.T) {
	var gotUserID, gotProvider, gotTenant string
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			gotUserID = userID
			gotProvider = providerName
			gotTenant = tenant
			return nil, auth.ErrProviderNotConnected
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if gotUserID != testUserID {
		t.Errorf("userID passed = %q, want %q", gotUserID, testUserID)
	}
	if gotProvider != "backlog" {
		t.Errorf("provider passed = %q, want backlog", gotProvider)
	}
	if gotTenant != testTenant {
		t.Errorf("tenant passed = %q, want %q", gotTenant, testTenant)
	}
}

// ---- 11. HandleStatus Connected ログ ----
func TestHandleStatus_LogsConnected(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{
				UserID: userID, Provider: "backlog", Tenant: tenant,
				AccessToken: "at", RefreshToken: "rt",
				Expiry: time.Now().Add(time.Hour), ProviderUserID: "12345",
			}, nil
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth status checked" && entry["outcome"] == "connected" {
			found = true
			if entry["user_id"] != testUserID {
				t.Errorf("log user_id = %v, want %q", entry["user_id"], testUserID)
			}
			if entry["provider"] != "backlog" {
				t.Errorf("log provider = %v, want backlog", entry["provider"])
			}
			if entry["tenant"] != testTenant {
				t.Errorf("log tenant = %v, want %q", entry["tenant"], testTenant)
			}
		}
	}
	if !found {
		t.Errorf("log with outcome=connected not found; logs=%s", buf.String())
	}
}

// ---- 12. HandleStatus NotConnected ログ ----
func TestHandleStatus_LogsNotConnected(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, auth.ErrProviderNotConnected
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth status checked" && entry["outcome"] == "not_connected" {
			found = true
		}
	}
	if !found {
		t.Errorf("log with outcome=not_connected not found; logs=%s", buf.String())
	}
}

// ---- 13. HandleStatus NeedsReauth ログ ----
func TestHandleStatus_LogsNeedsReauth(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, auth.ErrTokenRefreshFailed
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth status checked" && entry["outcome"] == "needs_reauth" {
			found = true
		}
	}
	if !found {
		t.Errorf("log with outcome=needs_reauth not found; logs=%s", buf.String())
	}
}

// ---- 14. HandleStatus InternalError ログ（err.Error() 生値漏れなし） ----
func TestHandleStatus_LogsInternalError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	secretErrMsg := "upstream: access_token=leaky-AAAA code=real-CCCC"
	tm := &fakeTokenManager{
		getFn: func(ctx context.Context, userID, providerName, tenant string) (*auth.TokenRecord, error) {
			return nil, errors.New(secretErrMsg)
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newStatusRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleStatus(rec, req)

	if rec.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusInternalServerError)
	}

	logs := buf.String()
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth status failed" {
			found = true
			if entry["reason"] != "store_error" {
				t.Errorf("log reason = %v, want store_error", entry["reason"])
			}
			if _, ok := entry["err_type"]; !ok {
				t.Errorf("log missing err_type")
			}
		}
	}
	if !found {
		t.Errorf("log 'oauth status failed' not found; logs=%s", buf.String())
	}

	// err.Error() 生値がログに漏れていないこと
	for _, leaky := range []string{"leaky-AAAA", "real-CCCC", "access_token=", secretErrMsg} {
		if strings.Contains(logs, leaky) {
			t.Errorf("logs leak sensitive substring %q; logs=%s", leaky, logs)
		}
	}
}

// ---- 15. HandleDisconnect メソッドチェック ----
func TestHandleDisconnect_MethodNotAllowed(t *testing.T) {
	methods := []string{stdhttp.MethodGet, stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
			req := httptest.NewRequest(method, "/oauth/backlog/disconnect", nil)
			req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
			rec := httptest.NewRecorder()

			h.HandleDisconnect(rec, req)

			if rec.Code != stdhttp.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusMethodNotAllowed)
			}
			var body map[string]string
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if body["error"] != "method_not_allowed" {
				t.Errorf("error = %q, want method_not_allowed", body["error"])
			}
		})
	}
}

// ---- 16. HandleDisconnect 未認証 ----
func TestHandleDisconnect_Unauthenticated(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	req := newDisconnectRequest("")
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "unauthenticated" {
		t.Errorf("error = %q, want unauthenticated", body["error"])
	}
}

// ---- 17. HandleDisconnect 成功 ----
func TestHandleDisconnect_Success(t *testing.T) {
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if body["status"] != "disconnected" {
		t.Errorf("status = %v, want disconnected", body["status"])
	}
	if body["provider"] != "backlog" {
		t.Errorf("provider = %v, want backlog", body["provider"])
	}
	if body["tenant"] != testTenant {
		t.Errorf("tenant = %v, want %q", body["tenant"], testTenant)
	}
}

// ---- 18. HandleDisconnect 冪等性（ErrProviderNotConnected）----
func TestHandleDisconnect_Idempotent_ProviderNotConnected(t *testing.T) {
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return auth.ErrProviderNotConnected
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want %d (idempotent on not-connected)", rec.Code, stdhttp.StatusOK)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "disconnected" {
		t.Errorf("status = %v, want disconnected", body["status"])
	}
}

// ---- 19. HandleDisconnect 内部エラー ----
func TestHandleDisconnect_InternalError(t *testing.T) {
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return errors.New("store down")
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if rec.Code != stdhttp.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusInternalServerError)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "internal_error" {
		t.Errorf("error = %q, want internal_error", body["error"])
	}
}

// ---- 20. HandleDisconnect RevokeToken 引数 ----
func TestHandleDisconnect_PassesCorrectArgsToRevoke(t *testing.T) {
	var gotUserID, gotProvider, gotTenant string
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			gotUserID = userID
			gotProvider = providerName
			gotTenant = tenant
			return nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if gotUserID != testUserID {
		t.Errorf("userID passed = %q, want %q", gotUserID, testUserID)
	}
	if gotProvider != "backlog" {
		t.Errorf("provider passed = %q, want backlog", gotProvider)
	}
	if gotTenant != testTenant {
		t.Errorf("tenant passed = %q, want %q", gotTenant, testTenant)
	}
}

// ---- 21. HandleDisconnect Content-Type ----
func TestHandleDisconnect_ContentTypeJSON(t *testing.T) {
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ---- 22. HandleDisconnect 成功ログ ----
func TestHandleDisconnect_LogsSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return nil
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth disconnect success" {
			found = true
			if entry["user_id"] != testUserID {
				t.Errorf("log user_id = %v, want %q", entry["user_id"], testUserID)
			}
			if entry["provider"] != "backlog" {
				t.Errorf("log provider = %v, want backlog", entry["provider"])
			}
			if entry["tenant"] != testTenant {
				t.Errorf("log tenant = %v, want %q", entry["tenant"], testTenant)
			}
		}
	}
	if !found {
		t.Errorf("log 'oauth disconnect success' not found; logs=%s", buf.String())
	}
}

// ---- 23. HandleDisconnect 失敗ログ ----
func TestHandleDisconnect_LogsFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	secretErrMsg := "upstream: access_token=leaky-BBBB"
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return errors.New(secretErrMsg)
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	if rec.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusInternalServerError)
	}

	logs := buf.String()
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "oauth disconnect failed" {
			found = true
			if entry["reason"] != "revoke_failed" {
				t.Errorf("log reason = %v, want revoke_failed", entry["reason"])
			}
			if _, ok := entry["err_type"]; !ok {
				t.Errorf("log missing err_type")
			}
		}
	}
	if !found {
		t.Errorf("log 'oauth disconnect failed' not found; logs=%s", buf.String())
	}

	for _, leaky := range []string{"leaky-BBBB", "access_token=", secretErrMsg} {
		if strings.Contains(logs, leaky) {
			t.Errorf("logs leak sensitive substring %q; logs=%s", leaky, logs)
		}
	}
}

// ---- 24. HandleDisconnect 成功時トークン漏れなし ----
func TestHandleDisconnect_DoesNotLeakToken(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tm := &fakeTokenManager{
		revokeFn: func(ctx context.Context, userID, providerName, tenant string) error {
			return nil
		},
	}
	h := newTestHandlerWithDeps(t, logger, &fakeProvider{}, tm)
	req := newDisconnectRequest(testUserID)
	rec := httptest.NewRecorder()

	h.HandleDisconnect(rec, req)

	logs := buf.String()
	// Disconnect はトークン値を受け取らないが、念のためログにアクセストークンっぽい単語が入らないことを確認
	for _, leaky := range []string{"access_token", "refresh_token"} {
		if strings.Contains(logs, leaky) {
			t.Errorf("logs contain sensitive key %q; logs=%s", leaky, logs)
		}
	}

	// レスポンスボディにもトークン関連フィールドが含まれないこと
	bodyStr := rec.Body.String()
	for _, leaky := range []string{"access_token", "refresh_token"} {
		if strings.Contains(bodyStr, leaky) {
			t.Errorf("response body contains sensitive key %q; body=%s", leaky, bodyStr)
		}
	}
}

// ---- 25. GetCurrentUser が nil を返した場合の防御 ----
func TestHandleCallback_NilProviderUser_Handled(t *testing.T) {
	fp := &fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			return &auth.TokenRecord{Provider: "backlog", Tenant: testTenant, AccessToken: "a", RefreshToken: "b", Expiry: time.Now().Add(time.Hour)}, nil
		},
		userFn: func(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
			return nil, nil // 違反: nil, nil
		},
	}
	h := newTestHandlerWithDeps(t, slog.New(slog.NewJSONHandler(io.Discard, nil)), fp, &fakeTokenManager{})
	state, _ := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)

	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Errorf("status = %d, want %d (nil ProviderUser should be treated as provider error)", rec.Code, stdhttp.StatusBadGateway)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "provider_error" {
		t.Errorf("error = %q, want provider_error", body["error"])
	}
}
