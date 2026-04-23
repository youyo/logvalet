---
title: マイルストーン M04 - ドキュメント整備・E2E 動作確認
project: logvalet
author: devflow:planning-agent
created: 2026-04-23
status: Ready for Review
complexity: M
parent: plans/playful-dreaming-peacock.md
---

# M04: ドキュメント整備・E2E 動作確認

## 概要

M01〜M03 の変更（MCP ツール 14 種新規追加、既存 6 種へのパラメータ追加、命名・型統一による破壊的変更）を README / CHANGELOG / skills に反映し、v0.16.0 としてリリース可能な状態に整える。

## スコープ

### 実装範囲

1. `README.md` — MCP セクションの最新化（ツール数・新ツール・20MB 上限・破壊的変更）
2. `README.ja.md` — README.md と同内容を日本語で反映
3. `skills/report/SKILL.md` — 旧パラメータ名の調査結果反映
4. `CHANGELOG.md` — `[Unreleased]` を `[0.16.0]` として確定し、M02 新ツール情報を追記
5. `plans/playful-dreaming-peacock.md` — status を `Draft` → `Released` に更新
6. E2E 動作確認: `go test ./...` / `go vet ./...` をパスさせる

### スコープ外

- コード変更（M01〜M03 で完了済み）
- `docs/specs/logvalet_full_design_spec_with_architecture.md` の更新（親プランには含まれていたが、本計画では時間配分と ROI を考慮して対象外とする。必要なら後続タスクで実施）
- `docs/specs/logvalet_SKILL.md` の更新（同上）
- Homebrew tap や GoReleaser 関連の設定変更
- MCP Inspector を用いた対話的 E2E（CI 環境では実行できないため手動検証のチェックリストに留める）

## 事前調査結果（重要）

### 調査 1: `skills/*.md` 内の旧パラメータ名使用状況

実行: `grep -rn -E "(--limit|--project-id|--pr-id|logvalet_|user_id:|project_id:)" skills/`

| ファイル:行 | 内容 | 判定 |
|------------|------|------|
| `skills/report/SKILL.md:69` | `lv user activity USER_ID ... --limit 1000 -f json` | **CLI 呼び出し例、修正不要** |

