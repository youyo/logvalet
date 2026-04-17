//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/auth/provider"
	tokenstore "github.com/youyo/logvalet/internal/auth/tokenstore"
	"github.com/youyo/logvalet/internal/cli"
	httptransport "github.com/youyo/logvalet/internal/transport/http"
)

// ---------------------------------------------------------------------------
// Backlog モックサーバー（OAuth token endpoint / users/myself / space を実装）
// ---------------------------------------------------------------------------

// userFixture はテスト用ユーザー情報。
// 発行する access_token / refresh_token / provider_user_id を固定化して
// どのユーザーの API コールか識別できるようにする。
type userFixture struct {
	AccessToken    string
	RefreshToken   string
	ProviderUserID int    // /api/v2/users/myself の `id`
	UserID         string // /api/v2/users/myself の `userId`（email）
	Name           string // /api/v2/users/myself の `name`
}

// backlogMock は Backlog API の最小モック実装。
// トークン交換、ユーザー取得、スペース情報取得を提供する。
// observedBearers にはリクエスト毎の Authorization ヘッダーを記録する。
type backlogMock struct {
	srv             *httptest.Server
	mu              sync.Mutex
	observedBearers []string

	// code → user fixture
	codeToUser map[string]userFixture
	// access_token → user fixture
	tokenToUser map[string]userFixture
}

func newBacklogMock(t *testing.T) *backlogMock {
	t.Helper()
	b := &backlogMock{
		codeToUser:  make(map[string]userFixture),
		tokenToUser: make(map[string]userFixture),
	}
	b.srv = httptest.NewServer(http.HandlerFunc(b.handle))
	t.Cleanup(b.srv.Close)
	return b
}

// addUser はテスト用ユーザーを登録する。code → user, access_token → user の両方に記録。
func (b *backlogMock) addUser(code string, u userFixture) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.codeToUser[code] = u
	b.tokenToUser[u.AccessToken] = u
}

// observe は Authorization ヘッダーを記録する。
func (b *backlogMock) observe(authHeader string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.observedBearers = append(b.observedBearers, authHeader)
}

// snapshotObserved は観測したヘッダーのコピーを返す。
func (b *backlogMock) snapshotObserved() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.observedBearers))
	copy(out, b.observedBearers)
	return out
}

func (b *backlogMock) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v2/oauth2/token":
		b.handleToken(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v2/users/myself":
		b.observe(r.Header.Get("Authorization"))
		b.handleUsersMyself(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v2/space":
		b.observe(r.Header.Get("Authorization"))
		b.handleSpace(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (b *backlogMock) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	grant := r.FormValue("grant_type")

	var user userFixture
	var ok bool
	switch grant {
	case "authorization_code":
		code := r.FormValue("code")
		b.mu.Lock()
		user, ok = b.codeToUser[code]
		b.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
	case "refresh_token":
		refresh := r.FormValue("refresh_token")
		b.mu.Lock()
		for _, u := range b.codeToUser {
			if u.RefreshToken == refresh {
				user = u
				ok = true
				break
			}
		}
		b.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  user.AccessToken,
		"refresh_token": user.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
	})
}

func (b *backlogMock) handleUsersMyself(w http.ResponseWriter, r *http.Request) {
	authz := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authz, "Bearer ")
	b.mu.Lock()
	user, ok := b.tokenToUser[token]
	b.mu.Unlock()
	if !ok {
		http.Error(w, `{"errors":[{"message":"invalid token"}]}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          user.ProviderUserID,
		"userId":      user.UserID,
		"name":        user.Name,
		"mailAddress": user.UserID,
	})
}

func (b *backlogMock) handleSpace(w http.ResponseWriter, r *http.Request) {
	authz := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authz, "Bearer ")
	b.mu.Lock()
	_, ok := b.tokenToUser[token]
	b.mu.Unlock()
	if !ok {
		http.Error(w, `{"errors":[{"message":"invalid token"}]}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"spaceKey":    "test-space",
		"name":        "Test Space",
		"ownerId":     1,
		"language":    "ja",
		"timezone":    "Asia/Tokyo",
		"reportSendTime": "08:00:00",
		"textFormattingRule": "markdown",
		"created":     "2026-01-01T00:00:00Z",
		"updated":     "2026-01-01T00:00:00Z",
	})
}

