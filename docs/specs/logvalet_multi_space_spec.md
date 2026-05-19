# logvalet 複数 Backlog スペース横断操作対応 超詳細仕様書

## 0. 目的

`github.com/youyo/logvalet` に対して、複数 Backlog スペースを横断して操作できる機能を追加する。

現在の logvalet は、CLI / MCP ともに基本的には単一 Backlog スペースを操作対象としている。既存の profile 切り替えにより別スペースを操作することはできるが、利用者が都度 profile を切り替える必要がある。

本仕様では、以下を実現する。

- CLI で複数スペースを明示指定して操作できる
- CLI で登録済みの全スペースを対象に操作できる
- remote MCP でユーザーごとに登録済みのスペースを対象に操作できる
- MCP サーバーは複数人で共有されるが、スペース登録・認証情報・default space はユーザーごとに分離される
- API key / OAuth の両方に対応する
- 既存の単一スペース操作・既存 profile 機構は壊さない
- まずは read-only 操作の横断対応を優先し、write 操作は安全策を入れた上で段階的に対応する

---

## 1. 背景と前提

### 1.1 Backlog API の性質

Backlog API は基本的にスペースごとの Base URL に対して呼び出す。

例:

```text
https://example.backlog.com/api/v2/...
https://example.backlog.jp/api/v2/...
```

そのため、「あるユーザーがアクセス可能な全 Backlog スペースをグローバルに列挙する API」は前提にしない。

複数スペース横断操作は、logvalet 側で以下のように実現する。

```text
対象スペースを解決
  ↓
各スペースに対応する baseURL / credential を取得
  ↓
各スペースに対して同じ Backlog API 操作を fan-out
  ↓
space alias ごとの結果として集約して返す
```

### 1.2 既存設計との関係

既存の profile 機構は維持する。

ただし、複数スペース対応後の主概念は以下になる。

```text
space alias -> baseURL -> credential resolver -> backlog client
```

従来の概念は以下。

```text
profile -> credential -> backlog client
```

長期的には profile は space registration に寄っていく可能性があるが、今回の実装では後方互換性を優先し、既存 profile を破壊しない。

### 1.3 remote MCP の前提

remote MCP は複数人で共有される。

そのため、スペース登録情報・認証情報・default/current space はすべてユーザーごとに分離する。

```text
shared remote MCP server
  ├─ user A
  │   ├─ spaces: foo, bar
  │   ├─ tokens: foo, bar
  │   └─ default_space: foo
  └─ user B
      ├─ spaces: baz
      ├─ tokens: baz
      └─ default_space: baz
```

`all_spaces` は「サーバー全体の全スペース」ではなく、必ず「リクエスト元ユーザーが登録済みの全スペース」を意味する。

---

## 2. 用語定義

### 2.1 Space

Backlog の 1 スペースを表す。

例:

```text
https://foo.backlog.com
https://bar.backlog.jp
```

### 2.2 Space Alias

logvalet 内でスペースを識別する短い名前。

例:

```text
foo
bar
customer-a
prod
```

原則として、初期値は Backlog space key / tenant 名から自動生成する。

例:

```text
https://foo.backlog.com -> foo
```

重複時はユーザーに明示指定させるか、自動で suffix を付ける。

例:

```text
foo
foo-2
foo-prod
```

### 2.3 Tenant

Backlog OAuth token / TokenStore の識別に使うスペース識別子。

既存の `TokenRecord.Tenant` を活用する。

原則として tenant は Backlog のスペースキーまたは BaseURL から導出した安定識別子とする。

### 2.4 Space Registry

ユーザーごとに登録済みスペース一覧を保持する永続ストア。

```text
user_id -> []SpaceRegistration
```

### 2.5 Current / Default Space

`spaces` 未指定時に使われるスペース。

MCP は stateless に扱うため、セッションメモリではなく、ユーザーごとの Preference Store に保存する。

---

## 3. 全体方針

### 3.1 CLI の基本 UX

単一スペース指定と複数スペース指定を `--spaces` に統一する。

**【§24 実装整合性メモ参照】`--space` フラグは `internal/cli/global_flags.go:39` に既存で存在する。**
**【GPT-5.4 指摘】`--space` を alias に寄せると既存ユーザーの意味が破壊される。**

`--space` は既存動作（`buildRunContext` で `https://<space>.backlog.com` のリテラル解釈）を**一切変えない**。
`--spaces` だけが alias ベースの multi-space 指定として動く。二本立てにする。

```text
後方互換方針（GPT-5.4 指摘を受け修正）:
  --space (既存)  → buildRunContext の既存動作を完全保持。
                    Backlog space 名（literal）として扱い、
                    https://<space>.backlog.com を組み立てる。
                    multi-space Scope には**使わない**。
  --spaces        → registry alias ベースの multi-space 指定（新規）
  --all-spaces    → 登録済み全スペース指定（新規）

spaces 未指定、all-spaces 未指定の場合:
  CLI では既存と同じく --space / config / profile 解決を優先する。
  （remote MCP のみ default_space_alias を参照する）
```

```bash
lv issue list
lv issue list --spaces foo
lv issue list --spaces foo,bar
lv issue list --all-spaces
```

意味:

```text
spaces 未指定:
  current/default space を対象にする

--spaces foo:
  foo のみ対象にする

--spaces foo,bar:
  foo と bar を対象にする

--all-spaces:
  現在のユーザーに登録済みの全スペースを対象にする
```

`--spaces` と `--all-spaces` の同時指定はエラー。

### 3.2 MCP の基本 UX

MCP tool には、共通引数として以下を追加する。

```json
{
  "spaces": {
    "type": "array",
    "items": {
      "type": "string"
    },
    "description": "Target Backlog space aliases. Omit to use the current/default space."
  },
  "all_spaces": {
    "type": "boolean",
    "description": "Run against all spaces registered for the current user."
  }
}
```

意味:

```text
spaces 未指定、all_spaces 未指定または false:
  current/default space を対象にする

spaces = ["foo"]:
  foo のみ対象にする

spaces = ["foo", "bar"]:
  foo と bar を対象にする

all_spaces = true:
  リクエスト元ユーザーに登録済みの全スペースを対象にする
```

`spaces` と `all_spaces: true` の同時指定はエラー。

MCP では `"current"` という値を LLM に渡させない。未指定を current/default として扱う。

### 3.3 認証ごとの方針

#### OAuth

OAuth の場合、接続成功時に対象スペースを自動登録する。

```text
OAuth authorize
  ↓
OAuth callback
  ↓
token 保存
  ↓
GetCurrentUser / GetSpace 等で接続確認
  ↓
SpaceRegistry.Upsert
```

OAuth はユーザーごとの Backlog 権限を使うため、remote MCP と相性がよい。

#### API key

API key は基本的に単一 Backlog スペースに紐づく。

したがって API key の場合、スペース登録時に BaseURL と API key を明示登録する。

```bash
lv space add
```

または非対話式で:

```bash
lv space add \
  --alias foo \
  --base-url https://foo.backlog.com \
  --auth-type apikey \
  --api-key-env LOGVALET_BACKLOG_API_KEY_FOO
```

API key 認証で `--spaces other` を指定した場合、その space に対応する credential が存在しなければ `not_configured` として扱う。

誤った credential で API call した場合は `unauthorized` として space 単位で失敗扱いにする。

---

## 4. データモデル

### 4.1 SpaceRegistration

新規型を追加する。

推奨ファイル:

```text
internal/space/types.go
```

構造体案:

```go
package space

import "time"

// 【GPT-5.4 指摘】既存 credentials.go の AuthTypeAPIKey = "api_key"（アンダースコア区切り）と
// 統一する必要がある。永続データの auth_type フィールドの値が異なると互換が壊れる。
// 既存コードに合わせて "api_key" を使う。
type AuthType string

const (
    AuthTypeOAuth  AuthType = "oauth"
    AuthTypeAPIKey AuthType = "api_key" // "apikey" ではなく "api_key"（credentials.go と統一）
)

type SpaceStatus string

const (
    SpaceStatusUnknown      SpaceStatus = "unknown"
    SpaceStatusOK           SpaceStatus = "ok"
    SpaceStatusUnauthorized SpaceStatus = "unauthorized"
    SpaceStatusNotConnected SpaceStatus = "not_connected"
    SpaceStatusDisabled     SpaceStatus = "disabled"
)

type SpaceRegistration struct {
    UserID string

    // User-visible identifier.
    Alias string

    // Backlog tenant / space key.
    Tenant string

    // Example: https://foo.backlog.com
    BaseURL string

    AuthType AuthType

    // Optional link to existing profile for backward compatibility or API key mode.
    AuthProfile string

    // Optional provider name. Initially always "backlog".
    Provider string

    Status SpaceStatus

    LastVerifiedAt time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time

    Disabled bool
}
```

### 4.2 UserPreference

ユーザーごとの default/current space を保存する。

推奨ファイル:

```text
internal/space/preference.go
```

構造体案:

```go
package space

import "time"

type UserPreference struct {
    UserID string

    DefaultSpaceAlias string

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 4.3 Store interface

推奨ファイル:

```text
internal/space/store.go
```

```go
package space

import "context"

type Store interface {
    List(ctx context.Context, userID string) ([]SpaceRegistration, error)
    Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error)
    Upsert(ctx context.Context, reg *SpaceRegistration) error
    Delete(ctx context.Context, userID, alias string) error

    GetPreference(ctx context.Context, userID string) (*UserPreference, error)
    PutPreference(ctx context.Context, pref *UserPreference) error

    Close() error
}
```

### 4.4 DynamoDB キー設計

**【§24 実装整合性メモ参照】既存 DynamoDB TokenStore と同一テーブルへの混在は技術的に不可能。**

既存 `DynamoDBStore`（`internal/auth/tokenstore/dynamodb.go`）は
`pk = "USER#<userID>#<provider>#<tenant>"` の**単一 PK（SK なし）**テーブルに保存している。
DynamoDB はテーブル定義が PK 単一か複合かで固定されるため、
「SK あり」と「SK なし」の混在は同一テーブルで不可能。

**決定: 別テーブルを使う（TokenStore は既存のまま）**

```text
既存テーブル (logvalet-auth または任意名):
  PK = "USER#<userID>#<provider>#<tenant>"  (SK なし)
  用途: OAuth token 保存

新規テーブル (logvalet-spaces または任意名):
  PK = "USER#<userID>"
  SK = "SPACE#<alias>" または "PREF"
  用途: SpaceRegistry + UserPreference
```

新規テーブルの詳細キー設計:

```text
PK: USER#u123
SK: SPACE#foo      -> SpaceRegistration

