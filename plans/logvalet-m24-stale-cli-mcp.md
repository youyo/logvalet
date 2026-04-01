# M24: Stale Issues CLI + MCP

## 目標

M23 で実装済みの `StaleIssueDetector.Detect()` を CLI コマンドと MCP ツールから呼び出せるようにする。

## 前提（M23 ハンドオフ）

- `internal/analysis/stale.go` — `StaleIssueDetector`, `StaleConfig`, `Detect(ctx, projectKeys, config)` 実装済み
- `StaleConfig.DefaultDays` — 日数閾値（0以下は DefaultStaleDays=7 にフォールバック）
- `StaleConfig.ExcludeStatus` — 除外ステータス名スライス
- ページネーション未対応（100件上限）— 将来対応

## コマンド仕様

```
logvalet issue stale -k PROJECT [--days 14] [--exclude-status "完了,対応済み"]
```

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|-----------|------|
| `-k` / `--project-key` | `[]string` | (必須) | プロジェクトキー |
| `--days` | `int` | 7 | 停滞判定の閾値日数 |
| `--exclude-status` | `string` | `""` | 除外ステータス（カンマ区切り） |

## MCP ツール仕様

```
logvalet_issue_stale
  project_keys: string[] (required) — プロジェクトキー配列
  days: number (optional) — 停滞閾値日数
  exclude_status: string (optional) — 除外ステータス（カンマ区切り）
```

## 実装ファイル

| # | ファイル | 内容 |
|---|---------|------|
| 1 | `internal/cli/issue_stale.go` | `IssueStaleCmd` 構造体 + `Run()` メソッド |
| 2 | `internal/cli/issue_stale_test.go` | Kong Parse テスト（4ケース） |
| 3 | `internal/cli/issue.go` | `IssueCmd` に `Stale IssueStaleCmd` フィールド追加 |
| 4 | `internal/mcp/tools_analysis.go` | `logvalet_issue_stale` ツール登録 |
| 5 | `internal/mcp/server_test.go` | expectedCount は不要（現状カウントテストなし） |

## TDD 設計

### Red Phase — テストを先に書く

#### T1: `issue stale -k PROJ` のパースとデフォルト値
```go
// Parse: issue stale -k PROJ
// 検証: ProjectKey=["PROJ"], Days=7, ExcludeStatus=""
```

#### T2: フラグ付きパース
```go
// Parse: issue stale -k PROJ --days 14 --exclude-status "完了,対応済み"
// 検証: ProjectKey=["PROJ"], Days=14, ExcludeStatus="完了,対応済み"
```

#### T3: 複数プロジェクトキー
```go
// Parse: issue stale -k PROJ1 -k PROJ2
// 検証: ProjectKey=["PROJ1","PROJ2"]
```

#### T4: `-k` なしでエラー
```go
// Parse: issue stale
// 検証: エラーが返される
```

### Green Phase — 最小限の実装

1. `IssueStaleCmd` 構造体を定義（Kong タグ付き）
2. `Run(g *GlobalFlags) error` を実装:
   - `buildRunContext(g)` で RunContext 取得
   - `--exclude-status` をカンマ分割して `StaleConfig.ExcludeStatus` に設定
   - `StaleIssueDetector.Detect()` 呼び出し
   - `rc.Renderer.Render()` で出力
3. `IssueCmd` に `Stale` フィールド追加
4. `tools_analysis.go` に `logvalet_issue_stale` 登録

### Refactor Phase

- issue_context.go パターンとの一貫性確認
- 不要なコード削除

## 実装手順

### Step 1: テストファイル作成
`internal/cli/issue_stale_test.go` — T1〜T4

### Step 2: IssueStaleCmd 実装
`internal/cli/issue_stale.go` — 構造体 + Run()

### Step 3: IssueCmd 更新
`internal/cli/issue.go` — `Stale IssueStaleCmd` フィールド追加

### Step 4: テスト実行・Green 確認
`go test ./internal/cli/...`

### Step 5: MCP ツール追加
`internal/mcp/tools_analysis.go` — `logvalet_issue_stale` 登録

### Step 6: 全テスト実行
`go test ./... && go vet ./...`

## 既存パターン準拠

- CLI: `issue_context.go` パターン（`buildRunContext` → `analysis.New*` → `Render`）
- MCP: `tools_analysis.go` パターン（`RegisterAnalysisTools` 内で `r.Register`）
- テスト: `issue_context_test.go` パターン（Kong Parse テスト）

## 完了条件

- [ ] `logvalet issue stale -k PROJECT` が停滞課題を JSON 出力
- [ ] `--days`, `--exclude-status` フラグが機能
- [ ] MCP `logvalet_issue_stale` ツールが登録・動作
- [ ] Kong Parse テスト 4ケースがパス
- [ ] `go test ./...` がパス
- [ ] `go vet ./...` がパス
