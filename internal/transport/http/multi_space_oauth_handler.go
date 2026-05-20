package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdhttp "net/http"
	stdurl "net/url"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/auth/provider"
	"github.com/youyo/logvalet/internal/space"
)

// nonce_already_used エラーコード定数（MS10）
const (
	errCodeNonceAlreadyUsed = "nonce_already_used"
	errMsgNonceAlreadyUsed  = "nonce already consumed (replay rejected)"
	errMsgMissingBaseURL    = "base_url parameter is required"
	errMsgMissingAlias      = "alias parameter is required"
	errMsgInvalidAlias      = "alias parameter is invalid"
	errMsgInvalidBaseURL    = "base_url parameter is invalid"
	errMsgSpaceUpsertFailed = "failed to register space"
)

// MultiSpaceOAuthHandler は multi-space 登録フロー用の OAuth ハンドラー。
// 既存の OAuthHandler は変更せず、multi-space 登録フロー専用に別ファイルで実装する。
//
// Authorize → Callback の処理順序 (C2):
//  1. state JWT 検証
//  2. userID 一致検証
//  3. nonce 消費（replay 防止: C3）
//  4. code exchange → token 取得
//  5. TokenManager.SaveToken（先に保存: C2）
//  6. provider.GetCurrentUser
//  7. SpaceStore.Upsert（C2: token 保存後に必ず Upsert）
//  8. UserPreference 条件付き更新
//  9. 200 JSON
type MultiSpaceOAuthHandler struct {
	provider     provider.OAuthProvider
	tokenManager auth.TokenManager
	nonceStore   space.NonceStore
	spaceStore   space.Store
	redirectURI  string
	stateSecret  []byte
	stateTTL     time.Duration
	logger       *slog.Logger
}

