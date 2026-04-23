---
milestone: M02
title: 未実装 MCP ツール 14 種の新規追加
status: planned
created: 2026-04-23
parent_plan: plans/playful-dreaming-peacock.md
---

# M02 実装詳細計画: 未実装 MCP ツール 14 種の新規追加

## 目的

`plans/playful-dreaming-peacock.md` § M02 に定義された 14 種の MCP ツールを TDD で実装する。
既存の `internal/digest/` Builder、`internal/backlog/` Client を最大限再利用し、
新規ロジックを最小化する。

---

## 追加ツール一覧と実装方針

### B1: `logvalet_user_me`

**配置**: `internal/mcp/tools_user.go`

**ハンドラ方針**:
```go
client.GetMyself(ctx)
```

**パラメータ**: なし  
**アノテーション**: `readOnlyAnnotation("認証ユーザー情報取得")`

**TDD テスト** (`tools_user_test.go` に追記):
- B1-N1: 正常系 — GetMyself が呼ばれ user が返る
- B1-E1: 異常系 — GetMyself がエラー → IsError=true

---

### B2: `logvalet_user_activity`

**配置**: `internal/mcp/tools_user.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| user_id | string | req | ユーザーID |
| since | string | - | 開始日 (YYYY-MM-DD) |
| until | string | - | 終了日 (YYYY-MM-DD) |
| limit | number | - | 取得上限（デフォルト 20） |
| project | string | - | プロジェクトキーでフィルタ（クライアント側）|
| activity_type_ids | string | - | CSV形式のアクティビティタイプID |

**ハンドラ方針**:
- `ListUserActivitiesOptions{Count, ActivityTypeIDs, Order}` を使用
- `since/until` は `parseDateStr()` でパース後、CLI と同様にクライアント側フィルタ（`ListUserActivitiesOptions` に since/until フィールドなし）
- `project` もクライアント側フィルタ（API非対応）
- `user_id="me"` → `GetMyself` で解決（既存 `tools_activity.go` と同パターン）

**アノテーション**: `readOnlyAnnotation("ユーザーアクティビティ取得")`

**TDD テスト**:
- B2-N1: user_id="12345", limit=10 → ListUserActivities(Count=10) 呼び出し
- B2-N2: user_id="me" → GetMyself → ListUserActivities
- B2-N3: since/until 指定 → クライアント側フィルタが適用される
- B2-E1: user_id 未指定 → IsError=true

---

### B3: `logvalet_digest_unified`

**配置**: `internal/mcp/tools_analysis.go`（`RegisterAnalysisTools` 内に追加）

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| since | string | req | 期間開始 (YYYY-MM-DD) |
| until | string | - | 期間終了 (YYYY-MM-DD) |
| project_keys | string | - | CSV形式のプロジェクトキー |
| user_ids | string | - | CSV形式のユーザーID |
| team_ids | string | - | CSV形式のチームID |
| issue_keys | string | - | CSV形式の課題キー |
| due_date | string | - | 期限日フィルタ (YYYY-MM-DD or YYYY-MM-DD:YYYY-MM-DD) |
| start_date | string | - | 開始日フィルタ |

**ハンドラ方針**:
- `digest.NewUnifiedDigestBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)`
- `digest.UnifiedDigestScope{ProjectKeys, ProjectIDs, UserIDs, TeamIDs, IssueKeys, Since, Until, DueDateSince, DueDateUntil, StartDateSince, StartDateUntil}`
- `since` のパースは `parseDateStr()` 使用（YYYY-MM-DD のみサポート。keyword は非サポート）
- `project_keys` 指定時は `GetProject` で ProjectIDs を解決（`tools_issue.go` の既存パターン参照）

**アノテーション**: `readOnlyAnnotation("統合ダイジェスト生成")`

**TDD テスト** (`tools_analysis_test.go` に追記):
- B3-N1: since="2026-04-01", project_keys="PROJ" → UnifiedDigestBuilder.Build 呼び出し確認
- B3-N2: user_ids="1,2", team_ids="5" → UserIDs/TeamIDs が設定される
- B3-E1: since 未指定 → IsError=true

---

### B4: `logvalet_activity_digest`

**配置**: `internal/mcp/tools_activity.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| since | string | - | 開始日 (YYYY-MM-DD) |
| until | string | - | 終了日 (YYYY-MM-DD) |
| limit | number | - | 取得上限（デフォルト 20） |
| project | string | - | プロジェクトキーでフィルタ |

