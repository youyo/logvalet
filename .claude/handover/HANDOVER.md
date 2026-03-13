# HANDOVER.md

> 生成日時: 2026-03-13 (M08 実装完了)
> プロジェクト: logvaret
> ブランチ: main

## 今回やったこと（M08 Issue write operations）

### internal/cli/validate.go — 新規作成

**共通バリデーターヘルパー関数:**
- `validateDescriptionFlags(description, descriptionFile string) error` — `--description` と `--description-file` の排他チェック
- `validateContentFlags(content, contentFile string) error` — `--content` と `--content-file` の排他チェック（どちらか必須）
- `validateAtLeastOneUpdateFlag(summary, description, status, priority, assignee, dueDate, startDate, descriptionFile *string, slices ...[]string) error` — 更新フラグ存在チェック（variadic でスライスも対応）
- `readContentFromFile(path string) (string, error)` — ファイルからテキスト読み込み
- `formatDryRun(operation string, params map[string]interface{}) ([]byte, error)` — dry-run JSON 出力フォーマット

### internal/cli/issue.go — 更新

**追加/変更されたコマンド（spec §14.4-14.8 準拠）:**

- `IssueCreateCmd` — フラグ大幅拡充（IssueType/Description/DescriptionFile/Priority/Assignee/Category/Version/Milestone/DueDate/StartDate）、Run() にバリデーション + dry-run 実装
- `IssueUpdateCmd` — Summary/Description/DescriptionFile/Status/Priority/Assignee/Category/Version/Milestone/DueDate/StartDate をポインタ型で追加、Run() にバリデーション + dry-run 実装
- `IssueCommentListCmd` — ListFlags を embed 追加（--limit/--offset サポート）
- `IssueCommentAddCmd` — Content/ContentFile フラグ追加、Run() にバリデーション + dry-run + ファイル読み込み実装
- `IssueCommentUpdateCmd` — Content/ContentFile フラグ追加、Run() にバリデーション + dry-run + ファイル読み込み実装

**共通ヘルパー（issue.go 末尾）:**
- `nilIfEmpty(s string) interface{}` — dry-run 出力用
- `ptrOrNil(s *string) interface{}` — dry-run 出力用

### internal/cli/issue_write_test.go — 新規作成（TDD）

26 テストケース:
- `TestValidateDescriptionFlags_*` (4ケース) — 排他バリデーション
- `TestValidateContentFlags_*` (4ケース) — 排他バリデーション
- `TestValidateAtLeastOneUpdateFlag_*` (3ケース) — 更新フラグ存在チェック
- `TestReadContentFromFile_*` (2ケース) — ファイル読み込み
- `TestFormatDryRun_*` (1ケース) — dry-run 出力
- `TestIssueCreateCmd_run_*` (2ケース) — create コマンド動作
- `TestIssueUpdateCmd_run_*` (3ケース) — update コマンド動作
- `TestIssueCommentAddCmd_run_*` (4ケース) — comment add コマンド動作
- `TestIssueCommentUpdateCmd_run_*` (3ケース) — comment update コマンド動作

## 決定事項

- **CLI バリデーション層のみ実装**: M08 では BacklogClient への実際の HTTP 呼び出し統合は行わない。credential/config システム完成後に統合予定。
- **dry-run 出力スキーマ**: `{"dry_run": true, "operation": "...", "params": {...}}` 形式で stdout に出力（exit 0）。
- **validateAtLeastOneUpdateFlag を variadic に**: categories/versions/milestones などスライスフィールドも更新フラグとして扱えるよう `slices ...[]string` で対応。
- **IssueCommentListCmd に ListFlags を embed**: spec §14.6 に基づきページネーションオプションを追加。

## 検証結果

- `go test ./...` — 全テストパス (9パッケージ、cli パッケージ 26新規テスト含む)
- `go build ./cmd/lv/` — ビルド成功
- `go vet ./...` — クリーン
- コミット: `2ae450c`

## 次にやること（優先度順）

- [ ] M09 Document commands
  - `document get / list / tree` (spec §14.18-14.20)
  - `DocumentDigestBuilder` (spec §13.5)
  - `document digest` (spec §14.21)
  - `document create` (spec §14.22)
  - 詳細計画: plans/logvalet-m09-document.md を生成してから実装開始
- [ ] CLI Run() に BacklogClient 統合（credential/config 完成後）
  - issue get / list / digest / project get / list / digest コマンドの Run() を実装
  - credentials.Resolve() → backlog.NewHTTPClient() → builder.Build() のフロー

## 関連ファイル

- `plans/logvalet-roadmap.md` — 12マイルストーンのロードマップ
- `plans/logvalet-m08-issue-write.md` — M08: 詳細計画（完了）
- `docs/specs/logvalet_full_design_spec_with_architecture.md` — 完全な設計仕様書
- `internal/cli/issue.go` — issue コマンド定義（M08 更新）
- `internal/cli/validate.go` — バリデーターヘルパー（M08 新規）
- `internal/cli/issue_write_test.go` — テスト（M08 新規）
- `internal/digest/project.go` — ProjectDigestBuilder（M07 新規）
- `internal/digest/issue.go` — IssueDigestBuilder（M06 新規）
- `internal/domain/domain.go` — domain 型（M05 完全実装）
- `internal/backlog/client.go` — Client interface (28メソッド)
- `internal/backlog/mock_client.go` — MockClient (テスト用)
