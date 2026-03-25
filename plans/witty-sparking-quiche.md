---
title: zsh completion フラグ補完修正 + SKILL.md CLI追従
project: logvalet
author: planning-agent
created: 2026-03-24
status: Draft
---

# zsh completion フラグ補完修正 + SKILL.md CLI追従

## Context

2つの問題が報告されている:

1. **zsh completion でフラグが出ない**: `lv user list --<TAB>` でフラグ候補が表示されない。commit 27571fe でスクリプト側の引用符問題は修正済みだが、バックエンド（`handleCompletionBash`）がフラグを一切出力していない
2. **SKILL.md が CLI に追いついていない**: 未実装コマンド（issue/project/user/team digest）の記載、新フラグの未記載、ListFlags の誤記載がある

## スコープ

### 実装範囲
- `handleCompletionBash` にフラグ補完ロジックを追加（全コマンド共通で修正）
- SKILL.md を CLI 実装の現状に完全追従させる
- `docs/specs/logvalet_SKILL.md` を `skills/` 版と同期

### スコープ外
- 未実装 digest サブコマンドの新規実装
- Homebrew での completion ファイル配布
- bash completion 対応

---

## 課題1: zsh completion フラグ補完

### 根本原因

`cmd/logvalet/main.go:64-107` の `handleCompletionBash` が `node.Children`（サブコマンド名）のみ出力し、`node.Flags` を出力していない。

### テスト設計書

#### テスト対象の分離

現在 `handleCompletionBash` は `fmt.Println` で直接 stdout に出力しているためテストしにくい。純粋関数 `collectCompletions(k *kong.Kong, args []string) ([]string, bool)` を切り出す。

#### 正常系ケース

| ID | 入力 args | 期待出力に含まれるもの | 備考 |
|----|-----------|----------------------|------|
| C1 | `["--completion-bash", "user", "list"]` | `--offset`, `--count` | リーフノードのフラグが出力される |
| C2 | `["--completion-bash", "user", "list"]` | `--format`, `--profile` | 親フラグ（GlobalFlags）も含まれる |
| C3 | `["--completion-bash", "issue", ""]` | `get`, `list`, `create` + `--format` 等 | サブコマンドとフラグが両方出力される |
| C4 | `["--completion-bash", ""]` | `auth`, `issue`, `user` 等 | トップレベルのサブコマンド（既存動作維持） |

#### 異常系/エッジケース

| ID | 入力 args | 期待 | 備考 |
|----|-----------|------|------|
| E1 | `["auth", "login"]` | `nil, false` | `--completion-bash` なし |
| E2 | `["--completion-bash", "user", "list", "--"]` | フラグのみ（サブコマンドなし） | `--` プレフィクスでフラグフィルタ |
| E3 | `["--completion-bash", "user", "list", "--c"]` | `--count`, `--config` 等 | プレフィクスマッチ |

### 実装手順

#### Step 1: テスト追加（Red）
- ファイル: `cmd/logvalet/main_test.go`
- `collectCompletions` のテストを追加（この時点では関数未定義でコンパイルエラー）

#### Step 2: collectCompletions 関数の実装（Green）
- ファイル: `cmd/logvalet/main.go`

```go
func collectCompletions(k *kong.Kong, args []string) ([]string, bool) {
    // 1. --completion-bash の位置を検出
    // 2. partial args を走査してノードを深く進める
    // 3. 使用済みフラグを記録
    // 4. 最後の word が "--" で始まるならフラグのみモード
    // 5. node.AllFlags(true) で Hidden 除外済み全フラグ収集
    // 6. 使用済みフラグを除外
    // 7. フラグのみモードでなければサブコマンド名も追加
}
```

アルゴリズム:
- Kong の `Node.AllFlags(true)` で親フラグ含む全フラグを取得（Hidden 自動除外）
- `AllFlags` は `[][]*Flag` を返すので flatten して `--name` 形式で出力
- 既に入力済みのフラグ（`usedFlags` map）は除外
- 最後の word が `--` で始まる未確定入力ならプレフィクスマッチ + フラグのみ

#### Step 3: handleCompletionBash のリファクタ
- `handleCompletionBash` は `collectCompletions` を呼んで結果を `fmt.Println` するだけに簡素化

#### Step 4: テスト確認 + Refactor
- `go test ./cmd/logvalet/...` が全パス

### 変更対象ファイル
- `cmd/logvalet/main.go` — `collectCompletions` 新規追加、`handleCompletionBash` リファクタ
- `cmd/logvalet/main_test.go` — フラグ補完テスト追加

