---
title: マイルストーン LC02 - conventions スキーマ・ローダー・init/validate
project: logvalet
author: planning-agent
created: 2026-09-03
status: Ready for Review
complexity: M
---

# マイルストーン LC02: conventions スキーマ・ローダー・init/validate

## 概要

運用規約 `conventions.yaml` の型定義・ローダー・検証と、`logvalet project conventions init` /
`validate` の 2 コマンドを実装する。apply（LC03/LC04）と読み出し（LC05）の両方が
このパッケージの型とローダーに乗る。

規約は「値の説明に見えて、書いているのは組織の優先順位に対する態度」（ロードマップ）なので、
`init` が出すスケルトンは全項目にコメントを持つ。AI も人間もこのファイルだけで意味を追える状態を
このマイルストーンで作る。

---

## スコープ

### 実装範囲

- `internal/conventions/` — 新規パッケージ
  - `schema.go` — `Conventions` 構造体一式
  - `load.go` — バイト列 / ファイル / 課題説明欄からのロード
  - `validate.go` — 検証と `Violation` 型
  - `color.go` — Backlog の許可色 allowlist（状態用・種別用）
  - `skeleton.go` — スケルトン生成（テンプレート + YAML 値エンコード）
  - `glossary.go` — 用語集（LC05 の `conventions show` が返す `glossary` の元）
  - 各 `_test.go`
- `internal/cli/project_conventions.go` — `ProjectConventionsCmd`（`init` / `validate`）
- `internal/cli/errors.go` — `quietExitError` の追加
- `internal/app/error_envelope.go` — `QuietExiter` 経路の追加（下記「設計判断 4」）
- `internal/cli/project.go` — `ProjectCmd` に `Conventions` フィールド追加
- `internal/conventions/testdata/` — 正常・異常な YAML と golden スケルトン

### スコープ外

- `project apply`（LC03/LC04）— 差分計画も適用も行わない
- `project conventions show` と MCP ツール（LC05）— ただし本 MS で用意する
  `LoadFromIssueDescription` と `glossary.go` を LC05 が使う
- 規約課題の作成・更新（LC04）
- カスタム属性（`custom_fields`）— スキーマに持たない

---

## conventions スキーマ

```go
// Conventions は組織の運用規約。conventions.yaml の最上位。
type Conventions struct {
	SchemaVersion int          `yaml:"schema_version" json:"schema_version"`
	Project       Project      `yaml:"project" json:"project"`
	Priority      Priority     `yaml:"priority" json:"priority"`
	ClosePolicy   ClosePolicy  `yaml:"close_policy" json:"close_policy"`
	Statuses      []Status     `yaml:"statuses" json:"statuses"`
	IssueTypes    []IssueType  `yaml:"issue_types" json:"issue_types"`
	Initiatives   []Initiative `yaml:"initiatives" json:"initiatives"`
	Engagements   []Engagement `yaml:"engagements" json:"engagements"`
}

type Project struct {
	Key  string `yaml:"key" json:"key"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// Priority は 高・中・低 の意味を組織の言葉で定義したもの。段階は増やせない。
type Priority struct {
	High   string `yaml:"high" json:"high"`
	Normal string `yaml:"normal" json:"normal"`
	Low    string `yaml:"low" json:"low"`
}

type ClosePolicy struct {
	// nil = 未指定（LC06 のクローズ候補検知を行わない）。
	// 0 以下の明示指定は violation にする。
	LowUntouchedDays *int `yaml:"low_untouched_days" json:"low_untouched_days"`
}

type Status struct {
	Name  string `yaml:"name" json:"name"`
	Color string `yaml:"color" json:"color"`
}

type IssueType struct {
	Name                string `yaml:"name" json:"name"`
	Color               string `yaml:"color" json:"color"`
	TemplateSummary     string `yaml:"template_summary,omitempty" json:"template_summary,omitempty"`
	TemplateDescription string `yaml:"template_description,omitempty" json:"template_description,omitempty"`
}

