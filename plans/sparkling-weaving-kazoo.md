# プラグイン形式への移行 + Root スキル再設計 + Description 全面刷新

## Context

logvalet のスキル群（14個）を Claude Code マーケットプレイス対応プラグイン形式に移行する。
プラグイン形式では呼び出し時に `logvalet:` が自動付与されるため、ディレクトリ名から `logvalet-` を削除。
root スキルを「PM メタモデル」に再設計し、全スキルの組み合わせ・ワークフローを案内するハブにする。

**最重要**: 各スキルの `description` を全力で最適化する。description の精度が
AI エージェントの自動選択精度を決定する。

---

## 1. `.claude-plugin/` ディレクトリ

### `.claude-plugin/marketplace.json`
```json
{
  "name": "youyo",
  "owner": {
    "name": "youyo"
  },
  "plugins": [
    {
      "name": "logvalet",
      "description": "LLM-first Backlog CLI skills: PM workflows, project health, risk analysis, activity intelligence, issue triage, and periodic digests for Backlog.com",
      "source": "./"
    }
  ]
}
```

### `.claude-plugin/plugin.json`
```json
{
  "name": "logvalet",
  "description": "LLM-first Backlog CLI skills for Claude Code: PM workflows, project health, risk analysis, activity intelligence, issue triage, and periodic digests",
  "version": "0.6.2",
  "author": {
    "name": "youyo"
  },
  "homepage": "https://github.com/youyo/logvalet",
  "license": "MIT"
}
```

---

## 2. スキルリネーム一覧

| 現在 | 変更後 | 呼び出し名 |
|------|--------|-----------|
| `skills/logvalet/` | `skills/logvalet/` | `logvalet:logvalet` |
| `skills/logvalet-context/` | `skills/context/` | `logvalet:context` |
| `skills/logvalet-decisions/` | `skills/decisions/` | `logvalet:decisions` |
| `skills/logvalet-digest-periodic/` | `skills/digest-periodic/` | `logvalet:digest-periodic` |
| `skills/logvalet-draft/` | `skills/draft/` | `logvalet:draft` |
| `skills/logvalet-health/` | `skills/health/` | `logvalet:health` |
| `skills/logvalet-intelligence/` | `skills/intelligence/` | `logvalet:intelligence` |
| `skills/logvalet-issue-create/` | `skills/issue-create/` | `logvalet:issue-create` |
| `skills/logvalet-my-next/` | `skills/my-next/` | `logvalet:my-next` |
| `skills/logvalet-my-week/` | `skills/my-week/` | `logvalet:my-week` |
| `skills/logvalet-report/` | `skills/report/` | `logvalet:report` |
| `skills/logvalet-risk/` | `skills/risk/` | `logvalet:risk` |
| `skills/logvalet-spec-to-issues/` | `skills/spec-to-issues/` | `logvalet:spec-to-issues` |
| `skills/logvalet-triage/` | `skills/triage/` | `logvalet:triage` |

各 SKILL.md の frontmatter `name:` も同様にリネーム。

---

## 3. Description 全面刷新

### 設計原則（調査結果に基づく）

1. **TRIGGER when / DO NOT TRIGGER when** を明示 — エージェント判定精度の核心
2. **日本語・英語の両方でトリガーワードを網羅** — ユーザーがどちらで話しても引っかかる
3. **Backlog / 課題管理 / タスク管理 / プロジェクト管理** の文脈キーワードを漏れなく含める
4. **pushy スタイル** — 「Use this skill whenever...」で積極的にトリガー
5. **ユースケース列挙** — 具体的なシーンで「これだ」と直感できるように
6. **スキル間の導線** — 前後のスキルを明示してワークフローを形成
7. **否定条件** — 競合する他スキルとの誤トリガーを防止

---

### 3.1 `logvalet` (root) — PM メタモデル

```yaml
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
```

