# Gateway ⇔ logvalet 間リクエスト契約

- 作成日: 2026-07-30
- 対象 issue: #52（再設計ロードマップ）ステップ S04
- 位置づけ: M1（検証スパイク）の一部。**AgentCore Gateway / AgentCore Identity の実機構築・end-to-end 検証は本リポジトリでは行わない**（2026-07-30 ユーザー決定・決定C）。実機構築はユーザーが別リポジトリで行う。本文書は logvalet 側がサーバ実装（S14 / S21 / S30）で参照するヘッダー名・値形式・エラー応答仕様を確定するための契約であり、Gateway 側の実装がこの契約に一致するかどうかの検証は実機構築側リポジトリの責務とする。
- 根拠資料:
  - S03 スパイク実測: `docs/specs/spike-go-sdk-2026-07-28.md`（ブランチ `voyager/mcp-impl/S03-a1`）
  - issue #52 本文の「採用アーキテクチャ」「制約」節
  - `github.com/modelcontextprotocol/go-sdk` v1.7.0 ソース（`design/mrtr.md`, `mcp/shared.go`, `mcp/protocol.go`, `mcp/client.go`, `mcp/mrtr.go`）

## 0. 全体構成（再掲）

```
MCP Client (2026-07-28)
  → AgentCore Gateway            … Entra ID JWT 検証・ユーザー認可をここで完結
  → logvalet MCP server          … 認証は none | apikey のみ。identity はヘッダー受領
  → Backlog API                  … per-user OAuth トークン（AgentCore Identity token vault
                                    が取得・保管・リフレッシュし、Gateway が
                                    Authorization: Bearer で注入。logvalet は passthrough）
```

複数ユーザー利用が大前提（決定D）。単一テナント固定 userID へのフォールバックは実装しない。per-user の
Backlog トークン紐付けは AgentCore Identity の **iss + sub × workload identity スコープ** で実現される。
logvalet はこのスコープ分離の実装を持たず、Gateway が渡す identity ヘッダーと Bearer トークンを
そのまま信頼して転送・区別するだけである。

以下、Gateway → logvalet 方向のリクエストに logvalet 側が要求するヘッダー契約を定義する。

---

## 1. apikey ヘッダー（Gateway → logvalet の service-to-service 認証）

### 1.1 目的

apikey は **Gateway を認証する共有鍵であり、エンドユーザーを認証しない**（issue #52 の設計原則）。
AgentCore Gateway の outbound API key credential provider が、Gateway → logvalet 間のすべてのリクエストに
この鍵を注入する前提を置く。

### 1.2 ヘッダー名（確定・既存実装からの破壊的変更）

```
X-Logvalet-Api-Key: <static-token>
```

**`Authorization` ヘッダーは使用しない。** 現行実装（`internal/cli/mcp_bearer.go`）は
`--auth-mode=bearer` で `Authorization: Bearer <token>` を検証しているが、この方式は §2 で定義する
Backlog credential の Bearer passthrough（同じく `Authorization: Bearer` を使う）と衝突する。
本再設計では apikey 用に専用ヘッダー `X-Logvalet-Api-Key` を新設し、`Authorization` ヘッダーは
Backlog credential passthrough 専用に予約する（§2.4 で分離を明記）。

`--auth-mode=bearer` は `apikey` の別名として CLI フラグ名は維持するが（issue #52 の破壊的変更一覧に
明記の通り）、実装（S20/S21）ではヘッダー検証先を `Authorization` から `X-Logvalet-Api-Key` へ切り替える。
これは S04 時点での文書上の決定であり、実コードの変更は S20/S21 で行う。

### 1.3 値の形式

- 平文の静的トークン文字列。既存 `--bearer-token` と同じ制約を踏襲する: 最小 32 文字。
- 大小文字を区別する完全一致比較。`crypto/subtle.ConstantTimeCompare` 等タイミング攻撃耐性のある比較を
  用いる（既存 `mcp_bearer.go` の実装方針を踏襲）。
- スキームプレフィックス（`Bearer` 等）は付けない。ヘッダー値そのものがトークン。

### 1.4 検証失敗時の応答

