---
title: マイルストーン LC01 - プロジェクトメタ書き込み API
project: logvalet
author: planning-agent
created: 2026-09-03
status: Ready for Review
complexity: M
---

# マイルストーン LC01: プロジェクトメタ書き込み API

## 概要

`project apply`（LC03/LC04）が conventions を Backlog に適用するために必要な書き込み API を
`backlog.Client` に追加する。読み取り API（`ListProjectStatuses` / `ListProjectCategories` /
`ListProjectIssueTypes`）は既に揃っているため（`internal/backlog/client.go:114-132`）、
本マイルストーンは対になる作成・更新系だけを足す。

apply は「名前で照合し、なければ作成・差分があれば更新」する冪等な差分適用なので、
Add 系だけでは足りない。課題種別のテンプレートや状態の色を後から直せるよう Update 系も入れる。
削除系は apply が行わない（conventions から項目を消しても Backlog 側は消さない）ため実装しない。

本マイルストーンは **API のラッパーだけ**を担う。名前照合・差分計算・スキップ判定・
部分失敗時の扱いはすべて LC03/LC04 の責務であり、ここには入れない。

---

## スコープ

### 実装範囲

- `internal/domain/domain.go` — `IssueType` 型の追加
- `internal/backlog/types.go` — リクエスト型 7 種の追加
- `internal/backlog/client.go` — `Client` インターフェースに 8 メソッド追加、
  `ListProjectIssueTypes` の戻り値変更
- `internal/backlog/http_client.go` — `HTTPClient` の実装
- `internal/backlog/http_client_test.go` — httptest ベースのユニットテスト
- `internal/backlog/mock_client.go` — `MockClient` に `*Func` フィールドと実装を追加
- `internal/cli/issue.go` — 戻り値変更に伴う呼び出し側の追従（下記「設計判断 1」）

### スコープ外

- `project apply` コマンド本体、名前照合・差分計算・冪等性の担保（LC03/LC04）
- conventions スキーマ・ローダー（LC02）
- 削除系 API（`DeleteCategory` 等）— apply が破壊的操作を行わないため不要
- **`AddCustomField`** — 現行 conventions スキーマ（LC02）はカスタム属性を持たず、
  apply から呼ばれない。`applicableIssueTypes[]` / `allowInput` / `allowAddItem` の
  マッピングと `domain.CustomFieldDefinition` の拡張が別途必要になるため、
  スキーマに `custom_fields` を足すと決めた時点で別マイルストーンとして切る
- カスタム状態が実際に作れるかの検証（LC03 の sandbox 検証で確定）

---

## 設計判断

### 1. `ListProjectIssueTypes` の戻り値を `domain.IssueType` に変更する

課題種別は `color` と `templateSummary` / `templateDescription` を持つが、現在の戻り値
`[]domain.IDName` はこれらを捨てている。LC03 の dry-run が
「`issue_type ~ 案件 (template: 変更あり)`」を判定するにはテンプレートの現在値が要る。

新メソッドを足して公開面を増やすのではなく、既存メソッドの戻り値を拡張する。

**影響箇所と対応（Go のスライスは共変でないため、単なるフィールド参照置換では通らない）:**

| 箇所 | 現状 | 対応 |
|---|---|---|
| `internal/cli/issue.go:235` | `resolveNameOrID(name, issueTypes)` に `[]domain.IDName` を渡す | 新ヘルパー `issueTypesAsIDNames([]domain.IssueType) []domain.IDName` を `internal/cli/resolve.go` に追加して噛ませる |
| `internal/cli/issue.go:488` | 同上 | 同上 |
| `internal/mcp/tools_meta.go:35` | `logvalet_meta_issue_types` が結果をそのまま返す | 変更不要。出力 JSON に `color` / `templateSummary` / `templateDescription` が増える |
| `internal/backlog/mock_client.go:54,332` | `ListProjectIssueTypesFunc` の型 | 追従 |
| `internal/backlog/http_client.go:819` | デコード先の型 | 追従 |
| 各 `_test.go` | 期待値の型 | 追従 |

`resolveNameOrID`（`internal/cli/resolve.go:334`）自体は `[]domain.IDName` のまま変えない。
ジェネリック化は他の呼び出し（優先度・状態など）にも波及して得がない。

