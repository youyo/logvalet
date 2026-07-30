# ToolDef ownership 型一式 (S06)

logvalet re-design (issue #52) の一環として、`internal/mcp` パッケージが
MCP SDK の型に依存せず MCP tool 定義・呼び出し結果を表現できるよう、logvalet 所有の型を
`internal/mcp/tooldef.go` / `internal/mcp/toolresult.go` に定義した。本ドキュメントは
SDK 型 → logvalet 型の対応表と、移行基準スナップショット
(`internal/mcp/testdata/tools_list_baseline.json`) の仕様を記録する。

> **S11 時点の現状**: 旧 SDK backend は削除され、本番の ServerBackend 実装は
> 公式 Go SDK (`github.com/modelcontextprotocol/go-sdk`) 版
> (`internal/mcp/backend_official.go`) のみ。以下の対応表のうち「SDK 型」列は
> baseline 採取時点の旧 SDK の型であり、現在の変換先は
> `internal/mcp/tooldef_official.go` の `ToOfficialSDKTool` /
> `ToOfficialSDKResult` およびその逆変換を参照すること。

このステップ (S06) では **既存コード (tools_*.go, server.go 等) は変更しない**。
新規型・新規ファイルの追加のみを行い、実際に既存ツール登録コードをこれらの型に
置き換えるのは後続ステップ (S07/S09 想定) の責務。

## 1. SDK 型 → logvalet 型 対応表

### 1.1 ツール定義 (tools/list の1ツール)

| 概念 | SDK 型 (baseline 採取時点の旧 SDK) | logvalet 型 (`internal/mcp`) |
|---|---|---|
| ツール定義全体 | `gomcp.Tool` | `ToolDef` |
| ツール名 | `Tool.Name` | `ToolDef.Name` |
| 表示名 | `Tool.Title` | `ToolDef.Title` |
| 説明文 | `Tool.Description` | `ToolDef.Description` |
| 入力スキーマ | `Tool.InputSchema` (`ToolInputSchema`, `map[string]any` ベースの `Properties`) | `ToolDef.Params []ParamSpec` + `ToolDef.Required []string` (構造化表現) |
| スキーマ1プロパティ | `map[string]any` (例: `{"type":"string","description":"..."}`) | `ParamSpec{Name, Type, Description, Enum, Items, Properties}` |
| annotation (behavior hint) | `Tool.Annotations` (`gomcp.ToolAnnotation`, `*bool` フィールド) | `ToolDef.Annotation` (`ToolAnnotation`, 同じく `*bool` で未設定/true/false の3値を保持) |

`ParamSpec.Type` は `ParamType` (`string`/`number`/`integer`/`boolean`/`array`/`object`) の
列挙型で表現し、SDK の生 `map[string]any` に依存する曖昧さを排除する。

変換関数:

- `ToolDef.ToSDKTool() gomcp.Tool` — logvalet 型 → SDK 型。既存の
  `ToolRegistry.Register(tool gomcp.Tool, fn ToolFunc)` 系 API へブリッジするために使う
  (S07/S09 で利用予定)。
- `ToolDefFromSDKTool(t gomcp.Tool) ToolDef` — 逆変換。`ToSDKTool` の逆。
- `ParamSpec.ToJSONSchema() map[string]any` / `ParamSpecFromJSONSchema(name string, schema map[string]any) ParamSpec`
  — 1プロパティ単位の JSON Schema 相互変換。

### 1.2 request meta (呼び出しリクエストに付随するメタ情報)

| 概念 | SDK 型 | logvalet 型 |
|---|---|---|
| クライアント識別情報 (`initialize` の `clientInfo`) | `gomcp.Implementation{Name, Version, Title, ...}` | `ClientInfo{Name, Version, Title}` |
| protocolVersion | `InitializeRequest.Params.ProtocolVersion` (string) | `RequestMeta.ProtocolVersion` |
| 呼び出し元付加情報 (progressToken 等、プロトコルで未定義の `_meta` フィールド) | `gomcp.Meta.AdditionalFields map[string]any` | `RequestMeta.Extra map[string]any` |

`RequestMeta` は `ClientInfo` + `ProtocolVersion` + `Extra` の3フィールド構成。
現時点では組み立て/参照する production コードは存在しない (型定義のみ)。

### 1.3 result meta (呼び出し結果に付随するメタ情報)

| 概念 | SDK 型 | logvalet 型 |
|---|---|---|
| サーバー識別情報 (`initialize` の `serverInfo`) | `gomcp.Implementation` | `ServerInfo{Name, Version, Title}` |
| OAuth 未接続時の認可 URL 通知 | `tools.go` の `toolResultAuthRequired` が組み立てる `gomcp.Meta{AdditionalFields: {"authorization_required": true, "authorization_url": url}}` | `ResultMeta{AuthorizationRequired bool, AuthorizationURL string}` |
| その他 `_meta` の任意フィールド | `gomcp.Meta.AdditionalFields` | `ResultMeta.Extra map[string]any` |

`ResultMeta.ToMap() map[string]any` は `gomcp.Meta.AdditionalFields` 互換の
`map[string]any` を組み立てる (`authorization_required`/`authorization_url` は
値が偽/空文字列のとき出力しない = optional field 省略と null の同一視)。

### 1.4 result content (呼び出し結果のコンテンツ)

| 概念 | SDK 型 | logvalet 型 |
|---|---|---|
| content 配列の1要素 (text) | `gomcp.TextContent{Type: "text", Text: string}` (`Content` interface の実装の一つ) | `ToolContent{Type: ToolContentTypeText, Text: string}` |
| structured content | `CallToolResult.StructuredContent any` | `ToolResult.StructuredContent any` (同じ `any` のまま保持) |

logvalet の全ツールは JSON をテキスト化した `text` content のみを返す
(`ToolRegistry.callWithDefaultClient` 等が `json.Marshal` → `NewToolResultText` する実装)
ため、`ToolContentType` は現時点で `text` のみをサポートする。image/audio/embedded
resource 等、SDK の `Content` interface が持つ他バリアントは対象外。

### 1.5 error 表現

| 概念 | SDK 型 | logvalet 型 |
|---|---|---|
| tool 呼び出しエラー | JSON-RPC レベルのエラーではなく `CallToolResult{IsError: true, Content: [TextContent{Text: message}]}` という規約 (`gomcp.NewToolResultError`) | `ToolError{Message string}` + `NewErrorToolResult(ToolError) ToolResult` (`IsError: true` の `ToolResult` を組み立てるコンストラクタ) |
| isError フラグ | `CallToolResult.IsError bool` (`omitempty`) | `ToolResult.IsError bool` |

`ToolResult`/`ToolError` は MCP の「tool エラーはプロトコルエラーではなく
結果オブジェクトの `isError=true` で表現する」という規約を型として明示する。

### 1.6 space injection 用パラメータ

`tools.go` の `injectSpaceParams` / `injectSpaceParamWrite` が `gomcp.Tool.InputSchema.Properties`
に直接注入している `spaces` / `all_spaces` パラメータの logvalet 型表現:

| 用途 | 既存実装 (`tools.go`) | logvalet 型 (`tooldef.go`) |
|---|---|---|
| read 系ツール (`RegisterWithSpaces` → `injectSpaceParams`) の `spaces` | `map[string]any{"type":"array","items":{"type":"string"},"description":"..."}` | `SpacesParamSpec(description string) ParamSpec` |
| read 系ツールの `all_spaces` | `map[string]any{"type":"boolean","description":"..."}` | `AllSpacesParamSpec(description string) ParamSpec` |
| write 系ツール (`RegisterWithSpacesWrite` → `injectSpaceParamWrite`) の `spaces` (単一指定) | 同上 (array/string items) | `SpacesWriteParamSpec(description string) ParamSpec` |

パラメータ名は `ParamNameSpaces = "spaces"` / `ParamNameAllSpaces = "all_spaces"` の
定数で固定する。

## 2. 移行基準スナップショット (`internal/mcp/testdata/tools_list_baseline.json`)

### 2.1 取得方法

- 取得プロトコル: MCP `tools/list` (JSON-RPC 2.0)。
- 取得経路 (S11 以降 / 公式 Go SDK 基準):
  `internal/mcp.NewOfficialStreamableHTTPHandler(backlog.NewMockClient(), "test", ServerConfig{})`
  が返す `StreamableHTTPHandler` (`Stateless: true` / `JSONResponse: true`) を `httptest` で
  起動し、本番と同一のツール登録 (`registerAllTools`) を経たサーバーに対して
  `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` を POST して取得する。
  実際の突き合わせは `internal/mcp/backend_official_test.go` の
  `TestOfficialServer_ToolsList_MatchesBaseline` が行う。
- 使用 SDK バージョン: `github.com/modelcontextprotocol/go-sdk v1.7.0` (go.mod に固定)。
  公式 SDK は SEP-2575 の `result.cacheScope` / `result.ttlMs` を追加で返すため、
  baseline 比較時は `stripOfficialSDKOnlyResultFields` でこの2フィールドのみ除外する
  (意図した SDK 間差分。baseline 自体は書き換えない)。
- 登録ツール総数: 72 (`server_test.go` の `TestNewServerWithFactory_RegistersAllTools`
  が期待する `expectedCount` と同期)。

### 2.2 正規化規則

`internal/mcp/snapshot_test.go` の `normalizeToolsListResponse` が実装する。

1. **キー順ソート**: JSON オブジェクトの全キーをコード順にソートする。生の
   `tools/list` レスポンスを一度 `map[string]any` に decode してから re-encode する
   ことで達成する (`encoding/json` は `map[string]any` を marshal する際に自動で
   キーをソートする)。
2. **ツール配列の名前順ソート**: `result.tools` 配列を `tool.name` の昇順で
   並べ替える。
3. **optional field 省略と null の同一視**: JSON 値が `null` のキーは出力から
   取り除く (`dropNullFields`)。SDK 側の実装差異で「フィールド省略」と
   「`null` 値」が揺れても同一の正規形になる。
4. **required 配列のソート**: 各ツールの `inputSchema.required` 配列を
   文字列昇順にソートする。
5. **annotation のポインタ値の値化**: `ToolAnnotation` の `*bool` ヒントは
   JSON テキスト上では素の `true`/`false` リテラルとして表現され、Go 特有の
   ポインタという概念はシリアライズの時点で消える。そのため規則1・3を適用した
   時点で自動的に達成される (未設定の hint はキーごと消え、設定済みの hint は
   値そのものになる)。

正規化後の出力は 2-space indent の JSON (`encoding/json.Encoder` + `SetIndent`) で、
`testdata/tools_list_baseline.json` はこの正規形そのものを保持する
(生レスポンスではなく、既に正規化済みの golden ファイル)。

### 2.3 べき等性

`normalizeToolsListResponse` は冪等でなければならない
(`normalize(x) == normalize(normalize(x))`)。`TestNormalizeToolsListResponse_Idempotent`
がこれを検証し、同時に `testdata/tools_list_baseline.json` 自体が既に正規形で
あることも確認する (`normalize(baseline) == baseline`)。

### 2.4 S07/S09 での使い方

移行後の実装 (ToolDef ベースのツール登録・SDK 型への変換) が生成する `tools/list`
レスポンスを同じ `normalizeToolsListResponse` に通し、`testdata/tools_list_baseline.json`
とバイト単位で比較する golden test を追加することで、移行前後で観測可能なプロトコル
応答が変化していないことを検証する。この baseline ファイルと正規化器の組が、
移行の正しさを判定する唯一の基準となる。
