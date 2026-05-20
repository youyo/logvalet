package http_test

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	httptransport "github.com/youyo/logvalet/internal/transport/http"
	"github.com/youyo/logvalet/internal/space"
)

// ============================================================================
// モック: fakeNonceStore (space.NonceStore)
// ============================================================================

type fakeNonceStore struct {
	storeFn   func(ctx context.Context, userID, nonce string, ttl time.Duration) error
	consumeFn func(ctx context.Context, userID, nonce string) error
}

func (f *fakeNonceStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error {
	if f.storeFn != nil {
		return f.storeFn(ctx, userID, nonce, ttl)
	}
	return nil
}

func (f *fakeNonceStore) Consume(ctx context.Context, userID, nonce string) error {
	if f.consumeFn != nil {
		return f.consumeFn(ctx, userID, nonce)
	}
	return nil
}

var _ space.NonceStore = (*fakeNonceStore)(nil)

// ============================================================================
// モック: fakeSpaceStore (space.Store)
// ============================================================================

type fakeSpaceStore struct {
	upsertFn       func(ctx context.Context, reg *space.SpaceRegistration) error
	getPrefFn      func(ctx context.Context, userID string) (*space.UserPreference, error)
	putPrefFn      func(ctx context.Context, pref *space.UserPreference) error
}

func (f *fakeSpaceStore) List(ctx context.Context, userID string) ([]space.SpaceRegistration, error) {
	return nil, nil
}
func (f *fakeSpaceStore) Get(ctx context.Context, userID, alias string) (*space.SpaceRegistration, error) {
	return nil, nil
}
func (f *fakeSpaceStore) Upsert(ctx context.Context, reg *space.SpaceRegistration) error {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, reg)
	}
	return nil
}
func (f *fakeSpaceStore) Delete(ctx context.Context, userID, alias string) error { return nil }
func (f *fakeSpaceStore) GetPreference(ctx context.Context, userID string) (*space.UserPreference, error) {
	if f.getPrefFn != nil {
		return f.getPrefFn(ctx, userID)
	}
	return nil, nil
}
func (f *fakeSpaceStore) PutPreference(ctx context.Context, pref *space.UserPreference) error {
	if f.putPrefFn != nil {
		return f.putPrefFn(ctx, pref)
	}
	return nil
}
func (f *fakeSpaceStore) Close() error { return nil }

var _ space.Store = (*fakeSpaceStore)(nil)

// ============================================================================
// ヘルパー
// ============================================================================

// testBootstrapSecret はテスト用 bootstrap 鍵素材。
var testBootstrapSecret = "0102030405060708090a0b0c0d0e0f10"

// testBootstrapKey はテスト用 HKDF 派生済み bootstrap 鍵。
var testBootstrapKey = func() []byte {
	k, err := auth.DeriveBootstrapKey(testBootstrapSecret)
	if err != nil {
		panic("DeriveBootstrapKey failed: " + err.Error())
	}
	return k
}()

// buildMultiSpaceHandler はテスト用の MultiSpaceOAuthHandler を構築する（bootstrapKey=nil）。
func buildMultiSpaceHandler(
	p fakeProvider,
	tm fakeTokenManager,
	nonceStore space.NonceStore,
	spaceStore space.Store,
) (*httptransport.MultiSpaceOAuthHandler, error) {
	return httptransport.NewMultiSpaceOAuthHandler(
		&p,
		&tm,
		nonceStore,
		spaceStore,
		testRedirectURI,
		testSecret,
		testTTL,
		nil,
		nil, // bootstrapKey=nil (callback テストでは不要)
	)
}

// buildMultiSpaceHandlerWithKey はテスト用の MultiSpaceOAuthHandler を bootstrapKey 付きで構築する。
func buildMultiSpaceHandlerWithKey(
	p fakeProvider,
	tm fakeTokenManager,
	nonceStore space.NonceStore,
	spaceStore space.Store,
) (*httptransport.MultiSpaceOAuthHandler, error) {
	return httptransport.NewMultiSpaceOAuthHandler(
		&p,
		&tm,
		nonceStore,
		spaceStore,
		testRedirectURI,
		testSecret,
		testTTL,
		nil,
		testBootstrapKey,
	)
}

