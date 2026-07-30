# S15: version negotiation と旧版互換方式の決定 (issue #52)

## 選択した方式

**(A)**: 公式 Go SDK (`github.com/modelcontextprotocol/go-sdk` v1.7.0) が提供する
version negotiation をそのまま利用し、logvalet 側での二重実装・追加の設定露出は
行わない。

参考: S03 スパイク実測レポート
(`git show voyager/mcp-impl/S03-a1:docs/specs/spike-go-sdk-2026-07-28.md`。
このファイルは S03 のブランチにのみ存在し、本ブランチの `docs/specs/` には
含まれない)。S14 (`internal/cli/mcp_headers.go` / `mcp_headers_test.go`) が
ヘッダ検証 (`Mcp-Method`/`Mcp-Name`、`-32020`/`-32022`) について確立した
「SDK が完全に代行するため logvalet 側の実装は無い」という前例を、
version negotiation 全体に対しても踏襲する。

## 決定理由

### supportedVersions は SDK 側に「設定可能な項目」自体が存在しない

S15 の done_criteria は「supportedVersions を設定可能にし、既定を 2026-07-28 +
2025-11-25 とする (SDK の設定項目で足りる場合は薄い設定露出のみ)」ことを
求めていたが、実装時に SDK v1.7.0 のソース (`mcp/shared.go`,`mcp/server.go`,
`mcp/streamable.go`) を確認した結果、次の事実が判明した。

- サーバーが応答する `supportedVersions` は `mcp/shared.go` の非公開変数
  `supportedProtocolVersions`（`2026-07-28`, `2025-11-25`, `2025-06-18`,
  `2025-03-26`, `2024-11-05` の5件、降順固定）で決まる。
- `ServerOptions` / `StreamableHTTPOptions` のいずれにも、このリストを
  上書き・縮小・拡張するためのフィールドは存在しない
  (`ServerOptions` は `Instructions`/`Logger`/各種ハンドラ/`Capabilities`等を
  持つが `SupportedVersions` 相当のフィールドは無い)。
- `server/discover` の応答 (`Server.discover`, `mcp/server.go`) が返す
  `SupportedVersions` は、この固定リストを `Session.supportedVersions`
  (トランスポートが `ProtocolVersionSupporter` を実装する場合にフィルタされた
  値。StreamableHTTP は実装していないため通常はフィルタなし) で絞り込んだもの。

つまり「supportedVersions を設定可能にする」ための薄い設定露出コード自体を
書く先が SDK に無い。既定の5バージョン（2026-07-28 を含む）が常に有効になって
おり、要求されていた「既定 2026-07-28 + 2025-11-25」は追加のコードなしで
最初から満たされている（それどころか 2025-06-18/2025-03-26/2024-11-05 まで
サポート範囲に含まれる、要求より広い後方互換性が SDK 標準で提供されている）。

このため mcp.go / discover.go に「supportedVersions 用のフラグ」等の
本番コードは追加しない。存在しない設定項目のために形だけのフラグ・環境変数を
生やすことは「二重実装しない」という契約（S15 前提の趣旨）に反すると判断した。
本ファイルが唯一の成果物であり、実装差分はテスト
(`internal/mcp/version_negotiation_test.go`) のみである。

## 同一 URL でのディスパッチ条件

logvalet の MCP サーバーは新旧いずれのプロトコルも同一エンドポイント (`/mcp`)・
同一 `*officialmcp.Server` インスタンスで受け付ける（`internal/mcp/backend_official.go`
の `newStreamableHTTPHandler`。`StreamableHTTPOptions{Stateless: true,
JSONResponse: true}`）。リクエストごとの分岐は SDK 内部 (`streamable.go`) が
以下の条件で行う。

1. **`_meta.protocolVersion` が無い、またはボディに `_meta` が無い**:
   旧プロトコル扱い。`Mcp-Protocol-Version` ヘッダの有無に関わらずヘッダ検証は
   一切行われない（`Mcp-Method`/`Mcp-Name` 要求も無し）。Stateless=true の
   ため、旧世代クライアント（2025-11-25 以前を想定した、initialize を先に
   送る実装であっても、送らずに直接 `tools/call` するだけの実装であっても
   両方）はリクエストごとの一時セッションでそのまま処理される。
2. **ボディの `_meta` に `protocolVersion` キーが存在する（値は問わない）**:
   `Mcp-Protocol-Version` HTTP ヘッダの存在・一致が必須になる
   (`streamable.go`: `if protocolVersion >= protocolVersion20260728 ||
   metaVersion != ""`)。ヘッダが無ければ `-32020`
   (`"Mcp-Protocol-Version header is required for requests carrying
   \"io.modelcontextprotocol/protocolVersion\""`)。ヘッダとボディの値が
   食い違えば同じく `-32020`。
   - この分岐は `_meta.protocolVersion` の値が `2026-07-28` 未満（例:
     `"2025-11-25"`）であっても発火する。つまり「`_meta` を送るが値は旧
     バージョン」というリクエストは SDK 上は成立しない構成であり、実際の
     2025-11-25 世代クライアントは元々 `_meta`/SEP-2575 を知らないため
     この形のリクエストを送ることはない（`version_negotiation_test.go` の
     V03 コメント参照）。
3. **`Mcp-Protocol-Version` ヘッダが `2026-07-28` 以上**: `Mcp-Method`/
   `Mcp-Name` ヘッダの検証が追加で有効になる（S14 で確立済み。
   `streamable_headers.go` の `minVersionForStandardHeaders =
   protocolVersion20260728`）。
