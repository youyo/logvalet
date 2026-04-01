# Roadmap v3: logvalet AI ネイティブ機能 (Phase 1–3)

## Meta
| 項目 | 値 |
|------|---|
| ゴール | logvalet に AI ネイティブ分析・ワークフロー・インテリジェンス機能を段階的に追加 |
| 成功基準 | Phase 1: 全 analysis コマンドが deterministic に動作し JSON スキーマで出力。Phase 2: LLM 連携ワークフローが MCP 経由で利用可能。Phase 3: リスク・異常検知が自動化 |
| 制約 | Go 1.26.1 / Kong CLI / TDD 必須 / モックベーステスト / CLI+MCP 二重実装禁止 / deterministic 優先 |
| 対象リポジトリ | github.com/youyo/logvalet |
| 作成日 | 2026-04-01 |
| ステータス | 計画中 |
| 前バージョン | plans/logvalet-roadmap-v2.md (v2, M01-M16 全完了) |

## Current Focus
- **マイルストーン**: M20 から開始
- **前提**: v0.5.0 リリース済み、M01-M16 完了

---

## アーキテクチャ概要

### 設計原則

1. **digest は要約、analysis は分析・洞察** — `internal/digest/` は既存データの構造化要約。`internal/analysis/` は課題群に対する判定・分類・検出ロジック
2. **Analyzer パターン** — 各分析機能は独立した Builder/Detector を実装。入力は `backlog.Client` + スコープ、出力は `AnalysisEnvelope`
3. **CLI/MCP 共通化** — analysis パッケージが全ロジックを持ち、CLI は `render` 経由、MCP は JSON 直接返却
4. **deterministic first** — Phase 1 は LLM 不要。Phase 2 以降で LLM 連携を段階的に導入
5. **clock injection** — 時刻依存ロジックは `now func() time.Time` で注入しテスタビリティを確保

### 新パッケージ: `internal/analysis/`

```
internal/analysis/
  analysis.go           — 共通型（AnalysisEnvelope, AnalysisLLMHints, Severity 等）
  analysis_test.go      — 共通型テスト
  context.go            — IssueContextBuilder（課題の総合コンテキスト）
  context_test.go
  stale.go              — StaleIssueDetector（停滞課題検出）
  stale_test.go
  blockers.go           — BlockerDetector（進行阻害要因抽出）
  blockers_test.go
  workload.go           — WorkloadAnalyzer（担当者負荷分析）
  workload_test.go
  project_health.go     — ProjectHealthAnalyzer（プロジェクト健全性）
  project_health_test.go
```

### 新パッケージ: `internal/workflow/` (Phase 2)

```
internal/workflow/
  workflow.go           — 共通型（WorkflowEnvelope）
  draft_comment.go      — DraftCommentBuilder
  triage.go             — IssueTriageBuilder
  spec_to_issues.go     — SpecToIssuesBuilder
  periodic_digest.go    — PeriodicDigestBuilder
```

### 新パッケージ: `internal/intelligence/` (Phase 3)

```
internal/intelligence/
  intelligence.go       — 共通型
  decision_log.go       — DecisionLogExtractor
  activity_intel.go     — ActivityIntelligenceAnalyzer
  risk.go               — RiskSummaryBuilder
  roadmap.go            — RoadmapAssistant
```

### 共通型定義

```go
// AnalysisEnvelope は全 analysis コマンドの共通ラッパー。
// DigestEnvelope と同構造だが resource が "analysis/*" になる。
type AnalysisEnvelope struct {
    SchemaVersion string           `json:"schema_version"`
    Resource      string           `json:"resource"`
    GeneratedAt   time.Time        `json:"generated_at"`
    Profile       string           `json:"profile"`
    Space         string           `json:"space"`
    BaseURL       string           `json:"base_url"`
    Warnings      []domain.Warning `json:"warnings"`
    Analysis      interface{}      `json:"analysis"`
}

// Severity は分析結果の深刻度。
type Severity string
const (
    SeverityCritical Severity = "critical"
    SeverityHigh     Severity = "high"
    SeverityMedium   Severity = "medium"
    SeverityLow      Severity = "low"
    SeverityInfo     Severity = "info"
)

// AnalysisLLMHints は分析結果向け LLM ヒント。
type AnalysisLLMHints struct {
    Findings           []string `json:"findings"`
    RecommendedActions []string `json:"recommended_actions"`
    RiskIndicators     []string `json:"risk_indicators"`
}

// BaseAnalysisBuilder は全 AnalysisBuilder に共通するフィールドと helper。
type BaseAnalysisBuilder struct {
    client  backlog.Client
    profile string
    space   string
    baseURL string
    now     func() time.Time  // テスト用 clock injection
}
```

### 既存パッケージとの関係

```
backlog.Client (interface)
    ├── digest/       ← 既存: 要約・構造化
    ├── analysis/     ← 新規: 分析・洞察・検出 (Phase 1)
    ├── workflow/     ← 新規: ワークフロー (Phase 2)
    ├── intelligence/ ← 新規: 高度分析 (Phase 3)
    ├── cli/          ← Kong コマンド定義（全パッケージを呼ぶ）
    ├── mcp/          ← MCP tool 登録（全パッケージを呼ぶ）
    └── render/       ← 出力フォーマッタ（全結果を描画）
```

