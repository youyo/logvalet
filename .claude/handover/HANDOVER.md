# HANDOVER.md

> 生成日時: 2026-03-13 (M04 実装完了)
> プロジェクト: logvaret
> ブランチ: main

## 今回やったこと（M04 Backlog API client core）

- `internal/domain/domain.go` — domain 型を M04 で先行定義
  - `User`, `Issue`, `Comment`, `Project`, `Activity`, `Document`, `DocumentNode`, `Attachment`
  - `Status`, `Category`, `Version`, `CustomFieldDefinition`, `CustomField`
  - `Team`, `Space`, `DiskUsage`, `IDName`
  - JSON タグは Backlog API レスポンス（camelCase）に準拠
- `internal/backlog/` パッケージを TDD (Red→Green→Refactor) で実装
  - **client.go**: `Client interface` — 28 メソッド (spec §18.1 全メソッド)
  - **errors.go**: `ErrNotFound`, `ErrUnauthorized`, `ErrForbidden`, `ErrRateLimited`, `ErrValidation`, `ErrAPI` + `BacklogError{Err, Code, Message, StatusCode}` + `ExitCodeFor()` — exit code マッピング (spec §18.4)
  - **options.go**: `ListIssuesOptions`, `ListCommentsOptions`, `ListActivitiesOptions`, `ListUserActivitiesOptions`, `ListDocumentsOptions` (spec §18.2)
  - **types.go**: `CreateIssueRequest`, `UpdateIssueRequest`, `AddCommentRequest`, `UpdateCommentRequest`, `CreateDocumentRequest` + `Pagination`, `RateLimitInfo` (spec §18.3)
  - **http_client.go**: `HTTPClient` 実装
    - OAuth 認証: `Authorization: Bearer <access_token>` ヘッダ
    - API key 認証: `?apiKey=<key>` クエリパラメータ（Backlog 公式方式）
    - HTTP エラー正規化: 404→ErrNotFound, 401→ErrUnauthorized, 403→ErrForbidden, 429→ErrRateLimited, 5xx→ErrAPI
    - context 対応: `http.NewRequestWithContext` 使用
  - **mock_client.go**: `MockClient` — 28 メソッド分の Func フィールド + `sync.Mutex` 保護の `CallCounts`
- コミット: 8c5d0fd

## 決定事項

- **Backlog API key 認証方式**: `?apiKey=<key>` クエリパラメータが Backlog 公式方式。OAuth は `Authorization: Bearer` ヘッダ。
- **domain 型の先行定義**: `internal/domain/domain.go` に M04 で最小限を定義。M05 で拡張予定。
- **JSON タグ**: domain 型の JSON タグは Backlog API レスポンス（camelCase: `issueKey`, `userId` 等）に合わせる。spec §9 の `snake_case` は digest 出力レイヤー（render）で変換する方針（M05 以降）。
- **ErrValidation の ExitCode**: `ExitArgumentError` (2) にマッピング（バリデーションエラーは引数エラーと同等）。
- **MockClient のデフォルト動作**: Func が未セットの場合 `ErrNotFound` を返す（安全側のデフォルト）。

## 検証結果

- `go test ./...` — 全テストパス (8パッケージ)
- `go build ./cmd/lv/` — ビルド成功
- `go vet ./...` — クリーン
- テストケース数 (internal/backlog/):
  - errors: 11 (sentinel errors, BacklogError, ExitCodeFor)
  - http_client: 12 (OAuth/APIkey auth, error handling, GetIssue, ListIssues, context cancellation, GetSpace, GetProject, field parsing)
  - mock_client: 7 (GetMyself, GetIssue, ListIssues, CallCount thread safety, default ErrNotFound)
  - options: 9 (ListIssuesOptions, ListCommentsOptions, ListActivitiesOptions, ListUserActivitiesOptions, ListDocumentsOptions)
  - types: 8 (CreateIssueRequest, UpdateIssueRequest, AddCommentRequest, UpdateCommentRequest, CreateDocumentRequest, Pagination, RateLimitInfo)

## 捨てた選択肢と理由

- `http.Client` ラッパーの RoundTripper パターン: M04 では newRequest/do ヘルパーで十分。M06 以降で必要なら追加。
- RateLimitInfo のヘッダパース実装: Backlog API が `X-Ratelimit-*` を返すか未確認。型定義のみ（M06 以降で統合時に追加）。
- CreateIssue の form body 実装: M04 では query param で暫定実装。M08（Issue write）で form body に整備予定。

## 次にやること（優先度順）

- [ ] M05 Domain models & full rendering
  - `internal/domain/` の拡張（spec §11-12 に完全準拠）
  - Warning / error envelope types (spec §9)
  - Renderer interface (spec §20)
  - JSON renderer (pretty-print 対応)
  - YAML renderer
  - Markdown renderer
  - Text renderer
  - 詳細計画: plans/logvalet-m05-domain-render.md を生成してから実装開始
- [ ] auth login の Run() に OAuth フロー統合（M04 以降 → M05 か M06 前に統合）
- [ ] auth whoami の Run() に Backlog API /api/v2/myself 呼び出し追加（HTTPClient 統合）

## 関連ファイル

- `plans/logvalet-roadmap.md` — 12マイルストーンのロードマップ
- `plans/logvalet-m04-api-client.md` — M04: 詳細計画（完了）
- `docs/specs/logvalet_full_design_spec_with_architecture.md` — 完全な設計仕様書
- `internal/domain/domain.go` — domain 型 (M04 先行定義)
- `internal/backlog/client.go` — Client interface (28メソッド)
- `internal/backlog/errors.go` — typed errors + ExitCodeFor
- `internal/backlog/options.go` — request option types
- `internal/backlog/types.go` — write request types + Pagination, RateLimitInfo
- `internal/backlog/http_client.go` — HTTPClient 実装
- `internal/backlog/mock_client.go` — MockClient (テスト用)
- `internal/credentials/credentials.go` — ResolvedCredential (M03)
- `internal/app/exitcode.go` — Exit code 定数
