# AgentCore Gateway 構築者向け参考ドキュメント (S28)

- 作成日: 2026-07-30
- 対象 issue: #52（再設計ロードマップ）ステップ S28
- 位置づけ: 本ドキュメントは logvalet リポジトリが提供する「Gateway を実際に構築する人向けの
  参考資料」である。AgentCore Gateway / AgentCore Identity の Terraform 実装・実機構築・
  end-to-end 攻撃テストは本リポジトリの責務ではない（2026-07-30 ユーザー決定）。それらは
  ユーザーが別リポジトリで行う。本ドキュメントは logvalet 側が確定させた契約
  (`docs/specs/gateway-request-contract.md`) と、SDK が固定するプロトコルバージョン挙動
  (`docs/specs/legacy-protocol-decision.md`) を前提に、Gateway 側で満たすべき要件を列挙する
  ものであり、Gateway 側の実装がここに一致するかどうかの検証・保証は行わない。
- 必読の前提資料:
  - `docs/specs/gateway-request-contract.md`（logvalet が要求するヘッダー契約全体）
  - `docs/specs/legacy-protocol-decision.md`（プロトコルバージョン別の挙動、§「AgentCore Gateway
    運用上の注意」）
  - issue #52 本文「採用アーキテクチャ」節・「AgentCore Identity 委譲に伴う制約事項」節 (a)〜(c)

---

## (a) backend を Gateway からのみ到達可能にする private ingress / security group

logvalet backend（HTTP MCP サーバー）は `--auth-mode=apikey` による共有鍵検証を持つが、
これは **Gateway を認証する**ものであり、backend への到達経路そのものを制限するものではない
（`gateway-request-contract.md` §1.1, §2.4）。apikey は一度漏洩・共有されれば任意のクライアントが
使える静的トークンであるため、apikey 検証だけを「backend が保護されている」根拠にしてはならない。

Gateway 構築者は以下を満たすこと:

1. logvalet backend を、Gateway が動作するネットワーク（VPC 等）からのみ到達可能な private
   ingress として構成する。パブリックインターネットから直接 backend の HTTP エンドポイントへ
   到達できる経路を作らない。
2. security group / ネットワーク ACL で、ソースを Gateway 側のみに限定する（Gateway の
   outbound IP レンジ、または Gateway が動作するサブネット/セキュリティグループを送信元とする
   許可ルールのみを設定し、それ以外を拒否する）。
3. この network-level の到達制限は、後述 (b) の identity ヘッダー strip-and-replace が
   安全に機能するための前提条件である（`gateway-request-contract.md` §2.4 の3条件のうちの一つ）。
   private ingress が破られると、Gateway を経由せずに backend へ直接 identity ヘッダーを
   偽装したリクエストを送れてしまい、(b) がいくら正しく実装されていても意味を失う。

logvalet 側はこの network 境界を自身では強制できない（アプリケーション層のみで動作するため）。
構築者側の責務として明記する。

## (b) identity ヘッダーの strip-and-replace（多層防御としての logvalet 側 strip も含む）

logvalet backend は Entra ID JWT を自ら検証せず、Gateway が検証・抽出した identity を
`X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` の2ヘッダーで受け取る
（`gateway-request-contract.md` §2.2）。この値は **apikey 検証を通過したリクエストでのみ
信用される**（同 §2.3）が、apikey は共有鍵でありユーザー本人を認証しないため、apikey を知る
任意のクライアントがこれら identity ヘッダーを直接偽装して送ってきても apikey 検証だけでは
なりすましを検知できない（同 §2.4）。

Gateway 構築者が満たすべき要件:

1. **strip-and-replace の必須実装**: Gateway は、クライアントから受信したリクエストに
   `X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject`（または対応するカスタム
   ヘッダー名）が既に含まれていた場合でも、それを**必ず**破棄し、Gateway 自身が Entra ID JWT
   から検証・抽出した iss/sub の値で上書きしてから backend へ転送すること。クライアントが
   送ってきた値をそのまま透過させる実装は不可。
2. **多層防御としての logvalet 側 strip の位置づけ**: 上記1が Gateway 側で確実に行われることは、
   本ドキュメント執筆時点（S28）では実機未検証である（`gateway-request-contract.md` 残余項目 #2）。
   このため logvalet 側の実装（S21）も、受信した identity ヘッダーをそのまま信用するのではなく
   「apikey 検証を通過したリクエストでのみ参照する」というゲートを持つ（同 §2.3）。これは
   Gateway 側の strip-and-replace が万一漏れていた場合の第二層であり、Gateway 側の実装が
   不要になるという意味ではない。**Gateway 側での strip-and-replace は必須であり、logvalet 側の
   ゲートはそれを代替しない。**両方が揃って初めて identity 偽装が防げる。
