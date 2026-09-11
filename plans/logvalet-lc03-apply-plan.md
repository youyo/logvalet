---
title: マイルストーン LC03/LC04 - project apply（差分計画と適用）
project: logvalet
author: planning-agent
created: 2026-09-03
status: Ready for Review
complexity: L
---

# マイルストーン LC03/LC04: project apply

## 概要

`conventions.yaml` を Backlog プロジェクトへ冪等に差分適用する。
LC03 は**差分計画（Plan）の生成と表示**（`--dry-run`）、LC04 は**Plan の実行**。

dry-run と実行のズレを構造的に防ぐため、**`apply` は dry-run と同じ Plan を作り、
それをそのまま実行するだけ**にする。Plan の生成と実行を 1 つの型に閉じ込め、
差分判定のロジックが 2 か所に散らないようにする。

書き込みを伴うため MCP には出さない（ロードマップの「一括更新は必ず人の承認を取る」）。

---

## スコープ

### 実装範囲

- `internal/conventions/plan.go` — `Plan` / `Action` 型と `BuildPlan`（LC03）
- `internal/conventions/plan_render.go` — 人間向けテキスト表示（LC03）
- `internal/conventions/apply.go` — `Apply`（Plan の実行、LC04）
- `internal/conventions/rule_issue.go` — 規約課題の説明欄の組み立てと検索（LC04）
- `internal/cli/project_apply.go` — `ProjectApplyCmd`
- `internal/cli/project.go` — `ProjectCmd` に `Apply` を追加
- 各 `_test.go` と `testdata/`（golden）

### スコープ外

- 過去課題の親子付け替え（ロードマップで明示的にスコープ外）
- カテゴリ・種別・状態の削除（conventions から消しても Backlog 側は消さない）
- MCP ツール化（書き込みは CLI のみ）

---

## コマンド

```
logvalet project apply --file conventions.yaml [--project KEY] [--dry-run] [--create]
```

- `--file` 必須（グローバルの `-f` は出力フォーマットなので short は付けない）
- `--project` は `conventions.yaml` の `project.key` を上書きする
- `--dry-run` は Plan を表示して終了する。書き込みは一切しない
- `--create` はプロジェクトが存在しないとき `CreateProject` で作る。
  未指定でプロジェクトが無ければ exit 5（`ExitNotFoundError`）

適用前に必ず `Validate` を通し、**error violation が 1 件でもあれば exit 2**
（`--dry-run` でも同じ）。apply は `validate --strict` 相当では動かないが、
Lead 空欄は下記のとおり skip として扱う。

---

## Plan の型

```go
// Action は Plan の 1 項目が行う操作。
type Action string

const (
	ActionCreate    Action = "create"
	ActionUpdate    Action = "update"
	ActionUnchanged Action = "unchanged"
	ActionSkip      Action = "skip"
)

// ResourceKind は Plan の 1 項目が対象とするリソース種別。
type ResourceKind string

const (
	KindProject   ResourceKind = "project"
	KindIssueType ResourceKind = "issue_type"
	KindStatus    ResourceKind = "status"
	KindCategory  ResourceKind = "category"
	KindIssue     ResourceKind = "issue"
)

// FieldChange は 1 フィールドの変更内容。
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// PlanItem は Plan の 1 項目。
type PlanItem struct {
	Resource ResourceKind  `json:"resource"`
	Action   Action        `json:"action"`
	Name     string        `json:"name"`
	Changes  []FieldChange `json:"changes,omitempty"`
	Reason   string        `json:"reason,omitempty"` // skip の理由
	// 実行に必要な内部情報（JSON には出さない）
	targetID int
	payload  any
}

// Plan は conventions を Backlog に適用するための差分計画。
type Plan struct {
	ProjectKey string     `json:"project_key"`
	Items      []PlanItem `json:"items"`
	Summary    PlanSummary `json:"summary"`
}

type PlanSummary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Skip      int `json:"skip"`
}
```

`targetID` / `payload` を非公開にすることで、Plan の JSON 出力（機械可読）と
実行に必要な内部情報を分離する。`Apply` は同じパッケージ内なのでアクセスできる。

---

## Plan の生成順序