PK: USER#u123
SK: PREF           -> UserPreference
```

環境変数で独立して設定できるようにする。

```text
LOGVALET_SPACE_DYNAMODB_TABLE=logvalet-spaces   # SpaceStore 用（新規）
LOGVALET_AUTH_DYNAMODB_TABLE=logvalet-auth      # TokenStore 用（既存変更なし）
```

**【GPT-5.4 指摘】DynamoDB の tenant 重複防止**:

```text
問題: PK=USER#uid, SK=SPACE#alias の設計では、
      同一 tenant を別 alias で二重登録できる。
      all_spaces 実行時に同一スペースへ重複 fan-out が発生する。

対策:
  Option A: GSI で (user_id, tenant) の複合インデックスを作り、
            Upsert 前に重複 tenant チェックをする
            （eventually consistent なのでレアな race は許容）
  Option B: tenant ベースの SK もレコードとして持つ
            PK=USER#uid, SK=TENANT#backlog#<tenant> -> SPACE_ALIAS アイテム追加
            → TransactWriteItems で SPACE と TENANT アイテムを同時書き込み

MVP 採用: Option A（実装がシンプル）
  - 重複 tenant は登録時に Software Check で reject
  - race はまれであり、Upsert の idempotent 化で実害を最小化

削除・リネームなどの複数 item 更新は TransactWriteItems を使う:
  例: lv spaces remove では SPACE アイテム削除 + PREF 更新を同一トランザクションで行う
```

将来的な統合テーブルへの移行（SK ありに統一）は本仕様の範囲外。
既存 TokenStore への breaking change は禁止。

### 4.5 SQLite キー設計

ローカル CLI / sqlite mode 用。

テーブル案:

```sql
CREATE TABLE IF NOT EXISTS spaces (
    user_id TEXT NOT NULL,
    alias TEXT NOT NULL,
    tenant TEXT NOT NULL,
    base_url TEXT NOT NULL,
    auth_type TEXT NOT NULL,
    auth_profile TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT 'backlog',
    status TEXT NOT NULL DEFAULT 'unknown',
    last_verified_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    disabled INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, alias)
);

CREATE INDEX IF NOT EXISTS idx_spaces_user_tenant
ON spaces(user_id, tenant);

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id TEXT PRIMARY KEY,
    default_space_alias TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

ローカル CLI では user_id が明確でないため、以下の固定 user_id を使う。

```text
local
```

ただし remote MCP では idproxy/OIDC 由来の user_id を必ず使う。

**【C1: devils-advocate 指摘】SQLite Store を remote MCP で使用した場合のデータ漏洩リスク:**

```text
問題: SQLite Store は user_id="local" 固定のため、
      remote MCP で誤って SQLite Store を使うと
      全ユーザーのスペースが同一 user_id="local" に格納され、
      ユーザー分離が崩壊する。

対策（必須）:
  サーバー起動時に以下の組み合わせを validation error として拒否する:
    LOGVALET_SPACE_STORE_TYPE=sqlite かつ remote MCP モード
    （remote MCP モードの判定: LOGVALET_MCP_MODE=remote または idproxy 設定が存在する場合）

  実装:
    func ValidateSpaceStoreConfig(storeType string, isMCPRemote bool) error {
        if storeType == "sqlite" && isMCPRemote {
            return errors.New("SQLite SpaceStore cannot be used with remote MCP mode. Use dynamodb.")
        }
        return nil
    }
```

---

## 5. Space 解決ロジック

### 5.1 Scope model

共通の scope 型を作る。

推奨ファイル:

```text
internal/space/scope.go
```

```go
package space

type Scope struct {
    Aliases   []string
    AllSpaces bool
}
```

### 5.2 Resolver

```go
type Resolver struct {
    Store Store
}

func (r *Resolver) Resolve(ctx context.Context, userID string, scope Scope) ([]SpaceRegistration, error)
```

### 5.3 Resolve 優先順位

```text
1. scope.AllSpaces == true
   -> userID の登録済み enabled spaces を全件返す

2. len(scope.Aliases) > 0
   -> 指定 alias を userID 配下で解決する
   -> 1つでも未登録ならエラー

3. user preference の default_space_alias を使う

4. 登録済み enabled space が 1件だけならそれを使う

5. 既存 config/profile から単一スペースを作れるなら backward compatibility fallback として使う

6. 決められなければ ErrNoDefaultSpace
```

### 5.4 エラー定義

```go
var (
    ErrNoSpacesRegistered = errors.New("no spaces registered")
    ErrNoDefaultSpace     = errors.New("no default space configured")
    ErrSpaceNotFound      = errors.New("space not found")
    ErrInvalidSpaceScope  = errors.New("invalid space scope")
)
```

`--spaces` と `--all-spaces` の同時指定は `ErrInvalidSpaceScope`。

---

## 6. ClientFactory 設計

### 6.1 現状の考え方

既存の OAuth 対応では、概ね以下のフローになっている。

```text
ctx -> userID -> TokenManager.GetValidToken(userID, provider, tenant) -> backlog.Client
```

複数スペース対応後は、tenant/baseURL を space registration から渡す。

```text
ctx -> userID + SpaceRegistration -> TokenManager.GetValidToken(userID, provider, tenant) -> backlog.Client
```

### 6.2 新しい factory

**【§24 実装整合性メモ参照】** 既存の `auth.NewClientFactory` は `(tm, provider, tenant, baseURL)` を
constructor で固定してクロージャを返す（`internal/auth/factory.go:22`）。
multi-space では `(ctx, reg)` 両方が動的なため、新たに `SpaceAwareClientFactory` を追加する。
**既存 `ClientFactory` は変更しない**（後方互換）。

推奨ファイル:

```text
internal/auth/space_factory.go
```

```go
// SpaceAwareClientFactory は (ctx, SpaceRegistration) -> backlog.Client を返す関数型。
// 認証方式（OAuth / APIKey）を内部で分岐する。
type SpaceAwareClientFactory func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error)
```

統合 factory（OAuth + API key を内部で分岐）:

```go
// NewSpaceAwareClientFactory は認証方式を内部で分岐する SpaceAwareClientFactory を返す。
// - reg.AuthType == "oauth":
//     ctx から userID を取得 -> tm.GetValidToken(ctx, userID, "backlog", reg.Tenant)
//     -> backlog.NewHTTPClient(BaseURL=reg.BaseURL, Bearer=token)
// - reg.AuthType == "apikey":
//     credResolver.Resolve(reg.AuthProfile, ...) で API key を取得
//     -> backlog.NewHTTPClient(BaseURL=reg.BaseURL, APIKey=key)
func NewSpaceAwareClientFactory(
    tm auth.TokenManager,
    credResolver credentials.Resolver,
) SpaceAwareClientFactory
```

既存 `buildRunContext`（`internal/cli/runner.go`）との共存:

```text
既存パス（profile 利用）:
  buildRunContext -> config.Resolve -> credentials.Resolve -> backlog.NewHTTPClient
  => 変更なし。--spaces / --all-spaces 未指定時は引き続きこのパスを使う。

新規パス（space registry 利用）:
  buildRunContext -> SpaceResolver.Resolve -> SpaceAwareClientFactory per space
  => --spaces / --all-spaces 指定時に使う。
  => Executor.Execute 内で per-space client を生成する。
```

フロー:

```text
1. reg.AuthType を見る

2. oauth の場合:
   2-1. ctx から userID を取得
   2-2. tm.GetValidToken(ctx, userID, "backlog", reg.Tenant)
   2-3. OAuth access token で backlog.NewHTTPClient

3. apikey の場合:
   3-1. reg.AuthProfile があれば profile から API key 解決
   3-2. reg に credential env name がある設計なら env から解決
   3-3. API key で backlog.NewHTTPClient

4. reg.BaseURL を ClientConfig.BaseURL にセット

5. backlog.Client を返す
```

**【H5: 同一 tenant を複数 alias が参照するケースの token refresh】**

```text
ケース: SpaceRegistration{Alias: "prod-ro", Tenant: "myorg"} と
        SpaceRegistration{Alias: "prod-rw", Tenant: "myorg"} が同一 tenant を参照。

ExecuteAcrossSpaces が並列に両スペースの factory を呼び、両方が同時に
token refresh を必要とする場合:
  → auth.TokenManager の singleflight.Group が (userID, provider, tenant) キーで
    refresh を1回だけ実行する（internal/auth/manager.go:101 の sfKey 参照）。
  → これは正しく動作する（singleflight で dedup される）。

実装注意事項:
  SpaceAwareClientFactory の OAuth パスで `tm.GetValidToken(ctx, userID, "backlog", reg.Tenant)` を
  呼ぶ際、Tenant キーが同一なら自動的に dedup される。
  実装者が独自の refresh ロジックを書かないこと。必ず TokenManager 経由で取得する。
```

### 6.3 API key と別スペース指定時の扱い

API key はスペース固有である。

そのため `--spaces foo` の場合は、必ず `foo` に紐づく credential を使う。

もし `foo` の credential が無い場合:

```text
space: foo
ok: false
error_code: not_configured
```

もし credential はあるが Backlog API が 401/403 を返した場合:

```text
space: foo
ok: false
error_code: unauthorized
```

横断操作全体は、他スペースの結果を返せるなら partial success とする。

---

## 7. Fan-out 実行基盤

### 7.1 共通結果型

推奨ファイル:

```text
internal/space/executor.go
```

```go
// Result は1スペースの実行結果を保持する generic 型。
// 【GPT-5.4 指摘】Result フィールドは T ではなく *T にする。
// T が struct 型のとき `json:",omitempty"` は struct を省略しない（空 struct でも出力される）。
// *T にすることで失敗時（OK=false）に null / omitted になる。
type Result[T any] struct {
    SpaceAlias string `json:"space"`
    Tenant     string `json:"tenant,omitempty"`
    BaseURL    string `json:"base_url,omitempty"`

    OK bool `json:"ok"`

    Result *T `json:"result,omitempty"` // *T で失敗時 null になる

    Error     string `json:"error,omitempty"`
    ErrorCode string `json:"error_code,omitempty"`
}
```

### 7.2 Executor

**【§24 実装整合性メモ参照】Go はメソッドに型パラメータを付けられない制約がある。**

`func (e *Executor) Execute[T any](...)` は Go のコンパイルエラーになる。
`Executor` struct の method は使わず、**package-level 関数**として実装する。

```go
// Executor は fan-out 実行設定を保持する。型パラメータは持たない。
type Executor struct {
    Factory        SpaceAwareClientFactory
    MaxConcurrency int
}

// ExecuteAcrossSpaces は spaces 一覧に対して fn を並列実行し、
// 入力順を保持した []Result[T] を返す。
// 1スペースの失敗は Result.OK=false に記録し、全体は続行する。
func ExecuteAcrossSpaces[T any](
    ctx context.Context,
    executor *Executor,
    spaces []SpaceRegistration,
    fn func(context.Context, SpaceRegistration, backlog.Client) (T, error),
) []Result[T]
```