**ハンドラ方針**:
- `digest.NewDefaultActivityDigestBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)`
- `digest.ActivityDigestOptions{Project, Since, Until, Limit}`
- `RegisterActivityTools` は `cfg ServerConfig` を受け取るようにシグネチャを変更する（現状は `r *ToolRegistry` のみ）

**シグネチャ変更**:
```go
// 変更前
func RegisterActivityTools(r *ToolRegistry) {
// 変更後
func RegisterActivityTools(r *ToolRegistry, cfg ServerConfig) {
```
`server.go` の `registerAllTools` でも対応修正。

**アノテーション**: `readOnlyAnnotation("アクティビティダイジェスト生成")`

**TDD テスト**:
- B4-N1: パラメータなし → DefaultActivityDigestBuilder.Build 呼び出し
- B4-N2: project="PROJ", limit=50 → ActivityDigestOptions に反映
- B4-N3: since/until 指定 → Since/Until がセットされる

---

### B5: `logvalet_document_tree`

**配置**: `internal/mcp/tools_document.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| project_key | string | req | プロジェクトキー |

**ハンドラ方針**:
```go
client.GetDocumentTree(ctx, projectKey)
```

**アノテーション**: `readOnlyAnnotation("ドキュメントツリー取得")`

**TDD テスト**:
- B5-N1: project_key="PROJ" → GetDocumentTree 呼び出し確認
- B5-E1: project_key 未指定 → IsError=true

---

### B6: `logvalet_document_digest`

**配置**: `internal/mcp/tools_document.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| document_id | string | req | ドキュメントID |

**ハンドラ方針**:
- `digest.NewDefaultDocumentDigestBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)`
- `builder.Build(ctx, documentID, digest.DocumentDigestOptions{})`
- `RegisterDocumentTools` は `cfg ServerConfig` を受け取るようにシグネチャを変更する

**シグネチャ変更**:
```go
// 変更前
func RegisterDocumentTools(r *ToolRegistry) {
// 変更後
func RegisterDocumentTools(r *ToolRegistry, cfg ServerConfig) {
```

**アノテーション**: `readOnlyAnnotation("ドキュメントダイジェスト生成")`

**TDD テスト**:
- B6-N1: document_id="doc-123" → DefaultDocumentDigestBuilder.Build 呼び出し
- B6-E1: document_id 未指定 → IsError=true

---

### B7: `logvalet_space_digest`

**配置**: `internal/mcp/tools_space.go`

**パラメータ**: なし（`SpaceDigestOptions{}` は空）

**ハンドラ方針**:
- `digest.NewDefaultSpaceDigestBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)`
- `builder.Build(ctx, digest.SpaceDigestOptions{})`
- `RegisterSpaceTools` は `cfg ServerConfig` を受け取るようにシグネチャを変更する

**シグネチャ変更**:
```go
// 変更前
func RegisterSpaceTools(r *ToolRegistry) {
// 変更後
func RegisterSpaceTools(r *ToolRegistry, cfg ServerConfig) {
```

**アノテーション**: `readOnlyAnnotation("スペースダイジェスト生成")`

**TDD テスト**:
- B7-N1: パラメータなし → DefaultSpaceDigestBuilder.Build 呼び出し

---

### B8: `logvalet_space_disk_usage`

**配置**: `internal/mcp/tools_space.go`

**パラメータ**: なし

**ハンドラ方針**:
```go
client.GetSpaceDiskUsage(ctx)
```

**アノテーション**: `readOnlyAnnotation("スペースディスク使用量取得")`

**TDD テスト**:
- B8-N1: パラメータなし → GetSpaceDiskUsage 呼び出し確認
- B8-E1: GetSpaceDiskUsage がエラー → IsError=true

---

### B9: `logvalet_meta_version`

