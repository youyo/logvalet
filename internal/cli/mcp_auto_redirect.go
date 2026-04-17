package cli

import (
	"errors"
	"net/http"
	"strings"

	"github.com/youyo/logvalet/internal/auth"
)

// EnsureBacklogConnected はブラウザアクセス時に Backlog 未接続ユーザーを
// Backlog OAuth 認可エンドポイントへ 302 リダイレクトするミドルウェアを返す。
//
// 以下の条件を ALL 満たす場合のみ 302 を返す:
//  1. method が GET または HEAD
//  2. Accept ヘッダーに "text/html" を含む
//  3. auth.UserIDFromContext で userID 取得成功
//  4. パス prefix が "/oauth/backlog/" でない（無限ループ防止）
//  5. パスが "/mcp" 完全一致または "/mcp/" prefix でない（MCP クライアント保護）
//  6. tokenManager.GetValidToken が ErrProviderNotConnected / ErrTokenRefreshFailed /
//     ErrTokenExpired のいずれかを返す
//
// 引数:
//
//	tm           - Backlog トークンの接続状態を問い合わせる TokenManager
//	providerName - "backlog" 固定（auth/provider.backlogProviderName）
//	tenant       - Backlog スペース名（Profile.Space）
//	authorizeURL - "<externalURL>/oauth/backlog/authorize" の完全修飾 URL
func EnsureBacklogConnected(
	tm auth.TokenManager,
	providerName, tenant, authorizeURL string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 条件 1: method が GET または HEAD のみ対象
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 2: Accept ヘッダーに "text/html" を含む
			if !strings.Contains(r.Header.Get("Accept"), "text/html") {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 3: userID が context に存在する
			userID, ok := auth.UserIDFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 4: /oauth/backlog/ prefix は除外（無限ループ防止）
			if strings.HasPrefix(r.URL.Path, "/oauth/backlog/") {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 5: /mcp 完全一致または /mcp/ prefix は除外（MCP クライアント保護）
			if r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/") {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 6: トークン状態を確認
			_, err := tm.GetValidToken(r.Context(), userID, providerName, tenant)
			if needsBacklogAuthorization(err) {
				http.Redirect(w, r, authorizeURL, http.StatusFound)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// needsBacklogAuthorization は GetValidToken のエラーが Backlog 認可を要求するものかを判定する。
// ホワイトリスト方式で判定し、予期しないエラー（ErrUnauthenticated 等）は対象外とする。
func needsBacklogAuthorization(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, auth.ErrProviderNotConnected) ||
		errors.Is(err, auth.ErrTokenRefreshFailed) ||
		errors.Is(err, auth.ErrTokenExpired)
}
