# logvalet / Backlog OAuth 対応 実装指示プロンプト

以下をそのまま coding agent への指示として利用してください。

---

# 目的

`logvalet` の remote MCP 構成において、

- **認証(AuthN)** は `youyo/idproxy` + **Entra ID / Google(OIDC)** に担当させる
- **Backlog へのアクセス権限(AuthZ)** は **Backlog OAuth 2.0** を別レイヤーで扱う
- その結果、**1つの remote MCP サーバー** であっても、**利用ユーザーごとに異なる Backlog 権限** で API を実行できるようにする

ことを目指す。

重要なのは、**idproxy に Backlog OAuth をログイン手段として統合することではない**。

今回やりたいのは、

- idproxy: ユーザーの本人確認とセッション確立
- logvalet: ユーザーごとの Backlog OAuth 接続管理と API 実行

という責務分離である。

---

# 背景 / 設計方針

## 前提

- `youyo/idproxy` は OIDC ベースの認証プロキシとして利用する
- `logvalet` は CLI が本体だが、remote MCP としても動かしたい
- remote MCP は Lambda での運用も想定する
- Lambda では VPC は避けたい
- そのため EFS 前提の構成は第一候補にしない
- Backlog 側は API Key ではなく **OAuth 2.0** を優先する
- 目的は「各ユーザーの Backlog 権限で API 実行できること」

## 強い設計制約

1. **認証(AuthN) と認可(AuthZ) を分離すること**

   - idproxy は OIDC ログイン専任
   - Backlog OAuth は外部 API コネクタ認可専任

2. **idproxy を generic OAuth login provider 化することは今回のスコープに含めない**

   - idproxy 本体の責務は増やしすぎない
   - Backlog はログイン元ではなく接続先サービスとして扱う

3. **TokenStore は抽象化すること**

   - `memory`
   - `sqlite`
   - `dynamodb` の3実装を持てるようにする

4. **Lambda 本命は DynamoDB、PoC は memory、ローカル/自己ホスト向けは sqlite** という位置づけにすること

5. **MCP server は1つでも、ユーザーごとに異なる Backlog トークンを利用できる設計** にすること

---

# サポート行列

logvalet は CLI / MCP × Backlog 認証方式（API key / OAuth）× クライアント認証（idproxy/OIDC）の組み合わせで 6 パターンが想定される。現状のサポート状況は以下の通り。

| # | クライアント | Backlog 認証 | クライアント認証（OIDC） | 状態 | 備考 |
|---|------------|-------------|------------------------|------|------|
| 1 | CLI | API key | — | ✅ サポート | `lv auth login` で tokens.json に保存 |
| 2 | CLI | OAuth | — | ❌ 未実装 | `credentials` 基盤はあるが `lv auth login` の OAuth 経路は未実装。手動 tokens.json 編集でのみ動作 |
| 3 | MCP | API key | なし | ✅ サポート | `lv mcp`（`--auth` 無し）。起動時の単一 API key を全リクエスト共有 |
| 4 | MCP | API key | OIDC（idproxy） | ✅ サポート | `lv mcp --auth`（`LOGVALET_BACKLOG_CLIENT_ID` 未設定）。OIDC 認証 + 起動時 API key 共有 |
| 5 | MCP | OAuth | なし | ❌ 非サポート | OAuth は per-user 設計。userID 識別経路が idproxy のみのため、OIDC 無しでは「誰のトークンを使うか」決定不能 |
| 6 | MCP | OAuth | OIDC（idproxy） | ✅ サポート | `lv mcp --auth` + `LOGVALET_BACKLOG_CLIENT_ID` 設定。per-user Backlog トークンで API 実行 |

## 非サポートの理由（#5）

- OAuth 時の TokenStore キーは `userID`
- userID の供給元は `idproxy.UserFromContext().Subject` のみ
- OIDC を外すと userID が取れず、ClientFactory がトークンを取得できない
- 実行時バリデーション: `LOGVALET_BACKLOG_CLIENT_ID` 設定時は `--auth` を必須とする fast-fail を実装（silent fallback 防止）

## 未実装の理由（#2）

- `lv auth login` の OAuth 経路（localhost callback 起動、ブラウザ認可、token 保存）が CLI コマンドとして組み込まれていない
- 基盤（`credentials.BuildAuthorizeURL` / `ExchangeCode` / `StartCallbackServer`）は存在するため、実装は別マイルストーンとして追加可能