- ヘッダー欠落・値不一致: `401 Unauthorized`、`WWW-Authenticate` ヘッダーは付与しない（Bearer スキームでは
  ないため RFC 6750 の対象外）。ボディは spec §9 のエラーエンベロープ形式（JSON, stdout ではなく
  HTTP レスポンスボディ）。
- `--auth-mode=none` の場合はこの検証自体をスキップする（apikey ヘッダーの有無に関わらず全リクエスト許可）。
  `none` は開発・信頼済みネットワーク限定での利用を想定し、本番の Gateway 経由運用では常に `apikey` を使う。

### 1.5 apikey とヘルスチェックの扱い

既存実装同様、`/healthz` 相当のエンドポイントは apikey 検証の対象外とする（`mcp_bearer_e2e_test.go` の
`TestE2E_BearerAuth_HealthzNoToken` の方針を踏襲）。

---

## 2. identity ヘッダー（Gateway → logvalet のエンドユーザー識別伝達）

### 2.1 目的

logvalet は Entra ID JWT を自ら検証しない（Gateway が検証を完結する）。しかし複数ユーザー利用が大前提
（決定D）であるため、logvalet 側にも「このリクエストは誰の操作か」を示す識別子が必要になる場面がある
（例: space registry の per-user スコープ、監査ログ、将来的なレート制限）。この識別子を Gateway が
ヘッダーで渡す。

### 2.2 ヘッダー名・値の形式（想定・logvalet 側の期待仕様）

```
X-Logvalet-Identity-Issuer: <Entra ID issuer URL>
X-Logvalet-Identity-Subject: <Entra ID sub claim>
```

2 ヘッダーに分離する。理由: per-user 紐付けは「iss + sub」の組で一意性を担保する設計（決定D）であり、
`sub` 単体では複数テナント・複数 issuer 運用時に衝突しうる。1 ヘッダーに `iss|sub` 等の区切り文字で
連結する案も検討したが、区切り文字のエスケープ規則が増えるため 2 ヘッダー分離を採用した。

logvalet 側の内部 userID（`auth.ContextWithUserID` に渡す値）は、この 2 値を正規化して連結した文字列
（例: `sha256(iss + "\x00" + sub)` の16進表現、または `iss + "#" + sub` の生値）とする。具体的な正規化
関数の実装は S21 のスコープとする。本文書では「2 つの値が独立したヘッダーとして渡される」ことのみを契約
として固定する。

**注意（未確定・実機検証が必要）:** AgentCore Gateway が実際にどのヘッダー名で identity を注入できるか
（Gateway 側のテンプレート機能でカスタムヘッダー名を設定できるか、あるいは AWS 標準ヘッダー名が
別途存在するか）は本リポジトリでは検証していない。上記ヘッダー名は **logvalet が受理する契約上の名称**
であり、Gateway 側の設定でこの名称にマッピングされることを前提とする。Gateway 標準のヘッダー名が
この名称と異なる場合、Gateway 側リポジトリでの実機検証後にリネームする可能性がある（§6 残余項目）。

### 2.3 付与条件

- **apikey 検証を通過したリクエストでのみ信用する。** apikey 検証前・失敗時に identity ヘッダーの値を
  一切参照してはならない（なりすまし対策の第一層。issue #52 設計原則の直接反映）。
- identity ヘッダーが欠落しているが apikey 検証は成功したリクエスト（例: Gateway 設定ミス、または
  ユーザーに紐付かないシステム間呼び出し）は、`auth.UserIDFromContext` が `ok=false` を返す状態として
  扱う。この状態でユーザースコープが必須のツール（space 登録操作等）を呼ぶとツールエラーを返す
  （既存 `tools.go:165,224` の `UserIDFromContext` 未設定時の分岐を踏襲）。単一テナント固定 userID への
  フォールバックは行わない（決定D）。

### 2.4 strip-and-replace 前提

**logvalet は identity ヘッダーを Gateway 経由の private ingress からのみ受理できる、という前提の上で
初めて安全になる。** apikey は共有鍵でありユーザーを認証しないため、apikey を知る任意のクライアントが
`X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` を直接偽装して送れば、apikey 検証だけでは
なりすましを防げない。

したがって Gateway 側の実装契約として以下を要求する（実機構築時の要件、S28 で構築者向け参考ドキュメント
として詳細化）:

1. Gateway は自身が Entra ID JWT から検証・抽出した iss/sub の値で、クライアントが送ってきた
   identity ヘッダーを **常に上書き（strip-and-replace）** した上で backend（logvalet）へ転送する。
   クライアントが直接これらのヘッダーを設定していた場合でも、Gateway が必ず自分の検証結果で上書きする
   ことが前提。
2. logvalet backend への直接到達経路（Gateway を経由しない経路）が存在しないこと（private ingress /
   ネットワーク境界で保証する。S28 のスコープ）。
3. apikey の rotation / revocation 運用（S28）。

この3点が揃って初めて identity ヘッダーの値を信頼できる。(1) は Gateway 実機構築側の責務、(2)(3) は
S28 で logvalet リポジトリ側が構築者向け参考ドキュメントとして記述する。**現時点（S04）では (1) が
Gateway 側で実際にそう振る舞うことを実機で確認していない**（§6 残余項目）。

---

## 3. Mcp-Method / Mcp-Name / MCP-Protocol-Version の透過要件

### 3.1 S03 実測の要約

公式 SDK（v1.7.0）は、リクエストの `Mcp-Protocol-Version` ヘッダーが存在し、かつ値が `2026-07-28` 以上
のときに限り、`Mcp-Method` / `Mcp-Name` ヘッダーの存在とボディ（`method` / `name` または `uri`）との
一致を検証する（`minVersionForStandardHeaders`）。この条件を満たさないリクエスト（`Mcp-Protocol-Version`
ヘッダーが無い、または `2025-11-25` 以下）は、この3ヘッダーに関する検証を一切受けない
（S03 spike の (c) 節、`TestC_LegacyRequest_NoHeadersRequired`）。

検証失敗時は `HeaderMismatch (-32020)`（ヘッダー欠落・不一致・`Mcp-Method`/`Mcp-Name` とボディの不一致は
すべてこのコードに集約される）、未知のプロトコルバージョン指定時は
`UnsupportedProtocolVersionError (-32022)`（`data.supported` に対応バージョン一覧が構造化データとして
返る）。

### 3.2 Gateway への透過要件（契約）

**Gateway は `Mcp-Protocol-Version` / `Mcp-Method` / `Mcp-Name` の3ヘッダーを、クライアントが送った値
そのままバイト単位で logvalet backend へ転送しなければならない（追加・削除・書き換え禁止）。**

理由:

- これら3ヘッダーは MCP wire protocol の一部であり、認証・認可とは無関係（Gateway の役務対象外）。
- ヘッダー検証の要否はリクエストごとに SDK backend 側が `Mcp-Protocol-Version` の値で自律的に判断する
  （§3.1）。Gateway 側でこれらのヘッダーを一律付与・削除する必要は無く、むしろ触ると壊れる。
- Gateway が誤ってこれらのヘッダーを剥がすと、新プロトコル（2026-07-28）のクライアントからのリクエストが
  すべて `-32020 HeaderMismatch` で失敗する（旧プロトコルのリクエストは元々ヘッダー検証対象外なので
  影響を受けない）。

### 3.3 未確定事項（実機検証が必要）

「AgentCore Gateway が実際にこれら3ヘッダーを透過するか」は本リポジトリでは検証していない（issue #52
「未確認事項とスパイクの対応表」に明記の通り、S04 は文書化のみで実機検証は対象外）。透過しないことが
実機検証で判明した場合の対応方針（S14 の分岐先）を以下に明記する:

- **Gateway 側の設定でカスタムヘッダーの透過を有効化できる場合**: Gateway 側リポジトリで設定し、
  logvalet 側の実装（S14）は変更不要。
- **Gateway が構造的にこれら3ヘッダーを透過できない場合**: logvalet backend 側で
  `Mcp-Protocol-Version` ヘッダーが欠落していても、ボディの `_meta["io.modelcontextprotocol/protocolVersion"]`
  が存在すれば新プロトコルとして扱うカスタムミドルウェアを追加検討する（公式 SDK の標準動作を上書きする
  ため追加実装コストが発生する）。この分岐の要否は Gateway 実機検証の結果を待って S14 で確定する。

---

## 4. Backlog credential の Bearer passthrough 注入仕様

### 4.1 ヘッダー名・トークン形式

```
Authorization: Bearer <backlog-oauth-access-token>
```