**SKILL.md 本文**: CLI リファレンスを削除し、以下の構成に全面書き換え:

```markdown
# logvalet — Backlog PM メタモデル

logvalet プラグインの全スキルの使い方・組み合わせ・ワークフローを案内する。

## スキル一覧

### 📥 情報収集（現状把握）
| スキル | 用途 | いつ使う |
|--------|------|---------|
| `/logvalet:context` | 課題の全コンテキスト一括取得 | 「この課題どうなってる？」 |
| `/logvalet:my-week` | 今週の自分のタスク一覧 | 「今週何やるんだっけ」 |
| `/logvalet:my-next` | 直近数日のタスク | 「明日何すればいい？」 |
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
- 初期設定: `logvalet config init` → `logvalet auth login`
- 各コマンドの詳細は個別スキルを参照
```

---

### 3.2 `context` — 課題コンテキスト

```yaml
name: context
description: >
  Fetch full context for a Backlog issue in one shot: issue details, comments, status signals,
  overdue/stale detection, and LLM hints — everything needed to understand and act on an issue.
  TRIGGER when: user provides a Backlog issue key (e.g. PROJ-123, ESU2_S2-32),
  a Backlog URL (*.backlog.com/view/*, *.backlog.com/alias/*),
  or says "課題の詳細", "この課題について", "issue context", "課題を理解したい",
  "issue を調べて", "この issue の状況", "課題の背景", "課題の情報をまとめて",
  "PROJ-123 について教えて", "バックログの課題の詳細", "issue summary",
  "課題コンテキスト", "backlog issue context", "チケットの状態",
  "このチケットについて", "課題の全体像", "what's the status of this issue".
  DO NOT TRIGGER when: user wants to create/update an issue (use issue-create),
  wants a triage recommendation (use triage), or wants a comment draft (use draft).
```

---

### 3.3 `decisions` — 意思決定ログ

```yaml
name: decisions
description: >
  Extract and summarize decision logs from a Backlog issue's timeline: who decided what,
  when, and why — by analyzing comments and status change history.
  TRIGGER when: user says "意思決定", "decision log", "決定履歴", "なぜこうなったか",
  "経緯を教えて", "決定の背景", "decision history", "どうして変更された",
  "意思決定ログ", "何が決まった", "承認経緯", "設計の経緯",
  "why was this decided", "変更の理由", "この課題の歴史", "議論の経緯",
  "ステータス変更の理由", "誰が決めたか", "合意形成の過程",
  "過去の決定を振り返りたい", "この仕様になった理由".
  DO NOT TRIGGER when: user wants current issue status (use context)
  or wants to analyze project-wide activity patterns (use intelligence).
  Complements: Use after /logvalet:context to dive deeper into the decision history.
```

---

### 3.4 `digest-periodic` — 定期ダイジェスト

```yaml
name: digest-periodic
description: >
  Generate a weekly or daily digest of Backlog project activity: completed issues,
  newly started work, blocked items, and active issue counts — deterministic, no LLM needed.
  TRIGGER when: user says "週次ダイジェスト", "日次ダイジェスト", "今週の進捗",
  "weekly digest", "daily digest", "今週のまとめ", "進捗まとめ",
  "先週の振り返り", "日報ネタ", "週報ネタ", "今日の進捗",
  "プロジェクトの今週の動き", "weekly summary", "daily summary",
  "weekly progress", "今週完了したもの", "今日のアクティビティ",
  "定期レポート", "periodic digest", "進捗サマリー".
  DO NOT TRIGGER when: user wants a full activity report with user/team breakdown
  (use report) or wants anomaly detection (use intelligence).
  Complements: Combine with /logvalet:report for a comprehensive periodic review.
```

---

### 3.5 `draft` — コメント下書き