// ---------------------------------------------------------------------------
// テスト用 userID 注入ミドルウェア
// ---------------------------------------------------------------------------

// testUserIDMiddleware は X-Test-User-ID ヘッダーを auth.ContextWithUserID に注入する。
// 本番 newUserIDBridge の意味論的等価物（idproxy.User.Subject の代わりに HTTP ヘッダーを使う）。
func testUserIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid := r.Header.Get("X-Test-User-ID"); uid != "" {
			r = r.WithContext(auth.ContextWithUserID(r.Context(), uid))
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// E2E テストハーネス
// ---------------------------------------------------------------------------

type oauthE2EHarness struct {
	backlogMock *backlogMock
	authSrv     *httptest.Server
	store       auth.TokenStore
	tm          auth.TokenManager
	factory     auth.ClientFactory
	redirectURI string
	tenant      string
}

func newOAuthE2EHarness(t *testing.T) *oauthE2EHarness {
	t.Helper()

	bl := newBacklogMock(t)

	p, err := provider.NewBacklogOAuthProvider(
		"test-space",
		"test-client-id",
		"test-client-secret",
		provider.WithBaseURL(bl.srv.URL),
	)
	if err != nil {
		t.Fatalf("NewBacklogOAuthProvider: %v", err)
	}

	store := tokenstore.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	providers := map[string]auth.TokenRefresher{p.Name(): p}
	tm := auth.NewTokenManager(store, providers)
	factory := auth.NewClientFactory(tm, p.Name(), "test-space", bl.srv.URL)

	// state secret はテスト用ダミー（本番は openssl rand -hex 32 など）
	stateSecret := bytes.Repeat([]byte{0xAB}, 32)

	// httptest.NewServer を dummy handler で先に起動し URL を確定させる
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(authSrv.Close)

	redirectURI := authSrv.URL + "/oauth/backlog/callback"

	authorizeURL := authSrv.URL + "/oauth/backlog/authorize"

	h, err := httptransport.NewOAuthHandler(p, tm, "test-space", redirectURI, authorizeURL, stateSecret, time.Minute, slog.Default())
	if err != nil {
		t.Fatalf("NewOAuthHandler: %v", err)
	}

	innerMux := http.NewServeMux()
	cli.InstallOAuthRoutes(innerMux, h)

	// EnsureBacklogConnected: userID bridge よりも内側に置く（条件 3: UserIDFromContext が先に必要）
	ensuredInner := cli.EnsureBacklogConnected(
		tm,
		"backlog",
		"test-space",
		authorizeURL,
	)(innerMux)

	topMux := http.NewServeMux()
	topMux.HandleFunc("/healthz", cli.HealthHandler)
	topMux.Handle("/", testUserIDMiddleware(ensuredInner))

	authSrv.Config.Handler = topMux

	return &oauthE2EHarness{
		backlogMock: bl,
		authSrv:     authSrv,
		store:       store,
		tm:          tm,
		factory:     factory,
		redirectURI: redirectURI,
		tenant:      "test-space",
	}
}

// noRedirectClient はリダイレクトを追跡しない HTTP クライアント。
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// doAuthorize は /oauth/backlog/authorize を叩き、state を取り出す。
// リダイレクト先が Backlog 認可 URL（test-space.backlog.com）であることを確認する。
func (h *oauthE2EHarness) doAuthorize(t *testing.T, userID string) string {
	t.Helper()
	client := noRedirectClient()
	req, _ := http.NewRequest(http.MethodGet, h.authSrv.URL+"/oauth/backlog/authorize", nil)
	req.Header.Set("X-Test-User-ID", userID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize status = %d, want 302", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if !strings.Contains(loc.Host, "test-space.backlog.com") {
		t.Errorf("authorize redirected to %q, want test-space.backlog.com host", loc.Host)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize redirect")
	}
	return state
}

// doCallback は /oauth/backlog/callback を叩き、レスポンス JSON を返す。
func (h *oauthE2EHarness) doCallback(t *testing.T, userID, code, state string) (int, map[string]any) {
	t.Helper()
	client := noRedirectClient()
	q := url.Values{}
	q.Set("code", code)
	q.Set("state", state)
	req, _ := http.NewRequest(http.MethodGet, h.authSrv.URL+"/oauth/backlog/callback?"+q.Encode(), nil)
	req.Header.Set("X-Test-User-ID", userID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &decoded)
	}
	return resp.StatusCode, decoded
}

