---
title: completion バグ修正 + assignee チーム名解決 + config.team_id 削除
project: logvalet
author: planning-agent
created: 2026-03-24
status: Draft
---

# completion バグ修正 + assignee チーム名解決 + config.team_id 削除

## コンテキスト

3つの改善:
1. zsh completion でフラグのタブ補完が効かない（`"${words[@]:1}"` が Kong に1文字列として渡されている）
2. `--assignee "チーム名"` でチームメンバー全員の課題を取得したい
3. `config.team_id` は不要（チーム名で直接解決できるようになったため）

---

## 修正1: zsh completion のフラグ補完バグ

### 原因
`internal/cli/completion.go:20` の zsh スクリプト:
```bash
completions=($(${words[1]} --completion-bash "${words[@]:1}"))
```
`"${words[@]:1}"` が1つの文字列として展開される → Kong が `"issue list --"` をパースできない。

### 修正
引用符を外して個別引数として展開:
```bash
completions=($(${words[1]} --completion-bash ${words[@]:1}))
```

### テスト
- `TestCompletionScript_noQuotes` — 生成スクリプトに `"${words[@]:1}"` が含まれないことを確認

---

## 修正2: `--assignee "チーム名"` サポート

### 設計
`resolveAssignee` のフォールバックチェーンにチーム名解決を追加:

```
"me" → GetMyself
数値 → 直接ID
文字列 → ユーザー名検索 → 一致なし → チーム名検索 → メンバーID展開
```

### 変更箇所
`internal/cli/resolve.go` の `resolveAssignee`:
- ユーザー名検索で `matched == 0` の場合、`ListTeams()` でチーム名を部分一致検索
- チーム一致 → `GetTeam(teamID)` でメンバー取得 → メンバーID展開
- チームも一致なし → 従来通りエラー（ユーザー名+チーム名の両方を利用可能一覧に表示）

### 既存の `"team"` ケース削除
- `input == "team"` の固定ケース（config.team_id 依存）を削除
- `teamID int` 引数を削除 → シグネチャを元に戻す: `resolveAssignee(ctx, input, client)`
- 呼び出し元（`issue.go`, `digest_cmd.go`）も `teamID` 引数を削除

### テスト

| ID | テスト | 入力 | 期待結果 |
|----|--------|------|---------|
| A1 | ユーザー名一致 | "Naoto Ishizawa" | ユーザーID |
| A2 | チーム名一致 | "株式会社ヘプタゴン全体" | メンバー全員のID |
| A3 | どちらも不一致 | "存在しない" | エラー（ユーザー名+チーム名の利用可能一覧） |
| A4 | チーム名部分一致 | "ヘプタゴン" | 部分一致で1件 → メンバーID |
| A5 | 既存テストが通る | "me", 数値, ユーザー名 | 変更なし |

---

## 修正3: `config.team_id` 削除

### 変更箇所
- `internal/config/config.go` — `ProfileConfig.TeamID` フィールド削除、`ResolvedConfig.TeamID` 削除
- `internal/config/config_test.go` — team_id 関連テスト削除
- `internal/config/testdata/valid_with_team_id.toml` — ファイル削除
- `internal/cli/issue.go` — `resolveAssignee` 呼び出しから `teamID` 引数削除
- `internal/cli/digest_cmd.go` — 同上
- `internal/cli/issue_list_test.go` — `teamID` 引数を使ったテスト修正
- `README.md` / `README.ja.md` — "Configuration: Team ID" セクション削除

---

## 変更対象ファイル

| ファイル | 変更内容 |
|---------|----------|
| `internal/cli/completion.go` | zsh スクリプトの引用符修正 |
| `internal/cli/completion_test.go` | 引用符なしのアサーション追加 |
| `internal/cli/resolve.go` | resolveAssignee からチーム名フォールバック追加、"team" ケース + teamID 引数削除 |
| `internal/cli/issue.go` | resolveAssignee 呼び出しの teamID 引数削除 |
| `internal/cli/digest_cmd.go` | 同上 |
| `internal/cli/issue_list_test.go` | テスト修正 + チーム名解決テスト追加 |
| `internal/config/config.go` | TeamID フィールド削除 |
| `internal/config/config_test.go` | team_id テスト削除 |
| `internal/config/testdata/valid_with_team_id.toml` | ファイル削除 |
| `README.md` / `README.ja.md` | Team ID セクション削除、`--assignee "チーム名"` 例追加 |

---

## コミット戦略

| # | メッセージ | 内容 |
|---|-----------|------|
| 1 | `fix(cli): zsh completion のフラグ補完を修正` | completion.go + テスト |
| 2 | `refactor(cli): resolveAssignee にチーム名フォールバックを追加し config.team_id を削除` | resolve.go, config.go, issue.go, digest_cmd.go + テスト |
| 3 | `docs: --assignee チーム名の利用例を追加し Team ID セクションを削除` | README |

---

## 検証方法

```bash
go test ./...

# completion 確認（zsh で再読み込み後）
eval "$(lv completion zsh --short)"
lv issue list --<TAB>

# チーム名で assignee
lv issue list --assignee "ヘプタゴン" --status not-closed --due-date this-week

# 従来のユーザー名も動作
lv issue list --assignee "Naoto Ishizawa" --status not-closed
```

---

## Next Action

> 1. `Skill(devflow:implement)` — このプランに基づいて実装を開始
