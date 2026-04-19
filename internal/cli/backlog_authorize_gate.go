package cli

import (
	"net/http"
	"net/url"

	idproxy "github.com/youyo/idproxy"
	"github.com/youyo/logvalet/internal/auth"
)

// NewBacklogAuthorizeGate は idproxy の /authorize エンドポイントをフックする middleware を返す。
//
// 発火条件（ALL を満たすとき 302 を返す）:
//  1. メソッドが GET
//  2. パスが "/authorize" と完全一致
//  3. idproxy セッション Cookie を正常に復号できた（ユーザー識別子あり）
//  4. tm.GetValidToken が needsBacklogAuthorization == true のエラーを返す
//
// それ以外（他パス・他メソッド・セッションなし・Cookie 改ざん・接続済み・予期しないエラー）は
// 次のハンドラーに pass-through する。
//
// sm は idproxy.New が使うものと同じ Config（CookieSecret / Store）を共有した
// 別インスタンス。決定的に導出されるため相互復号可能。
//
// backlogAuthorizeURL は "<externalURL>/oauth/backlog/authorize" の完全修飾 URL。
func NewBacklogAuthorizeGate(
	sm *idproxy.SessionManager,
	tm auth.TokenManager,
	providerName, tenant, backlogAuthorizeURL string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 条件 1: GET のみ対象
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 2: "/authorize" 完全一致のみ対象（/authorize/foo は除外）
			if r.URL.Path != "/authorize" {
				next.ServeHTTP(w, r)
				return
			}

			// 条件 3: セッション復号（失敗・存在なし → pass-through）
			sess, err := sm.GetSessionFromRequest(r.Context(), r)
			if err != nil || sess == nil {
				next.ServeHTTP(w, r)
				return
			}

			userID := sess.User.Subject

			// 条件 4: Backlog 接続状態を確認
			_, tmErr := tm.GetValidToken(r.Context(), userID, providerName, tenant)
			if !needsBacklogAuthorization(tmErr) {
				next.ServeHTTP(w, r)
				return
			}

			// 発火: continue に現在の /authorize?... URL をセットして redirect
			continueTarget := r.URL.RequestURI()
			redirectURL := backlogAuthorizeURL + "?continue=" + url.QueryEscape(continueTarget)
			http.Redirect(w, r, redirectURL, http.StatusFound)
		})
	}
}
