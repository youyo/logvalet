# M41: CommentTimeline CLI + MCP

## 概要

M40 で実装した `CommentTimelineBuilder` を CLI コマンド `logvalet issue timeline` と MCP ツール `logvalet_issue_timeline` として公開する。

## 依存

- M40: `internal/analysis/timeline.go` の `CommentTimelineBuilder` が実装済み

## 実装対象ファイル

| ファイル | 変更種別 | 内容 |
|---------|---------|------|
| `internal/cli/issue_timeline.go` | 新規作成 | `IssueTimelineCmd` の定義と `Run()` |
| `internal/cli/issue_timeline_test.go` | 新規作成 | Kong パーステスト |
| `internal/cli/issue.go` | 修正 | `IssueCmd` に `Timeline` フィールド追加 |
| `internal/mcp/tools_analysis.go` | 修正 | `logvalet_issue_timeline` ツール追加 |
| `internal/mcp/tools_analysis_test.go` | 修正 | MCP ツール登録・実行テスト追加 |

## CLI 設計

### コマンド

```
logvalet issue timeline ISSUE_KEY [flags]
```

### フラグ

| フラグ | 型 | デフォルト | 説明 |
|-------|---|----------|------|
| `--max-comments` | int | 0 (全件) | 取得するコメントの最大数 |
| `--include-updates` | bool | true | 更新履歴を含める |
| `--max-activity-pages` | int | 5 | アクティビティのページネーション上限 |
| `--since` | string | "" | 取得開始日時 (YYYY-MM-DD) |
| `--until` | string | "" | 取得終了日時 (YYYY-MM-DD) |

### Kong struct

```go
type IssueTimelineCmd struct {
    IssueKey         string `arg:"" required:"" help:"issue key (e.g., PROJ-123)"`
    MaxComments      int    `help:"max number of comments to include (0 = all)" default:"0"`
    IncludeUpdates   bool   `help:"include update history events" default:"true" negatable:""`
    MaxActivityPages int    `help:"max pages for activity pagination" default:"5"`
    Since            string `help:"filter events since date (YYYY-MM-DD)"`
    Until            string `help:"filter events until date (YYYY-MM-DD)"`
}
```

## MCP ツール設計

### ツール名

`logvalet_issue_timeline`

### パラメータ

| パラメータ | 型 | 必須 | 説明 |
|-----------|---|------|------|
| `issue_key` | string | yes | Issue key (e.g. PROJ-123) |
| `max_comments` | number | no | Max comments (0 = all) |
| `include_updates` | boolean | no | Include update events (default true) |
| `max_activity_pages` | number | no | Max activity pages (default 5) |
| `since` | string | no | Start date YYYY-MM-DD |
| `until` | string | no | End date YYYY-MM-DD |

## TDD テスト設計

### CLI テスト (issue_timeline_test.go)

- T1: `issue timeline PROJ-123` のパースとデフォルト値確認
- T2: フラグ付きパース (`--max-comments 20 --no-include-updates --since 2026-01-01`)
- T3: 必須引数なしでエラー
- T4: `--help` が panic しない

### MCP テスト (tools_analysis_test.go への追記)

- T5: `logvalet_issue_timeline` が ToolRegistry に登録されていること
- T6: ハンドラーが正常に `AnalysisEnvelope` を返すこと

## 実装手順

1. Red: `issue_timeline_test.go` 作成 → `go test ./internal/cli/...` 失敗確認
2. Green:
   a. `issue_timeline.go` 実装
   b. `issue.go` に `Timeline IssueTimelineCmd` フィールド追加
3. MCP Red: `tools_analysis_test.go` にテスト追加 → 失敗確認
4. MCP Green: `tools_analysis.go` に `logvalet_issue_timeline` 追加
5. Refactor: `go test ./...` 全通過確認、`go vet ./...` 確認

## リスク評価

| リスク | 対策 |
|--------|------|
| `--include-updates` デフォルト true の negatable 対応 | Kong の `negatable:""` タグを使い `--no-include-updates` で false 指定可能に |
| `--since/--until` の日付パース | 既存の `parseDateStr` ヘルパー（tools_analysis.go 内）を参照して同様に実装 |
| IncludeUpdates が *bool 型の処理 | CLI では bool フラグを受け取り、*bool に変換して渡す |
