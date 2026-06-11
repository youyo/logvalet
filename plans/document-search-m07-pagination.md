# M7: ページネーション改善（possibly_more バグ修正 + next_offset）

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | M6（Build の ctx 対応後） |
| 対象ファイル | `internal/digest/document_search.go` / `internal/digest/document_search_test.go` / `internal/cli/document.go` / `internal/mcp/tools_document.go` |

## Goal

### バグ修正（AD11）

`PossiblyMore: len(docs) >= 100` のハードコードが偽陰性を引き起こしている:

```
--count 50 で 200件ある場合:
  API が 50件返す → len(docs)=50 < 100 → possibly_more=false  ← 偽陰性！
  ユーザーは「もう結果がない」と誤解する
```

修正: `PossiblyMore: len(docs) >= requestedCount`（requestedCount = CLI/MCP から渡した count）

AD7（「100件返却時は possibly_more=true」）は `requestedCount=100` 時に自動的に維持される。

### 利便性向上

`next_offset` フィールドをダイジェストに追加し、ユーザーが次のページで使うべき `--offset` 値をそのまま提示する:

```json
{
  "total_returned": 100,
  "possibly_more": true,
  "next_offset": 100,   ← "次は --offset 100 を使え" という意味
  ...
}
```

## 設計

### DocumentSearchOptions の拡張

```go
type DocumentSearchOptions struct {
    Keyword       string // スニペット抽出のアンカー語（空可）
    Detail        string // "snippet"（既定）| "meta" | "full"
    RequestedCount int   // ← 追加: CLI/MCP で指定した count（0は100として扱う）
    Offset         int   // ← 追加: 今回のオフセット（next_offset 計算用）
}
```

### DocumentSearchDigest の拡張

```go
type DocumentSearchDigest struct {
    Keyword       string                 `json:"keyword"`
    Detail        string                 `json:"detail"`
    TotalReturned int                    `json:"total_returned"`
    PossiblyMore  bool                   `json:"possibly_more"`
    NextOffset    int                    `json:"next_offset,omitempty"`  // ← 追加（possibly_more=true のときのみ設定）
    Items         []DocumentSearchDetail `json:"items"`
}
```

### Build 内の変更

```go
func (b *DefaultDocumentSearchBuilder) Build(ctx context.Context, docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope {
    // ... 既存ロジック ...
    
    // requestedCount の正規化（0は100として扱う）
    requestedCount := opt.RequestedCount
    if requestedCount <= 0 {
        requestedCount = 100
    }
    
    possiblyMore := len(docs) >= requestedCount
    nextOffset := 0
    if possiblyMore {
        nextOffset = opt.Offset + len(docs)
    }
    
    digestData := &DocumentSearchDigest{
        Keyword:       opt.Keyword,
        Detail:        detail,
        TotalReturned: len(docs),
        PossiblyMore:  possiblyMore,
        NextOffset:    nextOffset,  // possibly_more=false のとき 0 → omitempty で出力されない
        Items:         items,
    }
    // ...
}
```

### CLI の更新（`internal/cli/document.go`）

```go
// DocumentSearchCmd.Run() 内
envelope := builder.Build(ctx, docs, digest.DocumentSearchOptions{
    Keyword:        c.Keyword,
    Detail:         c.Detail,
    RequestedCount: count,   // ← 追加（クランプ後の count）
    Offset:         c.Offset, // ← 追加
})
```

### MCP の更新（`internal/mcp/tools_document.go`）

```go
return builder.Build(ctx, docs, digest.DocumentSearchOptions{
    Keyword:        keyword,
    Detail:         detail,
    RequestedCount: count,   // ← 追加
    Offset:         offset,  // ← 追加（intArg で取得）
}), nil
```

## TDD テスト設計

### document_search_test.go

| # | テストケース | 入力 | 期待 |
|---|-------------|------|------|
| 1 | **バグ修正**: count=50 で 50件返却 | RequestedCount=50, 50件 | PossiblyMore=false（修正前は false だったが偽陰性確認のため明示） |
| 2 | **バグ修正**: count=50 で 50件返却（以前の偽陽性テストの逆） | RequestedCount=50, 49件 | PossiblyMore=false |
| 3 | **AD7 維持**: count=100 で 100件返却 | RequestedCount=100, 100件 | PossiblyMore=true |
| 4 | count=50 で 50件 | RequestedCount=50, 50件 | PossiblyMore=true, NextOffset=50 |
| 5 | offset=100 で 50件 | RequestedCount=50, Offset=100, 50件 | NextOffset=150 |
| 6 | possibly_more=false → next_offset=0 | RequestedCount=100, 99件 | NextOffset=0（omitempty で出力なし） |
| 7 | RequestedCount=0（未設定） | 100件 | PossiblyMore=true（0→100 として扱う） |

### golden test 更新

`testdata/document_search_snippet.json` の `possibly_more`・`next_offset` フィールドを更新。

## Implementation Steps（TDD: Red → Green → Refactor）

- [ ] Step 1 (Red): `document_search_test.go` に上記7テストを追加 → 既存テストとの差分を確認（特に Test #3 は M2 の 100件テストと重複する可能性 → テスト統合または追加）
- [ ] Step 2 (Green): `DocumentSearchOptions` に `RequestedCount`・`Offset` 追加、`DocumentSearchDigest` に `NextOffset` 追加、`Build` ロジック更新
- [ ] Step 3 (Green): `cli/document.go` / `mcp/tools_document.go` の DocumentSearchOptions に `RequestedCount`・`Offset` を渡すように更新
- [ ] Step 4 (Refactor): `go vet ./...`、golden test 更新（`-update` フラグ）

## ユーザーへの提示例（スキル内）

```
Found 100 document(s) matching "OAuth". 

⚠️ possibly_more=true: 100件以上ヒットしている可能性があります。
   次のページを取得するには: lv document search "OAuth" --offset 100
```

MCP の場合は `next_offset` フィールドをそのまま参照:
```json
{
  "possibly_more": true,
  "next_offset": 100
}
```

## Risks

| リスク | 影響度 | 対策 |
|--------|--------|------|
| M2 の golden test が `possibly_more`・`next_offset` フィールド変更で壊れる | 中 | Step 2 後に `-update` で再生成 |
| `RequestedCount=0` のデフォルト扱いが CLI のデフォルト count=100 と不一致 | 低 | CLI は常に count（クランプ後）を渡すので 0 にならない。0 は「オプション未設定時のフォールバック」 |

## Definition of Done
- `go test ./...` グリーン（バグ修正テスト含む）
- `go vet ./...` グリーン
- `--count 50` のとき `possibly_more` が正しく設定される
- `next_offset` が `possibly_more=true` のときのみ出力される