---

## Phase 1: AI ネイティブ操作層 (M20–M26)

全て deterministic。LLM 不要。既存の `backlog.Client` API のみで実装。

### M20: analysis パッケージ基盤 + IssueContext
**依存**: なし
**所要時間**: 1.5h
**完了条件**: `analysis.NewIssueContextBuilder().Build()` が `AnalysisEnvelope` を返す。テスト全パス。

- [ ] `internal/analysis/analysis.go` — 共通型定義
  - `AnalysisEnvelope`, `Severity`, `AnalysisLLMHints`, `BaseAnalysisBuilder`
  - `newEnvelope()` ヘルパー（digest.BaseDigestBuilder.newEnvelope と同パターン）
- [ ] `internal/analysis/analysis_test.go` — 共通型テスト（envelope 生成）
- [ ] `internal/analysis/context.go` — `IssueContextBuilder`
  - 入力: issue_key
  - API 呼び出し: `GetIssue` + `ListIssueComments` + `ListProjectStatuses`（errgroup 並行）
  - 出力: `IssueContext` 構造体
- [ ] `internal/analysis/context_test.go` — TDD テスト

**テストケース詳細**:
```
TestNewEnvelope_shape                            — envelope の全フィールド検証
TestSeverity_values                              — Severity 定数値検証
TestIssueContextBuilder_Build_success            — 全データ取得成功、envelope 構造検証
TestIssueContextBuilder_Build_withComments        — コメント付き課題のコンテキスト
TestIssueContextBuilder_Build_commentsFail        — コメント取得失敗→warning、課題データは返る
TestIssueContextBuilder_Build_issueNotFound       — 課題不存在→エラー
TestIssueContextBuilder_Build_envelopeShape       — schema_version, resource, generated_at 等の検証
```

### M21: IssueContext CLI + MCP 登録
**依存**: M20
**所要時間**: 1h
**完了条件**: `lv issue context PROJ-123` と `logvalet_issue_context` MCP tool が動作

- [ ] `internal/cli/issue.go` — `IssueContextCmd` 追加
  - `IssueCmd` に `Context IssueContextCmd` フィールド追加
  - フラグ: `IssueIDOrKey string` (arg, required), `--comments int` (default 20)
- [ ] `internal/cli/issue_context_test.go` — CLI テスト（Kong パース検証）
- [ ] `internal/mcp/tools_analysis.go` — `RegisterAnalysisTools()` 新規
  - `logvalet_issue_context` tool 登録
- [ ] `internal/mcp/tools_analysis_test.go` — MCP tool テスト
- [ ] `internal/mcp/server.go` — `RegisterAnalysisTools(reg)` 追加

**CLI コマンド**:
```
lv issue context PROJ-123 [--comments 20] [--format json|yaml|md]
```

**MCP tool**:
```
logvalet_issue_context { "issue_key": "PROJ-123", "comment_limit": 20 }
```

### M22: StaleIssueDetector
**依存**: M20
**所要時間**: 1.5h
**完了条件**: `StaleIssueDetector.Detect()` が停滞課題を検出。テスト全パス。

- [ ] `internal/analysis/stale.go` — `StaleIssueDetector`
  - 入力: project_key, stale_days (default 14), exclude_statuses
  - ロジック: `ListIssues` → 各課題の `Updated` を `now - stale_days` と比較
  - 出力: `StaleIssuesResult` 構造体
  - now は `func() time.Time` で inject（テスタビリティ）
- [ ] `internal/analysis/stale_test.go` — TDD テスト

**テストケース詳細**:
```
TestStaleIssueDetector_Detect_findsStaleIssues   — 14日超未更新の課題を検出
TestStaleIssueDetector_Detect_boundary           — ちょうど14日=staleでない、15日=stale
TestStaleIssueDetector_Detect_excludesClosed     — 完了ステータスは除外
TestStaleIssueDetector_Detect_emptyProject       — 課題なし→空結果
TestStaleIssueDetector_Detect_customDays         — stale_days=7 でカスタム閾値
TestStaleIssueDetector_Detect_envelopeShape      — AnalysisEnvelope の構造検証
```

### M23: StaleIssues CLI + MCP
**依存**: M22, M21
**所要時間**: 1h
**完了条件**: `lv analyze stale-issues -k PROJ` と MCP tool が動作

- [ ] `internal/cli/analyze.go` — `AnalyzeCmd` ルート新規 + `StaleIssuesCmd`
  - `AnalyzeCmd` を `CLI` struct に追加
  - フラグ: `--project-key` / `-k` (required), `--days 14`, `--exclude-status "完了,Close"`
- [ ] `internal/cli/analyze_test.go` — CLI テスト
- [ ] `internal/cli/root.go` — `Analyze AnalyzeCmd` 追加
- [ ] `internal/mcp/tools_analysis.go` — `logvalet_analyze_stale_issues` 追加

