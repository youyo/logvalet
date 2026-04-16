package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	stdhttp "net/http"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/auth/provider"
)

// errorResponse は OAuth ハンドラーの統一 JSON エラーレスポンス形式。
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// callbackSuccessResponse は /oauth/backlog/callback 正常系の JSON レスポンス形式。
type callbackSuccessResponse struct {
	Status           string `json:"status"`
	Provider         string `json:"provider"`
	Tenant           string `json:"tenant"`
	ProviderUserID   string `json:"provider_user_id"`
	ProviderUserName string `json:"provider_user_name"`
}

// エラーコード定数（JSON レスポンスの "error" フィールド値）。
const (
	// 既存 (M13)
	errCodeUnauthenticated  = "unauthenticated"
	errCodeMethodNotAllowed = "method_not_allowed"
	errCodeInternalError    = "internal_error"

	// 新規 (M14)
	errCodeInvalidRequest = "invalid_request"
	errCodeStateExpired   = "state_expired"
	errCodeStateInvalid   = "state_invalid"
	errCodeProviderDenied = "provider_denied"
	errCodeProviderError  = "provider_error"
	errCodeInvalidTenant  = "invalid_tenant"
	errCodeUserMismatch   = "user_mismatch"

	// メッセージ定数（M13）
	errMsgUnauthenticated  = "user ID is required to start OAuth flow"
	errMsgMethodNotAllowed = "only GET is allowed"
	errMsgInternalError    = "failed to start OAuth flow"

	// メッセージ定数（M14）
	errMsgMissingCode       = "authorization code is required"
	errMsgMissingState      = "state token is required"
	errMsgProviderDenied    = "provider denied the authorization request"
	errMsgStateExpired      = "state token expired"
	errMsgStateInvalid      = "state token invalid"
	errMsgInvalidTenant     = "state tenant does not match"
	errMsgCallbackUnauth    = "user ID is required to complete OAuth callback"
	errMsgUserMismatch      = "session user does not match state user"
	errMsgExchangeFailed    = "failed to exchange authorization code"
	errMsgUserFetchFailed   = "failed to fetch current user"
	errMsgSaveFailed        = "failed to save token"
	errMsgCallbackInternal  = "failed to complete OAuth callback"

	// メッセージ定数（M15 — ステータス / 切断）
	errMsgStatusInternal   = "failed to fetch connection status"
	errMsgDisconnectFailed = "failed to disconnect"
	errMsgStatusUnauth     = "user ID is required to check connection status"
	errMsgDisconnectUnauth = "user ID is required to disconnect"
)

