# Changelog

## [Unreleased]

### Added
- feat(mcp): 全 42 ツールに ToolAnnotations を付与し、読み取り系の自動実行を有効化
  - Read-only 32 件: `*_list`, `*_get`, `*_stats`, `*_health`, `*_digest` 等は `readOnlyHint=true` で確認ダイアログなし
  - Write 非冪等 3 件: `issue_create`, `issue_comment_add`, `document_create` は `idempotentHint=false`
  - Write 冪等 6 件: `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` は `idempotentHint=true`
  - Destructive 1 件: `watching_delete` は `destructiveHint=true` で強い確認ダイアログを表示