**CLI コマンド**:
```
lv analyze stale-issues -k PROJ [--days 14] [--exclude-status "完了,Close"]
```

**MCP tool**:
```
logvalet_analyze_stale_issues { "project_key": "PROJ", "stale_days": 14, "exclude_statuses": ["完了"] }
```

### M24: BlockerDetector
**依存**: M20
**所要時間**: 1.5h
**完了条件**: `BlockerDetector.Detect()` が進行阻害要因を検出。テスト全パス。

- [ ] `internal/analysis/blockers.go` — `BlockerDetector`
  - 入力: project_key, long_running_days (default 30)
  - 検出ルール (全て deterministic):
    1. **overdue**: `DueDate < now` かつ未完了 → severity=critical if >7d, high otherwise
    2. **no_assignee**: 担当者未設定かつ未完了 → severity=high
    3. **long_running**: 作成から N 日超経過かつ未完了 → severity=medium
    4. **high_priority_stale**: 高優先度(priority.id<=2) かつ 7 日超未更新 → severity=high
  - 出力: `BlockersResult` 構造体
- [ ] `internal/analysis/blockers_test.go` — TDD テスト

**テストケース詳細**:
```
TestBlockerDetector_Detect_overdue               — 期限超過課題を検出
TestBlockerDetector_Detect_noAssignee            — 担当者未設定を検出
TestBlockerDetector_Detect_longRunning           — 長期化課題を検出
TestBlockerDetector_Detect_highPriorityStale     — 高優先度停滞を検出
TestBlockerDetector_Detect_mixedBlockers         — 複数種類の blocker が混在
TestBlockerDetector_Detect_noBlockers            — ブロッカーなし→空結果
TestBlockerDetector_Detect_closedIssuesExcluded  — 完了課題は対象外
TestBlockerDetector_Detect_severityAssignment    — 各ルールの Severity が正しい
```

### M25: Blockers CLI + MCP + WorkloadAnalyzer
**依存**: M24, M23
**所要時間**: 1.5h
**完了条件**: `lv analyze blockers` CLI/MCP 動作 + `WorkloadAnalyzer.Analyze()` テスト全パス

- [ ] `internal/cli/analyze.go` — `BlockersCmd` 追加
- [ ] `internal/mcp/tools_analysis.go` — `logvalet_analyze_blockers` 追加
- [ ] `internal/analysis/workload.go` — `WorkloadAnalyzer`
  - 入力: project_key (optional), user_ids or team_id
  - ロジック: `ListIssues` (assignee 別) → グループ集計
  - 指標: total, by_status, overdue_count, due_this_week, avg_age_days
  - load_level: normal (<= avg*1.2), busy (<= avg*1.5), overloaded (> avg*1.5)
  - 出力: `WorkloadResult` 構造体
- [ ] `internal/analysis/workload_test.go` — TDD テスト

**CLI コマンド (blockers)**:
```
lv analyze blockers -k PROJ [--long-running-days 30]
```

**MCP tool**:
```
logvalet_analyze_blockers { "project_key": "PROJ", "long_running_days": 30 }
```

**テストケース (workload)**:
```
TestWorkloadAnalyzer_Analyze_singleUser          — 1ユーザーの負荷集計
TestWorkloadAnalyzer_Analyze_multipleUsers       — 複数ユーザー比較
TestWorkloadAnalyzer_Analyze_withTeam            — チーム指定→メンバー展開
TestWorkloadAnalyzer_Analyze_overdueCount        — 期限超過カウント
TestWorkloadAnalyzer_Analyze_dueThisWeek         — 今週期限カウント
TestWorkloadAnalyzer_Analyze_emptyAssignment     — 課題なし→ゼロ値
TestWorkloadAnalyzer_Analyze_loadLevel           — load_level 閾値判定
```

### M26: Workload CLI + MCP + ProjectHealth 統合
**依存**: M25
**所要時間**: 1.5h
**完了条件**: `lv analyze workload` と `lv analyze health` が動作。Phase 1 完了。

- [ ] `internal/cli/analyze.go` — `WorkloadCmd` + `HealthCmd` 追加
- [ ] `internal/mcp/tools_analysis.go` — `logvalet_analyze_workload` + `logvalet_analyze_project_health` 追加
- [ ] `internal/analysis/project_health.go` — `ProjectHealthAnalyzer`
  - 入力: project_key
  - 内部で StaleIssueDetector + BlockerDetector + WorkloadAnalyzer を合成
  - 出力: `ProjectHealthResult`（統合スコア + 各分析の要約）
- [ ] `internal/analysis/project_health_test.go` — TDD テスト

**CLI コマンド (workload)**:
```
lv analyze workload -k PROJ [--user me] [--team "開発チーム"]
```

**CLI コマンド (health)**:
```
lv analyze health -k PROJ
```

**MCP tools**:
```
logvalet_analyze_workload { "project_key": "PROJ", "user": "me" }
logvalet_analyze_project_health { "project_key": "PROJ" }
```

