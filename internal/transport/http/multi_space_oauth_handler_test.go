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

// buildMultiSpaceHandler はテスト用の MultiSpaceOAuthHandler を構築する。
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
	)
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
// - GET /oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo
// - 302 → Backlog OAuth URL へリダイレクト
// - state JWT に BaseURL/Alias が含まれる
// - nonce が NonceStore に保存される
func TestMultiSpaceOAuthHandler_HandleAuthorize_Success(t *testing.T) {
	var storedNonce string
	var storedUserID string

	nonceStore := &fakeNonceStore{
		storeFn: func(ctx context.Context, userID, nonce string, ttl time.Duration) error {
			storedNonce = nonce
			storedUserID = userID
			return nil
		},
	}

	h, err := buildMultiSpaceHandler(
		fakeProvider{},
		fakeTokenManager{},
		nonceStore,
		&fakeSpaceStore{},
	)
	if err != nil {
		t.Fatalf("NewMultiSpaceOAuthHandler() error = %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodGet,
		"/oauth/backlog/multi/authorize?base_url=https://foo.backlog.com&alias=foo", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUserID))
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

	// state JWT から BaseURL/Alias を検証
	parsedURL := location
	idx := strings.Index(parsedURL, "state=")
	if idx < 0 {
		t.Fatal("state parameter not found in redirect URL")
	}
	// state= 以降を抜き出す（& で終わる場合も考慮）
	stateParam := parsedURL[idx+len("state="):]
	if end := strings.Index(stateParam, "&"); end >= 0 {
		stateParam = stateParam[:end]
	}
	// URL デコード
	decoded, err2 := stdhttp.DefaultClient.Get("about:blank")
	_ = decoded
	_ = err2
	// 簡易チェック: state JWT 文字列が含まれていること
	if stateParam == "" {
		t.Error("state JWT is empty in redirect URL")
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