// statusResponse は /oauth/backlog/status のレスポンス形式。
// connected のみ常時出力、その他は omitempty。
type statusResponse struct {
	Connected      bool   `json:"connected"`
	NeedsReauth    bool   `json:"needs_reauth,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Tenant         string `json:"tenant,omitempty"`
	ProviderUserID string `json:"provider_user_id,omitempty"`
}

// disconnectResponse は /oauth/backlog/disconnect のレスポンス形式。
type disconnectResponse struct {
	Status   string `json:"status"`
	Provider string `json:"provider"`
	Tenant   string `json:"tenant"`
}

// OAuthHandler は Backlog OAuth フロー用の HTTP ハンドラーを提供する。
// ハンドラーは MCP サーバーに組み込まれる想定で、ルーティングは M16 で行う。
//
// tenant フィールドは BacklogOAuthProvider.space と同一値（例: "example-space"）
// である必要がある。M14 callback ハンドラーで state claims の tenant と
// OAuthHandler.tenant を比較するため、ここで不一致があると callback が失敗する。
type OAuthHandler struct {
	provider     provider.OAuthProvider
	tokenManager auth.TokenManager
	tenant       string
	redirectURI  string
	stateSecret  []byte
	stateTTL     time.Duration
	logger       *slog.Logger
}

// NewOAuthHandler は OAuthHandler を構築する。
//
// provider が nil の場合は panic する（programming error として早期検出）。
// tokenManager が nil の場合は panic する（programming error として早期検出）。
// tenant が空の場合は auth.ErrInvalidTenant を返す。
// redirectURI が空の場合は auth.ErrInvalidRedirectURI を返す。
// stateSecret が nil または空の場合は auth.ErrStateInvalid を返す。
// stateTTL が 0 以下の場合は auth.ErrStateInvalid を返す。
// logger が nil の場合は slog.Default() を使用する。
func NewOAuthHandler(
	p provider.OAuthProvider,
	tm auth.TokenManager,
	tenant, redirectURI string,
	stateSecret []byte,
	stateTTL time.Duration,
	logger *slog.Logger,
) (*OAuthHandler, error) {
	if p == nil {
		panic("http: NewOAuthHandler: provider must not be nil")
	}
	if tm == nil {
		panic("http: NewOAuthHandler: tokenManager must not be nil")
	}
	if tenant == "" {
		return nil, auth.ErrInvalidTenant
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
	return &OAuthHandler{
		provider:     p,
		tokenManager: tm,
		tenant:       tenant,
		redirectURI:  redirectURI,
		stateSecret:  stateSecret,
		stateTTL:     stateTTL,
		logger:       logger,
	}, nil
}

// HandleAuthorize は /oauth/backlog/authorize の GET ハンドラー。
//
// 処理フロー:
//  1. HTTP メソッドが GET であることを確認（それ以外は 405）
//  2. context から userID を取得（なければ 401）
//  3. signed state JWT を生成
//  4. provider.BuildAuthorizationURL で認可 URL を構築
//  5. 302 Found で認可 URL へリダイレクト
//
// セキュリティ: state JWT 生値・stateSecret は一切ログに出さない。
// ログに出すのは user_id / provider / tenant のみ。
func (h *OAuthHandler) HandleAuthorize(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	// 1. メソッドチェック
	if r.Method != stdhttp.MethodGet {
		writeJSONError(w, stdhttp.StatusMethodNotAllowed, errCodeMethodNotAllowed, errMsgMethodNotAllowed)
		return
	}

	// 2. userID 取得
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		h.logger.WarnContext(ctx, "oauth authorize rejected",
			slog.String("reason", "unauthenticated"),
		)
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUnauthenticated, errMsgUnauthenticated)
		return
	}

	// 3. signed state 生成
	state, err := auth.GenerateState(userID, h.tenant, h.stateSecret, h.stateTTL)
	if err != nil {
		h.logger.ErrorContext(ctx, "oauth authorize failed",
			slog.String("reason", "state_generation_failed"),
			slog.String("err", err.Error()),
			slog.String("user_id", userID),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}

	// 4. 認可 URL 構築
	authURL, err := h.provider.BuildAuthorizationURL(state, h.redirectURI)
	if err != nil {
		h.logger.ErrorContext(ctx, "oauth authorize failed",
			slog.String("reason", "build_url_failed"),
			slog.String("err", err.Error()),
			slog.String("user_id", userID),
			slog.String("provider", h.provider.Name()),
		)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgInternalError)
		return
	}

	// 5. 成功ログ → 302 Redirect
	h.logger.InfoContext(ctx, "oauth authorize started",
		slog.String("user_id", userID),
		slog.String("provider", h.provider.Name()),
		slog.String("tenant", h.tenant),
	)
	stdhttp.Redirect(w, r, authURL, stdhttp.StatusFound)
}

// HandleCallback は /oauth/backlog/callback の GET ハンドラー。
//
// 処理フロー:
//  1. HTTP メソッドが GET であることを確認（それ以外は 405）
//  2. error クエリを優先判定（値があれば 400 provider_denied）
//  3. code / state の空値チェック（400 invalid_request）
//  4. auth.ValidateState で state JWT 検証
//     - ErrStateExpired → 400 state_expired
//     - ErrStateInvalid → 400 state_invalid
//  5. state claims の Tenant が h.tenant と一致することを検証（400 invalid_tenant）
//  6. context から userID を取得（未設定なら 401）し、state.UserID と一致を検証（401 user_mismatch）
//  7. provider.ExchangeCode で code → TokenRecord（失敗で 502 provider_error）
//  8. provider.GetCurrentUser でユーザー情報取得（失敗で 502 provider_error）
//  9. TokenRecord.UserID, ProviderUserID を補完
// 10. tokenManager.SaveToken で永続化（失敗で 500 internal_error）
// 11. 200 OK + JSON レスポンス
//
// セキュリティ: code / state JWT 生値 / access_token / refresh_token / stateSecret / error_description は一切ログに出さない。
// ログに出すのは user_id / provider / tenant / provider_user_id / reason / err_type のみ。
func (h *OAuthHandler) HandleCallback(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	// 1. メソッドチェック
	if r.Method != stdhttp.MethodGet {
		writeJSONError(w, stdhttp.StatusMethodNotAllowed, errCodeMethodNotAllowed, errMsgMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	// 2. error クエリ優先（ユーザー拒否 / プロバイダー側エラー）
	if provErr := q.Get("error"); provErr != "" {
		// error_description は PII リスクがあるためログに含めない（reason のみ）
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", errCodeProviderDenied),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeProviderDenied, errMsgProviderDenied)
		return
	}

	// 3. code / state 空値チェック
	code := q.Get("code")
	state := q.Get("state")
	if code == "" {
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", "missing_code"),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgMissingCode)
		return
	}
	if state == "" {
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", "missing_state"),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidRequest, errMsgMissingState)
		return
	}

	// 4. state JWT 検証
	claims, err := auth.ValidateState(state, h.stateSecret)
	if err != nil {
		if errors.Is(err, auth.ErrStateExpired) {
			h.logger.WarnContext(ctx, "oauth callback rejected",
				slog.String("reason", errCodeStateExpired),
			)
			writeJSONError(w, stdhttp.StatusBadRequest, errCodeStateExpired, errMsgStateExpired)
			return
		}
		// ErrStateInvalid またはその他
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", errCodeStateInvalid),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeStateInvalid, errMsgStateInvalid)
		return
	}

	// 5. state.Tenant が handler の tenant と一致することを検証
	if claims.Tenant != h.tenant {
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", errCodeInvalidTenant),
			slog.String("user_id", claims.UserID),
		)
		writeJSONError(w, stdhttp.StatusBadRequest, errCodeInvalidTenant, errMsgInvalidTenant)
		return
	}

	// 6. ctx userID を取得し、state.UserID と一致を検証
	ctxUserID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", errCodeUnauthenticated),
		)
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUnauthenticated, errMsgCallbackUnauth)
		return
	}
	if ctxUserID != claims.UserID {
		h.logger.WarnContext(ctx, "oauth callback rejected",
			slog.String("reason", errCodeUserMismatch),
			slog.String("user_id", ctxUserID),
		)
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUserMismatch, errMsgUserMismatch)
		return
	}

	providerName := h.provider.Name()

	// 7. code → TokenRecord
	record, err := h.provider.ExchangeCode(ctx, code, h.redirectURI)
	if err != nil {
		// err.Error() の生値は upstream が token を echo する可能性があるためログに出さない。
		// 代わりに err_type を出す（M14 セキュリティ判断）。
		h.logCallbackFailed(ctx, "exchange_failed", err, ctxUserID, providerName)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgExchangeFailed)
		return
	}
	if record == nil {
		h.logCallbackFailed(ctx, "exchange_nil_record", nil, ctxUserID, providerName)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgExchangeFailed)
		return
	}

	// 8. AccessToken で GetCurrentUser
	providerUser, err := h.provider.GetCurrentUser(ctx, record.AccessToken)
	if err != nil {
		h.logCallbackFailed(ctx, "get_user_failed", err, ctxUserID, providerName)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgUserFetchFailed)
		return
	}
	if providerUser == nil {
		h.logCallbackFailed(ctx, "get_user_nil", nil, ctxUserID, providerName)
		writeJSONError(w, stdhttp.StatusBadGateway, errCodeProviderError, errMsgUserFetchFailed)
		return
	}

	// 9. identity fields を補完
	record.UserID = ctxUserID
	record.ProviderUserID = providerUser.ID
	// Provider / Tenant は ExchangeCode 側で設定されている想定だが、
	// 欠損があれば補完する（防御的）
	if record.Provider == "" {
		record.Provider = providerName
	}
	if record.Tenant == "" {
		record.Tenant = h.tenant
	}

	// 10. トークン永続化
	if err := h.tokenManager.SaveToken(ctx, record); err != nil {
		h.logCallbackFailed(ctx, "save_failed", err, ctxUserID, providerName)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgSaveFailed)
		return
	}

	// 11. 成功ログ → 200 JSON
	h.logger.InfoContext(ctx, "oauth callback success",
		slog.String("user_id", ctxUserID),
		slog.String("provider", providerName),
		slog.String("tenant", h.tenant),
		slog.String("provider_user_id", providerUser.ID),
	)

	writeJSONSuccess(w, stdhttp.StatusOK, callbackSuccessResponse{
		Status:           "connected",
		Provider:         providerName,
		Tenant:           h.tenant,
		ProviderUserID:   providerUser.ID,
		ProviderUserName: providerUser.Name,
	})
}

// logCallbackFailed は callback の失敗ログを出力する。
// err.Error() の生値は出力せず、型情報 (err_type) のみを出す。
// これは upstream provider エラーが access_token / refresh_token / code を
// echo するリスクを避けるための措置（M14 セキュリティ判断）。
func (h *OAuthHandler) logCallbackFailed(ctx context.Context, reason string, err error, userID, providerName string) {
	attrs := []any{
		slog.String("reason", reason),
		slog.String("user_id", userID),
		slog.String("provider", providerName),
		slog.String("tenant", h.tenant),
	}
	if err != nil {
		attrs = append(attrs, slog.String("err_type", fmt.Sprintf("%T", err)))
	}
	h.logger.ErrorContext(ctx, "oauth callback failed", attrs...)
}

// writeJSONError は統一 JSON エラーレスポンスを書き込む。
func writeJSONError(w stdhttp.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: code, Message: message})
}

// writeJSONSuccess は成功 JSON レスポンスを書き込む。
func writeJSONSuccess(w stdhttp.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// HandleStatus は /oauth/backlog/status の GET ハンドラー。
//
// 処理フロー:
//  1. HTTP メソッドが GET であることを確認（それ以外は 405）
//  2. context から userID を取得（なければ 401）
//  3. tokenManager.GetValidToken で接続状態を判定
//     - ErrProviderNotConnected → 200 {"connected": false}
//     - ErrTokenRefreshFailed / ErrTokenExpired → 200 {"connected": true, "needs_reauth": true, ...}
//     - その他エラー → 500 internal_error
//     - 成功 (record, nil) → 200 {"connected": true, ..., "provider_user_id": "..."}
//
// セキュリティ: access_token / refresh_token を一切レスポンス・ログに出さない。
// ログに出すのは user_id / provider / tenant / provider_user_id / outcome / reason / err_type のみ。
func (h *OAuthHandler) HandleStatus(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	// 1. メソッドチェック
	if r.Method != stdhttp.MethodGet {
		writeJSONError(w, stdhttp.StatusMethodNotAllowed, errCodeMethodNotAllowed, errMsgMethodNotAllowed)
		return
	}

	// 2. userID 取得
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		h.logger.WarnContext(ctx, "oauth status rejected",
			slog.String("reason", errCodeUnauthenticated),
		)
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUnauthenticated, errMsgStatusUnauth)
		return
	}

	providerName := h.provider.Name()

	// 3. GetValidToken で接続状態を判定
	record, err := h.tokenManager.GetValidToken(ctx, userID, providerName, h.tenant)
	switch {
	case err == nil:
		// 接続済み・有効
		h.logStatusResult(ctx, "connected", userID, providerName)
		writeJSONSuccess(w, stdhttp.StatusOK, statusResponse{
			Connected:      true,
			Provider:       providerName,
			Tenant:         h.tenant,
			ProviderUserID: record.ProviderUserID,
		})
		return

	case errors.Is(err, auth.ErrProviderNotConnected):
		// 未接続
		h.logStatusResult(ctx, "not_connected", userID, providerName)
		writeJSONSuccess(w, stdhttp.StatusOK, statusResponse{
			Connected: false,
		})
		return

	case errors.Is(err, auth.ErrTokenRefreshFailed), errors.Is(err, auth.ErrTokenExpired):
		// 再認可が必要
		h.logStatusResult(ctx, "needs_reauth", userID, providerName)
		writeJSONSuccess(w, stdhttp.StatusOK, statusResponse{
			Connected:   true,
			NeedsReauth: true,
			Provider:    providerName,
			Tenant:      h.tenant,
		})
		return

	default:
		// その他の内部エラー。err.Error() は upstream token を echo する可能性があるため出さない。
		h.logStatusFailed(ctx, "store_error", err, userID, providerName)
		writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgStatusInternal)
		return
	}
}

// HandleDisconnect は /oauth/backlog/disconnect の DELETE ハンドラー。
//
// 処理フロー:
//  1. HTTP メソッドが DELETE であることを確認（それ以外は 405）
//  2. context から userID を取得（なければ 401）
//  3. tokenManager.RevokeToken を実行
//     - 成功 → 200 {"status":"disconnected", ...}
//     - ErrProviderNotConnected → 200 {"status":"disconnected", ...}（冪等扱い）
//     - その他エラー → 500 internal_error
//
// セキュリティ: access_token / refresh_token を一切レスポンス・ログに出さない。
func (h *OAuthHandler) HandleDisconnect(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	// 1. メソッドチェック
	if r.Method != stdhttp.MethodDelete {
		writeJSONError(w, stdhttp.StatusMethodNotAllowed, errCodeMethodNotAllowed, errMsgMethodNotAllowed)
		return
	}

	// 2. userID 取得
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		h.logger.WarnContext(ctx, "oauth disconnect rejected",
			slog.String("reason", errCodeUnauthenticated),
		)
		writeJSONError(w, stdhttp.StatusUnauthorized, errCodeUnauthenticated, errMsgDisconnectUnauth)
		return
	}

	providerName := h.provider.Name()

	// 3. RevokeToken を実行
	if err := h.tokenManager.RevokeToken(ctx, userID, providerName, h.tenant); err != nil {
		// 存在しないレコードの削除は冪等として成功扱い
		if !errors.Is(err, auth.ErrProviderNotConnected) {
			h.logDisconnectFailed(ctx, "revoke_failed", err, userID, providerName)
			writeJSONError(w, stdhttp.StatusInternalServerError, errCodeInternalError, errMsgDisconnectFailed)
			return
		}
	}

	// 4. 成功ログ → 200 JSON
	h.logDisconnectSuccess(ctx, userID, providerName)
	writeJSONSuccess(w, stdhttp.StatusOK, disconnectResponse{
		Status:   "disconnected",
		Provider: providerName,
		Tenant:   h.tenant,
	})
}

// logStatusResult は /status の成功系ログを統一フォーマットで出力する。
// outcome は "connected" / "not_connected" / "needs_reauth" のいずれか。
func (h *OAuthHandler) logStatusResult(ctx context.Context, outcome string, userID, providerName string) {
	h.logger.InfoContext(ctx, "oauth status checked",
		slog.String("outcome", outcome),
		slog.String("user_id", userID),
		slog.String("provider", providerName),
		slog.String("tenant", h.tenant),
	)
}

// logStatusFailed は /status の失敗ログを出力する。
// err.Error() の生値は出力せず、型情報 (err_type) のみを出す（M14 の方針を継承）。
func (h *OAuthHandler) logStatusFailed(ctx context.Context, reason string, err error, userID, providerName string) {
	attrs := []any{
		slog.String("reason", reason),
		slog.String("user_id", userID),
		slog.String("provider", providerName),
		slog.String("tenant", h.tenant),
	}
	if err != nil {
		attrs = append(attrs, slog.String("err_type", fmt.Sprintf("%T", err)))
	}
	h.logger.ErrorContext(ctx, "oauth status failed", attrs...)
}

// logDisconnectSuccess は /disconnect の成功ログを出力する。
func (h *OAuthHandler) logDisconnectSuccess(ctx context.Context, userID, providerName string) {
	h.logger.InfoContext(ctx, "oauth disconnect success",
		slog.String("user_id", userID),
		slog.String("provider", providerName),
		slog.String("tenant", h.tenant),
	)
}

// logDisconnectFailed は /disconnect の失敗ログを出力する。
// err.Error() の生値は出力せず、型情報 (err_type) のみを出す。
func (h *OAuthHandler) logDisconnectFailed(ctx context.Context, reason string, err error, userID, providerName string) {
	attrs := []any{
		slog.String("reason", reason),
		slog.String("user_id", userID),
		slog.String("provider", providerName),
		slog.String("tenant", h.tenant),
	}
	if err != nil {
		attrs = append(attrs, slog.String("err_type", fmt.Sprintf("%T", err)))
	}
	h.logger.ErrorContext(ctx, "oauth disconnect failed", attrs...)
}