3. Gateway のテンプレート機能でカスタムヘッダー名を設定できる場合、上記2ヘッダーの名称を
   logvalet 側の契約名（`X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject`）に
   合わせて設定すること。Gateway 側の標準ヘッダー名がこれと異なる場合は、実機検証後に
   logvalet 側の受理ヘッダー名を Gateway の実際の注入名に合わせてリネームする調整が必要になる
   （影響範囲は S21 のヘッダー名のみで、設計自体への影響はない。残余項目 #1）。

## (c) API key の rotation / revocation

apikey (`X-Logvalet-Api-Key`) は Gateway → logvalet 間の service-to-service 認証に使う静的
トークンである（`gateway-request-contract.md` §1）。Gateway 構築者向けの運用要件:

1. **rotation 手順**: 新しい apikey を発行し、Gateway の outbound API key credential provider
   側の設定を新しい鍵に更新する。旧鍵は、無停止でのローテーションが必要な場合、新鍵への切り替えが
   完了し全 Gateway インスタンス（複数リージョン・複数インスタンスで運用している場合は全台）が
   新鍵を使うようになったことを確認してから revoke する。
2. **revocation 手順**: logvalet backend 側で当該鍵を検証対象から外す（backend の
   `--bearer-token` 相当の設定値を差し替えて再起動、または将来的に鍵ストアを持つ場合はそこから
   削除）。revoke 後は旧鍵を提示するリクエストがすべて `401 Unauthorized` になる
   （`gateway-request-contract.md` §1.4）。
3. **無停止ローテーションのための新旧2鍵受理についての考慮点（重要な制約）**:
   **logvalet 側は現状1鍵のみを受理する実装である。** `crypto/subtle.ConstantTimeCompare` 等で
   単一の静的トークンと完全一致比較するのみで、新旧2鍵を同時に受理するグレースピリオドの仕組みを
   持たない（`gateway-request-contract.md` §1.3）。したがって、Gateway 側で「新鍵配布 → 旧鍵
   revoke」を段階的に行う運用を取る場合、その切り替え window の間は logvalet backend 側が
   新旧どちらか一方の鍵しか受理できない点に注意すること。全 Gateway インスタンスが新鍵に切り替わる
   前に旧鍵を backend 側で無効化すると、切り替えが済んでいない Gateway インスタンスからの
   リクエストが失敗する。逆に旧鍵を有効なまま新鍵を追加受理させたい場合、**それは logvalet 側の
   将来拡張（2鍵同時受理のサポート）が必要になる**。本 S28 時点ではこの拡張は実装されていない。
   無停止ローテーションを要件とする場合は、(i) Gateway 側で全インスタンスへの新鍵配布が完了して
   から旧鍵を revoke する運用でカバーするか、(ii) logvalet 側に2鍵受理機能を追加する別issueを
   起票するか、のいずれかを選ぶ必要がある。

## (d) Mcp-Method / Mcp-Name / MCP-Protocol-Version の透過設定

公式 Go SDK は、`Mcp-Protocol-Version` ヘッダーの値が `2026-07-28` 以上のときに限り
`Mcp-Method` / `Mcp-Name` ヘッダーの存在とボディ（`method`/`name` または `uri`）との一致を
検証する（`gateway-request-contract.md` §3.1）。この判定はリクエストごとに backend 側 SDK が
自律的に行うため、Gateway 側でこれら3ヘッダーを一律付与・削除・書き換えする必要はない。

Gateway 構築者が満たすべき要件:

1. **`Mcp-Protocol-Version` / `Mcp-Method` / `Mcp-Name` の3ヘッダーを、クライアントが送った値
   そのままバイト単位で backend へ転送する設定にすること。** 追加・削除・書き換えは行わない。
2. これらのヘッダーは MCP wire protocol の一部であり認証・認可とは無関係なので、Gateway の
   認証・認可処理のパスでこれらを操作対象に含めないこと。
