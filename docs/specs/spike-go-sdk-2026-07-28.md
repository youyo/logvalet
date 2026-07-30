# スパイク調査: modelcontextprotocol/go-sdk v1.7.0 実測レポート

- 調査日: 2026-07-28（実行日 2026-07-30）
- 対象: `github.com/modelcontextprotocol/go-sdk` v1.7.0（proxy.golang.org で確認できた最新の安定版。v1.7.0-pre.1〜3 も存在するが正式リリースの v1.7.0 を採用）
- 目的: logvalet MCP サーバー再設計（issue #52）で公式Go SDKへの切り替えを検討するための事前調査。S15 の分岐判断（自前実装 継続 vs 公式SDK移行）の根拠とする。
- 検証コード: `spike/go-sdk-2026-07-28/`（logvalet 本体 go.mod とは独立した使い捨てモジュール。`spike/go-sdk-2026-07-28/go.mod` に SDK v1.7.0 のみを依存として追加、本体の `go.mod`/`go.sum` は未変更）
- 検証方法: `net/http/httptest` で `mcp.NewStreamableHTTPHandler` を起動し、SDKクライアントを介さず生の `net/http` リクエストを直接送って HTTP ステータス・レスポンスヘッダ・JSON-RPC ボディをそのままアサートした（9テスト、すべて green）。

## 実行結果サマリ

```
$ cd spike/go-sdk-2026-07-28 && go test -v ./...
--- PASS: TestA_StatelessDirectToolCall_NewProtocol (0.00s)
--- PASS: TestA_StatelessDirectToolCall_LegacyProtocol_RequiresInitialize (0.00s)
--- PASS: TestB_ServerDiscover (0.00s)
--- PASS: TestC_MissingProtocolVersionHeader_NewProtocolBody (0.00s)
--- PASS: TestC_MismatchedProtocolVersionHeader (0.00s)
--- PASS: TestC_MissingMcpMethodHeader (0.00s)
--- PASS: TestC_UnsupportedProtocolVersion (0.00s)
--- PASS: TestC_LegacyRequest_NoHeadersRequired (0.00s)
--- PASS: TestD_LegacyInitializeSessionFlow_SameHandlerSameServer (0.00s)
--- PASS: TestD_NewProtocolRejectedOnStatefulServer (0.00s)
PASS
ok  	spike/go-sdk-2026-07-28	0.545s
```

`go vet ./...`（spike モジュール内）もクリーン。

SDK が実際にサポートするプロトコルバージョン一覧（`supportedProtocolVersions`、新しい順）:
`2026-07-28`（最新, `latestProtocolVersion`）, `2025-11-25`, `2025-06-18`, `2025-03-26`, `2024-11-05`。

---

## (a) Stateless=true で initialize なしに直接 tools/call が通るか

### 判定: 条件付きで「はい」。ただし単に `Stateless: true` を付けただけでは不十分で、SEP-2575 の「新プロトコル」シグナリング（リクエストボディの `_meta["io.modelcontextprotocol/protocolVersion"]` + `Mcp-Protocol-Version` HTTPヘッダ）を併用したときだけ initialize レス呼び出しが成立する。

### 実測ログ

新プロトコルで直接 `tools/call`（`TestA_StatelessDirectToolCall_NewProtocol`）:

```
POST /  (Stateless: true, JSONResponse: true)
headers: Mcp-Protocol-Version: 2026-07-28, Mcp-Method: tools/call, Mcp-Name: echo
body._meta: {"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}

--> status=200
--> body={"jsonrpc":"2.0","id":1,"result":{
      "_meta":{"io.modelcontextprotocol/serverInfo":{"name":"spike-server","version":"v0.0.1-spike"}},
      "content":[{"type":"text","text":"{\"echo\":\"hello\"}"}],
      "structuredContent":{"echo":"hello"},
      "resultType":"complete"}}
```

initialize を一度も送らず、`tools/call` が 1 リクエストで成功した。

