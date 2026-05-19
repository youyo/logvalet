# MS01: Space domain model & errors

Roadmap: plans/logvalet-multi-space-roadmap.md

## 目的

`internal/space/` パッケージを新設し、複数スペース横断操作の全マイルストーンが依存する
型定義・Store interface・NonceStore interface・エラー定数・exit code 定義を確立する。

## 完了条件

- [ ] `internal/space/types.go` — SpaceRegistration, UserPreference, SpaceStatus, AuthType
- [ ] `internal/space/store.go` — Store interface, NonceStore interface（RH5 対応）
- [ ] `internal/space/errors.go` — エラー変数 + ExitCodePartialFailure=8（RH2 対応）
- [ ] `internal/space/scope.go` — Scope struct
- [ ] `internal/space/types_test.go` — 型の基本テスト
- [ ] `internal/space/errors_test.go` — exit code 定数テスト
- [ ] `go test ./internal/space/...` パス
- [ ] `go build ./...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### types_test.go

```
T1: TestAuthType_Constants
    - AuthTypeOAuth == "oauth"
    - AuthTypeAPIKey == "api_key"（credentials.go の定数と一致すること）
    
T2: TestSpaceStatus_Constants
    - SpaceStatusUnknown == "unknown"
    - SpaceStatusOK == "ok"
    - SpaceStatusUnauthorized == "unauthorized"
    - SpaceStatusNotConnected == "not_connected"
    - SpaceStatusDisabled == "disabled"

T3: TestSpaceRegistration_ZeroValue
    - SpaceRegistration{} の各フィールドがゼロ値であること
    - Disabled フィールドはデフォルト false

T4: TestUserPreference_ZeroValue
    - UserPreference{} の DefaultSpaceAlias がゼロ値（空文字）
```

### errors_test.go

```
T5: TestExitCodePartialFailure_Value
    - ExitCodePartialFailure == 8
    - 既存の ExitCode 定数（internal/app/exitcode.go）と衝突しないこと
    （exitcode.go を読み込んで 8 が未使用であることをコメントに記載）

T6: TestErrors_Sentinel
    - errors.Is(ErrNoSpacesRegistered, ErrNoSpacesRegistered) == true
    - errors.Is(ErrNoDefaultSpace, ErrNoDefaultSpace) == true
    - errors.Is(ErrSpaceNotFound, ErrSpaceNotFound) == true
    - errors.Is(ErrInvalidSpaceScope, ErrInvalidSpaceScope) == true
    - ErrNonceAlreadyUsed != nil
```

### scope_test.go

```
T7: TestScope_ZeroValue
    - Scope{} は AllSpaces=false, Aliases=nil

T8: TestScope_AllSpaces_And_Aliases_MutualExclusion
    - Scope{AllSpaces: true, Aliases: []string{"foo"}} の意味を文書化
    （バリデーションは Resolver 側で行うが、Scope 型は両方を保持できることを確認）
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/types.go` | SpaceRegistration, UserPreference, SpaceStatus, AuthType |
| `internal/space/store.go` | Store interface, NonceStore interface |
| `internal/space/errors.go` | エラー変数, ExitCodePartialFailure |
| `internal/space/scope.go` | Scope struct |
| `internal/space/types_test.go` | T1-T4 |
| `internal/space/errors_test.go` | T5-T6 |
| `internal/space/scope_test.go` | T7-T8 |

### 変更なし

MS01 では既存パッケージへの変更はない。

---

## 3. 型定義

### types.go

```go
package space

import "time"

type AuthType string

const (
    // AuthTypeOAuth は OAuth 認証。
    AuthTypeOAuth AuthType = "oauth"
    // AuthTypeAPIKey は API key 認証。
    // 既存 internal/credentials/credentials.go の AuthTypeAPIKey = "api_key" と統一する。
    AuthTypeAPIKey AuthType = "api_key"
)

type SpaceStatus string

const (
    SpaceStatusUnknown      SpaceStatus = "unknown"
    SpaceStatusOK           SpaceStatus = "ok"
    SpaceStatusUnauthorized SpaceStatus = "unauthorized"
    SpaceStatusNotConnected SpaceStatus = "not_connected"
    // SpaceStatusDisabled は将来の自動無効化機能向け。
    // MVP では disabled を設定/解除する CLI コマンドは実装しない。
    // Resolver と verify は disabled space を除外する（RH3）。
    SpaceStatusDisabled SpaceStatus = "disabled"
)