3. **透過しない場合の影響**: Gateway が誤ってこれらのヘッダーを剥がすと、新プロトコル
   （`2026-07-28`）のクライアントからのリクエストがすべて `-32020 HeaderMismatch` で失敗する
   （旧プロトコルのリクエストはヘッダー検証対象外なので影響を受けない、`gateway-request-contract.md`
   §3.2）。この透過が構造的に不可能な Gateway 実装だった場合の logvalet 側の代替対応方針は
   `gateway-request-contract.md` §3.3 に記載がある（本ドキュメントの守備範囲外）。
4. `docs/specs/legacy-protocol-decision.md` の「AgentCore Gateway 運用上の注意」節も参照
   （後述 (h) にも記載する `supportedVersions` の全置換の注意点と合わせて確認すること）。

## (e) Backlog 用 CustomOauth2 credential provider の手動設定例

Backlog は AWS Bedrock AgentCore Identity の組み込み OAuth プロバイダに含まれないため、
CustomOauth2 credential provider として手動設定する。以下は設定例（実際の値はスペースごとに
読み替えること）:

```
provider type:        CustomOauth2
authorization endpoint: https://<your-space>.backlog.com/OAuth2AccessRequest.action
token endpoint:         https://<your-space>.backlog.com/api/v2/oauth2/token
client authentication:  CLIENT_SECRET_POST
scope:                  (指定しない / 空)
response_type:          code
```

補足:

- **scope なし**: Backlog OAuth2 はスコープパラメータを要求しない。credential provider の
  scope 欄は空のままにすること。logvalet 側もスコープ検証を行わない
  （`gateway-request-contract.md` 残余項目 #5）。
- **CLIENT_SECRET_POST**: client_id/client_secret はリクエストボディに POST する方式を使う
  （Authorization ヘッダーでの Basic 認証方式ではない）。
- **response_type=code**: 認可コードフロー（3LO）を使う。
- client_id / client_secret は Backlog スペースの「アプリケーション連携」設定で発行する
  OAuth2 アプリケーションの値を使う。redirect URI には後述 (f) の Gateway 側 callback endpoint
  の URL を登録する。

## (f) per-user 3LO session binding と初回同意用の公開 callback endpoint

3LO（three-legged OAuth）は per-user のセッションバインディングを必要とする。すなわち、
どのエンドユーザー（Entra ID の iss+sub）がどの Backlog OAuth トークンに紐付くかを
AgentCore Identity 側で管理する必要がある。

Gateway 構築者が満たすべき要件:

1. **初回同意（consent）用の公開 HTTPS callback endpoint を Gateway 側リポジトリに配置する。**
   logvalet 側にはこの callback を配置しない（issue #52 制約(c)、`gateway-request-contract.md`
   §2.4）。これは Backlog の OAuth 認可コードを受け取り、AgentCore Identity のトークン
   ボールトへ登録するためのエンドポイントであり、logvalet backend とは別のライフサイクルで
   運用される。
2. redirect URI は (e) で設定した Backlog OAuth2 アプリケーションの redirect URI として登録
   済みであること。
3. 実装は AWS 公式サンプル（AgentCore Identity の 3LO / outbound credential provider を用いた
   MCP server target のサンプル実装）を参照すること。本ドキュメントは logvalet 側の契約のみを
   定義するものであり、callback endpoint の実装そのものは提供しない。
4. per-user session binding が正しく機能して初めて、Gateway は各リクエストについて正しい
   ユーザーの Backlog access token を取得し `Authorization: Bearer <token>` として logvalet
   backend へ注入できる（`gateway-request-contract.md` §4.2）。この紐付けが取り違えられると、
   あるユーザーのリクエストが別ユーザーの Backlog トークンで処理されるという重大な事故になり得る
   ため、実機検証時に重点的に確認すること（後述 (h)）。

## (g) MCP target の DYNAMIC listing と 3LO は併用不可

issue #52 の確認済み制約 (b): **MCP target の DYNAMIC listing（ツール一覧を都度動的に取得する
モード）と 3LO は併用できない。** Gateway 構築者は以下のいずれかを選ぶ必要がある:

1. **DEFAULT 同期**: Gateway 側でツール一覧を事前に同期（静的化）し、3LO と併用する。
2. **`mcpToolSchema` の事前指定**: MCP target 登録時にツールスキーマを明示的に指定し、DYNAMIC
   listing を使わずに 3LO と併用する。