MCP `logvalet_meta_issue_types` の出力はキーが増えるだけで既存キーは変わらないため後方互換。
`internal/mcp/tools_list_baseline.json` は入力スキーマの baseline でありレスポンスの golden
ではないため、この変更では更新不要。代わりに `internal/mcp/tools_meta_test.go` に
`logvalet_meta_issue_types` のレスポンスを検証するケースを新規追加し、
新フィールドが JSON に出ることを固定する。

### 2. `domain.IssueType` の JSON タグは camelCase にする

`HTTPClient.do` は変換層なしで API レスポンスを直接デコードするため、タグは Backlog API の
camelCase に一致させる必要がある。既存の `domain.Status` / `domain.Category`
（`internal/domain/domain.go:153-166`）も camelCase タグをそのまま持っており、これに揃える。

```go
// IssueType は課題種別情報。
type IssueType struct {
	ID                  int    `json:"id"`
	ProjectID           int    `json:"projectId"`
	Name                string `json:"name"`
	Color               string `json:"color,omitempty"`
	DisplayOrder        int    `json:"displayOrder"`
	TemplateSummary     string `json:"templateSummary,omitempty"`
	TemplateDescription string `json:"templateDescription,omitempty"`
}
```

MCP 出力のキーも camelCase になるが、これは既存の `logvalet_meta_statuses` /
`logvalet_meta_categories` と同じ挙動であり一貫している。
CLAUDE.md の「JSON キーは snake_case」は logvalet 自身が組み立てる digest / analysis の
出力スキーマに対する規約であり、Backlog API のパススルー型は既存どおり camelCase を維持する。

### 3. カスタム状態の失敗は既存の typed error のまま返す

当初案の `ErrStatusNotConfigurable`（400/403 をまとめてラップ）は導入しない。
400 は不正な色・名前・上限超過、403 は権限・プラン制限と原因が異なり、
一括して「スキップ可能」と扱うと規約不備を成功扱いにしてしまう。

`AddStatus` / `UpdateStatus` は既存の `do` が返す typed error をそのまま返す。
既存の変換規則は **404 → `ErrNotFound`、401 → `ErrUnauthorized`、403 → `ErrForbidden`、
422 → `ErrValidation`、429 → `ErrRateLimited`、その他（400 を含む）→ `ErrAPI`**
（`internal/backlog/http_client.go:249-263`）。400 を `ErrValidation` に付け替えるような
既存挙動の変更は行わない（LC01 のスコープ外）。
`BacklogError` は Backlog の `errors[].code` を保持しているので、
「カスタム状態が使えないプランならスキップ」の判定に必要な code の絞り込みは
LC03 の sandbox 検証で実値を確定してから apply 側に置く。

> 既存 sentinel は `ErrNotFound` / `ErrUnauthorized` / `ErrForbidden` / `ErrRateLimited` /
> `ErrValidation` / `ErrAPI` / `ErrDownloadTooLarge`（`internal/backlog/errors.go:19-37`）。
> `ErrPermission` という名前は存在しないので使わない。

### 4. `CreateProject` の bool はポインタで持つ

`textFormattingRule` の既定や各種 bool は「未指定」と `false` を区別する必要がある。
`CreateIssueRequest` の慣習（0 = 未指定）は bool に使えないため、ポインタで持ち、
nil のときはフォームに載せない。API 側の既定に委ねる。

`subtaskingEnabled` だけは規約上必ず true にする必要がある（規約課題・案件親課題の
親子関係が前提）。ゼロ値のまま送られる事故を防ぐため、`CreateProject` は
`SubtaskingEnabled` が nil のときに true を補って送る。

---

## 追加する型

`internal/backlog/types.go`:

```go
// CreateProjectRequest は CreateProject リクエストのパラメータ。
// bool はポインタ（nil = 未指定。API の既定に委ねる）。
type CreateProjectRequest struct {
	Name                              string // 必須
	Key                               string // 必須。大文字英数字と _ のみ
	ChartEnabled                      *bool
	SubtaskingEnabled                 *bool // nil のときは true を送る
	ProjectLeaderCanEditProjectLeader *bool
	GrandchildIssueEnabled            *bool // subtaskingEnabled が true のときのみ有効
	UseDevAttributes                  *bool // 優先度・バージョン・マイルストーンの有効化
	TextFormattingRule                string // "backlog" | "markdown"。空なら送らない
}

// AddCategoryRequest はカテゴリ追加のパラメータ。
type AddCategoryRequest struct {
	Name string // 必須
}

// UpdateCategoryRequest はカテゴリ更新のパラメータ。
type UpdateCategoryRequest struct {
	Name string // 必須
}

// AddIssueTypeRequest は課題種別追加のパラメータ。
type AddIssueTypeRequest struct {
	Name                string // 必須
	Color               string // 必須。Backlog が許可する色コードのみ
	TemplateSummary     string
	TemplateDescription string
}

// UpdateIssueTypeRequest は課題種別更新のパラメータ。
// 全フィールドはポインタ型（nil = 変更しない）。
type UpdateIssueTypeRequest struct {
	Name                *string
	Color               *string
	TemplateSummary     *string
	TemplateDescription *string
}

// AddStatusRequest は状態追加のパラメータ。
type AddStatusRequest struct {
	Name  string // 必須
	Color string // 必須。状態用の許可色コードのみ
}

// UpdateStatusRequest は状態更新のパラメータ。
// 全フィールドはポインタ型（nil = 変更しない）。
type UpdateStatusRequest struct {
	Name  *string
	Color *string
}
```

---

## 追加するメソッド

| メソッド | Backlog API | 戻り値 |
|---|---|---|
| `CreateProject(ctx, req CreateProjectRequest)` | `POST /api/v2/projects` | `*domain.Project` |
| `AddCategory(ctx, projectKey, req AddCategoryRequest)` | `POST /api/v2/projects/{key}/categories` | `*domain.Category` |
| `UpdateCategory(ctx, projectKey, categoryID int, req UpdateCategoryRequest)` | `PATCH /api/v2/projects/{key}/categories/{id}` | `*domain.Category` |
| `AddIssueType(ctx, projectKey, req AddIssueTypeRequest)` | `POST /api/v2/projects/{key}/issueTypes` | `*domain.IssueType` |
| `UpdateIssueType(ctx, projectKey, issueTypeID int, req UpdateIssueTypeRequest)` | `PATCH /api/v2/projects/{key}/issueTypes/{id}` | `*domain.IssueType` |
| `AddStatus(ctx, projectKey, req AddStatusRequest)` | `POST /api/v2/projects/{key}/statuses` | `*domain.Status` |
| `UpdateStatus(ctx, projectKey, statusID int, req UpdateStatusRequest)` | `PATCH /api/v2/projects/{key}/statuses/{id}` | `*domain.Status` |
| `ListProjectIssueTypes(ctx, projectKey)` | `GET /api/v2/projects/{key}/issueTypes` | `[]domain.IssueType`（**戻り値変更**） |

### 確定済み API 仕様（Backlog API ドキュメント, 2026-09-03 確認）

すべて `Content-Type: application/x-www-form-urlencoded`。

- `POST /api/v2/projects` — 必須は `name` / `key`（key は A-Z, 0-9, `_` のみ）。
  任意: `chartEnabled` `useResolvedForChart` `subtaskingEnabled`
  `grandchildIssueEnabled` `projectLeaderCanEditProjectLeader` `useWiki` `useDocument`
  `useFileSharing` `useWikiTreeView` `useSubversion` `useGit`
  `useOriginalImageSizeAtWiki` `textFormattingRule`（"backlog" | "markdown"）
  `useDevAttributes`。**Administrator 権限が必要**
- `POST|PATCH /api/v2/projects/{key}/categories[/{id}]` — `name`（追加時は必須）。
  全権限で可
- `POST|PATCH /api/v2/projects/{key}/issueTypes[/{id}]` — `name` / `color`
  （追加時は必須）、`templateSummary` / `templateDescription`。全権限で可。
  レスポンスは `{id, projectId, name, color, displayOrder, templateSummary,
  templateDescription}`
- `POST|PATCH /api/v2/projects/{key}/statuses[/{id}]` — `name` / `color`
  （追加時は必須）。**Administrator 権限が必要**。既定 4 状態のほかに
  **最大 8 個**まで
- 色は固定 allowlist のみ（LC02 の `conventions.StatusColors` /
  `IssueTypeColors` と同じ値）
  - 状態: `#ea2c00` `#e87758` `#e07b9a` `#868cb7` `#3b9dbd` `#4caf93` `#b0be3c`
    `#eda62a` `#f42858` `#393939`
  - 種別: `#e30000` `#990000` `#934981` `#814fbc` `#2779ca` `#007e9a` `#7ea800`
    `#ff9200` `#ff3265` `#666665`