呼び出し例:

```go
results := space.ExecuteAcrossSpaces(ctx, exec, regs,
    func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) ([]domain.Issue, error) {
        return c.ListIssues(ctx, opts)
    },
)
```

`Executor.MaxConcurrency` はデフォルト 4。`semaphore.NewWeighted(int64(maxConcurrency))` で制御し、
`golang.org/x/sync/semaphore` を使う（既存依存に含まれる）。

**【H1: MaxConcurrency=0 の deadlock 防止】**

`semaphore.NewWeighted(0)` は `Acquire` が永遠にブロックするため deadlock になる。

```go
// ExecuteAcrossSpaces 内で maxConcurrency を validate する
func resolveMaxConcurrency(n int) int {
    if n <= 0 {
        return 4 // デフォルト値にフォールバック
    }
    return n
}
```

`LOGVALET_SPACE_FANOUT_CONCURRENCY=0` または未設定の場合はデフォルト 4 を使う。
0 以下の値は起動時に warning ログを出してデフォルト値に置き換える（エラーにしない）。

### 7.3 並列数

デフォルトは 4 程度。

環境変数で変更可能にしてもよい。

```text
LOGVALET_SPACE_FANOUT_CONCURRENCY=4
```

CLI flag として追加する場合:

```bash
--space-concurrency 4
```

MVP では環境変数または固定値でよい。

### 7.4 エラー方針

横断実行では、1スペースの失敗で全体を即失敗にしない。

例:

```json
[
  {
    "space": "foo",
    "ok": true,
    "result": {
      "count": 12
    }
  },
  {
    "space": "bar",
    "ok": false,
    "error_code": "unauthorized",
    "error": "unauthorized"
  }
]
```

ただし以下は全体エラーにしてよい。

- spaces scope が不正
- 指定された alias が存在しない
- default space が決められない
- `--all-spaces` だが登録済みスペースが0件

**【M5: rate_limited（429）のファンアウト時リトライ方針】**

```text
MVP での 429 処理方針:
  ExecuteAcrossSpaces 内でスペースの API 呼び出しが 429 を返した場合:
    → 即座に partial failure として Result.ErrorCode = "rate_limited" にする
    → リトライ・バックオフは MVP では行わない

ユーザー向けエラーメッセージ:
  "Rate limit exceeded for space '<alias>'. Try again in a moment, or reduce --space-concurrency."
  エラーに現在の並列度（maxConcurrency）を含めて原因を示す。

将来対応（Phase 2 以降）:
  指数バックオフ + jitter でリトライ（最大3回）。
  バックオフは Backlog API の Retry-After ヘッダを優先する。
```

### 7.5 出力形式

単一スペース操作でも、横断モードを使った場合は配列で返す。

```bash
lv issue list --spaces foo
```

は、既存出力互換性を重視するなら単一結果でもよい。

ただし `--spaces` を指定した時点で横断 result envelope に統一する方が実装は安定する。

推奨:

```text
spaces 未指定:
  既存出力形式

--spaces / --all-spaces 指定:
  space result envelope 形式
```

---

## 8. CLI 仕様

### 8.1 Global flags

全コマンド共通で以下を追加する。

```bash
--spaces string
--all-spaces
```

`--spaces` は comma-separated alias list。

例:

```bash
--spaces foo
--spaces foo,bar,baz
```

空白を含む入力は trim する。

以下はエラー。

```bash
--spaces ""
--spaces ","
--spaces "foo,"
--spaces "foo,,bar"
--spaces foo --all-spaces
```

### 8.2 space 管理コマンド

**【§24 実装整合性メモ参照】`internal/cli/space.go` に既存で `lv space info`, `lv space disk-usage`, `lv space digest` が存在する。**
これらは Backlog API の `/api/v2/space` 操作であり、registry 管理とは別物。
コマンド名の衝突を避けるため registry 管理コマンドは `lv spaces`（複数形）に配置する。

```text
既存（変更なし）:
  lv space info         -> internal/cli/space.go SpaceInfoCmd
  lv space disk-usage   -> internal/cli/space.go SpaceDiskUsageCmd
  lv space digest       -> internal/cli/space.go SpaceDigestCmd

新規追加（registry 管理）:
  lv spaces list
  lv spaces add
  lv spaces connect
  lv spaces remove <alias>
  lv spaces use <alias>
  lv spaces verify [--spaces foo,bar | --all-spaces]
  （lv spaces rename は MVP 除外 → 下記 C4 参照）
```

**【C4: devils-advocate 指摘】lv spaces rename は MVP 除外:**

```text
問題: DynamoDB では SK を変更できないため、rename は Delete + Put が必要。
      この2操作は atomic でない（C2 と同様の問題）。
      Delete 成功 + Put 失敗でスペースが消失するリスクがある。

決定: lv spaces rename は MVP スコープから除外する。

代替策（ユーザー向け）:
  lv spaces add --alias new-name ...  で新規登録
  lv spaces remove old-name           で旧登録を削除
  これで実質的に rename と同等の効果が得られる。

将来実装する場合の仕様:
  1. PK=USER#uid, SK=SPACE#new 新規 Put
  2. UserPreference.DefaultSpaceAlias が old なら new に更新（条件付き）
  3. PK=USER#uid, SK=SPACE#old を Delete
  上記3操作を DynamoDB TransactWriteItems（同一テーブル内）でまとめて実行する。
  SQLite では BEGIN TRANSACTION / COMMIT で atomicity を保証する。
```

実装では `internal/cli/spaces_registry.go` に `SpacesCmd` として配置する。

```go
// SpacesCmd は spaces registry 管理コマンド群のルート。
// 既存 SpaceCmd (lv space ...) と名前空間を分ける。
type SpacesCmd struct {
    List    SpacesListCmd    `cmd:"" help:"list registered spaces"`
    Add     SpacesAddCmd     `cmd:"" help:"register a space with API key"`
    Connect SpacesConnectCmd `cmd:"" help:"register a space via OAuth"`
    Remove  SpacesRemoveCmd  `cmd:"" name:"remove" arg:"" help:"remove a registered space"`
    Use     SpacesUseCmd     `cmd:"" name:"use" arg:"" help:"set default space"`
    Verify  SpacesVerifyCmd  `cmd:"" help:"verify space connections"`
    // Rename は MVP 除外。将来 TransactWriteItems で安全実装後に追加。（C4 参照）
}
```

#### 8.2.1 lv spaces list

登録済みスペース一覧を表示する。

```bash
lv spaces list
```

出力例:

```text
NAME   TYPE    STATUS           DEFAULT   BASE_URL
foo    oauth   ok               *         https://foo.backlog.com
bar    oauth   ok                         https://bar.backlog.com
baz    apikey  unauthorized               https://baz.backlog.com
```

JSON 出力対応済みのCLIなら、以下の形にする。

```json
[
  {
    "alias": "foo",
    "tenant": "foo",
    "base_url": "https://foo.backlog.com",
    "auth_type": "oauth",
    "status": "ok",
    "default": true
  }
]
```

#### 8.2.2 lv spaces connect

OAuth 接続によるスペース登録。

```bash
lv space connect
```

対話式:

```text
Backlog Space URL:
> https://foo.backlog.com

Open browser for authorization? [Y/n]
```

非対話式:

```bash
lv space connect \
  --base-url https://foo.backlog.com \
  --alias foo
```

フロー:

```text
1. baseURL を受け取る
2. tenant/alias を導出する
3. OAuth authorize URL を生成する
4. ブラウザで認可させる
5. callback で token を保存する
6. GetCurrentUser / GetSpace で検証する
7. SpaceRegistry.Upsert する
8. default space 未設定なら default に設定する
```

#### 8.2.3 lv spaces add

API key など静的 credential による登録。

```bash
lv space add
```

対話式:

```text
Space alias:
> foo

Base URL:
> https://foo.backlog.com

Auth type:
  1. API Key
  2. OAuth
> 1

API Key:
> ********
```

非対話式:

```bash
lv space add \
  --alias foo \
  --base-url https://foo.backlog.com \
  --auth-type apikey \
  --api-key-env LOGVALET_BACKLOG_API_KEY_FOO
```

API key の保存方法は既存 credentials 機構に合わせる。

既存 profile を使う場合:

```bash
lv space add \
  --alias foo \
  --base-url https://foo.backlog.com \
  --auth-type apikey \
  --auth-profile foo
```

#### 8.2.4 lv spaces use

default/current space を設定する。

```bash
lv space use foo
```

効果:

```text
UserPreference.DefaultSpaceAlias = "foo"
```

MCP の spaces 未指定時にもこの default が使われる。

#### 8.2.5 lv spaces remove

登録済み space を削除する。

```bash
lv space remove foo
```

デフォルトでは registry から削除する。

OAuth token も削除するかは重要な仕様なので、以下を推奨する。

```bash
lv space remove foo
```

は registry のみ削除。

```bash
lv space remove foo --revoke-token
```

または:

```bash
lv space disconnect foo
```

で token も削除。

MVP では混乱を避けるため、次の仕様でもよい。

```text
space remove:
  registry と対応 token を両方削除する

token を残すユースケースは後回し
```

推奨は明示性の高い以下。

```bash
lv space remove foo              # registry から削除
lv space disconnect foo          # registry + credential/token 削除
```

**【H3: default space 削除後のフォールバック仕様】**

```text
問題: lv spaces remove <alias> で現在の default space を削除した場合、
      次の操作で ErrNoDefaultSpace になりエラーメッセージのみでは原因不明。

対策（採用）:
  削除時の UserPreference 更新ルール:
    1. 削除後に登録済み enabled space が 1件以上残る場合:
       → 最初の enabled space を自動的に新しい default に設定する
       → CLI に "Default space changed to '<alias>'" と表示する
    2. 削除後に登録済み space が 0件になる場合:
       → DefaultSpaceAlias を空文字にクリアする
       → CLI に "No spaces registered. Run 'lv spaces add' or 'lv spaces connect'." と表示する

エラーメッセージ改善:
  ErrNoDefaultSpace 発生時は「lv spaces use <alias> で default を設定してください」
  または「lv spaces list で登録スペースを確認してください」を案内する。
```

DynamoDB 実装:

```text
TransactWriteItems:
  1. Delete SPACE#<alias>
  2. Update PREF: DefaultSpaceAlias を新しい値に更新
     （新しい値 = 残りスペースの最初の enabled alias、または ""）
```

#### 8.2.6 lv spaces verify

登録済みスペースへの接続確認。

```bash
lv space verify --all-spaces
lv space verify --spaces foo,bar
```

内部では各スペースに対して `GetSpace` または軽量 API を呼び、status を更新する。

---

## 9. MCP 仕様

### 9.1 共通引数