logvalet は 71 個の `ToolFunc`（SDK 非依存の `ToolDef` 抽象）を持つツール群であり、ツール一覧
自体は頻繁には変わらない前提である。DYNAMIC listing の利便性（ツール追加時の自動反映）よりも
3LO による per-user Backlog 認可が設計の中核（issue #52「採用アーキテクチャ」節）であるため、
上記2択のうちどちらを採るかは Gateway 構築者の運用方針次第だが、**3LO を使う以上 DYNAMIC
listing は選択肢から外れる**ことを設計の前提として認識すること。

## (h) 実機検証チェックリスト

以下は logvalet リポジトリ側では検証できず、Gateway 実機構築時に確認が必要な項目
（`gateway-request-contract.md` §6.2 残余項目一覧、issue #52 制約節 と対応）:

- [ ] **scope なし認可の通過**: (e) で設定した scope 空の CustomOauth2 credential provider で
      Backlog の認可コードフローが正しく完了するか。Backlog 側が scope パラメータの欠落を
      エラーとしないか確認する。
- [ ] **refresh token rotation 競合**: 複数の並行リクエストが同時に refresh token を使おうと
      した場合の AgentCore Identity 側の挙動（competing refresh によるトークン失効・エラー）を
      確認する。logvalet 側はこの競合を意識しない設計（`gateway-request-contract.md` 残余項目
      #6）であるため、競合が発生した場合の影響は全面的に Gateway/AgentCore Identity 側の
      挙動に依存する。
- [ ] **未同意時の MCP エラー形式**: consent 未完了のユーザーがツールを呼び出した際、
      Gateway 層で完結してエラーを返すのか、それとも `Authorization` ヘッダーなし（または
      無効なトークン付き）のまま logvalet backend まで到達してしまうのかを確認する
      （`gateway-request-contract.md` §5.2）。後者の場合、logvalet は
      「Gateway 側の同意 URL を含まない単純なツールエラー」しか返せない点を踏まえ、
      Gateway 層で完結させる構成を優先することが望ましい。
- [ ] **Entra ID の `sub` claim と Gateway 側 userId の対応関係**: Gateway が backend へ渡す
      identity（(b) の strip-and-replace 後の値）が、Entra ID JWT の `sub` claim と実際に
      一致する対応関係になっているか確認する。対応関係が想定と異なる場合、logvalet 側の
      userID 正規化関数（S21、`iss` + `sub` を正規化元とする）のみを差し替えれば影響を吸収
      できる（`gateway-request-contract.md` 残余項目 #8）。
- [ ] **Gateway の実ヘッダー注入名の確認**: (b) で述べた identity ヘッダーの実際の注入名が
      `X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject`（logvalet 側の契約名）と
      一致しているか、Gateway のテンプレート設定を実機で確認する。異なる場合は logvalet 側の
      受理ヘッダー名をリネームする対応が必要になる（残余項目 #1）。
- [ ] **`supportedVersions` がリスト全置換である運用注意**: `docs/specs/legacy-protocol-decision.md`
      の「AgentCore Gateway 運用上の注意」節を参照。Gateway 側の `supportedVersions` 設定は
      全置換（差分適用ではない）である製品がある。logvalet backend が実際にサポートする
      プロトコルバージョンは公式 Go SDK v1.7.0 の固定5バージョン（`2026-07-28`, `2025-11-25`,
      `2025-06-18`, `2025-03-26`, `2024-11-05`）であり、Gateway 側の設定がこれとズレると
      「Gateway が広告するが backend が解釈できない」または「backend は対応できるが Gateway の
      リスト漏れで弾かれる」という不整合が起きる。logvalet の `go.mod` が固定する
      `github.com/modelcontextprotocol/go-sdk` のバージョンを更新するタイミングで、Gateway 側の
      `supportedVersions` リストも同時に見直す運用ルールを徹底すること。

---

## スコープ外の明記

本ドキュメントは logvalet リポジトリ側が確定させた契約に基づく参考情報であり、Terraform 等の
インフラコード・Gateway の実際のデプロイ・実際の Entra ID テナントに対する認可フローの動作
確認は含まない（2026-07-30 ユーザー決定）。これらはユーザーが別リポジトリで行う。本ドキュメント
に記載した各項目のうち「実機検証が必要」と明記したものは、`docs/specs/gateway-request-contract.md`
§6.2 の残余項目一覧に対応しており、実機検証の結果によっては logvalet 側（S14/S21/S22/S30 等）の
実装調整が必要になる場合がある。
