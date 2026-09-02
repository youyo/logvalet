---
title: logvalet Linear Conventions Roadmap
project: logvalet
author: planning-agent
created: 2026-09-02
status: Draft
---

# logvalet Linear Conventions Roadmap

> Linear の「曖昧さを許さない構造」を Backlog の語彙に翻訳した規約（conventions）を、
> 宣言的ファイルから Backlog プロジェクトへ冪等に適用し、その規約を AI に読ませるための一連の機能。

## 背景

参考記事: 「ツールではなく、思想を導入する〜カミナシで Linear を導入して分かったこと」
（https://note.com/suzu_4/n/n333d3ff27c18）

記事の核は次の 4 点。ツールの機能ではなく、制約と、その制約を自組織の言葉で埋める作業に価値がある。

1. Initiatives > Projects > Issues の 3 層。Project に紐づかない Issue は「溜まり」として可視化される
2. Project の Lead は 1 人。空欄がむき出しになる
3. Priority は Project との相対で組織の言葉として定義する。low は溜めずにクローズする
4. 組織の言葉になった規約は、そのまま AI エージェントへの指示書になる

logvalet は LLM-first CLI なので、4 が最も直接的に効く。規約を Backlog に「適用する」だけでなく、
「AI に返す」ところまでが本ロードマップのスコープ。

## 用語

| 用語 | 意味 | Backlog 上の実体 |
|---|---|---|
| conventions | 組織の運用規約。Linear の制約を Backlog の語彙に翻訳し、各項目の意味を自分たちの言葉で書いたもの | 規約課題（入力は `conventions.yaml`） |
| Initiative | 数か月規模の重点テーマ。並び順が優先度。案件は必ずいずれかに属する | なし（conventions 内のリスト） |
| 案件（engagement） | 数週間規模の取り組み。Linear の Project に相当 | カテゴリ + 種別「案件」の親課題 |
| Lead | 案件の責任者。1 人だけ | 案件親課題の担当者 |
| 案件親課題 | 案件のヘッダーとなる課題。Lead・期間・状態・Context & Goals を持つ | 種別「案件」の課題 |
| 規約課題 | 規約の正本。説明欄に運用ガイドと YAML を持ち、変更履歴とコメントで規約の議論を残す | 種別「規約」の課題（プロジェクトに 1 件） |
| 曖昧さ（ambiguity） | 規約に照らして決まっていないこと。案件不明の課題、Lead 不在の案件など | health の `ambiguities` |
| apply | conventions を Backlog に冪等に差分適用すること。`--dry-run` で計画だけ表示する | 書き込み |

用語の説明はロードマップのほかに、`conventions init` のスケルトン冒頭コメント、`conventions show` の `glossary` フィールド、docs の導入ガイドの 3 か所に置く。

## 前提となる運用（変更しない）

- Backlog プロジェクト = 顧客
- カテゴリ = 案件
- チームは顧客（プロジェクト）を横断する

## Linear との対応付け

| Linear | Backlog（本設計） | 備考 |
|---|---|---|
| Team | なし | 横断視点は既存の fan-out / multi-space で補う |
| Project | 案件 = カテゴリ + 種別「案件」の親課題 | 親課題が Lead（担当者）・期間（開始日/期限日）・状態・説明テンプレートを持つ |
| Issue | 課題 | 案件カテゴリをちょうど 1 つ持ち、案件親課題の子課題にする |
| Initiative | conventions.yaml 内の順序付きリスト | Backlog 側に横断プロジェクトは作らない。案件は Initiative 名を参照する |
| Priority (4 段階) | 高・中・低（固定） | 4 段階目は足さず、3 段階の意味を組織の言葉で定義する |

マイルストーンを案件の器にする案は不採用。担当者を持てず説明テンプレートもないため Lead の強制ができない。

## Backlog 側の制約（確認済み / 未確認）