```yaml
name: draft
description: >
  Draft a context-aware Backlog issue comment: progress update, inquiry, resolution notice,
  or escalation — the LLM generates a draft based on the issue's full context and comment history.
  TRIGGER when: user says "コメント下書き", "draft comment", "コメントを書いて",
  "返信を作って", "コメントを作成", "comment draft", "コメント草稿",
  "コメントを下書きして", "issue comment を書いて", "バックログにコメント",
  "進捗報告コメント", "確認依頼コメント", "解決通知コメント",
  "コメントで報告したい", "返信案を作って", "エスカレーションコメント",
  "write a comment", "reply to this issue", "コメントの文面を考えて".
  DO NOT TRIGGER when: user wants to create a new issue (use issue-create)
  or wants to update issue fields like status/assignee (use logvalet CLI directly).
  Workflow: Automatically fetches context via /logvalet:context before drafting.
```

---

### 3.6 `health` — プロジェクト健全性

```yaml
name: health
description: >
  Check the health of a Backlog project: stale issues, blockers, user workload imbalance,
  and an overall health score (0-100) with level (healthy/warning/critical).
  Use this skill whenever someone asks about the state of a project, team performance,
  or whether a project is on track.
  TRIGGER when: user says "プロジェクトの状態", "project health", "プロジェクト健全性",
  "プロジェクト大丈夫", "このプロジェクトどう", "停滞してる課題ある？",
  "ブロッカーは？", "プロジェクトの健全性", "health check",
  "チームの負荷状況", "ワークロード", "workload", "プロジェクト診断",
  "project status", "is the project on track", "プロジェクトのスコア",
  "開発の進み具合", "課題の滞留状況", "プロジェクト概況".
  DO NOT TRIGGER when: user wants a detailed risk assessment with recommendations
  (use risk) or wants activity trend analysis (use intelligence).
  Complements: Follow up with /logvalet:risk for actionable recommendations.
```

---

### 3.7 `intelligence` — アクティビティインテリジェンス

```yaml
name: intelligence
description: >
  Analyze Backlog activity patterns: detect anomalies, concentration bias, peak hours,
  and unusual trends — the LLM interprets activity statistics and project health data
  to surface risks that numbers alone don't reveal.
  TRIGGER when: user says "アクティビティ分析", "activity intelligence", "異常検知",
  "偏り", "アクティビティの傾向", "activity patterns", "最近の動きに異常は",
  "チームの活動パターン", "作業の偏り", "ピーク時間帯", "activity anomaly",
  "特定の人に集中してない？", "活動量の分析", "誰が何をしてるか",
  "concentration analysis", "activity trends", "稼働分析".
  DO NOT TRIGGER when: user wants a simple activity list (use logvalet CLI directly),
  wants a periodic digest (use digest-periodic), or wants risk recommendations (use risk).
  Workflow: Combines activity stats + project health for holistic analysis.
```

---

### 3.8 `issue-create` — 課題作成

```yaml
name: issue-create
description: >
  Create a Backlog issue interactively: gather project, type, summary, description, priority,
  assignee, and other fields via questions, preview with dry-run, then submit.
  Use this skill whenever the user wants to create, register, file, or add a new Backlog issue,
  ticket, task, or bug report.
  TRIGGER when: user says "課題作成", "issue作成", "チケット作成", "タスク登録",
  "backlogに課題を作って", "バックログに登録", "新しいissue", "create issue",
  "file a ticket", "make a task", "backlog.com に課題追加",
  "課題を作りたい", "issueを立てたい", "チケットを切って", "タスクを作って",
  "バグ報告を登録", "新規課題", "register an issue", "open a ticket",
  "add a task to backlog", "課題追加", "バックログに追加".
  DO NOT TRIGGER when: user wants to update an existing issue (use logvalet CLI directly)
  or wants to bulk-create issues from a spec (use spec-to-issues).
```

---

### 3.9 `my-next` — 直近タスク

