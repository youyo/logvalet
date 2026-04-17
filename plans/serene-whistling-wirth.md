# logvalet MCP Lambda マルチインスタンス対応 (signing-key 外部化 + idproxy Store 切替)

## Context

`logvalet mcp` を AWS Lambda (Function URL) にデプロイすると、同時並行リクエストで複数コンテナが起動し以下が発生する:

1. **JWT 署名鍵がコンテナ毎に異なる** — `internal/cli/mcp_auth.go:25` で起動時に `ecdsa.GenerateKey()` をランダム生成しているため、コンテナ A が発行した access_token をコンテナ B が検証すると 401 (Claude.ai: `Authorization with the MCP server failed`)。
2. **OAuth 状態 (DCR client / 認可コード / セッション) がコンテナ間で共有されない** — `mcp_auth.go:58` で `store.NewMemoryStore()` 固定。`/register` 済みクライアントが別コンテナで消え `redirect_uri is not allowed` / `invalid_client` が発生。

現状は `reserved-concurrent-executions=1` で 1 コンテナ固定運用しているが、コールドスタートで状態が飛ぶため根本対応が必要。

本 Plan は **logvalet 側の改修**のみを扱う (idproxy 側 DynamoDBStore 実装は `github.com/youyo/idproxy` **v0.2.2 で既にリリース済み**)。

### 要件

- JWT 署名鍵を **環境変数 + CLI フラグ両対応** で受け取る (PEM 形式の ECDSA P-256 秘密鍵)。
- idproxy Store を **環境変数 + CLI フラグ両対応** で memory / dynamodb 切替可能にする。
- 既存 CLI の Kong `env:` タグ規約を踏襲し、既存 `TokenStore*` 群 (`internal/cli/mcp.go:44-47`) と一貫性を保つ。

## 対応方針

### 1. `McpCmd` に 4 フラグを追加 (`internal/cli/mcp.go:23-48`)

Kong の `env:` タグで flag / env 両対応を自動実現する (既存 `LOGVALET_MCP_TOKEN_STORE*` と同パターン)。

```go
// JWT 署名鍵 (PEM 形式 ECDSA P-256 秘密鍵)
SigningKey string `name:"signing-key" help:"ECDSA P-256 signing key (PEM)" group:"auth" env:"LOGVALET_MCP_SIGNING_KEY"`

// idproxy Store 切替
IDProxyStore             string `name:"idproxy-store" help:"idproxy store type (memory|dynamodb)" group:"store" env:"LOGVALET_MCP_IDPROXY_STORE" default:"memory"`
IDProxyStoreDynamoDBTable  string `name:"idproxy-store-dynamodb-table" help:"DynamoDB table name for idproxy store" group:"store" env:"LOGVALET_MCP_IDPROXY_STORE_DYNAMODB_TABLE"`
IDProxyStoreDynamoDBRegion string `name:"idproxy-store-dynamodb-region" help:"AWS region for idproxy DynamoDB store" group:"store" env:"LOGVALET_MCP_IDPROXY_STORE_DYNAMODB_REGION"`
```

### 2. `McpCmd.Validate` に静的チェックを追加 (`internal/cli/mcp.go:56-92`)

- `IDProxyStore=dynamodb` のとき `IDProxyStoreDynamoDBTable` 必須。
- `IDProxyStore` が `memory`/`dynamodb`/空 以外はエラー (case-insensitive)。
- **Fail-fast**: `IDProxyStore=dynamodb` かつ `SigningKey=""` はエラー (ランダム鍵はマルチインスタンスで機能しないため)。
  - Plan 原本 `logvalet-mcp/plans/logvalet-multi-instance-support.md:278` は警告ログで可としていたが、silently 壊れるより fail-fast の方が運用事故を防げる。

### 3. `BuildAuthConfig` を改修 (`internal/cli/mcp_auth.go:16-63`)

**変更点 A — signing key の外部化 (`mcp_auth.go:25-28`):**

