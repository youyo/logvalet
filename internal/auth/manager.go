package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// defaultRefreshMargin はトークンリフレッシュのデフォルト安全マージン。
// 有効期限の 5 分前にリフレッシュを開始する。
const defaultRefreshMargin = 5 * time.Minute

// TokenRefresher はトークンリフレッシュ機能のみを抽象化する。
//
// auth/provider.OAuthProvider の部分集合であり、
// auth → auth/provider の循環依存を回避するために auth パッケージに定義する。
// Go の構造的部分型により provider.BacklogOAuthProvider は自動的にこの interface を満たす。
type TokenRefresher interface {
	// Name はプロバイダー名を返す（例: "backlog"）。
	Name() string

	// RefreshToken はリフレッシュトークンで新しいトークンを取得する。
	// 返り値の TokenRecord には UserID と ProviderUserID は設定されない（caller 側で設定すること）。
	RefreshToken(ctx context.Context, refreshToken string) (*TokenRecord, error)
}

// TokenManager は TokenStore と TokenRefresher を組み合わせて、
// 有効なトークンの取得・保存・失効を行う。
type TokenManager interface {
	// GetValidToken はユーザー・プロバイダー・テナントに対応する有効なトークンを返す。
	// トークンが期限切れ間近の場合は自動的にリフレッシュする。
	// レコード未存在の場合は ErrProviderNotConnected を返す。
	// リフレッシュ失敗の場合は ErrTokenRefreshFailed を返す。
	GetValidToken(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error)

	// SaveToken はトークンレコードを保存する。
	SaveToken(ctx context.Context, record *TokenRecord) error

	// RevokeToken はトークンレコードを削除する。
	RevokeToken(ctx context.Context, userID, provider, tenant string) error
}

// Option は tokenManager の構成オプション。
type Option func(*tokenManager)

// WithRefreshMargin はトークンリフレッシュの安全マージンを設定する。
// 未指定の場合はデフォルト 5 分。
func WithRefreshMargin(d time.Duration) Option {
	return func(tm *tokenManager) {
		tm.refreshMargin = d
	}
}

// tokenManager は TokenManager の実装。
type tokenManager struct {
	store         TokenStore
	providers     map[string]TokenRefresher
	refreshMargin time.Duration
}

// NewTokenManager は新しい TokenManager を返す。
func NewTokenManager(store TokenStore, providers map[string]TokenRefresher, opts ...Option) TokenManager {
	tm := &tokenManager{
		store:         store,
		providers:     providers,
		refreshMargin: defaultRefreshMargin,
	}
	for _, opt := range opts {
		opt(tm)
	}
	return tm
}

// GetValidToken はユーザー・プロバイダー・テナントに対応する有効なトークンを返す。
func (tm *tokenManager) GetValidToken(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error) {
	rec, err := tm.store.Get(ctx, userID, provider, tenant)
	if err != nil {
		return nil, fmt.Errorf("get token from store: %w", err)
	}
	if rec == nil {
		return nil, ErrProviderNotConnected
	}

	if !rec.NeedsRefresh(tm.refreshMargin) {
		return rec, nil
	}

	// プロバイダーが登録されているか確認
	refresher, ok := tm.providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered: %w", provider, ErrProviderNotConnected)
	}

	// リフレッシュ実行
	refreshed, err := refresher.RefreshToken(ctx, rec.RefreshToken)
	if err != nil {
		slog.Warn("token refresh failed",
			"provider", provider,
			"user_id", userID,
			"tenant", tenant,
			"error", err,
		)
		return nil, fmt.Errorf("refresh token for provider %q: %w", provider, ErrTokenRefreshFailed)
	}

	// identity fields をコピー（M05: RefreshToken は UserID/ProviderUserID を設定しない）
	refreshed.UserID = rec.UserID
	refreshed.Provider = rec.Provider
	refreshed.Tenant = rec.Tenant
	refreshed.ProviderUserID = rec.ProviderUserID
	refreshed.CreatedAt = rec.CreatedAt
	refreshed.UpdatedAt = time.Now()

	if err := tm.store.Put(ctx, refreshed); err != nil {
		return nil, fmt.Errorf("save refreshed token: %w", err)
	}

	slog.Info("token refreshed",
		"provider", provider,
		"user_id", userID,
		"tenant", tenant,
		"access_token", maskToken(refreshed.AccessToken),
	)

	return refreshed, nil
}

// SaveToken はトークンレコードを保存する。
func (tm *tokenManager) SaveToken(ctx context.Context, record *TokenRecord) error {
	return tm.store.Put(ctx, record)
}

// RevokeToken はトークンレコードを削除する。
func (tm *tokenManager) RevokeToken(ctx context.Context, userID, provider, tenant string) error {
	return tm.store.Delete(ctx, userID, provider, tenant)
}