適用順序 = Plan の項目順序。依存関係があるため固定する。

1. **project** — 存在しなければ `--create` で作成、あれば `unchanged`
2. **issue_type** — 「規約」「案件」を含む全種別
3. **status** — カスタム状態
4. **category** — 案件カテゴリ
5. **issue（規約課題）** — 種別「規約」の課題 1 件
6. **issue（案件親課題）** — 案件ごとに 1 件

種別・カテゴリが先なのは、規約課題と案件親課題がそれらの ID を要るため。

---

## 差分判定と冪等性

すべて**名前で照合**する（Backlog に安定した外部キーがないため）。

| リソース | 照合キー | 更新対象フィールド |
|---|---|---|
| issue_type | `name` | `color`, `template_summary`, `template_description` |
| status | `name` | `color` |
| category | `name` | （なし。存在すれば `unchanged`） |
| 規約課題 | 種別「規約」の課題を検索 | `description` |
| 案件親課題 | 種別「案件」かつ件名一致 | `assignee`, `start_date`, `due_date`, `category` |

### 複数一致は失敗させる

既存 Backlog 側に同名のカテゴリ・種別・状態が **2 件以上**あるとき、
どちらを更新すべきか決まらない。誤更新を避けるため、Plan の生成時点で
エラーにする（exit 2）。dry-run でも同じ。

規約課題も同じ。0 件なら作成、1 件なら更新、**2 件以上なら exit 2**（ADR 0005）。

### skip する条件

| 条件 | reason |
|---|---|
| `engagements[].lead` が空 | `Lead が未指定のため案件親課題を作成しません` |
| `lead` の表示名がプロジェクトメンバーに見つからない | `Lead "山田 太郎" がプロジェクトメンバーに見つかりません` |
| `lead` の表示名が複数のメンバーに一致 | `Lead "山田" が複数のメンバーに一致します` |
| status の追加でカスタム状態上限（8 個）に達している | `カスタム状態は最大 8 個までです` |

Lead 空欄の案件は**カテゴリは作るが親課題は作らない**。ADR 0006 の
「Lead は 1 人」を破らないため（LC02 で validate が warning を出している）。

Lead の解決は `ListUsers` ではなく `GetProject` 後の
プロジェクトメンバー一覧を使う。Backlog API に
`GET /api/v2/projects/{key}/users` があるので、LC03 の実装前に
`ListProjectUsers` を `backlog.Client` に追加する必要がある（下記「前提作業」）。

---

## 前提作業: `ListProjectUsers` の追加

LC01 では追加していないが、Lead の名前解決に必要。

| メソッド | Backlog API | 戻り値 |
|---|---|---|
| `ListProjectUsers(ctx, projectKey)` | `GET /api/v2/projects/{key}/users` | `[]domain.User` |

実装時に API ドキュメントでパラメータを確認すること。

---

## 規約課題（ADR 0005）

種別「規約」の課題 1 件が規約の正本。件名は `[規約] 運用規約` に固定する。

説明欄の構成:

```
（運用ガイドの本文。Glossary() の用語集を Markdown 表で埋め込む）

## 規約 (conventions.yaml)

```yaml
（conventions.yaml の内容そのまま）
```
```

