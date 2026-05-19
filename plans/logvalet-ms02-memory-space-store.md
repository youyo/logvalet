# MS02: Memory SpaceStore + テスト

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS01

## 目的

テスト・開発用のインメモリ Store を実装し、Store interface の振る舞い仕様を
テストで確立する。後続の SQLite/DynamoDB Store は本 MS のテストを流用できるようにする。

## 完了条件

- [ ] `internal/space/memorystore.go` — MemoryStore 実装
- [ ] `internal/space/memorystore_test.go` — 全テストケース pass
- [ ] `go test ./internal/space/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### memorystore_test.go

```
T1: TestMemoryStore_UpsertAndGet
    - Upsert(reg{UserID:"u1", Alias:"foo"}) → Get("u1", "foo") でデータが取得できる
    - Get で取得した値が Upsert した値と一致する

T2: TestMemoryStore_UpsertOverwrite
    - 同一 (userID, alias) を2回 Upsert → Get で最新値が返る
    - CreatedAt は最初の Upsert 時の値を保持しない（MemoryStore はシンプルに上書き）

T3: TestMemoryStore_List_UserIsolation
    - Upsert(u1/foo), Upsert(u1/bar), Upsert(u2/foo)
    - List("u1") → [{u1/foo}, {u1/bar}] のみ（u2/foo は含まれない）
    - List("u2") → [{u2/foo}] のみ

T4: TestMemoryStore_List_Empty
    - 未登録 userID に対して List → 空スライス（nil でなく []SpaceRegistration{}）

T5: TestMemoryStore_Delete_TargetOnly
    - Upsert(u1/foo), Upsert(u1/bar)
    - Delete("u1", "foo")
    - Get("u1", "foo") → nil（削除済み）
    - Get("u1", "bar") → bar は残っている

T6: TestMemoryStore_Delete_DifferentUserSameAlias
    - Upsert(u1/foo), Upsert(u2/foo)
    - Delete("u1", "foo")
    - Get("u2", "foo") → u2/foo は残っている（別ユーザーの同名 alias は独立）

T7: TestMemoryStore_Delete_NotExist
    - 存在しない alias を Delete → エラーにならない（冪等）

T8: TestMemoryStore_Preference_GetPut
    - PutPreference({UserID:"u1", DefaultSpaceAlias:"foo"})
    - GetPreference("u1") → DefaultSpaceAlias == "foo"

T9: TestMemoryStore_Preference_GetNotExist
    - 未設定の userID に GetPreference → nil, nil を返す（エラーでない）

T10: TestMemoryStore_Preference_UserIsolation
    - PutPreference({u1, "foo"}), PutPreference({u2, "bar"})
    - GetPreference("u1").DefaultSpaceAlias == "foo"
    - GetPreference("u2").DefaultSpaceAlias == "bar"

T11: TestMemoryStore_Close_Idempotent
    - Close() → エラーなし
    - Close() 2回目 → エラーなし（冪等）

T12: TestMemoryStore_Concurrent_Upsert
    - 10 goroutine が同時に異なる alias を Upsert
    - List 結果が 10 件（race detector 有効で実行）
```

### NonceStore テスト（MemoryStore が NonceStore を実装する場合）

```
T13: TestMemoryStore_NonceStore_StoreAndConsume
    - Store("u1", "nonce1", 1*time.Minute) → Consume("u1", "nonce1") → nil（成功）
    - Consume 2回目 → ErrNonceAlreadyUsed

T14: TestMemoryStore_NonceStore_ConsumeNotExist
    - 未保存 nonce を Consume → ErrNonceAlreadyUsed（存在しない = 使用済みとみなす）

T15: TestMemoryStore_NonceStore_UserIsolation
    - Store("u1", "nonce1", 1*time.Minute)
    - Consume("u2", "nonce1") → ErrNonceAlreadyUsed（別ユーザーの nonce は使えない）
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/memorystore.go` | MemoryStore（Store + NonceStore 実装） |
| `internal/space/memorystore_test.go` | T1-T15 |

---

## 3. 実装

### memorystore.go

```go
package space

import (
    "context"
    "sync"
    "time"
)

// MemoryStore は Store + NonceStore のインメモリ実装。
// テスト用。並行安全。
type MemoryStore struct {
    mu          sync.RWMutex
    spaces      map[string]map[string]SpaceRegistration // userID -> alias -> reg
    preferences map[string]UserPreference               // userID -> pref
    nonces      map[string]map[string]time.Time         // userID -> nonce -> expires_at
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        spaces:      make(map[string]map[string]SpaceRegistration),
        preferences: make(map[string]UserPreference),
        nonces:      make(map[string]map[string]time.Time),
    }
}

func (s *MemoryStore) List(ctx context.Context, userID string) ([]SpaceRegistration, error)
func (s *MemoryStore) Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error)
func (s *MemoryStore) Upsert(ctx context.Context, reg *SpaceRegistration) error
func (s *MemoryStore) Delete(ctx context.Context, userID, alias string) error
func (s *MemoryStore) GetPreference(ctx context.Context, userID string) (*UserPreference, error)
func (s *MemoryStore) PutPreference(ctx context.Context, pref *UserPreference) error
func (s *MemoryStore) Close() error

// NonceStore 実装
func (s *MemoryStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
func (s *MemoryStore) Consume(ctx context.Context, userID, nonce string) error
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/memorystore_test.go` を作成（T1-T15）
2. `go test ./internal/space/...` → コンパイルエラー（MemoryStore 未定義）

### Step 2: Green

1. `internal/space/memorystore.go` を実装
2. T1-T12: Store interface のメソッドを実装
3. T13-T15: NonceStore interface のメソッドを実装
4. `go test ./internal/space/...` → 全テストパス

### Step 3: Refactor

1. `go test -race ./internal/space/...` で T12 の race detector テストを通す
2. ロック粒度の確認（RWMutex の適切な使用）

---

## 5. 実装の要点

### List の返り値は nil でなく空スライス

```go
result := make([]SpaceRegistration, 0)
// ...
return result, nil
```

### NonceStore.Consume の実装

存在しない nonce も ErrNonceAlreadyUsed として扱う
（「消費済み」と「未登録」を区別しない — セキュリティ的に安全な選択）:

```go
func (s *MemoryStore) Consume(ctx context.Context, userID, nonce string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    userNonces, ok := s.nonces[userID]
    if !ok {
        return ErrNonceAlreadyUsed
    }
    if _, exists := userNonces[nonce]; !exists {
        return ErrNonceAlreadyUsed
    }
    delete(userNonces, nonce)
    return nil
}
```

---

## 6. 検証コマンド

```bash
go test ./internal/space/... -v -race
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS02 完了 + MS03 完了後 → MS04（Space Resolver）が着手可能。