// makeBootstrapToken はテスト用 bootstrap_token を生成し、NonceStore に Store する。
func makeBootstrapToken(t *testing.T, ns space.NonceStore, baseURL, alias string) string {
	t.Helper()
	jti := "test-jti-" + alias + "-" + baseURL
	tok, err := auth.GenerateBootstrapToken(testUserID, baseURL, alias, testBootstrapKey, auth.DefaultBootstrapTokenTTL, jti)
	if err != nil {
		t.Fatalf("GenerateBootstrapToken: %v", err)
	}
	// トークン検証で jti を確認
	userID, jti, err := auth.ValidateBootstrapToken(tok, baseURL, alias, testBootstrapKey)
	if err != nil {
		t.Fatalf("ValidateBootstrapToken in makeBootstrapToken: %v", err)
	}
	if err := ns.Store(context.Background(), userID, "bs:"+jti, auth.DefaultBootstrapTokenTTL); err != nil {
		t.Fatalf("NonceStore.Store: %v", err)
	}
	return tok
}

// defaultExchangeFn は正常系の ExchangeCode モック。
func defaultExchangeFn(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
	return &auth.TokenRecord{
		Provider:    "backlog",
		AccessToken: "test-access-token",
	}, nil
}

// defaultUserFn は正常系の GetCurrentUser モック。
func defaultUserFn(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
	return &auth.ProviderUser{ID: "provider-user-1", Name: "Test User"}, nil
}

// ============================================================================
// T3: Authorize 正常系
// ============================================================================

// T3: HandleAuthorize 正常系
// - GET /oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo&bootstrap_token=...
// - 302 → Backlog OAuth URL へリダイレクト
// - state JWT に BaseURL/Alias が含まれる
// - nonce が NonceStore に保存される
func TestMultiSpaceOAuthHandler_HandleAuthorize_Success(t *testing.T) {
	var storedNonce string
	var storedUserID string
	var consumedKey string

	nonceStore := &fakeNonceStore{
		storeFn: func(ctx context.Context, userID, nonce string, ttl time.Duration) error {
			storedNonce = nonce
			storedUserID = userID
			return nil
		},
		consumeFn: func(ctx context.Context, userID, nonce string) error {
			consumedKey = nonce
			return nil
		},
	}

	h, err := buildMultiSpaceHandlerWithKey(
		fakeProvider{},
		fakeTokenManager{},
		nonceStore,
		&fakeSpaceStore{},
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	const baseURL = "https://foo.backlog.com"
	const alias = "foo"
	btok := makeBootstrapToken(t, nonceStore, baseURL, alias)

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/authorize?base_url="+baseURL+"&alias="+alias+"&bootstrap_token="+btok, nil)
	w := httptest.NewRecorder()

	h.HandleAuthorize(w, req)

	resp := w.Result()
	if resp.StatusCode != stdhttp.StatusFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, stdhttp.StatusFound)
	}
	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}
	if !strings.Contains(location, "backlog.com") {
		t.Errorf("Location %q does not contain backlog.com", location)
	}

	if storedNonce == "" {
		t.Error("nonce was not stored in NonceStore")
	}
	if storedUserID != testUserID {
		t.Errorf("storedUserID = %q, want %q", storedUserID, testUserID)
	}
	if !strings.HasPrefix(consumedKey, "bs:") {
		t.Errorf("consumedKey = %q, want bs:... prefix (jti consumed)", consumedKey)
	}

	// state JWT に state= が含まれること
	if !strings.Contains(location, "state=") {
		t.Error("state parameter not found in redirect URL")
	}
	// Referrer-Policy ヘッダが設定されること
	if resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", resp.Header.Get("Referrer-Policy"))
	}
}

// ============================================================================
// T4: Callback 正常系
// ============================================================================

