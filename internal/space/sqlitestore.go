package space

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure Go SQLite ドライバー
)

const createSpacesSQL = `
CREATE TABLE IF NOT EXISTS spaces (
    user_id          TEXT NOT NULL,
    alias            TEXT NOT NULL,
    tenant           TEXT NOT NULL,
    base_url         TEXT NOT NULL,
    auth_type        TEXT NOT NULL DEFAULT '',
    auth_profile     TEXT NOT NULL DEFAULT '',
    provider         TEXT NOT NULL DEFAULT 'backlog',
    status           TEXT NOT NULL DEFAULT 'unknown',
    last_verified_at TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    disabled         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, alias)
);
CREATE INDEX IF NOT EXISTS idx_spaces_user_tenant ON spaces(user_id, tenant);`

const createUserPrefsSQL = `
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id             TEXT PRIMARY KEY,
    default_space_alias TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);`

const createNoncesSQL = `
CREATE TABLE IF NOT EXISTS nonces (
    user_id    TEXT NOT NULL,
    nonce      TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (user_id, nonce)
);`

var errSQLiteStoreClosed = errors.New("sqlite space store: store is closed")

// SQLiteStore は Store + NonceStore の SQLite 実装。
// modernc.org/sqlite（pure Go）を使用する。
type SQLiteStore struct {
	db     *sql.DB
	mu     sync.Mutex
	closed bool
}

// NewSQLiteStore は指定パスに SQLite DB を開き、必要なテーブルを自動作成して返す。
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite space store: open: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite space store: ping: %w", err)
	}

	// WAL モードで並行読み取りを改善し、単一接続でライタ競合を防ぐ
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite space store: set WAL mode: %w", err)
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite space store: set busy_timeout: %w", err)
	}

	for _, ddl := range []string{createSpacesSQL, createUserPrefsSQL, createNoncesSQL} {
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite space store: migrate: %w", err)
		}
	}

	// 起動時に期限切れ nonce を削除（エラーは無視してサービスを継続）
	if _, err := db.ExecContext(ctx, "DELETE FROM nonces WHERE expires_at < datetime('now')"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite space store: gc expired nonces: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// List は指定ユーザーの全 SpaceRegistration を返す。
func (s *SQLiteStore) List(ctx context.Context, userID string) ([]SpaceRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errSQLiteStoreClosed
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, alias, tenant, base_url, auth_type, auth_profile, provider,
		        status, last_verified_at, created_at, updated_at, disabled
		 FROM spaces WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("sqlite space store: list: %w", err)
	}
	defer rows.Close()

	result := make([]SpaceRegistration, 0)
	for rows.Next() {
		r, err := scanSpaceRegistration(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite space store: list scan: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Get は指定キーの SpaceRegistration を返す。存在しない場合は nil, nil。
func (s *SQLiteStore) Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errSQLiteStoreClosed
	}

	row := s.db.QueryRowContext(ctx,
		`SELECT user_id, alias, tenant, base_url, auth_type, auth_profile, provider,
		        status, last_verified_at, created_at, updated_at, disabled
		 FROM spaces WHERE user_id = ? AND alias = ?`, userID, alias)

	r, err := scanSpaceRegistration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite space store: get: %w", err)
	}
	return &r, nil
}

// Upsert は SpaceRegistration を保存（挿入または更新）する。
func (s *SQLiteStore) Upsert(ctx context.Context, reg *SpaceRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errSQLiteStoreClosed
	}

	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := now
	if !reg.CreatedAt.IsZero() {
		createdAt = reg.CreatedAt.UTC().Format(time.RFC3339)
	}
	updatedAt := now
	if !reg.UpdatedAt.IsZero() {
		updatedAt = reg.UpdatedAt.UTC().Format(time.RFC3339)
	}

	lastVerifiedAt := ""
	if !reg.LastVerifiedAt.IsZero() {
		lastVerifiedAt = reg.LastVerifiedAt.UTC().Format(time.RFC3339)
	}

	disabled := 0
	if reg.Disabled {
		disabled = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO spaces (
			user_id, alias, tenant, base_url, auth_type, auth_profile, provider,
			status, last_verified_at, created_at, updated_at, disabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, alias) DO UPDATE SET
			tenant           = excluded.tenant,
			base_url         = excluded.base_url,
			auth_type        = excluded.auth_type,
			auth_profile     = excluded.auth_profile,
			provider         = excluded.provider,
			status           = excluded.status,
			last_verified_at = excluded.last_verified_at,
			updated_at       = excluded.updated_at,
			disabled         = excluded.disabled`,
		reg.UserID, reg.Alias, reg.Tenant, reg.BaseURL,
		string(reg.AuthType), reg.AuthProfile,
		reg.Provider, string(reg.Status), lastVerifiedAt,
		createdAt, updatedAt, disabled,
	)
	if err != nil {
		return fmt.Errorf("sqlite space store: upsert: %w", err)
	}
	return nil
}

// Delete は指定キーのレコードを削除する。存在しなくてもエラーにならない。
func (s *SQLiteStore) Delete(ctx context.Context, userID, alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errSQLiteStoreClosed
	}

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM spaces WHERE user_id = ? AND alias = ?`, userID, alias)
	if err != nil {
		return fmt.Errorf("sqlite space store: delete: %w", err)
	}
	return nil
}

