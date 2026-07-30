package space

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrSpaceStoreTypeRequired は HTTP/Gateway モードで LOGVALET_SPACE_STORE_TYPE が
// 未設定、または memory が選択された場合に返される。stdio モードは memory 既定を
// 維持するため許容されるが、HTTP/Gateway モードは明示指定を必須とし fail-fast する。
var ErrSpaceStoreTypeRequired = errors.New(
	"space: LOGVALET_SPACE_STORE_TYPE must be set explicitly in HTTP/Gateway mode " +
		"(memory is not allowed; set sqlite or dynamodb)")

// RequireExplicitStoreType は HTTP/Gateway モード向けに、明示的な（memory 以外の）
// store type 指定を検証する。未設定または memory の場合は
// ErrSpaceStoreTypeRequired を返す。
func RequireExplicitStoreType(storeType string) error {
	normalized := strings.ToLower(strings.TrimSpace(storeType))
	if normalized == "" || StoreType(normalized) == StoreTypeMemory {
		return ErrSpaceStoreTypeRequired
	}
	return nil
}

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