// T4: HandleCallback 正常系
// - nonce 消費 → TokenStore.Put → SpaceRegistry.Upsert の順で実行
// - SpaceRegistry に {userID/foo} が登録される
// - レスポンス 200 JSON
func TestMultiSpaceOAuthHandler_HandleCallback_Success(t *testing.T) {
	var callOrder []string
	var upsertedReg *space.SpaceRegistration

	// state を事前生成
	state, err := auth.GenerateStateWithSpaceInfo(
		testUserID, "foo", "https://foo.backlog.com", "foo",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}

	// nonce を state から取得して NonceStore に保存（Authorize フロー相当）
	claims, err := auth.ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v", err)
	}
	nonce := claims.Nonce

	nonceStore := space.NewMemoryStore()
	if err := nonceStore.Store(context.Background(), testUserID, nonce, testTTL); err != nil {
		t.Fatalf("NonceStore.Store() error = %v", err)
	}

	spaceStore := &fakeSpaceStore{
		upsertFn: func(ctx context.Context, reg *space.SpaceRegistration) error {
			callOrder = append(callOrder, "upsert")
			upsertedReg = reg
			return nil
		},
	}

	tm := &fakeTokenManager{
		saveFn: func(ctx context.Context, record *auth.TokenRecord) error {
			callOrder = append(callOrder, "save")
			return nil
		},
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{
			exchangeFn: defaultExchangeFn,
			userFn:     defaultUserFn,
		},
		*tm,
		nonceStore,
		spaceStore,
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+state, nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	resp := w.Result()
	if resp.StatusCode != stdhttp.StatusOK {
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		t.Errorf("status = %d, want 200; body = %s", resp.StatusCode, body[:n])
	}

	// 書き込み順序チェック (C2): save → upsert
	if len(callOrder) < 2 || callOrder[0] != "save" || callOrder[1] != "upsert" {
		t.Errorf("call order = %v, want [save, upsert]", callOrder)
	}

	if upsertedReg == nil {
		t.Fatal("SpaceRegistry.Upsert was not called")
	}
	if upsertedReg.UserID != testUserID {
		t.Errorf("upsertedReg.UserID = %q, want %q", upsertedReg.UserID, testUserID)
	}
	if upsertedReg.Alias != "foo" {
		t.Errorf("upsertedReg.Alias = %q, want %q", upsertedReg.Alias, "foo")
	}
}

// ============================================================================
// T5: Callback - state 改ざん
// ============================================================================

// T5: HandleCallback - JWT 署名改ざん → 400 state_invalid
func TestMultiSpaceOAuthHandler_HandleCallback_StateTampering(t *testing.T) {
	state, err := auth.GenerateStateWithSpaceInfo(
		testUserID, "foo", "https://foo.backlog.com", "foo",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}
	// 署名部分を改ざん
	parts := strings.Split(state, ".")
	parts[2] = "invalidsignature"
	tamperedState := strings.Join(parts, ".")

	h, err := buildMultiSpaceHandler(
		fakeProvider{},
		fakeTokenManager{},
		&fakeNonceStore{},
		&fakeSpaceStore{},
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+tamperedState, nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	resp := w.Result()
	if resp.StatusCode != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "state_invalid" {
		t.Errorf("error = %q, want state_invalid", body["error"])
	}
}

// ============================================================================
// T6: Callback - userID mismatch
// ============================================================================

// T6: HandleCallback - state の uid="u1"、ctx の userID="u2" → 401 user_mismatch
func TestMultiSpaceOAuthHandler_HandleCallback_UserMismatch(t *testing.T) {
	state, err := auth.GenerateStateWithSpaceInfo(
		"user-state", "foo", "https://foo.backlog.com", "foo",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{},
		fakeTokenManager{},
		&fakeNonceStore{},
		&fakeSpaceStore{},
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+state, nil)
	// ctx の userID を異なる値に設定
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "other-user"))
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	resp := w.Result()
	if resp.StatusCode != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "user_mismatch" {
		t.Errorf("error = %q, want user_mismatch", body["error"])
	}
}

// ============================================================================
// T7: Callback - nonce replay
// ============================================================================

