# plan: examples/lambroll を OIDC 前提 + Backlog 認証 2 モード構成に対応

## Context

`examples/lambroll` は Backlog API キーによるシングルテナント構成（OIDC なし）を前提にしていて、
`function.json` の環境変数は `LOGVALET_API_KEY` / `LOGVALET_BASE_URL` のみ。
一方 MCP サーバーは既に OIDC 認証（idproxy）と Backlog OAuth（per-user）に対応しており、
リモート MCP を安全に公開するには **OIDC は常に有効化** すべき。
Backlog 側の認証情報は「共有 API キー」と「per-user OAuth（DynamoDB TokenStore）」の 2 形態から選択する。

本プランでは lambroll サンプルを以下の 2 モード並記構成に刷新する:

- **Mode A: OIDC + Backlog API Key** — MCP アクセスは OIDC で認証、Backlog は全員共通の API キーで叩く
- **Mode B: OIDC + Backlog OAuth** — MCP アクセスは OIDC で認証、Backlog は各ユーザーの OAuth トークン（DynamoDB 保存）で叩く

DynamoDB テーブル作成を mise task として同梱し、`LOGVALET_VERSION` を最新リリース `0.11.0` に更新する。

## 要件

- OIDC は両モード共通で必須（`LOGVALET_MCP_AUTH=true` 固定）
- Backlog 認証方式のみ Mode A/B で切り替え
- DynamoDB テーブル作成 / 削除を mise task として提供（Mode B のみ使用）
- Lambda 環境変数を最新仕様（`LOGVALET_MCP_*` 一式）に合わせる
- `LOGVALET_VERSION` を `0.11.0` に更新
- `.env.example` を新規作成し、共通 / Mode A / Mode B の区画で整理
- README を 2 モード並列構成に書き換え

## 変更対象ファイル

- `examples/lambroll/mise.toml` — `dynamodb-create` / `dynamodb-delete` task 追加、既存 task 維持
- `examples/lambroll/function.json` — Environment.Variables を OIDC + OAuth + DynamoDB 構成に差し替え
- `examples/lambroll/.env.example` — 新規作成
- `examples/lambroll/README.md` — セクションを OIDC + OAuth 前提に再構成
- `examples/lambroll/bootstrap` — 変更なし（`logvalet mcp` 起動のまま）

## 設計詳細

### 1. DynamoDB 作成 mise task

`mise.toml` に以下を追加:

```toml
[tasks."dynamodb-create"]
description = "Create DynamoDB table for OAuth TokenStore"
run = """
aws dynamodb create-table \
  --table-name "${LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE}" \
  --attribute-definitions AttributeName=pk,AttributeType=S \
  --key-schema AttributeName=pk,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region "${LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION:-$AWS_REGION}"
aws dynamodb wait table-exists \
  --table-name "${LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE}" \
  --region "${LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION:-$AWS_REGION}"
"""

[tasks."dynamodb-delete"]
description = "Delete DynamoDB TokenStore table (destructive)"
run = """
aws dynamodb delete-table \
  --table-name "${LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE}" \
  --region "${LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION:-$AWS_REGION}"
"""
```

スキーマ根拠: PK=`pk`(S)、Sort Key/GSI/TTL なし（Explore 調査結果）。
billing-mode は on-demand（PAY_PER_REQUEST）で最小運用コスト。

### 2. function.json 環境変数の刷新（2 モード対応）

OIDC 関連は両モードで **必須**（`must_env`）、Backlog 認証部分と DynamoDB 設定は
`env`（空デフォルト）にして、`.env` 側で Mode A/B を切り替える方式にする。

