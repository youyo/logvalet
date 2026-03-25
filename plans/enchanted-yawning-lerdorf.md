---
title: -f md リッチ化 + -f gantt 新設 + text/mermaid 削除
project: logvalet
author: planning-agent
created: 2026-03-25
status: Ready for Review
---

# `-f md` リッチ化 + `-f gantt` 新設 + text/mermaid 削除

## Context

現在の出力フォーマットに重複・死に機能がある:
- `markdown`: JSON をコードフェンスで包むだけ（リッチさゼロ）
- `mermaid`: Mermaid gantt 図（用途限定的、gantt に置き換え）
- `text`: コンパクト JSON（`json --pretty=false` と同じ）

これらを整理し、`-f md` をリッチ markdown テーブルに刷新、`-f gantt` を Issue 専用ガントテーブルとして新設する。

## 変更後のフォーマット体系

| フォーマット | 内容 | 対象 |
|---|---|---|
| `json` | 構造化JSON（デフォルト） | 全型 |
| `yaml` | YAML | 全型 |
| `md` / `markdown` | リッチ markdown テーブル | 全型 |
| `gantt` | ガントテーブル（日付列+経過/残り+URL） | `[]domain.Issue` 専用 |
| ~~`text`~~ | 削除 | — |
| ~~`mermaid`~~ | 削除 | — |

## `-f md` markdown テーブル仕様

### 配列データ → markdown テーブル
```
| issueKey | summary | status | startDate | dueDate |
|----------|---------|--------|-----------|---------|
| CND-7 | S3からファイル削除… | 処理中 | 2026-03-20 | 2026-03-28 |
```
- JSON Marshal → `[]map[string]any` → キーをヘッダー、値をセルに
- ネストしたオブジェクト（Status, Assignee 等）→ `.Name` があれば Name を表示、なければ JSON 文字列
- 空配列 → `(データなし)`

### 単体オブジェクト → キー・値リスト
```
- **profile**: heptagon
- **auth_type**: api_key
- **expired**: false
- **user**: Naoto Ishizawa
```
- ネストしたオブジェクト → `.Name` があれば Name、なければ JSON 文字列

## `-f gantt` ガントテーブル仕様（Issue 専用）

```
📅 タスク一覧 (3/23 〜 3/28)

| 課題 | 3/23 | 3/24 | 3/25 | 3/26 | 3/27 | 3/28 |
|------|------|------|------|------|------|------|
| [CND-7 S3からファイル削除してもKB…](https://heptagon.backlog.com/view/CND-7) |  |  | ░░ | ██ |  |  |
| [CND-8 sandbox2構築](https://heptagon.backlog.com/view/CND-8) |  |  |  |  | ██ | ██ |

凡例: ░░ 経過  ██ 残り
```

### ルール
- 日付列: 全課題の min(startDate) 〜 max(dueDate) を自動算出、M/D 形式
- 課題名: `[{issueKey} {summary(30文字…)}](https://{space}.backlog.com/view/{issueKey})`
- 日付セル: 期間外→空白、date < today→`░░`、date >= today→`██`
- startDate nil → dueDate で代用（1日タスク）
- 両方 nil → スキップ + stderr 警告
- ソート: startDate 昇順 → dueDate 昇順
- タイトル: `📅 タスク一覧 ({min} 〜 {max})`
- 凡例: テーブル末尾
- **非 Issue データで使用 → エラー**: `"gantt フォーマットは issue list でのみ使用できます"`

## 変更ファイル

### 削除
- `internal/render/text.go`
- `internal/render/text_test.go`
- `internal/render/mermaid.go`
- `internal/render/mermaid_test.go`

### `internal/render/render.go`
- `NewRenderer(format string, pretty bool, space string)` — space パラメータ追加
- `case "text":` 削除
- `case "mermaid":` 削除
- `case "gantt":` 追加 → `NewGanttTableRenderer(space)`
- `case "md", "markdown":` → `NewMarkdownRenderer()`（space 不要、汎用テーブル）
- エラーメッセージ: `"サポート: json, yaml, md, markdown, gantt"`

### `internal/render/markdown.go` — 全面書き換え
- `MarkdownRenderer` struct
- `Render(w, data)`:
  1. data を `json.Marshal` → `json.Unmarshal` で `any` に変換
  2. `[]any` → `renderTable(w, rows)`
  3. `map[string]any` → `renderKeyValueList(w, obj)`
  4. その他 → `fmt.Fprintf(w, "%v", data)`