// doStatus は /oauth/backlog/status を叩く。
func (h *oauthE2EHarness) doStatus(t *testing.T, userID string) (int, map[string]any) {
	t.Helper()
	client := noRedirectClient()
	req, _ := http.NewRequest(http.MethodGet, h.authSrv.URL+"/oauth/backlog/status", nil)
	req.Header.Set("X-Test-User-ID", userID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &decoded)
	}
	return resp.StatusCode, decoded
}

// doDisconnect は /oauth/backlog/disconnect を叩く。
func (h *oauthE2EHarness) doDisconnect(t *testing.T, userID string) (int, map[string]any) {
	t.Helper()
	client := noRedirectClient()
	req, _ := http.NewRequest(http.MethodDelete, h.authSrv.URL+"/oauth/backlog/disconnect", nil)
	req.Header.Set("X-Test-User-ID", userID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &decoded)
	}
	return resp.StatusCode, decoded
}

// ---------------------------------------------------------------------------
// 個別テストケース
// ---------------------------------------------------------------------------

func TestOAuthE2E_SingleUser_FullFlow(t *testing.T) {
	h := newOAuthE2EHarness(t)

	alice := userFixture{
		AccessToken:    "alice-access-token-001",
		RefreshToken:   "alice-refresh-001",
		ProviderUserID: 100,
		UserID:         "alice@example.com",
		Name:           "Alice",
	}
	h.backlogMock.addUser("alice-code-001", alice)

	// 1. authorize → state 取得
	state := h.doAuthorize(t, "alice-subject")

	// 2. callback → token 保存 + 成功 JSON
	status, body := h.doCallback(t, "alice-subject", "alice-code-001", state)
	if status != http.StatusOK {
		t.Fatalf("callback status = %d, body = %v", status, body)
	}
	if body["status"] != "connected" {
		t.Errorf("callback status field = %v, want connected", body["status"])
	}
	if body["provider_user_id"] != "100" {
		t.Errorf("provider_user_id = %v, want \"100\"", body["provider_user_id"])
	}
	if body["tenant"] != h.tenant {
		t.Errorf("tenant = %v, want %q", body["tenant"], h.tenant)
	}

	// 3. status → connected
	status, body = h.doStatus(t, "alice-subject")
	if status != http.StatusOK {
		t.Fatalf("status code = %d, body = %v", status, body)
	}
	if body["connected"] != true {
		t.Errorf("connected = %v, want true", body["connected"])
	}
	if body["provider_user_id"] != "100" {
		t.Errorf("provider_user_id = %v, want \"100\"", body["provider_user_id"])
	}

	// 4. disconnect
	status, body = h.doDisconnect(t, "alice-subject")
	if status != http.StatusOK {
		t.Fatalf("disconnect status = %d, body = %v", status, body)
	}
	if body["status"] != "disconnected" {
		t.Errorf("disconnect status = %v, want disconnected", body["status"])
	}

	// 5. status → not_connected
	status, body = h.doStatus(t, "alice-subject")
	if status != http.StatusOK {
		t.Fatalf("status code = %d, body = %v", status, body)
	}
	if body["connected"] != false {
		t.Errorf("connected = %v, want false", body["connected"])
	}
}

func TestOAuthE2E_Callback_InvalidState(t *testing.T) {
	h := newOAuthE2EHarness(t)
	alice := userFixture{
		AccessToken:    "alice-access-token-001",
		RefreshToken:   "alice-refresh-001",
		ProviderUserID: 100,
		UserID:         "alice@example.com",
		Name:           "Alice",
	}
	h.backlogMock.addUser("alice-code-001", alice)

	// state を適当な値で与える（署名無効）
	status, body := h.doCallback(t, "alice-subject", "alice-code-001", "not-a-valid-jwt")
	if status != http.StatusBadRequest {
		t.Fatalf("callback status = %d, body = %v", status, body)
	}
	if body["error"] != "state_invalid" {
		t.Errorf("error = %v, want state_invalid", body["error"])
	}
}

