# AgentCore Runtime デプロイガイド

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

### 認証あり（本番デプロイ）

```bash
docker run -p 8080:8080 \
  -e LOGVALET_MCP_AUTH=true \
  -e LOGVALET_MCP_EXTERNAL_URL=https://logvalet.example.com \
  -e LOGVALET_MCP_OIDC_ISSUER=https://accounts.google.com \
  -e LOGVALET_MCP_OIDC_CLIENT_ID=your-client-id \
  -e LOGVALET_MCP_OIDC_CLIENT_SECRET=your-client-secret \
  -e LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32) \
  -e LOGVALET_MCP_ALLOWED_DOMAINS=example.com \
  -e LOGVALET_API_KEY=your-backlog-api-key \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet
```

## 環境変数リファレンス

| 変数 | 必須 | 説明 | 例 |
|------|------|------|-----|
| `LOGVALET_API_KEY` | Yes* | Backlog API キー | `abcdef123456` |
| `LOGVALET_ACCESS_TOKEN` | Yes* | Backlog OAuth アクセストークン | `Bearer xyz...` |
| `LOGVALET_BASE_URL` | Yes | Backlog スペース URL | `https://your-space.backlog.com` |
| `LOGVALET_MCP_AUTH` | No | `true` で認証を有効化 | `true` |
| `LOGVALET_MCP_EXTERNAL_URL` | auth時必須 | OAuth コールバック URL | `https://logvalet.example.com` |
| `LOGVALET_MCP_OIDC_ISSUER` | auth時必須 | OIDC Issuer URL | `https://accounts.google.com` |
| `LOGVALET_MCP_OIDC_CLIENT_ID` | auth時必須 | OIDC Client ID | `123456.apps.googleusercontent.com` |
| `LOGVALET_MCP_OIDC_CLIENT_SECRET` | No | OIDC Client Secret | `GOCSPX-xxx` |
| `LOGVALET_MCP_COOKIE_SECRET` | auth時必須 | Hex エンコード 32+ バイト | 64文字の hex 文字列 |
| `LOGVALET_MCP_ALLOWED_DOMAINS` | No | メールドメイン制限（カンマ区切り） | `example.com,corp.example.com` |
| `LOGVALET_MCP_ALLOWED_EMAILS` | No | メールアドレス制限（カンマ区切り） | `admin@example.com` |

\* `LOGVALET_API_KEY` または `LOGVALET_ACCESS_TOKEN` のいずれか一方が必要

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

# 認証ありの場合（Bearer トークンが必要）
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ..." \
  -d '...'
```

## OAuth 2.1 エンドポイント（認証有効時のみ）

| パス | 説明 |
|------|------|
| `/.well-known/oauth-authorization-server` | OAuth メタデータ |
| `/.well-known/jwks.json` | 公開鍵 (JWKS) |
| `/register` | Dynamic Client Registration (RFC 7591) |
| `/authorize` | 認可エンドポイント (PKCE) |
| `/token` | トークンエンドポイント |
| `/login` | ブラウザログイン |
| `/callback` | OIDC コールバック |

## AgentCore Runtime 固有の注意事項

- AgentCore Runtime は AWS が管理するコンテナ実行環境のため、コンテナの再起動が発生する可能性があります
- 再起動時に ECDSA 署名鍵が再生成されるため、既存のアクセストークンは失効します
- 長時間のセッション維持が必要な場合は、署名鍵の外部永続化（AWS Secrets Manager 等）を検討してください
- ヘルスチェックは `/healthz` (HTTP 200) を使用してください
- シークレットの注入方法は AWS Secrets Manager または環境変数を推奨します

## OIDC プロバイダーの設定例

### Google

```
LOGVALET_MCP_OIDC_ISSUER=https://accounts.google.com
LOGVALET_MCP_OIDC_CLIENT_ID=xxxx.apps.googleusercontent.com
LOGVALET_MCP_OIDC_CLIENT_SECRET=GOCSPX-xxxx
```

### Microsoft Entra ID (Azure AD)

```
LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/{tenant-id}/v2.0
LOGVALET_MCP_OIDC_CLIENT_ID=xxxx-xxxx-xxxx
LOGVALET_MCP_OIDC_CLIENT_SECRET=xxxx
```