**重要な発見:** CLI の `user activity` は `DigestFlags` 経由で `--limit` フラグを持ち、これは M01〜M03 で改名されていない。MCP 側のみ `limit` → `count` に統一された（C1）。skills 内には MCP ツール直接呼び出し（`logvalet_*`）はなく、全て CLI コマンド例（`lv ...` / `logvalet ...`）。よって **skills/*.md は修正不要**。

### 調査 2: README.md 例文内の `--pr-id`

| ファイル:行 | 内容 | 処理 |
|------------|------|------|
| `README.md:462` | `logvalet star add --pr-id pr456` | `--pull-request-id` を正式名として例示に変更（alias `--pr-id` は維持、注記で言及） |
| `README.ja.md:469` | `logvalet star add --pr-id pr456` | 同上 |

### 調査 3: 現状の MCP ツール数表記

| ファイル:行 | 現状 | 更新後 |
|------------|------|--------|
| `README.md:507` | "31+ tools" | "56 tools" |
| `README.md:528` | "全 42 ツールに ToolAnnotations" | "全 56 ツールに ToolAnnotations" |
| `README.md:531` table 件数 | Read-only 32 / Write 非冪等 3 / Write 冪等 6 / Destructive 1 | 再集計が必要（下記参照） |
| `README.ja.md:514` | "31 個以上のツール" | "56 個のツール" |

### 調査 4: annotation 分類の実測（`internal/mcp/tool_categories.go` を直接集計）

`awk '/Category<X>, "/{c++} END{print c}'` で実装から直接集計:

| カテゴリ | 件数 | 備考 |
|---------|------|------|
| Read-only | **45** | 既存 32 + M02 新規 13（download 系 2 件を含む） |
| Write 非冪等 | **3** | `issue_create`, `issue_comment_add`, `document_create` |
| Write 冪等 | **6** | `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` |
| Destructive | **2** | `watching_delete`, `issue_attachment_delete`（M02 追加） |
| **合計** | **56** | `TestToolCategories_CoversAllRegisteredTools` が実ツール登録と一致保証 |

**注意:** `tool_categories.go` 内のコメントは "Read-only (32+14=46)" と書いているが実数は 45（M02 で新規追加されたのは `user_me`, `user_activity`, `digest_unified`, `activity_digest`, `document_tree`, `document_digest`, `space_digest`, `space_disk_usage`, `meta_version`, `meta_custom_field`, `team_project`, `issue_attachment_download`, `shared_file_download` の 13 件。`issue_attachment_delete` は Destructive）。実装コメントの誤記は M04 のスコープ外（必要なら別途修正）。

### 調査 5: README.ja.md に annotation 分類セクションが存在しない（重要発見）

`grep -n "annotation\|ToolAnnotations\|Read-only\|読み取り\|冪等\|Destructive\|破壊" README.ja.md` が 0 件。英語版 README.md L526-536 にしかない。よって README.ja.md 側は**新規追加**の作業になる（更新ではない）。

## ドキュメント更新詳細

### 1. README.md

#### 1.1 MCP サーバーセクション（L495-525 付近）

既存:
```
The MCP server provides 31+ tools including:
- `logvalet_issue_get`, `logvalet_issue_list`, `logvalet_issue_create`
- ...
- And many more...
```

更新後（案）:
```
The MCP server exposes **56 tools** covering essentially every operation available in the CLI. For every CLI subcommand, the MCP server provides an equivalent tool with the same parameters (renamed to `snake_case` and typed as JSON Schema).

Representative tools:
- Issue: `logvalet_issue_get`, `logvalet_issue_list`, `logvalet_issue_create`, `logvalet_issue_update`, `logvalet_issue_comment_{list,add,update}`, `logvalet_issue_attachment_{list,get,download,delete}`
- Project: `logvalet_project_{get,list,blockers,health}`, `logvalet_user_workload`
- Digest: `logvalet_digest`, `logvalet_digest_unified`, `logvalet_digest_{weekly,daily}`, `logvalet_space_digest`, `logvalet_activity_digest`, `logvalet_document_digest`
- Document: `logvalet_document_{get,list,tree,create}`
- Meta: `logvalet_meta_{status,category,version,custom_field}`
- User/Team: `logvalet_user_{me,list,get,activity}`, `logvalet_team_{list,project}`
- Space/Shared File: `logvalet_space_{info,disk_usage}`, `logvalet_shared_file_{list,get,download}`
- Star/Watching: `logvalet_star_add`, `logvalet_watching_{list,count,get,add,update,delete,mark_as_read}`
- Analysis: `logvalet_issue_{context,stale,timeline,triage_materials}`, `logvalet_activity_{list,stats}`
```

#### 1.2 Binary download size limit（新規追加、MCP セクション内）

```
### Binary Download Size Limit

`logvalet_issue_attachment_download` and `logvalet_shared_file_download` return the file contents as base64 in the JSON response. To keep MCP responses manageable and avoid client-side truncation:

- **Maximum size: 20 MB**
- Enforced at the Backlog HTTP client layer — if the `Content-Length` header exceeds 20 MB the request fails fast with an explicit error (no data is buffered).
- For larger files, use the CLI: `logvalet issue attachment download <KEY> <ID> --output <path>`.
```

#### 1.3 annotation 分類テーブル（L528-536）

更新後:
```
logvalet MCP サーバーは全 56 ツールに ToolAnnotations を付与しています。

| カテゴリ | 件数 | 対象ツール例 | 挙動 |
|---|---|---|---|
| Read-only | 45 | `*_list`, `*_get`, `*_stats`, `*_health`, `*_digest`, `*_download` 等 | 確認ダイアログなしで自動実行 |
| Write 非冪等 | 3 | `issue_create`, `issue_comment_add`, `document_create` | 通常の書き込み確認 |
| Write 冪等 | 6 | `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` | 通常の書き込み確認 |
| Destructive | 2 | `watching_delete`, `issue_attachment_delete` | 強い確認ダイアログを表示 |
```

#### 1.4 Breaking Changes（v0.16.0）セクション（MCP セクションの末尾に新規追加）

```
### Breaking Changes in v0.16.0

v0.16.0 introduces the following parameter renames to align MCP tool schemas with the CLI. MCP clients that used the old names must update their invocations.

| ID | Change | Affected tools | Before | After |
|----|--------|----------------|--------|-------|
| C1 | Pagination parameter unified to `count` | `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`, `logvalet_shared_file_list` | `limit: 50` | `count: 50` |
| C2 | `user_id` now string-only (`"me"` or numeric string) | `logvalet_watching_list` | `user_id: 12345` (number) | `user_id: "12345"` / `user_id: "me"` |
| C3 | `project_id` → `project_key` | `logvalet_document_list` | `project_id: 9999` (number) | `project_key: "PROJ"` (string) |
| C4 | CLI flag rename (backward-compatible alias) | `logvalet star add` | `--pr-id <id>` | `--pull-request-id <id>` (old `--pr-id` kept as alias) |

Migration note: MCP clients send parameter names as JSON keys; sending the old names will result in the parameter being silently ignored by the MCP framework (not an explicit error). Update integration code accordingly.
```

#### 1.5 Star 例文（L462）

変更:
```diff
- logvalet star add --pr-id pr456
+ logvalet star add --pull-request-id pr456
```

（注記: コメント行で `--pr-id` は alias として引き続き動作する旨を追記）

### 2. README.ja.md

README.md の 1.1〜1.5 と同内容を日本語で反映。該当行:

| 対応 | README.md | README.ja.md | 備考 |
|------|-----------|--------------|------|
| 1.1 ツール一覧 | L507-522 | L514-529 | 更新 |
| 1.2 20MB 上限 | MCP セクション | MCP セクション | 新規追加 |
| 1.3 annotation 分類 | L528-536 | （存在しない） | **新規追加**（英語版 README.md からの日本語訳セクションを追加） |
| 1.4 破壊的変更 | MCP セクション末尾 | MCP セクション末尾 | 新規追加 |
| 1.5 `--pr-id` 例文 | L462 | L469 | 更新 |

### 3. skills/report/SKILL.md

事前調査の結果、**修正不要**。CLI の `user activity` は `DigestFlags` 経由の `--limit` を現在も保持しているため、既存のコマンド例は正しい。

### 4. CHANGELOG.md

現状の `[Unreleased]` セクションを `[0.16.0] - 2026-04-23` として確定し、M02 の新規ツール情報を追記。

更新後の構造:
```markdown
# Changelog

## [0.16.0] - 2026-04-23

### Added
- feat(mcp): 新規ツール 14 種を追加（CLI/MCP のフィーチャーパリティ達成）
  - `logvalet_user_me`, `logvalet_user_activity`
  - `logvalet_digest_unified`, `logvalet_activity_digest`, `logvalet_document_digest`, `logvalet_space_digest`
  - `logvalet_document_tree`
  - `logvalet_space_disk_usage`
  - `logvalet_meta_version`, `logvalet_meta_custom_field`
  - `logvalet_team_project`
  - `logvalet_issue_attachment_delete`, `logvalet_issue_attachment_download`
  - `logvalet_shared_file_download`
  - バイナリダウンロード系ツールは base64 エンコードで返却、サイズ上限 20MB
- feat(mcp): 既存 6 ツールにパラメータを追加（M01、実装実測に基づく）
  - `logvalet_issue_create`: `parent_issue_id`, `category_ids`, `version_ids`, `milestone_ids`, `due_date`, `start_date`, `notified_user_ids`
  - `logvalet_issue_update`: `issue_type_id`, `category_ids`, `version_ids`, `milestone_ids`, `due_date`, `start_date`, `notified_user_ids`
  - `logvalet_issue_list`: `start_date`, `updated_since`, `updated_until`
  - `logvalet_issue_comment_add`: `notified_user_ids`
  - `logvalet_activity_list`: `activity_type_ids`, `order`（`offset` は Backlog API の制約により実装から除外。親プラン A5 の `offset` は未採用）
  - `logvalet_team_list`: `count`, `offset`, `no_members`
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
...（既存のまま）
```

### 5. plans/playful-dreaming-peacock.md

フロントマターの status を更新:
```diff
-status: Draft
+status: Released
```

## テスト設計書

テスト設計は **N/A（対象外）**：ドキュメント・マニフェストのみの変更で、自動テスト対象なし。代替として以下の E2E 確認を実施する:

### E2E 確認チェックリスト

1. [ ] `go test ./...` が全パス（コード変更はないので結果は M03 時点と同じはず）
2. [ ] `go vet ./...` が警告ゼロ
3. [ ] `grep -n "31+" README.md` が 0 件
4. [ ] `grep -n "31 個以上" README.ja.md` が 0 件
5. [ ] `grep -n "42 ツール" README.md` が 0 件
6. [ ] `grep -n "v0.16.0" CHANGELOG.md` が存在する
7. [ ] `grep -n "Unreleased" CHANGELOG.md` が 0 件
8. [ ] `grep -n "^status: Released$" plans/playful-dreaming-peacock.md` が 1 件
9. [ ] Markdown Lint（任意、vale などを使っていれば）

### 手動 E2E（README の記載内容検証、可能な範囲で）

1. [ ] MCP Inspector で `tools/list` を叩き、ツール総数が 56 となることを確認（任意、ローカル実行）
2. [ ] `logvalet star add --pr-id pr456` が deprecation warning と共に成功することを確認
3. [ ] `logvalet star add --pull-request-id pr456` が正常動作することを確認

## 実装手順

### Step 1: CHANGELOG.md 更新（先行、確定事実のみ）
- ファイル: `CHANGELOG.md`
- 内容: `[Unreleased]` → `[0.16.0] - 2026-04-23`。M02 の 14 ツール分のリストを Added に追記（既存リストは残し、前に追加）。

### Step 2: README.md 更新
- ファイル: `README.md`
- 順序: 1.1（ツール一覧再構成）→ 1.2（20MB 上限追加）→ 1.3（annotation 分類更新）→ 1.4（Breaking Changes セクション新規追加）→ 1.5（`--pr-id` 例文更新）
- 依存: Step 1

### Step 3: README.ja.md 更新
- ファイル: `README.ja.md`
- README.md と同じ構造で反映。表現は既存の日本語調に合わせる（ですます調 / 箇条書きスタイル）
- 依存: Step 2

### Step 4: plans/playful-dreaming-peacock.md の status 更新
- ファイル: `plans/playful-dreaming-peacock.md`
- フロントマターの `status: Draft` を `status: Released` に変更
- 依存: Step 1-3

### Step 5: 動作確認
- コマンド: `go test ./... && go vet ./...`
- E2E チェックリストを順に実行
- 依存: Step 1-4

### Step 6: git commit
- ブランチ: 現在の `main` にそのままコミット（Conventional Commits 形式）
- メッセージ: `docs: v0.16.0 リリースに向けてドキュメント整備（M04）`
- 同梱ファイル: README.md, README.ja.md, CHANGELOG.md, plans/playful-dreaming-peacock.md, plans/logvalet-m04-documentation.md
- `Plan:` フッターを追加

## アーキテクチャ整合性

| 観点 | 判定 |
|------|------|
| 既存の命名規則に従っているか | ✅ 変更なし |
| 設計パターンが一貫しているか | ✅ 既存 README 構造を踏襲 |
| モジュール分割が適切か | N/A（ドキュメント） |
| 依存方向が正しいか | N/A |
| 類似機能との統一性 | ✅ CHANGELOG は Keep a Changelog 準拠 |

## リスク評価

| リスク | 重大度 | 対策 |
|--------|-------|------|
| 破壊的変更の告知不足で既存 MCP ユーザーが混乱 | 高 | README と CHANGELOG の両方に Breaking Changes を明記、C1〜C4 を表形式で列挙。Migration note を記載 |
| annotation 分類件数の再集計ミス | 中 | 実装コードを grep して検証可能な方法で集計（`grep -rn "writeAnnotation\|readOnlyAnnotation" internal/mcp/` で確認） |
| README.md と README.ja.md の内容不一致 | 中 | 同一の情報構造を保つ。差分チェック時に両方並べて確認 |
| `--pr-id` alias が実装されているか確認漏れ | 低 | Step 5 の動作確認で `logvalet star add --pr-id` が成功することを検証 |
| CHANGELOG リリース日付の誤り | 低 | 2026-04-23 を使用（システム日付） |
| バイナリダウンロード 20MB 上限が実装されているか | 低 | M02 B13 のテストで検証済みの想定（コード側は触らない） |

## ロールバック計画

- 本 M04 はコード変更を含まないため、問題発生時は `git revert <commit-hash>` で即座にロールバック可能。
- リリースタグを切る前にレビューを経る（Phase 3.5 / Phase 4 の advisor）。

## シーケンス図

ドキュメント更新のみのため **N/A**。変更フローは以下の静的リスト:

```
git (main)
  └─ commit {hash}: docs: v0.16.0 リリースに向けてドキュメント整備（M04）
       ├─ CHANGELOG.md          (Unreleased → 0.16.0)
       ├─ README.md             (MCP セクション更新)
       ├─ README.ja.md          (MCP セクション更新)
       ├─ plans/playful-dreaming-peacock.md (status: Draft → Released)
       └─ plans/logvalet-m04-documentation.md (新規、本計画)
```

## チェックリスト

### 観点1: 実装実現可能性（5項目）
- [x] 手順の抜け漏れがないか（6 ステップで CHANGELOG → README → README.ja → plan → 検証 → commit）
- [x] 各ステップが十分に具体的か（該当行番号と差分例を記載）
- [x] 依存関係が明示されているか（Step 1 → 2 → 3 → 4 → 5 → 6）
- [x] 変更対象ファイルが網羅されているか（4 ファイル + 本計画ファイル）
- [x] 影響範囲が正確に特定されているか（MCP ユーザー向けドキュメント、Homebrew ユーザー向け CHANGELOG）

### 観点2: TDDテスト設計の品質（6項目）
- [x] N/A（ドキュメント変更のため）。代替として E2E チェックリスト 8 項目を定義。

### 観点3: アーキテクチャ整合性（5項目）
- [x] 既存の命名規則に従っているか
- [x] 設計パターンが一貫しているか（Keep a Changelog）
- [x] モジュール分割が適切か（該当なし）
- [x] 依存方向が正しいか（該当なし）
- [x] 類似機能との統一性があるか

### 観点4: リスク評価と対策（6項目）
- [x] リスクが適切に特定されているか
- [x] 対策が具体的か
- [x] フェイルセーフが考慮されているか（`git revert`）
- [x] パフォーマンスへの影響が評価されているか（N/A、ドキュメント）
- [x] セキュリティ観点が含まれているか（新 API トークン等の漏洩なし）
- [x] ロールバック計画があるか

### 観点5: シーケンス図の完全性（5項目）
- [x] N/A（ドキュメント変更のため）
