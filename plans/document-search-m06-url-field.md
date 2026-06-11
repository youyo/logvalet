# M6: URL フィールド追加（DocumentSearchDetail.url）

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | M1・M2・M3（実装済み） |
| 対象ファイル | `internal/digest/document_search.go` / `internal/digest/document_search_test.go` / `internal/digest/testdata/document_search_snippet.json`（golden 再生成） / `internal/cli/document.go` / `internal/mcp/tools_document.go` |

## Goal

`DocumentSearchDetail` に `url` フィールドを追加し、各ドキュメントへの Backlog Web UI リンクを返す。

URL 形式: `https://heptagon.backlog.com/document/PROJECT_KEY/019eb2a36c0f7b22b0ebcd3f033013f9`
一般形: `{baseURL}/document/{projectKey}/{documentID}`

**なぜ `ListProjects` が必要か（AD9）**:
`domain.Document` は `ProjectID`（int）のみ持ち `ProjectKey`（string）を含まない。
cross-project 検索（`--project` 省略・デフォルト）では CLI/MCP 層でプロジェクトを解決しないため、
Options 経由でマップを渡す A案は空になる。
`DefaultDocumentDigestBuilder.Build` が `ListProjects` で同じ解決をしており precedent がある（`internal/digest/document.go:80-92`）。

**URL は verbosity 非依存（AD10）**: snippet/meta/full 全モードで返す。

## 設計

### DocumentSearchDetail への url 追加

```go
type DocumentSearchDetail struct {
    ID          string          `json:"id"`
    ProjectID   int             `json:"project_id"`
    Title       string          `json:"title"`
    URL         string          `json:"url,omitempty"`       // ← 追加（verbosity 非依存）
    Snippet     string          `json:"snippet,omitempty"`
    Plain       string          `json:"plain,omitempty"`
    Created     *time.Time      `json:"created,omitempty"`
    Updated     *time.Time      `json:"updated,omitempty"`
    CreatedUser *domain.UserRef `json:"created_user,omitempty"`
    UpdatedUser *domain.UserRef `json:"updated_user,omitempty"`
}
```

### Build シグネチャ変更（ctx 追加）

```go
// Before
Build(docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope

// After
Build(ctx context.Context, docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope
```

インターフェース定義も更新:
```go
type DocumentSearchBuilder interface {
    Build(ctx context.Context, docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope
}
```

### Build 実装（ListProjects で projectID → projectKey マッピング）

```go
func (b *DefaultDocumentSearchBuilder) Build(ctx context.Context, docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope {
    detail := opt.Detail
    if detail == "" {
        detail = "snippet"
    }

    baseURL := strings.TrimRight(b.baseURL, "/")

    // baseURL が空 or docs が空なら ListProjects を呼ばない（AD12）
    projectKeyMap := make(map[int]string)
    var warnings []domain.Warning
    if baseURL != "" && len(docs) > 0 {
        projects, err := b.client.ListProjects(ctx)
        if err != nil {
            warnings = append(warnings, domain.Warning{
                Code:      "project_fetch_failed",
                Message:   fmt.Sprintf("failed to list projects for URL construction: %v", err),
                Component: "url",
                Retryable: true,
            })
        } else {
            for _, p := range projects {
                projectKeyMap[p.ID] = p.ProjectKey
            }
        }
    }

    items := make([]DocumentSearchDetail, 0, len(docs))
    for _, doc := range docs {
        item := DocumentSearchDetail{
            ID:          doc.ID,
            ProjectID:   doc.ProjectID,
            Title:       doc.Title,
            Created:     doc.Created,
            Updated:     doc.Updated,
            CreatedUser: toUserRef(doc.CreatedUser),
            UpdatedUser: toUserRef(doc.UpdatedUser),
        }
        // URL は verbosity 非依存（projectKey 不在なら省略）
        if key, ok := projectKeyMap[doc.ProjectID]; ok && key != "" {
            item.URL = fmt.Sprintf("%s/document/%s/%s", baseURL, key, doc.ID)
        }
        switch detail {
        case "snippet":
            item.Snippet = extractSnippet(doc.Plain, opt.Keyword)
        case "full":
            item.Snippet = extractSnippet(doc.Plain, opt.Keyword)
            item.Plain = doc.Plain
        }
        items = append(items, item)
    }

    digestData := &DocumentSearchDigest{
        Keyword:       opt.Keyword,
        Detail:        detail,
        TotalReturned: len(docs),
        PossiblyMore:  len(docs) >= 100,
        Items:         items,
    }
    // nil → warnings（partial success、ListProjects 失敗時に warning が入る）
    return b.newEnvelope("document_search", digestData, warnings)
}
```

