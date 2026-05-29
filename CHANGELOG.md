# Changelog

## [0.29.1] - 2026-05-29

マルチスペース OAuth の独立した 2 バグを修正。

### Fixed
- fix(mcp): OAuth callback で `SpaceRegistration` のタイムスタンプ（`created_at`/`updated_at`/`last_verified_at`）が未設定のまま保存され、本番 DynamoDB 経路で `logvalet_space_list` の値がゼロ値（`0001-01-01T00:00:00Z`）になっていた問題を修正
  - handler 側で `updated_at`/`last_verified_at` を現在時刻に設定し、`created_at` は既存レコードが非ゼロなら保持（`IsZero` ガード）。既存のゼロ値レコードも次回再認証で自己修復する
- fix(auth): 非プライマリスペース（megumilog 等）の OAuth アクセストークンのリフレッシュが常にデフォルト（heptagon）の token エンドポイントを叩いて失敗していた問題を修正（#14）
  - `TokenRefresher.RefreshToken` / `OAuthProvider.RefreshToken` / `TokenManager.GetValidToken` に `baseURL` 引数を追加（空ならデフォルトにフォールバック）
  - `SpaceAwareClientFactory` が `reg.BaseURL` を伝播し、スペースごとに正しい token エンドポイントを使用

## [0.29.0] - 2026-05-21

multi-space 呼び出し時に `AnalysisEnvelope` の `result.space` / `result.base_url` メタデータがスタートアップ設定値（heptagon）のままになっていた問題を修正（M22 secondary）。
`callWithSpaceClient` で `SpaceRegistration` を context に注入し、各 builder call site で `spaceInfoFromContext` を介して正しい space/baseURL を取得するよう変更。
`logvalet_my_tasks spaces=["megumilog"]` 時に `result.space == "megumilog"`、`result.base_url == "https://megumilog.backlog.jp"` が返るようになった。

### Fixed
- fix(mcp): multi-space 呼び出し時に `AnalysisEnvelope.Space`/`BaseURL` が常にスタートアップ設定値になっていた問題を修正（M22 secondary）
  - `callWithSpaceClient` / fan-out クロージャで `SpaceRegistration` を context に注入
  - `tools_analysis.go`（12箇所）・`tools_space.go`・`tools_activity.go`・`tools_document.go` の builder call site を `spaceInfoFromContext` 経由に変更
- fix(mcp): `callWithSpaceClient` / fan-out で `r.spaceFactory == nil` の場合に panic していた問題を修正（nil guard 追加）
- fix(test): `my_tasks_test.go` の `callCount` データレースを `sync/atomic.Int32` で修正

### Added
- test(mcp): `spaceInfoFromContext` の単体テスト（T1: fallback / reg-in-context / empty-reg）
- test(mcp): `callWithSpaceClient` / fan-out の context 注入テスト（T2〜T3）
- test(mcp): `MyTasksBuilder` / `ProjectHealthBuilder` の envelope メタデータ回帰テスト（T4）
- test(mcp): digest builder 3種（SpaceDigest / ActivityDigest / DocumentDigest）の envelope メタデータ回帰テスト（TD-1〜TD-3）
- test(mcp): single-space / resolver=nil 時の backward compat テスト（T5）

### Changed
- feat(mcp): `spaceInfoFromContext` が不完全な reg（Alias or BaseURL 空）を検出した場合 `slog.Warn` でログ出力

## [0.28.0] - 2026-05-21

`spaces=["megumilog"]` 指定時に megumilog の API を呼ぶよう修正（M22）。
`cli/mcp.go` の OAuth モードで `SpaceClientFactory` が `oauthDeps.Factory`（heptagon 固定）をラップしており `reg.BaseURL`/`reg.Tenant` を無視していた根本バグを修正。
`auth.NewSpaceAwareClientFactory` に置き換えることで各スペースの正しい baseURL にルーティングされるようになった。

### Fixed
- fix(cli): OAuth モードの `SpaceClientFactory` が `reg.BaseURL`/`reg.Tenant` を無視して常に heptagon クライアントを返していた問題を修正
  - `oauthDeps.Factory(ctx)` を `auth.NewSpaceAwareClientFactory(oauthDeps.TokenManager, nil)` に置き換え
  - `spaces=["megumilog"]` 指定時に megumilog の API が正しく呼ばれるようになった

### Added
- test(auth): `TestSpaceAwareClientFactory_OAuth_RoutesToRegBaseURL` を追加（マルチスペースルーティングの回帰防止）

## [0.27.0] - 2026-05-21