```yaml
name: my-next
description: >
  Show near-term Backlog issues assigned to me: next few business days across all projects,
  including overdue items — helps answer "what should I work on next?"
  TRIGGER when: user says "直近のタスク", "upcoming tasks", "次にやること", "次やること",
  "明日以降のタスク", "backlogの直近", "coming up", "what's next",
  "明日何やる", "次の予定", "今日と明日のタスク", "直近の課題",
  "next tasks", "upcoming issues", "what should I do next",
  "明日の予定", "次のアクション", "今日やること", "today's tasks",
  "直近やるべきこと", "tomorrow's tasks", "近日中のタスク".
  DO NOT TRIGGER when: user wants a full week overview (use my-week)
  or wants a project-wide task list (use logvalet CLI directly).
```

---

### 3.10 `my-week` — 週次タスク

```yaml
name: my-week
description: >
  Show this week's Backlog issues assigned to me across all projects,
  including overdue items from previous weeks — the weekly planning view.
  TRIGGER when: user says "今週のタスク", "my week", "今週やること", "今週の予定",
  "backlogの今週分", "weekly tasks", "this week's issues",
  "今週やるべきこと", "今週何やる", "今週の課題",
  "今週のバックログ", "weekly plan", "week overview",
  "今週の計画", "月曜から金曜のタスク", "今週のスケジュール",
  "weekly planning", "what's on my plate this week", "今週の見通し".
  DO NOT TRIGGER when: user wants only the next 1-2 days (use my-next)
  or wants team-wide workload (use health).
```

---

### 3.11 `report` — 活動レポート

```yaml
name: report
description: >
  Generate a Backlog activity report for specified users, teams, or projects over a time period:
  monthly report, weekly report, team activity summary, KPT retrospective material, and more.
  Use this skill whenever someone needs a shareable report or summary of Backlog activity.
  TRIGGER when: user says "レポート作成", "月次レポート", "活動レポート", "チームレポート",
  "backlogレポート", "activity report", "monthly report", "team report",
  "先月のレポート", "今月のレポート", "メンバーの活動",
  "チームの活動まとめ", "KPTレポート", "ふりかえりレポート",
  "活動報告", "月報", "週報", "進捗報告", "稼働報告",
  "generate a report", "activity summary", "team performance report",
  "期間レポート", "backlog 月報", "チームの成果", "実績報告".
  DO NOT TRIGGER when: user wants a quick daily/weekly digest without user breakdown
  (use digest-periodic) or wants project health metrics (use health).
```

---

### 3.12 `risk` — リスク評価

```yaml
name: risk
description: >
  Generate an integrated risk assessment for a Backlog project: combining project health,
  blockers, stale issues, and workload data — the LLM produces risk ratings, root cause analysis,
  and prioritized recommended actions.
  TRIGGER when: user says "リスク評価", "risk summary", "プロジェクトリスク", "リスク分析",
  "リスクは何", "対策は", "risk assessment", "リスクを洗い出して",
  "プロジェクトの懸念事項", "危険な兆候", "リスクマネジメント",
  "what are the risks", "risk analysis", "プロジェクトの課題を特定",
  "改善すべき点", "問題点の洗い出し", "risk mitigation",
  "推奨アクション", "次にやるべき対策", "プロジェクトの危機管理".
  DO NOT TRIGGER when: user wants a quick health score only (use health)
  or wants activity pattern analysis without risk interpretation (use intelligence).
  Workflow: Automatically gathers health + blockers + stale data before analysis.
```

---

### 3.13 `spec-to-issues` — 仕様→課題分解

