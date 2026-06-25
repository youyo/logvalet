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
func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
	hashed := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			lower := strings.ToLower(auth)
			const prefix = "bearer "
			if !strings.HasPrefix(lower, prefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			// スキームは7文字固定（"bearer "）なのでトークン部分を取り出す
			provided := sha256.Sum256([]byte(auth[len(prefix):]))
			if subtle.ConstantTimeCompare(provided[:], hashed[:]) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
