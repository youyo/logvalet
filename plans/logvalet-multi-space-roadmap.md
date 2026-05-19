# logvalet Multi-Space 対応 実装ロードマップ

> v0.21.0 以降、複数 Backlog スペース横断操作を実現するための実装計画。
> spec: `docs/specs/logvalet_multi_space_spec.md`
> 批評レポート: `docs/specs/logvalet_multi_space_spec_review.md`

---

## 概要

現在の logvalet (v0.21.0) は単一 Backlog スペースを前提とした設計。
本ロードマップでは、ユーザーが複数のBacklog スペースを登録・管理し、
CLI/MCP から横断操作できるようにするための実装を16マイルストーンで定義する。

devils-advocate 批評（CRITICAL 4件・HIGH 6件）の指摘を受け、spec が修正済みの前提で策定する。

---

## 現状サマリ (v0.21.0)

### 完了済みインフラ
- OAuth 認証基盤: `internal/auth/` (TokenManager, StateClaims, OAuthHandler)
  - singleflight によるトークンリフレッシュ並行性保護 済み
- MCP サーバー: idproxy + Streamable HTTP、65 tools 実装済み
- tokenstore: SQLite / DynamoDB / Memory（`internal/auth/tokenstore/`）
- GlobalFlags: `--space`, `--profile`, `--base-url` 等の既存フラグ
- `lv space info / disk-usage / digest` コマンド（Backlog API 操作）

### 未実装（本ロードマップ対象）
- `internal/space/` パッケージ全体（SpaceRegistration, Store, Resolver, Executor）
- `lv spaces` コマンド群（registry 管理）
- `--spaces / --all-spaces` グローバルフラグ
- MCP `spaces / all_spaces` 共通引数
- MultiSpaceOAuthHandler + state nonce 消費
- SpaceAwareClientFactory

---

## 目標アーキテクチャ

```
internal/space/
  types.go          — SpaceRegistration, UserPreference, SpaceStatus, AuthType
  store.go          — Store interface（NonceStore interface も含む: RH5 対応）
  errors.go         — ErrNoDefaultSpace 等、ExitCodePartialFailure=8 定義（RH2 対応）
  scope.go          — Scope struct
  resolver.go       — Resolver（5段階 fallback）
  normalize.go      — NormalizeBaseURL, DeriveAliasFromBaseURL, DeriveInitialTenant
  executor.go       — ExecuteAcrossSpaces[T any] (package-level func)
  memorystore.go    — テスト用インメモリ実装
  sqlitestore.go    — ローカル CLI 用 SQLite 実装（NonceStore SQLite 実装も含む: RC2 対応）
  dynamodbstore.go  — remote MCP 用 DynamoDB 実装（NonceStore DynamoDB 実装も含む）

internal/auth/
  space_factory.go  — SpaceAwareClientFactory（OAuth / APIKey 分岐）
  state.go          — StateClaims に BaseURL/Alias フィールド追加（後方互換）
  nonce.go          — NonceStore interface を internal/space から re-export（後方互換用エイリアス。
                      実体は internal/space/store.go に定義: RH5 対応）

internal/transport/http/
  multi_space_oauth_handler.go — MultiSpaceOAuthHandler（既存 OAuthHandler は変更なし）

internal/cli/
  spaces_registry.go — SpacesCmd (list/add/connect/remove/use/verify)
  global_flags.go    — Spaces/AllSpaces フィールド追加

internal/mcp/
  tools_space_registry.go — logvalet_space_{list,use,verify,connect_url,disconnect}
```

---

## マイルストーン一覧

