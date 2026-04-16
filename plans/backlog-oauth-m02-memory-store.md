# M02: MemoryTokenStore 実装 — 詳細計画

## 目標

TokenStore interface を定義し、MemoryTokenStore を実装する。
PoC / テスト / Lambda 単一同時実行の簡易運用で使用するインメモリストア。

## 前提条件（完了済み）

- M01: `internal/auth/types.go` に `TokenRecord` struct 定義済み
- M03: `OAuthEnvConfig.TokenStoreType` で `"memory"` がデフォルト
- M04: `ErrStateExpired`, `ErrStateInvalid` 追加済み

## 対象ファイル

| ファイル | 役割 |
|---------|------|
| `internal/auth/store.go` | TokenStore interface 定義（`auth` パッケージ内） |
| `internal/auth/tokenstore/memory.go` | MemoryStore 実装（`tokenstore` パッケージ） |
| `internal/auth/tokenstore/memory_test.go` | MemoryStore テスト |

### 設計判断: interface の配置場所

**`TokenStore` interface は `internal/auth/` パッケージに配置する。**

理由: Go の循環依存を回避するため。
- `internal/auth/tokenstore/` が `internal/auth` を import（TokenRecord, TokenStore を使用）
- 将来 `internal/auth/manager.go`（M06）が TokenStore を受け取る
- interface が `tokenstore` にあると `auth → tokenstore → auth` の循環依存が発生
- **interface は消費者側に置く** のが Go の標準パターン

## TokenStore Interface

```go
// internal/auth/store.go
package auth

type TokenStore interface {
    Get(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error)
    Put(ctx context.Context, record *TokenRecord) error
    Delete(ctx context.Context, userID, provider, tenant string) error
    Close() error
}
```

## MemoryStore 設計

- `sync.RWMutex` で保護
- `map[string]*auth.TokenRecord` で保持
- キー = `userID:provider:tenant`
- `NewMemoryStore() TokenStore` コンストラクタ
- process restart で消えるのは仕様

### メソッド仕様

| メソッド | 仕様 |
|---------|------|
| `Get` | キー存在: レコードのコピーを返す。キー不存在: `nil, nil` |
| `Put` | upsert（上書き）。レコードのコピーを保存 |
| `Delete` | キー存在: 削除。キー不存在: エラーにならない |
| `Close` | 冪等。マップをクリアし、closed フラグを立てる |

### スレッドセーフ設計

- `Get`: `RLock` / `RUnlock`
- `Put`: `Lock` / `Unlock`
- `Delete`: `Lock` / `Unlock`
- `Close`: `Lock` / `Unlock`

### キー生成

```go
func storeKey(userID, provider, tenant string) string {
    return userID + ":" + provider + ":" + tenant
}
```

## TDD テストケース

### Red Phase（先に書く失敗テスト）

1. **TestMemoryStore_PutAndGet** — Put → Get で同一レコード取得
2. **TestMemoryStore_GetNotFound** — Get で存在しないキーが `nil, nil`
3. **TestMemoryStore_PutOverwrite** — Put 上書きで最新が返る
4. **TestMemoryStore_DeleteAndGet** — Delete 後の Get が `nil, nil`
5. **TestMemoryStore_DeleteNotFound** — Delete で存在しないキーはエラーにならない
6. **TestMemoryStore_CloseIdempotent** — Close() が冪等（2回呼んでもエラーなし）
7. **TestMemoryStore_UserIsolation** — Put(userA) → Get(userB) が `nil, nil`（ユーザー隔離）
8. **TestMemoryStore_GetReturnsCopy** — Get で返されたレコードを変更してもストア内データに影響しない
9. **TestMemoryStore_PutStoresCopy** — Put 後にオリジナルを変更してもストア内データに影響しない
10. **TestMemoryStore_ConcurrentAccess** — 並行アクセスでデータ競合なし（`-race` 検出）

### 各テストの詳細

#### 1. PutAndGet
- TokenRecord を作成して Put
- 同じキーで Get
- 全フィールドが一致することを検証

#### 2. GetNotFound
- 空のストアで Get
- `nil, nil` が返ることを検証

#### 3. PutOverwrite
- 同じキーで2回 Put（2回目は AccessToken を変更）
- Get で2回目の値が返ることを検証

#### 4. DeleteAndGet
- Put → Delete → Get
- Get が `nil, nil` を返すことを検証

#### 5. DeleteNotFound
- 存在しないキーで Delete
- エラーが `nil` であることを検証

#### 6. CloseIdempotent
- Close() を2回呼ぶ
- 両方ともエラーなし

#### 7. UserIsolation
- userA でレコードを Put
- userB で同じ provider/tenant で Get
- `nil, nil` が返ることを検証（他ユーザーのトークンが漏洩しないこと）

#### 8. GetReturnsCopy
- Put → Get → 返されたレコードの AccessToken を変更
- 再度 Get して元の値が保持されていることを検証

#### 9. PutStoresCopy
- レコードを Put → オリジナルの AccessToken を変更
- Get して Put 時の値が保持されていることを検証

#### 10. ConcurrentAccess
- 複数 goroutine から Put/Get/Delete を同時実行
- `go test -race` でデータ競合がないことを検証

## 実装ステップ

### Step 1: テストファイル作成（Red）
`internal/auth/tokenstore/memory_test.go` に全テストケースを記述。
この段階ではコンパイルエラーになる。

### Step 2: Interface 定義
`internal/auth/tokenstore/store.go` に TokenStore interface を定義。

### Step 3: MemoryStore スケルトン（Green 開始）
`internal/auth/tokenstore/memory.go` に構造体とメソッドスタブを作成。
テストがコンパイルできるようにする。

### Step 4: MemoryStore 実装（Green 完了）
各メソッドを実装し、全テストをパスさせる。

### Step 5: Refactor
- 不要なコードの除去
- ドキュメントコメントの充実
- `go vet` 実行

## リスク評価

| リスク | 影響 | 対策 |
|--------|------|------|
| コピーセマンティクス漏れ | ストア外からの参照でデータ改変 | Get/Put でコピーを作成、テストで検証 |
| Close 後のアクセス | パニックまたはデータ不整合 | closed フラグで保護（今回はシンプルにマップクリアのみ） |
| キー衝突 | 異なるユーザーのデータが混在 | `userID:provider:tenant` の3要素でキー生成、隔離テストで検証 |
| context キャンセル未対応 | 将来的な問題 | MemoryStore は即時操作のため context は受け取るが無視してよい |

## 成功基準

- `go test ./internal/auth/tokenstore/... -race -v` が全パス
- `go vet ./internal/auth/tokenstore/...` がエラーなし
- ユーザー隔離テストが含まれている
- TokenStore interface が将来の sqlite/dynamodb 実装に対応できる設計
