---
name: logvalet:logvalet
description: >
  Backlog 向け LLM-first CLI「logvalet」の PM メタモデル。
  全スキルの使い方・組み合わせ・ワークフローを案内するハブスキル。
  TRIGGER when: user asks "logvaletって何", "どのスキルを使えばいい", "backlogの操作方法",
  "logvalet help", "スキル一覧", "ワークフロー", "logvaletの使い方",
  "Backlogで何ができる", "課題管理のやり方", "タスク管理の方法",
  "backlog.com の操作", "プロジェクト管理をやりたい", "PM ワークフロー",
  "logvalet commands", "available skills", "what can logvalet do".
  DO NOT TRIGGER when: user has a specific task (issue creation, triage, report, etc.)
  — use the specialized skill instead.
---

# logvalet — Backlog PM メタモデル

logvalet プラグインの全スキルの使い方・組み合わせ・ワークフローを案内する。

## 認証・MCP の前提

Remote HTTP MCP は `none` または `apikey` を使う。`apikey` では
AgentCore Gateway が `X-Logvalet-Api-Key` を送り、任意の利用者識別情報を
`X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` で伝える。
Backlog credential は Bearer passthrough とする。HTTP mode は space store の
明示指定が必要で `memory` は使えない。tokenstore は CLI/stdio 専用で、
ローカルの SQLite または `tokens.json` のみを使い、DynamoDB は使わない。
stdio (`mcp-stdio`) は認証なしで CLI 資格情報を使い、リモート HTTP (`mcp`) は `--auth-mode` で認証方式を指定して使い分ける。

## 運用規約（conventions）を先に読む

プロジェクトに運用規約が導入されている場合、**作業を始める前に規約を読むこと。**
規約は「案件（engagement）とは何か」「優先度の低が何を意味するか」といった
組織の言葉の定義そのものであり、それを知らずに提案すると規約に反した助言になる。

```
logvalet_project_conventions(project_key="PROJ")
```

返り値の `adopted` が false なら規約未導入なので、従来どおり進めてよい。
true なら `conventions` と `glossary` が返るので、次を前提にする。

- 課題は**案件カテゴリをちょうど 1 つ**持ち、案件親課題の子課題にする
- 案件の Lead は 1 人。決まっていない案件は始めない
- 優先度の 高・中・低 の意味は `conventions.priority` に書かれている。
  一般論ではなくこの定義で判断する
- 案件は必ずいずれかの Initiative に属する

課題を起票するときは `engagement` パラメータを使う。案件名 1 つで
案件カテゴリと親課題の両方が設定される。

```
logvalet_issue_create(project_key="PROJ", summary="...", engagement="顧客A 基盤更改", ...)
```

`/logvalet:health` の `ambiguities` は「規約に照らして決まっていないこと」で、
案件不明の課題・Lead 不在の案件・クローズ候補などが挙がる。
規約導入済みプロジェクトのレビューでは必ず確認する。

規約の変更（apply）は書き込みを伴うため MCP からは行えない。
CLI の `logvalet project apply` を人が実行する。

## スキル一覧

### 📥 情報収集（現状把握）
| スキル | 用途 | いつ使う |
|--------|------|---------|
| `/logvalet:context` | 課題の全コンテキスト一括取得 | 「この課題どうなってる？」 |
| `/logvalet:my-week` | 今週の担当タスク＋ウォッチ課題 | 「今週何やるんだっけ」 |
| `/logvalet:my-next` | 直近の担当タスク＋ウォッチ課題 | 「明日何すればいい？」 |
| `/logvalet:decisions` | 過去の意思決定ログ | 「なぜこうなったか経緯を知りたい」 |

### 🔍 分析・診断（状態評価）
| スキル | 用途 | いつ使う |
|--------|------|---------|
| `/logvalet:health` | プロジェクト健全性 | 「プロジェクト大丈夫？」 |
| `/logvalet:risk` | 統合リスク評価 | 「リスクは？対策は？」 |
| `/logvalet:intelligence` | アクティビティ異常検知 | 「最近の動きに異常は？」 |
| `/logvalet:triage` | 課題トリアージ | 「優先度決めて・担当者提案して」 |

