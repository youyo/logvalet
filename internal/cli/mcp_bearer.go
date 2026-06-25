package cli

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// bearerAuthMiddleware は静的Bearerトークンで認証するHTTPミドルウェア。
// sha256ハッシュ化+subtle.ConstantTimeCompareで長さ・内容ともにタイミング安全な比較を行う。
// RFC 7235に従いスキーム名はcase-insensitiveで比較する。
// トークン自体はcase-sensitiveで比較する（スキームのcase-insensitivityとは独立）。
func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
	hashed := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			lower := strings.ToLower(auth)
			// "bearer "は7文字。スキームはcase-insensitiveなので小文字に統一して判定する。
			// トークン部分はauth[7:]で取り出す（case-sensitiveなので元のauth文字列を使用）。
			const schemeLen = 7 // len("bearer ")
			if !strings.HasPrefix(lower, "bearer ") {
				unauthorized(w)
				return
			}
			// トークンはcase-sensitive比較のため元のauth文字列から取り出す
			provided := sha256.Sum256([]byte(auth[schemeLen:]))
			if subtle.ConstantTimeCompare(provided[:], hashed[:]) != 1 {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// unauthorized はWWW-Authenticateヘッダーを含む401レスポンスを返す。
// RFC 6750に従いBearer認証失敗時にWWW-Authenticateヘッダーを付与する。
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="logvalet"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized"}`))
}
