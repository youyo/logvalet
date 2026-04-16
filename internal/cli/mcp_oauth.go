package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	idproxy "github.com/youyo/idproxy"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/auth/provider"
	tokenstore "github.com/youyo/logvalet/internal/auth/tokenstore"
	httptransport "github.com/youyo/logvalet/internal/transport/http"
)

// OAuthDeps は OAuth モードに必要な依存をまとめた構造体。
// Run() 中で生成し、defer Close() でリソース解放する。
type OAuthDeps struct {
	Store        auth.TokenStore
	Provider     provider.OAuthProvider
	TokenManager auth.TokenManager
	Factory      auth.ClientFactory
	Handler      *httptransport.OAuthHandler
}

// Close は OAuthDeps の保持するリソース（主に TokenStore）を解放する。
// 冪等であり、nil レシーバや Store=nil のケースでも安全に呼べる。
func (d *OAuthDeps) Close() error {
	if d == nil || d.Store == nil {
		return nil
	}
	return d.Store.Close()
}

// BuildOAuthDeps は OAuth モード用の依存コンポーネントを一括構築する。
//
// 前提:
//   - cfg.OAuthEnabled() == true であること（caller で確認済み）
//   - cfg.Validate() が nil を返していること（caller で確認済み）
//   - space は Backlog スペース名（例: "example-space"）
//   - baseURL は Backlog API のベース URL（例: "https://example-space.backlog.com"）
//   - logger は nil でも可（handler 側で slog.Default() へフォールバック）
//
// 失敗条件:
//   - cfg が nil
//   - space が空
//   - hex.DecodeString(cfg.OAuthStateSecret) 失敗
//   - provider.NewBacklogOAuthProvider 失敗
//   - tokenstore.NewTokenStore 失敗
//   - httptransport.NewOAuthHandler 失敗（失敗時は store を Close する）
func BuildOAuthDeps(cfg *auth.OAuthEnvConfig, space, baseURL string, logger *slog.Logger) (*OAuthDeps, error) {
	if cfg == nil {
		return nil, fmt.Errorf("mcp: oauth config must not be nil")
	}
	if space == "" {
		return nil, fmt.Errorf("mcp: space must not be empty for OAuth mode")
	}

	secret, err := hex.DecodeString(cfg.OAuthStateSecret)
	if err != nil {
		return nil, fmt.Errorf("mcp: invalid OAuth state secret: %w", err)
	}

	// Provider
	p, err := provider.NewBacklogOAuthProvider(space, cfg.BacklogClientID, cfg.BacklogClientSecret)
	if err != nil {
		return nil, fmt.Errorf("mcp: build backlog provider: %w", err)
	}

	// Store
	store, err := tokenstore.NewTokenStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("mcp: build token store: %w", err)
	}

	// TokenManager
	providers := map[string]auth.TokenRefresher{p.Name(): p}
	tm := auth.NewTokenManager(store, providers)

	// ClientFactory
	factory := auth.NewClientFactory(tm, p.Name(), space, baseURL)

	// OAuthHandler
	handler, err := httptransport.NewOAuthHandler(p, tm, space, cfg.BacklogRedirectURL, secret, auth.DefaultStateTTL, logger)
	if err != nil {
		// 失敗時は store を閉じる（リソースリーク防止）
		_ = store.Close()
		return nil, fmt.Errorf("mcp: build oauth handler: %w", err)
	}

	return &OAuthDeps{
		Store:        store,
		Provider:     p,
		TokenManager: tm,
		Factory:      factory,
		Handler:      handler,
	}, nil
}

// InstallOAuthRoutes は OAuth ハンドラーの 4 メソッドを mux に登録する。
//
// 登録パス:
//   - GET    /oauth/backlog/authorize
//   - GET    /oauth/backlog/callback
//   - GET    /oauth/backlog/status
//   - DELETE /oauth/backlog/disconnect
//
// HTTP メソッドフィルタは各ハンドラー内で実施する（本関数では mux.HandleFunc でパスのみ登録）。
func InstallOAuthRoutes(mux *http.ServeMux, h *httptransport.OAuthHandler) {
	mux.HandleFunc("/oauth/backlog/authorize", h.HandleAuthorize)
	mux.HandleFunc("/oauth/backlog/callback", h.HandleCallback)
	mux.HandleFunc("/oauth/backlog/status", h.HandleStatus)
	mux.HandleFunc("/oauth/backlog/disconnect", h.HandleDisconnect)
}

// BridgeFromUserIDFn は、ctx から userID を取得する関数を受け取り、
// その値を auth.ContextWithUserID で再注入する HTTP ミドルウェアを返す。
//
// 通常は newUserIDBridge() から呼ばれ、idproxy の *User.Subject を
// auth.UserIDFromContext が参照する context key に橋渡しする。
//
// fn が nil、または fn(ctx) が空文字列の場合は何もせず pass-through する
// （後段ハンドラーが 401 を返す）。
func BridgeFromUserIDFn(fn func(ctx context.Context) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if fn != nil {
				if uid := fn(r.Context()); uid != "" {
					r = r.WithContext(auth.ContextWithUserID(r.Context(), uid))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newUserIDBridge は idproxy の *User.Subject を auth.UserIDFromContext が
// 参照する context key に橋渡しするミドルウェアを返す。
//
// 配置: auth.Wrap の内側（innerMux を wrap する手前）。
// idproxy の外側に置くと、idproxy が context に注入する前に bridge が動き無効化する。
func newUserIDBridge() func(http.Handler) http.Handler {
	return BridgeFromUserIDFn(idproxyUserID)
}

// idproxyUserID は idproxy.UserFromContext から Subject を取り出すヘルパー。
// ユーザー未注入または Subject が空の場合は "" を返す。
func idproxyUserID(ctx context.Context) string {
	if u := idproxy.UserFromContext(ctx); u != nil {
		return u.Subject
	}
	return ""
}
