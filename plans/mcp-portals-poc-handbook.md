# Cloudflare MCP Server Portals PoC 手順書（logvalet 単一スペース）

- 日付: 2026-09-11
- 課題: [HEP_ISSUES-1753](https://heptagon.backlog.com/view/HEP_ISSUES-1753)（HEP_ISSUES-1599 はクローズ済み）
- 前提: [mcp-gateway-portals-handoff.md](./mcp-gateway-portals-handoff.md)（方針決定の経緯・アーキテクチャ図）

## 1. 目的・スコープ

logvalet MCP（Backlog）を Cloudflare MCP Server Portals 経由で公開し、Backlog OAuth による
ユーザー個別認証で疎通することを PoC で確認する。

対象:

- logvalet-mcp（Lambda）を OIDC モードから Bearer passthrough へ切り替え（v0.40 以降は HTTP の唯一の構成で、フラグ指定は不要）
- Portals への手動 OAuth サーバー登録・初回認可・tool 許可リスト
- refresh・再同期・複数クライアント接続の確認

対象外:

- AgentCore Gateway 用 Terraform（`infra/`）の整理・削除判断
- HEP_ISSUES-1599 の子課題（WU-04〜13）のクローズ・再編
- 複数スペース対応、送信元 IP 固定の本実装（TODO に記載のみ）

## 2. アーキテクチャ（再掲）

```
ユーザー ─Entra ID(Access IdP)─▶ MCP Server Portals ─Backlog OAuth(手動 credentials)─▶ logvalet (Lambda)
```

- Portals 層: Cloudflare Access が Entra ID を IdP として利用者を認証
- 上流層: Portals が保持する各ユーザーの Backlog access token を `Authorization: Bearer` で Lambda に送る
- logvalet はそれを Backlog API へそのまま転送する（v0.36.0 で実装、v0.40 で HTTP の唯一の構成に一本化）

## 3. 事前準備

### 3.1 Cloudflare One

| 手順 | 内容 |
|---|---|
| IdP 登録 | Cloudflare Zero Trust ダッシュボードで Entra ID を Login method として追加 |
| Portal 作成 | Access > MCP Server Portals から新規 Portal を作成 |
| Access ポリシー | Portal に対する Access ポリシーで、対象を Heptagon の Entra ID グループ/ユーザーに限定 |

### 3.2 Backlog OAuth アプリ登録

- OAuth アプリは Backlog Developer サイト（https://backlog.com/developer/applications/ ）で登録する（developer.nulab.com の OAuth 2.0 認証ページに記載）。Backlog アカウントで登録し、対象スペースは heptagon.backlog.com。client 認証は client_secret を POST body に含める形式のみで、access token の有効期限は 3600 秒
- Redirect URI は **Portals がサーバー登録画面で表示する値**を使う。そのため登録順序は次の通り。
  1. Portals 側でサーバーを仮登録し Redirect URI を確認（4.2 参照）
  2. Backlog 側で OAuth アプリを作成し、その Redirect URI を登録
  3. 取得した Client ID / Client Secret を Portals の設定に反映

## 4. logvalet-mcp の変更手順

対象リポジトリ: `heptagon-inc/logvalet-mcp`（読み取り確認済み、変更は本手順で実施）

### 4.1 function.json の env 差分

| 変更前（OIDC モード） | 変更後（none / Bearer passthrough） |
|---|---|
| `LOGVALET_MCP_AUTH_MODE=oidc` | **削除（v0.40 で廃止、HTTP は常に passthrough）**。設定すると起動時に fail-fast する |
| `LOGVALET_MCP_AUTH` | 削除 |
| `LOGVALET_MCP_EXTERNAL_URL` | 削除 |
| `LOGVALET_MCP_OIDC_ISSUER` | 削除 |
| `LOGVALET_MCP_OIDC_CLIENT_ID` | 削除 |
| `LOGVALET_MCP_OIDC_CLIENT_SECRET` | 削除 |
| `LOGVALET_MCP_COOKIE_SECRET` | 削除 |
| `LOGVALET_MCP_ALLOWED_DOMAINS` | 削除 |
| `LOGVALET_MCP_BACKLOG_CLIENT_ID` / `_SECRET` / `_REDIRECT_URL` | 削除（logvalet 自身は OAuth AS 機能を使わない） |
| `LOGVALET_MCP_OAUTH_STATE_SECRET` | 削除 |
| `LOGVALET_MCP_TOKEN_STORE` 系（3件） | 削除 |
| `LOGVALET_MCP_SIGNING_KEY` / `LOGVALET_MCP_IDPROXY_STORE` 系（3件） | 削除（idproxy は OIDC モード専用） |
| `LOGVALET_API_KEY` | 削除（Bearer passthrough では不要） |
| `LOGVALET_BASE_URL` | 維持（対象スペースの Backlog URL） |
| `LOGVALET_SPACE` | 維持 |
| `AWS_LWA_INVOKE_MODE=response_stream` | 維持 |
| `LOGVALET_SPACE_STORE_TYPE=dynamodb` | **不要（multi-space 機能削除）**。v0.40 で multi-space を撤去したため、この変数を設定すると起動時に fail-fast する |
| `LOGVALET_SPACE_STORE_DYNAMODB_TABLE` / `_REGION` | **不要（multi-space 機能削除）**。同上 |

Function URL の `AuthType` は既存どおり `NONE` を維持し、`RESPONSE_STREAM` invoke mode も変えない。

### 4.2 mise.toml

- `LOGVALET_VERSION` を multi-space 撤去版（v0.40.0 以降）に更新。v0.36.x を使う場合は `LOGVALET_SPACE_STORE_TYPE=sqlite` の指定が必要

### 4.3 README

- Mode 表は v0.40 で廃止し、「利用経路」（CLI / ローカル MCP / リモート MCP）と「認証の2層」の構成に差し替え済み。
  リモート MCP は単一構成（呼び出し元認証なし + Bearer passthrough）で、Portals 前段を前提とする旨を明記した

### 4.4 デプロイ

```bash
git tag v<新バージョン>
git push origin v<新バージョン>
```

- デプロイ CI は `v*` タグプッシュでのみ起動する（main push では起動しない）
- CI 完了後、Function URL を確認:

```bash
aws lambda get-function-url-config --function-name logvalet-mcp \
  --query 'FunctionUrl' --output text
```

## 5. Portals へのサーバー登録

### 5.1 手動 OAuth 設定

Portals のサーバー追加で `auth_type: oauth`、認可方式は「手動 credentials」を選択し、以下を入力する。

```json
{
  "id": "logvalet-heptagon",
  "name": "logvalet (heptagon.backlog.com)",
  "hostname": "https://<Function URL のホスト>/mcp",
  "auth_type": "oauth",
  "auth_credentials": "{\"auth_mode\":\"manual\",\"config\":{\"authorization_endpoint\":\"https://heptagon.backlog.com/OAuth2AccessRequest.action\",\"token_endpoint\":\"https://heptagon.backlog.com/api/v2/oauth2/token\"},\"registration_info\":{\"client_id\":\"<Backlog OAuth アプリの Client ID>\",\"redirect_uris\":[\"<Portals が表示する Redirect URI>\"],\"token_endpoint_auth_method\":\"client_secret_post\"}}",
  "client_secret": "<Backlog OAuth アプリの Client Secret>"
}
```

- `client_secret` は `auth_credentials` の**外側**の兄弟フィールドに置く（公式 docs: 「Do not include `client_secret` or OAuth tokens inside `auth_credentials`」）。Cloudflare は暗号化保存し、ダッシュボード・API から再表示しない。編集時に空欄なら既存値を維持
- ダッシュボードから登録する場合は同じ値を各フォーム項目に入力する。`scope` は空のまま（Backlog OAuth に scope はない）

- `Discover OAuth endpoints` は使わず advanced で endpoint を手入力する（Backlog は `.well-known` を持たない）
- `token_endpoint_auth_method` は **`client_secret_post`** を指定する。Backlog は client 認証が `CLIENT_SECRET_POST` 固定で、既定の `client_secret_basic` だと認可コードフローが失敗する（AgentCore 時代の実機知見、engram ADR mcp-gateway/0010）
- 上流 URL: `https://<Function URL のホスト>/mcp`

### 5.2 初回認可と Ready 確認

- サーバー登録直後は **Waiting** 状態。Portals 管理者（または最初の利用者）が認可フローを1回実行する
- 認可完了後、サーバーが **Ready** になることを確認する

### 5.3 tool 許可リスト

- **除外は不要**。v0.40 の multi-space 撤去で space registry 系5ツール
  （`logvalet_space_list` / `_use` / `_verify` / `_connect_url` / `_disconnect`）は登録されなくなった
- 残る `logvalet_space_info` / `logvalet_space_digest` / `logvalet_space_disk_usage` は
  Backlog の `/api/v2/space` 系 API の単一スペース向けラッパーで、none モードでも正常に動作するため
  許可リストから除外しない

## 6. 確認項目チェックリスト

| 項目 | 手順 | 期待結果 | 結果 |
|---|---|---|---|
| 初回接続 | Portals 経由で MCP クライアントから接続 | `tools/list` が 67 ツールを返し、space registry 系5ツールを含まない | |
| tools/call 疎通 | 任意の issue 系ツールを呼ぶ | 本人の Backlog 権限範囲でデータが返る | |
| access token 期限切れ後の refresh | 1時間待機後に再度 `tools/call` | Portals が refresh token で自動更新し、追加操作なく成功する | |
| refresh 失効時の再認可 | refresh token を無効化（Backlog 側でアプリ連携解除等）後に `tools/call` | クライアントに再認可 URL が提示される。**注意**: logvalet は Backlog 401 を JSON-RPC tool error（HTTP 200, `isError=true`）で返すため、Portals が HTTP 401 をトリガに再認可を出す実装だと発火しない可能性がある。発火しない場合は logvalet 側で認証エラー時に HTTP 401 へ昇格する変更を検討する | |
| tool 追加時の Sync capabilities | logvalet に新しい tool を追加してデプロイ後、Portals ダッシュボードで「Sync capabilities」を実行（自動同期は約2時間） | 新しい tool が `tools/list` に反映される | |
| Claude.ai からの接続 | Claude.ai のコネクタ設定から Portals URL を追加 | 認可後に接続でき、tool が呼び出せる | |
| Claude Code からの接続 | `claude mcp add` 等で Portals URL を登録 | 認可後に接続でき、tool が呼び出せる | |
| Function URL 直叩き | Bearer なしで Function URL に直接リクエスト | エラーになり、Backlog データへアクセスできない | |
| Egress Policy（任意） | Cloudflare Gateway の Egress Policy で送信元 IP を固定 | Lambda 側アクセスログの送信元が固定 IP になる（PoC では未実施でも可） | |

## 7. 既知の制約・リスク

- Backlog OAuth に scope はなくユーザー権限フル。粒度制御は Portals の tool 許可リストのみ
- Backlog トークンは opaque で Lambda 側の事前検証はできない。401 は初回 API 呼び出し時にのみ判明する
- 手動 OAuth サーバーは初回認可完了まで Waiting。tool 一覧はその時点で固定されるため追加時は再同期が必要
- Portals は stdio-only MCP を登録できない（remote HTTP のみ）
- logvalet の Backlog 401 が JSON-RPC tool error に包まれるため、Portals の再認可トリガー（HTTP 401 前提の可能性）と噛み合わない懸念がある（6章参照）
- Cloudflare One の契約（シート課金）が前提

## 8. PoC 後の TODO

- 複数スペース対応: スペースごとに Lambda（`LOGVALET_BASE_URL` 固定）と Portals サーバー登録を追加
- 送信元 IP 固定の本実装: Cloudflare Egress Policy の IP レンジを Lambda 側で許可リスト化
- HEP_ISSUES-1753 に本 PoC 結果をコメントし、HEP_ISSUES-1599 配下の AgentCore 系子課題（WU-04〜13）のクローズ or 再編を判断
- AgentCore 用 Terraform（`infra/`）の扱い（削除 or アーカイブ）を決定