| M# | タイトル | フェーズ | 依存 | 優先度 |
|----|---------|---------|------|--------|
| MS01 | Space domain model & errors（exit code 8 定義含む） | 基盤 | — | 必須 |
| MS02 | Memory SpaceStore + テスト | 基盤 | MS01 | 必須 |
| MS03 | BaseURL / Alias / Tenant 正規化 | 基盤 | MS01 | 必須 |
| MS04 | Space Resolver (5段階 fallback) | 基盤 | MS02, MS03 | 必須 |
| MS05 | SQLite SpaceStore + NonceStore SQLite | ストア | MS01 | 必須 |
| MS06a | NonceStore interface 定義 | 基盤 | MS01 | 必須 |
| MS06 | DynamoDB SpaceStore + NonceStore DynamoDB | ストア | MS01, MS06a | 必須 |
| MS07 | SpaceStore 設定 validation (C1対応) | セキュリティ | MS05, MS06 | 必須 |
| MS08 | SpaceAwareClientFactory | 認証 | **MS01** | 必須 |
| MS09 | ExecuteAcrossSpaces fan-out Executor | 実行基盤 | MS08 | 必須 |
| MS10 | StateClaims 拡張 + MultiSpaceOAuthHandler + Nonce (C2/C3対応) | 認証 | MS06, MS06a, MS08 | 必須 |
| MS11 | CLI: --spaces / --all-spaces グローバルフラグ | CLI | MS04 | 必須 |
| MS12 | CLI: lv spaces 管理コマンド | CLI | MS08, MS10, MS11 | 必須 |
| MS13 | CLI: read-only コマンドの横断対応 | CLI | MS09, MS12 | 必須 |
| MS14 | MCP: spaces/all_spaces 共通引数 + space 管理 tools | MCP | MS09, **MS10** | 必須 |
| MS15 | backward compatibility テスト | テスト | MS13, MS14 | 必須 |
| MS16 | E2E・セキュリティテスト + ドキュメント | 品質 | MS15 | 必須 |

> **RC1 修正**: MS08 の依存を `MS04` → `MS01` に変更（SpaceAwareClientFactory は Resolver に依存しない）
> **RC2 修正**: MS06a（NonceStore interface）を新規追加。MS05 に NonceStore SQLite 実装を統合。
> **RC3 修正**: MS14 の依存に MS10 を追加（connect_url tool が MultiSpaceOAuthHandler に依存）

---

## 各マイルストーン詳細

### MS01: Space domain model & errors

**目的:** 型定義と error 定数を確立し、後続すべてのマイルストーンの土台にする。

**対象ファイル:**
- `internal/space/types.go`
- `internal/space/store.go`
- `internal/space/errors.go`
- `internal/space/scope.go`

**実装内容:**
```go
// types.go
type AuthType string
const (
    AuthTypeOAuth  AuthType = "oauth"
    AuthTypeAPIKey AuthType = "api_key"
)
type SpaceStatus string  // unknown/ok/unauthorized/not_connected/disabled
type SpaceRegistration struct { UserID, Alias, Tenant, BaseURL string; AuthType AuthType; ... }
type UserPreference struct { UserID, DefaultSpaceAlias string; ... }

// store.go
type Store interface {
    List/Get/Upsert/Delete/GetPreference/PutPreference/Close
}

// NonceStore interface も store.go に定義する（RH5: internal/space 内で完結させパッケージ循環防止）
type NonceStore interface {
    Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
    Consume(ctx context.Context, userID, nonce string) error  // 使用済みなら ErrNonceAlreadyUsed
}
var ErrNonceAlreadyUsed = errors.New("nonce already consumed")

// errors.go
var ErrNoSpacesRegistered, ErrNoDefaultSpace, ErrSpaceNotFound, ErrInvalidSpaceScope

// exit code 定義（RH2: MS16 先送りを防ぐため MS01 で確立する）
// 既存 internal/app/exitcode.go の定数と整合する形で追加
const ExitCodePartialFailure = 8  // partial failure（exit code 2 = argument error と分離）

// scope.go
type Scope struct { Aliases []string; AllSpaces bool }
```

**disabled space の挙動定義（RH3 対応）:**
```text
SpaceStatusDisabled の扱い（MS01 で確立・MS04/MS12 で実装）:
  - Resolver: disabled space は enabled のみフィルタから除外（all_spaces でも返さない）
  - lv spaces verify: disabled space はスキップし結果に含めない
  - disabled を設定する CLI コマンドは MVP スコープ外（remove のみ対応）
  - SpaceStatusDisabled が設定される唯一のケース: 将来の自動無効化機能（MVP 除外）
  → MVP では SpaceStatusDisabled は型として存在するが、設定・利用コマンドは実装しない
```

**完了条件:**
- `go test ./internal/space/...` が通る（型とエラーの基本 test）
- `go build ./...` が通る
- `ExitCodePartialFailure = 8` が定義され、既存 exit code 定数との衝突がないこと
- `NonceStore` interface が `internal/space/store.go` に定義されていること

---

### MS02: Memory SpaceStore + テスト

**目的:** テスト・開発用のインメモリ Store を実装し、Store interface の振る舞いを確立する。

**対象ファイル:**
- `internal/space/memorystore.go`
- `internal/space/memorystore_test.go`

