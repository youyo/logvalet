package auth

import "context"

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