---

## 実装方針

既存の `CreateIssue`（`internal/backlog/http_client.go:409`）のパターンをそのまま踏襲する。

- `url.Values` にパラメータを積み、`c.newBodyRequest(ctx, http.MethodPost, path, q)` で
  `application/x-www-form-urlencoded` リクエストを作る
- `c.do(req, &out)` でデコードとエラー変換を行う（typed errors への変換は `do` が担う）
- 未指定フィールドは `q.Set` しない（Update 系はポインタ nil で判定、`CreateProject` の
  bool もポインタ nil で判定）
- パスに含める `projectKey` は `url.PathEscape` する（既存メソッドと同じ扱い）

---

## テスト計画（TDD: Red → Green → Refactor）

`internal/backlog/http_client_test.go` に既存の httptest パターンで追加する。
各メソッドにつき以下を書く。

1. **正常系** — 期待するメソッド・パス・フォームボディを検証し、固定 JSON を返して
   デコード結果を突き合わせる
2. **エラー系** — 401 / 403 / 404 を返し、`ErrUnauthorized` / `ErrForbidden` /
   `ErrNotFound` に変換されることを確認

個別に足すケース:

- `CreateProject`: `SubtaskingEnabled` が nil でも `subtaskingEnabled=true` がボディに乗ること、
  他の nil bool がボディに現れないこと、`TextFormattingRule` が空なら送らないこと
- `UpdateIssueType` / `UpdateStatus` / `UpdateCategory`: nil フィールドがボディに現れないこと
- `AddStatus`: 400 で `errors.Is(err, ErrAPI)`、403 で `errors.Is(err, ErrForbidden)`
  になり、`BacklogError.Code` に Backlog の `errors[].code` が保持されること
- `ListProjectIssueTypes`: `templateSummary` / `templateDescription` / `color` を含む JSON を
  デコードでき `domain.IssueType` に載ること（戻り値変更の回帰テスト）

`MockClient` にはフィールドを足すだけでロジックを持たせない（既存方針どおり）。
`internal/backlog/mock_client_test.go` に、`*Func` 未設定時にゼロ値を返すことと
呼び出しカウントが増えることを確認するケースを追加する。

`internal/mcp/tools_meta_test.go` に `logvalet_meta_issue_types` のレスポンス検証を追加し、
`color` / `templateSummary` / `templateDescription` が出力 JSON に含まれることを固定する。

---

## 完了条件

- `go test ./...` が全パス
- `go vet ./...` がクリーン
- `Client` インターフェースに 7 メソッドが追加され、`HTTPClient` と `MockClient` の
  両方が満たしている
- `ListProjectIssueTypes` が `[]domain.IssueType` を返し、`internal/cli/issue.go` の
  2 か所が `issueTypesAsIDNames` 経由で `resolveNameOrID` に渡している
- `logvalet_meta_issue_types` のレスポンス検証テストが新フィールドを含む形で存在する

---

## 依存

- 前提: なし。LC02 本体（スキーマ・validate・スケルトン）と並行可能
- 後続:
  - LC02 の `--from-project` が `domain.IssueType` の `templateSummary` /
    `templateDescription` を必要とする（LC01 完了後に実装する）
  - LC03（`project apply --dry-run`）が本 API の存在を前提にする

## LC03/LC04 への申し送り

LC01 では扱わないが、apply の設計時に必ず決める必要がある事項:

- **名前照合の複数一致**: 既存 Backlog 側に同名のカテゴリ・種別・状態が 2 件以上あるとき、
  誤更新を避けるため mutation 前に失敗させる（exit 2）
- **部分失敗**: プロジェクト作成 → 種別・カテゴリ・状態追加 → 規約課題 → 案件親課題は
  トランザクションにならない。処理順、作成済みリソースの記録、再実行時の冪等性、
  部分失敗の exit code（`app.ExitPartialFailure` = 8）を plan の一部として定義する
- **カスタム状態の可否**: sandbox に `AddStatus` を 1 件 POST し、失敗する場合の
  `BacklogError.Code` の実値を確認してからスキップ判定を書く
