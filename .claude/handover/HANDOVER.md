# HANDOVER.md

> 生成日時: 2026-03-13 (M10 実装完了)
> プロジェクト: logvaret
> ブランチ: main

## 今回やったこと（M10 Activity & user commands）

### internal/digest/activity.go — 新規作成

**ActivityDigestBuilder（spec §13.3）:**
- `ActivityDigestOptions` — Project/Since/Until/Limit
- `ActivityDigestBuilder` interface — `Build(ctx, opt) (*domain.DigestEnvelope, error)`
- `DefaultActivityDigestBuilder` — 標準実装
- `ActivityDigest` — scope / activities / comments / projects / summary / llm_hints
- `ActivityScope` — scope 情報（project/since/until/limit）
- `ActivityProjectSummary` — プロジェクト別集計（activity_count / comment_count）
- `ActivityDigestSummary` — 決定論的サマリー

**Build() ロジック:**
1. `opt.Project` が指定されていれば `ListProjectActivities` を使用
2. `opt.Project` なしは `ListSpaceActivities` を使用
3. アクティビティ取得失敗は partial success（warning 付き空ダイジェスト）
4. Backlog type 番号 → 文字列名変換（issue_created, issue_commented 等）
5. type=3（issue_commented）のコメントを抽出
6. issue key からプロジェクトキーを抽出してグルーピング

### internal/digest/activity_test.go — 新規作成（TDD）

5 テストケース:
- `TestActivityDigestBuilder_Build_space_activities` — スペース全体アクティビティ
- `TestActivityDigestBuilder_Build_project_activities` — プロジェクトフィルタ
- `TestActivityDigestBuilder_Build_activities_fetch_failed` — 取得失敗（partial success）
- `TestActivityDigestBuilder_Build_empty_activities` — 空アクティビティ
- `TestActivityDigestBuilder_Build_with_comments` — コメント抽出

### internal/digest/user.go — 新規作成

**UserDigestBuilder（spec §13.4）:**
- `UserDigestOptions` — Since/Until/Limit/Project
- `UserDigestBuilder` interface — `Build(ctx, userID string, opt) (*domain.DigestEnvelope, error)`
- `DefaultUserDigestBuilder` — 標準実装
- `UserDigest` — user / scope / activities / comments / projects / summary / llm_hints
- `DigestUser` — id / name
- `UserScope` — since / until
- `UserDigestSummary` — 決定論的サマリー（headline / total_activity / comment_count / types）

**Build() ロジック:**
1. `GetUser(ctx, userID)` — 必須（失敗時 error）
2. `ListUserActivities(ctx, userID, opt)` — オプション（失敗は warning）
3. activities 正規化・コメント抽出・プロジェクト別集計
4. DigestEnvelope 組み立て（Resource: "user"）

### internal/digest/user_test.go — 新規作成（TDD）

5 テストケース:
- `TestUserDigestBuilder_Build_success` — 正常系
- `TestUserDigestBuilder_Build_user_not_found` — GetUser 失敗（必須エラー）
- `TestUserDigestBuilder_Build_activities_fetch_failed` — 取得失敗（partial success）
- `TestUserDigestBuilder_Build_empty_activities` — 空アクティビティ
- `TestUserDigestBuilder_Build_with_comments` — コメント抽出

### internal/cli/activity.go — 更新

**変更点（spec §14.12-14.13 準拠）:**
- `ActivityListCmd` に `--project`, `--since` フラグを追加（`ListFlags` に変更）
- `ActivityDigestCmd` に `--project` フラグを追加（`DigestFlags` から `Since/Until/Limit` を継承）

### internal/cli/user.go — 更新

**変更点（spec §14.14-14.17 準拠）:**
- `UserListCmd` の `ProjectKey` フラグを削除（spec では user list はフィルタなし）
- `UserGetCmd.UserID` を `int` → `string`（Backlog API は userId string を要求）
- `UserActivityCmd` に `--project`, `--type` フラグを追加
- `UserDigestCmd` に `--project` フラグを追加

## 決定事項

- **アクティビティ取得は partial success**: ActivityDigest においてアクティビティ取得失敗は warning として処理し、空のダイジェストを返す。UserDigest でも同様。
- **UserGetCmd.UserID は string 型**: Backlog API の GetUser は `userID string` を受け付けるため string に統一。
- **DigestFlags から Since/Until/Limit を継承**: ActivityDigestCmd・UserActivityCmd・UserDigestCmd は DigestFlags を埋め込み、重複フィールドを避ける。
- **activityTypeName ヘルパー関数**: Backlog type 番号（1-21）→ 文字列名のマッピングを activity.go に実装。

## 検証結果

- `go test ./...` — 全テストパス（10パッケージ）
- `go build ./cmd/lv/` — ビルド成功
- `go vet ./...` — クリーン
- コミット: `8160121`

## 次にやること（優先度順）

- [ ] M11 Team & space commands
  - `team list` (spec §14.27)
  - `team project` (spec §14.28)
  - `TeamDigestBuilder` (spec §13.6)
  - `team digest` (spec §14.29)
  - `space info` (spec §14.30)
  - `space disk-usage` (spec §14.31)
  - `SpaceDigestBuilder` (spec §13.7)
  - `space digest` (spec §14.32)
  - 詳細計画: plans/logvalet-m11-team-space.md を生成してから実装開始
- [ ] CLI Run() に BacklogClient 統合（credential/config 完成後）
  - 全コマンドの Run() を BacklogClient 呼び出しで実装