type Initiative struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type Engagement struct {
	Name       string `yaml:"name" json:"name"`
	Lead       string `yaml:"lead" json:"lead"`
	Initiative string `yaml:"initiative" json:"initiative"`
	StartDate  string `yaml:"start_date,omitempty" json:"start_date,omitempty"`
	DueDate    string `yaml:"due_date,omitempty" json:"due_date,omitempty"`
}
```

`Conventions` は logvalet 自身の出力スキーマ（`conventions show` / MCP）に載るため、
JSON タグは CLAUDE.md どおり snake_case にする。

### 設計判断

**1. 日付は `string` で持つ。** `time.Time` にすると空欄（`""`）と未指定の区別が
yaml.v3 のデコードで曖昧になる。`YYYY-MM-DD` 形式かどうかは `validate` で検査し、
`time.Parse` は apply 側で行う。

**2. `close_policy.low_untouched_days` はポインタ。** 未指定（検知しない）と
無効値（0 以下）を区別する。ポインタにしないと、未指定の YAML で LC06 が
全ての低優先度課題をクローズ候補に挙げてしまう。

**3. `template_summary` もスキーマに持つ。** Backlog の課題種別は件名テンプレートも持ち、
`--from-project` で既存値を取り込む以上、捨てると apply が意図せず消す可能性がある。

**4. `validate` の出力と exit code。** 結果 JSON を stdout に出したうえで exit 2 を返したいが、
`Run` が error を返すと `cmd/logvalet/main.go:58` の `app.HandleError` が
error envelope をもう 1 つ stdout に書いてしまう。これを避けるため、
`app` に静音経路を足す。

```go
// QuietExiter は envelope を出力せず exit code だけを返すエラー用インターフェース。
// 結果 JSON を自前で stdout に書き終えたコマンドが使う。
type QuietExiter interface {
	error
	QuietExit() bool
}
```

`HandleError` の先頭で `QuietExiter` かつ `QuietExit() == true` なら
envelope を書かずに exit code だけ返す。`internal/cli/errors.go` に
`quietExitError{code int}` を置き、`validate` がこれを返す。

---

## 検証ルール

`Validate(c *Conventions) []Violation` を純粋関数として実装する。
名前の比較・空判定はすべて `strings.TrimSpace` した値で行う（空白のみの名前は空と同じ）。

```go
type Severity string

const (
	SeverityError   Severity = "error"   // 常に exit 2
	SeverityWarning Severity = "warning" // --strict のときだけ exit 2
)

type Violation struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`    // 例: "engagements[0].lead"
	Message  string   `json:"message"` // 日本語
}
```

| # | 条件 | severity |
|---|---|---|
| 1 | `schema_version` が 1 以外 | error |
| 2 | `project.key` が空、または `^[A-Z][A-Z0-9_]*$` に一致しない | error |
| 3 | `project.name` が空（`--create` で必須） | warning |
| 4 | `issue_types` に名前「規約」がない | error |
| 5 | `issue_types` に名前「案件」がない | error |
| 6 | `issue_types[].name` が空 | error |
| 7 | `issue_types[].name` の重複 | error |
| 8 | `issue_types[].color` が種別用 allowlist にない | error |
| 9 | `statuses[].name` が空 | error |
| 10 | `statuses[].name` の重複 | error |
| 11 | `statuses[].name` が既定 4 状態（未対応・処理中・処理済み・完了）と同名 | error |
| 12 | `statuses[].color` が状態用 allowlist にない | error |
| 13 | `initiatives[].name` が空 | error |
| 14 | `initiatives[].name` の重複 | error |
| 15 | `engagements` が 1 件以上あるのに `initiatives` が空 | error |
| 16 | `engagements[].name` が空 | error |
| 17 | `engagements[].name` の重複 | error |
| 18 | `engagements[].initiative` が空 | error |
| 19 | `engagements[].initiative` が `initiatives[].name` に存在しない | error |
| 20 | `engagements[].lead` が空 | **warning** |
| 21 | `start_date` / `due_date` が空でなく `YYYY-MM-DD` としてパースできない | error |
| 22 | `due_date` < `start_date` | error |
| 23 | `priority.high` / `normal` / `low` のいずれかが空 | warning |
| 24 | `close_policy.low_untouched_days` が非 nil かつ 0 以下 | error |
| 25 | `statuses` が 9 件以上（Backlog のカスタム状態上限は 8 個） | error |

補足:

- 20（Lead 空欄）が warning なのは意図的。「決めていない案件は始めない」を強制はせず、
  むき出しにして `--strict`（CI）で落とせるようにする（記事の「空欄がむき出しになる」）。
  ただし **apply（LC04）は `--strict` 相当で動く**。Lead 空欄の案件は親課題を作らずスキップし、
  dry-run にスキップ理由として出す。ADR 0006 の「Lead は 1 人」を破らないため。
  この方針は LC04 の計画書にも書く
- 15 は条件付き。導入直後の「Initiative も案件もまだ無い」最小 YAML を許す
- 11 は既定状態と同名のカスタム状態を作ろうとして apply が 400 になるのを防ぐ

`valid` の定義: **error violation が 0 件**。warning の有無は `valid` に影響しない。

---

## 色 allowlist