ユーザー向けの実行例（環境変数設定例 + CLI 引数パターン）は README の "Supported Modes" セクションを参照。

---

# ユーザーフロー

## 初回

1. ユーザーが Claude / remote MCP を使う
2. idproxy により Entra ID または Google でログイン済みになる
3. Backlog 関連ツールを初めて使う
4. logvalet 側が「このユーザーは Backlog 未接続」と判定
5. logvalet が Backlog OAuth 接続フローを開始
6. ユーザーが Backlog 上で同意
7. logvalet が `access_token` / `refresh_token` を保存
8. 以後、そのユーザーの Backlog 権限で API 実行

## 2回目以降

- idproxy のセッションが有効なら再ログイン不要
- Backlog token が有効、または refresh 可能なら再接続不要
- 実行時に user context から対応する Backlog token を選んで利用

---

# 期待する実装成果物

以下を実装すること。

## 1. Backlog OAuth Connector 層

`logvalet` に Backlog OAuth 用の connector / provider を追加する。

責務:

- Authorization URL 生成
- OAuth callback 処理
- token exchange
- refresh token を使った token refresh
- `/users/myself` 等で Backlog ユーザー確認
- TokenStore との連携

このレイヤーは、将来的に GitHub / Linear / Google など他 provider を追加できる形で設計すること。

ただし今回は **Backlog provider のみ実装** でよい。

---

## 2. TokenStore 抽象化

以下の interface を導入すること。

```go
type TokenRecord struct {
    UserID         string
    Provider       string
    Tenant         string
    AccessToken    string
    RefreshToken   string
    TokenType      string
    Scope          string
    Expiry         time.Time
    ProviderUserID string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type TokenStore interface {
    Get(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error)
    Put(ctx context.Context, record *TokenRecord) error
    Delete(ctx context.Context, userID, provider, tenant string) error
    Close() error
}
```

### 設計意図

- `userID`: idproxy / アプリ側で確定した利用ユーザー識別子
- `provider`: 例 `backlog`
- `tenant`: Backlog スペース識別子（必須）
  - 例: `example.backlog.com`

### 実装対象

- `MemoryTokenStore`
- `SQLiteTokenStore`
- `DynamoDBTokenStore`

---

## 3. TokenManager 層

`TokenStore` は保存のみに責務を限定し、token 有効期限判定や refresh は `TokenManager` が担当すること。

例:

```go
type TokenManager interface {
    GetValidToken(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error)
    SaveToken(ctx context.Context, record *TokenRecord) error
    RevokeToken(ctx context.Context, userID, provider, tenant string) error
}
```

期待動作:

- TokenStore から token 取得
- expiry を確認
- 期限切れまたは期限切れ間近なら refresh 実施
- refresh 後の token を保存
- valid token を caller に返却

### 期限判定

- 厳密な期限ぴったりではなく、**安全マージン** を持つこと
- 例: `expiry - 5m` を過ぎていたら refresh

---

## 4. Backlog API 実行時のユーザー解決

MCP tool 実行時に、現在の利用ユーザーを確定し、そのユーザーに紐づく Backlog token を使って API を実行すること。

### 必須要件

- user context の取得方法を明確化する
- 認証済みセッションから userID を復元できること
- userID + provider + tenant で token を引けること
- 他ユーザーの token が使われないこと

### セキュリティ要件

- 絶対に共有 token にフォールバックしない
- 「未接続ユーザーなのに別ユーザー token で通る」事故を防ぐこと
- TokenStore の key 設計は衝突しないこと

---

## 5. 未接続時の UX

Backlog 未接続時には、明示的に接続フローへ誘導すること。

### 期待する挙動

- Backlog ツール実行時に token が見つからない
- `Backlog is not connected for this user` のような明確なエラーを返す
- 接続 URL または接続導線を返す
- 初回接続後は同じ操作を再試行すれば通る

### 方針

- eager connect ではなく **lazy connect** を採用
- つまり、Backlog を使うときだけ接続を要求する

---

# 各 TokenStore の詳細要件

## A. MemoryTokenStore

### 目的

- 開発
- テスト
- PoC
- Lambda 単一同時実行の簡易運用

### 実装要件

- `sync.RWMutex` で保護
- map key は `userID:provider:tenant`
- スレッドセーフであること
- process restart で消えるのは仕様として明記

### 注意

- 本番 durable store ではない
- Lambda では warm environment 依存になる

---

## B. SQLiteTokenStore

