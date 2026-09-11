# 0004. 関連課題の統合先を抽象度の高い digest 系サーフェスに限定する

## ステータス

承認済み（ユーザー決定、2026-07-31）

## コンテキスト

v0.37.0（run-20260731-related-issues）で関連課題（Related Issues）機能を実装したが、
`lv issue related list|add|remove` という独立サブコマンドと対応 MCP ツール3本のみに留め、
既存の課題取得系サーフェスへの統合は out_of_scope としていた（[[0003]] は取得方式自体の決定）。

ユーザーはこれを指摘した（2026-07-31）: 「`lv issue get` に関連課題が出てこない。他のコマンドや
ツールでも取得できないと意味がない」。関連課題は課題の文脈理解に直結する情報であり、独立
サブコマンドでしか取得できないと、LLM が課題を把握する主経路（issue get / context / digest）で
見えず、機能の価値が実質的に発揮されない。

一次情報の実機検証（2026-07-31、heptagon.backlog.com / IMPRACE-12）により、
`GET /api/v2/issues/:key` の実レスポンスには関連課題フィールドが存在せず（トップレベル28キー中
ゼロ件）、`GET /api/v2/issues/:key/relatedIssues` の別エンドポイントを logvalet 側で追加取得して
合成する必要があることが確定した。つまり「どのサーフェスに合成するか」はコストを伴う設計判断
であり、全サーフェスに機械的に足すのではなく対象を選ぶ必要があった。

## 決定

関連課題の統合先を、単一課題の文脈理解・digest を目的とする抽象度の高いサーフェスに限定する。

- **統合する**: `lv issue context` / `logvalet_issue_context`、`lv issue triage-materials` /
  `logvalet_issue_triage_materials`。いずれも `IssueContext` / `TriageMaterials` の出力末尾に
  `related_issues []RelatedIssueRef` を additive に追加する（既存キーは無変更）。
- **統合しない**（現状維持）: `lv issue get` / `lv issue related list`（API 1:1 の thin wrapper）、
  `lv issue timeline`（主旨がコメント時系列で関連課題と噛み合わない）、
  `digest weekly|daily|unified` / `project health` / `project blockers` / `my tasks` などの
  複数課題列挙・集計系（課題ごとに追加 API 1 本が乗るコストが非現実的）。

取得は常時 ON + opt-out（CLI `--no-include-related-issues` / MCP `include_related_issues: false`）
とし、失敗時は課題本体の取得を失敗させず `related_issues_fetch_failed` warning を envelope の
`warnings` に積む graceful degradation とする。

## 検討した代替案

### 代替案1: `lv issue get` / `issue related list` にもマージする

API と 1:1 対応する thin wrapper 系にも関連課題を出す案。「あらゆる経路で見えるべき」という
ユーザーの元発言に最も忠実ではある。

却下理由: ユーザー自身がスコープを確定させ、API 1:1 の thin wrapper は現状維持・マージ不要と
指示した（intent.md 成果物節）。thin wrapper は「API レスポンスをそのまま返す」という役割の
一貫性が壊れる上、`issue related list` と `issue get` の両方に同じ情報が重複して出ることになり、
LLM 消費者にとってどちらを見るべきかが曖昧になる。intent.md の完了基準に残る
「`lv issue get` の出力に関連課題が含まれる」という記述は、同一文書内でスコープ確定より前に
書かれた古い記述であり、確定スコープを優先して無効化した（plan-summary.md 該当節）。

### 代替案2: `lv issue timeline` にも含める

コメント時系列と関連課題は「課題の文脈」という点で近接している。

却下理由: timeline の主旨はコメントの時系列変化であり、関連課題は時系列の構成要素ではない。
概念的に噛み合わず、出力の一貫性を損なう。

### 代替案3: 複数課題を列挙・集計するサーフェス（digest weekly/daily/unified、project health/blockers、
my tasks、activity/workload 系）にも含める

網羅性は最も高い。

却下理由: intent.md が明示的に「複数課題を列挙する系」を対象外としている。これらのサーフェスは
1回の呼び出しで多数の課題を扱うため、課題ごとに関連課題取得の API 呼び出しが 1 本ずつ乗ると
コストが N 倍化し非現実的。

### 代替案4（統合対象内の副次判断）: `issue context` のみに統合し `triage-materials` は対象外とする

collector-design 段階の推奨は issue context のみだった。実装コストを最小化する選択。

却下理由: planner が review 段階で triage-materials を追加採用した。単一課題の判断材料という
目的が関連課題（特に重複課題の検知）と直結すること、そしてユーザーの不満の根本原因が
「単独サブコマンドでしか取れない到達経路の狭さ」にあり、統合先を1サーフェスに絞ると同種の
不満が再発しうることを理由とする。実装コストは S1 の共有ヘルパー（`internal/analysis/related.go`）
を両サーフェスで再利用するため、追加コストは既存 errgroup への goroutine 1 本分に収まる。

## 影響

- `internal/analysis/related.go`（新規）: `RelatedIssueRef` 射影型と `fetchRelatedIssues` 共有
  ヘルパーを S2/S3 が共通利用する。
- `internal/analysis/context.go` / `triage.go`: `IssueContext` / `TriageMaterials` に
  `related_issues` フィールドを追加、`Build()` の既存 errgroup に3本目の goroutine を追加
  （共有変数は既存 `mu sync.Mutex` 保護下でのみ書き込み、読み取りは `g.Wait()` 後）。
- `internal/cli/issue_context.go` / `issue_triage_materials.go`: `IncludeRelatedIssues`
  （`default:"true" negatable:""`）を追加。
- `internal/mcp/tools_analysis.go`: 両ツールに `include_related_issues` パラメータを追加、
  `tools_list_baseline.json` を再生成（MCP ツール総数ゲート 75 は変更なし）。
- README.md / README.ja.md / docs/specs / skills/context・triage・draft の SKILL.md を同期。
- `lv issue get` / `lv issue related list` は本決定の対象外のまま現状維持（関連課題は出力されない）。
  将来これらへの統合が必要になった場合は、本 ADR の代替案1の却下理由（役割の一貫性・重複）を
  再評価した上で別途決定する。