```go
var signingKey *ecdsa.PrivateKey
if c.SigningKey != "" {
    block, _ := pem.Decode([]byte(c.SigningKey))
    if block == nil {
        return idproxy.Config{}, fmt.Errorf("signing-key: invalid PEM")
    }
    key, err := x509.ParseECPrivateKey(block.Bytes)
    if err != nil {
        return idproxy.Config{}, fmt.Errorf("signing-key: %w", err)
    }
    signingKey = key
} else {
    var err error
    signingKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        return idproxy.Config{}, fmt.Errorf("failed to generate signing key: %w", err)
    }
}
```

追加 import: `crypto/x509`, `encoding/pem`。

**変更点 B — Store 切替 (`mcp_auth.go:58`):**

idproxy v0.2.2 の実 API (`store/dynamodb.go`):

```go
func NewDynamoDBStore(tableName, region string) (*DynamoDBStore, error)
func (s *DynamoDBStore) Close() error
```

切替コード:

```go
var idpStore idproxy.Store
switch strings.ToLower(c.IDProxyStore) {
case "", "memory":
    idpStore = store.NewMemoryStore()
case "dynamodb":
    s, err := store.NewDynamoDBStore(c.IDProxyStoreDynamoDBTable, c.IDProxyStoreDynamoDBRegion)
    if err != nil {
        return idproxy.Config{}, fmt.Errorf("failed to create idproxy dynamodb store: %w", err)
    }
    idpStore = s
default:
    return idproxy.Config{}, fmt.Errorf("invalid idproxy-store: %q", c.IDProxyStore)
}
```

`idproxy.Config.Store` にこの `idpStore` を設定する。

**注意**: `DynamoDBStore.Close()` を `McpCmd.Run` のシャットダウン時に呼ぶ必要がある。現状 `BuildAuthConfig` 内で Store 生成しているため、**Store 生成を `BuildAuthConfig` 外 (呼び出し元 `McpCmd.Run`) に移すリファクタ** が必要。または `BuildAuthConfig` が `(Config, io.Closer, error)` を返す形に変え、`Run` 側で `defer closer.Close()` を配置する。推奨は後者 (既存テストへの影響が小さい)。

### 4. go.mod を bump

- `github.com/youyo/idproxy` v0.1.6 → v0.2.2 (DynamoDBStore リリース済み)。
- `go get github.com/youyo/idproxy@v0.2.2 && go mod tidy`。
- リリース後に DynamoDBStore コンストラクタの実シグネチャ (関数名・引数) を確認し、本 Plan §3 変更点 B のコード例と差異があれば合わせる。

## 実装手順 (TDD)

1. **Red**: `internal/cli/mcp_validate_test.go` に以下ケースを追加:
   - `IDProxyStore=dynamodb` + table 空 → validate error
   - `IDProxyStore=dynamodb` + signing-key 空 → validate error
   - `IDProxyStore=invalid` → validate error
2. **Red**: `internal/cli/mcp_auth_test.go` に以下ケースを追加:
   - signing-key 空 → ランダム生成 (`idproxy.Config.OAuth.SigningKey != nil`)
   - signing-key 正常 PEM → 同一鍵が渡る (`PublicKey.Equal` で一致確認)
   - signing-key 不正 PEM → error
   - `IDProxyStore=memory` → `store.NewMemoryStore` 型アサート
   - `IDProxyStore=dynamodb` + 正常 → DynamoDBStore 型アサート (mock 注入点があれば利用)
3. **Green**: `internal/cli/mcp.go` にフラグ 4 つ追加・`Validate` 拡張。
4. **Green**: `internal/cli/mcp_auth.go` を書き換え。
5. **Refactor**: signing key のパースを `parseSigningKey(pem string) (*ecdsa.PrivateKey, error)` に切り出し、テスト容易性を確保。
6. `go test ./...` / `go vet ./...` 全緑で完了。

## 変更対象ファイル

- `internal/cli/mcp.go` (フラグ追加 + Validate 拡張)
- `internal/cli/mcp_auth.go` (signing-key 外部化 + Store 切替)
- `internal/cli/mcp_validate_test.go` (新規または追記)
- `internal/cli/mcp_auth_test.go` (新規または追記)
- `go.mod` / `go.sum` (idproxy v0.2.2 bump)