一方、`_meta.protocolVersion` を付けない**普通の（旧プロトコルの）** `tools/call` を `Stateless: true` サーバーへ initialize なしで直接送った場合（`TestA_StatelessDirectToolCall_LegacyProtocol_RequiresInitialize`）も、意外なことに 200 で成功した:

```
--> status=200
--> body={"jsonrpc":"2.0","id":1,"result":{"content":[...],"structuredContent":{"echo":"hello"}}}
```

これは SDK ソース（`streamable.go` の `serveStateless`）のコメント通り、Stateless モードでは「リクエストごとに一時セッションを作成する」実装になっており、旧プロトコルのリクエストであっても `initialize` を経ずに一時セッションが作られてハンドラが実行されるため。つまり `Stateless: true` 自体が「initialize 省略」を提供しており、SEP-2575 の `_meta` シグナリングは「initialize 省略」と「per-request の新プロトコル・ヘッダ検証（Mcp-Method/Mcp-Name/HeaderMismatch）」の両方を有効にするオプトインという棲み分け。

### 結論（S15向け）
- `Stateless: true` にすれば、旧プロトコル・新プロトコルいずれの `tools/call` も initialize なしで通る。
- ただし新プロトコル（`_meta.protocolVersion >= 2026-07-28`）は **`Stateless: true` のサーバーでのみ**サポートされる。stateful サーバーに新プロトコルのリクエストを送ると 400 になる（→ (d) 参照）。
- GET/DELETE は Stateless モードでは 405（`allowsessionsinstateless` 互換フラグを立てない限り）。

---

## (b) server/discover が protocolVersion / capabilities / identity を返すか

### 判定: はい。ただし REST の GET エンドポイントではなく **JSON-RPC メソッド** `"server/discover"` として実装されている（HTTP POST でボディに `"method":"server/discover"` を指定する）。

### 実測ログ

`TestB_ServerDiscover`:

```
POST / (Stateless: true)
body.method = "server/discover"
body.params._meta = {"io.modelcontextprotocol/protocolVersion":"2026-07-28", "io.modelcontextprotocol/clientCapabilities":{}}
headers: Mcp-Protocol-Version: 2026-07-28, Mcp-Method: server/discover

--> status=200
--> body={"jsonrpc":"2.0","id":1,"result":{
      "resultType":"complete",
      "_meta":{"io.modelcontextprotocol/serverInfo":{"name":"spike-server","version":"v0.0.1-spike"}},
      "ttlMs":0,
      "cacheScope":"public",
      "supportedVersions":["2026-07-28","2025-11-25","2025-06-18","2025-03-26","2024-11-05"],
      "capabilities":{"logging":{},"tools":{"listChanged":true}}}}
```

- protocolVersion 相当: `supportedVersions`（複数バージョンのリスト。単一の "protocolVersion" フィールドではない）
- capabilities: `capabilities` フィールドにサーバーの `ServerCapabilities` がそのまま入る
- identity: SDK トップレベルの `DiscoverResult` 構造体には identity フィールドは無いが、**新プロトコル（SEP-2575）で処理された全レスポンスに共通の仕組み**として `_meta["io.modelcontextprotocol/serverInfo"]`（`{name, version}`）が自動付与される（`annotateServerInfo` 関数、`server.go`）。discover もこの仕組みに乗っているため identity が取得できる。

### 追加の発見（重要）: discover の initialize 省略性 と stateful サーバーでの挙動差

`server/discover` は **常に新プロトコル扱い**（`_meta.protocolVersion` が必須。旧プロトコルの `server/discover` 呼び出しは `-32601 Method not found` になる、`server.go` の該当 switch 文より）で、かつ **stateful（`Stateless: false`）サーバーでも唯一の例外として許可される**（他の新プロトコル系メソッドは stateful サーバーで 400 になる。(d) 参照）。