```yaml
name: spec-to-issues
description: >
  Decompose a specification document into Backlog issues: the LLM analyzes a spec/requirements
  file, breaks it into appropriately-scoped tasks, and creates them in Backlog one by one
  with dry-run preview before each submission.
  TRIGGER when: user says "specから課題作成", "仕様から課題を作って", "spec to issues",
  "仕様書を課題に分解", "要件を課題にして", "spec 分解", "仕様分解",
  "仕様書からチケットを切って", "spec からタスクを作成", "要件定義を課題に",
  "仕様をバックログに登録", "spec を issue に変換",
  "設計書から課題を起こして", "PRD から課題を作成", "タスク分解",
  "break down spec into issues", "create issues from requirements",
  "一括課題作成", "bulk issue creation from spec".
  DO NOT TRIGGER when: user wants to create a single issue manually (use issue-create)
  or wants to write a spec first (use an external spec tool).
```

---

### 3.14 `triage` — 課題トリアージ

```yaml
name: triage
description: >
  Triage a Backlog issue: the LLM analyzes triage materials (issue attributes, history,
  project statistics, similar issues) and suggests priority, assignee, and category assignments.
  Use this skill whenever someone needs help deciding how to prioritize or assign an issue.
  TRIGGER when: user says "トリアージ", "triage", "優先度を決めて", "アサイン提案",
  "課題を振り分けて", "誰に担当させる", "priority 提案", "担当者を決めて",
  "課題のカテゴリ分類", "issue triage", "優先度の見直し", "アサインの提案",
  "この課題の優先度は", "誰にアサインすべき", "振り分けて",
  "prioritize this issue", "who should handle this", "assign this task",
  "課題の優先順位", "タスクの振り分け", "課題を整理して".
  DO NOT TRIGGER when: user wants full issue context without recommendations (use context)
  or wants to create a new issue (use issue-create).
  Workflow: Automatically fetches triage-materials before analysis.
```

---

## 4. Root スキル本文の構造

`skills/logvalet/SKILL.md` を PM メタモデルとして全面書き換え（セクション 3.1 の本文参照）。
現在の 26KB CLI リファレンスを削除し、スキル一覧 + ワークフロー例 + CLI 基本情報に置き換え。

---

## 5. 修正対象ファイル一覧

| ファイル | 操作 |
|---------|------|
| `.claude-plugin/marketplace.json` | 新規作成 |
| `.claude-plugin/plugin.json` | 新規作成 |
| `skills/logvalet/SKILL.md` | PM メタモデルに全面書き換え |
| `skills/logvalet-context/` → `skills/context/` | git mv + frontmatter 更新 |
| `skills/logvalet-decisions/` → `skills/decisions/` | 同上 |
| `skills/logvalet-digest-periodic/` → `skills/digest-periodic/` | 同上 |
| `skills/logvalet-draft/` → `skills/draft/` | 同上 |
| `skills/logvalet-health/` → `skills/health/` | 同上 |
| `skills/logvalet-intelligence/` → `skills/intelligence/` | 同上 |
| `skills/logvalet-issue-create/` → `skills/issue-create/` | 同上 |
| `skills/logvalet-my-next/` → `skills/my-next/` | 同上 |
| `skills/logvalet-my-week/` → `skills/my-week/` | 同上 |
| `skills/logvalet-report/` → `skills/report/` | 同上 |
| `skills/logvalet-risk/` → `skills/risk/` | 同上 |
| `skills/logvalet-spec-to-issues/` → `skills/spec-to-issues/` | 同上 |
| `skills/logvalet-triage/` → `skills/triage/` | 同上 |
| `README.md` | スキル参照を `logvalet:xxx` 形式に更新 |
| `README.ja.md` | 同上 |

---

## 6. 検証

1. `.claude-plugin/marketplace.json` と `plugin.json` が有効な JSON
2. 全 SKILL.md の frontmatter が有効な YAML（name, description 必須）
3. `git mv` でリネームが正しく追跡されている
4. README のスキル参照が新命名に統一されている
5. description のトリガーワード網羅性チェック:
   - 「Backlog」「バックログ」「backlog.com」
   - 「課題」「issue」「チケット」「タスク」「ticket」「task」
   - 「プロジェクト」「project」
   - 各スキル固有の日本語・英語キーワード