**テストケース:**
- Upsert → Get でデータが取得できる
- List はリクエスト userID のみ返す（別 userID のデータ混入しない）
- Delete は対象 userID+alias のみ削除
- 同一 alias を別 userID で持てる
- Preference の get/put

**完了条件:** 上記テストが全て pass

---

### MS03: BaseURL / Alias / Tenant 正規化

**目的:** スペース登録時の入力を安全に正規化する関数を実装する。

**対象ファイル:**
- `internal/space/normalize.go`
- `internal/space/normalize_test.go`

**実装関数:**
```go
func NormalizeBaseURL(raw string) (string, error)
func DeriveAliasFromBaseURL(baseURL string) (string, error)
func DeriveInitialTenant(baseURL string) (string, error)  // カスタムドメイン → ""
func ValidateAlias(alias string) error
```

**テストケース:**
- `foo.backlog.com` → `https://foo.backlog.com`
- `https://foo.backlog.com/` → `https://foo.backlog.com`（末尾スラッシュ除去）
- path/query 付きは拒否
- `Foo` → `foo`（alias 小文字化）
- 無効文字（スペース等）は拒否
- `*.backlog.com` → tenant = サブドメイン第一ラベル
- `*.backlog.jp` → tenant = サブドメイン第一ラベル
- カスタムドメイン → tenant = "" (GetSpace が必要)

---

### MS04: Space Resolver (5段階 fallback)

**目的:** scope → `[]SpaceRegistration` の解決ロジックを実装する。

**対象ファイル:**
- `internal/space/resolver.go`
- `internal/space/resolver_test.go`

**解決優先順位（spec §5.3）:**
1. scope.AllSpaces == true → enabled spaces 全件
2. len(scope.Aliases) > 0 → 指定 alias 解決
3. preference.DefaultSpaceAlias → default
4. enabled space が 1件 → それを使う
5. legacy profile fallback → SpaceRegistration として包む
6. ErrNoDefaultSpace

**テストケース:**
- `--all-spaces` で enabled のみ返す（disabled は除外）
- `--spaces foo,bar` で順序保持
- 未登録 alias → ErrSpaceNotFound
- `--spaces` + `--all-spaces` 同時 → ErrInvalidSpaceScope

---

### MS05: SQLite SpaceStore + NonceStore SQLite（RC2 対応）

**目的:** ローカル CLI 向け SQLite 実装を提供する。NonceStore SQLite 実装も同一マイルストーンで実装する（同じ SQLite ファイルを共有し、管理が一元化されるため）。

**対象ファイル:**
- `internal/space/sqlitestore.go`
- `internal/space/sqlitestore_test.go`

**テーブル:**
```sql
CREATE TABLE spaces (user_id TEXT, alias TEXT, tenant TEXT, ...);
CREATE TABLE user_preferences (user_id TEXT PRIMARY KEY, default_space_alias TEXT, ...);

-- NonceStore SQLite 実装用テーブル（RC2: MS05 に統合）
CREATE TABLE nonces (
    user_id TEXT NOT NULL,
    nonce   TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (user_id, nonce)
);
```

**SQLite NonceStore 実装:**
```go
// sqlitestore.go に追加（SQLiteStore が NonceStore interface を実装）
func (s *SQLiteStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
func (s *SQLiteStore) Consume(ctx context.Context, userID, nonce string) error
```

**完了条件:**
- MemoryStore のテストケースと同じテストが SQLite Store でも pass
- NonceStore.Consume の二重消費がエラーになる
- `go test ./internal/space/...` が通る

---

### MS06a: NonceStore interface 定義（RC2 追加）

**目的:** NonceStore interface と error 定数を先行定義し、MS10（MultiSpaceOAuthHandler）が MS06 完了前から着手可能にする。

**依存:** MS01

**対象ファイル:**
- `internal/space/store.go`（NonceStore interface を追加 — MS01 で定義済みなら不要。MS01 が完了していれば本 MS は実質不要だが、依存確認のため独立させる）

**内容:**
```go
// store.go（MS01 で定義済みの場合は MS06a は空の「確認マイルストーン」として扱う）
type NonceStore interface {
    Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
    Consume(ctx context.Context, userID, nonce string) error
}
var ErrNonceAlreadyUsed = errors.New("nonce already consumed")
```

**完了条件:**
- `internal/space.NonceStore` interface が利用可能
- `go build ./...` が通る

---

### MS06: DynamoDB SpaceStore + NonceStore DynamoDB（RC2 修正）