さらに実測で判明した点として、stateful サーバー上での discover の `supportedVersions` は **`2026-07-28` を含まない**（`TestD_NewProtocolRejectedOnStatefulServer` の (d-5) ログ参照: `"supportedVersions":["2025-11-25","2025-06-18","2025-03-26","2024-11-05"]`）。これは discover がそのセッション（トランスポート）が実際にサポートできるバージョンだけを返すよう設計されているためで、クライアントは discover の結果を見て「このサーバー・このトランスポートでは新プロトコルが使えるかどうか」を判断できる、という設計意図が読み取れる。

### 結論（S15向け）
discover は「initialize なしでプロトコルバージョン一覧・capabilities・サーバー識別情報をまとめて取得する」という要求を満たすが、REST的な `GET /discover` ではなく JSON-RPC メソッド呼び出しである点は API 設計上の前提として要注意（MCP-Protocol-Version ヘッダと `Mcp-Method: server/discover` ヘッダは依然必要）。

---

## (c) ヘッダ要求とエラー条件（HeaderMismatch -32020 / UnsupportedProtocolVersionError -32022）

### 判定: 実測どおり。ヘッダ検証は **`Mcp-Protocol-Version` ヘッダが存在し、かつ `2026-07-28` 以上のときのみ**有効化される（`minVersionForStandardHeaders = protocolVersion20260728`）。旧プロトコルのリクエスト、あるいは `Mcp-Protocol-Version` ヘッダ自体を送らないリクエストには、`Mcp-Method`/`Mcp-Name` の要求は一切かからない。

### 実測ログと発火条件

1. **`Mcp-Protocol-Version` ヘッダ欠落 + ボディに `_meta.protocolVersion` あり** → `-32020`
   ```
   (c-1) status=400
   body: {"error":{"code":-32020,"message":"Mcp-Protocol-Version header is required for requests carrying \"io.modelcontextprotocol/protocolVersion\""}}
   ```

2. **ヘッダとボディの protocolVersion 不一致** → `-32020`
   ```
   (c-2) status=400  (header=2025-11-25, body._meta.protocolVersion=2026-07-28)
   body: {"error":{"code":-32020,"message":"Mcp-Protocol-Version header \"2025-11-25\" does not match request io.modelcontextprotocol/protocolVersion \"2026-07-28\""}}
   ```

3. **`Mcp-Protocol-Version: 2026-07-28` はあるが `Mcp-Method` ヘッダが無い** → `-32020`
   ```
   (c-3) status=400
   body: {"error":{"code":-32020,"message":"missing required Mcp-Method header"}}
   ```
   （`Mcp-Name` も `tools/call`/`resources/read`/`prompts/get` では同様に必須。欠落・不一致でいずれも `-32020`。ソース上は `Mcp-Method` の値がボディの `method` と一致しない場合、`Mcp-Name` の値がボディの `name`/`uri` と一致しない場合も同じ `-32020` パスに入る。）

4. **`_meta.protocolVersion` に SDK が知らないバージョン文字列を指定** → `-32022`
   ```
   (c-4) status=400 (header=body=2099-01-01)
   body: {"error":{"code":-32022,"message":"unsupported protocol version",
         "data":{"supported":["2026-07-28","2025-11-25","2025-06-18","2025-03-26","2024-11-05"],"requested":"2099-01-01"}}}
   ```
   `data.supported` にサーバーがサポートするバージョン一覧が構造化データ（SEP-2575 の `UnsupportedProtocolVersionData`）として返る。

5. **旧プロトコル（`Mcp-Protocol-Version` ヘッダなし、`_meta` なしの通常 `initialize`）** → ヘッダ要求は一切かからず 200 で成功
   ```
   (c-5) status=200
   body: {"result":{"capabilities":{...},"protocolVersion":"2025-11-25","serverInfo":{...}}}
   ```