### 目的

- ローカルCLI
- 単一サーバー運用
- 自己ホスト

### 必須テーブル

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

### 実装要件

- upsert 対応
- UTC で保存
- DB open/close を明確化
- migration を最低限整備

### セキュリティ

- refresh token の保存については、将来的な暗号化余地を確保すること
- 今回は平文保存でもよいが、暗号化 adapter を差し込める設計にすること

### 注意

- Lambda の `/tmp` を durable store と見なさないこと
- SQLite は Lambda 本命ではなくローカル寄りの位置づけであることを README に明記

---

## C. DynamoDBTokenStore

### 目的

- Lambda 本命
- VPC 不要
- マルチインスタンス耐性

### テーブル設計

以下のどちらかで実装すること。

#### 推奨: PK/SK 設計

- `pk = USER#{userID}`
- `sk = TOKEN#{provider}#{tenant}`

属性:

- `access_token`
- `refresh_token`
- `token_type`
- `scope`
- `expiry`
- `provider_user_id`
- `created_at`
- `updated_at`

### 実装要件

- strongly consistent read の必要性を検討すること
- 基本は単純 CRUD でよい
- TTL に依存しないこと
  - refresh token を持つため、expiry で即削除しない
- 更新競合に備えて conditional write を検討すること

### 備考

- Lambda / remote MCP の本命 store として扱う
- README と config 例を整備すること

---

# Backlog Provider 実装要件

## 1. Provider interface を導入

```go
type OAuthProvider interface {
    Name() string
    BuildAuthorizationURL(state string, redirectURI string) (string, error)
    ExchangeCode(ctx context.Context, code string, redirectURI string) (*TokenRecord, error)
    RefreshToken(ctx context.Context, refreshToken string) (*TokenRecord, error)
    GetCurrentUser(ctx context.Context, accessToken string) (*ProviderUser, error)
}
```

Backlog 実装では以下を提供すること。

- authorize URL 生成
- code exchange
- refresh
- current user 取得

`GetCurrentUser` は `/users/myself` 等を利用してよい。

---

## 2. state 管理

OAuth callback の `state` は必ず検証すること。

### 要件

- CSRF 対策を行うこと
- state に user context / tenant / redirect intent を埋め込むか、server-side store で保持すること
- state 検証失敗時は即拒否

### 実装方針

どちらでもよいが、明示的に選ぶこと。

- signed state payload
- ephemeral state store

PoC では signed state payload でも可。

---

## 3. tenant 取り扱い

Backlog はスペース単位のため、tenant を明示的に扱うこと。

### 要件

- connect 時に tenant を指定できること
- token record に tenant を保存すること
- API 実行時も tenant 指定がぶれないこと

---

## 4. トークンの所有者検証

コード交換直後、および必要に応じて refresh 後にも current user を取得し、token 所有者を確認すること。

### 保存推奨項目

- provider user id
- display name
- tenant

### 目的

- 誰の Backlog token か可視化できるようにする
- 誤紐づけ防止

---

# remote MCP との接続整理

今回の実装では、以下を明確にすること。

- remote MCP に接続するための認証は idproxy/OIDC
- Backlog API を叩くための認可は Backlog OAuth
- これは別物であり、トークンを混同しないこと

### 重要

- idproxy のトークンを Backlog API に流用しない
- Backlog のトークンを MCP 接続認証に使わない
- TokenStore に保存するのは **Backlog 側の token**

---

# Config 設計

以下のような設定構造を導入すること。

```yaml
auth:
  token_store:
    type: memory # memory | sqlite | dynamodb

    sqlite:
      path: ./logvalet.db

    dynamodb:
      table_name: logvalet-oauth-tokens
      region: ap-northeast-1

backlog:
  oauth:
    client_id: xxx
    client_secret: xxx
    redirect_url: https://example.com/oauth/backlog/callback
```

環境変数でも設定可能にすること。

例:

```bash
LOGVALET_TOKEN_STORE=memory
LOGVALET_TOKEN_STORE_SQLITE_PATH=./logvalet.db
LOGVALET_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
LOGVALET_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1
LOGVALET_BACKLOG_CLIENT_ID=xxx
LOGVALET_BACKLOG_CLIENT_SECRET=xxx
LOGVALET_BACKLOG_REDIRECT_URL=https://example.com/oauth/backlog/callback
```

---

# エラー設計

以下の区別を明確にすること。