Backlog は課題種別・状態それぞれで指定できる色を固定している。任意の `#rrggbb` を通すと
`init` → `validate` は通るのに apply が 400 になる。`color.go` に allowlist を持ち、
ルール 8 / 12 で照合する。

確定値（Backlog API ドキュメント, 2026-09-03 確認）:

```go
// StatusColors は状態に指定できる色。
var StatusColors = []string{
	"#ea2c00", "#e87758", "#e07b9a", "#868cb7", "#3b9dbd",
	"#4caf93", "#b0be3c", "#eda62a", "#f42858", "#393939",
}

// IssueTypeColors は課題種別に指定できる色。
var IssueTypeColors = []string{
	"#e30000", "#990000", "#934981", "#814fbc", "#2779ca",
	"#007e9a", "#7ea800", "#ff9200", "#ff3265", "#666665",
}
```

ロードマップのスケルトン例にあった状態色 `#ea8462` は allowlist 外だったため
`#e87758` に修正済み。

allowlist は exported にし（`conventions.StatusColors` / `conventions.IssueTypeColors`）、
LC03 の dry-run とエラーメッセージが同じ一覧を出せるようにする。

---

## コマンド設計

```
logvalet project conventions init [--out conventions.yaml] [--from-project KEY]
logvalet project conventions validate --file conventions.yaml [--strict]
```

> グローバルフラグの `-f` は出力フォーマット（`internal/cli/global_flags.go:17`）なので、
> 入力ファイルには short を割り当てず `--file` / `--out` を使う。

### `init`

- `--out` 未指定なら stdout に出す。指定時、既存ファイルがあれば上書きせず exit 2
  （`--force` は作らない）
- `--from-project KEY` は Backlog を読み、既存プロジェクトへの導入起点を作る。
  出力を決定的にするため、以下を固定する:
  - `project.key` / `project.name` ← `GetProject`
  - `issue_types[]` ← `ListProjectIssueTypes`（LC01 の `domain.IssueType` から
    `color` / `template_summary` / `template_description` を引き継ぐ）。
    `displayOrder` 昇順、同値なら `id` 昇順でソート。
    「規約」「案件」が無ければスケルトン既定値を末尾に追加する
  - `statuses[]` ← `ListProjectStatuses` から **ID が 1〜4 の既定状態を除外**したもの
    （名前一致による除外はリネーム済み環境で誤動作するため使わない）。同じ順序規則でソート
  - `engagements[]` ← `ListProjectCategories`（同じ順序規則）。`name` にカテゴリ名、
    `lead` は空欄、`initiative` は全件 `未分類` を設定
  - `initiatives[]` ← `未分類` 1 件のみ。description に
    「自動生成の仮置き。案件を実際のテーマに割り当て直すこと」と書く
- `--from-project` なしのときは埋め込みスケルトンをそのまま出す

`--from-project` の出力には `engagements[].lead` 空欄由来の warning が必ず出る。
これは意図した状態（空欄をむき出しにする）で、error は 0 件でなければならない。

### スケルトンの `engagements`

標準スケルトンは `engagements: []` を出す（空リスト + 書き方を説明するコメント）。
`name: ""` のプレースホルダーはルール 16 で error になり、`init` の自己検査と矛盾するため
出さない。

### `validate`

- `--file` 必須。ネットワーク不要（オフラインで動く）
- stdout に結果 JSON `{"valid": bool, "violations": [...]}` を出す
- 人間向けの要約は stderr
- exit code:
  - error violation が 1 件以上 → 2（`quietExitError`）
  - warning のみ かつ `--strict` → 2
  - それ以外 → 0

---

## ローダー

```go
// Load はバイト列を Conventions にデコードする。未知フィールドはエラーにする。
func Load(data []byte) (*Conventions, error)

// LoadFile はファイルパスから読む。
func LoadFile(path string) (*Conventions, error)

// LoadFromIssueDescription は規約課題の説明欄から YAML コードブロックを取り出して
// デコードする。コードブロックが 0 個ならエラー、複数あれば最後のものを使う。
func LoadFromIssueDescription(description string) (*Conventions, error)
```

`LoadFromIssueDescription` は LC05 が使う。ここで実装しておくことで、
規約課題の説明欄フォーマットを LC02 のテストで固定できる。

抽出は ```` ```yaml ```` 〜 ```` ``` ```` のフェンスを対象にする。
言語指定なしのフェンスは対象外（運用ガイド中の他のコードブロックと衝突するため）。

デコードは `yaml.Decoder` に `KnownFields(true)` を設定し、タイポを黙って無視しない。

---

## スケルトン生成

