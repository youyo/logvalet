---
title: examples/lambroll の改善（README + 環境変数管理）
project: logvalet
author: planning-agent
created: 2026-04-13
status: Draft
complexity: L
---

# examples/lambroll の改善（README + 環境変数管理）

## Context

前タスクで作成した雛形を改善する:
1. メイン README の Lambda 手順を `examples/lambroll/README.md` に移し、リンクのみ残す
2. mise.toml に `[env]` セクションと `.env` 読み込みを追加
3. IAM Role 作成手順を README に含める（lambroll は IAM Role を管理しない）

## 変更対象ファイル

| ファイル | 操作 | 内容 |
|---------|------|------|
| `examples/lambroll/README.md` | 新規作成 | 詳細デプロイ手順（IAM Role 作成 + mise run deploy） |
| `examples/lambroll/mise.toml` | 編集 | `[env]` セクション追加 + シェル展開を簡略化 |
| `examples/lambroll/.env.example` | 新規作成 | 必須シークレット変数の雛形 |
| `README.md` | 編集 | Lambda セクションをリンクのみに差し替え |
| `README.ja.md` | 編集 | 同上（日本語） |
| `.gitignore` | 編集 | `examples/lambroll/.env` を追加 |

## 実装内容

### examples/lambroll/README.md（新規）

```markdown
# logvalet on Lambda Function URL

lambroll + Lambda Web Adapter (LWA) を使って logvalet MCP サーバーを Lambda Function URL にデプロイします。

## Prerequisites

- [lambroll](https://github.com/fujiwara/lambroll) (`brew install fujiwara/tap/lambroll`)
- [mise](https://mise.jdx.dev/)
- AWS CLI（認証済み）

## 1. IAM Role の作成

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

## 2. 環境変数の設定

```bash
cp .env.example .env
# .env を編集して実際の値を入力
```

## 3. デプロイ

```bash
mise run deploy
```

初回は Lambda 関数と Function URL が同時に作成されます。
2回目以降は関数コードと設定のみ更新されます。

## 4. Function URL の確認

```bash
aws lambda get-function-url-config --function-name logvalet-mcp \
  --query 'FunctionUrl' --output text
```

## 設定のカスタマイズ

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `AWS_REGION` | `ap-northeast-1` | デプロイ先リージョン |
| `LOGVALET_VERSION` | `0.9.1` | GitHub Release のバージョン |
| `LAMBDA_ARCH` | `arm64` | `arm64` または `x86_64` |
| `ROLE_ARN` | — | Lambda 実行ロール ARN（必須） |
| `LOGVALET_API_KEY` | — | Backlog API キー（必須） |
| `LOGVALET_BASE_URL` | — | Backlog スペース URL（必須） |
```

### examples/lambroll/mise.toml（編集）

`[env]` セクションを先頭に追加し、タスクのシェル展開を簡略化:

```toml
[env]
# Default values (override via .env or shell environment)
AWS_REGION = "ap-northeast-1"
LOGVALET_VERSION = "0.9.1"
LAMBDA_ARCH = "arm64"
# Load secrets from .env (ROLE_ARN, LOGVALET_API_KEY, LOGVALET_BASE_URL)
_.file = ".env"

[tasks.download]
description = "Download logvalet binary from GitHub Release"
run = """
ARCHIVE="logvalet_${LOGVALET_VERSION}_Linux_${LAMBDA_ARCH}.tar.gz"
curl -fsSL -o "${ARCHIVE}" "https://github.com/youyo/logvalet/releases/download/v${LOGVALET_VERSION}/${ARCHIVE}"
tar xzf "${ARCHIVE}" logvalet
rm -f "${ARCHIVE}"
"""

[tasks.package]
description = "Package logvalet + bootstrap into function.zip"
depends = ["download"]
run = """
chmod +x bootstrap logvalet
zip -j function.zip bootstrap logvalet
"""

[tasks.deploy]
description = "Deploy logvalet to Lambda via lambroll (with Function URL)"
depends = ["package"]
run = "lambroll deploy --function function.json --function-url function_url.json"

[tasks.clean]
description = "Remove packaging artifacts"
run = "rm -f logvalet function.zip"
```

### examples/lambroll/.env.example（新規）

```
# Required: copy to .env and fill in your values
ROLE_ARN=arn:aws:iam::123456789012:role/logvalet-lambda-role
LOGVALET_API_KEY=your-api-key
LOGVALET_BASE_URL=https://your-space.backlog.com

# Optional: override defaults defined in mise.toml [env]
# AWS_REGION=ap-northeast-1
# LOGVALET_VERSION=0.9.1
# LAMBDA_ARCH=arm64
```

### README.md / README.ja.md（編集）

現在の Lambda セクション（`mise run deploy` のコードブロックを含む複数行）を以下に差し替え:

**README.md:**
```markdown
### Lambda Function URL (lambroll)

Deploy logvalet as a Lambda Function URL using [lambroll](https://github.com/fujiwara/lambroll) and [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter).
See [examples/lambroll/](examples/lambroll/) for setup instructions.
```

**README.ja.md:**
```markdown
### Lambda Function URL (lambroll)

[lambroll](https://github.com/fujiwara/lambroll) と [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter) を使用して logvalet を Lambda Function URL にデプロイできます。
セットアップ手順は [examples/lambroll/](examples/lambroll/) を参照してください。
```

### .gitignore（編集）

既存の `.envrc` 行の近くに追加:
```
examples/lambroll/.env
```

## 検証方法

1. `examples/lambroll/README.md` が存在し、IAM Role 作成〜デプロイの手順が揃っていること
2. `examples/lambroll/.env.example` に必要変数が揃っていること
3. `cd examples/lambroll && mise tasks` でタスク一覧が表示されること
4. メイン README にリンクのみ残っていること
5. `.gitignore` に `examples/lambroll/.env` が含まれること

---

## Next Action

> **このプランが承認されたら:**
>
> 1. `Skill(devflow:implement)` — このプランに基づいて実装を開始
