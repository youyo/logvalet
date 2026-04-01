---
title: マイルストーン M43 - ActivityStats CLI + MCP
project: logvalet
author: planning-agent
created: 2026-04-01
status: Draft
complexity: S
---

# マイルストーン M43: ActivityStats CLI + MCP

## 概要

M42 で実装した `ActivityStatsBuilder` を CLI コマンド `logvalet activity stats` と MCP ツール `logvalet_activity_stats` として公開する。

## スコープ

### 実装範囲

- `internal/cli/activity_stats.go` — `ActivityStatsCmd` 定義
- `internal/cli/activity_stats_test.go` — CLI テスト（Kong パース + MockClient）
- `internal/cli/activity.go` — `ActivityCmd` struct に `Stats` フィールド追加
- `internal/mcp/tools_analysis.go` — `logvalet_activity_stats` ツール追加
- `internal/mcp/tools_analysis_test.go` — MCP ツールテスト追加
- `internal/mcp/tools_test.go` — ツール総数更新（32 → 33）

### スコープ外

- `ActivityStatsBuilder` 本体（M42 で実装済み）
- LLM 判断ロジック（SKILL 側）

## コマンド設計

### `logvalet activity stats`

```
logvalet activity stats [flags]

フラグ:
  --scope string       アクティビティスコープ (project/user/space) [デフォルト: "space"]
  -k, --project-key    プロジェクトキー (scope=project 時)
  --user-id string     ユーザーID (scope=user 時)
  --since string       取得開始日時 (ISO 8601 形式)
  --until string       取得終了日時 (ISO 8601 形式)
  --top-n int          top_active_actors/types の表示件数 [デフォルト: 5]
```

### Kong struct 設計

```go
type ActivityStatsCmd struct {
    Scope      string `help:"activity scope (project/user/space)" default:"space" enum:"project,user,space"`
    ProjectKey string `help:"project key (required when scope=project)" short:"k"`
    UserID     string `help:"user ID (required when scope=user)"`
    Since      string `help:"start date/time (ISO 8601)"`
    Until      string `help:"end date/time (ISO 8601)"`
    TopN       int    `help:"number of top actors/types to include" default:"5"`
}
```

## MCP ツール設計

### `logvalet_activity_stats`

```
ツール名: logvalet_activity_stats
説明: Get activity statistics (by type, actor, date, hour, patterns)

パラメータ:
  scope        string  (project/user/space, default: space)
  project_key  string  (required when scope=project)
  user_id      string  (required when scope=user)
  since        string  (YYYY-MM-DD format)
  until        string  (YYYY-MM-DD format)
  top_n        number  (default: 5)
```

## TDD テスト設計

### CLI テスト (`internal/cli/activity_stats_test.go`)

| ID | テスト名 | 概要 |
|----|---------|------|
| T1 | TestActivityStats_KongParse_Default | デフォルト値の確認（scope=space, top-n=5） |
| T2 | TestActivityStats_KongParse_WithProjectScope | --scope project -k PROJ のパース確認 |
| T3 | TestActivityStats_KongParse_WithUserScope | --scope user --user-id uid のパース確認 |
| T4 | TestActivityStats_KongParse_WithAllFlags | 全フラグ指定パース確認 |
| T5 | TestActivityStats_Run_SpaceScope | scope=space 実行テスト（MockClient） |
| T6 | TestActivityStats_Run_ProjectScope | scope=project 実行テスト（MockClient） |
| T7 | TestActivityStats_Run_UserScope | scope=user 実行テスト（MockClient） |

### MCP テスト (`internal/mcp/tools_analysis_test.go`)

| ID | テスト名 | 概要 |
|----|---------|------|
| A1 | TestActivityStats_MCPTool_Registered | ツールが登録されていること |
| A2 | TestActivityStats_MCPTool_SpaceScope | scope=space で正常に AnalysisEnvelope を返す |
| A3 | TestActivityStats_MCPTool_ProjectScope | scope=project で正常に AnalysisEnvelope を返す |
| A4 | TestActivityStats_MCPTool_InvalidSince | 不正な since 日付でエラー |
| A5 | TestActivityStats_MCPTool_WithTopN | top_n パラメータが反映される |

## 実装手順

### Step 1: テストファイル作成（Red）

1. `internal/cli/activity_stats_test.go` 作成
2. `internal/mcp/tools_analysis_test.go` に A1〜A5 追加
3. `internal/mcp/tools_test.go` の expectedCount を 33 に更新
4. `go test ./...` が失敗することを確認

### Step 2: CLI 実装（Green）

1. `internal/cli/activity_stats.go` 作成
2. `internal/cli/activity.go` に `Stats ActivityStatsCmd` フィールド追加
3. `go test ./internal/cli/...` が通ることを確認

### Step 3: MCP 実装（Green）

1. `internal/mcp/tools_analysis.go` に `logvalet_activity_stats` ツール追加
2. `go test ./internal/mcp/...` が通ることを確認

### Step 4: Refactor & 全テスト確認

1. `go test ./...` 全通過確認
2. `go vet ./...` 確認

## アーキテクチャ

### 既存パターンとの整合性

| 項目 | パターン | M43 採用 |
|------|---------|---------|
| CLI コマンド | Kong struct + Run() メソッド | ActivityStatsCmd |
| サブコマンド登録 | 親 struct にフィールド追加 | ActivityCmd.Stats |
| MCP ツール | RegisterXxxTools に追加 | RegisterAnalysisTools に追加 |
| 日付パース | `time.Parse(time.RFC3339, ...)` | 同パターン |
| MCP 日付パース | `parseDateStr(s)` ヘルパー | 同パターン |

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| scope=project 時に project_key 未指定 | Warning | CLI は Run() 内でバリデーション、MCP はエラー返却 |
| by_hour の JSON キーが数値文字列 | Info | M42 で確認済み |
| MCP ツール総数の更新忘れ | Warning | tools_test.go の expectedCount を更新 |