---

## Phase 2: AI ワークフロー層 (M27–M31)

LLM 連携を段階的に導入。ただし LLM 非依存の deterministic 部分を先に実装。

### M27: DraftComment 基盤（テンプレートベース）
**依存**: M20
**所要時間**: 1.5h
**完了条件**: テンプレートベースのコメント草案生成が動作

- [ ] `internal/workflow/workflow.go` — 共通型（WorkflowEnvelope 等）
- [ ] `internal/workflow/draft_comment.go` — `DraftCommentBuilder`
  - 入力: issue_key, template (progress_report | review_request | escalation | custom)
  - 課題コンテキストを取得 → テンプレートに流し込み → 草案生成
  - LLM 不要（テンプレートベース）
- [ ] `internal/workflow/draft_comment_test.go`
- [ ] CLI: `lv workflow draft-comment PROJ-123 --template progress_report`
- [ ] MCP: `logvalet_workflow_draft_comment`

### M28: IssueTriage
**依存**: M24 (BlockerDetector), M22 (StaleIssueDetector)
**所要時間**: 1.5h
**完了条件**: 課題仕分けロジックが動作

- [ ] `internal/workflow/triage.go` — `IssueTriageBuilder`
  - 入力: project_key
  - ロジック: 課題取得 → ルールベース分類 → カテゴリ付け
  - カテゴリ: urgent, needs_attention, on_track, stale, unassigned
  - 出力: `TriageResult` 構造体
- [ ] `internal/workflow/triage_test.go`
- [ ] CLI: `lv workflow triage -k PROJ`
- [ ] MCP: `logvalet_workflow_triage`

### M29: SpecToIssues
**依存**: M20
**所要時間**: 1.5h
**完了条件**: Markdown 仕様書から課題候補を抽出

- [ ] `internal/workflow/spec_to_issues.go` — `SpecToIssuesBuilder`
  - 入力: markdown_text (stdin or --file), project_key
  - ロジック: Markdown ヘッダー解析 → 課題候補リスト生成（deterministic）
  - 出力: `SpecIssuesResult`（CreateIssueRequest 互換の課題候補リスト + --dry-run）
- [ ] `internal/workflow/spec_to_issues_test.go`
- [ ] CLI: `lv workflow spec-to-issues -k PROJ --file spec.md [--dry-run]`
- [ ] MCP: `logvalet_workflow_spec_to_issues`

### M30: Weekly/Daily Digest
**依存**: M26 (ProjectHealth)
**所要時間**: 1.5h
**完了条件**: 期間指定 digest + analysis 統合が動作

- [ ] `internal/workflow/periodic_digest.go` — `PeriodicDigestBuilder`
  - 入力: project_key, period (daily | weekly), user/team scope
  - 内部: UnifiedDigestBuilder + ProjectHealthAnalyzer を合成
  - 出力: `PeriodicDigestResult`（digest + health_summary + highlights）
- [ ] `internal/workflow/periodic_digest_test.go`
- [ ] CLI: `lv workflow digest --period weekly -k PROJ`
- [ ] MCP: `logvalet_workflow_periodic_digest`

### M31: Workflow CLI/MCP 統合 + Phase 2 完了
**依存**: M27-M30
**所要時間**: 1h
**完了条件**: `lv workflow` サブコマンド群が全て動作。E2E テスト（参照系）パス。

- [ ] `internal/cli/workflow.go` — `WorkflowCmd` ルート統合
- [ ] `internal/cli/root.go` — `Workflow WorkflowCmd` 追加
- [ ] `internal/mcp/server.go` — `RegisterWorkflowTools(reg)` 追加
- [ ] `internal/mcp/tools_workflow.go` — workflow MCP tools 集約
- [ ] 参照系 E2E テスト

---

## Phase 3: Intelligence 層 (M32–M36)

高度な分析・検出機能。deterministic 実装を優先。

### M32: DecisionLogExtractor
**依存**: M20
**所要時間**: 1.5h
**完了条件**: コメント履歴から意思決定ログを抽出

- [ ] `internal/intelligence/intelligence.go` — 共通型
- [ ] `internal/intelligence/decision_log.go` — `DecisionLogExtractor`
  - 入力: issue_key or project_key
  - ロジック: コメント解析 → キーワード検出（「決定」「合意」「承認」「却下」等）→ 構造化
  - 出力: `DecisionLogResult`（時系列の意思決定リスト）
- [ ] `internal/intelligence/decision_log_test.go`
- [ ] CLI: `lv intel decision-log PROJ-123`
- [ ] MCP: `logvalet_intel_decision_log`

### M33: ActivityIntelligence + AnomalyDetection
**依存**: M20
**所要時間**: 1.5h
**完了条件**: アクティビティパターン分析と異常検知が動作

- [ ] `internal/intelligence/activity_intel.go` — `ActivityIntelligenceAnalyzer`
  - 入力: project_key, period
  - ロジック: アクティビティ頻度分析 → 曜日/時間帯パターン → 急増/急減検出
  - 統計: 標準偏差ベースの異常検知（deterministic）