**目的:** remote MCP 向け DynamoDB 実装と、nonce 消費（C3 対応）の DynamoDB 実装を提供する。NonceStore interface は MS06a（または MS01）で定義済みとする。

**依存:** MS01, MS06a

**対象ファイル:**
- `internal/space/dynamodbstore.go`
- `internal/space/dynamodbstore_test.go`（localstack 使用）

**DynamoDB キー設計:**
```
logvalet-spaces テーブル:
  PK=USER#<uid>, SK=SPACE#<alias>  → SpaceRegistration
  PK=USER#<uid>, SK=PREF            → UserPreference
  PK=USER#<uid>, SK=NONCE#<nonce>  → NonceRecord (TTL 付き、条件付き Delete で consume-once)
```

**DynamoDB NonceStore 実装:**
```go
// dynamodbstore.go に追加（DynamoDBStore が NonceStore interface を実装）
// Consume: ConditionExpression = "attribute_exists(pk)" で Delete
//          → KeyNotFound = ErrNonceAlreadyUsed（replay rejected）
func (s *DynamoDBStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
func (s *DynamoDBStore) Consume(ctx context.Context, userID, nonce string) error
```

**NonceStore パッケージ配置（RH5 対応）:**
```text
interface: internal/space/store.go（循環依存なし）
DynamoDB 実装: internal/space/dynamodbstore.go（同一パッケージ）
SQLite 実装:  internal/space/sqlitestore.go（MS05 で実装済み）
```

**localstack セットアップ（RL2 対応）:**
- `docs/development.md` に localstack 起動手順を記載する
- CI では `testcontainers-go` または `docker-compose.test.yml` で自動起動

**完了条件:**
- MemoryStore と同じインターフェーステストが DynamoDB Store でも pass（localstack）
- Nonce の二重消費がエラーになる
- localstack セットアップ手順が `docs/development.md` に記載済み

---

### MS07: SpaceStore 設定 validation (C1 対応)

**目的:** remote MCP + SQLite Store の組み合わせを起動時に拒否する。

**対象ファイル:**
- `internal/space/config.go`（StoreType, ValidateSpaceStoreConfig）
- `internal/cli/mcp.go`（起動時 ValidateSpaceStoreConfig 呼び出し）

**実装:**
```go
func ValidateSpaceStoreConfig(storeType string, isMCPRemote bool) error {
    if storeType == "sqlite" && isMCPRemote {
        return fmt.Errorf("SQLite SpaceStore は remote MCP モードで使用できません。dynamodb を指定してください")
    }
    return nil
}
```

**isMCPRemote の判定:**
- `McpCmd.Auth == true`（idproxy 認証が有効）の場合は remote MCP とみなす

**完了条件:**
- `lv mcp --auth --space-store sqlite` が起動時エラーになる
- `lv mcp --space-store sqlite`（非認証 = ローカル用途）は通る

---

### MS08: SpaceAwareClientFactory（RC1 修正）

**目的:** SpaceRegistration から backlog.Client を生成するファクトリを実装する。

**依存:** MS01（SpaceRegistration 型の定義のみ。Resolver は不要: RC1 対応）

**対象ファイル:**
- `internal/auth/space_factory.go`
- `internal/auth/space_factory_test.go`

**実装:**
```go
type SpaceAwareClientFactory func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error)

func NewSpaceAwareClientFactory(tm TokenManager, credResolver credentials.Resolver) SpaceAwareClientFactory
```

**内部分岐:**
- `reg.AuthType == "oauth"` → `tm.GetValidToken(ctx, userID, "backlog", reg.Tenant)` → Bearer client
- `reg.AuthType == "api_key"` → `credResolver.Resolve(reg.AuthProfile)` → APIKey client

**完了条件:**
- OAuth / APIKey 両方のパスでテスト pass
- credential なし → `not_configured` エラー
- 異なる userID が同じ alias でも別 token を使う（user isolation テスト）

---

### MS09: ExecuteAcrossSpaces fan-out Executor

**目的:** 複数スペースへの並列実行・partial failure・順序保持を実装する。

**対象ファイル:**
- `internal/space/executor.go`
- `internal/space/executor_test.go`

**実装:**
```go
type Executor struct {
    Factory        auth.SpaceAwareClientFactory
    MaxConcurrency int  // <= 0 の場合は defaultMaxConcurrency (4) にフォールバック
}

func ExecuteAcrossSpaces[T any](
    ctx context.Context,
    executor *Executor,
    spaces []SpaceRegistration,
    fn func(context.Context, SpaceRegistration, backlog.Client) (T, error),
) []Result[T]
```

