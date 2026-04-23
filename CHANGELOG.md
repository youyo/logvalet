# Changelog

## [Unreleased]

### Added
- feat(mcp): idproxy v0.3.0 取り込みと `--refresh-token-ttl` / `LOGVALET_MCP_REFRESH_TOKEN_TTL` を追加
  - OAuth 2.1 refresh_token grant が有効化
  - 未指定時は idproxy デフォルトの 30 日が適用される
  - 値は `time.ParseDuration` 互換（例: `720h` = 30 日）

### Changed (BREAKING)
- feat(mcp): **[BREAKING]** `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`,
  `logvalet_shared_file_list` のページネーションパラメータ `limit` を `count` に統一
  - 旧パラメータ名 `limit` は削除。`count` のみ受け付ける
  - MCP フレームワークが unknown param を無視するため、`limit` を渡しても暗黙的に無視される（エラーにはならない）
- feat(mcp): **[BREAKING]** `logvalet_document_list.project_id`（数値型）を `project_key`（文字列型）に変更
  - CLI の `--project-key` と整合性を持たせるため。内部で `GetProject` を呼び出し数値 ID に解決する
- feat(mcp): **[BREAKING]** `logvalet_watching_list.user_id` を数値型から文字列型に変更
  - `"me"`（GetMyself で解決）または `"12345"` のような数値文字列で指定する
  - 数値型（JSON の number）を渡した場合は型不一致として扱われる

### Changed
- feat(cli): `star add --pr-id` フラグを `--pull-request-id` に改名（`--pr-id` は alias として維持）
- chore(deps): `github.com/youyo/idproxy` を v0.2.2 から v0.3.0 に更新

## [0.14.0] - 2025-02-19

### Added
- feat(mcp): 全 42 ツールに ToolAnnotations を付与し、読み取り系の自動実行を有効化
  - Read-only 32 件: `*_list`, `*_get`, `*_stats`, `*_health`, `*_digest` 等は `readOnlyHint=true` で確認ダイアログなし
  - Write 非冪等 3 件: `issue_create`, `issue_comment_add`, `document_create` は `idempotentHint=false`
  - Write 冪等 6 件: `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` は `idempotentHint=true`
  - Destructive 1 件: `watching_delete` は `destructiveHint=true` で強い確認ダイアログを表示
