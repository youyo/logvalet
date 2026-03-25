---
title: CLI ユーザー向けテキストの英語化
project: logvalet
author: planning-agent
created: 2026-03-25
status: Ready for Review
---

# CLI ユーザー向けテキストの英語化

## Context

logvalet の CLI ヘルプテキスト・エラーメッセージ・プロンプト・stderr 出力がすべて日本語で書かれている。
CLI ツールとしての国際的な利用やエージェント連携を考慮し、ユーザー向けテキストをすべて英語に統一する。

**対象**: help タグ、エラーメッセージ（`fmt.Errorf`）、ユーザー向け出力（stderr/stdout のメッセージ）、プロンプト文字列
**対象外**: Go コードコメント（`//`）、テストファイル、Backlog API のステータス名マッチング用文字列（`"完了"` 等）

## スコープ

### 変更対象ファイル一覧

#### internal/cli/ (メイン: help タグ + エラーメッセージ)

| ファイル | 変更内容 |
|---------|---------|
| `root.go` | help タグ 14件 |
| `global_flags.go` | help タグ 14件、バリデーションエラー 1件 |
| `issue.go` | help タグ ~30件、エラーメッセージ ~20件 |
| `resolve.go` | エラーメッセージ ~20件 (**注意**: `closedStatusNames` の `"完了"` は Backlog API マッチング用なので変更しない) |
| `auth.go` | help タグ 4件、エラーメッセージ ~10件 |
| `config_cmd.go` | help タグ 8件、エラーメッセージ/プロンプト ~15件 |
| `document.go` | help タグ ~12件、エラーメッセージ ~3件 |
| `digest_cmd.go` | help タグ ~8件、エラーメッセージ ~8件 |
| `activity.go` | help タグ 4件 |
| `meta.go` | help タグ 8件 |
| `space.go` | help タグ 3件 |
| `team.go` | help タグ 4件 |
| `user.go` | help タグ 6件 |
| `project.go` | help タグ 3件 |
| `completion.go` | help タグ 2件 |
| `validate.go` | エラーメッセージ 5件 |
| `runner.go` | エラーメッセージ ~6件 |
| `version_cmd.go` | (コメントのみ、変更なし) |

#### internal/render/ (ユーザー向け出力)

| ファイル | 変更内容 |
|---------|---------|
| `render.go` | エラーメッセージ 1件 |
| `gantt.go` | 出力テキスト ~5件（`"(データなし)"`, `"📅 タスク一覧"`, `"警告:"`, `"課題"` 等） |
| `markdown.go` | 出力テキスト 1件（`"(データなし)"`） |

#### internal/digest/ (エラーメッセージ)

| ファイル | 変更内容 |
|---------|---------|
| `activity_filter.go` | エラーメッセージ 1件 |
| `document.go` | 警告メッセージ 1件 |
| `space.go` | 警告メッセージ 1件 |

### 変更しないもの

- Go コードコメント（`//`）: 内部ドキュメントなので日本語のまま
- テストファイル（`*_test.go`）: 変更しない
- `resolve.go:15` の `closedStatusNames = []string{"完了"}`: Backlog API が返す日本語ステータス名とのマッチングに使用されるため変更不可
- `issue.go:220` の `"中"` / `"Normal"`: Backlog API の優先度名マッチング用
- `domain/`, `backlog/`, `config/`, `credentials/`, `version/`: コメントのみで変更不要

## 実装手順

### Step 1: internal/cli/ の help タグを英語化

全 CLI ファイルの `help:"..."` タグを英語に変換。

**翻訳方針:**
- 簡潔で標準的な CLI ヘルプスタイル（小文字開始、ピリオドなし）
- 技術用語はそのまま（project key, issue type, etc.）

**例:**
```go
// Before
help:"課題一覧を取得する"
// After
help:"list issues"

// Before
help:"プロジェクトキー (--status open/名前指定時に必須)"
// After
help:"project key (required when --status uses open or name)"

// Before
help:"ステータス (not-closed, open, 名前, カンマ区切り, 数値ID)。open/名前指定は -k 必須"
// After
help:"status filter (not-closed, open, name, comma-separated, numeric ID). -k required for open/name"
```