## 前回の完了内容（M09 Document commands）

### internal/digest/document.go — 新規作成

**DocumentDigestBuilder（spec §13.5）:**
- `DocumentDigestOptions` — 将来拡張用プレースホルダー
- `DocumentDigestBuilder` interface — `Build(ctx, documentID int64, opt) (*domain.DigestEnvelope, error)`
- `DefaultDocumentDigestBuilder` — 標準実装
- `DocumentDigest` — digest フィールド（document / project / attachments / summary / llm_hints）
- `DigestDocumentDetail` — ドキュメント詳細（ID/ProjectID/Title/Content/Created/Updated/CreatedUser）
- `DocumentDigestSummary` — 決定論的サマリー（headline / attachment_count / has_content / content_length）

**Build() ロジック:**
1. `GetDocument(ctx, documentID)` — 必須（失敗時 error）
2. `ListProjects(ctx)` + ID マッチ — オプション（失敗は warning）
   - `Document.ProjectID` から `projectKey` が取れないため `ListProjects` 全件取得 + ID マッチ方式
3. `ListDocumentAttachments(ctx, documentID)` — オプション（失敗は warning）
4. `DigestEnvelope` 組み立て（Resource: "document"）

### internal/digest/document_test.go — 新規作成（TDD）

5 テストケース:
- `TestDocumentDigestBuilder_Build_success` — 正常系
- `TestDocumentDigestBuilder_Build_document_not_found` — GetDocument 失敗（必須）
- `TestDocumentDigestBuilder_Build_attachments_fetch_failed` — 添付取得失敗（partial success）
- `TestDocumentDigestBuilder_Build_project_fetch_failed` — プロジェクト一覧取得失敗（partial success）
- `TestDocumentDigestBuilder_Build_project_id_not_matched` — ID マッチ失敗（partial success）

### internal/cli/document.go — 更新

**変更点（spec §14.18-14.22 準拠）:**
- `DocumentGetCmd.NodeID string` → `DocumentGetCmd.DocumentID int64`（spec §14.18）
- `DocumentDigestCmd.NodeID string` → `DocumentDigestCmd.DocumentID int64`（spec §14.21）
- `DocumentCreateCmd` に `Content/ContentFile/ParentID` フラグを追加（spec §14.22）
- `DocumentCreateCmd.Run()` に `validateContentFlags` + dry-run 実装

### internal/cli/document_test.go — 新規作成（TDD）

5 テストケース:
- `TestDocumentCreateCmd_run_dry_run` — dry-run 正常系
- `TestDocumentCreateCmd_run_content_conflict` — --content と --content-file の排他エラー
- `TestDocumentCreateCmd_run_content_required` — どちらも未指定エラー
- `TestDocumentCreateCmd_run_content_file` — --content-file からの読み込み + dry-run
- `TestDocumentCreateCmd_run_not_dry_run` — dry-run なし → ErrNotImplemented

## 決定事項

- **projectKey 解決方法**: Document には `ProjectID int` しかなく `projectKey string` がない。`ListProjects` 全件取得 + ID マッチ方式を採用。失敗は warning（partial success）として処理。
- **CLI Run() は dry-run のみ実装**: M08 と同様に BacklogClient への実際の HTTP 呼び出し統合は行わない。credential/config システム完成後に統合予定。
- **DocumentGetCmd の arg 型変更**: `NodeID string` → `DocumentID int64` に変更。Kong は `int64` 型フィールドを arg として正常に処理する。

## 検証結果

- `go test ./...` — 全テストパス（9パッケージ、digest 5新規テスト + cli 5新規テスト含む）
- `go build ./cmd/lv/` — ビルド成功
- `go vet ./...` — クリーン
- コミット: `2775d09`

## 次にやること（優先度順）

- [ ] M10 Activity & user commands
  - `activity list` (spec §14.12)
  - `ActivityDigestBuilder` (spec §13.3)
  - `activity digest` (spec §14.13)
  - `user list / get` (spec §14.14-14.15)
  - `user activity` (spec §14.16)
  - `UserDigestBuilder` (spec §13.4)
  - `user digest` (spec §14.17)
  - 詳細計画: plans/logvalet-m10-activity-user.md を生成してから実装開始
- [ ] CLI Run() に BacklogClient 統合（credential/config 完成後）
  - 全コマンドの Run() を BacklogClient 呼び出しで実装
  - credentials.Resolve() → backlog.NewHTTPClient() → builder.Build() のフロー

## 関連ファイル

- `plans/logvalet-roadmap.md` — 12マイルストーンのロードマップ
- `plans/logvalet-m09-document.md` — M09: 詳細計画（完了）
- `docs/specs/logvalet_full_design_spec_with_architecture.md` — 完全な設計仕様書
- `internal/cli/document.go` — document コマンド定義（M09 更新）
- `internal/cli/document_test.go` — document CLI テスト（M09 新規）
- `internal/digest/document.go` — DocumentDigestBuilder（M09 新規）
- `internal/digest/document_test.go` — DocumentDigestBuilder テスト（M09 新規）
- `internal/cli/validate.go` — バリデーターヘルパー（M08 新規・M09 利用）
- `internal/digest/project.go` — ProjectDigestBuilder（M07 新規）
- `internal/digest/issue.go` — IssueDigestBuilder（M06 新規）
- `internal/domain/domain.go` — domain 型（M05 完全実装）
- `internal/backlog/client.go` — Client interface (28メソッド)
- `internal/backlog/mock_client.go` — MockClient (テスト用)
