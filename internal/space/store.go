package space

import (
	"context"
	"time"
)

// Store は SpaceRegistration と UserPreference の永続ストアインターフェース。
type Store interface {
	List(ctx context.Context, userID string) ([]SpaceRegistration, error)
	Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error)
	Upsert(ctx context.Context, reg *SpaceRegistration) error
	Delete(ctx context.Context, userID, alias string) error

	GetPreference(ctx context.Context, userID string) (*UserPreference, error)
	PutPreference(ctx context.Context, pref *UserPreference) error

	Close() error
}

// NonceStore は OAuth state の nonce を consume-once で管理するインターフェース。
// パッケージ配置: internal/space（循環依存防止のため internal/auth ではなく internal/space に置く: RH5）
// DynamoDB 実装: dynamodbstore.go、SQLite 実装: sqlitestore.go
type NonceStore interface {
	// Store は nonce を ttl 付きで保存する。
	Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
	// Consume は nonce を1回限り消費する。
	// 既に消費済みの場合は ErrNonceAlreadyUsed を返す（replay attack 防止）。
	Consume(ctx context.Context, userID, nonce string) error
}
