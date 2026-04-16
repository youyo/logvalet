// Package provider は外部 OAuth プロバイダーとのやりとりを抽象化する。
// Backlog, GitHub, Google 等の provider を統一的に扱うための interface を定義する。
package provider

import (
	"context"

	"github.com/youyo/logvalet/internal/auth"
)

// OAuthProvider は外部 OAuth プロバイダーとのやりとりを抽象化する。
// 各プロバイダー（Backlog, GitHub, Google 等）はこの interface を実装する。
type OAuthProvider interface {
	// Name はプロバイダー名を返す（例: "backlog"）。
	// TokenStore のキーやログ出力に使用される。
	Name() string

	// BuildAuthorizationURL は OAuth 認可 URL を構築する。
	// state は CSRF 防止用の signed state トークン。
	// redirectURI は OAuth コールバック URI。
	BuildAuthorizationURL(state, redirectURI string) (string, error)

	// ExchangeCode は認可コードをトークンに交換する。
	// 返り値の TokenRecord には UserID と ProviderUserID は設定されない（caller 側で設定すること）。
	ExchangeCode(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error)

	// RefreshToken はリフレッシュトークンで新しいトークンを取得する。
	// 返り値の TokenRecord には UserID と ProviderUserID は設定されない（caller 側で設定すること）。
	RefreshToken(ctx context.Context, refreshToken string) (*auth.TokenRecord, error)

	// GetCurrentUser はアクセストークンで現在のユーザー情報を取得する。
	GetCurrentUser(ctx context.Context, accessToken string) (*auth.ProviderUser, error)
}
