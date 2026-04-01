# M45: Phase 3 ドキュメント最終整備

## 概要

Phase 3（M40-M44）で追加されたコマンド・スキルをドキュメントに反映し、全体の整合性を確保する。

## Phase 3 追加内容（M40-M44）

### 新コマンド

| コマンド | 説明 |
|---------|------|
| `logvalet issue timeline ISSUE_KEY` | コメント + 更新履歴を時系列構造化 |
| `logvalet activity stats` | アクティビティ統計・パターン分析 |

### 新 MCP ツール

| ツール | 説明 |
|--------|------|
| `logvalet_issue_timeline` | コメント・履歴時系列 |
| `logvalet_activity_stats` | アクティビティ統計 |

### 新スキル（M44）

| スキル | 材料源 | LLM 判断 |
|--------|--------|---------|
| `logvalet-decisions` | `issue timeline` | 意思決定ログの抽出・要約 |
| `logvalet-intelligence` | `activity stats` + `project health` | 偏り・異常の解釈・リスク評価 |
| `logvalet-risk` | `project health` + `project blockers` + `issue stale` | 統合リスク評価・推奨アクション |

## 更新対象ファイル

### 1. README.md（英語）

- AI Intelligence Commands (Phase 3) セクションを追加
  - `issue timeline`, `activity stats` コマンドを記載
  - 使用例を追加
- MCP サーバーセクションに `logvalet_issue_timeline`, `logvalet_activity_stats` を追加
- Skills セクションに `logvalet-decisions`, `logvalet-intelligence`, `logvalet-risk` を追加

### 2. README.ja.md（日本語）

- README.md と同内容の日本語版更新

### 3. 既存スキルの確認・更新

- `logvalet` スキル: `issue timeline` と `activity stats` コマンドが既に追加済み（M44 で追加）
- `logvalet-my-week` スキル: Phase 3 との連携 Optional セクションを追加
- `logvalet-my-next` スキル: Phase 3 との連携 Optional セクションを追加
- `logvalet-report` スキル: Phase 3 との連携 Optional セクションを追加

### 4. ロードマップ更新

- `plans/logvalet-roadmap-v3.md` の Phase 3 完了条件チェックボックスを更新

## 実装方針

- M38（Phase 2 ドキュメント整備）のパターンに忠実に従う
- 既存のフォーマット・スタイルを踏襲する
- コマンド名・フラグ名は実装（logvalet スキル）に合わせる
- README は簡潔に（詳細はスキルファイルに委ねる）
