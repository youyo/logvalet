# Changelog

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