- 未認証: idproxy セッションなし
- 未接続: Backlog token が存在しない
- 期限切れ: refresh 不可
- provider API エラー
- tenant 不一致
- user mismatch

### 例

- `ErrUnauthenticated`
- `ErrProviderNotConnected`
- `ErrTokenExpired`
- `ErrTokenRefreshFailed`
- `ErrProviderUserMismatch`
- `ErrInvalidTenant`

MCP tool 側では、ユーザーにとって actionable なメッセージに変換すること。

---

# observability 要件

最低限、以下を structured log で残すこと。

- OAuth connect started
- OAuth callback success/failure
- token refresh success/failure
- token store get/put/delete result
- provider current user fetched
- backlog API request start/end
- current app user id
- provider user id
- tenant

### 禁止

- access token / refresh token の生値をログに出さない
- client secret をログに出さない

---

# セキュリティ要件

必須。

1. token は絶対にログ出力しない
2. state を検証する
3. refresh token をそのまま外部エラーへ露出しない
4. 他ユーザー token の参照ができない key 設計にする
5. provider callback 時に user context を失わないようにする
6. config / env の secrets を README で明示
7. 将来暗号化保存へ移行しやすい abstraction を維持する

---

# テスト要件

最低限以下を実装すること。

## Unit Test

- MemoryTokenStore CRUD
- SQLiteTokenStore CRUD
- DynamoDBTokenStore CRUD
- TokenManager refresh path
- expiry margin 判定
- state 生成/検証
- userID/provider/tenant key 組み立て

## Integration Test

- Backlog OAuth callback の正常系
- token refresh の正常系
- token 不在時に未接続エラーを返すこと
- user mismatch 時に拒否されること

DynamoDB は localstack かテスト用 mock を使ってよい。

---

# 非機能要件

- CLI と remote MCP の両方で使える実装構造にすること
- Lambda を強く意識するが、Lambda 専用実装にしすぎないこと
- VPC 前提にしないこと
- EFS / S3 Files 前提にしないこと
- 単一責務を守ること

---

# 望ましいディレクトリ構成例

```text
internal/
  auth/
    tokenstore/
      memory.go
      sqlite.go
      dynamodb.go
      types.go
    tokenmanager/
      manager.go
  provider/
    backlog/
      provider.go
      oauth.go
      api.go
      types.go
  transport/
    http/
      oauth_backlog_handlers.go
  mcp/
    tools/
      backlog_*.go
```

構成は変えてよいが、責務分離は維持すること。

---

# README / ドキュメント更新要件

以下を README に追記すること。

1. 認証と認可の違い
2. idproxy は OIDC ログイン担当であること
3. Backlog OAuth は接続先サービス認可であること
4. token store の種類と用途
   - memory
   - sqlite
   - dynamodb
5. Lambda では DynamoDB 推奨であること
6. sqlite はローカル/自己ホスト向けであること
7. Backlog 初回接続フロー

---

# 今回やらないこと

以下は今回のスコープ外。

- idproxy 本体を generic OAuth login provider 化すること
- Backlog API key を主方式として実装すること
- EFS 前提の sqlite 永続化
- VPC 必須構成の採用
- 複数 provider 同時実装
- 高度な UI 実装

---

# 実装順序

以下の順で進めること。

1. `TokenStore` interface 定義
2. `MemoryTokenStore` 実装
3. Backlog provider interface + stub
4. OAuth callback / exchange 実装
5. `TokenManager` 実装
6. 未接続時のエラー/導線実装
7. `SQLiteTokenStore` 実装
8. `DynamoDBTokenStore` 実装
9. テスト
10. README 更新

---

# 最終的に欲しい状態

- remote MCP は 1 つでよい
- 認証は idproxy + Entra ID / Google
- Backlog 操作時のみ lazy に Backlog OAuth を要求
- 接続後はユーザーごとの Backlog 権限で API 実行
- token store は memory / sqlite / dynamodb を差し替え可能
- Lambda では dynamodb を本命にできる
- local / self-host では sqlite を使える
- idproxy の責務は崩さない

---

# 実装時の注意

不明点があっても設計原則を崩さず、以下を優先すること。

1. AuthN と AuthZ の分離
2. TokenStore の抽象化
3. ユーザーごとの token 分離
4. Lambda / local 双方への適応性
5. セキュリティ上安全な既定値

必要なら適切な TODO を残してよいが、構造は完成形に近い状態まで実装すること。

---

以上。