横断対応する MCP tool には以下を追加する。

```json
{
  "spaces": {
    "type": "array",
    "items": {
      "type": "string"
    },
    "description": "Target Backlog space aliases. Omit to use the current/default space."
  },
  "all_spaces": {
    "type": "boolean",
    "description": "Run against all spaces registered for the current user."
  }
}
```

### 9.2 MCP tool の default

```text
spaces 未指定
all_spaces 未指定または false
```

の場合、現在のユーザーの default space を使う。

default space が無い場合:

```json
{
  "error": {
    "code": "no_default_space",
    "message": "No default Backlog space is configured. Connect or select a space first."
  }
}
```

### 9.3 MCP 用 space 管理 tools

以下を追加する。

```text
logvalet_space_list
logvalet_space_use
logvalet_space_verify
logvalet_space_connect_url
logvalet_space_disconnect
```

#### logvalet_space_list

現在のユーザーに登録済みのスペース一覧を返す。

引数なし。

返却例:

```json
{
  "spaces": [
    {
      "alias": "foo",
      "tenant": "foo",
      "base_url": "https://foo.backlog.com",
      "auth_type": "oauth",
      "status": "ok",
      "default": true
    }
  ]
}
```

#### logvalet_space_use

default space を変更する。

引数:

```json
{
  "alias": "foo"
}
```

#### logvalet_space_verify

スペース接続を検証する。

引数:

```json
{
  "spaces": ["foo", "bar"],
  "all_spaces": false
}
```

#### logvalet_space_connect_url

remote MCP では MCP tool の中でブラウザ OAuth を完結しづらい。

そのため、接続開始用 URL を返す tool を用意する。

引数:

```json
{
  "base_url": "https://foo.backlog.com",
  "alias": "foo"
}
```

返却:

```json
{
  "authorization_url": "https://...",
  "message": "Open this URL in your browser to connect the Backlog space."
}
```

実際の OAuth callback で token 保存と SpaceRegistry.Upsert を行う。

#### logvalet_space_disconnect

登録と認証情報を削除する。

引数:

```json
{
  "alias": "foo"
}
```

### 9.4 MCP ユーザー分離

MCP request は idproxy/OIDC 等により userID を確定している前提。

全ての MCP tool は以下の流れにする。

```text
1. ctx から userID を取得
2. userID 配下の SpaceRegistry を参照
3. spaces/all_spaces/default を解決
4. userID + tenant で token を取得
5. 対象スペースに対して実行
```

他ユーザーの registry / token / preference を参照してはいけない。

### 9.5 `all_spaces` の意味

MCP の `all_spaces: true` は、以下を意味する。

```text
現在の MCP リクエストユーザーが登録済みの enabled spaces 全て
```

以下ではない。

```text
MCP サーバーに登録されている全ユーザーのスペース
```

この区別はセキュリティ上重要。

---

## 10. OAuth 登録フロー

### 10.1 CLI OAuth

```text
lv space connect
  ↓
baseURL 入力
  ↓
tenant/alias 導出
  ↓
OAuth state に userID / tenant / baseURL / alias を含めて署名
  ↓
authorize URL を開く
  ↓
callback
  ↓
state 検証
  ↓
code exchange
  ↓
token 保存
  ↓
SpaceRegistry.Upsert
  ↓
default 未設定なら default に設定
```

### 10.2 remote MCP OAuth

```text
logvalet_space_connect_url
  ↓
authorization_url を返す
  ↓
ユーザーがブラウザで開く
  ↓
/oauth/backlog/callback
  ↓
idproxy/OIDC userID と state userID を検証
  ↓
token 保存
  ↓
SpaceRegistry.Upsert
  ↓
default 未設定なら default に設定
```

### 10.3 OAuth state

**【§24 実装整合性メモ参照】既存 `StateClaims` は `UserID/Tenant/Nonce/Continue` のみ。**
`internal/auth/state.go:23-29` の `StateClaims` struct に `BaseURL` と `Alias` フィールドを追加する。
既存フィールドは変更しない（後方互換）。

```go
// StateClaims は OAuth state JWT のカスタムクレームを保持する（拡張版）。
type StateClaims struct {
    UserID   string `json:"uid"`
    Tenant   string `json:"tenant"`
    Nonce    string `json:"nonce"`
    Continue string `json:"continue,omitempty"`
    // multi-space 対応で追加（既存フローでは空でよい）
    BaseURL  string `json:"base_url,omitempty"`
    Alias    string `json:"alias,omitempty"`
    jwt.RegisteredClaims
}
```

state JWT のペイロード例（multi-space 登録フロー）:

```json
{
  "uid": "u123",
  "tenant": "foo",
  "nonce": "abc123def456",
  "base_url": "https://foo.backlog.com",
  "alias": "foo",
  "exp": 1710000600,
  "iat": 1710000000
}
```

既存の `/oauth/backlog/authorize` は `tenant` を `OAuthHandler.tenant` フィールドから固定で取得する。
multi-space では `GenerateStateWithSpaceInfo(userID, tenant, baseURL, alias, ...)` として
別関数を追加し、既存 `GenerateStateWithContinue` には手を入れない。

`OAuthHandler.tenant` フィールドが固定されている問題（§24 参照）については、
multi-space 登録時専用の `MultiSpaceOAuthHandler` を別途実装することで
既存 `OAuthHandler` への変更を最小化する。

**【GPT-5.4 指摘】OAuth callback の replay-safe / race condition 対策**:

```text
問題1: 既存 state JWT は one-time-use ではない（期限内に replay 可能）。
       callback 再送で token 上書き・space 二重登録・default 再設定が起きる。

問題2: alias 自動採番を callback 側でやると、同一 connect の二重到達で
       "foo" と "foo-2" が生成される。

問題3: "default 未設定なら default に設定" は compare-and-set なしだと
       並行 callback で競合する。

対策（採用）:
  1. alias は authorize 前（lv spaces connect 実行時）に確定する。
     callback 側では alias の新規生成はしない。
  2. SpaceStore.Upsert は idempotent に実装する（同一 userID+alias の上書き）。
  3. default space 設定は PutPreference に条件付き write を使う:
       if DefaultSpaceAlias == "" then set alias（DynamoDB: ConditionExpression）
  4. **【C3: devils-advocate 指摘 → MVP 必須に変更】**
     state の one-time-use 実装（nonce 消費）は **MVP から必須** とする。
     multi-space 登録フロー（MultiSpaceOAuthHandler）限定で実装することで影響範囲を最小化する。

     実装方針:
       - nonce を DynamoDB（SpaceStore テーブルの特別 SK）または SQLite に TTL 付きで保存
       - PK=USER#<uid>, SK=NONCE#<nonce> で保存（TTL=state 有効期限と同じ）
       - callback 到達時: nonce を条件付き Delete（ConditionExpression: attribute_exists）
         → Delete 成功: 正常処理継続
         → Delete 失敗（KeyNotFound）: replay として 400 を返す
       - nonce 消費と SpaceRegistry.Upsert は別操作でよい（nonce 消費で replay を防ぐことが目的）
```


### 10.4 callback 時の検証

callback では以下を検証する。

```text
1. state 署名が正しい
2. state が期限切れでない
3. provider == backlog
4. userID が現在の認証ユーザーと一致する
5. tenant/baseURL が空でない
6. code が空でない
7. token exchange 成功
8. Backlog API で current user / space 確認成功
```

成功後、TokenStore と SpaceRegistry を更新する。

**【C2: devils-advocate 指摘】TokenStore と SpaceRegistry の書き込み順序と障害対策:**

```text
問題: TokenStore.Put と SpaceRegistry.Upsert は別テーブルのため atomic でない。
      一方が失敗すると不整合状態（Token あり Space なし、またはその逆）が永続化する。

対策（採用）:
  書き込み順序を「TokenStore 先」に固定する:

  1. nonce 消費（DynamoDB: 条件付き Delete） → 失敗なら replay エラー返却
  2. TokenStore.Put（token 保存） → 失敗なら 500 エラー返却
  3. SpaceRegistry.Upsert（space 登録） → 失敗しても idempotent なので再試行可能
  4. UserPreference 条件付き更新（default space 未設定なら設定）

  根拠: Token が先に保存されていれば、Space Upsert が失敗しても
        ユーザーが lv spaces connect を再実行することで回復できる。
        逆（Space あり Token なし）は lv spaces verify が機能しないため回復困難。

  不整合検知（lv spaces verify の強化）:
    verify は接続確認の前に TokenStore の token 存在チェックも行う。
    token が存在しない場合は error_code="not_connected" ではなく
    error_code="token_missing" として返し、「lv spaces connect を再実行してください」と案内する。
```

---

## 11. API key 登録フロー

### 11.1 対話式

```bash
lv space add
```

```text
Space alias:
> foo

Base URL:
> https://foo.backlog.com

Auth type:
  1. API Key
  2. OAuth
> 1

API Key:
> ********
```

その後:

```text
1. BaseURL 正規化
2. API key を既存 credential store に保存
3. SpaceRegistry.Upsert
4. GetSpace で検証
5. default 未設定なら default に設定
```

### 11.2 既存 profile からの登録

既存 profile を活用できるようにする。

```bash
lv space add \
  --alias foo \
  --base-url https://foo.backlog.com \
  --auth-type apikey \
  --auth-profile foo-profile
```

### 11.3 既存 profile import

任意実装だが、UX と移行性のため推奨。

```bash
lv space import-profiles
```

既存 config から profile を読み、space registry に登録する。

MVP では後回しでもよい。

---

## 12. BaseURL / Alias / Tenant 正規化

### 12.1 BaseURL

入力例:

```text
foo.backlog.com
https://foo.backlog.com
https://foo.backlog.com/
```

正規化後:

```text
https://foo.backlog.com
```

仕様:

```text
scheme が無い場合は https を付与
末尾 slash は削除
path/query/fragment は原則禁止
```

不正例:

```text
https://foo.backlog.com/api/v2
https://foo.backlog.com?x=1
```

### 12.2 Alias

デフォルト alias は host から導出する。

```text
https://foo.backlog.com -> foo
https://foo.backlog.jp  -> foo
```

許可文字:

```text
[a-zA-Z0-9._-]
```

推奨として小文字に正規化する。

```text
Foo -> foo
```

### 12.3 Tenant

Tenant は `TokenStore` の lookup key であるため安定性が最重要。

**tenant 導出アルゴリズム（厳密版）**:

```text
ドメイン別ルール:

1. *.backlog.com (例: foo.backlog.com)
   tenant = サブドメイン第一ラベル
   例: "https://foo.backlog.com" -> tenant = "foo"

2. *.backlog.jp (例: foo.backlog.jp)
   tenant = サブドメイン第一ラベル
   例: "https://foo.backlog.jp" -> tenant = "foo"

3. カスタムドメイン (例: backlog.example.com)
   URL からの導出は不可能。
   必ず GetSpace() を呼び spaceKey を取得してから tenant に使う。
   例: GET /api/v2/space -> {"spaceKey": "example-org"}
       tenant = "example-org"
   
   カスタムドメインの判定: host が ".backlog.com" でも ".backlog.jp" でもない場合
```