### ✍️ アクション（実行）
| スキル | 用途 | いつ使う |
|--------|------|---------|
| `/logvalet:draft` | コメント下書き | 「コメント書いて」 |
| `/logvalet:issue-create` | 対話型課題作成 | 「課題作って」 |
| `/logvalet:spec-to-issues` | 仕様書→課題分解 | 「specから課題を自動生成」 |

### 📊 レポート（報告・共有）
| スキル | 用途 | いつ使う |
|--------|------|---------|
| `/logvalet:report` | 月次・週次活動レポート | 「レポート作って」 |
| `/logvalet:digest-periodic` | 定期ダイジェスト | 「今週の進捗まとめて」 |

## ワークフロー例

### 🌅 朝のルーティン
1. `/logvalet:my-week` → 今週全体の俯瞰
2. `/logvalet:my-next` → 今日・明日の具体的なタスク

### 📋 プロジェクトレビュー
0. `logvalet_project_conventions` → 運用規約と用語を確認（導入済みなら必須）
1. `/logvalet:health PROJECT` → 全体の健全性スコア（`ambiguities` を含む）
2. `/logvalet:risk PROJECT` → リスク評価と推奨アクション
3. `/logvalet:intelligence PROJECT` → アクティビティの偏り・異常
4. `/logvalet:report PROJECT` → 共有用レポート生成

### 🔧 課題対応フロー
1. `/logvalet:context ISSUE` → コンテキスト一括取得
2. `/logvalet:decisions ISSUE` → 過去の意思決定を確認
3. `/logvalet:triage ISSUE` → 優先度・担当者を提案
4. `/logvalet:draft ISSUE` → 対応コメントを下書き

### 🚀 新規開発キックオフ
1. `/logvalet:spec-to-issues` → 仕様書から課題を自動生成
2. `/logvalet:health PROJECT` → 現状のリソース確認
3. `/logvalet:digest-periodic PROJECT` → 定期進捗追跡を開始

## MCP での spaces/all_spaces 使い方

logvalet MCP サーバーの 75 ツールはすべて `spaces` / `all_spaces` パラメータに対応している。

### Read-only: 登録済み全スペースを横断取得

```json
{
  "tool": "logvalet_issue_list",
  "arguments": {
    "project_id": 1,
    "all_spaces": true
  }
}
```

`all_spaces: true` を指定すると、登録済み全スペースのイシューをまとめて返す。

### Read-only: 特定スペースを指定

```json
{
  "tool": "logvalet_issue_list",
  "arguments": {
    "project_id": 1,
    "spaces": ["foo", "bar"]
  }
}
```

### Write: 単一スペースへの課題作成

```json
{
  "tool": "logvalet_issue_create",
  "arguments": {
    "spaces": ["foo"],
    "project_id": 1,
    "summary": "課題タイトル"
  }
}
```

Write 操作（create/update 等）は必ず単一スペースを `spaces` で指定する。`all_spaces` は Read-only ツールのみ有効。

## CLI 基本情報
- コマンド: `logvalet` (エイリアス: `lv`)
- 出力: JSON (デフォルト) / YAML / Markdown / Gantt
- 初期設定: `logvalet configure`
- 各コマンドの詳細は個別スキルを参照

## ウォッチ（CLI 直接操作）

ウォッチ課題は担当ではないが自分の仕事に影響する課題。スキル（my-week, my-next 等）で自動表示されるが、CLI で直接操作も可能:

```bash
lv watching list me          # 自分のウォッチ一覧
lv watching count me         # 件数
lv watching get <ID>         # 詳細
lv watching add PROJ-123     # ウォッチ追加
lv watching delete <ID>      # ウォッチ解除
lv watching mark-as-read <ID> # 既読化
```

## 関連課題（CLI 直接操作）

課題間の関連付け。専用スキルはなく CLI で直接操作する。Backlog の非公開 API
（`/api/v2/issues/{issueKey}/relatedIssues`）を利用しており、レスポンス形式や
挙動は Backlog による公式な保証がない:

```bash
lv issue related list PROJ-123              # 関連課題一覧
lv issue related add PROJ-123 456789         # 関連課題を追加（数値の課題 ID）
lv issue related remove PROJ-123 789012      # 関連課題を削除（関連 ID）
```
