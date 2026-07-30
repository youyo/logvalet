# 0001. MCP SDK を mark3labs/mcp-go から公式 go-sdk へ一本化する

## ステータス

承認済み（issue #52、2026-07-30）

## コンテキスト

logvalet の MCP サーバーは `github.com/mark3labs/mcp-go v0.57.0` を用いて実装されていた。
MCP プロトコルは 2026-07-28 版で `Stateless=true` を前提とした stateless サーバー、
`server/discover`、per-request `_meta`、MRTR（InputRequiredResult）等の新しい wire protocol
を要求するようになった。`modelcontextprotocol/go-sdk` (公式 Go SDK) は v1.7.0 で 2026-07-28
対応を完了しているが、`mark3labs/mcp-go` の対応状況（`server/discover`・MRTR・新ヘッダー）は
確認できなかった。

72 個のツール定義を SDK 型に直接依存させたまま書いていたため、SDK 切り替えは大規模な書き換えを
伴う。一方で `ToolFunc`（ツール本体のハンドラ実装）自体は SDK 非依存に保たれていたため、
定義部分の抽象化次第でリスクを分離できる状態だった。

## 決定

公式 Go SDK (`modelcontextprotocol/go-sdk`) へ一本化する。移行は2段階で行う。

1. logvalet 所有の SDK 非依存な `ToolDef` 抽象を導入し、72 ツール定義をこの抽象経由で
   `mark3labs/mcp-go` backend に接続する（M2、S06-S08）。この時点でツール定義の形は
   SDK から独立するが、実行 backend はまだ mark3labs のまま。
2. `ServerBackend` interface の公式 SDK 実装を追加し、`Stateless=true` で稼働することを
   確認したうえで、mark3labs backend と依存一式を削除する（M3、S09-S11）。

`go.mod` から `github.com/mark3labs/mcp-go` を削除し、`github.com/modelcontextprotocol/go-sdk`
を追加する。両 SDK の恒久併存は行わない。

## 検討した代替案

### 代替案A: 両 SDK の併存を維持する

`mark3labs/mcp-go` を stdio 等の既存経路に残し、新しい wire protocol が必要な箇所だけ
公式 SDK を使う。

却下理由: 保守コストが二重化する（ツール定義・テスト・ヘッダー処理を2系統維持する必要がある）。
また mark3labs 側が 2026-07-28 に将来対応したとしても、その間 logvalet は2つの SDK の挙動差異
（エラー形式、meta の扱い等）を吸収し続けるコストを払い続けることになる。

### 代替案B: mark3labs/mcp-go の 2026-07-28 対応を待つ

mark3labs のアップストリーム対応を待ってから移行する。

却下理由: 対応時期が不明であり、MCP 2026-07-28 対応（本再設計の中核テーマ）を含む全マイルストーン
がブロックされる。対応時期に依存しない自社所有の抽象層（`ToolDef`）を先に挟む方が、
SDK の選択自体をいつでも切り替え可能な設計にでき、待つ理由がなくなる。

## 影響

- `internal/mcp` 配下のツール定義・ハンドラ登録経路が `ToolDef` / `ServerBackend` 抽象を経由する
  設計に変わった。ツール本体（`ToolFunc`）71個は無変更で移行できた。
- `go.mod` から `github.com/mark3labs/mcp-go` が削除され、`github.com/modelcontextprotocol/go-sdk`
  が追加された。
- stdio・HTTP 双方のトランスポートが公式 SDK backend 上で動作する（`Stateless=true`）。
- 破壊的変更として次期メジャーリリースの CHANGELOG に記載する。
