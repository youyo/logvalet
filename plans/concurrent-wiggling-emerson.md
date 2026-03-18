# Roadmap v3: バグ修正 & 未実装機能の完成

## Context

M01-M16 完了後、全コマンドを Backlog API 実機テストしたところ、
API エンドポイント・パラメータの不整合、ドメインモデルの型不一致、未実装機能を発見。
リリース前にこれらを修正し、全コマンドが正常動作する状態にする。

## バグ・未実装一覧

| # | コマンド | 症状 | 根本原因 |
|---|---------|------|---------|
| B1 | `issue list --project-key` | `error.unknownParameter: projectKey[]` | Backlog API は `projectId[]`（数値）のみ。`projectKey[]` は存在しない |
| B2 | `document list` | `Undefined resource: /api/v2/projects/{key}/documents` | 正しくは `GET /api/v2/documents?projectId[]={id}&offset=N` |
| B3 | `document tree` | `Undefined resource: /api/v2/projects/{key}/documents/tree` | 正しくは `GET /api/v2/documents/tree?projectIdOrKey={key}` |
| B4 | `document get/digest/create` | Document ID が `int64` | 実際は `string`（UUID: `019b02404a9a7c90...`） |
| B5 | `document create` | `projectKey` パラメータ | API は `projectId`（数値）が必須 |
| B6 | domain.Document | フィールド不足 | `plain`, `json`, `statusId`, `emoji`, `tags`, `updatedUser` が欠落 |
| B7 | domain.DocumentNode | ツリー構造が不一致 | API は `{projectId, activeTree, trashTree}` を返す |
| F1 | `auth whoami` | `"user": null` | GetMyself API 呼び出しが未実装（auth.go:301） |

## API 実機検証結果

```
GET /api/v2/documents?projectId[]=533082&offset=0&count=2  → OK (2 docs)
GET /api/v2/documents/tree?projectIdOrKey=HEP_ISSUES       → OK ({projectId, activeTree, trashTree})
GET /api/v2/issues?projectId[]=533082&count=2               → OK (2 issues)

Document レスポンスフィールド:
  id(string/UUID), projectId, title, json, plain, statusId, emoji,
  attachments, tags, createdUser, created, updatedUser, updated

Document Tree レスポンス:
  {projectId: int, activeTree: {id, name, children: [{id, name, children, emoji}]}, trashTree: {...}}
```

---

## M17: issue list の projectKey → projectId 変換

**対象**: B1

**修正内容**:
1. `IssueListCmd.Run` (issue.go:48) で `--project-key` の値を `rc.Client.GetProject(ctx, key)` で解決し、`project.ID` を取得
2. `ListIssuesOptions.ProjectKey string` → `ProjectIDs []int` に変更
3. `HTTPClient.ListIssues` (http_client.go:264) で `q.Add("projectId[]", strconv.Itoa(id))` に変更
4. 複数プロジェクト対応: CLI の `--project-key` は `[]string` なので全て変換

**対象ファイル**:
- `internal/backlog/options.go:8` — ProjectKey → ProjectIDs
- `internal/backlog/http_client.go:261-264` — クエリパラメータ修正
- `internal/cli/issue.go:48-64` — Run で key→id 変換
- テストファイル

---

## M18: document コマンドの API 修正

**対象**: B2, B3, B4, B5, B6, B7

### 18-1: domain.Document の修正 (domain/domain.go:104-126)

```go
// 変更前
type Document struct {
    ID          int64      `json:"id"`          // → string (UUID)
    ProjectID   int        `json:"projectId"`
    Title       string     `json:"title"`
    Content     string     `json:"content,omitempty"`
    Created     *time.Time `json:"created,omitempty"`
    Updated     *time.Time `json:"updated,omitempty"`
    CreatedUser *User      `json:"createdUser,omitempty"`
}

// 変更後
type Document struct {
    ID          string     `json:"id"`           // UUID 文字列
    ProjectID   int        `json:"projectId"`
    Title       string     `json:"title"`
    Plain       string     `json:"plain,omitempty"`   // 追加
    JSON        string     `json:"json,omitempty"`    // 追加
    StatusID    int        `json:"statusId,omitempty"` // 追加
    Emoji       string     `json:"emoji,omitempty"`   // 追加
    Attachments []Attachment `json:"attachments,omitempty"` // 追加
    Tags        []Tag      `json:"tags,omitempty"`    // 追加
    CreatedUser *User      `json:"createdUser,omitempty"`
    Created     *time.Time `json:"created,omitempty"`
    UpdatedUser *User      `json:"updatedUser,omitempty"` // 追加
    Updated     *time.Time `json:"updated,omitempty"`
}

type Tag struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}
```