**テストケース:**
- 全成功: `[]Result[T]` の len == len(spaces)、全て OK=true
- partial failure: 1件失敗でも他は続行
- MaxConcurrency=0 はデフォルト値 (4) にフォールバック（deadlock しない）
- 入力順を保持した結果が返る
- context cancellation で goroutine リークしない

---

### MS10: StateClaims 拡張 + MultiSpaceOAuthHandler + Nonce (C2/C3 対応)

**目的:** multi-space OAuth 登録フローを実装し、nonce replay 防止と書き込み順序保証を実現する。

**対象ファイル:**
- `internal/auth/state.go`（StateClaims に BaseURL/Alias 追加）
- `internal/transport/http/multi_space_oauth_handler.go`（既存 OAuthHandler は変更なし）

**StateClaims 拡張（後方互換）:**
```go
type StateClaims struct {
    UserID   string `json:"uid"`
    Tenant   string `json:"tenant"`
    Nonce    string `json:"nonce"`
    Continue string `json:"continue,omitempty"`
    BaseURL  string `json:"base_url,omitempty"`  // 追加
    Alias    string `json:"alias,omitempty"`      // 追加
    jwt.RegisteredClaims
}
```

**MultiSpaceOAuthHandler の callback 処理順序（C2 対応）:**
```
1. state JWT 検証（署名・期限）
2. userID 一致検証
3. nonce 消費（NonceStore.Consume → 失敗なら 400 replay error）
4. code exchange → token 取得
5. TokenStore.Put（先に保存）→ 失敗なら 500
6. GetCurrentUser / GetSpace で検証
7. SpaceRegistry.Upsert（べき等なので失敗しても再実行可能）
8. UserPreference 条件付き更新（default_space_alias == "" なら設定）
```

**完了条件:**
- state 改ざんで 401
- userID mismatch で 401
- nonce replay で 400
- 正常 callback で SpaceRegistry にスペースが登録される
- TokenStore 書き込み先 → SpaceRegistry 書き込み順が保証される

---

### MS11: CLI --spaces / --all-spaces グローバルフラグ

**目的:** 全コマンドで使える `--spaces` / `--all-spaces` フラグを追加する。

**対象ファイル:**
- `internal/cli/global_flags.go`（Spaces/AllSpaces フィールド追加）
- `internal/cli/global_flags_test.go`

**追加フィールド:**
```go
Spaces    string `help:"comma-separated space aliases" env:"LOGVALET_SPACES"`
AllSpaces bool   `help:"run against all registered spaces" env:"LOGVALET_ALL_SPACES"`
```

**バリデーション:**
```
--spaces ""      → エラー
--spaces ","     → エラー
--spaces "foo,"  → エラー
--spaces foo --all-spaces → エラー（ErrInvalidSpaceScope）
```

**Kong 二重渡し対策（RH1 対応）:**
```text
--spaces foo --spaces bar の場合:
  GlobalFlags.Spaces は string 型のため Kong は後者（bar）で上書きする。
  ユーザーへのサイレント上書きを防ぐため、help に以下を明記する:
    「複数スペースは --spaces foo,bar のように comma-separated で指定してください。
      --spaces を複数回指定すると最後の値のみ有効になります。」

テストケースに追加:
  --spaces foo --all-spaces → ErrInvalidSpaceScope（既存）
  parseSpacesFlag("foo,,bar") → エラー（空要素を含む）
  parseSpacesFlag("foo,foo") → ["foo"]（重複は静かにスキップ）
  parseSpacesFlag("") → nil（指定なし扱い）
```

**後方互換性:**
```go
// GlobalFlags.Space（既存）と GlobalFlags.Spaces（新規）の共存
// --space foo かつ --spaces 未指定なら従来の buildRunContext を使う
// --spaces または --all-spaces 指定時のみ SpaceResolver を使う
```

---

### MS12: CLI lv spaces 管理コマンド

**目的:** `lv spaces` 管理コマンド群（list/add/connect/remove/use/verify）を実装する。

**対象ファイル:**
- `internal/cli/spaces_registry.go`

**コマンド一覧（rename は MVP 除外）:**
- `lv spaces list` → 登録済みスペース一覧（JSON / Text）
- `lv spaces add` → API key 登録（対話式 / 非対話式）
- `lv spaces connect` → OAuth 登録（Browser フロー）
- `lv spaces remove <alias>` → registry から削除（TransactWriteItems で PREF も更新）
- `lv spaces use <alias>` → default space 設定
- `lv spaces verify [--spaces / --all-spaces]` → 接続確認

