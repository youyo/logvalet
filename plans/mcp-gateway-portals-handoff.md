# mcp-gateway 方針変更 引き継ぎ: AgentCore Gateway → Cloudflare MCP Server Portals

- 日付: 2026-09-10
- 課題: [HEP_ISSUES-1599](https://heptagon.backlog.com/view/HEP_ISSUES-1599)
- 決定者: ishizawa

## 決定

複数 MCP を束ねる社内基盤は **Amazon Bedrock AgentCore Gateway ではなく Cloudflare MCP Server Portals（Cloudflare One）** を採用する。
MCP サーバー本体（logvalet 等）は **AWS Lambda のまま** 運用し、Portals に上流 URL として登録する。

## 経緯

- HEP_ISSUES-1599 で AgentCore Gateway + Terraform（WU-01〜12 実装済み、WU-04 PoC 未実施）を進めていた
- 「Cloudflare AI Gateway で足りないか」を検討 → AI Gateway は LLM API プロキシで MCP 集約は対象外
- 代わりに Cloudflare One の MCP Server Portals が要件（複数 MCP の1エンドポイント集約、ユーザー認証、上流毎の認証、監査ログ）を満たすことを確認
- AgentCore 側で自前構築していた監査ログ（WU-10）・ユーザー管理 Runbook（WU-12）の大半が Portals 標準機能で代替される

## アーキテクチャ

```
ユーザー ─Entra ID(Access IdP)─▶ MCP Server Portals ─(サーバー毎の上流認証)─▶ 各 MCP サーバー
                                        ├─ logvalet (Lambda)   : Backlog OAuth
                                        ├─ 社内 MCP (Lambda等) : Access for SaaS (= Entra) or bearer
                                        └─ 外部 MCP            : 各サービスの OAuth
```

2層の認証認可:

1. **Portals 層（共通）**: Cloudflare Access が Entra ID を IdP として「Heptagon の誰がポータルに入れるか」を制御。MCP クライアントは Access Managed OAuth（認可コードフロー）で接続。監査ログは Portals で集約。
2. **上流層（サーバー毎）**: 各 MCP サーバーの上流認証が「その人がそのサービスで何ができるか」を制御。

## logvalet の方針

- 上流認証は **Backlog OAuth（`auth_type: oauth`、手動 credentials）**。AS は Backlog 自体。
  - Authorization endpoint: `https://{space}.backlog.com/OAuth2AccessRequest.action`
  - Token endpoint: `https://{space}.backlog.com/api/v2/oauth2/token`
  - Backlog 側で OAuth アプリを登録し、Portals が表示する Redirect URI を許可する
  - `Discover OAuth endpoints` は Backlog が `.well-known` を持たないため失敗する → advanced で手動入力
- Portals がユーザー毎の Backlog access token を `Authorization: Bearer` で Lambda に送る。logvalet はそれをそのまま Backlog API に使う。**権限判定は Backlog が行う**。
- logvalet の改修は「API キー（`apiKey` クエリ）の代わりに `Authorization: Bearer` を受けて Backlog API へ渡す」分岐のみ。OAuth AS 実装・JWT 検証・ユーザー→キーのマッピングは不要。
- Lambda Function URL は `AuthType: NONE`（Portals から SigV4 は打てない）。直叩きされても有効な Backlog トークンがなければ何もできないため攻撃面は増えない。必要なら Cloudflare IP レンジで絞る。

### 不採用とした案

**Entra access token → email 抽出 → logvalet が Backlog OAuth を代行（refresh token を保持）**

技術的には可能だが、logvalet に OAuth AS 機能（Access for SaaS への委譲）、Backlog 連携 UX（初回の URL 案内 + callback）、トークンストア（email → refresh token の暗号化保存・失効）、Entra と Backlog の email 一致という運用前提が必要になる。得られるセキュリティ水準は Backlog を直接 AS にする案と同じため不採用。

Entra は Portals 層で全サーバー共通に効いている。Backlog 以外の社内 MCP で Entra を上流にしたい場合は Access for SaaS を使い、その MCP 側で Access JWT を検証する。

## 留意点

- Backlog トークンは opaque。Lambda 側で事前検証はできず、最初の API 呼び出しの 401 で弾かれる（Lambda authorizer は置けない）
- Backlog OAuth に scope はなくユーザー権限フル。粒度を絞る場合は Portals 側の tool 許可リストで行う
- OAuth アプリはスペース単位。複数スペース対応なら Portals にスペース毎に MCP サーバーを登録
- access token は1時間。Portals が refresh_token grant で更新する想定（PoC で確認）
- 手動 OAuth のサーバーは最初のユーザー認可完了まで **Waiting**。tool 一覧はその時点で固定されるため、logvalet の tool 追加時は再同期が必要
- Portals は stdio-only MCP を登録できない（remote HTTP のみ）
- Cloudflare One の契約（シート課金）が必要

## 次のアクション

1. HEP_ISSUES-1599 に方針変更コメント、AgentCore 系子課題（WU-04〜13）のクローズ or 再編
2. ADR 追加: 「MCP 集約基盤に Cloudflare MCP Server Portals を採用」「logvalet の上流認証は Backlog OAuth」（既存 ADR-0001〜0008 の該当分を置換済みに）
3. Cloudflare One: Entra ID を IdP 登録、MCP Portal 作成、Access ポリシー設定
4. Backlog: OAuth アプリ登録（Redirect URI は Portals 表示値）
5. logvalet: `Authorization: Bearer` 受け口の実装、Lambda Function URL（`AuthType: NONE`）で公開
6. Portals に logvalet を登録（oauth / 手動 credentials）→ 初回認可 → Ready 確認
7. PoC 確認項目: refresh 動作、tool 追加時の再同期手順、Claude.ai / Claude Code からの接続
8. AgentCore 用 Terraform（infra/）の扱いを決める（削除 or アーカイブ）

## 参考

- [MCP server portals · Cloudflare One docs](https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/mcp-portals/)
- [Secure MCP servers · Cloudflare One docs](https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/secure-mcp-servers/)
- [Introducing MCP Server Portals · Cloudflare Blog](https://blog.cloudflare.com/zero-trust-mcp-server-portals/)
