// Package tokenstore は auth.TokenStore interface の具象実装を提供する。
package tokenstore

import (
	"context"
	"sync"

	"github.com/youyo/logvalet/internal/auth"
)

// MemoryStore は sync.RWMutex + map によるインメモリ TokenStore 実装。
//
// 用途:
//   - 開発 / テスト / PoC
//   - Lambda 単一同時実行の簡易運用
//
// プロセス再起動でデータは消失する。本番の永続ストアではない。
type MemoryStore struct {
	mu     sync.RWMutex
	data   map[string]auth.TokenRecord
	closed bool
}

// NewMemoryStore は新しい MemoryStore を返す。
func NewMemoryStore() auth.TokenStore {
	return &MemoryStore{
		data: make(map[string]auth.TokenRecord),
	}
}

// Get は指定されたキーに対応する TokenRecord のコピーを返す。
// レコードが存在しない場合は nil, nil を返す。
func (m *MemoryStore) Get(_ context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := auth.StoreKey(userID, provider, tenant)
	rec, ok := m.data[key]
	if !ok {
		return nil, nil
	}

	// コピーを返す（呼び出し元からの変更を防ぐ）
	cp := rec
	return &cp, nil
}

// Put はレコードのコピーを保存する。同じキーが存在する場合は上書き（upsert）する。
func (m *MemoryStore) Put(_ context.Context, record *auth.TokenRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := auth.StoreKey(record.UserID, record.Provider, record.Tenant)
	// コピーを保存する（呼び出し元からの変更を防ぐ）
	m.data[key] = *record
	return nil
}

// Delete は指定されたキーのレコードを削除する。
// レコードが存在しない場合もエラーにならない。
func (m *MemoryStore) Delete(_ context.Context, userID, provider, tenant string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := auth.StoreKey(userID, provider, tenant)
	delete(m.data, key)
	return nil
}

// Close はストアを閉じてマップをクリアする。冪等であり、複数回呼んでもエラーにならない。
func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	clear(m.data)
	return nil
}