**lv spaces verify の強化（C2 対応）:**
- TokenStore に token が存在しない場合は `error_code="token_missing"` を返す
- 「`lv spaces connect --alias <alias>` を再実行してください」メッセージを付与

**完了条件:**
- `lv spaces list` で登録済みスペースが表示される
- `lv spaces add / connect / remove / use` の基本フローが動作する
- `lv spaces verify --all-spaces` で各スペースの接続状態が確認できる

---

### MS13: CLI read-only コマンドの横断対応

**目的:** `lv issue list` 等の read-only コマンドで `--spaces / --all-spaces` が機能するようにする。

**対象ファイル:**
- `internal/cli/runner.go`（buildRunContext の拡張）
- `internal/cli/issue.go` 等（各コマンドに fan-out 処理を追加）

**対応コマンド（MVP）:**
```
lv issue list / get / context / stale
lv project list / get / health / blockers
lv space info / disk-usage / digest
lv digest daily / weekly / unified
lv activity list / stats / digest
```

**出力形式:**
```
--spaces / --all-spaces 指定:
  []Result[T] の space result envelope 形式

--spaces / --all-spaces 未指定:
  既存出力形式（変更なし）
```

**完了条件:**
- `lv issue list --spaces foo,bar` が foo と bar の結果を配列で返す
- `lv issue list --all-spaces` が登録済み全スペースの結果を返す
- partial failure で一部スペースが失敗しても他スペースの結果は返る

---

### MS14: MCP spaces/all_spaces 共通引数 + space 管理 tools

**目的:** MCP の全 read-only ツールに spaces/all_spaces 引数を追加し、space 管理 tools を実装する。

**対象ファイル:**
- `internal/mcp/tools_space_registry.go`（新規: space 管理 tools）
- 各 `tools_*.go`（spaces/all_spaces 引数追加）

**新規 MCP tools:**
```
logvalet_space_list
logvalet_space_use
logvalet_space_verify
logvalet_space_connect_url
logvalet_space_disconnect
```

**既存 tools への引数追加（MVP スコープのみ）:**
```json
{
  "spaces": { "type": "array", "items": {"type": "string"}, ... },
  "all_spaces": { "type": "boolean", ... }
}
```

**後方互換性:**
- 両引数は optional のままにする
- 未指定時は従来の default space / profile の動作を保持

**完了条件:**
- `logvalet_space_list` でユーザーの登録済みスペースが返る
- `logvalet_issue_list` に `spaces: ["foo", "bar"]` を渡すと横断結果が返る
- 他ユーザーのスペース情報が漏洩しない（user isolation テスト）

---

### MS15: backward compatibility テスト

**目的:** 既存ユーザーのフローが壊れていないことを全テストケースで確認する。

**対象ファイル:**
- `internal/e2e/` または `internal/cli/*_test.go` に BC テストを追加

**テストケース（spec §30.1）:**
```
BC1  --spaces 未指定、既存 profile のみ → 従来通り profile の space を使う
BC2  --space foo（既存フラグ）           → 既存 buildRunContext の動作を保持
BC3  LOGVALET_SPACE=foo（env）           → 従来通り動作する
BC4  config.toml の profile を使う       → 従来通り動作する
BC5  tokens.json の OAuth token を使う  → 従来通り動作する
BC6  SpaceRegistry が空でも profile があれば動く（Resolver fallback 5）
BC7  MCP spaces/all_spaces 未指定       → 従来と同等の結果を返す
BC8  auth_type="api_key" が新 SpaceStore で正しく解釈される
BC9  default_profile あり、SpaceRegistry 空 → no_default_space にならない
```

---

### MS16: E2E・セキュリティテスト + ドキュメント

**目的:** user isolation・replay attack 防止・ログ秘密漏洩なしを自動テストで証明し、ドキュメントを整備する。

**テストケース（spec §27, §18.4）:**
```
user A/B 分離: user A の all_spaces に user B の spaces が含まれない
state 改ざん: 改ざんされた state は ValidateState がエラーを返す
callback user mismatch: userID 不一致で 401
nonce replay: 同じ callback 2回目は 400
ログにトークン含まれない: slog capture で access_token/refresh_token/Bearer が出ない
```

**ドキュメント:**
- README への multi-space セクション追加
- `lv spaces connect / add` のユースケース例
- remote MCP 運用ガイド（DynamoDB テーブル作成順序 → spec §30.2）