4. **既定値の扱い（`Mcp-Protocol-Version` ヘッダ自体が無い場合）**:
   HTTP トランスポートの MCP 仕様上、ヘッダ省略時の解釈は
   クライアント側の実装に委ねられており、2025-03-26 世代の spec がヘッダ
   自体を導入する前のクライアントは送ってこない。SDK はヘッダが空文字列の
   場合を「ヘッダ無し = 旧プロトコル扱い」として一律処理する
   (`streamable.go`: `protocolVersion := header.Get(protocolVersionHeader)`
   の空文字列を弾く分岐は存在せず、`protocolVersion == ""` は単に
   「新プロトコル判定に使う値が空」として扱われ、上記1の旧プロトコル経路に
   合流する)。logvalet 側で `2025-03-26` をデフォルト値として明示的に
   採用・注入するコードは無い（存在しないヘッダを補完する処理を追加すると
   二重実装になるため）。

## legacy session の stateless 経路からの隔離

新プロトコル（`_meta.protocolVersion >= 2026-07-28`）は
`StreamableHTTPOptions.Stateless = true` のサーバーでのみ許可される
(`streamable.go`: `if !c.stateless && jreq.Method != methodDiscover { ... only
supported on stateless HTTP servers ... }`)。logvalet の MCP サーバーは
常に `Stateless: true` で構築されている
(`internal/mcp/backend_official.go` の `newStreamableHTTPHandler`) ため、
新プロトコルは常に許可される一方、Stateless モードでは
「`Mcp-Session-Id` を読み書きしない」「`GET`/`DELETE` は 405」という制約により
サーバー→クライアントの非同期プッシュ通知やセッションをまたぐ状態
（`notifications/*` 等）が前提の機能は使えない。

この Stateless モードの下では、旧プロトコルの `initialize` →
`notifications/initialized` → `Mcp-Session-Id` によるセッション継続という
本来のフローも「毎リクエスト使い捨てセッション」に置き換わり、
`Mcp-Session-Id` はそもそも読まれない。つまり「legacy session を stateless
経路から隔離する」という要求は、SDK が Stateless モードそのものの定義として
担保している（隔離のための追加コードを書く必要が logvalet 側に無い）。
根拠: S03 スパイクの `TestA_StatelessDirectToolCall_LegacyProtocol_RequiresInitialize`
（`_meta` 無しの `tools/call` が `Stateless: true` サーバーへ initialize なしで
そのまま成功した実測ログ）。

## 影響クライアント一覧

| プロトコルバージョン (クライアント側の想定) | logvalet (Stateless=true) での扱い |
|---|---|
| 2026-07-28 (SEP-2575 新プロトコル) | `_meta.protocolVersion` + `Mcp-Protocol-Version`/`Mcp-Method`/(`tools/call`等では)`Mcp-Name` ヘッダを揃えれば initialize 無しで完全動作。`version_negotiation_test.go` V02。 |
| 2025-11-25 (initialize/session ベース、Streamable HTTP) | `_meta` を送らない通常の `tools/call` として、initialize 無しでそのまま成功（Stateless モードの一時セッション）。`version_negotiation_test.go` V03。 |
| 2025-06-18 / 2025-03-26 / 2024-11-05 | 同上（`_meta` を使わない旧世代クライアントは全てこの経路）。SDK の `supportedProtocolVersions` に含まれるため `initialize.protocolVersion` として送っても `-32022` にならない。 |
| 上記以外の未知バージョン文字列 (`_meta.protocolVersion` に指定した場合) | `-32022` (`UnsupportedProtocolVersionError`) + `data.supported` に上記5バージョンの一覧。`version_negotiation_test.go` V01。 |

## AgentCore Gateway 運用上の注意（S28 参照用）

AgentCore Gateway 等、MCP サーバーの前段でプロトコルバージョンをアドバタイズする
ゲートウェイ型のプロキシを経由する構成では、ゲートウェイ側の
`supportedVersions` 設定が **全置換 (リスト全体を上書き)** である製品がある。
そのため、ゲートウェイ側の設定を logvalet バックエンドの実際のサポート範囲
（本ドキュメントの表: 2026-07-28 〜 2024-11-05 の5バージョン）とズレたまま
デプロイすると、次のいずれかの不整合が発生し得る。

- ゲートウェイが「サポートしている」と広告するバージョンを、logvalet
  バックエンドが実際には解釈できない（バックエンドは常にこの5バージョン
  固定であり、ゲートウェイ側だけを更新しても実体は変わらない）。
- 逆に、logvalet が実際にサポートしているにも関わらず、ゲートウェイの
  設定漏れでリストから欠落したバージョンを、クライアントが
  `-32022` として弾かれてしまう（バックエンドは対応できるのに、ゲートウェイの
  リストが古いままだと到達させてもらえない）。

運用上は、logvalet 自体の `go.mod` が固定する
`github.com/modelcontextprotocol/go-sdk` のバージョンを更新した際
（＝ `supportedProtocolVersions` の中身が変わり得るタイミング）に、
ゲートウェイ側の `supportedVersions` リスト設定も同時に見直す運用ルールを
徹底すること。ゲートウェイの設定が「差分適用」ではなく「全置換」である前提
なので、片方だけ更新した状態でのデプロイ（特に一時的なロールバック）を避ける。

## go build / go vet / go test

```
go build ./...
go vet ./...
go test ./...
```

いずれも green（`internal/mcp/version_negotiation_test.go` の V01〜V03 を含む）。
`-tags integration` を要するテストは本 issue 群に存在しない
（`go test -tags integration ./...` も通常の `go test ./...` と同じ結果になる）。