### Kong API メモ
- `Node.AllFlags(hide bool) [][]*Flag` — `hide=true` で Hidden 除外、親ノード再帰
- `Flag.Name` (via `*Value`) — フラグ名
- `Flag.Hidden bool`
- `Flag.Short rune` — 短縮形
- `Node.Children []*Node` — サブコマンド
- `Node.Hidden bool`

---

## 課題2: SKILL.md CLI追従

### 変更一覧

#### A. 削除するセクション（未実装コマンド）
- `## issue` > `### Issue digest`（行 400-425）→ 削除
- `## project` > `### Project digest`（行 574-587）→ 削除
- `## user` > `### User digest`（行 639-658）→ 削除
- `## team` > `### Team digest`（行 784-790）→ 削除

#### B. 新規追加セクション
- `## digest` セクションを追加（`## issue` の前）:
  - `logvalet digest --project PROJ --since 30d`
  - `logvalet digest --user me --since this-week`
  - `logvalet digest --team "TeamName" --since this-month`
  - `logvalet digest --issue PROJ-123 --since 30d`
  - フラグ: `--project/-k`(複数可), `--user`(複数可), `--team`(複数可), `--issue`(複数可), `--since`(必須), `--until`

#### C. フラグ追加記載
- `issue list`: `--due-date`, `--sort`, `--order` フラグ追加
- `issue create`: `--start-date` フラグ追加
- `issue update`: `--start-date` フラグ追加
- `issue list --assignee`: 説明を `(me, 数値ID, ユーザー名, またはチーム名)` に更新
- `team list`: `--no-members` フラグ追加
- `user activity`: `--project` と `--type` フラグ追加

#### D. ListFlags の修正
- Global flags > List-oriented flags: `--limit <n>` → `--count <n>` に修正

#### E. Digest philosophy セクション修正
- Examples の未実装コマンドを `digest` 統合コマンドの例に置換

#### F. Recommended patterns 修正
- `logvalet issue digest PROJ-123` → `logvalet digest --issue PROJ-123 --since 30d`
- `logvalet user digest 12345` → `logvalet digest --user 12345 --since 30d`
- Pattern 4 の `user digest` → `digest --user`

#### G. Minimal command set 修正
- 未実装コマンドを統合 `digest` コマンドに置換
- `logvalet project digest PROJ` → `logvalet digest --project PROJ --since 30d`

#### H. docs/specs/logvalet_SKILL.md 同期
- `skills/logvalet/SKILL.md` をマスターとして内容を同期
- frontmatter の `description` → `summary` の違いのみ維持

### 変更対象ファイル
- `skills/logvalet/SKILL.md` — メインスキルファイル
- `docs/specs/logvalet_SKILL.md` — 同期先

---

## 実装順序

1. **課題1 Red**: `main_test.go` にフラグ補完テスト追加
2. **課題1 Green**: `collectCompletions` 実装 + `handleCompletionBash` リファクタ
3. **課題1 Refactor**: テスト全パス確認後にコード整理
4. **課題2**: `skills/logvalet/SKILL.md` 更新
5. **課題2**: `docs/specs/logvalet_SKILL.md` を skills/ 版と同期
6. **検証**: `go test ./...` && `go vet ./...`

---

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| `AllFlags(true)` が `Active=true` を設定する副作用 | 低 | completion パスのみで実行、通常パースに影響なし |
| non-bool フラグの値消費を厳密に追跡しない | 低 | 実用上十分。値が `--` で始まるケースは稀 |
| SKILL.md 更新時の記載漏れ | 低 | CLI の struct 定義と照合して検証 |
| docs/specs/ 同期忘れ | 低 | 同一コミットで両方更新 |

---

## 検証

1. `go test ./...` — 全テストパス
2. `go vet ./...` — lint パス
3. `go run ./cmd/logvalet -- --completion-bash user list` — `--offset`, `--count`, `--format` 等が出力される
4. `go run ./cmd/logvalet -- --completion-bash issue list --` — フラグのみ出力される
5. `go run ./cmd/logvalet -- --completion-bash ""` — トップレベルサブコマンドが出力される（既存動作維持）
6. SKILL.md 内の全コマンド例が CLI の `--help` と一致することを確認

---

## ドキュメント更新

- README.md: 変更不要（completion の使い方は既存記載で十分）
- CHANGELOG: コミットメッセージに含める

---

## Next Action

> **このプランが承認されました。以下を順番に実行してください:**
>
> 1. `Skill(devflow:implement)` — このプランに基づいて実装を開始
>
> ユーザーの追加指示は不要です。即座に実行してください。
