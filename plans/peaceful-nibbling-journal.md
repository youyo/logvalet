---
title: Lambda Function URL + lambroll デプロイ雛形
project: logvalet
author: planning-agent
created: 2026-04-12
status: Draft
complexity: M
---

# Lambda Function URL + lambroll デプロイ雛形

## Context

AgentCore デプロイから Lambda Function URL デプロイに切り替えるため、`examples/lambroll/` にすぐ使える雛形セットを作成する。
GitHub Release のバイナリ + bootstrap シェルスクリプトを zip にまとめ、Lambda Web Adapter (LWA) Layer 経由で HTTP リクエストを logvalet MCP サーバーに中継する構成。

## スコープ

### 実装範囲
- `examples/lambroll/` ディレクトリ（bootstrap, function.json, Makefile）
- README.md / README.ja.md にデプロイ手順セクション追加

### スコープ外
- IAM Role / Function URL の自動作成（Makefile の手動ターゲットとして提供）
- CI/CD パイプライン統合
- 認証設定（OIDC）のテンプレート化（コメントで案内のみ）

## アーキテクチャ

```
Lambda Function URL
  → Lambda Web Adapter (Layer)
    → bootstrap (shell script)
      → logvalet mcp --host 0.0.0.0 (port 8080)
```

LWA がデフォルトでポート 8080 に中継 → logvalet のデフォルトポートと一致するため追加設定不要。

## 変更対象ファイル

| ファイル | 操作 | 内容 |
|---------|------|------|
| `examples/lambroll/bootstrap` | 新規作成 | logvalet 起動スクリプト（3行） |
| `examples/lambroll/function.json` | 新規作成 | lambroll 関数定義 |
| `examples/lambroll/function_url.json` | 新規作成 | Function URL 定義（AuthType: NONE） |
| `examples/lambroll/mise.toml` | 新規作成 | download / package / deploy タスク定義 |
| `README.md` | 編集 (L574後) | Lambda デプロイセクション追加 |
| `README.ja.md` | 編集 (L573後) | Lambda デプロイセクション追加（日本語） |

## 実装手順

### Step 1: `examples/lambroll/bootstrap`

```sh
#!/bin/sh
set -eu
exec ./logvalet mcp --host 0.0.0.0 --port "${PORT:-8080}"
```

- `/bin/sh` を使用（`provided.al2023` で保証）
- `exec` で logvalet に PID 1 を渡し、SIGTERM を直接受信させる
- `PORT` 環境変数でポートを制御（デフォルト 8080、LWA の `AWS_LWA_PORT` と合わせる）

### Step 2: `examples/lambroll/function.json`

```json
{
  "FunctionName": "logvalet-mcp",
  "Description": "logvalet MCP server on Lambda Function URL",
  "Runtime": "provided.al2023",
  "Handler": "bootstrap",
  "Architectures": ["arm64"],
  "MemorySize": 256,
  "Timeout": 900,
  "Role": "{{ must_env `ROLE_ARN` }}",
  "Layers": [
    "arn:aws:lambda:{{ must_env `AWS_REGION` }}:753240598075:layer:LambdaAdapterLayerArm64:27"
  ],
  "Environment": {
    "Variables": {
      "LOGVALET_API_KEY": "{{ must_env `LOGVALET_API_KEY` }}",
      "LOGVALET_BASE_URL": "{{ must_env `LOGVALET_BASE_URL` }}"
    }
  }
}
```

設計判断:
- `provided.al2023`: カスタムランタイム（LWA が Runtime API を処理）
- `arm64`: Graviton、コスト最適
- `Timeout: 900`: Function URL のストリーミング MCP セッション用に最大値
- `MemorySize: 256`: Go バイナリの Cold Start を考慮（128 だと遅い）
- `must_env`: lambroll のテンプレート関数でデプロイ時に環境変数を解決
- LWA Layer バージョン `27`（v1.0.0 GA）が 2026 年時点の最新

### Step 2.5: `examples/lambroll/function_url.json`

```json
{
  "Config": {
    "AuthType": "NONE"
  }
}
```

- lambroll の `--function-url` オプションで Function URL をデプロイ時に同時管理
- `AuthType: NONE` で公開エンドポイント（IAM 認証が必要な場合は `AWS_IAM` に変更）
- `Permissions` は `NONE` の場合自動で `Principal: *` が設定される