// GetPreference は指定ユーザーの UserPreference を返す。未設定なら nil, nil。
func (s *SQLiteStore) GetPreference(ctx context.Context, userID string) (*UserPreference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errSQLiteStoreClosed
	}

	var (
		pref                   UserPreference
		createdAtStr, updatedAtStr string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, default_space_alias, created_at, updated_at
		 FROM user_preferences WHERE user_id = ?`, userID,
	).Scan(&pref.UserID, &pref.DefaultSpaceAlias, &createdAtStr, &updatedAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite space store: get preference: %w", err)
	}

	var parseErr error
	pref.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("sqlite space store: parse created_at: %w", parseErr)
	}
	pref.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("sqlite space store: parse updated_at: %w", parseErr)
	}
	return &pref, nil
}

// PutPreference は UserPreference を保存（挿入または更新）する。
func (s *SQLiteStore) PutPreference(ctx context.Context, pref *UserPreference) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errSQLiteStoreClosed
	}

	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := now
	if !pref.CreatedAt.IsZero() {
		createdAt = pref.CreatedAt.UTC().Format(time.RFC3339)
	}
	updatedAt := now
	if !pref.UpdatedAt.IsZero() {
		updatedAt = pref.UpdatedAt.UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, default_space_alias, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			default_space_alias = excluded.default_space_alias,
			updated_at          = excluded.updated_at`,
		pref.UserID, pref.DefaultSpaceAlias, createdAt, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite space store: put preference: %w", err)
	}
	return nil
}

// Close はストアを閉じる。冪等。
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("sqlite space store: close: %w", err)
	}
	return nil
}

// Store は nonce を ttl 付きで保存する。
func (s *SQLiteStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errSQLiteStoreClosed
	}

	expiresAt := time.Now().Add(ttl).UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nonces (user_id, nonce, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, nonce) DO UPDATE SET expires_at = excluded.expires_at`,
		userID, nonce, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite space store: nonce store: %w", err)
	}
	return nil
}

// Consume は nonce を1回限り消費する。期限切れまたは存在しない場合は ErrNonceAlreadyUsed。
func (s *SQLiteStore) Consume(ctx context.Context, userID, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errSQLiteStoreClosed
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM nonces WHERE user_id = ? AND nonce = ? AND expires_at > ?`,
		userID, nonce, now,
	)
	if err != nil {
		return fmt.Errorf("sqlite space store: nonce consume: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNonceAlreadyUsed
	}
	return nil
}

// scanner は *sql.Row と *sql.Rows の共通インターフェース。
type scanner interface {
	Scan(dest ...any) error
}

func scanSpaceRegistration(s scanner) (SpaceRegistration, error) {
	var (
		r                                         SpaceRegistration
		authType, status                          string
		lastVerifiedAtStr, createdAtStr, updatedAtStr string
		disabled                                  int
	)
	err := s.Scan(
		&r.UserID, &r.Alias, &r.Tenant, &r.BaseURL,
		&authType, &r.AuthProfile, &r.Provider,
		&status, &lastVerifiedAtStr, &createdAtStr, &updatedAtStr, &disabled,
	)
	if err != nil {
		return SpaceRegistration{}, err
	}

	r.AuthType = AuthType(authType)
	r.Status = SpaceStatus(status)
	r.Disabled = disabled != 0

	if lastVerifiedAtStr != "" {
		t, err := time.Parse(time.RFC3339, lastVerifiedAtStr)
		if err != nil {
			return SpaceRegistration{}, fmt.Errorf("parse last_verified_at: %w", err)
		}
		r.LastVerifiedAt = t
	}

	r.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return SpaceRegistration{}, fmt.Errorf("parse created_at: %w", err)
	}
	r.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return SpaceRegistration{}, fmt.Errorf("parse updated_at: %w", err)
	}

	return r, nil
}
