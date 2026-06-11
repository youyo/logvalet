# M3: CLI + MCP 配線 — lv document search / logvalet_document_search

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | M1（SearchDocuments）・M2（DocumentSearchBuilder） |
| 対象ファイル | `internal/cli/document.go` / `internal/cli/document_test.go`（既存）/ `internal/mcp/tools_document.go` / `internal/mcp/server_test.go`（既存） |

## Goal

M1・M2 をワイヤアップして `lv document search <keyword>` と `logvalet_document_search` MCP ツールを提供する。

**責務**:
- projectKey → projectID 解決（任意・複数可）
- count 既定 100・上限 100 クランプ（V4 対策）
- `backlog.SearchDocuments` 呼び出し → `DocumentSearchBuilder.Build()` → JSON 出力

**範囲外**:
- V1（projectId[] 省略時の横断挙動）の確認は M5 E2E の責務。M3 は実装が正しければよい
- 複数スペース fan-out は既存 `RegisterWithSpaces` が担う

## CLI 仕様

```
lv document search <keyword> [--project KEY ...] [--sort created|updated] [--order asc|desc]
                              [--count N] [--offset N] [--detail snippet|meta|full]
```

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|-----------|------|
| `keyword`（arg） | `string` | 必須 | 検索語 |
| `--project` / `-p` | `[]string` | `[]` | プロジェクトキー（複数可・省略=全体横断） |
| `--sort` | `string` | `""` | `"created"` \| `"updated"` |
| `--order` | `string` | `""` | `"asc"` \| `"desc"` |
| `--count` | `int` | `100` | 取得件数（1-100、100 にクランプ） |
| `--offset` | `int` | `0` | ページネーション開始位置 |
| `--detail` | `string` | `"snippet"` | `"snippet"` \| `"meta"` \| `"full"` |

## MCP ツール仕様

```
logvalet_document_search
  keyword:      string   (required) — 検索語
  project_keys: string[] (optional) — 絞り込みプロジェクトキー（複数可）
  sort:         string   (optional) — "created" | "updated"
  order:        string   (optional) — "asc" | "desc"
  count:        number   (optional) — 1-100（上限 100 クランプ）
  offset:       number   (optional) — ページネーション開始位置
  detail:       string   (optional) — "snippet" | "meta" | "full"（既定 "snippet"）
```

read-only → `RegisterWithSpaces`（`readOnlyAnnotation`）を使用。

## 設計

### DocumentCmd への Search フィールド追加（document.go）

```go
type DocumentCmd struct {
    Get    DocumentGetCmd    `cmd:"" help:"get document"`
    List   DocumentListCmd   `cmd:"" help:"list documents"`
    Tree   DocumentTreeCmd   `cmd:"" help:"get document tree"`
    Digest DocumentDigestCmd `cmd:"" help:"generate document digest"`
    Create DocumentCreateCmd `cmd:"" help:"create document"`
    Search DocumentSearchCmd `cmd:"" help:"search documents by keyword"`  // 追加
}
```

### DocumentSearchCmd（document.go に追加）

```go
type DocumentSearchCmd struct {
    Keyword     string   `arg:"" required:"" help:"search keyword"`
    ProjectKeys []string `short:"p" help:"project key(s) to filter (optional, multiple)"`
    Sort        string   `help:"sort field: created | updated"`
    Order       string   `help:"sort order: asc | desc"`
    Count       int      `default:"100" help:"max results (1-100)"`
    Offset      int      `default:"0" help:"pagination offset"`
    Detail      string   `default:"snippet" help:"verbosity: snippet | meta | full"`
}

func (c *DocumentSearchCmd) Run(g *GlobalFlags) error
```

### Run() の実装ロジック

1. `buildRunContext(g)` → `rc`
2. projectKeys が空でなければ `GetProject` で各キー → ID に解決
3. count を `max(1, min(c.Count, 100))` にクランプ
4. `rc.Client.SearchDocuments(ctx, backlog.SearchDocumentsOptions{...})`
5. `digest.NewDefaultDocumentSearchBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL).Build(docs, opts)`
6. `rc.Renderer.Render(os.Stdout, envelope)`

### MCP ツール（tools_document.go に追加）

`RegisterDocumentTools` 末尾に `logvalet_document_search` を追加。
- projectKeys は `stringSliceArg` or `interfaceSliceToStringSlice` パターンで取得（既存コードを参照）
- count クランプ（上限 100）をここでも適用
- `digest.NewDefaultDocumentSearchBuilder(...).Build(docs, opts)` を呼び出す

## TDD テスト設計

### document_test.go（CLI パース）

| # | ケース | 入力 | 期待 |
|---|--------|------|------|
| 1 | keyword のみ | `document search OAuth` | Keyword="OAuth", Count=100, Detail="snippet", ProjectKeys=[] |
| 2 | フラグ付き | `document search OAuth -p PROJ --detail meta --count 50` | ProjectKeys=["PROJ"], Detail="meta", Count=50 |
| 3 | 複数プロジェクト | `document search k -p A -p B` | ProjectKeys=["A","B"] |
| 4 | keyword なし | `document search` | エラー |
| 5 | count>100 | `document search k --count 200` | （Run 時に 100 にクランプ。パーステストでは値が 200 のまま。クランプは Run 内） |

### tools_document_test.go（MCP ツール・モックベース）

| # | ケース | 入力 | 期待 |
|---|--------|------|------|
| 1 | keyword 必須チェック | keyword="" | エラー |
| 2 | keyword のみ | keyword="test" | SearchDocuments が {Keyword:"test", Count:100, Offset:0} で呼ばれる |
| 3 | project_keys 指定 | project_keys=["PROJ"] | GetProject → projectID 解決 → SearchDocuments の ProjectIDs が設定される |
| 4 | count クランプ | count=200 | SearchDocuments の Count が 100 になる |

## Implementation Steps（TDD: Red → Green → Refactor）

- [ ] Step 1 (Red): `document_test.go` に `TestDocumentSearchCmd_Parse` テストを追加 → `go test ./internal/cli/...` FAIL
- [ ] Step 2 (Green): `document.go` に `DocumentSearchCmd` と `Run()` を実装 → パス
- [ ] Step 3 (Red): `tools_document_test.go` に `TestLogvaletDocumentSearch` テストを追加 → FAIL
- [ ] Step 4 (Green): `tools_document.go` に `logvalet_document_search` を追加 → パス
- [ ] Step 5 (Refactor): `go vet ./...`、既存テスト全パス確認

## Risks

| リスク | 影響度 | 対策 |
|--------|--------|------|
| projectKey → ID 変換で GetProject が複数プロジェクトに N+1 API コール | 低 | MVP では許容。テストはモックで確認 |
| MCP の string[] 引数取得パターンが既存コードと異なる | 中 | 既存 tools_issue.go の複数文字列引数を参照して合わせる |
| count=0 が渡された時の挙動 | 低 | `max(1, min(count, 100))`→ count=0 は 1 になる。または 0 を「既定 100」として扱う（SearchDocuments 層は Count=0 を「送らない」で API デフォルト 20 になる）。CLI/MCP 層では count=0 → 100 に統一する |

## Definition of Done
- `go test ./internal/cli/...` / `go test ./internal/mcp/...` / `go test ./...` グリーン
- `go vet ./...` グリーン
- `lv document search` が `--help` で表示される
- `logvalet_document_search` が MCP ツールとして登録されている（`internal/mcp/server_test.go` の `expectedCount` があれば +1）