- [ ] `internal/intelligence/activity_intel_test.go`
- [ ] CLI: `lv intel activity -k PROJ --since this-month`
- [ ] MCP: `logvalet_intel_activity`

### M34: RiskSummary
**依存**: M24 (BlockerDetector), M22 (StaleIssueDetector), M33
**所要時間**: 1.5h
**完了条件**: プロジェクトリスクサマリーが動作

- [ ] `internal/intelligence/risk.go` — `RiskSummaryBuilder`
  - 入力: project_key
  - 内部: Blockers + Stale + ActivityIntel を合成 → リスクスコア算出
  - 出力: `RiskSummaryResult`（risk_score 0-100, risk_factors, trend）
- [ ] `internal/intelligence/risk_test.go`
- [ ] CLI: `lv intel risk -k PROJ`
- [ ] MCP: `logvalet_intel_risk_summary`

### M35: RoadmapAssistance
**依存**: M26
**所要時間**: 1.5h
**完了条件**: マイルストーン進捗・ロードマップ支援が動作

- [ ] `internal/intelligence/roadmap.go` — `RoadmapAssistant`
  - 入力: project_key
  - ロジック: Version (milestone) + Issue 紐付け → 進捗率・遅延検出
  - 出力: `RoadmapResult`（milestone_progress, delays, projected_completion）
- [ ] `internal/intelligence/roadmap_test.go`
- [ ] CLI: `lv intel roadmap -k PROJ`
- [ ] MCP: `logvalet_intel_roadmap`

### M36: Intelligence CLI/MCP 統合 + Phase 3 完了
**依存**: M32-M35
**所要時間**: 1h
**完了条件**: `lv intel` サブコマンド群が全て動作。

- [ ] `internal/cli/intel.go` — `IntelCmd` ルート統合
- [ ] `internal/cli/root.go` — `Intel IntelCmd` 追加
- [ ] `internal/mcp/server.go` — `RegisterIntelligenceTools(reg)` 追加
- [ ] `internal/mcp/tools_intelligence.go` — intel MCP tools 集約
- [ ] 参照系 E2E テスト

---

## 依存関係グラフ

```
M20 ─────┬──── M21 (issue context CLI/MCP)
         │
         ├──── M22 ──── M23 (stale CLI/MCP)
         │               │
         ├──── M24 ──── M25 ──── M26 (workload CLI/MCP + health)
         │               │        │
         │               │        ├──── M30 (periodic digest)
         │               │        └──── M35 (roadmap)
         │               │
         │               ├──── M28 (triage)
         │               │
         │               └──── M34 (risk)
         │
         ├──── M27 (draft comment)
         ├──── M29 (spec to issues)
         ├──── M32 (decision log)
         └──── M33 ──── M34 (activity intel → risk)

M23 + M25 ──── M26
M27-M30 ─────── M31 (workflow 統合)
M32-M35 ─────── M36 (intel 統合)
```

---

## JSON スキーマ設計

### 1. IssueContext (`analysis/issue_context`)

```json
{
  "schema_version": "1",
  "resource": "analysis/issue_context",
  "generated_at": "2026-04-01T12:00:00Z",
  "profile": "work",
  "space": "heptagon",
  "base_url": "https://heptagon.backlog.com",
  "warnings": [],
  "analysis": {
    "issue": {
      "id": 12345,
      "issue_key": "PROJ-123",
      "summary": "ログイン画面のレスポンス改善",
      "description": "...",
      "status": { "id": 2, "name": "処理中" },
      "priority": { "id": 2, "name": "高" },
      "issue_type": { "id": 1, "name": "タスク" },
      "assignee": { "id": 42, "name": "田中太郎" },
      "reporter": { "id": 10, "name": "鈴木花子" },
      "due_date": "2026-04-15",
      "start_date": "2026-03-20",
      "created": "2026-03-15T09:00:00Z",
      "updated": "2026-03-28T14:30:00Z",
      "categories": [{ "id": 1, "name": "バックエンド" }],
      "milestones": [{ "id": 5, "name": "v2.0" }],
      "url": "https://heptagon.backlog.com/view/PROJ-123"
    },
    "comments": [
      {
        "id": 100,
        "content": "初回レビュー完了。パフォーマンステスト追加が必要。",
        "author": { "id": 10, "name": "鈴木花子" },
        "created": "2026-03-20T10:00:00Z"
      }
    ],
    "comment_count": 5,
    "project_statuses": [
      { "id": 1, "name": "未対応" },
      { "id": 2, "name": "処理中" },
      { "id": 3, "name": "処理済み" },
      { "id": 4, "name": "完了" }
    ],
    "age_days": 17,
    "days_since_update": 4,
    "days_until_due": 14,
    "is_overdue": false,
    "llm_hints": {
      "findings": [
        "課題は処理中ステータスで担当者がアサインされている",
        "期限まで14日の余裕がある",
        "直近4日間更新がない"
      ],
      "recommended_actions": [
        "担当者に進捗確認を推奨",
        "パフォーマンステストの追加が未対応の可能性"
      ],
      "risk_indicators": []
    }
  }
}
```

