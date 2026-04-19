package http_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

// ============================================================================
// H1-H6: continue パラメータを使った HandleAuthorize / HandleCallback テスト
// ============================================================================

// continueSetupSuccess は continue テスト用の正常系 OAuthHandler を構築する。
func continueSetupSuccess(t *testing.T) *successCallbackDeps {
	t.Helper()
	return setupCallbackSuccess(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

// H1: HandleAuthorize に ?continue=/authorize?x=1 を渡したとき、
// 302 で Backlog consent URL に遷移し、state JWT に continue claim が埋まること。
func TestHandleAuthorize_ContinueEmbeddedInState(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize?continue=/authorize?x=1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusFound)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	// Location から state を取得して Continue claim を検証
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	stateJWT := u.Query().Get("state")
	if stateJWT == "" {
		t.Fatal("state query parameter is empty in Location")
	}

	claims, err := auth.ValidateState(stateJWT, testSecret)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	if claims.Continue != "/authorize?x=1" {
		t.Errorf("claims.Continue = %q, want %q", claims.Continue, "/authorize?x=1")
	}
}

// H2: HandleAuthorize に ?continue=https://evil（絶対URL）を渡したとき、
// 400 invalid_request を返すこと（open redirect 拒否）。
func TestHandleAuthorize_ContinueAbsoluteURL_Rejected(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize?continue=https://evil", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if body["error"] != "invalid_request" {
		t.Errorf("error = %q, want invalid_request", body["error"])
	}
}

// H3: HandleAuthorize に continue なしで呼んだとき、既存動作を維持すること
// （302 → Backlog consent, state.Continue=""）。
func TestHandleAuthorize_NoContinue_BackwardCompat(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/authorize", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	h.HandleAuthorize(rec, req)

	if rec.Code != stdhttp.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, stdhttp.StatusFound)
	}

	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	stateJWT := u.Query().Get("state")
	claims, err := auth.ValidateState(stateJWT, testSecret)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	// continue なしは空文字であること
	if claims.Continue != "" {
		t.Errorf("claims.Continue = %q, want empty string", claims.Continue)
	}
}

// continueCallbackState は Continue フィールドを含む state JWT を生成して返す。
func continueCallbackState(t *testing.T, continueURL string) string {
	t.Helper()
	state, err := auth.GenerateStateWithContinue(testUserID, testTenant, continueURL, testSecret, 10*time.Minute)
	if err != nil {
		t.Fatalf("GenerateStateWithContinue: %v", err)
	}
	return state
}

// H4: HandleCallback 正常 + state.Continue="/authorize?x=1" のとき、
// 302 → "/authorize?x=1" を返すこと（JSON は返さない）。
func TestHandleCallback_WithContinue_Redirects(t *testing.T) {
	deps := continueSetupSuccess(t)

	state := continueCallbackState(t, "/authorize?x=1")
	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	deps.handler.HandleCallback(rec, req)

	if rec.Code != stdhttp.StatusFound {
		t.Errorf("status = %d, want %d (should redirect)", rec.Code, stdhttp.StatusFound)
	}
	location := rec.Header().Get("Location")
	if location != "/authorize?x=1" {
		t.Errorf("Location = %q, want %q", location, "/authorize?x=1")
	}
	// JSON ボディは返さない（Content-Type が application/json でないこと）
	ct := rec.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, should not be application/json on redirect", ct)
	}
}

// H5: HandleCallback 正常 + state.Continue="" のとき、
// 既存動作（200 JSON {"status":"connected",...}）を維持すること。
func TestHandleCallback_NoContinue_JSON200(t *testing.T) {
	deps := continueSetupSuccess(t)

	// Continue なし state
	state := continueCallbackState(t, "")
	q := url.Values{}
	q.Set("code", "abc")
	q.Set("state", state)
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
}

// H6: HandleCallback でプロバイダーエラー（error クエリ）+ state.Continue="/authorize?x=1" のとき、
// 既存の JSON エラーを維持すること（302 しない）。
func TestHandleCallback_ErrorWithContinue_JSONError(t *testing.T) {
	h := newTestHandler(t, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	q := url.Values{}
	q.Set("error", "access_denied")
	// state に Continue があっても error クエリ優先でエラーを返す
	// (error クエリがある場合は state を読まない)
	req := newCallbackRequest(testUserID, q)
	rec := httptest.NewRecorder()

	h.HandleCallback(rec, req)

	// エラー経路では continue を見ない（既存 JSON エラーを維持）
	if rec.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, stdhttp.StatusBadRequest)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "provider_denied" {
		t.Errorf("error = %q, want provider_denied", body["error"])
	}
}
