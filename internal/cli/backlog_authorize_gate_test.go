package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	idproxy "github.com/youyo/idproxy"
	idproxystore "github.com/youyo/idproxy/store"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/cli"
)

// ============================================================================
// G1-G9: BacklogAuthorizeGate テスト
// ============================================================================

// gateTestConfig は BacklogAuthorizeGate テスト共通の idproxy.Config を返す。
// CookieSecret は 32 バイト固定値。
func gateTestConfig(st idproxy.Store) idproxy.Config {
	return idproxy.Config{
		CookieSecret: []byte("12345678901234567890123456789012"), // 32 bytes
		Store:        st,
		ExternalURL:  "http://localhost:8080",
	}
}

// gateTestBacklogAuthorizeURL は gate が redirect 先とする URL。
const gateTestBacklogAuthorizeURL = "http://localhost:8080/oauth/backlog/authorize"

// issueTestSession は SessionManager を使ってテスト用セッションを発行し、
// 返ってきた Cookie を req に設定する。
func issueTestSession(t *testing.T, sm *idproxy.SessionManager, w http.ResponseWriter) string {
	t.Helper()
	user := &idproxy.User{
		Subject: "test-user-subject",
		Email:   "test@example.com",
		Name:    "Test User",
	}
	sess, err := sm.IssueSession(context.Background(), user, "https://issuer.example.com", "id-token")
	if err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	if err := sm.SetCookie(w, sess.ID); err != nil {
		t.Fatalf("SetCookie: %v", err)
	}
	// Set-Cookie ヘッダーから cookie 値を取得
	resp := &http.Response{Header: w.Header()}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("SetCookie did not set any cookie")
	}
	return cookies[0].Value
}

// withCookie は req に Cookie ヘッダーを付与した新しいリクエストを返す。
func withCookie(r *http.Request, name, value string) *http.Request {
	r.AddCookie(&http.Cookie{Name: name, Value: value})
	return r
}

// newGateSetup は BacklogAuthorizeGate をテスト用に構成して返す。
// st は共有ストア、tm は TokenManager モック。
func newGateSetup(t *testing.T, tm auth.TokenManager) (gate func(http.Handler) http.Handler, sm *idproxy.SessionManager) {
	t.Helper()
	st := idproxystore.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	cfg := gateTestConfig(st)
	var err error
	sm, err = idproxy.NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	gate = cli.NewBacklogAuthorizeGate(sm, tm, "backlog", "test-space", gateTestBacklogAuthorizeURL)
	return gate, sm
}

// G1: GET /authorize + セッションあり + Backlog 未接続 → 302 to /oauth/backlog/authorize?continue=%2Fauthorize%3Fclient_id%3Dx
func TestBacklogAuthorizeGate_G1_UnconnectedRedirects(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, sm := newGateSetup(t, tm)

	// セッション発行
	cookieW := httptest.NewRecorder()
	cookieVal := issueTestSession(t, sm, cookieW)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=x", nil)
	req = withCookie(req, "_idproxy_session", cookieVal)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}
	continueParam := parsed.Query().Get("continue")
	if continueParam == "" {
		t.Errorf("continue param missing in Location: %q", loc)
	}
	// continue は /authorize?client_id=x を URL エンコードしたもの
	decoded, err := url.QueryUnescape(continueParam)
	if err != nil {
		t.Fatalf("QueryUnescape: %v", err)
	}
	if decoded != "/authorize?client_id=x" {
		t.Errorf("continue decoded = %q, want %q", decoded, "/authorize?client_id=x")
	}
}

// G2: GET /authorize + セッションあり + Backlog 接続済み → pass-through (next 呼び出し)
func TestBacklogAuthorizeGate_G2_ConnectedPassThrough(t *testing.T) {
	tm := &fakeTM{result: &auth.TokenRecord{AccessToken: "tok"}}
	gate, sm := newGateSetup(t, tm)

	cookieW := httptest.NewRecorder()
	cookieVal := issueTestSession(t, sm, cookieW)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=x", nil)
	req = withCookie(req, "_idproxy_session", cookieVal)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (should pass-through)", rec.Code, http.StatusOK)
	}
}

// G3: GET /authorize + セッションなし → pass-through (idproxy が login redirect)
func TestBacklogAuthorizeGate_G3_NoSessionPassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, _ := newGateSetup(t, tm)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=x", nil)
	// Cookie 付与なし
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (should pass-through)", rec.Code, http.StatusOK)
	}
}

// G4: GET /authorize + Cookie 改ざん → pass-through (idproxy が再ログイン)
func TestBacklogAuthorizeGate_G4_TamperedCookiePassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, _ := newGateSetup(t, tm)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=x", nil)
	// 改ざんされた Cookie
	req.AddCookie(&http.Cookie{Name: "_idproxy_session", Value: "tampered-invalid-value"})
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (should pass-through on invalid cookie)", rec.Code, http.StatusOK)
	}
}

