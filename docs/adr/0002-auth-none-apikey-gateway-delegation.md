# 0002. 認証を none|apikey に縮退し、OAuth/EntraID/Backlog トークン管理を AgentCore Gateway/Identity へ委譲する

## ステータス

承認済み（issue #52、2026-07-30）

## コンテキスト

logvalet の HTTP MCP サーバーは idproxy（OIDC ログインフロー・Cookie セッション・
`--oidc-*` 系フラグ・`--allowed-domains`/`--allowed-emails`・DynamoDB を含む tokenstore）を
自前で実装していた。この構成は認証・認可・トークン管理のすべてを logvalet が担うため、
保守面積が大きく、セキュリティクリティカルなコード（OIDC コールバック、署名鍵管理、
トークン永続化）を自前で持ち続けるリスクがあった。

一方で AWS AgentCore Gateway / AgentCore Identity は、Entra ID JWT の inbound 検証、
API key の outbound 発行、CustomOauth2 credential provider による per-user OAuth トークンの
取得・保管・リフレッシュを提供することが公式ドキュメントで確認できた（2026-07-30 web 調査）。
Gateway が認証・認可・トークン管理を代行できるなら、logvalet はコア機能（Backlog API
ラッピング）に注力し、認証基盤は Gateway 側に委譲する方が保守面積を削減できる。

未確認点として、(a) Gateway が元の Entra ID JWT を backend（logvalet）へ転送できるか、
(b) API key outbound 時にユーザー identity が backend へどう渡るか、の2点があった。
(b) は identity ヘッダーの受理設計（S21）で解決し、(a) は S04（契約文書化）で
「実機構築は別リポジトリの責務であり logvalet 側では確認しない」というユーザー決定
（2026-07-30）により、契約文書レベルでは未確定のまま残った。この未確定を理由に、
JWT パススルー検証（S22）は多層防御としての位置づけに留め、実施可否を Gateway 側の
実機検証結果に委ねる（未検証の間は実施しない）という判断を明示的に採った。

## 決定

認証モードを `none` | `apikey` の2値に縮退する。`apikey` は Gateway 自身を認証する共有鍵
であり、個々のユーザーを認証するものではない。ユーザー識別は Gateway が付与する identity
ヘッダーを `apikey` 検証を通過したリクエストでのみ信用する設計（S21）で行う。

OIDC ログインフロー・Cookie セッション・署名鍵管理は完全に削除する（S19）。削除対象は
`--auth`, `--external-url`, `--oidc-issuer`, `--oidc-client-id`, `--oidc-client-secret`,
`--cookie-secret`, `--allowed-domains`, `--allowed-emails`, `--signing-key`,
`--idproxy-store*`（5個）の計14フラグ・環境変数と、`github.com/youyo/idproxy` およびその
派生依存（`coreos/go-oidc`, `gorilla/securecookie`, `redis/go-redis`）。

Backlog OAuth のトークン管理は AgentCore Identity の CustomOauth2 credential provider に
完全委譲する（決定E）。logvalet は Gateway が注入する `Authorization: Bearer` トークンを
リクエストごとに Backlog API へそのまま転送する passthrough のみを実装する（S30）。
これにより HTTP/Gateway モードでは logvalet 側でのトークン永続化が不要になった。

tokenstore は CLI/stdio（ローカル利用）専用に縮小し、バックエンドをローカルストレージ
（sqlite / tokens.json）のみに限定する（決定F）。DynamoDB バックエンド
（`internal/auth/tokenstore/dynamodb.go` 一式、`--token-store=dynamodb`、
`--token-store-dynamodb-table`、`--token-store-dynamodb-region` および対応する環境変数）
を削除する。

**JWT パススルー検証（S22）は実施しない。** Gateway が元の Entra ID JWT を backend へ
転送できるかが実機未確認（AgentCore Gateway の実機構築・検証は別リポジトリの責務という
ユーザー決定により logvalet 側では検証しない）であるため、S04 の契約文書レベルで
条件が満たされたと確認できていない。したがって S21 の共有鍵（apikey）+ ネットワーク境界
（private ingress）+ identity ヘッダーの strip-and-replace を最終防御とし、JWT 検証層は
追加しない。この判断は Gateway 側の実機検証で転送可能性が確認された場合に再検討しうる
（`docs/specs/gateway-request-contract.md` に記録済み）。

## 検討した代替案

### 代替案A: idproxy による OIDC ログインフローを継続する

logvalet 自身が OIDC 認証・セッション管理・許可リスト（domain/email）を持ち続ける。

却下理由: セキュリティクリティカルな認証コード（コールバック処理、Cookie 署名、
セッション管理）を自前で保守し続けるコストと、AgentCore Gateway が同等以上の機能
（Entra ID JWT 検証、per-user トークン管理）を代行できるという確認結果を踏まえ、
logvalet はコア機能（Backlog API ラッピング）に注力する方針を優先した。

### 代替案B: logvalet 内 tokenstore（DynamoDB 含む）による Backlog トークン管理を継続する

Backlog OAuth トークンの取得・保管・リフレッシュを logvalet が引き続き担う。

却下理由: AgentCore Identity の CustomOauth2 credential provider が同等機能を提供する
ことが確認できた（決定E）ため、HTTP/Gateway モードでのサーバーサイドトークン永続化が
不要になった。永続化を続けることは複数インスタンス運用時のストア共有問題
（memory ストアが機能しない等）を logvalet 側に残し続けることを意味する。

### 代替案C: S22（JWT パススルー検証）を無条件で実装する

Gateway 側の転送可否によらず、JWT 検証ミドルウェアを実装しておく。

却下理由: S04 で「Gateway が元の JWT を backend へ転送できる」ことが実機で確認できて
初めて意味を持つ機能であり、確認されていない前提のコードを追加すると、実際には機能しない
（Gateway が JWT を転送しない）多層防御を「効いているつもり」で運用するリスクがある。
実機検証は別リポジトリの責務と決定されているため、logvalet 側では確認できる範囲
（S21 の共有鍵 + ネットワーク境界）を最終防御として明示し、JWT 検証は見送った。

## 影響

- `internal/cli/mcp_auth.go` 等の OIDC/idproxy 関連コードと14フラグが削除された。
  `--auth-mode=oidc` は起動時エラー、`--auth-mode=bearer` は `apikey` の別名として残る。
- `internal/cli/mcp_identity.go` に Gateway identity ヘッダー受理ミドルウェアが追加された
  （apikey 検証を通過したリクエストでのみ信用、strip-and-replace 前提）。
- `internal/auth/`, `internal/backlog/` に Backlog credential の Bearer passthrough が
  実装された（S30）。トークンはリクエストスコープで扱われ、プロセス内・永続ストアいずれにも
  保存されない。
- `internal/auth/tokenstore/dynamodb.go` 一式が削除され、tokenstore は sqlite/memory の
  ローカルバックエンドのみになった。HTTP モードでは tokenstore 自体を使用しない。
- Gateway 構築者向けの参考ドキュメント（`docs/specs/agentcore-gateway-builder-guide.md`）を
  用意し、private ingress・identity ヘッダーの strip-and-replace・API key rotation 等の
  要件を明記した。Terraform 実装・実機での攻撃テストは logvalet リポジトリのスコープ外。
- 破壊的変更として次期メジャーリリースの CHANGELOG に記載する。