**配置**: `internal/mcp/tools_meta.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| project_key | string | req | プロジェクトキー |

**ハンドラ方針**:
```go
client.ListProjectVersions(ctx, projectKey)
```

**アノテーション**: `readOnlyAnnotation("バージョン一覧取得")`

**TDD テスト**:
- B9-N1: project_key="PROJ" → ListProjectVersions 呼び出し確認
- B9-E1: project_key 未指定 → IsError=true

---

### B10: `logvalet_meta_custom_field`

**配置**: `internal/mcp/tools_meta.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| project_key | string | req | プロジェクトキー |

**ハンドラ方針**:
```go
client.ListProjectCustomFields(ctx, projectKey)
```

**アノテーション**: `readOnlyAnnotation("カスタムフィールド一覧取得")`

**TDD テスト**:
- B10-N1: project_key="PROJ" → ListProjectCustomFields 呼び出し確認
- B10-E1: project_key 未指定 → IsError=true

---

### B11: `logvalet_team_project`

**配置**: `internal/mcp/tools_team.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| project_key | string | req | プロジェクトキー |

**ハンドラ方針**:
```go
client.ListProjectTeams(ctx, projectKey)
```

**アノテーション**: `readOnlyAnnotation("プロジェクトチーム一覧取得")`

**TDD テスト**:
- B11-N1: project_key="PROJ" → ListProjectTeams 呼び出し確認
- B11-E1: project_key 未指定 → IsError=true

---

### B12: `logvalet_issue_attachment_delete`

**配置**: `internal/mcp/tools_issue.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| issue_key | string | req | 課題キー |
| attachment_id | number | req | 添付ファイルID |

**ハンドラ方針**:
```go
client.DeleteIssueAttachment(ctx, issueKey, int64(attachmentID))
```

**アノテーション**: `destructiveAnnotation("添付ファイル削除")`

**TDD テスト**:
- B12-N1: issue_key="PROJ-1", attachment_id=99 → DeleteIssueAttachment 呼び出し確認
- B12-E1: issue_key 未指定 → IsError=true
- B12-E2: attachment_id 未指定 → IsError=true
- B12-E3: DeleteIssueAttachment がエラー → IsError=true

---

### B13: `logvalet_issue_attachment_download`

