package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/cli"
)

// fakeTM は計画書記載の TokenManager モック。
type fakeTM struct {
	result *auth.TokenRecord
	err    error
}

func (f *fakeTM) GetValidToken(ctx context.Context, uid, p, t string) (*auth.TokenRecord, error) {
	return f.result, f.err
}

func (f *fakeTM) SaveToken(ctx context.Context, r *auth.TokenRecord) error { return nil }
func (f *fakeTM) RevokeToken(ctx context.Context, uid, p, t string) error  { return nil }

// redirectTestAuthorizeURL は EnsureBacklogConnected テストで使う固定 authorize URL。
const redirectTestAuthorizeURL = "https://mcp.example.com/oauth/backlog/authorize"

// okHandler は常に 200 OK を返すハンドラー。
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// withUserID は context に userID を注入したリクエストを返す。
func withUserID(r *http.Request, uid string) *http.Request {
	return r.WithContext(auth.ContextWithUserID(r.Context(), uid))
}

func TestEnsureBacklogConnected(t *testing.T) {
	connected := &auth.TokenRecord{AccessToken: "tok"}

	tests := []struct {
		name         string
		method       string
		path         string
		accept       string
		userID       string // 空なら context に注入しない
		tmResult     *auth.TokenRecord
		tmErr        error
		wantStatus   int
		wantLocation string // 302 の場合のみ検証
	}{
		{
			name:         "T1: 未接続 GET / text/html → 302",
			method:       http.MethodGet,
			path:         "/",
			accept:       "text/html,application/xhtml+xml",
			userID:       "user1",
			tmErr:        auth.ErrProviderNotConnected,
			wantStatus:   http.StatusFound,
			wantLocation: redirectTestAuthorizeURL,
		},
		{
			name:       "T2: 未接続 POST / text/html → 200 (method 除外)",
			method:     http.MethodPost,
			path:       "/",
			accept:     "text/html",
			userID:     "user1",
			tmErr:      auth.ErrProviderNotConnected,
			wantStatus: http.StatusOK,
		},
		{
			name:       "T3: 未接続 GET / application/json → 200 (Accept 除外)",
			method:     http.MethodGet,
			path:       "/",
			accept:     "application/json",
			userID:     "user1",
			tmErr:      auth.ErrProviderNotConnected,
			wantStatus: http.StatusOK,
		},
		{
			name:       "T4a: /mcp 完全一致 → 200 (MCP 除外)",
			method:     http.MethodGet,
			path:       "/mcp",
			accept:     "text/html",
			userID:     "user1",
			tmErr:      auth.ErrProviderNotConnected,
			wantStatus: http.StatusOK,
		},
		{
			name:         "T4b: /mcphello → 302 (前方一致誤判定なし)",
			method:       http.MethodGet,
			path:         "/mcphello",
			accept:       "text/html",
			userID:       "user1",
			tmErr:        auth.ErrProviderNotConnected,
			wantStatus:   http.StatusFound,
			wantLocation: redirectTestAuthorizeURL,
		},
		{
			name:       "T5: /oauth/backlog/authorize → 200 (無限ループ防止)",
			method:     http.MethodGet,
			path:       "/oauth/backlog/authorize",
			accept:     "text/html",
			userID:     "user1",
			tmErr:      auth.ErrProviderNotConnected,
			wantStatus: http.StatusOK,
		},
		{
			name:       "T6: userID なし → 200 (素通り)",
			method:     http.MethodGet,
			path:       "/",
			accept:     "text/html",
			userID:     "", // 注入しない
			tmErr:      auth.ErrProviderNotConnected,
			wantStatus: http.StatusOK,
		},
		{
			name:       "T7: 接続済み GET / text/html → 200 (通常フロー)",
			method:     http.MethodGet,
			path:       "/",
			accept:     "text/html",
			userID:     "user1",
			tmResult:   connected,
			tmErr:      nil,
			wantStatus: http.StatusOK,
		},
		{
			name:         "T8: ErrTokenRefreshFailed → 302",
			method:       http.MethodGet,
			path:         "/",
			accept:       "text/html",
			userID:       "user1",
			tmErr:        auth.ErrTokenRefreshFailed,
			wantStatus:   http.StatusFound,
			wantLocation: redirectTestAuthorizeURL,
		},
		{
			name:         "T9: HEAD / text/html 未接続 → 302",
			method:       http.MethodHead,
			path:         "/",
			accept:       "text/html",
			userID:       "user1",
			tmErr:        auth.ErrProviderNotConnected,
			wantStatus:   http.StatusFound,
			wantLocation: redirectTestAuthorizeURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := &fakeTM{result: tc.tmResult, err: tc.tmErr}
			mw := cli.EnsureBacklogConnected(tm, "backlog", "test-space", redirectTestAuthorizeURL)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Accept", tc.accept)
			if tc.userID != "" {
				req = withUserID(req, tc.userID)
			}

			rec := httptest.NewRecorder()
			mw(okHandler).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantLocation != "" {
				loc := rec.Header().Get("Location")
				if loc != tc.wantLocation {
					t.Errorf("Location = %q, want %q", loc, tc.wantLocation)
				}
			}
		})
	}
}