`<backlog-oauth-access-token>` は AgentCore Identity の CustomOauth2 credential provider（3LO）が
取得・保管・リフレッシュする Backlog OAuth2 access token をそのまま渡す（不透明文字列。logvalet 側は
形式を検証しない）。

**この `Authorization` ヘッダーは §1 の apikey ヘッダーとは完全に別チャンネルである。** apikey は
`X-Logvalet-Api-Key` を使うため、`Authorization` ヘッダーは Backlog credential passthrough 専用に
予約される（§1.2 で述べた破壊的変更の理由）。

### 4.2 付与タイミング

Gateway は、logvalet へのリクエストを転送する **都度**（リクエストごとに）AgentCore Identity から
最新の（必要ならリフレッシュ済みの）Backlog access token を取得し、`Authorization` ヘッダーへ注入する。
トークンのリフレッシュ・失効管理は完全に AgentCore Identity 側の責務であり、logvalet は一切関与しない
（決定E）。logvalet は tokenstore の DynamoDB バックエンドを持たず（決定F）、HTTP モードでは Backlog
トークンを一切永続化しない。

### 4.3 logvalet 側の処理仕様（S30 実装契約）

logvalet の HTTP MCP サーバーは、受信した `Authorization: Bearer <token>` ヘッダーの値を
**そのまま** Backlog API 呼び出しの `Authorization: Bearer <token>` ヘッダーとして転送する
（passthrough）。logvalet 自身はこのトークンを検証・デコード・キャッシュしない。

- `Authorization` ヘッダーが欠落している状態で Backlog API 呼び出しが必要なツールが呼ばれた場合、
  logvalet は Backlog API 呼び出しを試みずに即座にツールエラーを返す（Backlog API 側の 401 を待たない
  fail-fast。既存 `needsAuthorization` 判定の代替として、HTTP モードでは「Authorization ヘッダー無し」
  自体を authorization-required 相当の状態として扱う）。
- Backlog API がトークン失効・スコープ不足で 401/403 を返した場合は、Backlog API のエラーをそのまま
  logvalet のツールエラーとして forward する（§5 でこの場合のレスポンス形式を定義する）。

### 4.4 CLI/stdio モードとの違い

CLI/stdio モードは本契約の対象外（Gateway を経由しない）。stdio はローカルクレデンシャルキャッシュ
（sqlite / tokens.json、決定F）を使う直接 OAuth を継続し、`internal/auth/provider` /
`internal/auth/manager.go` の既存フローをそのまま使う。本 §4 は HTTP（Gateway 経由）モードにのみ適用
される。

---

## 5. Backlog 未同意時に返す authorization URL フロー

### 5.1 現行実装（変更前の挙動、参考）

現行 logvalet（`internal/mcp/tools.go:288-334`）は、`needsAuthorization(err)` が真になった場合に
`toolResultAuthRequired` を呼び、isError な `CallToolResult` に以下の `_meta` を付与して返す:

```json
{
  "isError": true,
  "content": [{"type": "text", "text": "Backlog authorization required. Open the following URL ..."}],
  "_meta": {
    "authorization_required": true,
    "authorization_url": "https://..."
  }
}
```

この URL は logvalet 自身が `internal/auth/provider.BuildAuthorizationURL` で構築した、logvalet 自前の
OAuth コールバックへの導線である。これは mark3labs SDK 上の実装であり、MRTR（SEP-2322）以前の
ad-hoc な独自 `_meta` キー方式であって、公式 SDK の `InputRequests` / `resultType` とは無関係な
別チャンネルである。

### 5.2 決定Eによる変更: HTTP/Gateway モードでは logvalet 自身の authorization_url 構築が無くなる

決定E（Backlog OAuth の AgentCore Identity への完全委譲）と制約(c)（初回同意用の公開 HTTPS
session-binding callback は Gateway 側リポジトリに配置し logvalet 側には置かない）により、
**HTTP/Gateway モードでは logvalet は Backlog の authorization URL を自分で構築する手段を持たなくなる**
（`internal/auth/provider.BuildAuthorizationURL` を呼び出す経路自体が S30 で HTTP モードから削除される
想定）。同意フローの起点は Gateway / AgentCore Identity 側にあり、以下のいずれかの形でクライアントに
届く:

1. **Gateway 層で完結するケース（未確認）**: AgentCore Identity が未同意を検知し、Gateway が
   logvalet へリクエストを転送する前に、Gateway 自身が MCP クライアントへ同意 URL を含むエラー
   （もしくは MRTR 準拠の `input_required` 応答）を返す。この場合 logvalet は当該リクエストを一切
   受け取らない。
2. **logvalet まで到達するケース**: 何らかの理由で Gateway が未同意のまま `Authorization` ヘッダー無し
   （または無効なトークン付き）でリクエストを転送してしまう。この場合 logvalet は §4.3 の fail-fast
   仕様に従い、**logvalet 自身が構築した authorization_url を含まない、単純なツールエラー**を返す
   （例: `"Backlog access is not authorized for this session. Complete Backlog consent via the Gateway/AgentCore Identity flow and retry."`）。logvalet はこのメッセージにリンクを含めることができない
   （Gateway 側の同意 URL を知らないため）。

### 5.3 MRTR / InputRequiredResult との対応関係

公式 SDK（M4, S12-S17 で導入予定）は SEP-2322（MRTR）で「サーバーが追加入力を要求する」ための標準
チャンネルを持つ（`design/mrtr.md`, `mcp/mrtr.go`）:

- `CallToolResult` に `InputRequests`（`map[string]InputRequest`）と `RequestState`（文字列トークン）を
  設定し、`Content` は設定しない（両方設定すると SDK がランタイムエラーにする）。
- 新プロトコル（`2026-07-28`）のクライアントに対しては `resultType: "input_required"` として wire に
  乗る。クライアントは `inputRequests` をローカルで解決し、同じ `requestState` を添えて同一呼び出しを
  リトライする。
- `InputRequest` は `*mcp.ElicitParams` / `*mcp.CreateMessageParams` / `*mcp.ListRootsParams` の
  いずれか（sealed interface）。
- **URL 型の elicitation** が SDK に既に存在する: `ElicitParams{Mode: "url", URL: "...", ElicitationID: "..."}`。
  さらに `URLElicitationRequiredError` という **事前拒否（プレフライト）用の JSON-RPC エラー**
  （コード `-32042`, `CodeURLElicitationRequired`）も存在し、こちらは `InputRequests` を使わずに
  「このツール呼び出しの前に URL を開いて完了してから同一リクエストをリトライしてください」という
  意味のエラー応答として使える。

これは Backlog 未同意時の「ブラウザで URL を開いて認可を完了してから再試行する」というフローと構造的に
一致する。**もし将来、Gateway/AgentCore Identity が logvalet に対して「このユーザーは未同意であり、
同意 URL は `https://...`」という情報を（何らかのヘッダーまたはボディで）伝達できるようになれば**、
logvalet は公式 SDK 移行後（M4 以降）に以下のいずれかの形で `_meta.authorization_url`
（旧: ad-hoc）を置き換えられる:

- `ElicitParams{Mode: "url", URL: <gateway が伝えた同意 URL>}` を `InputRequests` に載せた
  `input_required` 結果（MRTR、非エラー・リトライ可能）
- 同じ `ElicitParams` を `URLElicitationRequiredError`（`-32042`）に載せたプレフライトエラー
  （ツール呼び出し自体をエラーとして即時拒否する形）

どちらを採用するかは「Gateway が同意 URL を logvalet に伝達できるか」という §5.2 の未確認事項に依存する
ため、**本 S04 時点では確定しない**。伝達できないことが実機検証で判明した場合（可能性が高いと想定される
——制約(c)により同意コールバックは Gateway 側にあり、logvalet が同意 URL を知る積極的な理由が無いため
——）、logvalet の HTTP モードは §4.3/§5.2-2 の「URL を含まない単純なツールエラー」を恒久的な仕様として
採用する。

### 5.4 CLI/stdio モードでは現行の `_meta.authorization_url` を維持

決定Fにより stdio はローカル OAuth を継続するため、§5.1 の現行実装（`toolResultAuthRequired`,
`internal/auth/provider.BuildAuthorizationURL`）は **stdio モードに限定して維持する**。stdio は
Gateway を経由せず、MRTR ともローカルブラウザ起動フローとも別の話（プロセス内蔵のローカル webserver
コールバック）であるため、本契約の変更対象外である。