### 2. StaleIssues (`analysis/stale_issues`)

```json
{
  "schema_version": "1",
  "resource": "analysis/stale_issues",
  "generated_at": "2026-04-01T12:00:00Z",
  "profile": "work",
  "space": "heptagon",
  "base_url": "https://heptagon.backlog.com",
  "warnings": [],
  "analysis": {
    "scope": {
      "project_key": "PROJ",
      "stale_days": 14,
      "excluded_statuses": ["完了", "Close"],
      "checked_at": "2026-04-01T12:00:00Z"
    },
    "summary": {
      "total_issues_checked": 45,
      "stale_count": 8,
      "stale_percentage": 17.8,
      "by_status": {
        "未対応": 3,
        "処理中": 5
      },
      "by_assignee": {
        "田中太郎": 4,
        "unassigned": 2,
        "鈴木花子": 2
      }
    },
    "stale_issues": [
      {
        "issue_key": "PROJ-45",
        "summary": "API ドキュメントの更新",
        "status": "処理中",
        "assignee": { "id": 42, "name": "田中太郎" },
        "priority": "中",
        "last_updated": "2026-03-10T08:00:00Z",
        "days_stale": 22,
        "due_date": "2026-03-25",
        "is_overdue": true,
        "severity": "high",
        "url": "https://heptagon.backlog.com/view/PROJ-45"
      }
    ],
    "llm_hints": {
      "findings": [
        "8件の課題が14日以上更新されていない（全体の17.8%）",
        "田中太郎に停滞課題が集中している（4件）",
        "未アサインの停滞課題が2件ある"
      ],
      "recommended_actions": [
        "田中太郎の負荷を確認し、必要に応じてリアサイン",
        "未アサイン課題に担当者を設定",
        "期限超過の停滞課題を優先対応"
      ],
      "risk_indicators": [
        "停滞率が15%を超えている"
      ]
    }
  }
}
```

### 3. Blockers (`analysis/blockers`)

```json
{
  "schema_version": "1",
  "resource": "analysis/blockers",
  "generated_at": "2026-04-01T12:00:00Z",
  "profile": "work",
  "space": "heptagon",
  "base_url": "https://heptagon.backlog.com",
  "warnings": [],
  "analysis": {
    "scope": {
      "project_key": "PROJ",
      "long_running_days": 30,
      "checked_at": "2026-04-01T12:00:00Z"
    },
    "summary": {
      "total_blockers": 6,
      "by_type": {
        "overdue": 2,
        "no_assignee": 1,
        "long_running": 2,
        "high_priority_stale": 1
      },
      "by_severity": {
        "critical": 2,
        "high": 3,
        "medium": 1
      }
    },
    "blockers": [
      {
        "issue_key": "PROJ-12",
        "summary": "認証基盤のリファクタリング",
        "type": "overdue",
        "severity": "critical",
        "status": "処理中",
        "assignee": { "id": 42, "name": "田中太郎" },
        "priority": "高",
        "detail": {
          "due_date": "2026-03-20",
          "days_overdue": 12
        },
        "url": "https://heptagon.backlog.com/view/PROJ-12"
      },
      {
        "issue_key": "PROJ-30",
        "summary": "エラーハンドリング改善",
        "type": "no_assignee",
        "severity": "high",
        "status": "未対応",
        "assignee": null,
        "priority": "中",
        "detail": {
          "created": "2026-02-15T10:00:00Z",
          "age_days": 45
        },
        "url": "https://heptagon.backlog.com/view/PROJ-30"
      }
    ],
    "llm_hints": {
      "findings": [
        "6件の進行阻害要因を検出",
        "期限超過が2件（うち1件は12日超過でcritical）",
        "担当者未設定の課題が1件"
      ],
      "recommended_actions": [
        "PROJ-12 の期限超過（12日）を最優先で対応",
        "PROJ-30 に担当者をアサイン",
        "長期化課題のスコープ見直しを検討"
      ],
      "risk_indicators": [
        "critical レベルのブロッカーが2件存在",
        "担当者未設定の課題がある"
      ]
    }
  }
}
```

### 4. Workload (`analysis/workload`)

