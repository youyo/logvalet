---
name: logvalet
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
1. `/logvalet:health PROJECT` → 全体の健全性スコア
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
