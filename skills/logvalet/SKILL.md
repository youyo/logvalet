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

利用経路は3つあり、違いは Backlog 資格情報の出所にある。

- CLI (`lv ...`) と stdio (`mcp-stdio`) はサーバー側の資格情報を使う。
  設定・env・フラグの API キーまたはアクセストークンで、呼び出せるのはバイナリを
  実行できるローカル利用者に限られる。
- Remote HTTP (`mcp`) は呼び出し元を認証しない。Backlog 資格情報はリクエストごとの
  `Authorization: Bearer <token>` で受け取り、logvalet はそれをそのまま Backlog API に
  渡す（Bearer passthrough）。トークンの保存・リフレッシュは行わない。

Remote HTTP はサポート構成として Cloudflare MCP Server Portals の背後に置く。
Portals が利用者を認証し、その利用者の Backlog OAuth トークンを Bearer で注入する。
利用者ごとのアクセス制御・監査ログ・tool 許可リストは Portals 側の機能で、
logvalet 単体では提供しない。

複数の Backlog スペースは logvalet 内では扱わず、スペース毎に MCP サーバーを立てて
Portals に登録する。

v0.40 で `--auth-mode`・`--auth-api-key`・`X-Logvalet-*` ヘッダーと内蔵 OAuth
コールバックを廃止した。

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