### `internal/render/gantt.go` (新規)
- `GanttTableRenderer` struct（`space string`）
- `NewGanttTableRenderer(space string)`
- `Render(w, data)`:
  1. `[]domain.Issue` を type assert → 非 Issue はエラー
  2. nil 日付処理 + ソート
  3. min/max 算出 → 日付列生成
  4. タイトル + テーブル + 凡例出力

### `internal/render/markdown_test.go` — 全面書き換え
### `internal/render/gantt_test.go` (新規)

### `internal/cli/runner.go` (2箇所)
- `NewRenderer(format, pretty)` → `NewRenderer(format, pretty, resolved.Space)` / `NewRenderer(format, g.Pretty, g.Space)`

### `internal/cli/version_cmd.go`
- `NewRenderer(format, g.Pretty)` → `NewRenderer(format, g.Pretty, "")`

### 既存テスト（render/*_test.go）
- `NewRenderer(...)` に `""` 追加（約10箇所）
- text/mermaid 参照の削除

## テスト設計

### ガントテーブル（gantt_test.go）

| ID | テスト名 | 入力 | 期待出力 |
|---|---|---|---|
| G1 | TestGantt_basic | 2件Issue、space="heptagon" | タイトル+テーブル+凡例+URL |
| G2 | TestGantt_pastFuture | today跨ぎ | ░░ と ██ 混在 |
| G3 | TestGantt_nilStart | startDate=nil | dueDate代用 |
| G4 | TestGantt_bothNil | 両方nil | スキップ+stderr |
| G5 | TestGantt_truncate | 31文字件名 | 30文字+… |
| G6 | TestGantt_sort | 3件順不同 | startDate昇順 |
| G7 | TestGantt_empty | 空スライス | `(データなし)` |
| G8 | TestGantt_singleDay | start=due | 1列 |
| G9 | TestGantt_nonIssue | string | エラー |

### markdown テーブル（markdown_test.go）

| ID | テスト名 | 入力 | 期待出力 |
|---|---|---|---|
| M1 | TestMarkdown_table_basic | []User 2件 | mdテーブル |
| M2 | TestMarkdown_table_nested | ネストあり | Name表示 |
| M3 | TestMarkdown_table_empty | 空配列 | `(データなし)` |
| M4 | TestMarkdown_kv_basic | authWhoami resp | `- **key**: value` |
| M5 | TestMarkdown_kv_nested | ネストあり | Name or JSON |
| M6 | TestMarkdown_issueList | []Issue | 通常mdテーブル（ガントではない） |

### NewRenderer

| ID | テスト名 | 入力 | 期待出力 |
|---|---|---|---|
| R1 | TestNewRenderer_md | `"md"` | MarkdownRenderer |
| R2 | TestNewRenderer_gantt | `"gantt"` | GanttTableRenderer |
| R3 | TestNewRenderer_text_removed | `"text"` | error |
| R4 | TestNewRenderer_mermaid_removed | `"mermaid"` | error |

## 検証手順

```bash
go test ./...

# ガントテーブル
lv issue list --assignee me --due-date this-week -f gantt
lv issue list --assignee me --start-date this-week -f gantt

# 汎用 markdown テーブル
lv issue list --assignee me --due-date this-week -f md
lv user list -f md
lv project list --count 5 -f md
lv auth list -f md

# キー・値リスト
lv auth whoami -f md

# 削除確認
lv issue list -f text     # → エラー
lv issue list -f mermaid  # → エラー

# gantt を非 Issue で使用 → エラー
lv user list -f gantt     # → エラー
```

## コミット戦略

```
refactor(render): text/mermaid レンダラーを削除

feat(render): -f md をリッチ markdown テーブルに刷新

配列データは markdown テーブル、単体オブジェクトはキー・値リスト形式で出力。
ネストしたオブジェクトは .Name があれば Name を表示。

feat(render): -f gantt ガントテーブルを新設

Issue 専用のガントテーブル。日付列に経過(░░)/残り(██)表示、
課題名に Backlog URL リンク付き。NewRenderer に space パラメータ追加。

docs: README/SKILL.md のフォーマット一覧を更新
```

## ドキュメント・ヘルプ更新

### `internal/cli/global_flags.go` (16-17行)
- Format の help 文字列とコメントを更新:
  ```
  Before: `"出力フォーマット (json|yaml|markdown|text)"`
  After:  `"出力フォーマット (json|yaml|md|gantt)"`
  ```
- completion は Kong が struct タグから自動生成するため、help 更新で自動対応

### `README.md` / `README.ja.md`
- フォーマット一覧更新、`-f md` / `-f gantt` の出力例追加
- text/mermaid の記載を削除

### `skills/logvalet/SKILL.md`
- 同上