---

## 実装優先順位の根拠

### なぜこの順番か

1. **MS01-MS04（基盤層）を先行させる理由:** 後続の全マイルストーンが Store interface と Resolver に依存するため。TDD では Memory Store を先に実装し、テストで仕様を固める。
2. **MS05-MS06（ストア層）を基盤層と並行可能:** SQLite と DynamoDB の実装は Store interface に対してテストするため、基盤層（MS01-MS02）が完了すれば着手できる。
3. **MS07（Validation）を MS05/06 直後に入れる理由:** C1 の SQLite 漏洩リスクは起動時 validation で防ぐ必要があり、実際に MCP コマンドに接続する前に確立しておく必要がある。
4. **MS08-MS10（認証・実行層）は基盤後:** ClientFactory と Executor は Store と Resolver が揃ってから実装する。
5. **MS11-MS14（CLI/MCP）は実行基盤後:** Executor が動いてから CLI と MCP に接続する。
6. **MS15-MS16（品質保証）は最後:** 実装が揃ってから backward compatibility と security を保証する。

### 並列実行可能な組み合わせ（RC1・RC3 修正後）

> 依存表が single source of truth。このダイアグラムは依存表から派生。

```
MS01 完了後（並列可能）:
  MS02（Memory Store）
  MS03（正規化）
  MS05（SQLite Store + NonceStore SQLite）← MS01 さえあれば着手可能
  MS06a（NonceStore interface 定義）← MS01 さえあれば着手可能
  MS08（ClientFactory）← MS01 さえあれば着手可能 ★RC1 修正: MS04 待ち不要
  ↓

MS02 + MS03 完了後:
  MS04（Resolver）
  ↓

MS01 + MS06a 完了後:
  MS06（DynamoDB Store + NonceStore DynamoDB）← MS01, MS06a さえあれば着手可能
  ↓

MS05 + MS06 完了後:
  MS07（Validation）← ★RC3 修正: MS04 ではなく MS05+MS06 が必要
  ↓

MS06 + MS06a + MS08 完了後:
  MS10（OAuth Handler）← MS06 + MS06a + MS08 完了後
  ↓

MS08 完了後:
  MS09（Executor）← MS08 完了後
  ↓

MS04 完了後:
  MS11（CLI flags）← MS04 完了後
  ↓

MS09 + MS10 完了後:
  MS14（MCP tools）← ★RC3 修正: MS09 + MS10 両方が必要（connect_url は MS10 依存）
  ↓

MS08 + MS10 + MS11 完了後:
  MS12（lv spaces コマンド）← MS08, MS10, MS11 完了後
  ↓

MS09 + MS12 完了後:
  MS13（read-only 横断対応）
  ↓

MS13 + MS14 完了後:
  MS15（BC テスト）
  ↓

MS15 完了後:
  MS16（E2E・セキュリティ）
```

---

## リスクと対策

| リスク | 深刻度 | 対策 |
|--------|--------|------|
| SQLite + remote MCP の誤設定（C1） | CRITICAL | MS07 で起動時 validation 必須 |
| TokenStore/SpaceRegistry 非 atomic 書き込み（C2） | CRITICAL | MS10 で書き込み順序固定 + token_missing エラーコード |
| OAuth state replay attack（C3） | CRITICAL | MS06a+MS06+MS10 で Nonce Store 実装（MVP 必須） |
| lv spaces rename の DynamoDB 非 atomic 問題（C4） | CRITICAL | MVP 除外。rename 不要な UX を提供 |
| MS08 依存関係の誤り（RC1） | CRITICAL | 修正済み: MS08 依存 MS04→MS01。並列化ダイアグラム更新 |
| MS06 過負荷（RC2） | CRITICAL | 修正済み: MS06a 新設、MS05 に NonceStore SQLite 追加 |
| 依存表とダイアグラム矛盾（RC3） | CRITICAL | 修正済み: MS07→MS05+MS06、MS14→MS09+MS10 に統一 |
| MaxConcurrency=0 deadlock（H1） | HIGH | MS09 で <= 0 → デフォルト値フォールバック |
| Kong --spaces 複数渡し（H2・RH1） | HIGH | MS11 で string 型 + comma-split 統一 + テストケース追加 |
| default space 削除後の状態未定義（H3） | HIGH | MS12 remove 実装時に仕様化 |
| 65ツール schema 変更による既存 LLM 破壊（H4） | HIGH | optional 引数のみ、既存スキーマ変更なし |
| rate limit fan-out 時の対策なし（M5） | MEDIUM | MVP: 429 → rate_limited として partial failure 扱い |
| partial_failure exit code 2 の衝突（H6・RH2） | HIGH | **MS01 で** exit code 8 を定義（MS16 先送り解消） |
| NonceStore パッケージ循環依存リスク（RH5） | HIGH | internal/space/store.go に interface 定義（循環なし） |
| SpaceStatus=disabled 時 verify 挙動未定義（RH3） | HIGH | MS01 で挙動定義、MS04/MS12 で実装 |
| MCP 17ツールへの引数追加実装量（RH4） | HIGH | MS14 前に共通 space 解決ミドルウェア設計を追加 |

