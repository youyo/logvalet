---
title: team 名前解決 + team list メンバー表示
project: logvalet
author: planning-agent
created: 2026-03-24
status: Draft
---

# team 名前解決 + team list メンバー表示

## コンテキスト

`lv digest --team 173843` は動作するが、チーム名での指定ができない。
`lv team list` がメンバー情報を含まない。

`this-week` の月曜始まりは `weekStart()` で正しく実装済み（3/23 は月曜日）。バグなし。

## スコープ

### 実装範囲
1. `--team` フラグを `[]int` → `[]string` に変更し、チーム名解決を追加
2. `ListTeams` の戻り値を `[]TeamWithMembers` に変更（メンバー含む）

### スコープ外
- `this-week` の修正（正常動作確認済み）

---

## 修正1: `--team` チーム名解決

### 変更箇所

**`internal/cli/digest_cmd.go`**:
- `Team []int` → `Team []string` に変更
- Run() のチーム解決ロジックで名前 → ID 変換を追加

**`internal/cli/resolve.go`**:
- `resolveTeamIDs(ctx, inputs []string, client) ([]int, error)` 関数を新規追加
- ロジック:
  1. 数値文字列 → そのまま ID
  2. 文字列 → `ListTeams()` で全チーム取得、名前で部分一致検索
  3. 一致なし → エラー（利用可能なチーム名一覧を表示）
  4. 複数一致 → エラー

### テスト

| ID | テスト | 入力 | 期待結果 |
|----|--------|------|---------|
| T1 | 数値文字列 | "173843" | [173843] |
| T2 | チーム名 | "ヘプタゴン" | 名前一致のチームID |
| T3 | 一致なし | "存在しない" | エラー + 利用可能一覧 |
| T4 | 複数指定 | ["173843", "221464"] | [173843, 221464] |

---

## 修正2: `ListTeams` にメンバーを含める

### 変更箇所

**`internal/backlog/http_client.go`**:
- `ListTeams` の戻り値を `[]domain.Team` → `[]domain.TeamWithMembers` に変更
- Backlog API `GET /api/v2/teams` は実際に `members[]` を返すので、デシリアライズ先を変えるだけ

**`internal/backlog/client.go`**:
- `ListTeams(ctx) ([]domain.Team, error)` → `ListTeams(ctx) ([]domain.TeamWithMembers, error)` に変更

**`internal/backlog/mock_client.go`**:
- `ListTeamsFunc` の戻り値型を更新

**`internal/cli/team.go`**:
- `TeamListCmd.Run` は変更不要（Render に渡すだけ）
- `TeamProjectCmd` は `ListProjectTeams` を使っているので影響なし

### テスト

| ID | テスト | 内容 |
|----|--------|------|
| M1 | `TestListTeams_withMembers` | ListTeams が TeamWithMembers（Members 含む）を返す |
| M2 | 既存テストの型修正 | MockClient の ListTeamsFunc 戻り値型を更新 |

---

## 変更対象ファイル

| ファイル | 変更内容 |
|---------|----------|
| `internal/cli/digest_cmd.go` | Team フラグ型変更 + resolveTeamIDs 呼び出し |
| `internal/cli/resolve.go` | resolveTeamIDs 関数追加 |
| `internal/cli/issue_list_test.go` | resolveTeamIDs テスト追加 |
| `internal/backlog/client.go` | ListTeams 戻り値型変更 |
| `internal/backlog/http_client.go` | ListTeams 戻り値型変更 |
| `internal/backlog/mock_client.go` | ListTeamsFunc 型更新 |
| `internal/backlog/http_client_test.go` | ListTeams テスト更新 |

---

## コミット戦略

| # | メッセージ | 内容 |
|---|-----------|------|
| 1 | `refactor(backlog): ListTeams の戻り値を TeamWithMembers に変更` | client.go, http_client.go, mock + テスト |
| 2 | `feat(cli): digest --team にチーム名解決を追加` | resolve.go, digest_cmd.go + テスト |

---

## 検証方法

```bash
go test ./...

# チーム名で digest
lv digest --team "ヘプタゴン" --since this-week

# team list でメンバー表示
lv team list --pretty | jq '.[0].members'

# 数値ID も引き続き動作
lv digest --team 173843 --since this-week
```

---

## Next Action

> 1. `Skill(devflow:implement)` — このプランに基づいて実装を開始