## 既存資産 (再利用対象)

- Kong `env:` タグ規約: `internal/cli/mcp.go:28-47` と完全に同一パターンで追加。
- Store 切替パターン: `internal/auth/tokenstore/factory.go` (memory/sqlite/dynamodb の切替)。
- DynamoDB mock 注入の先行事例: `internal/auth/tokenstore/dynamodb.go`。
- Validate パターン: `internal/cli/mcp.go:56-92` に既存バリデーションと並べて追加。

## 検証手順

### ローカル (単体)

```bash
go test ./internal/cli/...
go vet ./...
go build -o logvalet ./cmd/logvalet/
```

### 手動 (flag / env 両系統)

```bash
# PEM 生成
openssl ecparam -name prime256v1 -genkey -noout -out /tmp/sk.pem

# flag 経由
./logvalet mcp --auth --signing-key "$(cat /tmp/sk.pem)" \
  --idproxy-store memory --oidc-issuer ... --oidc-client-id ... \
  --cookie-secret $(openssl rand -hex 32) --external-url http://localhost:8080

# env 経由
LOGVALET_MCP_SIGNING_KEY="$(cat /tmp/sk.pem)" \
LOGVALET_MCP_IDPROXY_STORE=memory \
./logvalet mcp --auth ...

# 異常系: dynamodb + signing-key 未指定 → fail-fast
LOGVALET_MCP_IDPROXY_STORE=dynamodb ./logvalet mcp --auth ...  # exit != 0
```

### Lambda 本番 (idproxy v0.2.2 利用)

logvalet-mcp 側 Plan (`logvalet-mcp/plans/logvalet-multi-instance-support.md` §C) に委譲。idproxy v0.2.2 README に準拠する手順:

1. SSM Parameter Store に PEM 登録 (SecureString)。
2. DynamoDB テーブル作成:
   ```bash
   aws dynamodb create-table \
     --table-name logvalet-idproxy-store \
     --attribute-definitions AttributeName=pk,AttributeType=S \
     --key-schema AttributeName=pk,KeyType=HASH \
     --billing-mode PAY_PER_REQUEST \
     --region ap-northeast-1
   aws dynamodb update-time-to-live \
     --table-name logvalet-idproxy-store \
     --time-to-live-specification "Enabled=true,AttributeName=ttl" \
     --region ap-northeast-1
   ```
3. Lambda 実行ロールに IAM 権限付与 (**idproxy v0.2.2 は `GetItem` / `PutItem` / `DeleteItem` のみ使用** — `UpdateItem` / `Query` は不要):
   ```json
   {
     "Effect": "Allow",
     "Action": ["dynamodb:GetItem","dynamodb:PutItem","dynamodb:DeleteItem"],
     "Resource": "arn:aws:dynamodb:ap-northeast-1:*:table/logvalet-idproxy-store"
   }
   ```
4. `function.json` に `LOGVALET_MCP_SIGNING_KEY` / `LOGVALET_MCP_IDPROXY_STORE=dynamodb` / `LOGVALET_MCP_IDPROXY_STORE_DYNAMODB_TABLE` / `LOGVALET_MCP_IDPROXY_STORE_DYNAMODB_REGION` を `{{ ssm ... }}` で追加。
5. `reserved-concurrent-executions` を解除し、Claude.ai から並行ツール呼び出しで 401 / `redirect_uri is not allowed` が発生しないこと、コールドスタート跨ぎでトークン再発行不要なこと、`logvalet-idproxy-store` に DCR クライアント・認可コード・セッションが永続化されることを確認。

## Out of Scope

- idproxy 側の DynamoDBStore 実装 (`github.com/youyo/idproxy` リポジトリでの作業)。
- signing key ローテーション機構 (必要なら別 Plan)。
- Redis / Postgres Store 対応。
- logvalet-mcp 側 SSM/IAM/function.json 更新 (別リポジトリ)。