**配置**: `internal/mcp/tools_issue.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| issue_key | string | req | 課題キー |
| attachment_id | number | req | 添付ファイルID |

**ハンドラ方針**:
- `client.DownloadIssueAttachmentBounded(ctx, issueKey, int64(attachmentID), 20*1024*1024)` で `([]byte, filename, contentType, error)` 取得
  - Bounded メソッドは `internal/backlog/` Client 層でサイズ検査を実施（`ErrDownloadTooLarge` を返す）
- `base64.StdEncoding.EncodeToString(content)` でエンコード
- レスポンス: `map[string]any{"content_base64": ..., "filename": ..., "content_type": ..., "size_bytes": ...}`

**サイズ検査の設計原則**:
- 親プラン (playful-dreaming-peacock.md §M02 設計判断) 準拠:
  「サイズ上限 20MB を `internal/backlog/client.go` のダウンロードメソッド内（transport 層）で検査し、超過時は早期 error。MCP ハンドラ層では検査せず Client 層に単一責任化」
- MCP ハンドラは Bounded メソッドを呼ぶだけ。サイズロジックを持たない

**アノテーション**: `readOnlyAnnotation("添付ファイルダウンロード")`

**TDD テスト**:
- B13-N1: 正常系 → base64 encoded content + filename + content_type + size_bytes が返る
- B13-E1: DownloadIssueAttachmentBounded が `ErrDownloadTooLarge` → IsError=true
- B13-E2: issue_key 未指定 → IsError=true
- B13-E3: attachment_id 未指定 → IsError=true

---

### B14: `logvalet_shared_file_download`

**配置**: `internal/mcp/tools_shared_file.go`

**パラメータ**:
| 名前 | 型 | 必須 | 説明 |
|------|-----|------|------|
| project_key | string | req | プロジェクトキー |
| file_id | number | req | 共有ファイルID |

**ハンドラ方針**:
- `client.DownloadSharedFileBounded(ctx, projectKey, int64(fileID), 20*1024*1024)` で `([]byte, filename, contentType, error)` 取得
  - Bounded メソッドは Client 層でサイズ検査を実施（B13 と同様の設計原則）
- `base64.StdEncoding.EncodeToString(content)` でエンコード
- レスポンス: `map[string]any{"content_base64": ..., "filename": ..., "content_type": ..., "size_bytes": ...}`

**アノテーション**: `readOnlyAnnotation("共有ファイルダウンロード")`

**TDD テスト**:
- B14-N1: 正常系 → base64 encoded content が返る
- B14-E1: DownloadSharedFileBounded が `ErrDownloadTooLarge` → IsError=true
- B14-E2: project_key 未指定 → IsError=true
- B14-E3: file_id 未指定 → IsError=true

---

## シグネチャ変更が必要なファイル一覧

| ファイル | 変更内容 |
|---------|---------|
| `tools_activity.go` | `RegisterActivityTools(r, cfg ServerConfig)` に変更 |
| `tools_document.go` | `RegisterDocumentTools(r, cfg ServerConfig)` に変更 |
| `tools_space.go` | `RegisterSpaceTools(r, cfg ServerConfig)` に変更 |
| `server.go` | `registerAllTools` 内の3関数呼び出しに `cfg` を追加 |

---

## 実装ステップ（TDD: Red → Green → Refactor）

### Step 0a: Red — Client 層テストを先に書く

`internal/backlog/http_client_test.go` に追加（既存の `httptest.NewServer` + `newOAuthClient` パターンを踏襲）:
- `TestHTTPClientDownloadIssueAttachmentBounded_Normal`: 正常系 → `[]byte` + filename + contentType が返る
- `TestHTTPClientDownloadIssueAttachmentBounded_TooLarge`: 20MB+1 byte のレスポンス → `ErrDownloadTooLarge`
- `TestHTTPClientDownloadSharedFileBounded_Normal`: 正常系
- `TestHTTPClientDownloadSharedFileBounded_TooLarge`: 20MB+1 byte → `ErrDownloadTooLarge`

`go test ./internal/backlog/...` が RED（コンパイルエラー）であることを確認

### Step 0b: Green — Client 層実装

1. `internal/backlog/errors.go` に `ErrDownloadTooLarge` sentinel を追加
2. `internal/backlog/client.go` に以下を追加:
   ```go
   DownloadIssueAttachmentBounded(ctx context.Context, issueKey string, attachmentID int64, maxBytes int64) (content []byte, filename, contentType string, err error)
   DownloadSharedFileBounded(ctx context.Context, projectKey string, fileID int64, maxBytes int64) (content []byte, filename, contentType string, err error)
   ```
3. `internal/backlog/http_client.go` に Bounded メソッドを実装:
   - 既存 `doDownload` は `resp.Body` を返した後 `resp` への参照を持たないため、Content-Type を取得できない
   - 新規プライベートヘルパー `doDownloadBounded(req, maxBytes)` を追加:
     - `c.httpClient.Do(req)` でレスポンス取得
     - ステータスエラー処理（`doDownload` と同様）
     - `filename` は `filenameFromResponse(resp)` で取得
     - `contentType` は `resp.Header.Get("Content-Type")` で取得
     - `io.LimitReader(resp.Body, maxBytes+1)` + `io.ReadAll` で読み込み
     - `len(data) > maxBytes` なら `ErrDownloadTooLarge` を返す
     - 戻り値: `([]byte, filename, contentType string, error)`
   - `DownloadIssueAttachmentBounded` / `DownloadSharedFileBounded` は `doDownloadBounded` を呼ぶ
4. `internal/backlog/mock_client.go` に以下を追加（既存パターンに準拠）:
   - `DownloadIssueAttachmentBoundedFunc` フィールド
   - `DownloadSharedFileBoundedFunc` フィールド
   - 各メソッド本体（call count 記録 + Func 委譲、デフォルトはゼロ値返却）

### Step 0c: 確認

`go test ./internal/backlog/...` が GREEN であることを確認

### Step 1: Red — 全テストを先に書く

1. `tools_user_test.go` (新規) に B1, B2 のテストを作成
2. `tools_activity_test.go` に B4 のテストを追加
3. `tools_analysis_test.go` に B3 のテストを追加
4. `tools_document_test.go` (新規) に B5, B6 のテストを追加
5. `tools_space_test.go` (新規) に B7, B8 のテストを追加
6. `tools_meta_test.go` (新規) に B9, B10 のテストを追加
7. `tools_team_test.go` に B11 のテストを追加
8. `tools_issue_test.go` に B12, B13 のテストを追加
9. `tools_shared_file_test.go` (新規) に B14 のテストを追加
10. `go test ./internal/mcp/...` が RED (コンパイルエラー含む) であることを確認

### Step 2: Green — 最小限の実装

1. `tools_user.go` に B1, B2 を実装
2. `tools_activity.go` のシグネチャ変更 + B4 実装
3. `tools_analysis.go` に B3 を実装
4. `tools_document.go` のシグネチャ変更 + B5, B6 実装
5. `tools_space.go` のシグネチャ変更 + B7, B8 実装
6. `tools_meta.go` に B9, B10 を実装
7. `tools_team.go` に B11 を実装
8. `tools_issue.go` に B12, B13 を実装（Bounded メソッド使用）
9. `tools_shared_file.go` に B14 を実装（Bounded メソッド使用）
10. `server.go` の `registerAllTools` を更新
11. `go test ./...` が GREEN であることを確認

### Step 3: Refactor

- B13, B14 で base64 エンコードのパターンが一致しているか確認
- パラメータパース共通パターンが冗長でないか確認
- `go vet ./...` でエラーがないことを確認

---

## リスク評価と対策

| リスク | 重大度 | 対策 |
|--------|-------|------|
| `RegisterActivityTools` シグネチャ変更によるビルドエラー | 中 | `server.go` を同時修正 |
| `RegisterDocumentTools` / `RegisterSpaceTools` 同様 | 中 | 同上 |
| Bounded メソッドの `io.LimitReader` メモリ消費 (20MB) | 中 | Client 層で上限強制、超過時は `ErrDownloadTooLarge` で早期拒否 |
| `digest_unified` の `GetProject` 呼び出しが失敗する場合 | 低 | エラーを即座に返す（tools_issue.go と同パターン） |
| `logvalet_user_activity` の since/until が API 非対応 | 低 | クライアント側フィルタ（CLI と同方式）。計画に明記 |
| `DownloadIssueAttachmentBounded` / `DownloadSharedFileBounded` の mock 追加漏れ | 低 | mock_client.go に明示的に追加。コンパイルエラーで検出 |

---

## 変更ファイル一覧

**変更** (既存ファイル):
- `internal/backlog/errors.go` — `ErrDownloadTooLarge` sentinel 追加
- `internal/backlog/client.go` — `DownloadIssueAttachmentBounded` / `DownloadSharedFileBounded` インターフェース追加
- `internal/backlog/http_client.go` — Bounded メソッドの実装追加
- `internal/backlog/http_client_test.go` — Bounded メソッドのテスト追加
- `internal/backlog/mock_client.go` — `DownloadIssueAttachmentBoundedFunc` / `DownloadSharedFileBoundedFunc` 追加
- `internal/mcp/tools_user.go` — B1, B2 追加
- `internal/mcp/tools_activity.go` — シグネチャ変更 + B4 追加
- `internal/mcp/tools_analysis.go` — B3 追加
- `internal/mcp/tools_document.go` — シグネチャ変更 + B5, B6 追加
- `internal/mcp/tools_space.go` — シグネチャ変更 + B7, B8 追加
- `internal/mcp/tools_meta.go` — B9, B10 追加
- `internal/mcp/tools_team.go` — B11 追加
- `internal/mcp/tools_issue.go` — B12, B13 追加
- `internal/mcp/tools_shared_file.go` — B14 追加
- `internal/mcp/server.go` — `registerAllTools` 更新

**新規** (テストファイル):
- `internal/mcp/tools_user_test.go`
- `internal/mcp/tools_document_test.go`
- `internal/mcp/tools_space_test.go`
- `internal/mcp/tools_meta_test.go`
- `internal/mcp/tools_shared_file_test.go`

---

## 補足: content_type の取得方式

Bounded メソッドは `resp.Header.Get("Content-Type")` で Content-Type を返す。
`http.DetectContentType` による推定は不要。
`doDownload` のシグネチャは変更しない（最小影響）。