`RegisterWithSpaces` / `RegisterWithSpacesWrite` の InputSchema に `spaces` / `all_spaces` パラメータを自動注入（M21）。
Claude.ai 等 MCP クライアントが InputSchema 未宣言パラメータを送信しない仕様に対応。
これにより `logvalet_my_tasks spaces=["megumilog"]` のように fan-out 先スペースを明示指定できるようになった。

### Added
- feat(mcp): `RegisterWithSpaces` が tool.InputSchema.Properties に `spaces`（配列型）と `all_spaces`（ブール型）を自動注入
- feat(mcp): `RegisterWithSpacesWrite` が tool.InputSchema.Properties に `spaces`（配列型）を自動注入
- feat(mcp): `injectSpaceParams` / `injectSpaceParamWrite` ヘルパー追加

## [0.26.0] - 2026-05-21

`logvalet_space_use` で設定したデフォルトスペースが全ツールに反映されるようになった（M20）。
これまで `spaces` パラメータを省略すると起動時設定スペースが使われていたが、DynamoDB UserPreference を参照するよう修正。

### Fixed
- fix(mcp): `RegisterWithSpaces` / `RegisterWithSpacesWrite` が `spaces` 未指定時に DynamoDB UserPreference を無視していた問題を修正
  - `logvalet_space_use megumilog` 後に `logvalet_my_tasks` を呼ぶと heptagon のデータが返っていたバグを解消
  - 解決優先順位: DynamoDB preference → 単一 enabled space → default client fallback

### Added
- feat(mcp): `callWithSpaceClient` ヘルパー追加（spaceFactory 経由クライアント生成 + needsAuthorization ハンドリング）

## [0.25.0] - 2026-05-20

Multi-space Backlog OAuth の baseURL 動的切り替え修正（M19）。`logvalet_space_connect_url` で生成した URL で OAuth を完走すると、指定した Backlog スペース（megumilog 等）のトークンが正しく保存されるようになった。

### Fixed
- fix(http): `MultiSpaceOAuthHandler` が常に heptagon.backlog.com へ OAuth リクエストしていた問題を修正（`CloneWithBaseURL` を使ってターゲットスペースの URL でプロバイダーをクローン）

### Added
- feat(auth/provider): `OAuthProvider` インターフェースに `CloneWithBaseURL(baseURL string) OAuthProvider` を追加
- feat(auth/provider): `BacklogOAuthProvider.CloneWithBaseURL` 実装（別スペースの baseURL で動作するシャローコピーを返す。`space.DeriveInitialTenant` でスペース名も更新）

### Refactor
- refactor(test): `TestHandleCallback_UsesStateBaseURL` に `CloneWithBaseURL` が targetBaseURL で呼ばれたことを検証する assertion を追加
- refactor(security): `spaceConnectURL` の `NormalizeBaseURL` 付近に SSRF 許可リスト実装 TODO コメントを追加

## [0.24.0] - 2026-05-20

Multi-space Backlog OAuth フロー（M18）リリース。`logvalet_space_connect_url` で生成した URL からブラウザ OAuth フローを完走し、複数の Backlog スペースを登録できるようになった。

### Added
- feat(auth): bootstrap_token 生成・検証・HKDF 鍵派生（`GenerateBootstrapToken`, `ValidateBootstrapToken`, `DeriveBootstrapKey`）
- feat(http): `MultiSpaceOAuthHandler.HandleAuthorize` に bootstrap_token 検証を統合
- feat(auth): `StateClaims.Flow` フィールド追加 + callback dispatcher（multi/single フロー振り分け）
- feat(auth): state JWT に Typ/Aud/Iss フィールドを追加（H1 backward compat 維持）
- feat(mcp): `spaceConnectURL` に bootstrap_token 統合（NonceStore.Store + jti one-time 管理）
- feat(cli): `GET /oauth/backlog/multi/authorize` を topMux に直登録（idproxy ラップ外）
- feat(cli): `LOGVALET_SPACE_STORE_TYPE=memory` + auth モード起動時に replay detection 制約警告を出力

### Fixed
- fix(space): DynamoDB `Consume` の ConditionExpression に `AND expires_at > :now` を追加（TTL 遅延期間中の期限切れ jti replay を防止）

### Refactor
- refactor(auth): `hashValue` のハッシュ長を 8→16 バイトに拡張
- refactor(cli): `InstallOAuthRoutes` を 2-arg に変更（multiSpaceHandler 引数撤去）

## [0.16.0] - 2026-04-23

CLI/MCP パラメータ統一リリース。MCP ツールを CLI と同等以上の機能セットに拡張し、命名・型の不整合を解消。
詳細は `plans/playful-dreaming-peacock.md` および M01〜M04 の各マイルストーン計画を参照。

