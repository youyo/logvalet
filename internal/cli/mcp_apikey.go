package cli

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// apiKeyHeaderName は Gateway → logvalet の service-to-service 認証に使うヘッダー名。
// docs/specs/gateway-request-contract.md §1.2 で確定した契約であり、Authorization ヘッダーは
// Backlog credential の Bearer passthrough 専用に予約されているため apikey には使わない。
const apiKeyHeaderName = "X-Logvalet-Api-Key"

// apiKeyAuthMiddleware は静的 apikey で Gateway を認証する HTTP ミドルウェア。
// apikey は Gateway を認証する共有鍵であってエンドユーザーを認証しない（契約 §1.1）。
//
// ヘッダー値はスキームプレフィックスを持たない生のトークンで、大小文字を区別する完全一致比較を
// 行う（契約 §1.3）。sha256 ハッシュ化 + subtle.ConstantTimeCompare により長さ・内容ともに
// タイミング安全に比較する。
func apiKeyAuthMiddleware(key string) func(http.Handler) http.Handler {
	hashed := sha256.Sum256([]byte(key))
	// 鍵未設定のまま apikey モードへ到達した場合は全リクエストを拒否する（fail-closed）。
	// ハッシュ比較だけだと空ヘッダーが空鍵と一致して無認証で素通りするため明示的に落とす。
	configured := key != ""
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !configured {
				unauthorized(w)
				return
			}
			provided := sha256.Sum256([]byte(r.Header.Get(apiKeyHeaderName)))
			if subtle.ConstantTimeCompare(provided[:], hashed[:]) != 1 {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// unauthorized は 401 レスポンスを返す。
// Bearer スキームではないため WWW-Authenticate ヘッダーは付与しない（契約 §1.4、RFC 6750 対象外）。
// ボディは spec §9 のエラーエンベロープ形式。
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"schema_version":"1","error":{"code":"unauthorized","message":"Missing or invalid X-Logvalet-Api-Key header.","retryable":false}}`))
}
