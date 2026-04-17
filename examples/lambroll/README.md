# logvalet on Lambda Function URL

lambroll + Lambda Web Adapter (LWA) を使って logvalet MCP サーバーを Lambda Function URL にデプロイします。
OIDC 認証を前提とし、Backlog 側の認証は次の 2 モードから選択します。

| Mode | MCP 認証 | Backlog 認証 | TokenStore | 用途 |
|------|---------|-------------|-----------|------|
| **A: OIDC + API Key** | OIDC（idproxy） | 共有 API キー | 不要 | 組織内で 1 アカウントを共用する場合 |
| **B: OIDC + OAuth**   | OIDC（idproxy） | 各ユーザーの OAuth | DynamoDB | ユーザーごとの権限で Backlog を叩きたい場合 |

## Prerequisites（共通）

- [lambroll](https://github.com/fujiwara/lambroll) (`brew install fujiwara/tap/lambroll`)
- [mise](https://mise.jdx.dev/)
- AWS CLI（認証済み）
- OIDC プロバイダ（Azure AD / Google Workspace / Auth0 など）で登録可能なクライアント

## 1. IAM Role の作成（共通）

Lambda 実行ロールを作成します（初回のみ）:

```bash
aws iam create-role \
  --role-name logvalet-lambda-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": { "Service": "lambda.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy \
  --role-name logvalet-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
```

ARN を確認:

```bash
aws iam get-role --role-name logvalet-lambda-role --query 'Role.Arn' --output text
```

> Mode B を使う場合は、後述の「Mode B 手順」で DynamoDB 用インラインポリシーを追加します。

## 2. OIDC クライアントの作成（共通）

利用する OIDC プロバイダで OAuth 2.0 / OIDC クライアントを作成します。

- **Redirect URI**: Function URL が確定するまでは仮の値（例: `https://example.com/callback`）で登録し、デプロイ後に確定値に更新します。
  - 確定値: `<FUNCTION_URL>/callback`
- 取得する値: `Issuer URL`, `Client ID`, `Client Secret`

## 3. `.env` の作成（共通）

```bash
cp .env.example .env
# .env を編集
```

最低限、以下の共通項目を埋めます:

- `ROLE_ARN` — 手順 1 で取得した Lambda 実行ロール ARN
- `LOGVALET_BASE_URL` — Backlog スペース URL
- OIDC 一式: `LOGVALET_MCP_EXTERNAL_URL`（暫定値）, `LOGVALET_MCP_OIDC_ISSUER`, `LOGVALET_MCP_OIDC_CLIENT_ID`, `LOGVALET_MCP_OIDC_CLIENT_SECRET`, `LOGVALET_MCP_COOKIE_SECRET`
- シークレット生成: `openssl rand -hex 32`

## 4. Mode A: OIDC + Backlog API Key

### 4.1 `.env` 追記

```dotenv
LOGVALET_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Mode B 用の `LOGVALET_MCP_BACKLOG_*` / `LOGVALET_MCP_TOKEN_STORE*` は空のまま。

### 4.2 デプロイ

```bash
mise run deploy
```

### 4.3 Function URL 確定と再デプロイ

```bash
aws lambda get-function-url-config --function-name logvalet-mcp \
  --query 'FunctionUrl' --output text
```

- `.env` の `LOGVALET_MCP_EXTERNAL_URL` を確定値に更新
- OIDC プロバイダの redirect_uri を `<FUNCTION_URL>/callback` に更新
- `mise run deploy` で再デプロイ

### 4.4 動作確認

ブラウザで Function URL にアクセス → OIDC ログイン → MCP エンドポイントが応答すれば OK。

## 5. Mode B: OIDC + Backlog OAuth（per-user, DynamoDB）

### 5.1 Backlog OAuth クライアントの作成

Backlog スペース上で OAuth クライアントを作成します。

- **Redirect URI**: `<FUNCTION_URL>/oauth/backlog/callback`（Function URL 確定後に更新）
- 取得: `Client ID`, `Client Secret`

### 5.2 IAM Role に DynamoDB policy を追加

```bash
aws iam put-role-policy \
  --role-name logvalet-lambda-role \
  --policy-name logvalet-dynamodb-tokens \
  --policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Action": ["dynamodb:GetItem","dynamodb:PutItem","dynamodb:DeleteItem","dynamodb:UpdateItem"],
      "Resource": "arn:aws:dynamodb:*:*:table/logvalet-oauth-tokens"
    }]
  }'
