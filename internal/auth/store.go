package auth

import "context"

// StoreKey は TokenStore で使用する複合キーを生成する。
// キーは userID:provider:tenant の形式で、ユーザー・プロバイダー・テナントの
// 組み合わせごとに一意になる。
func StoreKey(userID, provider, tenant string) string {
	return userID + ":" + provider + ":" + tenant
}

// TokenStore はユーザーごとの OAuth トークンレコードの永続化を抽象化する。
//
// 実装は memory / sqlite / dynamodb の3種類を提供する。
// interface は消費者側（auth パッケージ）に配置し、実装は tokenstore サブパッケージに置く。
// これにより auth → tokenstore の循環依存を回避する。
type TokenStore interface {
	// Get は指定されたキーに対応する TokenRecord を返す。
	// レコードが存在しない場合は nil, nil を返す（エラーではない）。
	Get(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error)

	// Put はレコードを保存する。同じキーが存在する場合は上書き（upsert）する。
	Put(ctx context.Context, record *TokenRecord) error

	// Delete は指定されたキーのレコードを削除する。
	// レコードが存在しない場合もエラーにならない。
	Delete(ctx context.Context, userID, provider, tenant string) error

	// Close はストアを閉じてリソースを解放する。冪等であること。
	Close() error
}
