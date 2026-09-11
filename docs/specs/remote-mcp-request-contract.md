# リモート MCP リクエスト契約

`logvalet mcp`（Streamable HTTP）のリクエスト契約。v0.40 で
`gateway-request-contract.md`（AgentCore Gateway 前提）を置き換えた。

## 0. 全体構成

```
MCP クライアント
  → Cloudflare MCP Server Portals（利用者認証・tool 許可リスト・監査）
    → logvalet mcp（Streamable HTTP, Stateless=true）
      → Backlog API
```

logvalet は**呼び出し元を認証しない**。呼び出し元の認証・認可・監査は前段
（Portals 等）の責務である。logvalet の責務はリクエストに載った Backlog 資格情報を
Backlog API へ転送し、結果を MCP のツール結果へ変換することに限られる。

Backlog スペースごとに MCP サーバー（Lambda 等）を個別に登録する。単一プロセスが
複数スペースを扱う機構は持たない（multi-space は v0.40 で撤去）。

## 1. エンドポイント

| パス | メソッド | 資格情報 | 用途 |
|---|---|---|---|
| `/mcp` | POST | `Authorization: Bearer` 必須 | MCP Streamable HTTP |
| `/healthz` | GET | 不要 | ヘルスチェック |

`/healthz` は資格情報の検証対象外で、常に `{"status":"ok"}` と 200 を返す。
ロードバランサー・コンテナオーケストレーターからの到達性確認に使う。

## 2. Backlog credential の Bearer passthrough

### 2.1 ヘッダー名・トークン形式

```
Authorization: Bearer <backlog-oauth-access-token>
```

- スキームは `Bearer`（大文字小文字を区別しない）。
- 値は Backlog の OAuth アクセストークン。logvalet はトークンを解析せず、
  失効判定もしない。
- `Authorization` ヘッダーは Backlog 資格情報専用である。logvalet 自身の
  呼び出し元認証には用いない（v0.40 でその層自体を撤去した）。

### 2.2 付与タイミング

前段（Portals）がリクエストごとに、認証済み利用者に紐づく Backlog アクセストークンを
注入する。トークンのライフサイクル（取得・リフレッシュ・保管）は前段が持つ。
logvalet はトークンを保存もリフレッシュもしない。

### 2.3 logvalet 側の処理仕様

1. `/mcp` へのリクエストから `Authorization` ヘッダーを読む。
2. 欠落または `Bearer` スキームでない場合、MCP ハンドラへ到達させずに `401` と
   エラーエンベロープ（spec §9 形式）を返す。

   ```json
   {"schema_version":"1","error":{"code":"authentication_error","message":"...","retryable":false}}
   ```

3. 検証を通ったトークンはリクエスト単位の `context.Context` に載せ、その
   リクエストを処理する `backlog.Client` の資格情報として使う。リクエスト間で
   共有・キャッシュしない。
4. Backlog API がそのトークンを拒否した場合（401/403）、HTTP レベルのエラーでは
   なく **tool error**（`isError=true` のツール結果）として返す。プロトコル
   エラーと権限エラーを混同させないため。

### 2.4 CLI/stdio モードとの違い

CLI と `mcp-stdio` はこの契約の対象外である。両者はサーバー側の資格情報
（設定・env・フラグの API キーまたはアクセストークン）を使い、`Authorization`
ヘッダーを参照しない。

## 3. MCP プロトコル

- 公式 Go SDK の `StreamableHTTPHandler` を `Stateless=true` で使う。
- `server/discover`、per-request `_meta`、非冪等ツールの冪等キーに対応する。
- プロトコルバージョン交渉の判断は
  [legacy-protocol-decision.md](legacy-protocol-decision.md) を参照。

## 4. スコープ外

- 呼び出し元（エンドユーザー）の認証・認可。前段の責務。
- 利用者ごとのアクセス制御・監査ログ・tool 許可リスト。いずれも Portals の機能。
- Backlog OAuth の認可フロー（認可 URL の提示、コールバック受理、トークン保管）。
  logvalet はこれらを実装しない。v0.36 までの内蔵 OAuth コールバックと
  `_meta.authorization_url` 導線は v0.40 で削除した。
- 複数 Backlog スペースの横断操作。スペース毎にサーバーを登録して解決する。
