package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// NormalizeUserID は Gateway が伝達した Entra ID の issuer + subject の組を、
// logvalet 内部の userID 表現へ写す（gateway-request-contract.md §2.2）。
//
// per-user 紐付けは iss + sub の組で一意性を担保する設計であり、sub 単体では
// 複数テナント・複数 issuer 運用時に衝突しうる。2 値を NUL 区切りで連結した
// sha256 の16進表現を userID とすることで、境界のずれ（iss="https://ex/a",
// sub="b" と iss="https://ex/", sub="ab"）が同じ userID に落ちず、かつ userID の
// 長さ・文字種が常に一定（64 文字の16進）になりストアのキーやログへそのまま
// 載せても issuer URL / sub の生値が漏れない。
// 同じ iss+sub に対しては常に同じ値を返す。
func NormalizeUserID(issuer, subject string) string {
	sum := sha256.Sum256([]byte(issuer + "\x00" + subject))
	return hex.EncodeToString(sum[:])
}

// contextKey は context に格納する userID のキー型。
// unexported にすることで外部パッケージからの衝突を防ぐ。
type contextKey struct{}

// ContextWithUserID は context に userID を設定して返す。
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKey{}, userID)
}

// UserIDFromContext は context から userID を取得する。
// キーが存在しない場合、または空文字列の場合は ("", false) を返す。
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(contextKey{}).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