```json
{
  "schema_version": "1",
  "resource": "analysis/workload",
  "generated_at": "2026-04-01T12:00:00Z",
  "profile": "work",
  "space": "heptagon",
  "base_url": "https://heptagon.backlog.com",
  "warnings": [],
  "analysis": {
    "scope": {
      "project_key": "PROJ",
      "users": [
        { "id": 42, "name": "田中太郎" },
        { "id": 10, "name": "鈴木花子" }
      ],
      "checked_at": "2026-04-01T12:00:00Z"
    },
    "summary": {
      "total_users": 2,
      "total_open_issues": 25,
      "avg_issues_per_user": 12.5,
      "max_issues_user": { "id": 42, "name": "田中太郎" },
      "max_issues_count": 18,
      "total_overdue": 4
    },
    "users": [
      {
        "user": { "id": 42, "name": "田中太郎" },
        "total_issues": 18,
        "by_status": {
          "未対応": 5,
          "処理中": 10,
          "処理済み": 3
        },
        "overdue_count": 3,
        "due_this_week": 4,
        "avg_age_days": 21.5,
        "oldest_issue": {
          "issue_key": "PROJ-12",
          "summary": "認証基盤のリファクタリング",
          "age_days": 45
        },
        "load_level": "overloaded"
      },
      {
        "user": { "id": 10, "name": "鈴木花子" },
        "total_issues": 7,
        "by_status": {
          "未対応": 2,
          "処理中": 4,
          "処理済み": 1
        },
        "overdue_count": 1,
        "due_this_week": 2,
        "avg_age_days": 12.3,
        "oldest_issue": {
          "issue_key": "PROJ-55",
          "summary": "テストカバレッジ改善",
          "age_days": 28
        },
        "load_level": "normal"
      }
    ],
    "unassigned": {
      "total_issues": 3,
      "by_status": {
        "未対応": 3
      }
    },
    "llm_hints": {
      "findings": [
        "田中太郎の負荷が突出（18件、平均12.5件の1.4倍）",
        "田中太郎に期限超過が3件集中",
        "未アサイン課題が3件ある"
      ],
      "recommended_actions": [
        "田中太郎から鈴木花子への課題移管を検討",
        "未アサイン課題3件に担当者を設定",
        "今週期限の課題（計6件）の進捗確認"
      ],
      "risk_indicators": [
        "負荷偏りが1.4倍を超えている",
        "期限超過課題が計4件"
      ]
    }
  }
}
```

---

## M20 詳細計画（テストファースト設計）

### ファイル一覧

| ファイル | 説明 | 行数目安 |
|---------|------|---------|
| `internal/analysis/analysis.go` | 共通型定義 | 80-100 |
| `internal/analysis/analysis_test.go` | 共通型テスト（envelope 生成） | 40-50 |
| `internal/analysis/context.go` | IssueContextBuilder | 120-150 |
| `internal/analysis/context_test.go` | IssueContext TDD テスト | 200-250 |

### 実装手順（TDD サイクル）

#### Step 1: 共通型定義 (Red → Green)

1. `analysis_test.go` を先に書く
   - `TestNewEnvelope_shape` — envelope の全フィールド検証
   - `TestSeverity_values` — Severity 定数値検証
2. `analysis.go` を実装
   - `AnalysisEnvelope`, `Severity`, `AnalysisLLMHints`, `BaseAnalysisBuilder`
   - `newEnvelope()` ヘルパー

#### Step 2: IssueContextBuilder (Red → Green → Refactor)

1. `context_test.go` を先に書く（5テストケース）
2. `context.go` を実装
   - `IssueContextBuilder` struct
   - `NewIssueContextBuilder()` コンストラクタ
   - `Build(ctx, issueKey, opts)` メソッド
3. Refactor: errgroup 並行化、ヘルパー抽出

### テストコード設計（context_test.go）

```go
// compile-time interface check (Builder は interface ではないが型確認)
func TestIssueContextBuilder_Build_success(t *testing.T) {
    // Setup: MockClient with GetIssue, ListIssueComments, ListProjectStatuses
    // Action: builder.Build(ctx, "PROJ-123", IssueContextOpts{CommentLimit: 20})
    // Verify: envelope.Resource == "analysis/issue_context"
    // Verify: analysis.Issue.IssueKey == "PROJ-123"
    // Verify: analysis.Comments has expected count
    // Verify: analysis.AgeDays > 0
    // Verify: analysis.DaysSinceUpdate >= 0
}

func TestIssueContextBuilder_Build_withComments(t *testing.T) {
    // Setup: Issue with 3 comments
    // Verify: analysis.Comments length == 3
    // Verify: analysis.CommentCount == 3
}

func TestIssueContextBuilder_Build_commentsFail(t *testing.T) {
    // Setup: GetIssue succeeds, ListIssueComments returns error
    // Verify: envelope.Warnings has "comments_fetch_failed"
    // Verify: analysis.Comments == [] (empty, not nil)
    // Verify: analysis.Issue is still populated
}

func TestIssueContextBuilder_Build_issueNotFound(t *testing.T) {
    // Setup: GetIssue returns ErrNotFound
    // Verify: Build returns error (not envelope with warning)
}

func TestIssueContextBuilder_Build_envelopeShape(t *testing.T) {
    // JSON marshal → unmarshal roundtrip
    // Verify all top-level fields present
}
```

### IssueContext Go 型定義

