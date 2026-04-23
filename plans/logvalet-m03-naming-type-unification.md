# M03: 命名・型統一（破壊的変更） 実装詳細計画

## 概要

MCP ツールパラメータ名と CLI フラグ名の命名揺れを解消する。全変更は 0.x バージョン帯であるため破壊的変更を許容し、後方互換ブリッジは設けない。

## 変更項目

### C1: `limit` → `count` 統一（MCP パラメータ名のみ）

**対象ツール**: `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`, `logvalet_shared_file_list`

**変更内容**:
- MCP ツール定義の `WithNumber("limit", ...)` → `WithNumber("count", ...)`
- ハンドラー内の `intArg(args, "limit")` → `intArg(args, "count")`
- Go 構造体フィールド名（`ListIssuesOptions.Limit` 等）は**変更しない**（MCP パラメータ名とは独立）

**後方互換方針**: `limit` パラメータはスキーマから削除。`count` のみ受け付ける。CHANGELOG に明示。

**注意**: `issue_list` には `count` パラメータ追加のみ（`limit` 削除）。ToolRegistry の unknown param handling はフレームワーク任せ（MCP-Go が unknown param を無視するため、`limit` を渡してもエラーにはならず単に無視される）。

### C2: `watching_list.user_id` を string 型に統一

**対象**: `logvalet_watching_list` の `user_id` パラメータ（`watching_count` は**対象外**、既知の非対称性として記録）

**変更内容**:
- `WithNumber("user_id", ...)` → `WithString("user_id", ...)`
- パターン説明に `"me or numeric ID (e.g. 12345)"` を追記
- ハンドラー内で `intArg` → `stringArg` に変更
- `"me"` の場合 `GetMyself` で解決、数値文字列の場合 `strconv.Atoi` で変換
- 数値型（`float64`）を受け取った場合は reject（JSON スキーマレベルで type mismatch）

**既知の非対称性**: `watching_count` は引き続き `user_id` を `Number` 型で受け付ける。この非対称性は M04 で解消するか、`watching_count` の別 PR で対処する。

### C3: `document_list.project_id` → `project_key`

**対象**: `logvalet_document_list`

**変更内容**:
- `WithNumber("project_id", ...)` → `WithString("project_key", ...)`
- ハンドラー内で `intArg(args, "project_id")` → `stringArg(args, "project_key")`
- `GetProject(ctx, projectKey)` で `projectID` を解決し `ListDocuments(ctx, projectID, opt)` を呼び出す

**影響**:
- `document_create` の `project_id` は**対象外**（今回の変更範囲外）
- `document_create` の `project_id` は今後の独立 PR で対処

### C4: CLI `star add --pr-id` → `--pull-request-id` (alias 維持)

**対象**: `internal/cli/star.go`

**変更内容**:
```go
// 変更前
PrID *int `help:"pull request ID to star" name:"pr-id"`

// 変更後
PrID *int `help:"pull request ID to star" name:"pull-request-id" aliases:"pr-id"`
```

**後方互換**: `--pr-id` は alias として引き続き動作。

## TDD 設計

### Red フェーズ（先にテストを書く）

#### C1 テスト（`tools_issue_test.go` に追加）

```
TestIssueListCount_Normal: count=50 でフィルタされることを確認
TestIssueCommentListCount_Normal: count=10 でフィルタされることを確認
```

#### C1 テスト（`tools_document_test.go` に追加）

```
TestDocumentListCount_Normal: count=5 でフィルタされることを確認
TestDocumentListProjectKey_Normal: project_key="PROJ" で GetProject + ListDocuments
```

#### C1 テスト（`tools_shared_file_test.go` に追加）

```
TestSharedFileListCount_Normal: count=10 でフィルタされることを確認
```

#### C2 テスト（`tools_test.go` 既存テスト更新 + 新テスト追加）

```
TestWatchingListHandler: user_id を "123" (string) に変更
TestWatchingListHandler_MissingUserID: そのまま
TestWatchingListHandler_UserIDMe: user_id="me" で GetMyself 経由
TestWatchingListHandler_UserIDNumeric: user_id="12345" で直接解決
TestWatchingListHandler_UserIDInvalid: user_id="abc" でエラー
```

#### C4 テスト（`internal/cli/` に追加）

```
TestStarAddPullRequestIDFlag: --pull-request-id=100 で正常処理
TestStarAddPrIDAlias: --pr-id=100 で alias として動作
```

### Green フェーズ（実装）

各テストを通す最小限の実装変更を実施。

### Refactor フェーズ

テストが全 green になった後、重複コードの整理を行う。

## 影響範囲

| ファイル | 変更種別 |
|---------|---------|
| `internal/mcp/tools_issue.go` | C1: `issue_list` の `limit` → `count`、`issue_comment_list` の `limit` → `count` |
| `internal/mcp/tools_document.go` | C1: `document_list` の `limit` → `count`、C3: `project_id` → `project_key` |
| `internal/mcp/tools_shared_file.go` | C1: `shared_file_list` の `limit` → `count` |
| `internal/mcp/tools_watching.go` | C2: `watching_list.user_id` の型変更 |
| `internal/cli/star.go` | C4: `--pr-id` → `--pull-request-id` + alias |
| `internal/mcp/tools_issue_test.go` | C1 テスト追加 |
| `internal/mcp/tools_document_test.go` | C1/C3 テスト追加 |
| `internal/mcp/tools_shared_file_test.go` | C1 テスト追加 |
| `internal/mcp/tools_test.go` | C2 テスト更新・追加 |
| `CHANGELOG.md` | 破壊的変更の記載 |

## CHANGELOG 記載内容

```markdown
## [Unreleased]

### Changed (BREAKING)
- feat(mcp): `issue_list`, `issue_comment_list`, `document_list`, `shared_file_list` のページネーション
  パラメータ `limit` を `count` に改名（MCP 統一命名規則への準拠）
- feat(mcp): `document_list` の `project_id`（数値型）を `project_key`（文字列型）に変更
  （CLI との整合性確保）
- feat(mcp): `watching_list` の `user_id` を数値型から文字列型に変更
  （`"me"` または `"12345"` 形式で指定）

### Changed
- feat(cli): `star add --pr-id` フラグを `--pull-request-id` に改名（`--pr-id` は alias として維持）
```

## 実装手順

1. **Red**: テストファイルに失敗するテストを追記する
2. `go test ./...` でテストが失敗することを確認（Red）
3. **Green**: tools_issue.go, tools_document.go, tools_shared_file.go, tools_watching.go, star.go を変更
4. `go test ./...` で全テスト green を確認
5. `go vet ./...` で警告ゼロを確認
6. CHANGELOG.md を更新
7. git commit（Conventional Commits 形式、日本語、`Plan:` フッター付き）

## 既知の制約

- `watching_count` の `user_id` は今回変更しない（非対称性あり、M04 または独立 PR で解消）
- `document_create` の `project_id` は今回変更しない（スコープ外）
- `issue_list` に既存の `limit` フィールドが `backlog.ListIssuesOptions` に残るが、MCP パラメータ名のみ `count` に統一（Go 構造体は変更しない）

## 参照

- 親プラン: `plans/playful-dreaming-peacock.md` M03 セクション
- Kong alias 構文: `aliases:"pr-id"` タグ（Kong v1.14.0 対応）