```json
{
  "LOGVALET_BASE_URL":                        "{{ must_env `LOGVALET_BASE_URL` }}",

  "LOGVALET_MCP_AUTH":                        "true",
  "LOGVALET_MCP_EXTERNAL_URL":                "{{ must_env `LOGVALET_MCP_EXTERNAL_URL` }}",
  "LOGVALET_MCP_OIDC_ISSUER":                 "{{ must_env `LOGVALET_MCP_OIDC_ISSUER` }}",
  "LOGVALET_MCP_OIDC_CLIENT_ID":              "{{ must_env `LOGVALET_MCP_OIDC_CLIENT_ID` }}",
  "LOGVALET_MCP_OIDC_CLIENT_SECRET":          "{{ must_env `LOGVALET_MCP_OIDC_CLIENT_SECRET` }}",
  "LOGVALET_MCP_COOKIE_SECRET":               "{{ must_env `LOGVALET_MCP_COOKIE_SECRET` }}",
  "LOGVALET_MCP_ALLOWED_DOMAINS":             "{{ env `LOGVALET_MCP_ALLOWED_DOMAINS` `` }}",
  "LOGVALET_MCP_ALLOWED_EMAILS":              "{{ env `LOGVALET_MCP_ALLOWED_EMAILS` `` }}",

  "LOGVALET_API_KEY":                         "{{ env `LOGVALET_API_KEY` `` }}",

  "LOGVALET_MCP_BACKLOG_CLIENT_ID":           "{{ env `LOGVALET_MCP_BACKLOG_CLIENT_ID` `` }}",
  "LOGVALET_MCP_BACKLOG_CLIENT_SECRET":       "{{ env `LOGVALET_MCP_BACKLOG_CLIENT_SECRET` `` }}",
  "LOGVALET_MCP_BACKLOG_REDIRECT_URL":        "{{ env `LOGVALET_MCP_BACKLOG_REDIRECT_URL` `` }}",
  "LOGVALET_MCP_OAUTH_STATE_SECRET":          "{{ env `LOGVALET_MCP_OAUTH_STATE_SECRET` `` }}",
  "LOGVALET_MCP_TOKEN_STORE":                 "{{ env `LOGVALET_MCP_TOKEN_STORE` `memory` }}",
  "LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE":  "{{ env `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE` `` }}",
  "LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION": "{{ env `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION` `` }}"
}
```

- **Mode A (OIDC + API Key)**: `.env` で OIDC 一式 + `LOGVALET_API_KEY` を設定。Backlog OAuth / DynamoDB 系は空。
- **Mode B (OIDC + OAuth)**: `.env` で OIDC 一式 + `LOGVALET_MCP_BACKLOG_*` + DynamoDB 設定。`LOGVALET_API_KEY` は空。

**単一 function.json で成立する根拠**（実装確認済み）:

- `internal/auth/config.go` の `OAuthEnabled()` は `BacklogClientID != ""` が唯一の条件。
  Mode A では `LOGVALET_MCP_BACKLOG_CLIENT_ID` を空にすれば OAuth ブロック全体が無効化され、
  `ClientSecret` / `RedirectURL` / `OAuthStateSecret` の必須チェックはスキップされる。
- `ParseStoreType("")` は `StoreTypeMemory` を返し、Mode A で `LOGVALET_MCP_TOKEN_STORE=""` なら
  `DynamoDB_TABLE` / `_REGION` の検証も走らない。
- `LOGVALET_API_KEY` は `credentials.Resolver` が空チェック済み、空なら未設定扱い。
- `LOGVALET_MCP_ALLOWED_DOMAINS` / `_EMAILS` は空なら制限なし（idproxy に nil 渡し）。

よって上記 `function.json` をそのまま使い、`.env` の設定差分で Mode A/B を切り替えられる。

### 3. .env.example（新規, 両モード対応）

セクションで分けて記載:

```dotenv
# =============================================================
# 共通（必須）
# =============================================================
ROLE_ARN=arn:aws:iam::123456789012:role/logvalet-lambda-role
LOGVALET_BASE_URL=https://example.backlog.com

# --- OIDC（両モード共通・必須） ---
LOGVALET_MCP_EXTERNAL_URL=https://xxx.lambda-url.ap-northeast-1.on.aws
LOGVALET_MCP_OIDC_ISSUER=https://your-idp.example.com
LOGVALET_MCP_OIDC_CLIENT_ID=...
LOGVALET_MCP_OIDC_CLIENT_SECRET=...
LOGVALET_MCP_COOKIE_SECRET=       # openssl rand -hex 32

# 任意: アクセス制限
# LOGVALET_MCP_ALLOWED_DOMAINS=example.com,example.co.jp
# LOGVALET_MCP_ALLOWED_EMAILS=user1@example.com

# =============================================================
# Mode A: OIDC + Backlog API Key（以下 1 行のみ設定）
# =============================================================
LOGVALET_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# =============================================================
# Mode B: OIDC + Backlog OAuth（per-user, DynamoDB）
#   → 使う場合は Mode A の LOGVALET_API_KEY を空にし、以下を設定
# =============================================================
# LOGVALET_MCP_BACKLOG_CLIENT_ID=...
# LOGVALET_MCP_BACKLOG_CLIENT_SECRET=...
# LOGVALET_MCP_BACKLOG_REDIRECT_URL=https://xxx.lambda-url.ap-northeast-1.on.aws/oauth/backlog/callback
# LOGVALET_MCP_OAUTH_STATE_SECRET=  # openssl rand -hex 32
# LOGVALET_MCP_TOKEN_STORE=dynamodb
# LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
# LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1
```

