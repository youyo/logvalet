# MS05: SQLite SpaceStore + NonceStore SQLite

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS01

## 目的

ローカル CLI 向け SQLite 実装を提供する。
NonceStore SQLite 実装も同一マイルストーンで実装する（同一 SQLite ファイル管理で一元化: RC2 対応）。

## 完了条件

- [ ] `internal/space/sqlitestore.go` — SQLiteStore（Store + NonceStore 実装）
- [ ] `internal/space/sqlitestore_test.go` — 全テストケース pass
- [ ] MemoryStore と同じ T1-T15 テストが SQLite でも pass
- [ ] NonceStore の二重消費がエラーになる
- [ ] `go test ./internal/space/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

MS02 で確立した T1-T15 を SQLiteStore でも実行する。
加えて SQLite 固有のテストを追加する。

### sqlitestore_test.go

```
T1-T12: MemoryStore と同一テストを SQLiteStore で実行（Store interface 共通テスト）
T13-T15: NonceStore テスト（MemoryStore と同一）

T16: TestSQLiteStore_AutoMigration
    - 新規 DB ファイルで NewSQLiteStore を呼ぶ
    - spaces, user_preferences, nonces テーブルが自動作成される

T17: TestSQLiteStore_Persistence
    - SQLiteStore を一度 Close して再 Open
    - Upsert したデータが再 Open 後も存在する

T18: TestSQLiteStore_Nonce_Expiry
    - Store("u1", "nonce1", 1*time.Millisecond) → 数 ms 待機
    - Consume("u1", "nonce1") → expires_at 超過なので ErrNonceAlreadyUsed
    （TTL はアプリ側で管理: expires_at を DB に保存し Consume 時に確認）

T19: TestSQLiteStore_Close_RejectsOps
    - Close() 後に Upsert → error
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/sqlitestore.go` | SQLiteStore（Store + NonceStore 実装） |
| `internal/space/sqlitestore_test.go` | T1-T19 |

---

## 3. テーブル設計

```sql
-- spaces テーブル
CREATE TABLE IF NOT EXISTS spaces (
    user_id         TEXT NOT NULL,
    alias           TEXT NOT NULL,
    tenant          TEXT NOT NULL,
    base_url        TEXT NOT NULL,
    auth_type       TEXT NOT NULL,
    auth_profile    TEXT NOT NULL DEFAULT '',
    provider        TEXT NOT NULL DEFAULT 'backlog',
    status          TEXT NOT NULL DEFAULT 'unknown',
    last_verified_at TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    disabled        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, alias)
);

CREATE INDEX IF NOT EXISTS idx_spaces_user_tenant ON spaces(user_id, tenant);

-- user_preferences テーブル
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id             TEXT PRIMARY KEY,
    default_space_alias TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

-- nonces テーブル（NonceStore SQLite 実装: RC2）
CREATE TABLE IF NOT EXISTS nonces (
    user_id    TEXT NOT NULL,
    nonce      TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (user_id, nonce)
);
```

---

## 4. 実装

### sqlitestore.go のシグネチャ

```go
package space

import (
    "database/sql"
    "sync"
    _ "modernc.org/sqlite"
)

// SQLiteStore は Store + NonceStore の SQLite 実装。
// modernc.org/sqlite（pure Go）を使用する（既存 auth/tokenstore/sqlite.go と同じドライバー）。
type SQLiteStore struct {
    db     *sql.DB
    mu     sync.RWMutex
    closed bool
}

func NewSQLiteStore(path string) (*SQLiteStore, error)
func (s *SQLiteStore) List(ctx context.Context, userID string) ([]SpaceRegistration, error)
func (s *SQLiteStore) Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error)
func (s *SQLiteStore) Upsert(ctx context.Context, reg *SpaceRegistration) error
func (s *SQLiteStore) Delete(ctx context.Context, userID, alias string) error
func (s *SQLiteStore) GetPreference(ctx context.Context, userID string) (*UserPreference, error)
func (s *SQLiteStore) PutPreference(ctx context.Context, pref *UserPreference) error
func (s *SQLiteStore) Close() error

// NonceStore 実装
func (s *SQLiteStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
func (s *SQLiteStore) Consume(ctx context.Context, userID, nonce string) error
```

---

## 5. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/sqlitestore_test.go` を作成（T1-T19）
2. `go test ./internal/space/...` → コンパイルエラー（SQLiteStore 未定義）

### Step 2: Green

1. `internal/space/sqlitestore.go` を実装
2. `go test ./internal/space/...` → 全テストパス

### Step 3: Refactor

- 既存 `internal/auth/tokenstore/sqlite.go` のパターンを踏襲してコードを整理
- upsert は `INSERT OR REPLACE` または `INSERT ... ON CONFLICT DO UPDATE SET` を使う

---

## 6. 実装の要点

### ファイルパス分離（C1 対応）

```text
TokenStore の SQLite: ~/.config/logvalet/tokens.db（既存）
SpaceStore の SQLite: LOGVALET_SPACE_SQLITE_PATH（デフォルト: ~/.config/logvalet/spaces.db）
```

2つは異なるファイル。同一ファイルへの混在は禁止。

### NonceStore.Consume の expiry チェック

```go
func (s *SQLiteStore) Consume(ctx context.Context, userID, nonce string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC().Format(time.RFC3339)
    result, err := s.db.ExecContext(ctx,
        `DELETE FROM nonces WHERE user_id=? AND nonce=? AND expires_at > ?`,
        userID, nonce, now,
    )
    if err != nil {
        return err
    }
    n, _ := result.RowsAffected()
    if n == 0 {
        return ErrNonceAlreadyUsed // 期限切れ or 存在しない
    }
    return nil
}
```

---

## 7. 検証コマンド

```bash
go test ./internal/space/... -v -run TestSQLite
go test -race ./internal/space/...
go build ./...
go vet ./...
```

---

## 8. 次のマイルストーン

MS05 + MS06 完了後 → MS07（SpaceStore 設定 validation）が着手可能。