```go
type IssueContextOpts struct {
    CommentLimit int  // default 20
}

type IssueContext struct {
    Issue           IssueDetail       `json:"issue"`
    Comments        []ContextComment  `json:"comments"`
    CommentCount    int               `json:"comment_count"`
    ProjectStatuses []domain.Status   `json:"project_statuses"`
    AgeDays         int               `json:"age_days"`
    DaysSinceUpdate int               `json:"days_since_update"`
    DaysUntilDue    *int              `json:"days_until_due"`
    IsOverdue       bool              `json:"is_overdue"`
    LLMHints        AnalysisLLMHints  `json:"llm_hints"`
}

type IssueDetail struct {
    ID          int              `json:"id"`
    IssueKey    string           `json:"issue_key"`
    Summary     string           `json:"summary"`
    Description string           `json:"description"`
    Status      *domain.IDName   `json:"status,omitempty"`
    Priority    *domain.IDName   `json:"priority,omitempty"`
    IssueType   *domain.IDName   `json:"issue_type,omitempty"`
    Assignee    *domain.UserRef  `json:"assignee,omitempty"`
    Reporter    *domain.UserRef  `json:"reporter,omitempty"`
    DueDate     *string          `json:"due_date,omitempty"`
    StartDate   *string          `json:"start_date,omitempty"`
    Created     *time.Time       `json:"created,omitempty"`
    Updated     *time.Time       `json:"updated,omitempty"`
    Categories  []domain.IDName  `json:"categories"`
    Milestones  []domain.IDName  `json:"milestones"`
    URL         string           `json:"url"`
}

type ContextComment struct {
    ID      int64           `json:"id"`
    Content string          `json:"content"`
    Author  *domain.UserRef `json:"author,omitempty"`
    Created *time.Time      `json:"created,omitempty"`
}
```

---

## コマンド体系まとめ

### Phase 1 CLI

| コマンド | 説明 |
|---------|------|
| `lv issue context <key>` | 課題の総合コンテキスト |
| `lv analyze stale-issues -k <key>` | 停滞課題検出 |
| `lv analyze blockers -k <key>` | 進行阻害要因抽出 |
| `lv analyze workload -k <key>` | 担当者負荷分析 |
| `lv analyze health -k <key>` | プロジェクト健全性 |

### Phase 2 CLI

| コマンド | 説明 |
|---------|------|
| `lv workflow draft-comment <key>` | コメント草案生成 |
| `lv workflow triage -k <key>` | 課題仕分け |
| `lv workflow spec-to-issues -k <key>` | 仕様→課題変換 |
| `lv workflow digest --period weekly` | 定期ダイジェスト |

### Phase 3 CLI

| コマンド | 説明 |
|---------|------|
| `lv intel decision-log <key>` | 意思決定ログ抽出 |
| `lv intel activity -k <key>` | アクティビティ分析 |
| `lv intel risk -k <key>` | リスクサマリー |
| `lv intel roadmap -k <key>` | ロードマップ支援 |

### MCP Tools 一覧

| Phase | Tool Name | 説明 |
|-------|-----------|------|
| 1 | `logvalet_issue_context` | 課題コンテキスト |
| 1 | `logvalet_analyze_stale_issues` | 停滞課題検出 |
| 1 | `logvalet_analyze_blockers` | ブロッカー検出 |
| 1 | `logvalet_analyze_workload` | 負荷分析 |
| 1 | `logvalet_analyze_project_health` | 健全性分析 |
| 2 | `logvalet_workflow_draft_comment` | コメント草案 |
| 2 | `logvalet_workflow_triage` | 課題仕分け |
| 2 | `logvalet_workflow_spec_to_issues` | 仕様→課題 |
| 2 | `logvalet_workflow_periodic_digest` | 定期ダイジェスト |
| 3 | `logvalet_intel_decision_log` | 意思決定ログ |
| 3 | `logvalet_intel_activity` | アクティビティ分析 |
| 3 | `logvalet_intel_risk_summary` | リスクサマリー |
| 3 | `logvalet_intel_roadmap` | ロードマップ |

---

## Architecture Decisions

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 7 | `analysis/` を `digest/` と分離 | digest は要約（what）、analysis は洞察（so what）。責務が異なる | 2026-04-01 |
| 8 | `AnalysisEnvelope` を `DigestEnvelope` と同構造にする | LLM が同じパターンで処理可能。schema_version で互換性管理 | 2026-04-01 |
| 9 | `now func() time.Time` で clock injection | stale/overdue 判定のテスタビリティ確保 | 2026-04-01 |
| 10 | Phase 1 は全て deterministic | LLM 依存は信頼性・テスト容易性の観点から Phase 2 以降に先送り | 2026-04-01 |
| 11 | `lv analyze` / `lv workflow` / `lv intel` の3コマンドツリー | Phase ごとに明確な名前空間。1コマンド1意思決定の原則に合致 | 2026-04-01 |
| 12 | `load_level` は閾値ベースの deterministic 判定 | normal/busy/overloaded を課題数の偏差で判定。LLM 不要 | 2026-04-01 |
| 13 | Severity は 5 段階固定 (critical/high/medium/low/info) | blocker/risk の深刻度を統一的に表現 | 2026-04-01 |

## Blockers
なし

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-01 | 作成 | roadmap v3 作成。Phase 1-3 AI ネイティブ機能の M20-M36 を設計 |