func TestOAuthE2E_Status_NotConnected_InitialState(t *testing.T) {
	h := newOAuthE2EHarness(t)

	status, body := h.doStatus(t, "unknown-user")
	if status != http.StatusOK {
		t.Fatalf("status code = %d, body = %v", status, body)
	}
	if body["connected"] != false {
		t.Errorf("connected = %v, want false", body["connected"])
	}
}

func TestOAuthE2E_TwoUsers_TokenIsolation(t *testing.T) {
	h := newOAuthE2EHarness(t)

	alice := userFixture{
		AccessToken:    "alice-access-token-001",
		RefreshToken:   "alice-refresh-001",
		ProviderUserID: 100,
		UserID:         "alice@example.com",
		Name:           "Alice",
	}
	bob := userFixture{
		AccessToken:    "bob-access-token-001",
		RefreshToken:   "bob-refresh-001",
		ProviderUserID: 200,
		UserID:         "bob@example.com",
		Name:           "Bob",
	}
	h.backlogMock.addUser("alice-code-001", alice)
	h.backlogMock.addUser("bob-code-001", bob)

	// 両ユーザーの OAuth フローを完了
	aliceState := h.doAuthorize(t, "alice-subject")
	_, aliceBody := h.doCallback(t, "alice-subject", "alice-code-001", aliceState)
	if aliceBody["provider_user_id"] != "100" {
		t.Fatalf("alice callback provider_user_id = %v, want 100", aliceBody["provider_user_id"])
	}

	bobState := h.doAuthorize(t, "bob-subject")
	_, bobBody := h.doCallback(t, "bob-subject", "bob-code-001", bobState)
	if bobBody["provider_user_id"] != "200" {
		t.Fatalf("bob callback provider_user_id = %v, want 200", bobBody["provider_user_id"])
	}

	// Load-bearing assertion:
	// ClientFactory を alice / bob の context で呼び、生成された client が
	// 対応するユーザーの access_token で Backlog API を叩くことを確認する。
	aliceCtx := auth.ContextWithUserID(context.Background(), "alice-subject")
	aliceClient, err := h.factory(aliceCtx)
	if err != nil {
		t.Fatalf("alice factory: %v", err)
	}
	if _, err := aliceClient.GetSpace(aliceCtx); err != nil {
		t.Fatalf("alice GetSpace: %v", err)
	}

	bobCtx := auth.ContextWithUserID(context.Background(), "bob-subject")
	bobClient, err := h.factory(bobCtx)
	if err != nil {
		t.Fatalf("bob factory: %v", err)
	}
	if _, err := bobClient.GetSpace(bobCtx); err != nil {
		t.Fatalf("bob GetSpace: %v", err)
	}

	// Backlog モックが観測した Authorization ヘッダーを確認
	// （callback フローで users/myself 分も記録されている点に注意）
	observed := h.backlogMock.snapshotObserved()
	var aliceCount, bobCount, otherCount int
	for _, a := range observed {
		switch a {
		case "Bearer " + alice.AccessToken:
			aliceCount++
		case "Bearer " + bob.AccessToken:
			bobCount++
		default:
			otherCount++
			t.Errorf("unexpected Authorization header observed: %q", a)
		}
	}
	// alice: callback の users/myself (1) + factory の GetSpace (1) = 2
	if aliceCount != 2 {
		t.Errorf("alice bearer count = %d, want 2 (observed=%v)", aliceCount, observed)
	}
	// bob: callback の users/myself (1) + factory の GetSpace (1) = 2
	if bobCount != 2 {
		t.Errorf("bob bearer count = %d, want 2 (observed=%v)", bobCount, observed)
	}
	if otherCount != 0 {
		t.Errorf("unexpected bearers: %d (observed=%v)", otherCount, observed)
	}
}