**注意**: `b.newEnvelope` の第3引数は `warnings []domain.Warning`。nil の代わりに warnings を渡すように変更する（M2 実装では `nil` を渡していた）。

### 呼び出し元の更新

#### `internal/cli/document.go`

```go
// Before
envelope := builder.Build(docs, digest.DocumentSearchOptions{...})

// After
envelope := builder.Build(ctx, docs, digest.DocumentSearchOptions{...})
```

#### `internal/mcp/tools_document.go`

```go
// Before
return builder.Build(docs, digest.DocumentSearchOptions{...}), nil

// After
return builder.Build(ctx, docs, digest.DocumentSearchOptions{...}), nil
```

## TDD テスト設計

### document_search_test.go の更新

1. **全既存テスト**: `Build(docs, opt)` → `Build(context.Background(), docs, opt)` に更新
2. **url 構築テスト**: MockClient.ListProjectsFunc を設定し、url が正しく組み立てられることを検証
   - projectKey = "PROJ" のとき: `url = "{baseURL}/document/PROJ/{docID}"`
   - url 構築は exact 文字列アサートを使う: `item.URL == "https://example.backlog.com/document/PROJ/{docID}"` 形式（golden スナップショットは代替にならない）
   - url 否定系（ListProjects 失敗 / projectKey 不在 / baseURL 空）は個別 builder で MockClient を override（共有ヘルパーは使わない）
3. **url 省略テスト**: ListProjects が失敗する場合 → url が空・warnings が1件
4. **projectKey 不在テスト**: マップに projectID がない場合 → url が空（エラーなし）
5. **baseURL 空テスト**: baseURL = "" のとき → url が空（エラーなし）
6. **verbosity 非依存**: detail="meta" でも url が設定されること

### 既存 MockClient（確認済み）

`mock_client.go:35` に `ListProjectsFunc` は**既存**。追加不要。

**重要 — テストヘルパーへの stub 追加（必須）**:
`newTestDocumentSearchBuilder()` ヘルパーに `ListProjectsFunc` を設定しないと、
`baseURL != ""` のとき `ListProjects` が呼ばれ未設定 mock がエラーを返す → warning 1件が追加され
B8（`len(Warnings)==0` 期待）が落ちる。golden の url フィールドも壊れる。

実装時に必ずヘルパーを以下のように修正すること:
```go
func newTestDocumentSearchBuilder() *DefaultDocumentSearchBuilder {
    mock := backlog.NewMockClient()
    mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
        return []domain.Project{{ID: 10, ProjectKey: "PROJ"}}, nil
    }
    return NewDefaultDocumentSearchBuilder(mock, "default", "myspace", "https://example.backlog.com")
}
```

## Implementation Steps（TDD: Red → Green → Refactor）

- [ ] Step 1 (Red): `document_search_test.go` に url テスト（6ケース）を追加 + Build 呼び出しを ctx 付きに更新 → コンパイルエラー確認
- [ ] Step 2 (Green): `DocumentSearchDetail` に `URL` 追加、`Build` シグネチャ変更、`ListProjects` ロジック実装
- [ ] Step 3 (Green): `cli/document.go` / `mcp/tools_document.go` の Build 呼び出しを ctx 付きに更新 → `go test ./...` green
- [ ] Step 4 (Refactor): `go vet ./...`、imports 整理（`context`, `fmt`, `strings` の追加）
- [ ] Step 5: golden test を `-update` で再生成（url フィールドが追加されるため）

## Risks

| リスク | 影響度 | 対策 |
|--------|--------|------|
| `ListProjects` の追加コールで検索レスポンスが遅くなる | 低 | 1回のみ。プロジェクト数が多くても O(N) のマップ構築のみ |
| アーカイブ済みプロジェクト / 権限外プロジェクト が ListProjects に出ない | 低 | projectKey miss → url 省略。warning なし（正常ケース）|
| `b.newEnvelope` への warnings 渡し漏れ | 中 | Step 4 で nil→warnings に変更して確認 |
| MockClient に `ListProjectsFunc` がない | 中 | Step 1 で確認。ない場合は先に追加 |

## Definition of Done
- `go test ./internal/digest/...` / `go test ./...` グリーン
- `go vet ./...` グリーン
- url が snippet/meta/full 全モードで返る
- ListProjects 失敗時も digest が返る（partial success）
- golden test が更新済み
- テストヘルパーに ListProjectsFunc stub を追加済み
- golden test を `-update` フラグで再生成済み（url フィールドが追加される）
