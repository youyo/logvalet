package http

import (
	"encoding/json"
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

// エラーコード定数（JSON レスポンスの "error" フィールド値）。
const (
	errCodeUnauthenticated   = "unauthenticated"
	errCodeMethodNotAllowed  = "method_not_allowed"
	errCodeInternalError     = "internal_error"
	errMsgUnauthenticated    = "user ID is required to start OAuth flow"
	errMsgMethodNotAllowed   = "only GET is allowed"
	errMsgInternalError      = "failed to start OAuth flow"
)

// OAuthHandler は Backlog OAuth フロー用の HTTP ハンドラーを提供する。
// ハンドラーは MCP サーバーに組み込まれる想定で、ルーティングは M16 で行う。
//
// tenant フィールドは BacklogOAuthProvider.space と同一値（例: "example-space"）
// である必要がある。M14 callback ハンドラーで state claims の tenant と
// TokenRecord.Tenant を比較するため、ここで不一致があると callback が失敗する。
type OAuthHandler struct {
	provider    provider.OAuthProvider
	tenant      string
	redirectURI string
	stateSecret []byte
	stateTTL    time.Duration
	logger      *slog.Logger
}

// NewOAuthHandler は OAuthHandler を構築する。
//
// provider が nil の場合は panic する（programming error として早期検出）。
// tenant が空の場合は auth.ErrInvalidTenant を返す。
// redirectURI が空の場合は auth.ErrInvalidRedirectURI を返す。
// stateSecret が nil または空の場合は auth.ErrStateInvalid を返す。
// stateTTL が 0 以下の場合は auth.ErrStateInvalid を返す。
// logger が nil の場合は slog.Default() を使用する。
func NewOAuthHandler(
	p provider.OAuthProvider,
	tenant, redirectURI string,
	stateSecret []byte,
	stateTTL time.Duration,
	logger *slog.Logger,
) (*OAuthHandler, error) {
	if p == nil {
		panic("http: NewOAuthHandler: provider must not be nil")
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
		provider:    p,
		tenant:      tenant,
		redirectURI: redirectURI,
		stateSecret: stateSecret,
		stateTTL:    stateTTL,
		logger:      logger,
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

// writeJSONError は統一 JSON エラーレスポンスを書き込む。
func writeJSONError(w stdhttp.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: code, Message: message})
}