// T7: HandleCallback - 同じ state で2回 callback → 2回目は 400 nonce_already_used (C3)
func TestMultiSpaceOAuthHandler_HandleCallback_NonceReplay(t *testing.T) {
	state, err := auth.GenerateStateWithSpaceInfo(
		testUserID, "foo", "https://foo.backlog.com", "foo",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}

	claims, err := auth.ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v", err)
	}
	nonce := claims.Nonce

	nonceStore := space.NewMemoryStore()
	if err := nonceStore.Store(context.Background(), testUserID, nonce, testTTL); err != nil {
		t.Fatalf("NonceStore.Store() error = %v", err)
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{
			exchangeFn: defaultExchangeFn,
			userFn:     defaultUserFn,
		},
		fakeTokenManager{},
		nonceStore,
		&fakeSpaceStore{},
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	// 1回目: 成功
	req1 := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+state, nil)
	req1 = req1.WithContext(auth.ContextWithUserID(req1.Context(), testUserID))
	w1 := httptest.NewRecorder()
	h.HandleCallback(w1, req1)
	if w1.Code != stdhttp.StatusOK {
		t.Fatalf("first callback status = %d, want 200", w1.Code)
	}

	// 2回目: replay → 400
	req2 := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+state, nil)
	req2 = req2.WithContext(auth.ContextWithUserID(req2.Context(), testUserID))
	w2 := httptest.NewRecorder()
	h.HandleCallback(w2, req2)

	if w2.Code != stdhttp.StatusBadRequest {
		t.Errorf("second callback status = %d, want 400", w2.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(w2.Body).Decode(&body)
	if body["error"] != "nonce_already_used" {
		t.Errorf("error = %q, want nonce_already_used", body["error"])
	}
}

// ============================================================================
// T8: Callback - TokenStore 失敗
// ============================================================================

// T8: TokenStore.Put がエラーを返す → 500 internal_error、SpaceRegistry.Upsert は呼ばれない (C2)
func TestMultiSpaceOAuthHandler_HandleCallback_TokenStoreFailure(t *testing.T) {
	state, err := auth.GenerateStateWithSpaceInfo(
		testUserID, "foo", "https://foo.backlog.com", "foo",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}

	claims, err := auth.ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v", err)
	}
	nonce := claims.Nonce

	nonceStore := space.NewMemoryStore()
	if err := nonceStore.Store(context.Background(), testUserID, nonce, testTTL); err != nil {
		t.Fatalf("NonceStore.Store() error = %v", err)
	}

	upsertCalled := false
	spaceStore := &fakeSpaceStore{
		upsertFn: func(ctx context.Context, reg *space.SpaceRegistration) error {
			upsertCalled = true
			return nil
		},
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{
			exchangeFn: defaultExchangeFn,
			userFn:     defaultUserFn,
		},
		fakeTokenManager{
			saveFn: func(ctx context.Context, record *auth.TokenRecord) error {
				return errors.New("storage failure")
			},
		},
		nonceStore,
		spaceStore,
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+state, nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	if w.Code != stdhttp.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if upsertCalled {
		t.Error("SpaceRegistry.Upsert should NOT be called when TokenStore.Put fails (C2)")
	}
}

// ============================================================================
// T9: Callback - SpaceStore.Upsert 失敗
// ============================================================================

// T9: TokenStore.Put 成功、SpaceRegistry.Upsert 失敗 → 500 internal_error
func TestMultiSpaceOAuthHandler_HandleCallback_SpaceUpsertFailure(t *testing.T) {
	state, err := auth.GenerateStateWithSpaceInfo(
		testUserID, "foo", "https://foo.backlog.com", "foo",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}

	claims, err := auth.ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v", err)
	}
	nonce := claims.Nonce

	nonceStore := space.NewMemoryStore()
	if err := nonceStore.Store(context.Background(), testUserID, nonce, testTTL); err != nil {
		t.Fatalf("NonceStore.Store() error = %v", err)
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{
			exchangeFn: defaultExchangeFn,
			userFn:     defaultUserFn,
		},
		fakeTokenManager{},
		nonceStore,
		&fakeSpaceStore{
			upsertFn: func(ctx context.Context, reg *space.SpaceRegistration) error {
				return errors.New("upsert failure")
			},
		},
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=auth-code&state="+state, nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	if w.Code != stdhttp.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// ============================================================================
// T10: Callback - DefaultSpaceAlias 条件付き更新
// ============================================================================

// T10: DefaultSpaceAlias が未設定の場合は callback 後に設定される。
// 既に設定済みの場合は変更しない。
func TestMultiSpaceOAuthHandler_HandleCallback_DefaultSpaceSetIfEmpty(t *testing.T) {
	buildStateAndNonce := func(alias string) (string, *space.MemoryStore) {
		st, err := auth.GenerateStateWithSpaceInfo(
			testUserID, alias, "https://"+alias+".backlog.com", alias,
			testSecret, testTTL,
		)
		if err != nil {
			t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
		}
		cl, err := auth.ValidateState(st, testSecret)
		if err != nil {
			t.Fatalf("ValidateState() error = %v", err)
		}
		ms := space.NewMemoryStore()
		if err := ms.Store(context.Background(), testUserID, cl.Nonce, testTTL); err != nil {
			t.Fatalf("NonceStore.Store() error = %v", err)
		}
		return st, ms
	}

	t.Run("DefaultSpaceAlias が空の場合は設定される", func(t *testing.T) {
		st, ns := buildStateAndNonce("bar")
		var putPrefCalled bool
		var savedPref *space.UserPreference
		ss := &fakeSpaceStore{
			getPrefFn: func(ctx context.Context, userID string) (*space.UserPreference, error) {
				return nil, nil // 未設定
			},
			putPrefFn: func(ctx context.Context, pref *space.UserPreference) error {
				putPrefCalled = true
				savedPref = pref
				return nil
			},
		}

		h, err := buildMultiSpaceHandler(
			fakeProvider{exchangeFn: defaultExchangeFn, userFn: defaultUserFn},
			fakeTokenManager{},
			ns,
			ss,
		)
		if err != nil {
			t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
		}

		req := httptest.NewRequest(stdhttp.MethodGet,
			"/oauth/backlog/multi/callback?code=auth-code&state="+st, nil)
		req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
		w := httptest.NewRecorder()
		h.HandleCallback(w, req)

		if w.Code != stdhttp.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if !putPrefCalled {
			t.Error("PutPreference should be called when DefaultSpaceAlias is empty")
		}
		if savedPref != nil && savedPref.DefaultSpaceAlias != "bar" {
			t.Errorf("DefaultSpaceAlias = %q, want bar", savedPref.DefaultSpaceAlias)
		}
	})

	t.Run("DefaultSpaceAlias が設定済みの場合は変更しない", func(t *testing.T) {
		st, ns := buildStateAndNonce("baz")
		putPrefCalled := false
		ss := &fakeSpaceStore{
			getPrefFn: func(ctx context.Context, userID string) (*space.UserPreference, error) {
				return &space.UserPreference{
					UserID:            userID,
					DefaultSpaceAlias: "existing",
				}, nil
			},
			putPrefFn: func(ctx context.Context, pref *space.UserPreference) error {
				putPrefCalled = true
				return nil
			},
		}

		h, err := buildMultiSpaceHandler(
			fakeProvider{exchangeFn: defaultExchangeFn, userFn: defaultUserFn},
			fakeTokenManager{},
			ns,
			ss,
		)
		if err != nil {
			t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
		}

		req := httptest.NewRequest(stdhttp.MethodGet,
			"/oauth/backlog/multi/callback?code=auth-code&state="+st, nil)
		req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
		w := httptest.NewRecorder()
		h.HandleCallback(w, req)

		if w.Code != stdhttp.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if putPrefCalled {
			t.Error("PutPreference should NOT be called when DefaultSpaceAlias is already set")
		}
	})
}

// ============================================================================
// Step 2: HandleAuthorize bootstrap_token テスト
// ============================================================================

// TestHandleAuthorize_BootstrapTokenMissing: bootstrap_token なしで 401。
func TestHandleAuthorize_BootstrapTokenMissing(t *testing.T) {
	h, err := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, &fakeNonceStore{}, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
	}
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo", nil)
	w := httptest.NewRecorder()
	h.HandleAuthorize(w, req)
	if w.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// TestHandleAuthorize_BootstrapTokenInvalid: 無効な bootstrap_token で 401。
func TestHandleAuthorize_BootstrapTokenInvalid(t *testing.T) {
	ns := &fakeNonceStore{consumeFn: func(ctx context.Context, userID, nonce string) error { return nil }}
	h, err := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, ns, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
	}
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo&bootstrap_token=invalid.token.here", nil)
	w := httptest.NewRecorder()
	h.HandleAuthorize(w, req)
	if w.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// TestHandleAuthorize_BootstrapTokenBaseURLMismatch: token は valid だが URL query の base_url が異なる → 401。
func TestHandleAuthorize_BootstrapTokenBaseURLMismatch(t *testing.T) {
	ns := &fakeNonceStore{}
	const baseURL = "https://foo.backlog.com"
	btok := makeBootstrapToken(t, ns, baseURL, "foo")

	h, err := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, ns, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
	}
	// 別の base_url で検証
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url=https://other.backlog.com&alias=foo&bootstrap_token="+btok, nil)
	w := httptest.NewRecorder()
	h.HandleAuthorize(w, req)
	if w.Code != stdhttp.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (base_url mismatch)", w.Code)
	}
}