// NewMultiSpaceOAuthHandler は MultiSpaceOAuthHandler を構築する。
//
// provider が nil の場合は panic する。
// tokenManager が nil の場合は panic する。
// nonceStore が nil の場合は panic する。
// spaceStore が nil の場合は panic する。
// redirectURI が空の場合は auth.ErrInvalidRedirectURI を返す。
// stateSecret が空の場合は auth.ErrStateInvalid を返す。
// stateTTL が 0 以下の場合は auth.ErrStateInvalid を返す。
// logger が nil の場合は slog.Default() を使用する。
func NewMultiSpaceOAuthHandler(
	p provider.OAuthProvider,
	tm auth.TokenManager,
	nonceStore space.NonceStore,
	spaceStore space.Store,
	redirectURI string,
	stateSecret []byte,
	stateTTL time.Duration,
	logger *slog.Logger,
) (*MultiSpaceOAuthHandler, error) {
	if p == nil {
		panic("http: NewMultiSpaceOAuthHandler: provider must not be nil")
	}
	if tm == nil {
		panic("http: NewMultiSpaceOAuthHandler: tokenManager must not be nil")
	}
	if nonceStore == nil {
		panic("http: NewMultiSpaceOAuthHandler: nonceStore must not be nil")
	}
	if spaceStore == nil {
		panic("http: NewMultiSpaceOAuthHandler: spaceStore must not be nil")
	}
	if redirectURI == "" {
		return nil, auth.ErrInvalidRedirectURI
	}
	if len(stateSecret) == 0 {
		return nil, auth.ErrStateInvalid
	}
	if stateTTL <= 0 {
		return nil, auth.ErrStateInvalid
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MultiSpaceOAuthHandler{
		provider:     p,
		tokenManager: tm,
		nonceStore:   nonceStore,
		spaceStore:   spaceStore,
		redirectURI:  redirectURI,
		stateSecret:  stateSecret,
		stateTTL:     stateTTL,
		logger:       logger,
	}, nil
}

// HandleAuthorize は multi-space 登録フロー用の authorize エンドポイント。
//
// クエリパラメータ:
//   - base_url: Backlog スペースの base URL（例: https://foo.backlog.com）
//   - alias: スペースの alias（オプション。空の場合は base_url から導出）
//
// 処理フロー:
//  1. GET メソッドであることを確認
//  2. base_url / alias パラメータを取得・検証
//  3. context から userID を取得
//  4. nonce を生成して NonceStore に保存（TTL: stateTTL）
//  5. state JWT 生成（BaseURL/Alias/UserID を含む）
//  6. 302 Redirect
//
// セキュリティ: state JWT 生値・stateSecret はログに出さない。
func (h *MultiSpaceOAuthHandler) HandleAuthorize(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	if r.Method != stdhttp.MethodGet {
		writeJSONError(w, stdhttp.StatusMethodNotAllowed, errCodeMethodNotAllowed, errMsgMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	rawBaseURL := q.Get("base_url")
	alias := q.Get("alias")

	if rawBaseURL == "" {
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgMissingBaseURL)
		return
	}

	// idproxy の continue リダイレクト経由で二重エンコードされた場合に備えてデコードを試みる
	if decoded, decErr := stdurl.QueryUnescape(rawBaseURL); decErr == nil && decoded != rawBaseURL {
		rawBaseURL = decoded
	}

	// base_url 正規化
	baseURL, err := space.NormalizeBaseURL(rawBaseURL)
	if err != nil {
		h.logger.WarnContext(ctx, "multi-space authorize rejected",
			slog.String("reason", "invalid_base_url"),
			slog.String("raw_base_url", rawBaseURL),
			slog.String("err", err.Error()),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgInvalidBaseURL)
		return
	}

	// alias 未指定の場合は base_url から導出
	if alias == "" {
		derived, deriveErr := space.DeriveAliasFromBaseURL(baseURL)
		if deriveErr != nil || derived == "" {
			// カスタムドメインの場合は alias 必須
			writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgMissingAlias)
			return
		}
		alias = derived
	}

	// alias バリデーション
	if err := space.ValidateAlias(alias); err != nil {
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgInvalidAlias)
		return
	}

	// userID 取得
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUnauthenticated, errMsgUnauthenticated)
		return
	}

	// tenant を導出（base_url から）
	tenant, err := space.DeriveInitialTenant(baseURL)
	if err != nil || tenant == "" {
		tenant = alias
	}

	// state JWT 生成（BaseURL/Alias を含む）
	state, err := auth.GenerateStateWithSpaceInfo(userID, tenant, baseURL, alias, h.stateSecret, h.stateTTL)
	if err != nil {
		h.logger.ErrorContext(ctx, "multi-space authorize failed",
			slog.String("reason", "state_generation_failed"),
			slog.String("err", err.Error()),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}

	// state JWT から nonce を取得して NonceStore に保存
	claims, err := auth.ValidateState(state, h.stateSecret)
	if err != nil {
		h.logger.ErrorContext(ctx, "multi-space authorize failed",
			slog.String("reason", "state_validate_failed"),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}
	if err := h.nonceStore.Store(ctx, userID, claims.Nonce, h.stateTTL); err != nil {
		h.logger.ErrorContext(ctx, "multi-space authorize failed",
			slog.String("reason", "nonce_store_failed"),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}

	authURL, err := h.provider.BuildAuthorizationURL(state, h.redirectURI)
	if err != nil {
		h.logger.ErrorContext(ctx, "multi-space authorize failed",
			slog.String("reason", "build_url_failed"),
			slog.String("user_id", userID),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}

	h.logger.InfoContext(ctx, "multi-space authorize started",
		slog.String("user_id", userID),
		slog.String("provider", h.provider.Name()),
		slog.String("alias", alias),
	)
	stdhttp.Redirect(w, r, authURL, stdhttp.StatusFound)
}

// HandleCallback は multi-space 登録フロー用の callback エンドポイント。
//
// 処理順序（C2/C3 対応）:
//  1. state JWT 検証
//  2. userID 一致検証（ctx vs state.uid）
//  3. nonce 消費（replay 防止: C3）
//  4. code exchange → token 取得
//  5. TokenManager.SaveToken（先に保存: C2）
//  6. provider.GetCurrentUser
//  7. SpaceStore.Upsert
//  8. UserPreference 条件付き更新（DefaultSpaceAlias == "" なら設定）
//  9. 200 JSON
//
// セキュリティ: code/state/token をログに出さない。
func (h *MultiSpaceOAuthHandler) HandleCallback(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	if r.Method != stdhttp.MethodGet {
		writeJSONError(w, stdhttp.StatusMethodNotAllowed, errCodeMethodNotAllowed, errMsgMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	if provErr := q.Get("error"); provErr != "" {
		h.logger.WarnContext(ctx, "multi-space callback rejected",
			slog.String("reason", errCodeProviderDenied),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeProviderDenied, errMsgProviderDenied)
		return
	}

	code := q.Get("code")
	stateJWT := q.Get("state")
	if code == "" {
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgMissingCode)
		return
	}
	if stateJWT == "" {
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgMissingState)
		return
	}

	// 1. state JWT 検証
	claims, err := auth.ValidateState(stateJWT, h.stateSecret)
	if err != nil {
		if errors.Is(err, auth.ErrStateExpired) {
			writeJSONError(w, stdhttp.StatusBadRequest, errCodeStateExpired, errMsgStateExpired)
			return
		}
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeStateInvalid, errMsgStateInvalid)
		return
	}

	// 防御的: flow="multi" であることを確認（dispatcher の誤呼び出しに対する保険）
	if claims.Flow != "multi" {
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeStateInvalid, errMsgStateInvalid)
		return
	}

	// 2. userID 一致検証（idproxy セッション切れ時は state.UserID をフォールバックとして使用）
	ctxUserID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		// idproxy セッション切れケース: state.UserID を信頼して注入
		ctx = auth.ContextWithUserID(ctx, claims.UserID)
		r = r.WithContext(ctx)
		ctxUserID = claims.UserID
	} else if ctxUserID != claims.UserID {
		h.logger.WarnContext(ctx, "multi-space callback rejected",
			slog.String("reason", errCodeUserMismatch),
			slog.String("user_id", ctxUserID),
		)
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUserMismatch, errMsgUserMismatch)
		return
	}

	// 3. nonce 消費（replay 防止: C3）
	if err := h.nonceStore.Consume(ctx, claims.UserID, claims.Nonce); err != nil {
		if errors.Is(err, space.ErrNonceAlreadyUsed) {
			h.logger.WarnContext(ctx, "multi-space callback rejected",
				slog.String("reason", errCodeNonceAlreadyUsed),
				slog.String("user_id", ctxUserID),
			)
			writeJSONError(w, stdhttp.StatusBadRequest, errCodeNonceAlreadyUsed, errMsgNonceAlreadyUsed)
			return
		}
		h.logger.ErrorContext(ctx, "multi-space callback failed",
			slog.String("reason", "nonce_consume_failed"),
			slog.String("user_id", ctxUserID),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}

	// 4. code exchange → token 取得
	record, err := h.provider.ExchangeCode(ctx, code, h.redirectURI)
	if err != nil {
		h.logCallbackFailed(ctx, "exchange_failed", err, ctxUserID)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgExchangeFailed)
		return
	}
	if record == nil {
		h.logCallbackFailed(ctx, "exchange_nil_record", nil, ctxUserID)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgExchangeFailed)
		return
	}

	// identity 補完
	record.UserID = ctxUserID
	record.Provider = h.provider.Name()
	if record.Provider == "" {
		record.Provider = "backlog"
	}
	record.Tenant = claims.Tenant

	// 5. TokenManager.SaveToken（先に保存: C2）
	if err := h.tokenManager.SaveToken(ctx, record); err != nil {
		h.logCallbackFailed(ctx, "save_failed", err, ctxUserID)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgSaveFailed)
		return
	}

	// 6. GetCurrentUser
	providerUser, err := h.provider.GetCurrentUser(ctx, record.AccessToken)
	if err != nil {
		h.logCallbackFailed(ctx, "get_user_failed", err, ctxUserID)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgUserFetchFailed)
		return
	}
	if providerUser == nil {
		h.logCallbackFailed(ctx, "get_user_nil", nil, ctxUserID)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgUserFetchFailed)
		return
	}

	record.ProviderUserID = providerUser.ID

	// 7. SpaceStore.Upsert（C2: token 保存後に必ず Upsert）
	reg := &space.SpaceRegistration{
		UserID:    ctxUserID,
		Alias:     claims.Alias,
		Tenant:    claims.Tenant,
		BaseURL:   claims.BaseURL,
		AuthType:  space.AuthTypeOAuth,
		Provider:  h.provider.Name(),
		Status:    space.SpaceStatusOK,
	}
	if err := h.spaceStore.Upsert(ctx, reg); err != nil {
		h.logCallbackFailed(ctx, "upsert_failed", err, ctxUserID)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgSpaceUpsertFailed)
		return
	}

	// 8. UserPreference 条件付き更新（DefaultSpaceAlias == "" なら設定）
	h.updateDefaultSpaceIfEmpty(ctx, ctxUserID, claims.Alias)

	h.logger.InfoContext(ctx, "multi-space callback success",
		slog.String("user_id", ctxUserID),
		slog.String("alias", claims.Alias),
		slog.String("provider", h.provider.Name()),
	)

	writeJSONSuccess(w, stdhttp.StatusOK, callbackSuccessResponse{
		Status:           "connected",
		Provider:         h.provider.Name(),
		Tenant:           claims.Tenant,
		ProviderUserID:   providerUser.ID,
		ProviderUserName: providerUser.Name,
	})
}