// G5: POST /authorize → pass-through (GET 以外は非対象)
func TestBacklogAuthorizeGate_G5_PostMethodPassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, _ := newGateSetup(t, tm)

	req := httptest.NewRequest(http.MethodPost, "/authorize", nil)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (POST should pass-through)", rec.Code, http.StatusOK)
	}
}

// G6: GET /token → pass-through (パス不一致)
func TestBacklogAuthorizeGate_G6_DifferentPathPassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, _ := newGateSetup(t, tm)

	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (path /token should pass-through)", rec.Code, http.StatusOK)
	}
}

// G7: GET /oauth/backlog/authorize + セッションあり + Backlog 未接続 → pass-through (ループ防止)
func TestBacklogAuthorizeGate_G7_BacklogAuthorizePathPassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, sm := newGateSetup(t, tm)

	cookieW := httptest.NewRecorder()
	cookieVal := issueTestSession(t, sm, cookieW)

	req := httptest.NewRequest(http.MethodGet, "/oauth/backlog/authorize", nil)
	req = withCookie(req, "_idproxy_session", cookieVal)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	// gate は /authorize 完全一致のみを対象にするため、pass-through
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (should pass-through to avoid loop)", rec.Code, http.StatusOK)
	}
}

// G8: GET /mcp → pass-through
func TestBacklogAuthorizeGate_G8_McpPathPassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, _ := newGateSetup(t, tm)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (path /mcp should pass-through)", rec.Code, http.StatusOK)
	}
}

// G_URLEncoding: MCP の実際のリクエスト形式（redirect_uri が %3A%2F%2F でエンコード済み）でも
// continue パラメータが正しく round-trip することを確認する。
// Claude Desktop は redirect_uri=http%3A%2F%2F127.0.0.1%3AXXXX%2Fcb のように URL エンコードして送る。
// gate が r.URL.RequestURI() で取得した文字列を url.QueryEscape すると二重エンコードされるが、
// HandleAuthorize の r.URL.Query().Get("continue") が一層デコードするため最終的に
// ValidateContinueURL に渡る文字列は "/authorize?..." で始まる。
func TestBacklogAuthorizeGate_URLEncodedRedirectURI_ContinueIsValid(t *testing.T) {
	tm := &fakeTM{err: auth.ErrProviderNotConnected}
	gate, sm := newGateSetup(t, tm)

	cookieW := httptest.NewRecorder()
	cookieVal := issueTestSession(t, sm, cookieW)

	// Claude Desktop が送る実際の形式: redirect_uri を %3A%2F%2F でエンコード済み
	req := httptest.NewRequest(http.MethodGet,
		"/authorize?client_id=xxx&redirect_uri=http%3A%2F%2F127.0.0.1%3A1234%2Fcb&state=S1",
		nil,
	)
	req = withCookie(req, "_idproxy_session", cookieVal)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	loc := rec.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location parse: %v", err)
	}

	continueEncoded := parsed.Query().Get("continue")
	if continueEncoded == "" {
		t.Fatal("continue param missing in Location")
	}

	// continue を1層デコードすると /authorize?... で始まるはず
	continueDecoded, err := url.QueryUnescape(continueEncoded)
	if err != nil {
		t.Fatalf("QueryUnescape: %v", err)
	}
	if !startsWith(continueDecoded, "/authorize?") {
		t.Errorf("continue decoded = %q, want prefix /authorize?", continueDecoded)
	}

	// ValidateContinueURL で受け入れられることを確認（HandleAuthorize が同様に判定する）
	if err := auth.ValidateContinueURL(continueDecoded); err != nil {
		t.Errorf("ValidateContinueURL(%q) = %v, want nil", continueDecoded, err)
	}
}

// startsWith は strings.HasPrefix の代替（テスト専用ヘルパー）。
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// G9: GET /authorize + Backlog 状態が ErrUnauthenticated → pass-through
// (予期しないエラーは redirect しない、needsBacklogAuthorization allowlist と整合)
func TestBacklogAuthorizeGate_G9_UnexpectedErrorPassThrough(t *testing.T) {
	tm := &fakeTM{err: auth.ErrUnauthenticated}
	gate, sm := newGateSetup(t, tm)

	cookieW := httptest.NewRecorder()
	cookieVal := issueTestSession(t, sm, cookieW)

	req := httptest.NewRequest(http.MethodGet, "/authorize", nil)
	req = withCookie(req, "_idproxy_session", cookieVal)
	rec := httptest.NewRecorder()

	gate(okHandler).ServeHTTP(rec, req)

	// ErrUnauthenticated は needsBacklogAuthorization で false → pass-through
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (ErrUnauthenticated should pass-through)", rec.Code, http.StatusOK)
	}
}