### Step 2: internal/cli/ のエラーメッセージを英語化

`fmt.Errorf(...)` 内の日本語メッセージを英語に変換。

**例:**
```go
// Before
fmt.Errorf("プロジェクトキー %q の解決に失敗: %w", key, err)
// After
fmt.Errorf("failed to resolve project key %q: %w", key, err)

// Before
fmt.Errorf("--content と --content-file は同時に指定できません (exit 2)")
// After
fmt.Errorf("--content and --content-file are mutually exclusive (exit 2)")
```

### Step 3: config_cmd.go のプロンプト・出力メッセージを英語化

対話型セットアップのプロンプトと完了メッセージ。

**例:**
```go
// Before
"API Key (空欄でスキップ)"
// After
"API Key (leave empty to skip)"

// Before
"セットアップ完了！ logvalet project list で動作確認できます\n"
// After
"Setup complete! Run logvalet project list to verify.\n"
```

### Step 4: internal/render/ の出力テキストを英語化

gantt.go と markdown.go のユーザー向け出力。

**例:**
```go
// Before
fmt.Fprintf(w, "📅 タスク一覧 (%d/%d 〜 %d/%d)\n\n", ...)
// After
fmt.Fprintf(w, "📅 Tasks (%d/%d – %d/%d)\n\n", ...)

// Before
fmt.Fprintf(w, "| 課題 | %s |\n", ...)
// After
fmt.Fprintf(w, "| Issue | %s |\n", ...)

// Before
fmt.Fprintln(w, "(データなし)")
// After
fmt.Fprintln(w, "(no data)")

// Before
fmt.Fprintf(os.Stderr, "警告: %d 件の課題は日付未設定のためスキップしました\n", skipped)
// After
fmt.Fprintf(os.Stderr, "warning: %d issue(s) skipped (missing dates)\n", skipped)
```

### Step 5: internal/digest/ のメッセージを英語化

```go
// Before
fmt.Sprintf("プロジェクト一覧の取得に失敗しました: %v", err)
// After
fmt.Sprintf("failed to list projects: %v", err)

// Before
fmt.Sprintf("ディスク使用量の取得に失敗しました: %v", err)
// After
fmt.Sprintf("failed to get disk usage: %v", err)

// Before
fmt.Errorf("activities 取得失敗 (maxID=%d): %w", maxID, err)
// After
fmt.Errorf("failed to fetch activities (maxID=%d): %w", maxID, err)
```

### Step 6: テスト実行・検証

```bash
go test ./...
go vet ./...
go build -o logvalet ./cmd/logvalet/
./logvalet --help
./logvalet issue list --help
./logvalet config init --help
```

### Step 7: SKILL.md の gantt/md 出力サンプル更新

gantt.go の出力ヘッダが英語に変わるため、SKILL.md の Output conventions セクションの記述も必要に応じて更新。

## 注意事項

- `resolve.go` の `closedStatusNames` (`"完了"`) は Backlog API が返すステータス名とのマッチングに使うため絶対に変更しない
- `issue.go` の優先度デフォルト値 `"中"` / `"Normal"` も Backlog API マッチング用なので変更しない
- テストファイル内の日本語文字列はそのまま（テストの期待値として使われている）
- Go コメントは変更しない（内部ドキュメント）

## テスト設計

N/A（テスト対象外: ドキュメント/テキスト変更のため。既存テストの通過で検証）

## チェックリスト

- [x] 観点1: 実装実現可能性 — 全ファイル・全行を特定済み
- [x] 観点2: TDD — N/A（テキスト変更のみ）
- [x] 観点3: アーキテクチャ整合性 — 既存パターン維持、機能変更なし
- [x] 観点4: リスク — `closedStatusNames` 等の機能的文字列を変更しないことを明記
- [x] 観点5: シーケンス図 — N/A（処理フロー変更なし）
