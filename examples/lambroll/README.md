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

ARN を確認してください:

```bash
aws iam get-role --role-name logvalet-lambda-role --query 'Role.Arn' --output text
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
