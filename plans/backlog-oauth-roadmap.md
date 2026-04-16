# Roadmap: Backlog OAuth 対応

## Context

logvalet の remote MCP 構成において、ユーザーごとに異なる Backlog 権限で API を実行できるようにする。
認証(AuthN) は既存の idproxy + OIDC が担当し、Backlog へのアクセス権限(AuthZ) は Backlog OAuth 2.0 を別レイヤーで扱う。

**スペック**: `docs/specs/logvalet_backlog_oauth_coding_agent_prompt.md`

## Meta

| 項目 | 値 |
|------|---|
| ゴール | remote MCP でユーザーごとの Backlog OAuth トークンを管理し、各ユーザーの権限で API 実行 |
| 成功基準 | 2ユーザーが同時に異なる Backlog トークンで MCP ツールを実行できる |
| 制約 | Zero config file（全て環境変数）、Lambda/VPC不要、idproxy の責務を崩さない |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/logvalet |
| 作成日 | 2026-04-16 |
| 最終更新 | 2026-04-16 22:50 |
| ステータス | 未着手 |

## 設計決定

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | Zero config file — 全設定を環境変数のみで行う | Lambda デプロイとの相性、設定ファイル管理の簡素化 | 2026-04-16 |
| 2 | OAuth state は signed state payload (JWT HMAC-SHA256) | 外部ストア不要で Lambda との相性が良い。既存 golang-jwt/jwt/v5 を活用 | 2026-04-16 |
| 3 | OAuth HTTP ハンドラーは MCP サーバーに組み込み | Lambda 1デプロイで完結。既存の mux パターンに追加 | 2026-04-16 |
| 4 | TokenStore は memory → sqlite → dynamodb の順に実装 | memory で全フローを先に完成させ、永続化は後から追加 | 2026-04-16 |
| 5 | 既存 credentials.BuildAuthorizeURL / ExchangeCode を再利用 | 動作実績あり。BacklogOAuthProvider 内でラップ | 2026-04-16 |
| 6 | 既存パスは一切変更しない（追加のみ） | CLI profile/API key、MCP 単一クライアントは壊さない | 2026-04-16 |

## 後方互換性の保証

### 変更しないもの（既存パス）

| コンポーネント | 影響 | 理由 |
|---------------|------|------|
| `config.toml` / profile 機構 | **変更なし** | `internal/config/` は一切触れない |
| `tokens.json` / API key 認証 | **変更なし** | `internal/credentials/Store`, `Resolver` は一切触れない |
| `buildRunContext()` | **変更なし** | CLI・既存 MCP の認証解決フローはそのまま |
| `NewToolRegistry(server, client)` | **変更なし** | M11 で `NewToolRegistryWithFactory` を**追加**するのみ |
| `NewServer(client, ver, cfg)` | **変更なし** | M12 で `NewServerWithFactory` を**追加**するのみ |
| CLI コマンド全般 | **変更なし** | `lv issue get`, `lv project list` 等の動作は不変 |

### 新パスの発動条件（M16）

OAuth の per-user パスが有効になるのは以下が**全て揃った場合のみ**:
1. `--auth` フラグが有効
2. `LOGVALET_BACKLOG_CLIENT_ID` 環境変数が設定されている

それ以外は全て既存パスが使われる:
```go
// mcp.go での分岐イメージ
if c.Auth && oauthCfg != nil {
    // 新パス: per-user ClientFactory → NewServerWithFactory
    s := mcpinternal.NewServerWithFactory(factory, ver, cfg)
} else {
    // 既存パス: 単一 Client → NewServer（変更なし）
    s := mcpinternal.NewServer(rc.Client, ver, cfg)
}
```

## 既存コード再利用判定

