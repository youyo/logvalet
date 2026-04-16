package tokenstore

import (
	"fmt"

	"github.com/youyo/logvalet/internal/auth"
)

// NewTokenStore は OAuthEnvConfig.TokenStoreType に基づいて適切な TokenStore 実装を返す。
//
// サポートする種別:
//   - StoreTypeMemory (デフォルト): MemoryStore
//   - StoreTypeSQLite: 未実装 (M08 で実装予定)
//   - StoreTypeDynamoDB: 未実装 (M09 で実装予定)
//
// cfg が nil の場合はデフォルト（memory）として扱う。
func NewTokenStore(cfg *auth.OAuthEnvConfig) (auth.TokenStore, error) {
	if cfg == nil {
		return NewMemoryStore(), nil
	}

	switch cfg.TokenStoreType {
	case auth.StoreTypeMemory, "":
		return NewMemoryStore(), nil
	case auth.StoreTypeSQLite:
		return NewSQLiteStore(cfg.SQLitePath)
	case auth.StoreTypeDynamoDB:
		return nil, fmt.Errorf("dynamodb token store: %w", auth.ErrNotImplemented)
	default:
		return nil, fmt.Errorf("token store type %q: %w", cfg.TokenStoreType, auth.ErrInvalidStoreType)
	}
}