### 結論（S15向け）
- `Mcp-Method`/`Mcp-Name` ヘッダは **新プロトコル専用**の追加要件であり、既存の 2025-xx 系クライアント・自前実装には一切影響しない（後方互換性は保たれる）。
- `-32020`（HeaderMismatch）は「ヘッダとボディの不整合・不足」全般に使われる包括的なエラーコードで、(c-1)〜(c-3) はすべて同じコードを返す（メッセージ文字列でしか区別できない）。
- `-32022`（UnsupportedProtocolVersionError）はバージョン文字列そのものが未知のときのみで、`data.supported` に候補一覧が構造化されて載るため、クライアント側のフォールバック実装がしやすい。

---

## (d) 旧バージョン（2025-11-25 等）とのプロトコルネゴシエーションをSDKが代行するか / 旧initialize・sessionベースのserver型を並行提供するか

### 判定: **はい、代行する。新プロトコル(SEP-2575)対応と旧initialize/sessionベースの対応は、同一の `mcp.Server` / `mcp.NewStreamableHTTPHandler` インスタンス上で完全に並行稼働する。** 「新旧で別のサーバー型を用意する」必要は無く、SDK 内部で1リクエストごとにモード判定（`validateRequestMeta` による `usesNewProtocol` フラグ）をしている。これは S15 の分岐判断において最も重要な事実。

### 実測: 同一ハンドラでの旧initialize/session フロー全体

`TestD_LegacyInitializeSessionFlow_SameHandlerSameServer`（`Stateless` 未設定 = stateful/旧来のセッションベース構成）:

```
1) POST /  {"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{...}}}
   --> status=200, Mcp-Session-Id: XDY6TXDVC5L7M4IXWX5HPKBN4U
   --> result.protocolVersion = "2025-11-25"   (クライアント要求どおりの旧バージョンをそのまま negotiate)

2) POST /  {"method":"notifications/initialized"}  (header: Mcp-Session-Id)
   --> status=202

3) POST /  {"method":"tools/call","params":{"name":"echo","arguments":{"message":"legacy-flow"}}}
   (header: Mcp-Session-Id)
   --> status=200
   --> result.structuredContent = {"echo":"legacy-flow"}
```

`initialize` → `notifications/initialized` → `Mcp-Session-Id` ヘッダによるセッション継続 → `tools/call` という **MCP 2025-11-25 (streamable HTTP) 仕様どおりの旧来フロー**が、コード変更なし・型変更なしでそのまま動作した。SDK が要求バージョンをそのまま `protocolVersion` として応答しており（ダウングレード拒否なし）、クライアントが `2025-06-18` や `2024-11-05` を指定した場合も同様に動くはず（SDK テストコード `streamable_test.go` に `2024-11-05`/`2025-06-18` の initialize テストが多数存在することからも裏付けられる）。

### 実測: 新旧プロトコルの排他性（新プロトコルは stateless 限定）

`TestD_NewProtocolRejectedOnStatefulServer`:

```
(d-4) stateful サーバーへ新プロトコル(_meta.protocolVersion=2026-07-28)の tools/call
--> status=400
--> body: "Bad Request: protocol version \"2026-07-28\" is only supported on stateless HTTP servers (set StreamableHTTPOptions.Stateless = true)"

(d-5) 同じ stateful サーバーへ server/discover（新プロトコル・唯一の例外）
--> status=200
--> supportedVersions=["2025-11-25","2025-06-18","2025-03-26","2024-11-05"]  (2026-07-28 を含まない)
```

つまり実運用上のトポロジは次の3通りになる:

| サーバー設定 | 旧プロトコル (initialize+session) | 新プロトコル (_meta sessionless) |
|---|---|---|
| `Stateless: false`（stateful, デフォルト） | ○ フル機能 | × (`server/discover` のみ例外的に応答可) |
| `Stateless: true` | ○ （ただし一時セッション扱い。GET/DELETE不可） | ○ フル機能 |

### SDK内部実装の要点（なぜ「代行」と言えるか）