// TestHandleAuthorize_BootstrapTokenJTIReplay: 同じ token を 2 回送ると 2 回目が 401。
func TestHandleAuthorize_BootstrapTokenJTIReplay(t *testing.T) {
	consumeCount := 0
	ns := &fakeNonceStore{
		storeFn: func(ctx context.Context, userID, nonce string, ttl time.Duration) error { return nil },
		consumeFn: func(ctx context.Context, userID, nonce string) error {
			consumeCount++
			if consumeCount > 1 {
				return space.ErrNonceAlreadyUsed
			}
			return nil
		},
	}
	const baseURL = "https://foo.backlog.com"
	btok := makeBootstrapToken(t, ns, baseURL, "foo")

	for i := 0; i < 2; i++ {
		// 2 回目は別トークンでないと jti が違うので同じトークンを使う
		tok := btok
		h, err := buildMultiSpaceHandlerWithKey(
			fakeProvider{},
			fakeTokenManager{},
			ns,
			&fakeSpaceStore{},
		)
		if err != nil {
			t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
		}
		req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url="+baseURL+"&alias=foo&bootstrap_token="+tok, nil)
		w := httptest.NewRecorder()
		h.HandleAuthorize(w, req)
		if i == 0 && w.Code != stdhttp.StatusFound {
			t.Errorf("1st request: status = %d, want 302", w.Code)
		}
		if i == 1 && w.Code != stdhttp.StatusUnauthorized {
			t.Errorf("2nd request (replay): status = %d, want 401", w.Code)
		}
	}
}

