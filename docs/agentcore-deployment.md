# AgentCore Runtime デプロイガイド

MCP の認証は AgentCore Gateway に委譲します。logvalet 側は `none` または
`apikey` を受け付け、Gateway は `X-Logvalet-Api-Key` と、必要に応じて
`X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` を送信します。
詳細は [gateway-request-contract.md](specs/gateway-request-contract.md) を参照してください。

logvalet MCP サーバーを Docker コンテナとしてデプロイし、AWS Bedrock AgentCore Runtime から利用する手順。

## ビルド

```bash
docker build -t logvalet .
```

## 実行

### 認証なし（ローカル開発）

```bash
docker run -p 8080:8080 \
  -e LOGVALET_API_KEY=your-backlog-api-key \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet
```

### Gateway 経由（本番デプロイ）

```bash
docker run -p 8080:8080 \
  -e LOGVALET_MCP_AUTH_MODE=apikey \
  -e LOGVALET_MCP_API_KEY=shared-gateway-key \
  -e LOGVALET_SPACE_STORE_TYPE=sqlite \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet
```

Lambda/AgentCore の multi-tenant 運用では、再起動後もスペース登録を保持できる
DynamoDB store を使用します。

```bash
docker run -p 8080:8080 \
  -e LOGVALET_MCP_AUTH_MODE=apikey \
  -e LOGVALET_MCP_API_KEY=shared-gateway-key \
  -e LOGVALET_SPACE_STORE_TYPE=dynamodb \
  -e LOGVALET_SPACE_STORE_DYNAMODB_TABLE=logvalet-spaces \
  -e LOGVALET_SPACE_STORE_DYNAMODB_REGION=ap-northeast-1 \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet
```

## 環境変数リファレンス

| 変数 | 必須 | 説明 | 例 |
|------|------|------|-----|
| `LOGVALET_API_KEY` | CLI/stdioのみ | ローカル CLI/stdio の Backlog API キー | `abcdef123456` |
| `LOGVALET_ACCESS_TOKEN` | CLI/stdioのみ | ローカル CLI/stdio の Backlog Bearer credential | `Bearer xyz...` |
| `LOGVALET_BASE_URL` | Yes | Backlog スペース URL | `https://your-space.backlog.com` |
| `LOGVALET_MCP_AUTH_MODE` | No | `none` または `apikey` | `none` |
| `LOGVALET_MCP_API_KEY` | apikey時必須 | Gateway と共有するキー | `shared-gateway-key` |
| `LOGVALET_SPACE_STORE_TYPE` | No | space store の種類（`sqlite` または `dynamodb`） | `sqlite` |
| `LOGVALET_SPACE_STORE_PATH` | SQLite時 | SQLite DB パス | `~/.logvalet/spaces.db` |
| `LOGVALET_SPACE_STORE_DYNAMODB_TABLE` | DynamoDB時 | space store の DynamoDB テーブル名 | `logvalet-spaces` |
| `LOGVALET_SPACE_STORE_DYNAMODB_REGION` | DynamoDB時 | space store の DynamoDB リージョン | `ap-northeast-1` |
| `Authorization` | HTTP Gateway経由 | Gateway が渡す Backlog Bearer passthrough | `Bearer <credential>` |

HTTP remote では `LOGVALET_API_KEY` / `LOGVALET_ACCESS_TOKEN` を設定せず、
Gateway の `Authorization: Bearer` passthrough を使用します。

## ヘルスチェック

認証の有無にかかわらず `/healthz` エンドポイントは認証なしでアクセス可能:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## MCP エンドポイント

MCP プロトコルのエンドポイントは `/mcp`:

```bash
# 認証なしの場合
curl -X POST http://localhost:8080/mcp -H "Content-Type: application/json" -d '...'

# Gateway 認証あり（API key、Backlog credential は Bearer passthrough）
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "X-Logvalet-Api-Key: shared-gateway-key" \
  -H "Authorization: Bearer <backlog-credential-passthrough>" \
  -d '...'
```

## AgentCore Runtime 固有の注意事項

- AgentCore Runtime は AWS が管理するコンテナ実行環境のため、コンテナの再起動が発生する可能性があります
- Backlog credential の取得・更新・ライフサイクルは AgentCore Gateway が管理します
- logvalet は Gateway から受け取った Bearer credential を Backlog API へ passthrough します
- ヘルスチェックは `/healthz` (HTTP 200) を使用してください
- シークレットの注入方法は AWS Secrets Manager または環境変数を推奨します