| 既存コード | 判定 | 理由 |
|-----------|------|------|
| `credentials.BuildAuthorizeURL` | **再利用** | BacklogOAuthProvider 内でラップ |
| `credentials.ExchangeCode` | **再利用** | Provider 内でラップ。httptest 対応済み |
| `credentials.TokenResponse` | **再利用** | TokenRecord へのマッピングに利用 |
| `credentials.StartCallbackServer` | **不要** | CLI localhost 用。remote MCP では HTTP handler がコールバック |
| `credentials.GenerateState` | **置換** | HMAC 署名付き JWT state に変更 |
| `credentials.Store` (fileStore) | **不要** | tokens.json はファイルベース。TokenStore interface に置換 |
| `golang-jwt/jwt/v5` (go.mod) | **活用** | signed state payload の JWT 実装に使用 |

## 追加依存

| マイルストーン | パッケージ | 用途 |
|-------------|-----------|------|
| M08 | `modernc.org/sqlite` | SQLite TokenStore（pure Go、CGO 不要） |
| M09 | `github.com/aws/aws-sdk-go-v2` | DynamoDB TokenStore |

## Current Focus

- **マイルストーン**: M01 (TokenRecord 型定義とセキュリティ基盤)
- **直近の完了**: ロードマップ作成
- **次のアクション**: M01 の TDD 開始

---

## Progress

### Phase 1: 基盤型定義（並列実行可能: M01, M03, M04）

#### M01: TokenRecord 型定義とセキュリティ基盤
- [ ] TokenRecord struct + String() マスキング
- [ ] IsExpired() / NeedsRefresh(margin) メソッド
- [ ] センチネルエラー 6種定義
- 詳細: plans/backlog-oauth-m01-types.md

#### M03: OAuthConfig 環境変数ローダー
- [ ] OAuthEnvConfig struct + LoadOAuthEnvConfig()
- [ ] Validate() — 必須項目チェック
- [ ] TokenStore type / SQLite path / DynamoDB 設定の環境変数
- 詳細: plans/backlog-oauth-m03-env-config.md

#### M04: Signed State (JWT ベース)
- [ ] StateClaims struct + GenerateState()
- [ ] ValidateState() — HMAC-SHA256 検証
- [ ] TTL + 改竄検知テスト
- 詳細: plans/backlog-oauth-m04-signed-state.md

### Phase 2: コアストア & プロバイダー（M01完了後）

#### M02: MemoryTokenStore 実装
- [ ] TokenStore interface 定義 (Get/Put/Delete/Close)
- [ ] MemoryStore 実装 (sync.RWMutex + map)
- [ ] ユーザー隔離テスト
- 詳細: plans/backlog-oauth-m02-memory-store.md

#### M05: OAuthProvider interface と Backlog 実装
- [ ] OAuthProvider interface 定義
- [ ] BacklogOAuthProvider (BuildAuthorizationURL, ExchangeCode, RefreshToken, GetCurrentUser)
- [ ] credentials パッケージの既存関数をラップ
- 詳細: plans/backlog-oauth-m05-backlog-provider.md

### Phase 3: マネージャー & ファクトリー（M02+M05完了後）

#### M06: TokenManager 実装
- [ ] TokenManager interface (GetValidToken/SaveToken/RevokeToken)
- [ ] 有効期限判定 + 自動リフレッシュ (margin: 5min)
- [ ] Observability: リフレッシュ成功/失敗ログ
- 詳細: plans/backlog-oauth-m06-token-manager.md

#### M07: TokenStore ファクトリー
- [ ] NewTokenStore(storeType, cfg) — 環境変数ベースで切り替え
- [ ] memory のみ実装、sqlite/dynamodb は ErrNotImplemented スタブ
- 詳細: plans/backlog-oauth-m07-store-factory.md

### Phase 4: 追加 TokenStore（M07完了後、並列実行可能）

#### M08: SQLite TokenStore
- [ ] SQLiteStore 実装 (modernc.org/sqlite, pure Go)
- [ ] UPSERT + UTC 保存 + auto migration
- [ ] ファクトリー統合
- 詳細: plans/backlog-oauth-m08-sqlite-store.md

#### M09: DynamoDB TokenStore
- [ ] DynamoDBStore 実装 (aws-sdk-go-v2)
- [ ] PK = userID#provider#tenant
- [ ] ファクトリー統合
- 詳細: plans/backlog-oauth-m09-dynamodb-store.md