- 確認済み（HEP_ISSUES, GET, 2026-09-02）: カスタム属性 endpoint は 200 で空配列。利用可能で未定義
- 確認済み: 状態は既定 4 つのみ。種別 6、カテゴリ 7、バージョン 0
- 未確認: カスタム状態の追加可否。LC03 の dry-run 検証時に sandbox で 1 件 POST して確定する
- 制約: 優先度はカスタマイズ不可。プロジェクト作成はスペース管理者権限が必要
- 制約: カテゴリは複数選択可なので「1 案件に属する」は API で強制できず、検知（LC06）で補う

## 適用範囲

- 対象は新規プロジェクト作成、または sandbox プロジェクトへの適用から始める
- 過去案件の移行（既存課題を親課題にぶら下げ直す）はスコープ外
- 既存プロジェクトへの導入は可能。apply は名前で照合する冪等な差分適用なので既存のカテゴリ・種別を壊さない。導入手順は health で棚卸し → `init --from-project` → apply の順

## コマンド設計

新トップレベルコマンドは作らず `project` 配下に置く。

| 機能 | コマンド | MCP |
|---|---|---|
| スケルトン生成 | `logvalet project conventions init [-o conventions.yaml] [--from-project KEY]` | なし |
| 規約の検証 | `logvalet project conventions validate -f conventions.yaml [--strict]` | なし |
| 差分表示 | `logvalet project apply -f conventions.yaml --project KEY --dry-run` | なし |
| 適用 | `logvalet project apply -f conventions.yaml --project KEY [--create]` | なし（書き込みは CLI のみ） |
| 規約の読み出し | `logvalet project conventions show --project KEY`（規約課題から読む） | `logvalet_project_conventions` |
| 案件指定の起票 | `logvalet issue create --engagement 案件名 ...` | 既存 `logvalet_issue_create` に `engagement` 追加 |
| 曖昧さ検知 | `logvalet project health KEY`（既存に統合） | 既存 `logvalet_project_health` |

`apply` は書き込みを伴うため MCP には出さない（記事の「一括更新は必ず人の承認を取る」に従う）。

### dry-run の出力（案）

`--dry-run` は差分計画（plan）を人間向けテキストと JSON の両方で出す。`apply` 本体は同じ plan を実行するだけにし、dry-run と実行のズレを構造的に防ぐ。

```
project SANDBOX
  issue_type  + 案件 (template: 3 sections)
  status      + レビュー中                      (skip if custom status unavailable)
  category    = 開発チーム                      (exists, no change)
  category    + 顧客A 基盤更改
  issue       + [案件] 顧客A 基盤更改  lead=山田 太郎  due=2026-10-31
  issue       ~ [案件] 運用保守       lead: (none) -> 鈴木 花子
plan: 4 create, 1 update, 1 skip, 1 unchanged
```

記号は `+` 作成、`~` 更新、`=` 変更なし、`!` スキップ（理由付き）。JSON は `{"resource","action","name","changes","reason"}` の配列。

## conventions.yaml スキーマ（案）

`init` が出力するスケルトンは、全項目にコメントで説明を付ける（AI と人間の両方が読む前提）。