---

## テスト戦略

### ユニットテスト（各マイルストーンで実施）
- Store interface のテストは MemoryStore で共通化し、SQLite/DynamoDB でも同じテストを走らせる
- Resolver の全 fallback パスをテスト
- ExecuteAcrossSpaces の concurrent テスト（race detector 有効）

### 統合テスト（MS15/MS16）
- httptest で複数の Backlog サーバーを立てて fan-out をテスト
- localstack で DynamoDB テスト

### セキュリティテスト（MS16）
- slog capture によるログ秘密漏洩テスト
- user isolation テスト（user A/B 分離）
- nonce replay テスト
- state 改ざんテスト

---

## backward compatibility 方針

既存ユーザーへの影響ゼロを保証する:

1. `--spaces / --all-spaces` 未指定時は既存パス（`buildRunContext`）を完全保持
2. `--space`（既存フラグ）は alias ベースに変更しない（literal space name のまま）
3. MCP の `spaces / all_spaces` 引数は optional で、未指定時は従来動作
4. SpaceRegistry が空でも既存 profile/config があれば動く（Resolver fallback 5）
5. auth_type="api_key" の値は既存 credentials.go の "api_key" 表記と統一済み

---

## フェーズ完了条件

### MVP 完了（MS01-MS16 全完了）
spec §23 の完了条件12項目を全て満たすこと:
1. ローカル CLI で複数 API key space を登録できる
2. `lv issue list --spaces foo,bar` で横断 read-only 操作ができる
3. `lv issue list --all-spaces` が動作する
4. OAuth callback 成功時に SpaceRegistry が自動更新される
5. remote MCP でユーザーごとに space registry が分離される
6. MCP all_spaces が現在ユーザーのスペースのみ対象にする
7. default space がユーザーごとに保存される
8. credential なしスペースは not_configured になる
9. 認証失敗は partial failure として返る
10. write 操作で multi-space 指定は拒否される
11. token/API key がログに出ない
12. unit/integration/security tests が通る

---

## 次工程への引き継ぎ

各マイルストーンの実装プランは以下のファイルで管理:
- `plans/logvalet-ms{01-16}-{slug}.md`

実装着手前に各マイルストーンの実装プランを作成し、devflow:plan でレビューすること。

並列実行可能なマイルストーン（MS02+MS03、MS05+MS06 等）は
devflow:team の並列実装ループで処理すること。

---

*Created by: devflow:team Phase 5 (team-lead) on 2026-05-20*
*Updated by: devflow:team Task #8 (architect) on 2026-05-20 — RC1/RC2/RC3 + RH1-RH5 反映*

**変更サマリー（Task #8）:**
- RC1: MS08 依存を `MS04` → `MS01` に修正（Resolver は不要）。並列化ダイアグラムで MS08 が MS01 直後から着手可能に更新
- RC2: MS06a（NonceStore interface のみ）を新規追加。MS05 に NonceStore SQLite 実装を統合。MS06 は DynamoDB 実装専任に
- RC3-1: MS07 の依存をダイアグラムで `MS05, MS06` に統一（MS04 を削除）
- RC3-2: MS14 の依存ダイアグラムに MS10 を追加（connect_url tool が MultiSpaceOAuthHandler 依存）
- RH1: MS11 に Kong 二重渡し（--spaces foo --spaces bar）のテストケースと help 文言を追加
- RH2: partial_failure exit code 8 の定義を MS16 から MS01 に前倒し
- RH3: SpaceStatusDisabled 時の verify 挙動を MS01 で定義（disabled は Resolver/verify から除外）
- RH5: NonceStore interface を `internal/space/store.go` に配置（循環依存防止）
- RL2: MS06 完了条件に localstack セットアップ手順ドキュメント化を追加