実装例:

```go
// DeriveInitialTenant は BaseURL から暫定 tenant を導出する。
// カスタムドメインの場合は空文字を返す（GetSpace が必要）。
func DeriveInitialTenant(baseURL string) (string, error) {
    u, err := url.Parse(baseURL)
    if err != nil {
        return "", err
    }
    host := strings.ToLower(u.Hostname())
    switch {
    case strings.HasSuffix(host, ".backlog.com"):
        parts := strings.SplitN(host, ".", 2)
        return parts[0], nil
    case strings.HasSuffix(host, ".backlog.jp"):
        parts := strings.SplitN(host, ".", 2)
        return parts[0], nil
    default:
        // カスタムドメイン: GetSpace() で spaceKey を取得してから設定
        return "", nil
    }
}
```

スペース登録フロー（GetSpace を使う場合）:

```text
1. BaseURL 正規化
2. DeriveInitialTenant(baseURL) で暫定 tenant を取得
3. 暫定 tenant が空の場合（カスタムドメイン）:
   3-1. API key または仮 Bearer token で GET /api/v2/space を呼ぶ
   3-2. レスポンスの spaceKey を tenant に使う
4. alias rename しても tenant は変えない（SpaceRegistration.Tenant は immutable）
```

**OAuth callback での tenant 確定**:

callback 時 state から `tenant` を取得する。
カスタムドメインの場合は state 生成前に GetSpace を呼んでおく（§10.1 フロー参照）。

**【GPT-5.4 指摘】カスタムドメインの OAuth 鶏卵問題**:

```text
問題: カスタムドメインでは OAuth authorize 前に token がないため
      GetSpace() を呼べず spaceKey（tenant）を取得できない。

解決策（採用）:
  1. lv spaces connect 時にユーザーへ tenant 名（spaceKey）の手動入力を求める
     例: "Your Backlog space key (e.g. 'my-org'):"
  2. *.backlog.com / *.backlog.jp はホストから自動導出
  3. カスタムドメイン判定時のみ入力を求める

alias 自動生成もカスタムドメインでは host 依存にしない:
  backlog.example.com → host 依存だと "backlog" になり誤解を招く
  → カスタムドメインでは alias も入力必須にする

```


---

## 13. 横断対応対象

### 13.1 MCP ツール横断対応マトリクス

**【§24 実装整合性メモ参照】** `internal/mcp/tool_categories.go` に登録された全65ツールに対する優先度。

#### MVP（M11〜M12 で対応）

```text
ツール名                           カテゴリ     優先度  備考
logvalet_space_info                read-only    MVP     Backlog API /space
logvalet_space_disk_usage          read-only    MVP
logvalet_space_digest              read-only    MVP
logvalet_project_list              read-only    MVP
logvalet_project_get               read-only    MVP
logvalet_project_health            read-only    MVP
logvalet_project_blockers          read-only    MVP
logvalet_issue_list                read-only    MVP
logvalet_issue_get                 read-only    MVP
logvalet_issue_context             read-only    MVP
logvalet_issue_stale               read-only    MVP
logvalet_digest_daily              read-only    MVP
logvalet_digest_weekly             read-only    MVP
logvalet_digest_unified            read-only    MVP
logvalet_activity_list             read-only    MVP
logvalet_activity_stats            read-only    MVP
logvalet_activity_digest           read-only    MVP
```

#### Phase 2（M11 後続）

```text
ツール名                           カテゴリ     優先度
logvalet_user_list                 read-only    Phase2
logvalet_user_get                  read-only    Phase2
logvalet_user_me                   read-only    Phase2
logvalet_user_activity             read-only    Phase2
logvalet_user_workload             read-only    Phase2
logvalet_team_list                 read-only    Phase2
logvalet_team_get                  read-only    Phase2
logvalet_team_project              read-only    Phase2
logvalet_issue_timeline            read-only    Phase2
logvalet_issue_triage_materials    read-only    Phase2
logvalet_issue_comment_list        read-only    Phase2
logvalet_issue_attachment_list     read-only    Phase2
logvalet_issue_attachment_download read-only    Phase2
logvalet_my_tasks                  read-only    Phase2
logvalet_document_list             read-only    Phase2
logvalet_document_get              read-only    Phase2
logvalet_document_tree             read-only    Phase2
logvalet_document_digest           read-only    Phase2
logvalet_shared_file_list          read-only    Phase2
logvalet_shared_file_download      read-only    Phase2
logvalet_meta_categories           read-only    Phase2
logvalet_meta_issue_types          read-only    Phase2
logvalet_meta_statuses             read-only    Phase2
logvalet_meta_version              read-only    Phase2
logvalet_meta_custom_field         read-only    Phase2
logvalet_wiki_list                 read-only    Phase2
logvalet_wiki_get                  read-only    Phase2
logvalet_wiki_count                read-only    Phase2
logvalet_wiki_tags                 read-only    Phase2
logvalet_wiki_history              read-only    Phase2
logvalet_wiki_stars                read-only    Phase2
logvalet_wiki_attachment_list      read-only    Phase2
logvalet_wiki_sharedfile_list      read-only    Phase2
logvalet_watching_list             read-only    Phase2
logvalet_watching_get              read-only    Phase2
logvalet_watching_count            read-only    Phase2
```

#### 将来対応（write / destructive: MVP では multi-space 禁止）

```text
ツール名                            カテゴリ              MVP方針
logvalet_issue_create               write-non-idempotent  multi-space 禁止
logvalet_issue_comment_add          write-non-idempotent  multi-space 禁止
logvalet_document_create            write-non-idempotent  multi-space 禁止
logvalet_issue_attachment_upload    write-non-idempotent  multi-space 禁止
logvalet_issue_update               write-idempotent      multi-space 禁止
logvalet_issue_comment_update       write-idempotent      multi-space 禁止
logvalet_star_add                   write-idempotent      multi-space 禁止
logvalet_watching_add               write-idempotent      multi-space 禁止
logvalet_watching_update            write-idempotent      multi-space 禁止
logvalet_watching_mark_as_read      write-idempotent      multi-space 禁止
logvalet_watching_delete            destructive           multi-space 禁止
logvalet_issue_attachment_delete    destructive           multi-space 禁止
```

### 13.2 write 操作

初期段階では write 操作の `all_spaces` を禁止することを推奨する。

例:

```text
issue create
issue update
comment add
status change
assignee change
```

MVP 仕様:

```text
write 操作:
  spaces 未指定または --spaces 1件のみ許可
  --spaces 複数件は禁止
  --all-spaces 禁止
```

将来対応する場合は以下の安全策を必須にする。

CLI:

```bash
lv issue update ABC-123 --status Done --spaces foo,bar --confirm-spaces foo,bar
```

MCP:

```json
{
  "spaces": ["foo", "bar"],
  "confirm_multi_space_write": true
}
```

---

## 14. エラーコード

共通エラーコードを定義する。

```text
invalid_space_scope
space_not_found
no_spaces_registered
no_default_space
not_configured
not_connected
unauthorized
forbidden
rate_limited
backlog_error
partial_failure
internal_error
```

### 14.1 space_not_found

指定された alias が userID 配下に存在しない。

### 14.2 not_configured

space registration は存在するが credential が解決できない。

### 14.3 not_connected

OAuth token が存在しない、または refresh 不能。

### 14.4 unauthorized

Backlog API が 401 を返した。

### 14.5 forbidden

Backlog API が 403 を返した。

### 14.6 partial_failure

複数スペース実行で一部だけ失敗した。

CLI の exit code は以下を推奨する。

```text
0: 全成功
1: 全体エラー
2: partial failure
```

既存 CLI の exit code 方針がある場合はそれに合わせる。

---

## 15. 設定

### 15.1 環境変数

追加候補:

```text
LOGVALET_SPACE_STORE_TYPE=memory|sqlite|dynamodb
LOGVALET_SPACE_SQLITE_PATH=/path/to/logvalet-spaces.db
LOGVALET_SPACE_DYNAMODB_TABLE=logvalet-auth
LOGVALET_SPACE_FANOUT_CONCURRENCY=4
```

TokenStore と同じ DynamoDB table を使う場合は、命名を統合してもよい。

例:

```text
LOGVALET_AUTH_DYNAMODB_TABLE
```

既存の OAuth token store 設定がある場合は、それに合わせる。

### 15.2 config file

既存方針として remote MCP は zero config file を重視しているため、remote MCP では環境変数 + DynamoDB を使う。

ローカル CLI では既存 config.toml との互換のため、SpaceRegistry を sqlite または既存 config に保存できるようにする。

MVP では以下が現実的。

```text
local CLI:
  sqlite

remote MCP:
  dynamodb

test:
  memory
```

---

## 16. 後方互換性

### 16.1 既存コマンド

`--spaces` / `--all-spaces` を指定しない既存コマンドは、従来通り動作すること。

```bash
lv issue list
lv project list
lv space info
```

既存 profile / config がある場合は、それを使う。

### 16.2 既存 MCP tools

既存 MCP tool は、`spaces` / `all_spaces` 未指定で従来と同等の結果を返すこと。

ただし remote MCP で OAuth per-user が有効な場合は、ユーザー default space を解決する。

**【H4: MCP ツール引数スキーマ変更と既存 LLM プロンプトへの影響】**

```text
問題: spaces / all_spaces 引数を 65 ツール全てに追加すると、
      既存の Claude Code スキル定義（skills/ 配下）が
      新しい引数スキーマを知らない状態で動き続ける。

対策:
  1. spaces / all_spaces は optional（デフォルト null）で追加する。
     LLM が spaces: [] を明示的に渡した場合は「current/default space」として扱う
     （empty array は「指定なし」と同等）。
  2. スキルファイルの更新をマイルストーン（M12/M13）の完了条件に含める。
  3. 高頻度ツール（logvalet_issue_list 等）の Phase 2 対応完了後に
     skills/ ディレクトリのスキーマ同期タスクを追加する。

migration checklist への追加:
  □ skills/ の全スキル定義ファイルで spaces/all_spaces を認識するように更新
  □ スキル description に multi-space 操作の説明を追記
```

### 16.3 移行

既存ユーザーの移行パス:

```text
1. 既存 profile はそのまま利用可能
2. 新機能を使うユーザーは lv space add/connect を実行
3. 必要に応じて lv space import-profiles を実装
```

---

## 17. セキュリティ要件

### 17.1 ユーザー分離