### Phase 5: Per-User クライアント（M06完了後）

#### M10: Per-User ClientFactory
- [ ] UserIDFromContext / ContextWithUserID ヘルパー
- [ ] ClientFactory — TokenManager からトークン取得 → backlog.HTTPClient 生成
- [ ] ユーザー隔離テスト
- 詳細: plans/backlog-oauth-m10-client-factory.md

#### M11: ToolRegistry Per-User 対応
- [ ] NewToolRegistryWithFactory(server, factory)
- [ ] Register 内で factory(ctx) 呼び出し
- [ ] 既存 NewToolRegistry 後方互換維持
- 詳細: plans/backlog-oauth-m11-registry-factory.md

#### M12: NewServer Per-User 対応
- [ ] NewServerWithFactory(factory, ver, cfg)
- [ ] 既存 NewServer 後方互換維持
- 詳細: plans/backlog-oauth-m12-server-factory.md

### Phase 6: OAuth HTTP ハンドラー（M04+M05+M10完了後）

#### M13: OAuth HTTP ハンドラー（認可開始）
- [ ] /oauth/backlog/authorize — state 生成 → Backlog OAuth URL へ 302 リダイレクト
- [ ] idproxy コンテキストから userID 取得
- [ ] Observability: 認可開始ログ
- 詳細: plans/backlog-oauth-m13-authorize-handler.md

#### M14: OAuth HTTP ハンドラー（コールバック）
- [ ] /oauth/backlog/callback — state 検証 → code exchange → GetCurrentUser → SaveToken
- [ ] エラーハンドリング (state 不正、code 空、exchange 失敗)
- [ ] Observability: コールバック成功/失敗ログ
- 詳細: plans/backlog-oauth-m14-callback-handler.md

#### M15: OAuth ステータス & 切断ハンドラー
- [ ] /oauth/backlog/status — 接続状態確認
- [ ] /oauth/backlog/disconnect — トークン削除
- 詳細: plans/backlog-oauth-m15-status-handler.md

### Phase 7: MCP 統合（M12+M14+M15完了後）

#### M16: MCP サーバーへの OAuth ルート統合
- [ ] mcp.go の mux に OAuth ハンドラー追加
- [ ] --auth 有効 + LOGVALET_BACKLOG_CLIENT_ID 設定時のみ有効化
- [ ] NewServerWithFactory で per-user MCP サーバー構築
- [ ] idproxy 認証ミドルウェアを OAuth ルートにも適用
- 詳細: plans/backlog-oauth-m16-mcp-integration.md

### Phase 8: E2E テスト & デプロイ設定（M16完了後）