`Content` フィールドは削除し、`Plain` と `JSON` に分離（API レスポンスに合わせる）。

### 18-2: domain.DocumentNode の修正 (domain/domain.go:114-119)

```go
// 変更前
type DocumentNode struct {
    ID       int64          `json:"id"`
    Title    string         `json:"title"`
    Children []DocumentNode `json:"children,omitempty"`
}

// 変更後
type DocumentTree struct {
    ProjectID  int            `json:"projectId"`
    ActiveTree DocumentNode   `json:"activeTree"`
    TrashTree  DocumentNode   `json:"trashTree"`
}

type DocumentNode struct {
    ID       string         `json:"id"`          // string に変更
    Name     string         `json:"name"`         // Title → Name
    Emoji    string         `json:"emoji,omitempty"` // 追加
    Children []DocumentNode `json:"children,omitempty"`
}
```

### 18-3: Client interface の修正 (backlog/client.go:85-105)

| メソッド | 変更 |
|---------|------|
| `GetDocument(ctx, documentID int64)` | `documentID` を `string` に |
| `ListDocuments(ctx, projectKey string, opt)` | `projectKey` → `projectID int` + `opt` に `offset` 必須化 |
| `GetDocumentTree(ctx, projectKey string)` | 戻り型を `*domain.DocumentTree` に |
| `CreateDocument(ctx, req)` | `req.ProjectKey` → `req.ProjectID int` |
| `ListDocumentAttachments(ctx, documentID int64)` | `documentID` を `string` に |

### 18-4: HTTPClient の修正 (backlog/http_client.go:481-563)

| メソッド | パス変更 |
|---------|---------|
| `GetDocument` | `/api/v2/documents/%d` → `/api/v2/documents/{id}` (string) |
| `ListDocuments` | `/api/v2/projects/{key}/documents` → `/api/v2/documents?projectId[]={id}&offset=N` |
| `GetDocumentTree` | `/api/v2/projects/{key}/documents/tree` → `/api/v2/documents/tree?projectIdOrKey={key}` |
| `CreateDocument` | パス OK。パラメータ `projectKey` → `projectId` |
| `ListDocumentAttachments` | `/api/v2/documents/%d/attachments` → string ID |

### 18-5: CLI の修正 (cli/document.go)

- `DocumentGetCmd.DocumentID`: `int64` → `string`
- `DocumentDigestCmd.DocumentID`: `int64` → `string`
- `DocumentListCmd`: `ProjectKey` で GetProject して `ProjectID` を取得し、ListDocuments に渡す
- `DocumentCreateCmd`: 同様に key→id 変換。`ParentID *int64` → `*string`

### 18-6: DocumentDigestBuilder の修正

- `Build(ctx, documentID int64, ...)` → `string`
- 内部で使用している `ListDocumentAttachments` の引数も string に

**対象ファイル**:
- `internal/domain/domain.go` — Document, DocumentNode, DocumentTree, Tag
- `internal/backlog/client.go` — interface
- `internal/backlog/http_client.go` — 5メソッド全て
- `internal/backlog/options.go` — ListDocumentsOptions, CreateDocumentRequest
- `internal/cli/document.go` — 全コマンド
- `internal/digest/document_digest.go` — DocumentDigestBuilder
- テストファイル全般

---

## M19: auth whoami に GetMyself API 呼び出しを追加

**対象**: F1

**修正内容**:
1. `AuthWhoamiCmd.Run` (auth.go:247) で `buildRunContext(g)` を呼び、`rc.Client.GetMyself(ctx)` でユーザー取得
2. API 呼び出し失敗時は `user: null` でフォールバック（オフライン時も認証情報は表示）
3. `authWhoamiResponse.User` を `interface{}` → `*domain.User` に変更
4. `Space` フィールドも `resolved.Space` から設定

**対象ファイル**:
- `internal/cli/auth.go:247-307` — Run / RunWithStoreCapture
- `internal/cli/auth_test.go` — テスト追加

---

## 実行順序

```
M17 (issue list) → M18 (document API) → M19 (auth whoami)
```

M17: 影響小、即修正可能
M18: 最大変更（ドメインモデル + interface + 5 API メソッド + CLI + digest + テスト）
M19: 小変更

## 検証方法

各マイルストーン完了後:
1. `go test ./...` 全テスト通過
2. 実機コマンド実行:
   - `./logvalet issue list --project-key HEP_ISSUES` → issues 返る
   - `./logvalet document list --project-key HEP_ISSUES` → documents 返る
   - `./logvalet document tree --project-key HEP_ISSUES` → tree 返る
   - `./logvalet document get <uuid>` → document 返る
   - `./logvalet auth whoami` → `user` フィールドが埋まる
3. `go vet ./...` 通過