// TestHandleAuthorize_HEAD_405: HEAD request では jti が consume されず 405 を返す。
func TestHandleAuthorize_HEAD_405(t *testing.T) {
	consumeCalled := false
	ns := &fakeNonceStore{
		consumeFn: func(ctx context.Context, userID, nonce string) error {
			consumeCalled = true
			return nil
		},
	}
	const baseURL = "https://foo.backlog.com"
	btok := makeBootstrapToken(t, ns, baseURL, "foo")

	h, err := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, ns, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
	}
	req := httptest.NewRequest(stdhttp.MethodHead, "/oauth/backlog/multi/authorize?base_url="+baseURL+"&alias=foo&bootstrap_token="+btok, nil)
	w := httptest.NewRecorder()
	h.HandleAuthorize(w, req)

	if w.Code != stdhttp.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 for HEAD", w.Code)
	}
	if consumeCalled {
		t.Error("jti should NOT be consumed on HEAD request")
	}
}

// TestHandleAuthorize_ReferrerPolicy: 成功時も失敗時も Referrer-Policy: no-referrer が付与される。
func TestHandleAuthorize_ReferrerPolicy(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"success", "/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo&bootstrap_token=PLACEHOLDER"},
		{"missing_token", "/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ns := &fakeNonceStore{}
			h, err := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, ns, &fakeSpaceStore{})
			if err != nil {
				t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
			}
			url := tc.url
			if tc.name == "success" {
				btok := makeBootstrapToken(t, ns, "https://foo.backlog.com", "foo")
				url = "/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo&bootstrap_token=" + btok
			}
			req := httptest.NewRequest(stdhttp.MethodGet, url, nil)
			w := httptest.NewRecorder()
			h.HandleAuthorize(w, req)

			if w.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Errorf("Referrer-Policy = %q, want no-referrer", w.Header().Get("Referrer-Policy"))
			}
		})
	}
}

// TestHandleAuthorize_DuplicateBaseURLQuery_400: base_url が重複していると 400。
func TestHandleAuthorize_DuplicateBaseURLQuery_400(t *testing.T) {
	h, err := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, &fakeNonceStore{}, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
	}
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&base_url=https://bar.backlog.com&alias=foo&bootstrap_token=x", nil)
	w := httptest.NewRecorder()
	h.HandleAuthorize(w, req)
	if w.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want 400 (duplicate base_url)", w.Code)
	}
}

