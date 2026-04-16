# M08: SQLite TokenStore 詳細計画

## 概要

SQLiteStore が auth.TokenStore interface を実装する。
modernc.org/sqlite（pure Go、CGO 不要）を使用し、ローカル CLI / 単一サーバー / 自己ホスト向けの永続 TokenStore を提供する。

## 対象ファイル

| ファイル | 操作 | 内容 |
|---------|------|------|
| `internal/auth/tokenstore/sqlite.go` | 新規作成 | SQLiteStore 実装 |
| `internal/auth/tokenstore/sqlite_test.go` | 新規作成 | TDD テスト |
| `internal/auth/tokenstore/factory.go` | 修正 | StoreTypeSQLite case を実装に差し替え |
| `internal/auth/tokenstore/factory_test.go` | 修正 | SQLite テストを NotImplemented → 成功テストに変更 |
| `go.mod` / `go.sum` | 修正 | modernc.org/sqlite 追加 |

## 追加依存

- `modernc.org/sqlite` — pure Go SQLite ドライバー（CGO 不要）
  - driver name: `"sqlite"`
  - `database/sql` 経由で使用

## DDL

```sql
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
);
```

## API

```go
func NewSQLiteStore(dbPath string) (*SQLiteStore, error)
func (s *SQLiteStore) Get(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error)
func (s *SQLiteStore) Put(ctx context.Context, record *auth.TokenRecord) error
func (s *SQLiteStore) Delete(ctx context.Context, userID, provider, tenant string) error
func (s *SQLiteStore) Close() error
```

## 実装詳細

### NewSQLiteStore
- `sql.Open("sqlite", dbPath)` で DB を開く
- `CREATE TABLE IF NOT EXISTS` で自動マイグレーション
- `db.Ping()` で接続確認

### Get
- `SELECT` → `TokenRecord` マッピング
- `sql.ErrNoRows` の場合は `nil, nil` を返す

### Put
- UPSERT: `INSERT ... ON CONFLICT(user_id, provider, tenant) DO UPDATE SET ...`
- `UpdatedAt` は常に現在時刻（UTC）を設定
- `CreatedAt` は新規の場合のみ設定

### Delete
- `DELETE FROM oauth_tokens WHERE user_id = ? AND provider = ? AND tenant = ?`
- 存在しないキーでもエラーにならない

### Close
- `db.Close()` を呼び出す
- `closed` フラグで冪等性を保証
- Close 後の操作はエラーを返す

### 時刻形式
- UTC で保存、`time.RFC3339` 形式
- 読み取り時に `time.Parse(time.RFC3339, ...)` で復元

## TDD テストケース

### Red（失敗するテストを先に書く）

1. **CRUD 正常系**: Put → Get で同一レコード取得（t.TempDir() 使用）
2. **テーブル自動作成**: NewSQLiteStore が CREATE TABLE IF NOT EXISTS を実行
3. **Get 未存在**: 存在しないキーが nil, nil
4. **Put 上書き**: Put で UpdatedAt が更新される
5. **Delete 後の Get**: nil, nil
6. **Delete 未存在**: エラーにならない
7. **Close 冪等**: 複数回 Close でエラーなし
8. **Close 後の操作**: Get/Put/Delete がエラーを返す
9. **ユーザー隔離**: Put(userA) → Get(userB) が nil, nil
10. **並行書き込み**: goroutine からの同時 Put でデータ競合なし

### Green（テストを通す最小限の実装）

上記テストを通す sqlite.go を実装。

### Refactor

- エラーメッセージの統一
- 定数の整理

## ファクトリー更新

```go
// factory.go の StoreTypeSQLite case
case auth.StoreTypeSQLite:
    return NewSQLiteStore(cfg.SQLitePath)
```

## factory_test.go 更新

`TestNewTokenStore_SQLite_NotImplemented` を `TestNewTokenStore_SQLite` に変更し、
成功パスのテストに書き換える（t.TempDir() を使用）。

## リスク評価

| リスク | 影響 | 対策 |
|--------|------|------|
| modernc.org/sqlite のビルド時間 | 中 | CI キャッシュ活用 |
| SQLite の WAL モード未設定 | 低 | 並行テストで確認、必要なら PRAGMA 追加 |
| DB ファイルのパーミッション | 低 | OS デフォルトに委ねる（将来改善余地） |

## ステータス

- [x] modernc.org/sqlite を go get
- [x] TDD Red: テスト作成
- [x] TDD Green: 実装
- [x] TDD Refactor: 整理
- [x] ファクトリー更新
- [x] ファクトリーテスト更新
- [x] 全テスト通過確認
- [ ] git commit