```yaml
# logvalet conventions: Linear の思想を Backlog の語彙に翻訳した運用規約。
# 値の説明に見えて、書いているのは組織の優先順位に対する態度。埋めるのは自分たちの言葉。
schema_version: 1

project:
  # 適用先の Backlog プロジェクトキー。--project フラグで上書きできる。
  key: SANDBOX
  # --create で新規作成するときの表示名。既存プロジェクトへの適用では無視される。
  name: Sandbox

# 優先度の意味。Backlog は 高・中・低 の 3 段階固定なので段階は足さず、意味を定義する。
# 案件（engagements）との相対で書く。Initiative に紐づかない課題で「中」が増えるのは
# 優先度を決めきれていない裏返しとして扱う。
priority:
  high: "契約・SLA 上、他案件を止めてでも対応する"
  normal: "案件と同じ優先度。必ず担当者を割り当て、担当者が責任を持つ"
  low: "案件より劣後し、実行は保証されない。進むには担当者の熱量か 20% ルール的な仕組みが必要"

# クローズも決断のうち。低優先度を溜め続けないための規約。
close_policy:
  # この日数を超えて未着手の「低」課題を project health でクローズ候補として挙げる。
  low_untouched_days: 90

# 既定の 4 状態（未対応・処理中・処理済み・完了）に追加する状態。
# カスタム状態が使えないプランでは警告してスキップする。不要なら空リストにする。
statuses:
  - name: レビュー中
    # Backlog が許可する色コード（例: "#ea8462"）。
    color: "#ea8462"

# 課題種別。「案件」は案件のヘッダー（親課題）、「規約」はこの規約を保存する規約課題に使う。どちらも必須。
issue_types:
  - name: 規約
    color: "#666665"
  - name: 案件
    color: "#7ea800"
    # 案件親課題を起票するときに説明欄へ自動挿入されるテンプレート。
    # Context & Goals / Scope / Acceptance criteria を埋めないと案件を始められない構造にする。
    template_description: |
      ## Context & Goals
      （なぜやるのか、何を達成するのか）
      ## Scope
      （やること / やらないこと）
      ## Acceptance criteria
      （何をもって完了とするか）

# Initiative: 数か月規模の重点テーマ。Backlog には対応する概念がないので、この一覧で持つ。
# 並び順がそのまま優先度。横断テーマと顧客テーマのどちらが上かを明示せざるを得ない。
# 案件は必ずいずれかの Initiative に属する。定常業務も「運用保守」のように明示して置く。
initiatives:
  - name: 運用保守
    description: "契約範囲内の定常対応。案件が属する Initiative を決めきれないときの逃げ場にしない"

# 案件: 数週間規模の取り組み。1 件ごとにカテゴリと「案件」種別の親課題を作る。
# 課題（子課題）は案件カテゴリをちょうど 1 つ持ち、案件親課題の子にする。
engagements:
  - # 案件名。カテゴリ名と親課題の件名に使う。必須。
    name: ""
    # Lead。Backlog 上の表示名で指定し、apply がユーザー ID に解決して親課題の担当者にする。
    # 1 人だけ。空欄のまま apply すると警告（--strict では exit 2）。決めていない案件は始めない。
    lead: ""
    # 属する Initiative。initiatives[].name を参照する。必須。
    initiative: 運用保守
    # 期間。親課題の開始日 / 期限日に反映する。YYYY-MM-DD。
    start_date: ""
    due_date: ""
```

- `init` はデフォルト値入りのスケルトンを出力する。空欄は `engagements[].name` と `lead` のみ
- `init --from-project KEY` は既存プロジェクトのカテゴリを `engagements[]` の候補、種別を `issue_types[]` として埋めたスケルトンを出す。既存プロジェクトへ導入するときの起点にする
- `validate` は空欄を警告として stderr に出す。`--strict` のときだけ exit 2
- `apply` は名前で既存リソースを照合し、存在すればスキップ、差分があれば更新、なければ作成する
- `lead` は表示名で書き、apply 時にプロジェクトメンバー一覧から ID に解決する。同名が複数いれば exit 2
- 課題と案件の紐づけは親子課題機能を使う（運用上すでに有効）。`--create` 時は subtaskingEnabled を有効にして作成する

## マイルストーン