// TestHandleAuthorize_BootstrapTokenJTIReplay_AcrossHandlers: 同一 NonceStore を共有する 2 つの handler instance で replay 拒否。
func TestHandleAuthorize_BootstrapTokenJTIReplay_AcrossHandlers(t *testing.T) {
	consumeCount := 0
	sharedNS := &fakeNonceStore{
		storeFn: func(ctx context.Context, userID, nonce string, ttl time.Duration) error { return nil },
		consumeFn: func(ctx context.Context, userID, nonce string) error {
			consumeCount++
			if consumeCount > 1 {
				return space.ErrNonceAlreadyUsed
			}
			return nil
		},
	}
	const baseURL = "https://foo.backlog.com"
	btok := makeBootstrapToken(t, sharedNS, baseURL, "foo")

	h1, _ := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, sharedNS, &fakeSpaceStore{})
	h2, _ := buildMultiSpaceHandlerWithKey(fakeProvider{}, fakeTokenManager{}, sharedNS, &fakeSpaceStore{})

	// h1 で 1 回目（成功）
	req1 := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url="+baseURL+"&alias=foo&bootstrap_token="+btok, nil)
	w1 := httptest.NewRecorder()
	h1.HandleAuthorize(w1, req1)
	if w1.Code != stdhttp.StatusFound {
		t.Errorf("h1 1st: status = %d, want 302", w1.Code)
	}

	// h2 で同じ token を送ると replay 拒否
	req2 := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/multi/authorize?base_url="+baseURL+"&alias=foo&bootstrap_token="+btok, nil)
	w2 := httptest.NewRecorder()
	h2.HandleAuthorize(w2, req2)
	if w2.Code != stdhttp.StatusUnauthorized {
		t.Errorf("h2 replay: status = %d, want 401", w2.Code)
	}
}

// ============================================================================
// Step 3: MultiSpaceOAuthHandler.HandleCallback 防御テスト
// ============================================================================

// TestHandleCallback_DefensiveFlowCheck: flow != "multi" の state を直接渡すと 400 state_invalid。
func TestMultiSpaceOAuthHandler_HandleCallback_DefensiveFlowCheck(t *testing.T) {
	h, err := buildMultiSpaceHandler(fakeProvider{}, fakeTokenManager{}, &fakeNonceStore{}, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandler: %v", err)
	}

	// flow="" (single) の state JWT を生成
	state, stErr := auth.GenerateState(testUserID, testTenant, testSecret, testTTL)
	if stErr != nil {
		t.Fatalf("GenerateState: %v", stErr)
	}

	ctx := auth.ContextWithUserID(context.Background(), testUserID)
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/callback?code=abc&state="+state, nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)

	if w.Code != stdhttp.StatusBadRequest {
		t.Errorf("status = %d, want 400 (defensive flow check)", w.Code)
	}
}

// TestHandleCallback_NoIdproxyContext_StillSucceeds: idproxy ctx に uid なくても state.UserID で完走。
func TestMultiSpaceOAuthHandler_HandleCallback_NoIdproxyContext_StillSucceeds(t *testing.T) {
	var upsertedAlias string
	nonceStore := &fakeNonceStore{}
	spaceStore := &fakeSpaceStore{
		upsertFn: func(ctx context.Context, reg *space.SpaceRegistration) error {
			upsertedAlias = reg.Alias
			return nil
		},
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{
			exchangeFn: defaultExchangeFn,
			userFn:     defaultUserFn,
		},
		fakeTokenManager{},
		nonceStore,
		spaceStore,
	)
	if err != nil {
		t.Fatalf("buildMultiSpaceHandler: %v", err)
	}

	// flow="multi" の state JWT
	st, stErr := auth.GenerateStateWithSpaceInfo(testUserID, testTenant, "https://foo.backlog.com", "foo", testSecret, testTTL)
	if stErr != nil {
		t.Fatalf("GenerateStateWithSpaceInfo: %v", stErr)
	}

	// context に uid を注入しない（idproxy セッション切れを模擬）
	req := httptest.NewRequest(stdhttp.MethodGet, "/oauth/backlog/callback?code=auth-code&state="+st, nil)
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)

	if w.Code != stdhttp.StatusOK {
		t.Errorf("status = %d, want 200 (no idproxy ctx should still succeed)", w.Code)
	}
	if upsertedAlias != "foo" {
		t.Errorf("upsertedAlias = %q, want %q", upsertedAlias, "foo")
	}
}

