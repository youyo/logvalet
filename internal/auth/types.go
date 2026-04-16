package auth

import (
	"fmt"
	"time"
)

// TokenRecord は OAuth アクセストークンとリフレッシュトークンを保持する。
// プロバイダーごと・ユーザーごとに1レコード存在する。
type TokenRecord struct {
	UserID         string
	Provider       string
	Tenant         string
	AccessToken    string
	RefreshToken   string
	TokenType      string
	Scope          string
	Expiry         time.Time
	ProviderUserID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ProviderUser は OAuth プロバイダーから取得したユーザー情報を保持する。
// 構造は M05 で Backlog /users/myself のレスポンスに合わせて調整する。
type ProviderUser struct {
	ID    string
	Name  string
	Email string
}

// maskToken はトークン文字列を安全にマスクする。
//
// - 8文字以上: 先頭2文字 + "..." + 末尾2文字
// - 7文字以下または空: "***"
func maskToken(s string) string {
	if len(s) >= 8 {
		return s[:2] + "..." + s[len(s)-2:]
	}
	return "***"
}

// String はトークンをマスクした TokenRecord の文字列表現を返す。
// ログ出力時に生のトークンが漏洩しないよう AccessToken と RefreshToken はマスクされる。
func (r TokenRecord) String() string {
	return fmt.Sprintf(
		"TokenRecord{UserID:%s Provider:%s Tenant:%s AccessToken:%s RefreshToken:%s TokenType:%s Scope:%s Expiry:%s}",
		r.UserID,
		r.Provider,
		r.Tenant,
		maskToken(r.AccessToken),
		maskToken(r.RefreshToken),
		r.TokenType,
		r.Scope,
		r.Expiry.Format(time.RFC3339),
	)
}

// IsExpired はトークンが期限切れかどうかを返す。
//
// Expiry がゼロ値の場合は期限切れとして扱う。
func (r TokenRecord) IsExpired() bool {
	if r.Expiry.IsZero() {
		return true
	}
	return time.Now().After(r.Expiry)
}

// NeedsRefresh は現在時刻から margin を加えた時点で期限切れになるかどうかを返す。
// トークンが既に期限切れの場合も true を返す。
func (r TokenRecord) NeedsRefresh(margin time.Duration) bool {
	if r.Expiry.IsZero() {
		return true
	}
	return time.Now().Add(margin).After(r.Expiry)
}
