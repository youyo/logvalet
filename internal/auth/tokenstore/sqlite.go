package tokenstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/auth"

	_ "modernc.org/sqlite" // pure Go SQLite ドライバー
)

// createTableSQL は oauth_tokens テーブルを自動作成する DDL。
const createTableSQL = `
CREATE TABLE IF NOT EXISTS oauth_tokens (
  user_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  tenant TEXT NOT NULL,
  access_token TEXT NOT NULL,
  refresh_token TEXT NOT NULL,
  token_type TEXT,
  scope TEXT,
  expiry TEXT NOT NULL,
  provider_user_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, provider, tenant)
);`

// upsertSQL は INSERT ... ON CONFLICT ... DO UPDATE で upsert を行う。
// CreatedAt は新規挿入時のみ設定し、更新時は保持する。
const upsertSQL = `
INSERT INTO oauth_tokens (
  user_id, provider, tenant,
  access_token, refresh_token, token_type, scope,
  expiry, provider_user_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, provider, tenant) DO UPDATE SET
  access_token = excluded.access_token,
  refresh_token = excluded.refresh_token,
  token_type = excluded.token_type,
  scope = excluded.scope,
  expiry = excluded.expiry,
  provider_user_id = excluded.provider_user_id,
  updated_at = excluded.updated_at;`

// selectSQL は指定されたキーのレコードを取得する。
const selectSQL = `
SELECT user_id, provider, tenant,
       access_token, refresh_token, token_type, scope,
       expiry, provider_user_id, created_at, updated_at
FROM oauth_tokens
WHERE user_id = ? AND provider = ? AND tenant = ?;`

// deleteSQL は指定されたキーのレコードを削除する。
const deleteSQL = `
DELETE FROM oauth_tokens
WHERE user_id = ? AND provider = ? AND tenant = ?;`

// errStoreClosed は Close 済みの SQLiteStore への操作時に返されるエラー。
var errStoreClosed = errors.New("sqlite token store: store is closed")

// SQLiteStore は SQLite ベースの auth.TokenStore 実装。
//
// 用途:
//   - ローカル CLI
//   - 単一サーバー運用
//   - 自己ホスト
//
// modernc.org/sqlite（pure Go、CGO 不要）を使用する。
// Lambda の /tmp を永続ストアとして使用しないこと。
type SQLiteStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	closed bool
}

// NewSQLiteStore は新しい SQLiteStore を返す。
// dbPath に指定したパスに SQLite DB ファイルを作成（または開く）し、
// oauth_tokens テーブルが存在しなければ自動作成する。
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite token store: open: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite token store: ping: %w", err)
	}

	// WAL モードを有効化して並行アクセス性能を向上
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite token store: set WAL mode: %w", err)
	}

	// テーブル自動作成
	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite token store: create table: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Get は指定されたキーに対応する TokenRecord を返す。
// レコードが存在しない場合は nil, nil を返す（エラーではない）。
func (s *SQLiteStore) Get(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, errStoreClosed
	}

	var (
		rec                                      auth.TokenRecord
		expiryStr, createdAtStr, updatedAtStr    string
		providerUserID                           sql.NullString
		tokenType, scope                         sql.NullString
	)

	err := s.db.QueryRowContext(ctx, selectSQL, userID, provider, tenant).Scan(
		&rec.UserID,
		&rec.Provider,
		&rec.Tenant,
		&rec.AccessToken,
		&rec.RefreshToken,
		&tokenType,
		&scope,
		&expiryStr,
		&providerUserID,
		&createdAtStr,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite token store: get: %w", err)
	}

	// nullable フィールドの復元
	if tokenType.Valid {
		rec.TokenType = tokenType.String
	}
	if scope.Valid {
		rec.Scope = scope.String
	}
	if providerUserID.Valid {
		rec.ProviderUserID = providerUserID.String
	}

	// 時刻フィールドの復元
	rec.Expiry, err = time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return nil, fmt.Errorf("sqlite token store: parse expiry: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("sqlite token store: parse created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("sqlite token store: parse updated_at: %w", err)
	}

	return &rec, nil
}

// Put はレコードを保存する。同じキーが存在する場合は上書き（upsert）する。
// 時刻は UTC / RFC3339 形式で保存する。
func (s *SQLiteStore) Put(ctx context.Context, record *auth.TokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errStoreClosed
	}

	_, err := s.db.ExecContext(ctx, upsertSQL,
		record.UserID,
		record.Provider,
		record.Tenant,
		record.AccessToken,
		record.RefreshToken,
		record.TokenType,
		record.Scope,
		record.Expiry.UTC().Format(time.RFC3339),
		record.ProviderUserID,
		record.CreatedAt.UTC().Format(time.RFC3339),
		record.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite token store: put: %w", err)
	}

	return nil
}

// Delete は指定されたキーのレコードを削除する。
// レコードが存在しない場合もエラーにならない。
func (s *SQLiteStore) Delete(ctx context.Context, userID, provider, tenant string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errStoreClosed
	}

	_, err := s.db.ExecContext(ctx, deleteSQL, userID, provider, tenant)
	if err != nil {
		return fmt.Errorf("sqlite token store: delete: %w", err)
	}

	return nil
}

// Close はストアを閉じて DB 接続を解放する。冪等であり、複数回呼んでもエラーにならない。
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("sqlite token store: close: %w", err)
	}
	return nil
}