- `mcp/shared.go` の `validateRequestMeta` が JSON-RPC リクエストごとに `_meta["io.modelcontextprotocol/protocolVersion"]` の有無・値を見て `usesNewProtocol` を決定する。存在しない、または `2026-07-28` 未満なら旧プロトコル扱い。
- 旧プロトコル扱いのリクエストは、これまで通り `Session.InitializeParams` に基づくセッション初期化必須の分岐（`ServerSession.handle` 内の `switch req.Method` ）を通る。
- `supportedProtocolVersions`（降順: 2026-07-28, 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05）に対する `initialize.protocolVersion` の照合・不一致時のフォールバック処理もクライアント側 API（`mcp.Client.Connect`）に実装されており、`CodeUnsupportedProtocolVersion` を受け取った場合は `Data.Supported` を見て別バージョンで再試行するロジックがある（`client.go` 360行目付近, `streamable_client_test.go` の `TestStreamableClientConnect_DiscoverUnsupportedProtocolVersion` 系テストで確認できる。今回のスパイクでは実測していないが、ソース上明確に存在する）。
- つまり「新プロトコルの世界に合わせて自前でネゴシエーションコードを書く」必要はなく、SDK が両プロトコルの受付・応答・（クライアント側では）フォールバックまで面倒を見る。ロジックの二重実装（旧server型 + 新server型）は不要。

### 結論（S15向け、詳細）

1. **公式SDKへ移行しても、logvalet が現在サポート中/想定している旧世代クライアント（2025-11-25 以前）との互換性は失われない。** 同一の `mcp.Server`/`mcp.NewStreamableHTTPHandler` がそのまま両対応する。
2. **新プロトコル（sessionless, SEP-2575）を有効化するには `StreamableHTTPOptions.Stateless: true` が前提**。現行 logvalet MCP サーバーが将来的にステートレス化（サーバーレス/複数レプリカ運用）を検討しているなら、この移行と新プロトコル対応は事実上セットになる。stateful のまま新プロトコルの恩恵（initialize省略等）だけを得ることはできない。
3. **`server/discover` は stateful/stateless どちらでも呼べる唯一の新プロトコル系メソッド**であり、クライアントが「このサーバーは新プロトコルに対応しているか」を安全にプローブする経路として設計されている（実際、SDK 内の `streamable_test.go` コメントに「2026-07-28 下ではクライアントは discover プローブを行う」旨の記述が複数箇所ある）。
4. ヘッダ要件（`Mcp-Method`/`Mcp-Name`、`-32020`/`-32022`）は新プロトコル選択時のみ課される追加の検証層であり、既存の実装・クライアントへの後方互換破壊は無い。
5. リスクとして把握しておくべき点: 新プロトコルの一時セッション実装（`serveStateless`）は「リクエストごとに使い捨てセッションを作る」方式であるため、`notifications/*` のようなサーバー→クライアントのプッシュ通知や、複数リクエストにまたがる状態（progress token 等）を前提とする機能は、新プロトコルでは設計が変わる可能性がある。今回のスパイクではその点まで検証していない（スコープ外）。

---

## 未検証・スコープ外の事項（S15判断時の留意点）

- SSE ストリーミング応答（`text/event-stream`、`JSONResponse: true` を外した場合の挙動）は未検証。今回はレスポンス比較を単純化するため全テストで `JSONResponse: true` を使用した。
- クライアント側（`mcp.Client`）の実際のフォールバック挙動（`CodeUnsupportedProtocolVersion` 受信後の自動リトライ）はソースコードの確認に留まり、実行テストはしていない。
- 認証・認可（OAuth, `RequireBearerToken` 等）まわりは本スパイクの対象外。
- goroutine リーク（SDK の `ExampleStreamableHTTPHandler` コメントにある既知の issue #499）など、長期運用時の安定性は未調査。

## 再現手順

```bash
cd spike/go-sdk-2026-07-28
go test -v ./...
go vet ./...
```

（本体 `logvalet` の `go.mod`/`go.sum` には一切変更なし。`spike/go-sdk-2026-07-28/go.mod` が独立したモジュールとして SDK v1.7.0 のみを依存に持つ。）