type SpaceRegistration struct {
    UserID      string
    Alias       string
    Tenant      string
    BaseURL     string
    AuthType    AuthType
    AuthProfile string // API key mode で使う profile 名
    Provider    string // 現在は常に "backlog"
    Status      SpaceStatus
    LastVerifiedAt time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Disabled    bool
}

type UserPreference struct {
    UserID            string
    DefaultSpaceAlias string
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

### store.go

```go
package space

import (
    "context"
    "time"
)

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
```

### errors.go

```go
package space

import "errors"

var (
    ErrNoSpacesRegistered = errors.New("no spaces registered for this user")
    ErrNoDefaultSpace     = errors.New("no default space configured; run 'lv spaces use <alias>'")
    ErrSpaceNotFound      = errors.New("space not found")
    ErrInvalidSpaceScope  = errors.New("--spaces and --all-spaces are mutually exclusive")
    ErrNonceAlreadyUsed   = errors.New("nonce already consumed (replay rejected)")
)

// ExitCodePartialFailure は複数スペース実行で一部が失敗した場合の CLI exit code。
// 既存の exit code 定義（internal/app/exitcode.go）:
//   0=success, 1=generic, 2=arg/validation, 3=auth, 4=permission, 5=not_found, 6=api, 7=digest
// 8 は未使用のため割り当てる（H6/RH2 対応: partial_failure を argument error と分離）。
const ExitCodePartialFailure = 8
```

### scope.go

```go
package space

// Scope は --spaces / --all-spaces フラグから解決された操作対象スペースの指定。
// バリデーション（AllSpaces=true かつ len(Aliases)>0 はエラー）は Resolver で行う。
type Scope struct {
    Aliases   []string
    AllSpaces bool
}
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red — テストを先に書く

1. `internal/space/types_test.go` を作成（T1-T4）
2. `internal/space/errors_test.go` を作成（T5-T6）
3. `internal/space/scope_test.go` を作成（T7-T8）
4. `go test ./internal/space/...` → コンパイルエラー（型・定数未定義）

### Step 2: Green — 最小限の実装

1. `internal/space/types.go` を作成 → T1-T4 パス
2. `internal/space/errors.go` を作成 → T5-T6 パス
   - errors_test.go で `internal/app/exitcode` の定数値を確認し 8 が未使用であることを検証
3. `internal/space/scope.go` を作成 → T7-T8 パス
4. `internal/space/store.go` を作成（interface のみ、実装は MS02/MS05/MS06）
5. `go test ./internal/space/...` → 全テストパス

### Step 3: Refactor

1. コメントの整備（特に disabled の MVP 方針、ExitCode の既存定数との対比）
2. `go test ./internal/space/...` → 全テストパス
3. `go vet ./...` → クリーン

---

## 5. 実装の要点

### ExitCode 衝突チェック

```bash
grep -n "ExitCode\|= [0-9]" internal/app/exitcode.go
```

で既存定数を確認し、8 が未使用であることを errors_test.go のコメントに記録する。

### SpaceStatusDisabled の MVP 方針（RH3）

disabled を設定する CLI コマンドは MVP 外。Resolver と verify では以下の挙動を明示:
- `Resolver.Resolve(AllSpaces=true)` → disabled space は除外（enabled のみ返す）
- `lv spaces verify` → disabled space はスキップし結果に含めない

### NonceStore パッケージ配置（RH5）

`internal/auth/nonce.go` に置かない理由:
- `internal/auth` は `internal/space` に依存しないため循環依存が発生しない
- ただし DynamoDB/SQLite 実装は `internal/space/dynamodbstore.go` と `internal/space/sqlitestore.go` に置く
- `internal/auth` が NonceStore を使う場合は `internal/space.NonceStore` 型として参照する

---

## 6. 検証コマンド

```bash
go test ./internal/space/... -v
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS01 完了後、以下が並列で着手可能（RC1 修正後の依存グラフ）:
- MS02（Memory SpaceStore）← MS01 完了で着手可能
- MS03（BaseURL/Alias/Tenant 正規化）← MS01 完了で着手可能
- MS05（SQLite SpaceStore + NonceStore SQLite）← MS01 完了で着手可能
- MS06a（NonceStore interface 確認）← MS01 完了で着手可能（実質確認のみ）
- MS08（SpaceAwareClientFactory）← MS01 完了で着手可能（RC1: MS04 待ち不要）