remote MCP では必ず userID をキーにして registry / token / preference を分離する。

禁止事項:

```text
- userID 未指定で SpaceRegistry.List する
- all_spaces で全ユーザーの spaces を取得する
- alias だけで token を引く
- tenant だけで token を引く
```

必ず以下のキーを含める。

```text
userID
provider
tenant
```

### 17.2 ログ

ログに以下を出してはいけない。

```text
access_token
refresh_token
api_key
authorization header
OAuth code
raw state
```

ログに出してよいもの:

```text
user_id: ハッシュ化または内部ID
space alias
tenant
baseURL host
error_code
```

### 17.3 MCP tool の安全性

LLM が誤って全スペース write 操作を行わないよう、write 操作の multi-space は MVP では禁止する。

### 17.4 OAuth state

state は署名・期限付きにする。

state に含める userID は callback 時の authenticated userID と一致することを検証する。

---

## 18. テスト計画

### 18.1 Unit tests

#### Space Store

```text
- Upsert then Get
- Upsert overwrite
- List returns only target user spaces
- Delete removes only target user alias
- Different users can have same alias
- Preference get/put
```

#### Resolver

```text
- --all-spaces resolves all enabled user spaces
- --spaces foo resolves foo
- --spaces foo,bar resolves both in order
- unknown alias -> ErrSpaceNotFound
- no args + default set -> default
- no args + one registered space -> that space
- no args + multiple spaces + no default -> ErrNoDefaultSpace
- --spaces + --all-spaces -> ErrInvalidSpaceScope
```

#### BaseURL normalization

```text
- foo.backlog.com -> https://foo.backlog.com
- https://foo.backlog.com/ -> https://foo.backlog.com
- path/query/fragment rejected
```

#### Alias normalization

```text
- Foo -> foo
- invalid chars rejected
- empty alias rejected
```

#### ClientFactory

```text
- OAuth uses userID + tenant token
- API key uses space auth_profile
- missing credential -> not_configured
- unauthorized response -> unauthorized result
- different users same alias use different credentials
```

#### Executor

```text
- all success
- partial failure
- max concurrency respected
- result preserves input space order
- context cancellation
```

### 18.2 CLI tests

```text
lv space add
lv space list
lv space use
lv space verify
lv issue list --spaces foo
lv issue list --spaces foo,bar
lv issue list --all-spaces
lv issue list --spaces foo --all-spaces -> error
```

### 18.3 MCP tests

```text
- spaces omitted -> default space
- spaces ["foo"] -> foo
- all_spaces true -> user spaces only
- spaces + all_spaces -> error
- user A cannot access user B spaces
- same alias across two users resolves separately
```

### 18.4 Integration tests

httptest Backlog servers を複数立てる。

```text
server foo:
  expects Authorization: Bearer token-foo
  returns foo data

server bar:
  expects Authorization: Bearer token-bar
  returns bar data
```

テストケース:

```text
- OAuth token for foo and bar
- issue list --all-spaces returns both foo and bar
- one server returns 401 -> partial failure
- user A all_spaces does not include user B spaces
```

### 18.5 Security tests

```text
- logs do not contain access token
- logs do not contain refresh token
- logs do not contain API key
- state tampering rejected
- callback user mismatch rejected
```

---

## 19. 実装マイルストーン

### M01: Space domain model

対象:

```text
internal/space/types.go
internal/space/store.go
internal/space/errors.go
```

実装:

```text
- SpaceRegistration
- UserPreference
- Store interface
- errors
```

完了条件:

```text
go test ./internal/space/... が通る
```

### M02: Memory SpaceStore

対象:

```text
internal/space/memorystore.go
internal/space/memorystore_test.go
```

実装:

```text
- userID + alias で保存
- preference 保存
- user isolation
```

### M03: SQLite SpaceStore

対象:

```text
internal/space/sqlitestore.go
```

実装:

```text
- spaces table
- user_preferences table
- auto migration
```

### M04: DynamoDB SpaceStore

対象:

```text
internal/space/dynamodbstore.go
```

実装:

```text
PK = USER#<userID>
SK = SPACE#<alias>
SK = PREF
```

既存 TokenStore と同一テーブルを使うかは既存設計に合わせる。

### M05: Space Resolver

対象:

```text
internal/space/resolver.go
```

実装:

```text
- Scope
- Resolve
- validation
```

### M06: BaseURL / alias 正規化

対象:

```text
internal/space/normalize.go
```

実装:

```text
- NormalizeBaseURL
- DeriveAliasFromBaseURL
- ValidateAlias
```

### M07: Space-aware ClientFactory

対象:

```text
internal/auth/space_factory.go
```

実装:

```text
- OAuth client creation by userID + tenant
- API key/profile client creation by space registration
```

### M08: Fan-out Executor

対象:

```text
internal/space/executor.go
```

実装:

```text
- ExecuteAcrossSpaces
- partial failure
- concurrency
- order preservation
```

### M09: CLI global flags

対象:

```text
internal/cli/*
```

実装:

```text
- --spaces
- --all-spaces
- parse scope
- validation
```

### M10: CLI space commands

対象:

```text
internal/cli/space.go
```

実装:

```text
- lv space list
- lv space add
- lv space connect
- lv space use
- lv space remove
- lv space verify
```

### M11: read-only CLI 横断対応

対象:

```text
issue list/search/get
project list
space info/digest/disk usage
analysis 系
```

実装:

```text
- spaces scope resolve
- fan-out
- result envelope output
```

### M12: MCP common space args

対象:

```text
internal/mcp/*
```

実装:

```text
- spaces
- all_spaces
- parse scope
- error handling
```

### M13: MCP space management tools

対象:

```text
internal/mcp/tools_space_registry.go
```

実装:

```text
- logvalet_space_list
- logvalet_space_use
- logvalet_space_verify
- logvalet_space_connect_url
- logvalet_space_disconnect
```

### M14: OAuth callback registry upsert

対象:

```text
internal/auth/oauth handlers
internal/cli/mcp.go
```

実装:

```text
- callback success after token save
- SpaceRegistry.Upsert
- default space set if empty
```

### M15: remote MCP user isolation E2E

対象:

```text
integration tests
```

実装:

```text
- user A and user B same alias but different token
- all_spaces only returns own spaces
```

### M16: write 操作安全制御

対象:

```text
write tools / commands
```

実装:

```text
- MVP: multi-space write 禁止
- single explicit space write は許可
```

---

## 20. 期待する最終UX

### 20.1 CLI OAuth

```bash
lv space connect
lv space list
lv space use foo

lv issue list
lv issue list --spaces foo
lv issue list --spaces foo,bar
lv issue list --all-spaces
```

### 20.2 CLI API key

```bash
lv space add \
  --alias foo \
  --base-url https://foo.backlog.com \
  --auth-type apikey \
  --auth-profile foo

lv space add \
  --alias bar \
  --base-url https://bar.backlog.com \
  --auth-type apikey \
  --auth-profile bar

lv issue list --spaces foo,bar
```

### 20.3 MCP

```json
{
  "tool": "logvalet_issue_search",
  "arguments": {
    "query": "期限切れ",
    "all_spaces": true
  }
}
```

返却:

```json
{
  "results": [
    {
      "space": "foo",
      "ok": true,
      "result": {
        "issues": []
      }
    },
    {
      "space": "bar",
      "ok": false,
      "error_code": "unauthorized",
      "error": "unauthorized"
    }
  ]
}
```

---

## 21. 実装時の注意点

### 21.1 `--space` は新規作成しない（既存フラグは後方互換として残す）

**【§24 実装整合性メモ参照】** `--space` は `internal/cli/global_flags.go` に既存で存在するため削除しない。
新規 multi-space 操作は `--spaces`（複数形）を使う。

```bash
--spaces foo        # 推奨（新規）
--spaces foo,bar    # 複数
--space foo         # 後方互換のみ、非推奨
```

`GlobalFlags.Space != ""` かつ `GlobalFlags.Spaces == ""` の場合、
`Spaces = GlobalFlags.Space` として扱う（単一スペース backward compatibility）。

### 21.2 `current` という明示値は作らない

MCP/CLI ともに未指定を current/default とする。

### 21.3 API key の別スペース指定は space 単位で失敗させる

API key は単一スペースに紐づくため、別スペースを指定した場合は以下のいずれかになる。

```text
not_configured
unauthorized
forbidden
```

横断時は partial failure として扱う。

### 21.4 登録はユーザーごと

remote MCP は複数人で使うため、登録は必ずユーザーごと。

```text
USER#A SPACE#foo
USER#B SPACE#foo
```

は別物。

### 21.5 動的 discovery は限定的

Backlog に「アクセス可能な全スペース一覧」を取得するグローバル API は前提にしない。

可能な discovery:

```text
- OAuth callback 時に接続したスペースを自動登録
- 既存 profile から import
- lazy verify により status 更新
```

---

## 22. MVP スコープ

MVP で必ずやること:

```text
- SpaceRegistry (memory / sqlite / dynamodb)
- UserPreference default space
- --spaces / --all-spaces（GlobalFlags.Space との後方互換）
- lv spaces list/add/connect/use/verify（複数形コマンド。既存 lv space info は変更なし）
- OAuth callback 時の auto-register（StateClaims 拡張、MultiSpaceOAuthHandler 追加）
- API key space registration
- read-only 操作の横断対応（Executor.ExecuteAcrossSpaces）
- MCP spaces/all_spaces
- MCP user isolation
```

MVP でやらないこと:

```text
- 全 write 操作の multi-space 対応
- Backlog 全スペース自動 discovery
- 複雑な alias rename 履歴
- GUI 管理画面
```

---

## 23. 完了条件

以下を満たしたら完了。

```text
1. ローカル CLI で複数 API key space を登録できる
2. CLI で --spaces foo,bar により横断 read-only 操作ができる
3. CLI で --all-spaces により登録済み全スペースを対象にできる
4. OAuth callback 成功時に SpaceRegistry が自動更新される
5. remote MCP でユーザーごとに space registry が分離される
6. MCP all_spaces が現在ユーザーの登録済みスペースだけを対象にする
7. default space がユーザーごとに保存され、spaces 未指定時に使われる
8. API key で credential が無いスペースは not_configured になる
9. 認証失敗は space 単位の partial failure として返る
10. write 操作で multi-space 指定は安全に拒否される
11. token/API key がログに出ない
12. unit/integration/security tests が通る
```

---

## 24. 実装で発見した既存コードとの整合性メモ

> この章は Phase 4 spec refinement（architect）が実コードを読んで得た重要事実を記録する。
> 各セクションの実装判断の根拠として参照すること。

### 24.1 `--space` フラグが既存で存在する

`internal/cli/global_flags.go:39`

```go
Space string `short:"s" help:"specify Backlog space name directly (env: LOGVALET_SPACE)"`
```