func TestOAuthE2E_TwoUsers_DisconnectDoesNotAffectOtherUser(t *testing.T) {
	h := newOAuthE2EHarness(t)

	alice := userFixture{
		AccessToken:    "alice-access-token-001",
		RefreshToken:   "alice-refresh-001",
		ProviderUserID: 100,
		UserID:         "alice@example.com",
		Name:           "Alice",
	}
	bob := userFixture{
		AccessToken:    "bob-access-token-001",
		RefreshToken:   "bob-refresh-001",
		ProviderUserID: 200,
		UserID:         "bob@example.com",
		Name:           "Bob",
	}
	h.backlogMock.addUser("alice-code-001", alice)
	h.backlogMock.addUser("bob-code-001", bob)

	// 両ユーザー接続完了
	aliceState := h.doAuthorize(t, "alice-subject")
	if _, body := h.doCallback(t, "alice-subject", "alice-code-001", aliceState); body["status"] != "connected" {
		t.Fatalf("alice callback failed: %v", body)
	}
	bobState := h.doAuthorize(t, "bob-subject")
	if _, body := h.doCallback(t, "bob-subject", "bob-code-001", bobState); body["status"] != "connected" {
		t.Fatalf("bob callback failed: %v", body)
	}

	// bob を disconnect
	status, body := h.doDisconnect(t, "bob-subject")
	if status != http.StatusOK {
		t.Fatalf("bob disconnect status = %d, body = %v", status, body)
	}

	// alice は依然 connected
	status, body = h.doStatus(t, "alice-subject")
	if status != http.StatusOK || body["connected"] != true {
		t.Errorf("alice status after bob disconnect = %d / %v, want 200 / connected=true", status, body)
	}
	if body["provider_user_id"] != "100" {
		t.Errorf("alice provider_user_id = %v, want 100", body["provider_user_id"])
	}

	// bob は not_connected
	status, body = h.doStatus(t, "bob-subject")
	if status != http.StatusOK || body["connected"] != false {
		t.Errorf("bob status after disconnect = %d / %v, want 200 / connected=false", status, body)
	}
}

// TestE2E_BrowserRedirectsToAuthorizeWhenDisconnected は EnsureBacklogConnected ミドルウェア経由で
// ログイン済みかつ Backlog 未接続のユーザーが GET / (Accept: text/html) を叩いた際に
// 302 /oauth/backlog/authorize にリダイレクトされることを検証する（Proposal A 相当）。
func TestE2E_BrowserRedirectsToAuthorizeWhenDisconnected(t *testing.T) {
	h := newOAuthE2EHarness(t)

	// OAuth フローを完了していない（= Backlog 未接続）ユーザーで GET / をブラウザアクセスに見立てる
	client := noRedirectClient()
	req, _ := http.NewRequest(http.MethodGet, h.authSrv.URL+"/", nil)
	req.Header.Set("X-Test-User-ID", "unconnected-user")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/oauth/backlog/authorize") {
		t.Errorf("Location = %q, want to contain /oauth/backlog/authorize", loc)
	}
}

func TestOAuthE2E_ClientFactory_UsesPerUserToken(t *testing.T) {
	h := newOAuthE2EHarness(t)

	alice := userFixture{
		AccessToken:    "alice-access-token-001",
		RefreshToken:   "alice-refresh-001",
		ProviderUserID: 100,
		UserID:         "alice@example.com",
		Name:           "Alice",
	}
	h.backlogMock.addUser("alice-code-001", alice)

	// OAuth フロー完了
	state := h.doAuthorize(t, "alice-subject")
	if _, body := h.doCallback(t, "alice-subject", "alice-code-001", state); body["status"] != "connected" {
		t.Fatalf("callback failed: %v", body)
	}

	// ClientFactory で client を取得し、Backlog API を叩く
	ctx := auth.ContextWithUserID(context.Background(), "alice-subject")
	client, err := h.factory(ctx)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}

	space, err := client.GetSpace(ctx)
	if err != nil {
		t.Fatalf("GetSpace: %v", err)
	}
	if space == nil {
		t.Fatal("space is nil")
	}

	// 観測した Authorization ヘッダーに alice の access_token が含まれていること
	observed := h.backlogMock.snapshotObserved()
	found := false
	for _, a := range observed {
		if a == "Bearer "+alice.AccessToken {
			found = true
		} else if strings.HasPrefix(a, "Bearer ") && a != "Bearer "+alice.AccessToken {
			t.Errorf("unexpected bearer observed: %q", a)
		}
	}
	if !found {
		t.Errorf("alice access_token not observed in Authorization headers: %v", observed)
	}

	// Factory が返したクライアントが userID 未設定 context で呼ばれた場合はエラー
	unauthCtx := context.Background()
	if _, err := h.factory(unauthCtx); err == nil {
		t.Error("factory with no userID should return error")
	}
}