```

### 5.3 `.env` 追記

Mode A の `LOGVALET_API_KEY` は空にし、Mode B ブロックのコメントを外して設定:

```dotenv
LOGVALET_API_KEY=

LOGVALET_MCP_BACKLOG_CLIENT_ID=...
LOGVALET_MCP_BACKLOG_CLIENT_SECRET=...
LOGVALET_MCP_BACKLOG_REDIRECT_URL=https://xxx.lambda-url.ap-northeast-1.on.aws/oauth/backlog/callback
LOGVALET_MCP_OAUTH_STATE_SECRET=$(openssl rand -hex 32)
LOGVALET_MCP_TOKEN_STORE=dynamodb
LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1
```

### 5.4 DynamoDB テーブル作成

```bash
mise run dynamodb-create
```

### 5.5 デプロイ

```bash
mise run deploy
```

### 5.6 Function URL 確定と再デプロイ

```bash
aws lambda get-function-url-config --function-name logvalet-mcp \
  --query 'FunctionUrl' --output text
```

- `.env` の `LOGVALET_MCP_EXTERNAL_URL` / `LOGVALET_MCP_BACKLOG_REDIRECT_URL` を確定値に更新
- OIDC プロバイダ / Backlog OAuth クライアントの redirect_uri を確定値に更新
- `mise run deploy` で再デプロイ

### 5.7 動作確認

ブラウザで Function URL → OIDC ログイン → Backlog OAuth 連携 → MCP 応答。
トークンが DynamoDB に保存されていることを `aws dynamodb scan` などで確認。

## 6. クリーンアップ

```bash
# Lambda 関数削除
lambroll delete --function function.json

# Mode B の場合のみ
mise run dynamodb-delete
aws iam delete-role-policy --role-name logvalet-lambda-role --policy-name logvalet-dynamodb-tokens

# 共通
aws iam detach-role-policy --role-name logvalet-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
aws iam delete-role --role-name logvalet-lambda-role
mise run clean
```

## 環境変数リファレンス

### 共通

| 変数 | 必須 | 説明 |
|------|:---:|------|
| `AWS_REGION` | | デプロイ先リージョン（既定 `ap-northeast-1`） |
| `LOGVALET_VERSION` | | GitHub Release バージョン（既定 `0.11.0`） |
| `LAMBDA_ARCH` | | `arm64` または `x86_64`（既定 `arm64`） |
| `ROLE_ARN` | ✅ | Lambda 実行ロール ARN |
| `LOGVALET_BASE_URL` | ✅ | Backlog スペース URL |

### OIDC（両モード共通・必須）

| 変数 | 説明 |
|------|------|
| `LOGVALET_MCP_EXTERNAL_URL` | Function URL（deploy 後に確定値へ更新） |
| `LOGVALET_MCP_OIDC_ISSUER` | OIDC Issuer URL |
| `LOGVALET_MCP_OIDC_CLIENT_ID` | OIDC Client ID |
| `LOGVALET_MCP_OIDC_CLIENT_SECRET` | OIDC Client Secret |
| `LOGVALET_MCP_COOKIE_SECRET` | Cookie 暗号鍵（hex, 32 bytes 以上） |
| `LOGVALET_MCP_ALLOWED_DOMAINS` | 任意 — 許可するメールドメイン（カンマ区切り） |
| `LOGVALET_MCP_ALLOWED_EMAILS` | 任意 — 許可するメールアドレス（カンマ区切り） |

### Mode A（OIDC + API Key）

| 変数 | 説明 |
|------|------|
| `LOGVALET_API_KEY` | Backlog API キー（共有） |

### Mode B（OIDC + OAuth）

| 変数 | 説明 |
|------|------|
| `LOGVALET_MCP_BACKLOG_CLIENT_ID` | Backlog OAuth Client ID |
| `LOGVALET_MCP_BACKLOG_CLIENT_SECRET` | Backlog OAuth Client Secret |
| `LOGVALET_MCP_BACKLOG_REDIRECT_URL` | `<FUNCTION_URL>/oauth/backlog/callback` |
| `LOGVALET_MCP_OAUTH_STATE_SECRET` | state HMAC 鍵（hex, 16 bytes 以上） |
| `LOGVALET_MCP_TOKEN_STORE` | `dynamodb`（固定） |
| `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE` | DynamoDB テーブル名 |
| `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION` | DynamoDB リージョン |
