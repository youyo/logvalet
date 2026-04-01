# M46: Phase 3 E2E テスト + 最終リリース

## 概要

Phase 3 で追加されたコマンド（`issue timeline`, `activity stats`）の E2E テストを追加し、Phase 3 の完了を検証する。

## 対象コマンド

| コマンド | MCP ツール | 実装場所 |
|---------|-----------|---------|
| `logvalet issue timeline ISSUE_KEY` | `logvalet_issue_timeline` | `internal/analysis/timeline.go` |
| `logvalet activity stats --scope project/user/space` | `logvalet_activity_stats` | `internal/analysis/activity_stats.go` |

## E2E テスト設計

### テストファイル

- `internal/e2e/issue_timeline_e2e_test.go` — issue timeline E2E テスト
- `internal/e2e/activity_stats_e2e_test.go` — activity stats E2E テスト

### テスト方針

- `//go:build e2e` ビルドタグで分離（既存パターンに従う）
- 環境変数から Backlog 接続情報を取得（`loadE2EEnv` ヘルパーを利用）
- 読み取り専用操作のみ
- JSON 出力の基本構造を検証（値の完全一致ではなくスキーマ検証）

### 環境変数（既存）

| 変数名 | 説明 |
|-------|------|
| `LOGVALET_E2E_API_KEY` | Backlog API キー |
| `LOGVALET_E2E_SPACE` | スペース名（例: heptagon） |
| `LOGVALET_E2E_PROJECT_KEY` | プロジェクトキー |
| `LOGVALET_E2E_ISSUE_KEY` | 課題キー（省略可、デフォルト: PROJECT_KEY-1） |

## テストケース詳細

### issue_timeline_e2e_test.go

1. `TestE2E_IssueTimeline` — 基本的な timeline 取得
   - `CommentTimelineBuilder.Build()` を呼び出し
   - AnalysisEnvelope 基本検証（schema_version, resource, space, base_url, generated_at, analysis, warnings）
   - `*analysis.CommentTimeline` 型アサーション
   - `Events` スライスが nil でないこと
   - `Meta.TotalEvents` が `len(Events)` と一致すること
   - `IssueKey` が空でないこと

2. `TestE2E_IssueTimeline_NoUpdates` — --no-include-updates オプション
   - `IncludeUpdates: false` で Build()
   - 全イベントが `kind=comment` であること

### activity_stats_e2e_test.go

1. `TestE2E_ActivityStats_Project` — scope=project
   - `ActivityStatsBuilder.Build()` を project scope で呼び出し
   - AnalysisEnvelope 基本検証
   - `*analysis.ActivityStats` 型アサーション
   - `Scope == "project"`, `ScopeKey == env.ProjectKey` の検証
   - `TotalCount >= 0` の検証
   - `ByType`, `ByActor` が nil でないこと
   - `TopActiveActors`, `TopActiveTypes` が nil でないこと

2. `TestE2E_ActivityStats_Space` — scope=space
   - `ActivityStatsBuilder.Build()` を space scope で呼び出し
   - `Scope == "space"` の検証
   - `TotalCount >= 0` の検証

## 実装手順

1. `internal/e2e/issue_timeline_e2e_test.go` を作成（Red）
2. `internal/e2e/activity_stats_e2e_test.go` を作成（Red）
3. `go test ./...` で unit テスト全通過確認（E2E は e2e ビルドタグでスキップ）
4. `go vet ./...` 通過確認

## 完了条件

- [x] `internal/e2e/issue_timeline_e2e_test.go` 作成
- [x] `internal/e2e/activity_stats_e2e_test.go` 作成
- [x] `go test ./...` 全通過（unit テスト）
- [x] `go vet ./...` 通過
- [x] git commit（`feat(e2e): Phase 3 E2E テストを追加 (M46)`）