// updateDefaultSpaceIfEmpty は DefaultSpaceAlias が空の場合のみ alias を設定する。
func (h *MultiSpaceOAuthHandler) updateDefaultSpaceIfEmpty(ctx context.Context, userID, alias string) {
	pref, err := h.spaceStore.GetPreference(ctx, userID)
	if err != nil {
		h.logger.WarnContext(ctx, "multi-space callback: failed to get preference",
			slog.String("user_id", userID),
			slog.String("err_type", fmt.Sprintf("%T", err)),
		)
		return
	}
	if pref != nil && pref.DefaultSpaceAlias != "" {
		return
	}
	newPref := &space.UserPreference{
		UserID:            userID,
		DefaultSpaceAlias: alias,
	}
	if pref != nil {
		newPref.CreatedAt = pref.CreatedAt
	}
	if err := h.spaceStore.PutPreference(ctx, newPref); err != nil {
		h.logger.WarnContext(ctx, "multi-space callback: failed to set default space",
			slog.String("user_id", userID),
			slog.String("err_type", fmt.Sprintf("%T", err)),
		)
	}
}

// logCallbackFailed は callback 失敗ログを出力する。
// err.Error() は出力せず型情報のみ出す（セキュリティ判断）。
func (h *MultiSpaceOAuthHandler) logCallbackFailed(ctx context.Context, reason string, err error, userID string) {
	attrs := []any{
		slog.String("reason", reason),
		slog.String("user_id", userID),
		slog.String("provider", h.provider.Name()),
	}
	if err != nil {
		attrs = append(attrs, slog.String("err_type", fmt.Sprintf("%T", err)))
	}
	h.logger.ErrorContext(ctx, "multi-space callback failed", attrs...)
}