| ID | 内容 | 依存 | 規模 |
|---|---|---|---|
| LC01 | backlog クライアントに書き込み API 追加（CreateProject, AddCategory, AddIssueType, AddStatus, AddCustomField） | なし | M |
| LC02 | conventions スキーマ・ローダー・validate・init（スケルトン生成、`--from-project`） | なし | M |
| LC03 | `project apply --dry-run`（差分計画の生成と表示） | LC01, LC02 | M |
| LC04 | `project apply` 本体（冪等適用、golden test） | LC03 | M |
| LC05 | `project conventions show` CLI + MCP ツール（`glossary` を含む） | LC02 | S |
| LC06 | analysis 層に曖昧さ検知を追加し `project health` に統合 | なし | M |
| LC07 | `issue create / update` に `--engagement` 追加と規約違反の警告 | LC05 | S |
| LC08 | スキル `logvalet` への規約参照の追記、docs（導入ガイド + 用語集）、E2E、リリース | LC04, LC06, LC07 | S |

LC01 と LC02 と LC06 は独立で並行可能。LC05 と LC06 は apply がなくても価値が出る。

### 規約の保存場所（読み出しの正本）

- `conventions.yaml` は apply の入力。MCP サーバーは stateless で複数ユーザーから使われるため、ローカルファイルを読み出しの正本にはしない
- 正本は「規約課題」。プロジェクトに種別「規約」の課題を 1 件だけ置き、説明欄に人間向けの運用ガイドと用語集、末尾に YAML をコードブロックで埋め込む
- Backlog Document は更新・削除 API がなく、Wiki は運用上使っていないため不採用
- 作成は既存 `CreateIssue`、更新は既存 `UpdateIssue`。規約の変更は Backlog の更新履歴に残り、議論はコメントで行う
- 規約課題の特定はプロジェクト内で種別「規約」の課題を検索する。0 件なら「規約未導入」として警告なしに動き（既存の挙動を変えない）、2 件以上なら exit 2
- `conventions show`、MCP ツール、LC07 の警告は規約課題の説明欄からコードブロック内の YAML をパースする。ローカルファイルは `-f` で明示したときだけ優先する
- 規約課題は閉じないため、stale / blockers / health / workload の分析対象から種別「規約」を除外する（LC06 で対応）

### LC07: issue コマンドの対応

- `issue create / update --engagement 案件名`: 案件名から案件親課題とカテゴリを解決し、`parent_issue_id` と `category` を同時に設定する。既存の `--category` / `--parent-issue-id` との併用時は明示指定を優先
- 規約導入済みプロジェクトで案件カテゴリが 0 個または 2 個以上なら stderr に警告する。exit code と書き込みは変えない
- `issue list --engagement` は既存の `--parent-issue-id` で代替できるため対象外

### LC06 の検知項目

- 案件カテゴリが 0 個または 2 個以上の課題
- 親課題（種別「案件」）のない案件カテゴリ
- 担当者または期限のない案件親課題
- conventions の Initiative に紐づかない案件
- `close_policy.low_untouched_days` を超えて未着手の低優先度課題（クローズ候補）
- 既存の stale / blockers / workload から種別「規約」の課題を除外する

既存の `AnalysisEnvelope` に `ambiguities` として載せ、`health_score` の減点要因に加える。

## 既存コードとの接点

- 読み取り API は揃っている: `ListProjectStatuses` / `ListProjectCategories` / `ListProjectIssueTypes` / `ListProjectCustomFields`（`internal/backlog/client.go:114-132`）
- 子課題の絞り込みは `ParentIssueIDs` が既にある（`internal/backlog/http_client.go:356`）
- 書き込みは `CreateIssue` / `UpdateIssue` のみ。LC01 で追加する
- project コマンド群は `internal/cli/project.go:13` の `ProjectCmd` に追加する
- 検知は `internal/analysis/` の既存パターン（`BlockerDetector`, `ProjectHealthBuilder`）を踏襲する

## 設計原則

- 宣言的スキーマで定義し、apply ロジックは 1 か所に集約する
- CLI と MCP は同じドメインモデル・同じ出力スキーマを共有する
- 書き込みは dry-run を必ず持ち、MCP には出さない
- Backlog API テストはモックのみ。golden test で apply の差分計画を固定する
- TDD 必須