コメント付き YAML が要件で、`yaml.Marshal` はコメントを保てない。一方で
`--from-project` はカテゴリ名・種別名・テンプレート本文といった**任意の文字列**を
埋め込むため、テンプレートに素で差し込むと `Ops: on-call #1`・引用符・改行・`- item`
などで YAML が壊れる。

**方針: 固定のコメントと構造は `text/template` で書き、動的値は必ずエンコーダーを通す。**

```go
// yamlScalar は任意の文字列を YAML のスカラー値として安全に表現する。
// 必要なら引用し、改行を含む場合はブロックスカラーにする。
func yamlScalar(s string, indent int) string
```

実装は `yaml.Marshal(s)` の結果を使い（yaml.v3 が引用・エスケープを判断する）、
複数行は `|` ブロックスカラーに整形して指定インデントを付ける。
テンプレートには `{{ yamlScalar .Name 4 }}` の形でのみ動的値を差し込み、
生の `{{ .Name }}` は使わない（レビュー時のチェック項目にする）。

生成結果は必ず `Load` → `Validate` に通し、error violation が出たら
バグとして exit 1 にする（自己検査）。

---

## テスト計画（TDD: Red → Green → Refactor）

### `internal/conventions/`

- `schema_test.go` — 最小 YAML / フル YAML のラウンドトリップ、未知フィールドで
  エラーになること、`low_untouched_days` の未指定と 0 が区別されること
- `validate_test.go` — 上表 24 ルールを 1 ケースずつ。violation なしの正常系も置く
- `color_test.go` — allowlist 内外の色が判定されること
- `load_test.go` — `LoadFromIssueDescription`：
  - 運用ガイド + ```yaml ブロック 1 個 → デコード成功
  - ブロック 0 個 → エラー
  - ブロック複数 → 最後のものを使う
  - 言語指定なしフェンスのみ → エラー
- `skeleton_test.go`
  - 埋め込みスケルトンが `Load` → `Validate` を通り error 0 件であること。
    golden ファイル（`testdata/skeleton.golden.yaml`）と一致すること
  - **エスケープの回帰テスト**: プロジェクト名 `Ops: on-call #1`、
    カテゴリ名 `- 顧客A "基盤" 更改`、テンプレート本文に改行と `#` を含む入力で
    `--from-project` 相当のスケルトンを生成し、`Load` が成功して値が
    往復すること
  - `--from-project` の決定性: ID 1〜4 の状態が除外され、`displayOrder` 順に並ぶこと

`--from-project` の生成は `BuildSkeletonFromProject(ctx, client backlog.Client, projectKey string) ([]byte, error)`
のように **`backlog.Client` を引数で受ける関数**として `internal/conventions` に置き、
`backlog.MockClient` で直接テストする。CLI 側（`buildRunContext` で実クライアントを組む）
に生成ロジックを置かない。

### `internal/cli/`

- `project_conventions_test.go` — Kong パースのテスト（既存
  `project_health_test.go` の様式を踏襲）
  - `init` の既定 / `--out` 指定 / `--from-project` 指定
  - `validate` の `--file` 必須 / `--strict`
  - `-f json` がグローバルの format として解釈され、`--file` と衝突しないこと

### `internal/app/`

- `error_envelope_test.go` — `QuietExiter` を実装したエラーで envelope が
  出力されず exit code だけ返ること、既存エラーの挙動が変わらないこと

---

## 完了条件

- `go test ./...` 全パス、`go vet ./...` クリーン
- `logvalet project conventions init` がコメント付きスケルトンを stdout に出し、
  それをそのまま `validate` に通すと error 0 件になる
- `logvalet project conventions init --from-project KEY` が既存プロジェクトの
  カテゴリ・種別・状態を反映した、決定的で YAML として壊れないスケルトンを出す
- `validate` の exit code が error / warning / `--strict` で仕様どおり分岐し、
  stdout に JSON が 1 つだけ出る
- `LoadFromIssueDescription` が規約課題フォーマットをパースできる
- 色 allowlist が上記の確定値と一致している

---

## 依存

- 前提: スキーマ・validate・埋め込みスケルトン・ローダーは LC01 と並行可能。
  **`--from-project` のみ LC01（`domain.IssueType`）に依存**するため、LC01 完了後に着手する
- 後続: LC03（apply の差分計画）、LC05（`conventions show` / MCP）、
  LC06（Initiative 照合・規約課題除外）が本パッケージに乗る。
  ロードマップの「LC01・LC02・LC06 は独立並行可能」は不正確で、
  正しくは LC01 ∥ LC02（本体）→ LC02（`--from-project`）、LC06 は LC02 に依存する