### Added
- feat(mcp): 新規ツール 14 種を追加（CLI/MCP フィーチャーパリティ達成、M02）
  - `logvalet_user_me`, `logvalet_user_activity`
  - `logvalet_digest_unified`, `logvalet_activity_digest`, `logvalet_document_digest`, `logvalet_space_digest`
  - `logvalet_document_tree`
  - `logvalet_space_disk_usage`
  - `logvalet_meta_version`, `logvalet_meta_custom_field`
  - `logvalet_team_project`
  - `logvalet_issue_attachment_delete`, `logvalet_issue_attachment_download`
  - `logvalet_shared_file_download`
  - バイナリダウンロード系ツール (`issue_attachment_download`, `shared_file_download`) は base64 エンコードで返却。Backlog HTTP クライアント層で `Content-Length > 20MB` を早期エラーとして弾く
- feat(mcp): 既存 6 ツールにパラメータを追加（M01）
  - `logvalet_issue_create`: `parent_issue_id`, `category_ids`, `version_ids`, `milestone_ids`, `due_date`, `start_date`, `notified_user_ids`
  - `logvalet_issue_update`: `issue_type_id`, `category_ids`, `version_ids`, `milestone_ids`, `due_date`, `start_date`, `notified_user_ids`
  - `logvalet_issue_list`: `start_date`, `updated_since`, `updated_until`
  - `logvalet_issue_comment_add`: `notified_user_ids`
  - `logvalet_activity_list`: `activity_type_ids`, `order`
  - `logvalet_team_list`: `count`, `offset`, `no_members`
- feat(mcp): idproxy v0.3.0 取り込みと `--refresh-token-ttl` / `LOGVALET_MCP_REFRESH_TOKEN_TTL` を追加
  - OAuth 2.1 refresh_token grant が有効化
  - 未指定時は idproxy デフォルトの 30 日が適用される
  - 値は `time.ParseDuration` 互換（例: `720h` = 30 日）

### Changed (BREAKING)
- feat(mcp): **[BREAKING]** `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`,
  `logvalet_shared_file_list` のページネーションパラメータ `limit` を `count` に統一（M03 C1）
  - 旧パラメータ名 `limit` は削除。`count` のみ受け付ける
  - MCP フレームワークが unknown param を無視するため、`limit` を渡しても暗黙的に無視される（エラーにはならない）
- feat(mcp): **[BREAKING]** `logvalet_document_list.project_id`（数値型）を `project_key`（文字列型）に変更（M03 C3）
  - CLI の `--project-key` と整合性を持たせるため。内部で `GetProject` を呼び出し数値 ID に解決する
- feat(mcp): **[BREAKING]** `logvalet_watching_list.user_id` を数値型から文字列型に変更（M03 C2）
  - `"me"`（GetMyself で解決）または `"12345"` のような数値文字列で指定する
  - 数値型（JSON の number）を渡した場合は型不一致として扱われる

### Changed
- feat(cli): `star add --pr-id` フラグを `--pull-request-id` に改名（M03 C4、`--pr-id` は alias として維持）
- chore(deps): `github.com/youyo/idproxy` を v0.2.2 から v0.3.1 に更新（OAuth 2.1 refresh_token grant 対応含む）

### Migration Guide

MCP クライアント (Claude Desktop / Claude Code 等) から logvalet MCP サーバーを利用している場合:

- `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`, `logvalet_shared_file_list` の呼び出しで `limit` を使っていたら `count` に置換する
- `logvalet_document_list` の呼び出しで `project_id: 9999` を使っていたら `project_key: "PROJ"` に置換する
- `logvalet_watching_list` の呼び出しで `user_id: 12345` を使っていたら `user_id: "12345"` に置換（あるいは `"me"`）

CLI 利用者は `--pr-id` → `--pull-request-id` への切り替えが推奨されるが、`--pr-id` は当面 alias として動作する。

## [0.14.0] - 2025-02-19

### Added
- feat(mcp): 全 42 ツールに ToolAnnotations を付与し、読み取り系の自動実行を有効化
  - Read-only 32 件: `*_list`, `*_get`, `*_stats`, `*_health`, `*_digest` 等は `readOnlyHint=true` で確認ダイアログなし
  - Write 非冪等 3 件: `issue_create`, `issue_comment_add`, `document_create` は `idempotentHint=false`
  - Write 冪等 6 件: `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` は `idempotentHint=true`
  - Destructive 1 件: `watching_delete` は `destructiveHint=true` で強い確認ダイアログを表示
