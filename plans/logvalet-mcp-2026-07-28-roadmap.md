# logvalet 再設計ロードマップ — MCP 2026-07-28 (stateless) 対応

親 issue: [#52](https://github.com/youyo/logvalet/issues/52)

## マイルストーン表

| M | issue | タイトル | ステップ | 主な依存 |
|---|---|---|---|---|
| M0 | — | 既存 issue の整理と新ロードマップ展開 | S01, S02 | — |
| M1 | [#54](https://github.com/youyo/logvalet/issues/54) | 検証スパイク: 公式 Go SDK / Gateway リクエスト契約定義 | S03, S04 | — |
| M2 | [#55](https://github.com/youyo/logvalet/issues/55) | MCP SDK 非依存の ToolDef 抽象を導入し 72 ツール定義を移行 | S06, S07, S08 | — |
| M3 | [#56](https://github.com/youyo/logvalet/issues/56) | MCP サーバーを公式 Go SDK へ移行し stateless 化する | S09, S10, S11 | M1(S03), M2 |
| M4 | [#57](https://github.com/youyo/logvalet/issues/57) | MCP 2026-07-28 wire protocol への準拠（discover / _meta / ヘッダー / MRTR） | S12–S17 | M3 |
| M5 | [#58](https://github.com/youyo/logvalet/issues/58) | idproxy / OIDC 経路の完全削除 | S19 | M3 |
| M6 | [#59](https://github.com/youyo/logvalet/issues/59) | 認証を none / apikey に簡素化し Gateway identity を受理し Backlog credential を Bearer passthrough する | S20, S21, S22, S30 | M1(S04 の契約文書), M5 |
| M7 | [#60](https://github.com/youyo/logvalet/issues/60) | 永続ストアの既定を見直し HTTP モードで memory を禁止する | S23 | M5 |
| M8 | — | Backlog 課題リレーション対応（対応不要・実施しない、ユーザー決定 2026-07-30） | — | — |
| M9 | [#61](https://github.com/youyo/logvalet/issues/61) | ドキュメント更新・Gateway 構築者向け参考ドキュメント・破壊的変更リリース | S27, S28, S29 | M6(S21), M7 |

## ステップ依存関係

クリティカルパス: **M1 → M2 → M3 → M4 → M5 → M6 → M9**。
M8（Backlog 課題リレーション）は対応不要のため依存グラフから除外。M7 は M5 完了後にいつでも着手可。

```
S01 → S02
S03 ─┬─────────────┐
     │             │
S04 ─┘             │
                    ▼
S06 → S07 → S08 → S09 → S10 → S11
                              │
              ┌───────────────┼───────────────┐
              ▼                                ▼
      S12→S13→S14→S15                        S19
              │    └→S16→S17                  │
              │                    ┌───────────┼───────────┐
              │                    ▼           ▼           ▼
              │                  S20→S21→S22  S20→S30    S23
              │                        │        │
              └────────────────────────┴────────┴──→ S27,S28 → S29
```

条件付きステップ:
- **S15**（version negotiation）: S03 の verdict で分岐。公式 SDK が旧 initialize/session 型 server を提供するかで方式選択。
- **S22**（JWT パススルー検証）: S04 で Gateway が元の JWT を転送できると確認できた場合のみ実施。
- **S26**（課題間 related issues 実装）: 対応不要（決定A）のため M8 全体を実施しない。

## 決定 A〜F の要約

- **決定A**: Backlog related issues API は公開 API に実在根拠がなく、本テーマ（M8: S24/S25/S26）は対応不要。
- **決定B**（アーキテクチャ採用）: MCP SDK は公式 Go SDK へ乗り換える。logvalet 所有の SDK 非依存 `ToolDef` 抽象を先に挟み（M2）、backend だけを差し替える（M3）ことで 72 箇所の機械的変換と wire protocol 対応を分離する。apikey はゲートウェイを認証する共有鍵でありユーザーを認証しない前提で、identity ヘッダーは apikey 検証を通過したリクエストでのみ信用する設計とする（S21）。
- **決定C**: AgentCore Gateway の構築（Terraform・インフラ・実機検証）は logvalet リポジトリでは実装しない。別リポジトリでユーザーが実施し、logvalet 側は構築者向け参考ドキュメント（S28）のみ配送する。
- **決定D**: 単一テナント縮退の事前承認は撤回。複数ユーザー利用が大前提。per-user の Backlog トークン紐付けは AgentCore Identity の iss+sub × workload identity スコープで実現し、単一テナント固定 userID へのフォールバックは実装しない。
- **決定E**: Backlog OAuth のトークン管理は AgentCore Identity の CustomOauth2 credential provider に完全委譲する。logvalet は Gateway が注入する `Authorization: Bearer` トークンをリクエストごとに Backlog API へ転送する passthrough のみを実装し（S30）、tokenstore・OAuth callback・永続ストアでのトークン保管は HTTP モードでは不要になる。
- **決定F**: tokenstore は CLI/stdio（ローカル利用）専用であり、バックエンドはローカルストレージ（sqlite / tokens.json）のみに縮小する。tokenstore の DynamoDB バックエンドは削除する。

## 破壊的変更（リリースノート必須項目）

- 削除フラグ・環境変数 14 個: `--auth`, `--external-url`, `--oidc-issuer`, `--oidc-client-id`, `--oidc-client-secret`, `--cookie-secret`, `--allowed-domains`, `--allowed-emails`, `--signing-key`, `--idproxy-store*`（5個）。`--auth-mode=oidc` は起動時エラー。`--auth-mode=bearer` は `apikey` の別名として残す。
- HTTP モードでの `--token-store=memory` および space store 未指定は警告からエラーへ格上げ（stdio モードは memory 既定を維持）。
- tokenstore の DynamoDB バックエンドを削除（決定F）。`--token-store=dynamodb`、`--token-store-dynamodb-table`、`--token-store-dynamodb-region`（および対応する環境変数）が削除され、`--token-store` は memory/sqlite のみを受け付ける。
- `go.mod` から `github.com/mark3labs/mcp-go` と `github.com/youyo/idproxy`（および派生する `coreos/go-oidc`, `gorilla/securecookie`, `redis/go-redis`）が消える。
