# M2: digest — DocumentSearchBuilder + スニペット抽出

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | M1（backlog.Client.SearchDocuments が必要） |
| 対象ファイル | `internal/digest/document_search.go` / `internal/digest/document_search_test.go` / `testdata/document_search_*.json`（golden） |

## Goal

`[]domain.Document` を単一 `DigestEnvelope` に変換する `DocumentSearchBuilder` を追加する。
クライアント層（M1）とワイヤ層（M3）の橋渡し。API コールは行わず、渡されたデータから純粋に digest を構築する。

**責務境界**:
- スニペット抽出・verbosity 切替・件数サマリーは本マイルストーンの責務
- API 呼び出し（`SearchDocuments`）・count クランプは M3 の責務
- 複数スペース fan-out は上位 `RegisterWithSpaces` の責務（本層は単一スペース分の `[]Document` を受け取るだけ）

## 設計

### DocumentSearchDetail（各ドキュメントの検索結果エントリ）

```go
// DocumentSearchDetail は検索結果の1件エントリ
type DocumentSearchDetail struct {
    ID          string          `json:"id"`
    ProjectID   int             `json:"project_id"`
    Title       string          `json:"title"`
    Snippet     string          `json:"snippet,omitempty"`   // snippet/full のみ
    Plain       string          `json:"plain,omitempty"`     // full のみ
    Created     *time.Time      `json:"created,omitempty"`
    Updated     *time.Time      `json:"updated,omitempty"`
    CreatedUser *domain.UserRef `json:"created_user,omitempty"`
    UpdatedUser *domain.UserRef `json:"updated_user,omitempty"`
}
```

### DocumentSearchDigest（digest フィールド）

```go
// DocumentSearchDigest は document search digest のトップレベル
type DocumentSearchDigest struct {
    Keyword       string                  `json:"keyword"`
    Detail        string                  `json:"detail"`          // "snippet" | "meta" | "full"
    TotalReturned int                     `json:"total_returned"`
    PossiblyMore  bool                    `json:"possibly_more"`   // true = 100件ちょうど返却
    Items         []DocumentSearchDetail  `json:"items"`
}
```

### DocumentSearchOptions（verbosity など）

```go
// DocumentSearchOptions は DocumentSearchBuilder.Build() のオプション
type DocumentSearchOptions struct {
    Keyword string // スニペット抽出のアンカー語（空可）
    Detail  string // "snippet"（既定）| "meta" | "full"
}
```

### DocumentSearchBuilder インターフェース

```go
// DocumentSearchBuilder は []domain.Document から DigestEnvelope を生成するインターフェース
type DocumentSearchBuilder interface {
    Build(docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope
}

// DefaultDocumentSearchBuilder は DocumentSearchBuilder の標準実装
type DefaultDocumentSearchBuilder struct {
    BaseDigestBuilder
}

func NewDefaultDocumentSearchBuilder(client backlog.Client, profile, space, baseURL string) *DefaultDocumentSearchBuilder
```

### スニペット抽出関数（内部）

```go
// extractSnippet は plain からキーワード周辺 ±snippetRadius rune を切り出す。
// - []rune ベース（マルチバイト安全）
// - ケースインセンシティブ（strings.ToLower で正規化してから検索）
// - keyword が空またはヒットなし: 先頭 snippetRadius*2 rune をリード抜粋
// - 複数語（スペース区切り）: 最初にマッチした語をアンカーにする
// - 切り出し位置が rune 境界と一致するため []byte 禁止
const snippetRadius = 100 // keyword 前後に含める rune 数

func extractSnippet(plain, keyword string) string
```

### verbosity ルール

| detail | Snippet フィールド | Plain フィールド |
|--------|-------------------|-----------------|
| snippet（既定） | keyword 周辺抜粋（なければリード抜粋） | 返さない |
| meta | 返さない | 返さない |
| full | keyword 周辺抜粋（なければリード抜粋） | 全文 |

## TDD テスト設計

### document_search_test.go — Build() ユニットテスト

| # | テストケース | 入力 | 期待 |
|---|-------------|------|------|
| 1 | 空スライス | `[]Document{}` | Items=[], total_returned=0, possibly_more=false |
| 2 | snippet モード（keyword あり） | 2件、keyword="OAuth" | Snippet が非空・Plain が空 |
| 3 | meta モード | 2件、keyword="k" | Snippet 空・Plain 空 |
| 4 | full モード | 1件、keyword="k" | Snippet 非空・Plain が全文 |
| 5 | possibly_more=true | 100件スライス | PossiblyMore=true |
| 6 | possibly_more=false | 99件スライス | PossiblyMore=false |
| 7 | keyword ヒットなし → リード抜粋 | keyword="zzz"、plain 長文 | Snippet が先頭 200 rune 以内 |
| 8 | DigestEnvelope 構造 | 任意 | Resource="document_search"、Warnings=[] |

### extractSnippet ユニットテスト（document_search_test.go に含める）

| # | テストケース | 期待 |
|---|-------------|------|
| 1 | ASCII keyword、中央ヒット | keyword 周辺 ±100 rune |
| 2 | 日本語 keyword（マルチバイト） | []rune ベースで正しい切り出し |
| 3 | ケースインセンシティブ | "oauth" で "OAuth" にマッチ |
| 4 | keyword なし/ヒットなし | 先頭 200 rune |
| 5 | 複数語 keyword（スペース区切り） | 最初にマッチした語をアンカーに |
| 6 | plain が radius*2 より短い | plain 全体を返す |

### golden test（document_search_test.go）

`testdata/document_search_snippet.json`（2件・snippet モード）を fixture に使い、
`Build()` の JSON 出力が golden ファイルと一致することを検証。
`-update` フラグで更新可能にする（既存 golden test と同パターン）。

## Implementation Steps（TDD: Red → Green → Refactor）

- [ ] Step 1 (Red): `document_search_test.go` に `TestDocumentSearchBuilder_Build` と `TestExtractSnippet` を追加 → コンパイルエラー確認
- [ ] Step 2 (Green): `document_search.go` に型・関数・Builder を実装 → `go test ./internal/digest/...` green
- [ ] Step 3 (Golden): `testdata/` に fixture JSON を作成し golden test を通す
- [ ] Step 4 (Refactor): `go vet ./...`、`extractSnippet` の境界値を再確認

## Risks

| リスク | 影響度 | 対策 |
|--------|--------|------|
| plain が markdown 記法を含み snippet が読みにくい | 低 | V2 確認まで除去スキップ。extractSnippet は pure rune 操作のみ |
| 複数語 keyword のアンカー決定ルールの曖昧さ | 中 | 「最初にマッチした語（スペース区切りで先頭の語）をアンカーに」とルールを明記 |
| possibly_more の境界：100 件 → true の判定 | 低 | `len(docs) >= 100` で判定（count 上限は M3 の責務で 100 に固定） |

## Definition of Done
- `go test ./internal/digest/...` がグリーン（extractSnippet ユニットテスト + Build テスト + golden）
- `go vet ./...` グリーン
- M1 `SearchDocuments` テストが無変更で通り続ける