→ §3.1 の「`--space` は作らない」は不正確。後方互換として残す（§3.1, §21.1 参照）。

### 24.2 `lv space` に既存コマンドが存在する（Backlog API 操作）

`internal/cli/space.go`:
- `SpaceInfoCmd` → `lv space info`
- `SpaceDiskUsageCmd` → `lv space disk-usage`
- `SpaceDigestCmd` → `lv space digest`

これらは Backlog `/api/v2/space` への操作。SpaceRegistry 管理コマンドは `lv spaces`（複数形）に配置する（§8.2 参照）。

### 24.3 DynamoDB TokenStore は単一 PK（SK なし）

`internal/auth/tokenstore/dynamodb.go:67`:

```go
func dynamoDBPK(userID, provider, tenant string) string {
    return "USER#" + userID + "#" + provider + "#" + tenant
}
```

DynamoDB テーブルは PK のみのフラットなスキーマ。`SK` は存在しない。
SpaceStore は別テーブルで実装する（§4.4 参照）。

### 24.4 OAuth StateClaims は UserID/Tenant/Nonce/Continue のみ

`internal/auth/state.go:23-29`:

```go
type StateClaims struct {
    UserID   string `json:"uid"`
    Tenant   string `json:"tenant"`
    Nonce    string `json:"nonce"`
    Continue string `json:"continue,omitempty"`
    jwt.RegisteredClaims
}
```

multi-space 登録用に `BaseURL`, `Alias` フィールドを追加する必要がある（§10.3 参照）。

### 24.5 OAuthHandler.tenant は constructor で固定

`internal/transport/http/oauth_handler.go:96-99`:

```go
type OAuthHandler struct {
    // ...
    tenant string  // constructor で固定
}
```

multi-space では tenant が登録ごとに変わるため、既存 `OAuthHandler` は変更せず
`MultiSpaceOAuthHandler` を新規実装する（§10 参照）。

### 24.6 auth.NewClientFactory は (provider, tenant, baseURL) が固定

`internal/auth/factory.go:22`:

```go
func NewClientFactory(tm TokenManager, provider, tenant, baseURL string) ClientFactory {
```

multi-space では tenant/baseURL が動的なため `SpaceAwareClientFactory` を追加する。
既存 `ClientFactory` は変更しない（§6.2 参照）。

### 24.7 Go はメソッドに型パラメータを付けられない

Go 1.26.1 でも `func (e *Executor) Execute[T any](...)` は構文エラー。
`ExecuteAcrossSpaces[T any]` を package-level 関数として実装する（§7.2 参照）。

### 24.8 既存 GlobalFlags.Spaces は未定義（追加が必要）

`internal/cli/global_flags.go` に `Spaces` フィールドと `AllSpaces` フィールドは存在しない。
新規追加が必要（§8.1 参照）。

**【H2: Kong の --spaces パース方式の確定】**

Kong で `string` 型の `--spaces` を使うと `--spaces foo --spaces bar`（複数回渡し）で
2回目の値が1回目を無言で上書きする問題がある。

採用方針: `Spaces` は `string` 型（comma-separated）とし、複数回渡しを**明示的にエラー**にする。

```go
// 追加するフィールド
Spaces    string `help:"comma-separated space aliases (e.g. foo,bar)" env:"LOGVALET_SPACES"`
AllSpaces bool   `help:"run against all registered spaces" env:"LOGVALET_ALL_SPACES"`
```

パース実装:

```go
// parseSpacesFlag は "--spaces foo,bar" を []string{"foo", "bar"} に変換する。
// 空要素・重複を除去し、エラーメッセージを明確にする。
func parseSpacesFlag(s string) ([]string, error) {
    if s == "" {
        return nil, nil
    }
    parts := strings.Split(s, ",")
    seen := make(map[string]bool)
    result := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p == "" {
            return nil, fmt.Errorf("--spaces: empty alias in %q", s)
        }
        if seen[p] {
            continue // 重複は静かにスキップ
        }
        seen[p] = true
        result = append(result, p)
    }
    return result, nil
}
```

`--spaces foo --spaces bar` パターンは Kong が単一 `string` フィールドに対して
後者で上書きするため、ユーザーに `--spaces foo,bar` を使うよう help に明記する。

### 24.9 SQLite TokenStore テーブル名は oauth_tokens

`internal/auth/tokenstore/sqlite.go:17`:
`CREATE TABLE IF NOT EXISTS oauth_tokens`

SpaceStore の SQLite テーブル名は `spaces`, `user_preferences` として衝突しない（§4.5 参照）。

### 24.10 MCP ToolRegistry は context から userID を取得済み

`internal/mcp/tools.go:55-70`: factory 経由で `auth.UserIDFromContext(ctx)` を使う仕組みが実装済み。
multi-space 対応では同じ context から userID を取得し、SpaceRegistry.List に渡せばよい。

### 24.11 provider.BacklogOAuthProvider.space はコンストラクタで決定

`internal/auth/provider/backlog.go:64-74`: `space` フィールドは constructor で設定。
multi-space 登録時は登録ごとに `NewBacklogOAuthProvider(spaceName, clientID, clientSecret, WithBaseURL(baseURL))` を呼ぶ。

### 24.12 `buildRunContext` の BaseURL 導出ロジック

`internal/cli/runner.go:75-80`:

```go
if baseURL == "" && resolved.Space != "" {
    baseURL = fmt.Sprintf("https://%s.backlog.com", resolved.Space)
}
```

これは `.backlog.com` 固定。カスタムドメインは BaseURL 直接指定が必要。
multi-space 対応後は `SpaceRegistration.BaseURL` を直接使うため、このフォールバックは不要になる。

---

## 25. 性能・コスト

### 25.1 DynamoDB RCU/WCU 見積もり

SpaceStore 用新規テーブル（logvalet-spaces）の想定アクセスパターン:

```text
List (spaces): PK=USER#<uid> の Query → 1 RCU per request（1KB 以内想定）
Get: GetItem → 0.5 RCU（eventually consistent）
Upsert: PutItem → 1 WCU
```

ユーザー 100 人 × spaces 3件 = 300 items。1 item ≈ 256B。
MCP request per user per minute = 10 → RCU = 100 users × 10 rpm = 1000 RCU/分 ≈ 17 RCU/秒。

オンデマンドキャパシティモードで十分（月 $0.25/百万 RCU）。

### 25.2 Executor 並列度制御

`ExecuteAcrossSpaces` では `golang.org/x/sync/semaphore` でゴルーチン数を制限する。

```go
sem := semaphore.NewWeighted(int64(maxConcurrency)) // デフォルト 4
```

各スペースへのリクエストは Backlog API の Rate Limit（300 req/min）を考慮し、
デフォルト 4 並列を推奨。環境変数 `LOGVALET_SPACE_FANOUT_CONCURRENCY` で変更可能。

### 25.3 SpaceStore キャッシュ

MVP では SpaceStore へのキャッシュは不要（DynamoDB のレイテンシは数ミリ秒）。
将来的にアクセス頻度が高い場合は TTL 付き in-process キャッシュを検討する。

---

## 26. 運用

### 26.1 ログ設計

```text
出力してよいもの:
  user_id（ハッシュ化推奨）
  space_alias
  tenant
  base_url のホスト部分のみ（https://foo.backlog.com → foo.backlog.com）
  error_code
  operation name

絶対に出してはいけないもの:
  access_token
  refresh_token
  api_key
  Authorization ヘッダ値
  OAuth code
  JWT state 生値
```

ログエントリ例（slog 形式）:

```go
slog.Info("space fan-out started",
    slog.String("user_id", userID),
    slog.Int("space_count", len(spaces)),
    slog.String("operation", "issue_list"),
)
slog.Warn("space fan-out partial failure",
    slog.String("user_id", userID),
    slog.String("space_alias", reg.Alias),
    slog.String("tenant", reg.Tenant),
    slog.String("error_code", result.ErrorCode),
)
```

### 26.2 メトリクス

CloudWatch または slog 構造化ログ経由で以下を記録する:

```text
space_fanout_total{operation, user_id_hash}
space_fanout_partial_failures{space_alias, error_code}
space_registry_list_latency_ms
tokenstore_get_latency_ms
```

### 26.3 トラブルシュート手順

**症状: `lv spaces verify` で特定スペースが `unauthorized`**
1. `lv spaces list` で space status を確認
2. OAuth の場合: `lv spaces connect --alias <alias>` で再認証
3. API key の場合: `--auth-profile` 設定と tokens.json を確認

**症状: `no_default_space` エラー**
1. `lv spaces list` で登録済みスペースを確認
2. `lv spaces use <alias>` で default を設定

**症状: MCP で `all_spaces` が 0件返る**
1. idproxy userID の確認（`auth.UserIDFromContext` の返り値）
2. SpaceStore の user_id キーが idproxy userID と一致しているか確認
3. DynamoDB console で `PK=USER#<userID>` で Query

---

## 27. セキュリティテスト具体ケース

### 27.1 ユーザー A/B 分離テスト

```go
// user A と user B が同じ alias "foo" で登録しているとき
// user A のリクエストで user B の token が使われないことを検証

func TestUserIsolation_SameAlias(t *testing.T) {
    store := NewMemorySpaceStore()
    
    // user A: foo -> token-A
    store.Upsert(ctx, &SpaceRegistration{UserID: "userA", Alias: "foo", Tenant: "foo"})
    tokenStoreA.Put(ctx, &TokenRecord{UserID: "userA", Tenant: "foo", AccessToken: "token-A"})
    
    // user B: foo -> token-B
    store.Upsert(ctx, &SpaceRegistration{UserID: "userB", Alias: "foo", Tenant: "foo"})
    tokenStoreB.Put(ctx, &TokenRecord{UserID: "userB", Tenant: "foo", AccessToken: "token-B"})
    
    // user A の context で factory を呼ぶ
    ctxA := auth.WithUserID(context.Background(), "userA")
    clientA, err := factory(ctxA, SpaceRegistration{UserID: "userA", Alias: "foo", Tenant: "foo"})
    require.NoError(t, err)
    
    // clientA が token-A を使っていることを確認
    // （httptest server に Authorization: Bearer token-A が届くことを検証）
}
```

### 27.2 state 改ざんテスト

```go
func TestOAuthState_TamperingRejected(t *testing.T) {
    // 正規 state を生成
    state, _ := auth.GenerateStateWithSpaceInfo("user1", "foo", "https://foo.backlog.com", "foo", secret, ttl)
    
    // state を改ざん（中身を decode → userID 書き換え → encode）
    tampered := tamperJWT(state, func(claims map[string]any) {
        claims["uid"] = "user2"
    })
    
    // ValidateState が失敗することを確認
    _, err := auth.ValidateState(tampered, secret)
    require.ErrorIs(t, err, auth.ErrStateInvalid)
}
```