`LoadFromIssueDescription`（LC02 実装済み）が最後の ```yaml ブロックを読むので、
この形式で往復できる。説明欄の組み立ては `rule_issue.go` の
`BuildRuleIssueDescription(c *Conventions, yamlSource []byte) string` に閉じ込め、
LC05 の読み出しと対で golden test を置く。

規約課題は閉じないため、LC06 で stale / blockers / workload / health の
分析対象から種別「規約」を除外する。

---

## 案件親課題（ADR 0006）

- 件名: `[案件] <案件名>`
- 種別: 「案件」
- 担当者: Lead
- 開始日 / 期限日: `start_date` / `due_date`
- カテゴリ: 案件カテゴリ 1 つ
- 説明: 種別「案件」の `template_description`（新規作成時のみ。
  既存課題の説明は**上書きしない** — 人が書いた Context & Goals を消さないため）

更新対象は `assignee` / `start_date` / `due_date` / `category` のみ。

---

## 部分失敗

プロジェクト作成 → 種別・カテゴリ・状態 → 規約課題 → 案件親課題は
Backlog 上で 1 つのトランザクションにならない。

- Plan の項目を**順に**実行し、失敗したらそこで止める
- 失敗するまでに成功した項目は残る（ロールバックしない。削除 API を実装しないため）
- 実行結果を stdout に JSON で出す。各項目に `status`（`applied` / `failed` /
  `skipped` / `not_reached`）と、失敗時は `error` を持たせる
- 1 件でも failed があれば exit 8（`app.ExitPartialFailure`）。
  ただし最初の項目が失敗して 1 件も適用できなかった場合は、その項目の
  エラーに応じた exit code（認証 3、権限 4、not found 5、API 6）を返す
- **再実行で回復する**のが冪等性の要点。途中まで適用された状態から
  もう一度 apply すれば、成功済みは `unchanged` になり残りが適用される

`--dry-run` はネットワーク読み取りのみ（Plan 生成に必要な GET）を行い、
書き込みは一切しない。

---

## 出力

### `--dry-run`

stdout に Plan の JSON、stderr に人間向けテキスト。

```
project SANDBOX
  issue_type  + 案件 (template_description)
  status      + レビュー中
  category    = 開発チーム
  category    + 顧客A 基盤更改
  issue       + [案件] 顧客A 基盤更改  lead=山田 太郎  due=2026-10-31
  issue       ~ [案件] 運用保守  assignee: (none) -> 鈴木 花子
  issue       ! [案件] 新規案件  Lead が未指定のため案件親課題を作成しません
plan: 4 create, 1 update, 1 unchanged, 1 skip
```

記号は `+` create、`~` update、`=` unchanged、`!` skip。

### `apply`

stdout に実行結果 JSON（Plan と同構造 + `status` / `error`）、
stderr に同じ形式の進捗テキスト。

どちらも結果を自前で stdout に書くため、exit code は LC02 で追加した
`quietExitError` 経由で返す（error envelope の二重出力を避ける）。

---

## テスト計画（TDD）

Backlog API テストはモックのみ（`backlog.MockClient`）。

### `plan_test.go`（LC03）

- 空のプロジェクトに対する Plan（全部 create）
- 既に適用済みのプロジェクト（全部 unchanged）— **冪等性の中核**
- 種別のテンプレートだけ違う（update 1 件）
- 同名のカテゴリが 2 件ある → エラー
- 規約課題が 2 件ある → エラー
- Lead 空欄 → カテゴリは create、親課題は skip
- Lead が見つからない / 複数一致 → skip
- カスタム状態が既に 8 個 → skip
- プロジェクトが無く `--create` なし → エラー
- Plan の項目順序が固定であること（golden）

### `plan_render_test.go`（LC03）

- 上記 Plan のテキスト表示 golden

### `apply_test.go`（LC04）

- Plan どおりに Client のメソッドが呼ばれること（呼び出し順も検証）
- 途中で失敗したとき、以降が `not_reached` になり exit 8 相当の結果になること
- `unchanged` の項目で API を呼ばないこと
- 適用 → 再度 Plan 生成で全て `unchanged` になること（冪等性の往復テスト）

### `rule_issue_test.go`（LC04）

- `BuildRuleIssueDescription` の golden
- 生成した説明欄を `LoadFromIssueDescription` で読み戻すと元の conventions と
  一致すること（往復テスト）

### `internal/cli/project_apply_test.go`

- Kong パース（`--file` 必須、`--dry-run`、`--create`、`--project`）

---

## 完了条件

- `go test ./...` 全パス、`go vet ./...` クリーン
- 同じ conventions を 2 回 apply して 2 回目が全て `unchanged` になる
- `--dry-run` が書き込み系 API を 1 度も呼ばない（MockClient の呼び出しカウントで検証）
- 同名リソースが複数あるときに mutation 前に失敗する
- 部分失敗が exit 8 で報告され、再実行で回復する

---

## 依存

- 前提: LC01（書き込み API）、LC02（スキーマ・ローダー・検証）
- 前提作業: `ListProjectUsers` の追加
- 後続: LC05（`conventions show`）は `rule_issue.go` の説明欄フォーマットに依存する