### Step 3: `examples/lambroll/mise.toml` 新規作成

`examples/lambroll/` にローカル `mise.toml` を配置し、lambroll デプロイ用タスクを定義する。

```toml
[tasks.download]
description = "Download logvalet binary from GitHub Release"
run = """
VERSION=${LOGVALET_VERSION:-0.9.1}
ARCH=${LAMBDA_ARCH:-arm64}
ARCHIVE="logvalet_${VERSION}_Linux_${ARCH}.tar.gz"
curl -fsSL -o "${ARCHIVE}" "https://github.com/youyo/logvalet/releases/download/v${VERSION}/${ARCHIVE}"
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

設計判断:
- `examples/lambroll/mise.toml` にローカル配置（`cd examples/lambroll && mise run deploy`）
- タスク名は `lambda:` プレフィックス不要（ディレクトリスコープで自明）
- `depends` で download → package → deploy の依存チェーン
- 環境変数 `LOGVALET_VERSION`, `LAMBDA_ARCH` でバージョン・アーキテクチャを指定
- `zip -j`: フラットアーカイブ（Lambda は `/var/task/` にルート展開）
- `chmod +x`: zip 内のパーミッション保持（これがないと Permission denied）
- Function URL は `--function-url function_url.json` で lambroll が管理（別途 AWS CLI 不要）

### Step 4: README.md 編集

L574（AgentCore 参照行）の後に挿入:

```markdown

### Lambda Function URL (lambroll)

Deploy logvalet as a Lambda Function URL using [lambroll](https://github.com/fujiwara/lambroll) and [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter).
See [examples/lambroll/](examples/lambroll/) for the template.

```bash
cd examples/lambroll

export AWS_REGION=ap-northeast-1
export ROLE_ARN=arn:aws:iam::123456789012:role/logvalet-lambda-role
export LOGVALET_API_KEY=your-api-key
export LOGVALET_BASE_URL=https://your-space.backlog.com

mise run deploy      # Download binary, package, deploy, and create Function URL
```
```

### Step 5: README.ja.md 編集

L573（AgentCore 参照行）の後に挿入:

```markdown

### Lambda Function URL (lambroll)

[lambroll](https://github.com/fujiwara/lambroll) と [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter) を使用して logvalet を Lambda Function URL にデプロイできます。
テンプレート一式は [examples/lambroll/](examples/lambroll/) を参照してください。

```bash
cd examples/lambroll

export AWS_REGION=ap-northeast-1
export ROLE_ARN=arn:aws:iam::123456789012:role/logvalet-lambda-role
export LOGVALET_API_KEY=your-api-key
export LOGVALET_BASE_URL=https://your-space.backlog.com

mise run deploy      # バイナリダウンロード・パッケージ・デプロイ・Function URL 作成
```
```

## テスト設計書

テスト対象はテンプレートファイル（静的設定ファイル）のため、自動テストは N/A。

### 検証方法
1. `examples/lambroll/` の 3 ファイルが存在すること
2. `bootstrap` が `#!/bin/sh` で始まり実行可能フラグを持つこと
3. `function.json` が有効な JSON であること（`jq . function.json`）
4. `examples/lambroll/mise.toml` のタスクが正しく定義されていること（`cd examples/lambroll && mise tasks` で確認）
5. README.md / README.ja.md に Lambda セクションが追加されていること

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| LWA Layer バージョン陳腐化 | 低 | v27 (1.0.0 GA) を使用。[awslabs/aws-lambda-web-adapter](https://github.com/awslabs/aws-lambda-web-adapter) で最新を確認 |
| bootstrap の exec 忘れ | 中 | `exec` を明記、コメントで理由を説明 |
| zip パーミッション不足 | 中 | Makefile で `chmod +x` を明示 |

## チェックリスト

- [x] 観点1: 実装実現可能性 — 5ステップ、全ファイル名明記、依存関係なし
- [x] 観点2: TDD — N/A（静的テンプレート）
- [x] 観点3: アーキテクチャ整合性 — 既存 Dockerfile/README パターンに準拠
- [x] 観点4: リスク評価 — 3件特定、対策明記
- [x] 観点5: シーケンス図 — アーキテクチャ図で代替（単純な直列フロー）

---

## Next Action

> **このプランが承認されたら:**
>
> 1. `Skill(devflow:implement)` — このプランに基づいて実装を開始