---

## 6. スコープ外の明記と実機検証で確定すべき残余項目

### 6.1 スコープ外の明記

AgentCore Gateway / AgentCore Identity の実機構築・end-to-end 検証は logvalet リポジトリでは実施しない
（2026-07-30 ユーザー決定・決定C）。Terraform 等のインフラコード、Gateway の実際のデプロイ、実際の
Entra ID テナントに対する認可フローの動作確認は、ユーザーが別リポジトリで行う。logvalet 側は
「構築する人向けの参考ドキュメント」を S28/S29 で用意する程度に留める。本文書はその参考ドキュメントが
実装として参照できる具体性を持つ「契約」であり、実機で検証済みの事実ではない箇所は本節に列挙する。

### 6.2 残余項目一覧（実機検証で確定すべき事項）

| # | 未確定事項 | 本文書での暫定契約 | 実機検証で否定された場合の影響 |
|---|---|---|---|
| 1 | Gateway が identity ヘッダーをどのヘッダー名で注入できるか（§2.2） | `X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` を logvalet 側の契約名として固定 | logvalet 側の受理ヘッダー名を Gateway の実際の注入名に合わせてリネームする（S21 のヘッダー名を差し替えるだけで済み、設計自体への影響は無い） |
| 2 | Gateway が identity ヘッダーを strip-and-replace する実装になっているか（§2.4 (1)） | Gateway 側が必ず上書きすることを前提に、logvalet はヘッダー値をそのまま信頼する | strip-and-replace されない場合、apikey を知る任意のクライアントによる identity 偽装がネットワーク境界（§2.4 (2)）だけでは防げなくなる。S28 で追加の防御層（例: S22 の JWT パススルー検証を無条件で必須化する）を検討する必要がある |
| 3 | Gateway が `Mcp-Protocol-Version` / `Mcp-Method` / `Mcp-Name` を透過するか（§3.3） | 透過される前提で S14 を実装 | 透過されない場合、§3.3 のカスタムミドルウェア分岐を S14 で追加実装する |
| 4 | Gateway が元の Entra ID JWT を backend（logvalet）へも転送できるか | 転送できる場合のみ S22（JWT パススルー検証、多層防御・既定 off）を実施する（issue #52 の条件） | 転送できない場合 S22 は実施しない。identity ヘッダーへの信頼が §2.4 の3条件のみに依存する |
| 5 | Backlog scope なし認可の通過可否 | 対象外（logvalet はスコープ検証を行わない。Backlog API からの 403 をそのまま forward するのみ） | 影響なし（logvalet 側の実装変更は不要） |
| 6 | refresh token rotation 競合時の挙動 | 対象外（AgentCore Identity の責務。logvalet はリクエストごとに渡された `Authorization` ヘッダーをそのまま使うだけで、競合状態を意識しない） | 影響なし |
| 7 | 未同意（consent 未完了）時に MCP がどのエラー形式を返すか（§5.2, §5.3） | §4.3/§5.2-2 の「URL を含まない単純なツールエラー」を暫定仕様とする | Gateway が同意 URL を logvalet に伝達できることが判明すれば、§5.3 の MRTR/URL elicitation 経路へ移行を検討する（M4 以降の追加ステップとして起票） |
| 8 | Entra ID の `sub` と Gateway 側 userId の対応関係 | §2.2 の `iss` + `sub` の組をそのまま logvalet 内部 userID の正規化元とする | 対応関係が想定と異なる場合、S21 の正規化関数のみ差し替える |

### 6.3 本文書が確定させた事項（実装がそのまま参照してよい契約）

- apikey ヘッダー名: `X-Logvalet-Api-Key`（`Authorization` は使わない、§1.2）
- Backlog credential ヘッダー: `Authorization: Bearer <token>`, per-request 注入, logvalet は
  passthrough のみ（§4）
- apikey 検証を通過しない限り identity ヘッダーを信用しない（§2.3）
- HTTP モードの Backlog 未認可時デフォルト挙動: `Authorization` ヘッダー欠落は fail-fast、Backlog API の
  401/403 はそのまま forward、logvalet 自身は authorization_url を構築しない（§4.3, §5.2）
- stdio モードは現行の `_meta.authorization_url` フローを変更なく維持する（§5.4）