### 27.3 user mismatch テスト

```go
func TestOAuthCallback_UserMismatch(t *testing.T) {
    // state は user1 で生成
    state, _ := auth.GenerateStateWithSpaceInfo("user1", "foo", ..., secret, ttl)
    
    // callback は user2 として実行
    req := httptest.NewRequest("GET", "/oauth/backlog/callback?code=abc&state="+state, nil)
    ctx := auth.WithUserID(req.Context(), "user2")
    req = req.WithContext(ctx)
    
    // 401 user_mismatch が返ることを確認
    rr := httptest.NewRecorder()
    handler.HandleCallback(rr, req)
    assert.Equal(t, 401, rr.Code)
    
    var resp map[string]string
    json.Unmarshal(rr.Body.Bytes(), &resp)
    assert.Equal(t, "user_mismatch", resp["error"])
}
```

### 27.4 all_spaces クロスユーザー漏洩防止テスト

```go
func TestAllSpaces_NoLeakAcrossUsers(t *testing.T) {
    store := NewMemorySpaceStore()
    
    // user A: foo, bar
    store.Upsert(ctx, &SpaceRegistration{UserID: "userA", Alias: "foo"})
    store.Upsert(ctx, &SpaceRegistration{UserID: "userA", Alias: "bar"})
    // user B: baz
    store.Upsert(ctx, &SpaceRegistration{UserID: "userB", Alias: "baz"})
    
    resolver := NewResolver(store)
    
    // user A の all_spaces は foo, bar のみ
    spaces, err := resolver.Resolve(ctx, "userA", Scope{AllSpaces: true})
    require.NoError(t, err)
    aliases := aliasesOf(spaces)
    assert.ElementsMatch(t, []string{"foo", "bar"}, aliases)
    // baz が含まれないことを確認
    assert.NotContains(t, aliases, "baz")
}
```

### 27.5 ログにトークンが含まれないことのテスト

```go
func TestExecutor_NoTokenInLogs(t *testing.T) {
    // slog の出力を capture するハンドラーを設定
    var logBuf bytes.Buffer
    handler := slog.NewTextHandler(&logBuf, nil)
    logger := slog.New(handler)
    
    // executor を実行（エラーケース含む）
    // ...
    
    // ログに "access_token", "refresh_token", "Bearer " が含まれないことを確認
    logOutput := logBuf.String()
    assert.NotContains(t, logOutput, "access_token")
    assert.NotContains(t, logOutput, "refresh_token")
    assert.NotContains(t, logOutput, "Bearer ")
    assert.NotContains(t, logOutput, "api_key")
}
```

---

## 28. エラー一覧（全対応表）

| エラーコード | 発生場所 | HTTP status | CLI exit code | MCP error envelope |
|---|---|---|---|---|
| `invalid_space_scope` | Resolver.Resolve | - | 2 (arg error) | `{"error": {"code": "invalid_space_scope"}}` |
| `space_not_found` | Resolver.Resolve | - | 2 | `{"error": {"code": "space_not_found"}}` |
| `no_spaces_registered` | Resolver.Resolve | - | 2 | `{"error": {"code": "no_spaces_registered"}}` |
| `no_default_space` | Resolver.Resolve | - | 2 | `{"error": {"code": "no_default_space"}}` |
| `not_configured` | SpaceAwareClientFactory | - | 6 (api error) | Result.ErrorCode = "not_configured" |
| `not_connected` | SpaceAwareClientFactory | - | 3 (auth error) | Result.ErrorCode = "not_connected" |
| `unauthorized` | Backlog API 401 | 401 | 3 | Result.ErrorCode = "unauthorized" |
| `forbidden` | Backlog API 403 | 403 | 4 (permission) | Result.ErrorCode = "forbidden" |
| `rate_limited` | Backlog API 429 | 429 | 6 | Result.ErrorCode = "rate_limited" |
| `backlog_error` | Backlog API 5xx | 502/503 | 6 | Result.ErrorCode = "backlog_error" |
| `partial_failure` | Executor (1件以上失敗) | - | **8** (partial failure) | Results に OK=false 混在 |
| `internal_error` | 内部エラー | 500 | 1 | `{"error": {"code": "internal_error"}}` |
| `multi_space_write_forbidden` | write ツール | - | 2 | `{"error": {"code": "multi_space_write_forbidden"}}` |
| `token_missing` | lv spaces verify | - | 3 | Result.ErrorCode = "token_missing" |

CLI exit code 定義（既存 CLAUDE.md より + multi-space 追加）:

**【H6: partial_failure の exit code を既存 2（argument error）と分離】**

```text
0: success
1: generic error
2: argument / validation error（--spaces/--all-spaces の不正指定など）
3: authentication error
4: permission error
6: API error
8: partial failure（複数スペース実行で一部失敗 ← 新規追加）
```

exit code 8 を新規追加する理由:
- exit code 2 は argument/validation error として既存スクリプトが使用している
- `--spaces foo --spaces bar` で bar が unauthorized の場合（実行時 partial）と
  `--spaces ""` のような入力エラー（argument error）をスクリプトから区別できる必要がある
- exit code 5（resource_not_found）は既存で使用済み、7 は空き（既存 CLAUDE.md では 7=未定義）
  → より意味が明確な 8 を割り当てる

---

## 29. GPT-5.4 指摘への対応サマリー

| 指摘 | 対応 | 該当セクション |
|---|---|---|
| カスタムドメインの OAuth 鶏卵問題 | alias/tenant 入力必須化、backlog.*/backlog.jp のみ自動導出 | §12.3 |
| `--space` を alias 化しない | 既存動作完全保持、`--spaces` のみ alias ベース | §3.1, §21.1 |
| OAuth callback の replay-safe 化 | alias 事前確定、Upsert idempotent、default に条件付き write | §10.3 |
| `Result[T]` の `*T` 問題 | `Result *T` に変更 | §7.1 |
| `apikey` vs `api_key` 統一 | 既存 credentials.go に合わせて `"api_key"` | §4.1 |
| DynamoDB tenant 重複防止 | GSI + Software Check（MVP）、削除は TransactWriteItems | §4.4 |
| binary/streaming ツールの multi-space 非対応 | download 系は Phase2 マトリクスで明示的に除外検討 | §13.1 |
| CLI と remote MCP で未指定時の意味を分ける | CLI は既存 profile/config 優先、MCP は default_space | §3.1 |
| migration: default_profile -> default_space_alias 初期化 | §29 migration path に追記 | 下記 |

---

## 30. backward compatibility テストマトリクス

既存 profile-only ユーザーが破壊されないことを保証するテスト。

### 30.1 テストケース一覧

```text
ID   シナリオ                                   期待結果
BC1  --spaces 未指定、既存 profile のみ存在     従来通り profile の space を使う（CLI のみ）
BC2  --space foo（既存フラグ）                  既存 buildRunContext の動作を保持（alias 解釈しない）
BC3  LOGVALET_SPACE=foo（env）                  従来通り動作する
BC4  config.toml の profile を使う              従来通り動作する
BC5  tokens.json の OAuth token を使う          従来通り動作する
BC6  SpaceRegistry が空でも profile があれば動く Resolver の fallback 5 が機能する
BC7  MCP spaces/all_spaces 未指定              従来と同等の結果を返す
BC8  auth_type="api_key"（既存）が新 SpaceStore で正しく解釈される
     auth_type は "api_key" で統一（"apikey" との混在なし）
BC9  default_profile あり、SpaceRegistry 空   CLI は default_profile を使い、
     （migration 前ユーザー）                 no_default_space エラーにならない
```

### 30.2 migration path の追加仕様（GPT-5.4 指摘を受けて追記）

```text
問題: default_profile を持つ既存ユーザーが lv spaces を使い始めるとき、
      default_space_alias が未設定で no_default_space になる。

対策:
  1. lv spaces list 実行時、SpaceRegistry が空であれば import 提案メッセージを出す
     例: "No spaces registered. Run 'lv spaces import-profiles' to migrate."
  2. default_space_alias 初期化ルール:
       import-profiles 時: default_profile が存在すれば、そのプロファイルの
       space を default_space_alias として設定する
  3. remote MCP deploy 順序を doc に明記:
       1. DynamoDB logvalet-spaces テーブル作成
       2. 新コード deploy
       3. ユーザーに lv spaces connect を案内
```

### 30.3 Resolver fallback 5 の実装メモ

§5.3 の「5. 既存 config/profile から backward compatibility fallback」は以下のように実装する。

```go
// fallback 5: SpaceRegistry が空だが既存 profile がある場合
// buildRunContext で使っていた config.Resolve の結果を SpaceRegistration として包む
func (r *Resolver) resolveFromLegacyProfile(
    ctx context.Context,
    resolvedCfg *config.ResolvedConfig,
) (*SpaceRegistration, error) {
    if resolvedCfg.BaseURL == "" && resolvedCfg.Space == "" {
        return nil, ErrNoDefaultSpace
    }
    baseURL := resolvedCfg.BaseURL
    if baseURL == "" {
        baseURL = fmt.Sprintf("https://%s.backlog.com", resolvedCfg.Space)
    }
    tenant := resolvedCfg.Space
    return &SpaceRegistration{
        UserID:      "local",
        Alias:       tenant,
        Tenant:      tenant,
        BaseURL:     baseURL,
        AuthType:    AuthTypeAPIKey, // profile は credential 解決済み
        AuthProfile: resolvedCfg.AuthRef,
    }, nil
}
```

---

## Updated by: devflow:team Phase 4 (architect) on 2026-05-20
## Updated by: devflow:team Phase 5 (devils-advocate CRITICAL 対応) on 2026-05-20
## Updated by: devflow:team Task #7 (architect — CRITICAL+HIGH 追加反映) on 2026-05-20

devils-advocate 批評レポート（`docs/specs/logvalet_multi_space_spec_review.md`）の CRITICAL 指摘（C1-C4）は devils-advocate が反映済み。
architect（Task #7）が HIGH 指摘と MEDIUM/LOW の一部を追加反映:
- H1: MaxConcurrency=0 deadlock 防止（§7.2）
- H2: Kong --spaces パース方式の確定（§24.8）
- H3: default space 削除後フォールバック仕様（§8.2.5）
- H4: MCP ツールスキーマ変更と LLM 影響（§16.2）
- H5: 同一 tenant 複数 alias の singleflight dedup 明記（§6.2）
- H6: partial_failure exit code を 8 に変更（§28）
- M2: §8.2 サブコマンド見出しを lv spaces（複数形）に統一
- M5: rate_limited ファンアウト時のリトライ方針追加（§7.4）
- L3: §8.2.1 コマンド例を lv spaces list に修正
- C1 追加: §15.3 に remote MCP store type validation セクション追加