#### M17: E2E 統合テストとデプロイ設定更新
- [ ] OAuth フロー全体の E2E テスト (//go:build integration)
- [ ] 2ユーザー同時接続テスト（ユーザー隔離証明）
- [ ] examples/lambroll 環境変数追加
- [ ] README 更新（AuthN vs AuthZ, Token Store, 初回接続フロー）
- 詳細: plans/backlog-oauth-m17-e2e-docs.md

---

## 依存関係グラフ

```
M01 ──→ M02 ──→ M07 ──→ M08 (SQLite)
 │        │        └────→ M09 (DynamoDB)
 │        └──→ M06 ──→ M10 ──→ M11 ──→ M12 ──→ M16 ──→ M17
 │              ↑        ↑
 ├──→ M05 ──────┘        │
 │    ↑                  │
M03 ──┘            M13 ──┤
                   ↑     │
M04 ───────────────┘     │
                   M14 ──┤
                   M15 ──┘
```

## 並列実行可能グループ

| Phase | 並列可能 | 条件 |
|-------|---------|------|
| A | M01, M03, M04 | 依存なし |
| B | M02, M05 | M01 完了後 |
| C | M06, M07 | M02+M05 完了後 |
| D | M08, M09, M10 | M07+M06 完了後 |
| E | M11, M13 | M10 完了後 |
| F | M12, M14, M15 | M11+M13 完了後 |
| G | M16 | M12+M14+M15 完了後 |
| H | M17 | M16 完了後 |

## セキュリティ要件マッピング

| セキュリティ要件 | 対応 M |
|----------------|--------|
| トークンをログ出力しない | **M01** (String() マスク)、全 M で遵守 |
| state 検証必須 | **M04** (JWT 署名)、**M14** (コールバックで検証) |
| 他ユーザー token 参照不可 | **M02** (隔離テスト)、**M10** (context userID) |
| 共有 token フォールバックしない | **M02** (テスト証明)、**M10** (factory 個別取得) |
| OAuth client_secret 安全管理 | **M03** (環境変数のみ) |

## Observability 要件マッピング

| Observability 要件 | 対応 M |
|-------------------|--------|
| トークンリフレッシュ成功/失敗 | **M06** (TokenManager) |
| OAuth 認可開始 | **M13** (HandleAuthorize) |
| OAuth コールバック成功/失敗 | **M14** (HandleCallback) |
| 接続状態変更 | **M15** (HandleDisconnect) |

## ディレクトリ構成

```
internal/
  auth/
    types.go              # TokenRecord, ProviderUser
    types_test.go
    errors.go             # センチネルエラー
    errors_test.go
    config.go             # OAuthEnvConfig (環境変数ローダー)
    config_test.go
    state.go              # Signed state (JWT)
    state_test.go
    context.go            # UserIDFromContext / ContextWithUserID
    context_test.go
    factory.go            # ClientFactory
    factory_test.go
    manager.go            # TokenManager
    manager_test.go
    tokenstore/
      store.go            # TokenStore interface
      memory.go
      memory_test.go
      sqlite.go
      sqlite_test.go
      dynamodb.go
      dynamodb_test.go
      factory.go          # NewTokenStore ファクトリー
      factory_test.go
    provider/
      provider.go         # OAuthProvider interface
      backlog.go          # BacklogOAuthProvider
      backlog_test.go
  transport/
    http/
      oauth_handler.go    # OAuth HTTP ハンドラー群
      oauth_handler_test.go
  mcp/
    tools.go              # ToolRegistry 拡張 (factory 対応)
    server.go             # NewServerWithFactory 追加
  cli/
    mcp.go                # OAuth ルート統合
```

## 環境変数一覧

| 環境変数 | 必須 | デフォルト | 説明 |
|---------|------|-----------|------|
| `LOGVALET_TOKEN_STORE` | No | `memory` | TokenStore 種別 (memory/sqlite/dynamodb) |
| `LOGVALET_TOKEN_STORE_SQLITE_PATH` | sqlite時 | `./logvalet.db` | SQLite DB パス |
| `LOGVALET_TOKEN_STORE_DYNAMODB_TABLE` | dynamodb時 | — | DynamoDB テーブル名 |
| `LOGVALET_TOKEN_STORE_DYNAMODB_REGION` | dynamodb時 | — | AWS リージョン |
| `LOGVALET_BACKLOG_CLIENT_ID` | Yes* | — | Backlog OAuth クライアントID |
| `LOGVALET_BACKLOG_CLIENT_SECRET` | Yes* | — | Backlog OAuth クライアントシークレット |
| `LOGVALET_BACKLOG_REDIRECT_URL` | Yes* | — | OAuth コールバック URL |
| `LOGVALET_OAUTH_STATE_SECRET` | Yes* | — | JWT state 署名鍵 (hex) |

*OAuth 機能を有効にする場合のみ必須

## Blockers

なし

## Changelog

| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-16 22:50 | 作成 | ロードマップ初版作成（17マイルストーン、TDD、zero config file） |

---

## マイルストーン詳細サマリー

以下は各マイルストーンの TDD テストケースと実装対象の要約。
承認後に `plans/backlog-oauth-m{NN}-{slug}.md` として個別ファイルを生成する。

### M01: TokenRecord 型定義とセキュリティ基盤

**TDD Red**:
- `TokenRecord` 全フィールド初期化
- `String()` がアクセストークン・リフレッシュトークンをマスク (`"ac...xy"` 形式、4文字以下は `"***"`)
- `IsExpired()` が expiry 前後で正しく判定
- `NeedsRefresh(5min)` が margin 内で true
- 6種の sentinel error が `errors.Is()` で判定可能

**対象ファイル**: `internal/auth/types.go`, `internal/auth/errors.go` + テスト

---

### M02: MemoryTokenStore 実装

**TDD Red**:
- Put → Get で同一レコード取得
- Get で存在しないキーが `nil, nil`（エラーではない）
- Put 上書きで最新が返る
- Delete 後の Get が `nil, nil`
- Delete で存在しないキーはエラーにならない
- Close() が冪等
- **ユーザー隔離**: Put(userA) → Get(userB) が `nil, nil`

**対象ファイル**: `internal/auth/tokenstore/store.go`, `internal/auth/tokenstore/memory.go` + テスト

---

### M03: OAuthConfig 環境変数ローダー

**TDD Red**:
- 各環境変数が取得できること
- `LOGVALET_TOKEN_STORE` が memory/sqlite/dynamodb を受け付ける
- 未設定時デフォルト `memory`
- 不正値でエラー
- `LOGVALET_BACKLOG_CLIENT_ID` 未設定で Validate エラー

**対象ファイル**: `internal/auth/config.go` + テスト

---

### M04: Signed State (JWT ベース)

**TDD Red**:
- GenerateState → ValidateState で claims 復元
- 異なる secret で検証失敗
- 期限切れ state が ErrStateExpired
- 改竄 payload で検証失敗
- nonce が毎回異なる

**対象ファイル**: `internal/auth/state.go` + テスト

---

### M05: OAuthProvider interface と Backlog 実装

**TDD Red**:
- `Name()` が `"backlog"` を返す
- `BuildAuthorizationURL` が正しい Backlog OAuth URL を返す
- `ExchangeCode` が httptest サーバーに正しいパラメータを送信し TokenRecord を返す
- `RefreshToken` が grant_type=refresh_token で新しい TokenRecord を返す
- `GetCurrentUser` が `/api/v2/users/myself` を呼びユーザー情報を返す
- space 空でエラー

**対象ファイル**: `internal/auth/provider/provider.go`, `internal/auth/provider/backlog.go` + テスト
**再利用**: `credentials.BuildAuthorizeURL`, `credentials.ExchangeCode`

---

### M06: TokenManager 実装

**TDD Red**:
- GetValidToken で有効トークン取得
- expiry - 5min 以内で自動リフレッシュ実行
- リフレッシュ後 store 更新
- リフレッシュ失敗で ErrTokenRefreshFailed
- レコード未存在で ErrProviderNotConnected
- SaveToken で store 保存
- RevokeToken で store 削除

**対象ファイル**: `internal/auth/manager.go` + テスト

---

### M07: TokenStore ファクトリー

**TDD Red**:
- `NewTokenStore("memory", cfg)` が MemoryStore
- `NewTokenStore("sqlite", cfg)` が ErrNotImplemented（M08 で解除）
- `NewTokenStore("dynamodb", cfg)` が ErrNotImplemented（M09 で解除）
- `NewTokenStore("unknown", cfg)` がエラー
- 空文字列がデフォルト memory

**対象ファイル**: `internal/auth/tokenstore/factory.go` + テスト

---

### M08: SQLite TokenStore

**TDD Red**:
- CRUD 正常系 (t.TempDir())
- テーブル自動作成 (CREATE IF NOT EXISTS)
- Put 上書きで UpdatedAt 更新
- Close() 後の操作がエラー
- ユーザー隔離
- 並行書き込みでデータ競合なし

**対象ファイル**: `internal/auth/tokenstore/sqlite.go` + テスト、ファクトリー更新
**追加依存**: `modernc.org/sqlite`

---

### M09: DynamoDB TokenStore

**TDD Red**:
- CRUD 正常系 (DynamoDB Local or mock)
- 存在しないキーが nil, nil
- ユーザー隔離
- テーブル名が環境変数から取得

**対象ファイル**: `internal/auth/tokenstore/dynamodb.go` + テスト、ファクトリー更新
**追加依存**: `github.com/aws/aws-sdk-go-v2`
**テストタグ**: `//go:build integration`

---

### M10: Per-User ClientFactory

**TDD Red**:
- ClientFactory(ctx) が userID 取得 → TokenManager → HTTPClient 返却
- userID 未設定で ErrUnauthenticated
- TokenManager が ErrProviderNotConnected を返した場合に伝搬
- 生成クライアントが正しい Bearer ヘッダ設定 (httptest)

**対象ファイル**: `internal/auth/context.go`, `internal/auth/factory.go` + テスト

---

### M11: ToolRegistry Per-User 対応

**TDD Red**:
- NewToolRegistryWithFactory でツール登録可能
- ツール呼び出し時に factory(ctx) が呼ばれる
- factory エラーで ToolResultError
- 既存 NewToolRegistry が後方互換で動作

**対象ファイル**: `internal/mcp/tools.go` (修正) + テスト

---

### M12: NewServer Per-User 対応

**TDD Red**:
- NewServerWithFactory が非nil サーバーを返す
- 既存 NewServer が後方互換で動作
- Factory モードで登録ツールがリクエスト時に factory を呼ぶ

**対象ファイル**: `internal/mcp/server.go` (修正) + テスト

---

### M13: OAuth HTTP ハンドラー（認可開始）

**TDD Red**:
- GET /oauth/backlog/authorize が 302 リダイレクト
- リダイレクト先が Backlog OAuth URL
- state パラメータが有効な JWT
- userID 未設定で 401

**対象ファイル**: `internal/transport/http/oauth_handler.go` + テスト

---

### M14: OAuth HTTP ハンドラー（コールバック）

**TDD Red**:
- callback?code=xxx&state=yyy でトークン保存 + レスポンス
- state 検証失敗で 400
- code 空で 400
- error パラメータで適切なエラー
- ExchangeCode 失敗で 502
- GetCurrentUser で ProviderUserID 記録

**対象ファイル**: `internal/transport/http/oauth_handler.go` (追加) + テスト

---

### M15: OAuth ステータス & 切断ハンドラー

**TDD Red**:
- 接続済み → `{"connected": true, "provider_user_id": "xxx"}`
- 未接続 → `{"connected": false}`
- 期限切れ → `{"connected": true, "needs_reauth": true}`
- /disconnect でトークン削除

**対象ファイル**: `internal/transport/http/oauth_handler.go` (追加) + テスト

---

### M16: MCP サーバーへの OAuth ルート統合

**TDD Red**:
- /mcp が MCP プロトコルで動作
- /oauth/backlog/* が各ハンドラーにルーティング
- /healthz がヘルスチェック応答
- idproxy 認証ミドルウェアが OAuth ルートに適用

**対象ファイル**: `internal/cli/mcp.go` (修正) + テスト

---

### M17: E2E 統合テストとデプロイ設定更新

**TDD Red**:
- 未認証ユーザーで ErrUnauthenticated
- OAuth フロー全体が MemoryStore で動作
- トークン期限切れ → 自動リフレッシュ → 成功
- 2ユーザー同時接続（隔離証明）

**ドキュメント**:
- README: AuthN vs AuthZ、Token Store 種類、環境変数一覧、初回接続フロー
- examples/lambroll: 環境変数追加

**対象ファイル**: `internal/e2e/oauth_e2e_test.go`, `examples/lambroll/*`, `README.md`

---

## 検証方法

### 単体テスト
```bash
go test ./internal/auth/... ./internal/mcp/... ./internal/transport/...
```

### 統合テスト（DynamoDB Local 必要）
```bash
go test -tags=integration ./internal/auth/tokenstore/...
```

### E2E テスト
```bash
# 環境変数設定後
go test -tags=integration ./internal/e2e/...
```

### 手動検証
1. `LOGVALET_BACKLOG_CLIENT_ID` 等を設定して MCP サーバー起動
2. `/oauth/backlog/authorize` にアクセス → Backlog 認可画面へリダイレクト
3. 認可後 `/oauth/backlog/callback` で token 保存
4. MCP ツール呼び出しでユーザーの Backlog データ取得確認