### 4. IAM role 追加 policy（Mode B のみ必要）

DynamoDB アクセス用のインラインポリシーを `logvalet-lambda-role` に付与:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["dynamodb:GetItem","dynamodb:PutItem","dynamodb:DeleteItem","dynamodb:UpdateItem"],
    "Resource": "arn:aws:dynamodb:*:*:table/logvalet-oauth-tokens"
  }]
}
```

README の「1. IAM Role の作成」セクションに `aws iam put-role-policy` 例を追記。

### 5. README の再構成（2 モード並記）

構成:

1. 概要と 2 モード比較表（OIDC+API Key vs OIDC+OAuth）
2. 共通 Prerequisites（lambroll, mise, AWS CLI, OIDC プロバイダ）
3. 共通: IAM Role 作成（Basic 実行ロール）
4. 共通: OIDC プロバイダでクライアント作成（redirect_uri 暫定値）
5. 共通: `.env` 作成（`.env.example` コピー、共通 + OIDC 変数を入力、secrets は `openssl rand -hex 32`）
6. **Mode A: OIDC + Backlog API Key の手順**
   - `.env` に `LOGVALET_API_KEY` を追記
   - `mise run deploy`
   - Function URL を取得し `LOGVALET_MCP_EXTERNAL_URL` を確定値に更新
   - OIDC プロバイダの redirect_uri を確定値に更新
   - `mise run deploy`（再デプロイ）
   - 動作確認（OIDC ログイン → MCP 応答）
7. **Mode B: OIDC + Backlog OAuth の手順**
   - Backlog で OAuth クライアント作成
   - IAM Role に DynamoDB policy 追加
   - `.env` に Mode B 変数一式を追記（`LOGVALET_API_KEY` は空）
   - `mise run dynamodb-create`
   - `mise run deploy` → Function URL 取得
   - `LOGVALET_MCP_EXTERNAL_URL` / `LOGVALET_MCP_BACKLOG_REDIRECT_URL` を確定値で更新
   - OIDC / Backlog 両方の redirect_uri を確定値に更新
   - `mise run deploy`（再デプロイ）
   - 動作確認（OIDC ログイン → Backlog OAuth 連携 → MCP 応答）
8. クリーンアップ（`mise run dynamodb-delete`, `lambroll delete`）
9. 環境変数リファレンス表（共通 / OIDC 共通 / Mode A / Mode B の 4 区画）

`LOGVALET_VERSION` を `mise.toml` で `0.11.0` に更新し、README の記載も合わせる。

## 検証

- `mise tasks` で `dynamodb-create` / `dynamodb-delete` が表示
- `lambroll diff --function function.json` がテンプレート展開エラーなしで完走
- Mode A: OIDC 一式 + `LOGVALET_API_KEY` → `mise run deploy` → OIDC ログイン経由で MCP 応答
- Mode B:
  - `mise run dynamodb-create` → `aws dynamodb describe-table` で ACTIVE 確認
  - `mise run deploy` → `aws lambda get-function-configuration` で Environment.Variables 反映
  - Function URL にブラウザアクセス → OIDC ログインへリダイレクト
  - ログイン後 Backlog OAuth フロー通過 → DynamoDB に token 保存確認
- `LOGVALET_VERSION=0.11.0` でバイナリダウンロード成功

## 注意事項

- `bootstrap` / `function_url.json` は変更なし（AuthType=NONE、OIDC は logvalet 内部で処理）
- 単一 `function.json` 構成は実装確認済みで成立（上記「根拠」セクション参照）
- Mode A/B の切替は `.env` 差分のみ。lambroll は `function.json` を毎回再テンプレート展開するため
  Lambda の Environment.Variables も `.env` に追従する