// ============================================================================
// M19: baseURL 動的切り替えテスト
// ============================================================================

// TestHandleAuthorize_UsesTargetBaseURL: base_url=https://megumilog.backlog.jp を渡すと
// 認可 URL が megumilog.backlog.jp ホストになること。
func TestHandleAuthorize_UsesTargetBaseURL(t *testing.T) {
	ns := &fakeNonceStore{
		storeFn:   func(ctx context.Context, userID, nonce string, ttl time.Duration) error { return nil },
		consumeFn: func(ctx context.Context, userID, nonce string) error { return nil },
	}

	// buildFn なし: fakeProvider.CloneWithBaseURL のデフォルト実装が targetBaseURL を使う
	fp := fakeProvider{}

	const targetBaseURL = "https://megumilog.backlog.jp"
	btok := makeBootstrapToken(t, ns, targetBaseURL, "megumilog")

	h, err := buildMultiSpaceHandlerWithKey(fp, fakeTokenManager{}, ns, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandlerWithKey: %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/authorize?base_url="+targetBaseURL+"&alias=megumilog&bootstrap_token="+btok, nil)
	w := httptest.NewRecorder()
	h.HandleAuthorize(w, req)

	resp := w.Result()
	if resp.StatusCode != stdhttp.StatusFound {
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		t.Fatalf("status = %d, want 302; body = %s", resp.StatusCode, body[:n])
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	// 認可 URL が megumilog.backlog.jp を含むこと
	if !strings.Contains(location, "megumilog.backlog.jp") {
		t.Errorf("Location %q should contain megumilog.backlog.jp (CloneWithBaseURL not applied)", location)
	}
}

// TestHandleCallback_UsesStateBaseURL: callback が state JWT の BaseURL でトークン交換を行うこと。
func TestHandleCallback_UsesStateBaseURL(t *testing.T) {
	const targetBaseURL = "https://megumilog.backlog.jp"
	var exchangeCalledWith string // ExchangeCode が呼ばれたときの context 情報（テスト用）
	var getUserCalledWith string  // GetCurrentUser が呼ばれたことを確認

	state, err := auth.GenerateStateWithSpaceInfo(
		testUserID, "megumilog", targetBaseURL, "megumilog",
		testSecret, testTTL,
	)
	if err != nil {
		t.Fatalf("GenerateStateWithSpaceInfo() error = %v", err)
	}

	claims, err := auth.ValidateState(state, testSecret)
	if err != nil {
		t.Fatalf("ValidateState() error = %v", err)
	}

	nonceStore := space.NewMemoryStore()
	if err := nonceStore.Store(context.Background(), testUserID, claims.Nonce, testTTL); err != nil {
		t.Fatalf("NonceStore.Store() error = %v", err)
	}

	fp := fakeProvider{
		exchangeFn: func(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
			exchangeCalledWith = code
			return &auth.TokenRecord{
				Provider:    "backlog",
				AccessToken: "megumilog-token",
			}, nil
		},
		userFn: func(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
			getUserCalledWith = accessToken
			return &auth.ProviderUser{ID: "megumilog-user", Name: "Megumi"}, nil
		},
	}

	h, err := buildMultiSpaceHandler(fp, fakeTokenManager{}, nonceStore, &fakeSpaceStore{})
	if err != nil {
		t.Fatalf("buildMultiSpaceHandler: %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/callback?code=megumi-code&state="+state, nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)

	resp := w.Result()
	if resp.StatusCode != stdhttp.StatusOK {
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body[:n])
	}

	if exchangeCalledWith != "megumi-code" {
		t.Errorf("ExchangeCode was not called with correct code: got %q", exchangeCalledWith)
	}
	if getUserCalledWith != "megumilog-token" {
		t.Errorf("GetCurrentUser was not called with correct token: got %q", getUserCalledWith)
	}
}
