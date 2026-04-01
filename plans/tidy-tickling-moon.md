# logvalet Phase 1〜3 ロードマップ + M20 詳細計画

## Context

logvalet v0.5.0 は基本 CRUD + Digest + MCP (M01-M18) が完備済み。
`docs/specs/logvalet_roadmap_instruction.md` に定義された Phase 1〜3 の AI ネイティブ機能を実装し、
Backlog を Linear に遜色ない AI 体験へ引き上げる。

**方針**: deterministic な分析を優先し、LLM 依存機能は後回し。TDD 必須。参照系 E2E テスト OK。

---

## 1. アーキテクチャ設計

### 新パッケージ: `internal/analysis/`

`digest/`（what: 何があるか）とは別に `analysis/`（so what: だから何か）を新設。

```
internal/analysis/
  analysis.go         — 共通型（AnalysisEnvelope, BaseAnalysisBuilder, AnalysisConfig）
  context.go          — IssueContextBuilder
  context_test.go
  stale.go            — StaleIssueDetector
  stale_test.go
  blocker.go          — BlockerDetector
  blocker_test.go
  workload.go         — WorkloadCalculator
  workload_test.go
```

### 共通型

```go
// AnalysisEnvelope は DigestEnvelope と同構造（LLM が同パターンで処理可能）
type AnalysisEnvelope struct {
    SchemaVersion string           `json:"schema_version"`
    Resource      string           `json:"resource"`      // "issue_context", "stale_issues", etc.
    GeneratedAt   time.Time        `json:"generated_at"`
    Profile       string           `json:"profile"`
    Space         string           `json:"space"`
    BaseURL       string           `json:"base_url"`
    Warnings      []domain.Warning `json:"warnings"`
    Analysis      any              `json:"analysis"`
}

// BaseAnalysisBuilder — digest.BaseDigestBuilder を踏襲 + clock injection
type BaseAnalysisBuilder struct {
    client  backlog.Client
    profile string
    space   string
    baseURL string
    now     func() time.Time  // テスタビリティ用
}
```

### コマンド配置

既存サブコマンドの下に配置（新トップレベルコマンド `analyze` は作らない）:

| 機能 | コマンド | 理由 |
|------|---------|------|
| Issue Context | `logvalet issue context PROJ-123` | issue の分析→issue 配下 |
| Stale Issues | `logvalet issue stale -k PROJECT` | issue のフィルタリング→issue 配下 |
| Project Blockers | `logvalet project blockers PROJECT` | project の分析→project 配下 |
| User Workload | `logvalet user workload [USER]` | user の分析→user 配下 |
| Project Health | `logvalet project health PROJECT` | Phase 1 統合→project 配下 |
| Draft Comment | `logvalet issue draft-comment PROJ-123` | issue のアクション→issue 配下 |
| Issue Triage | `logvalet issue triage PROJ-123` | issue のアクション→issue 配下 |
| Weekly Digest | `logvalet digest weekly -k PROJECT` | 既存 digest の拡張 |
| Daily Digest | `logvalet digest daily -k PROJECT` | 既存 digest の拡張 |

---

## 2. ロードマップ v3: マイルストーン一覧

### Phase 1: AI ネイティブ操作層 (M20-M32)

| M# | タイトル | 依存 | 概要 |
|----|---------|------|------|
| **M20** | analysis 基盤 + IssueContext ロジック | — | 共通型、BaseAnalysisBuilder、IssueContextBuilder |
| **M21** | IssueContext CLI コマンド | M20 | `issue context` コマンド + フラグ + help |
| **M22** | IssueContext MCP ツール | M21 | `logvalet_issue_context` MCP tool |
| **M23** | StaleIssueDetector ロジック | M20 | 停滞検出アルゴリズム（デフォルト7日）+ 閾値設定 |
| **M24** | Stale Issues CLI + MCP | M23 | `issue stale` コマンド + MCP tool + help |
| **M25** | BlockerDetector ロジック | M20 | 進行阻害要因検出アルゴリズム |
| **M26** | Project Blockers CLI + MCP | M25 | `project blockers` コマンド + MCP tool + help |
| **M27** | WorkloadCalculator ロジック | M20 | 担当者負荷計算 |
| **M28** | User Workload CLI + MCP | M27 | `user workload` コマンド + MCP tool + help |
| **M29** | Enhanced Project Digest | M23,M25,M27 | 既存 project digest に stale/blocker/workload 統合 |
| **M30** | Project Health CLI + MCP | M29 | `project health` 統合ビュー |
| **M31** | Phase 1 スキル・ドキュメント整備 | M30 | README 更新、既存スキル更新、新スキル作成 |
| **M32** | Phase 1 E2E テスト + リリース | M31 | 参照系 E2E テスト、Phase 1 完了検証 |

#### M31 スキル・ドキュメント詳細

**新規スキル作成:**
| スキル | トリガー | 内容 |
|--------|---------|------|
| `logvalet-health` | "プロジェクトの状態", "project health", "ブロッカー", "停滞" | `issue stale` + `project blockers` + `project health` を組み合わせた分析ワークフロー |
| `logvalet-context` | "課題の詳細", "issue context", "コンテキスト" | `issue context` の使い方ガイド + 活用パターン |

**既存スキル更新:**
| スキル | 変更内容 |
|--------|---------|
| `logvalet` (SKILL.md) | issue context, issue stale, project blockers, user workload, project health コマンドを追加 |
| `logvalet-my-week` | stale/overdue signals を活用した追加情報表示 |
| `logvalet-my-next` | workload 情報の参照を追加 |
| `logvalet-report` | project health/blockers データの統合 |

**ドキュメント更新:**
| ファイル | 変更内容 |
|---------|---------|
| `README.md` | Phase 1 新コマンド一覧、使用例、AI 活用シナリオ |
| `README.ja.md` | 同上（日本語版） |

> **注意**: `docs/specs/` は初期構想として保存。編集しない。

### Phase 2: AI ワークフロー層 (M33-M41)

| M# | タイトル | 依存 | 概要 |
|----|---------|------|------|
| **M33** | DraftComment テンプレートエンジン | M20 | intent ベースのコメント構造化テンプレート |
| **M34** | DraftComment CLI + MCP | M33 | `issue draft-comment` コマンド + MCP tool + help |
| **M35** | IssueTriage 判定ロジック | M20 | priority/assignee/category の suggestion |
| **M36** | IssueTriage CLI + MCP | M35 | `issue triage` コマンド + MCP tool + help |
| **M37** | Weekly/Daily Digest ロジック | M29 | 期間ベース集約 + completed/started/blocked |
| **M38** | Weekly/Daily Digest CLI + MCP | M37 | `digest weekly/daily` コマンド + MCP tool + help |
| **M39** | SpecToIssues | M20 | `spec-to-issues --file spec.md --preview` |
| **M40** | Phase 2 スキル・ドキュメント整備 | M39 | 新スキル作成、既存スキル更新、README 更新 |
| **M41** | Phase 2 E2E テスト + リリース | M40 | Phase 2 完了検証 |

#### M40 スキル・ドキュメント詳細

**新規スキル作成:**
| スキル | トリガー | 内容 |
|--------|---------|------|
| `logvalet-triage` | "課題の仕分け", "triage", "振り分け" | issue triage ワークフロー |
| `logvalet-digest-periodic` | "週次レポート", "daily digest", "今週のまとめ" | weekly/daily digest 生成ガイド |

**既存スキル更新:**
| スキル | 変更内容 |
|--------|---------|
| `logvalet` | draft-comment, triage, weekly/daily digest コマンドを追加 |
| `logvalet-issue-create` | triage 情報の活用を追加 |

### Phase 3: Intelligence / 差別化層 (M42-M49)

| M# | タイトル | 依存 | 概要 |
|----|---------|------|------|
| **M42** | DecisionLog 抽出ロジック | M20 | コメント・更新履歴から意思決定抽出 |
| **M43** | DecisionLog CLI + MCP | M42 | `issue decisions` / `project decisions` + help |
| **M44** | ActivityIntelligence | M20 | 偏り・異常・停滞検出 |
| **M45** | ActivityIntelligence CLI + MCP | M44 | `activity intelligence` + help |
| **M46** | RiskSummary | M23,M25,M27 | overdue+blocker+stale+imbalance 統合リスク |
| **M47** | RiskSummary CLI + MCP | M46 | `project risk` + help |
| **M48** | Phase 3 スキル・ドキュメント整備 | M47 | 新スキル作成、全スキル最終更新、README 完全更新 |
| **M49** | Phase 3 E2E テスト + 最終リリース | M48 | 全 Phase 完了検証 |

#### M48 スキル・ドキュメント詳細

**新規スキル作成:**
| スキル | トリガー | 内容 |
|--------|---------|------|
| `logvalet-intelligence` | "リスク分析", "プロジェクトリスク", "異常検出" | risk summary + activity intelligence ワークフロー |

**最終ドキュメント更新:**
| ファイル | 変更内容 |
|---------|---------|
| `skills/logvalet/SKILL.md` | Phase 3 全コマンド反映、AI 活用ベストプラクティス |
| `README.md` / `README.ja.md` | Phase 1-3 全機能、アーキテクチャ図、活用シナリオ |

> **注意**: `docs/specs/` は初期構想として保存。編集しない。

---

## 3. Phase 1 詳細設計

### M20: analysis 基盤 + IssueContext ロジック

**目的**: analysis パッケージの共通基盤と、最初の分析機能 IssueContext のロジックを実装。

#### 新規ファイル

| ファイル | 内容 |
|---------|------|
| `internal/analysis/analysis.go` | AnalysisEnvelope, BaseAnalysisBuilder, NewBaseAnalysisBuilder |
| `internal/analysis/analysis_test.go` | envelope 生成テスト、JSON shape テスト |
| `internal/analysis/context.go` | IssueContextBuilder, IssueContext 型, Build() |
| `internal/analysis/context_test.go` | MockClient ベースの全ケーステスト |

#### IssueContext レスポンス構造

```go
type IssueContext struct {
    Issue         IssueSnapshot        `json:"issue"`
    Meta          ContextMeta          `json:"meta"`
    RecentComments []ContextComment    `json:"recent_comments"`
    RecentUpdates  []ContextActivity   `json:"recent_updates"`
    Signals       IssueSignals         `json:"signals"`
    LLMHints      digest.DigestLLMHints `json:"llm_hints"`
}

type IssueSnapshot struct {
    IssueKey    string           `json:"issue_key"`
    Summary     string           `json:"summary"`
    Description string           `json:"description,omitempty"`
    Status      *domain.IDName   `json:"status,omitempty"`
    Priority    *domain.IDName   `json:"priority,omitempty"`
    IssueType   *domain.IDName   `json:"issue_type,omitempty"`
    Assignee    *domain.UserRef  `json:"assignee,omitempty"`
    Reporter    *domain.UserRef  `json:"reporter,omitempty"`
    Categories  []domain.IDName  `json:"categories"`
    Milestones  []domain.IDName  `json:"milestones"`
    DueDate     *time.Time       `json:"due_date,omitempty"`
    StartDate   *time.Time       `json:"start_date,omitempty"`
    Created     *time.Time       `json:"created,omitempty"`
    Updated     *time.Time       `json:"updated,omitempty"`
}

type ContextMeta struct {
    Statuses []domain.Status `json:"statuses"`
}

type ContextComment struct {
    ID      int64           `json:"id"`
    Content string          `json:"content"`
    Author  *domain.UserRef `json:"author,omitempty"`
    Created *time.Time      `json:"created,omitempty"`
}

type ContextActivity struct {
    ID      int64           `json:"id"`
    Type    string          `json:"type"`
    Actor   *domain.UserRef `json:"actor,omitempty"`
    Created *time.Time      `json:"created,omitempty"`
}

type IssueSignals struct {
    IsOverdue       bool `json:"is_overdue"`
    DaysSinceUpdate int  `json:"days_since_update"`
    IsStale         bool `json:"is_stale"`          // 7日以上更新なし
    HasAssignee     bool `json:"has_assignee"`
    CommentCount    int  `json:"comment_count"`
}
```

#### IssueContextBuilder API

```go
type IssueContextOptions struct {
    MaxComments int  // default: 10
    Compact     bool // true: description, comments を省略
}

type IssueContextBuilder struct {
    BaseAnalysisBuilder
}

func NewIssueContextBuilder(client backlog.Client, profile, space, baseURL string, opts ...func(*BaseAnalysisBuilder)) *IssueContextBuilder

func (b *IssueContextBuilder) Build(ctx context.Context, issueKey string, opt IssueContextOptions) (*AnalysisEnvelope, error)
```

#### TDD テストケース (context_test.go)

```
1. TestIssueContextBuilder_Build_Success
   - 正常系: issue + comments + statuses 取得、全フィールド検証
2. TestIssueContextBuilder_Build_Compact
   - compact=true: description と comment content が空
3. TestIssueContextBuilder_Build_MaxComments
   - MaxComments=3: コメント件数制限
4. TestIssueContextBuilder_Build_OverdueSignal
   - DueDate が過去 → is_overdue=true
5. TestIssueContextBuilder_Build_StaleSignal
   - Updated が7日以上前 → is_stale=true, days_since_update 正確
6. TestIssueContextBuilder_Build_IssueNotFound
   - ErrNotFound → エラー返却
7. TestIssueContextBuilder_Build_PartialFailure
   - comments 取得失敗 → warnings 付き部分結果
8. TestAnalysisEnvelope_JSONShape
   - JSON シリアライズ → schema_version, resource, analysis フィールド存在確認
```

#### 変更なしの既存ファイル

M20 は analysis パッケージのみ。CLI/MCP 変更は M21/M22。

---

### M21: IssueContext CLI コマンド

#### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/cli/issue.go` | IssueCmd に `Context IssueContextCmd` 追加 |
| `internal/cli/issue_context.go` | 新規: IssueContextCmd 定義 + Run() |
| `internal/cli/issue_context_test.go` | 新規: CLI 統合テスト |

#### コマンド定義

```go
type IssueContextCmd struct {
    IssueIDOrKey string `arg:"" required:"" help:"issue key (e.g., PROJ-123)"`
    Comments     int    `help:"max recent comments to include" default:"10"`
    Compact      bool   `help:"omit description and comment bodies"`
}
```

#### 使用例

```bash
logvalet issue context PROJ-123
logvalet issue context PROJ-123 --comments 20
logvalet issue context PROJ-123 --compact
logvalet issue context PROJ-123 -f md
```

---

### M22: IssueContext MCP ツール

#### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/mcp/tools_analysis.go` | 新規: RegisterAnalysisTools(), logvalet_issue_context |
| `internal/mcp/tools_analysis_test.go` | 新規: MCP tool テスト |
| `internal/mcp/server.go` | RegisterAnalysisTools(reg) 追加 |

---

### M23: StaleIssueDetector ロジック

#### 新規ファイル

| ファイル | 内容 |
|---------|------|
| `internal/analysis/stale.go` | StaleIssueDetector, StaleConfig, StaleIssueResult |
| `internal/analysis/stale_test.go` | 閾値別テスト、ステータス別テスト |

#### StaleIssueResult 構造

```go
type StaleConfig struct {
    DefaultDays   int            // default: 7
    StatusDays    map[string]int // status名 → 独自閾値 (e.g., "処理中": 7)
    ExcludeStatus []string       // 除外ステータス (e.g., "完了", "対応済み")
}

type StaleIssueResult struct {
    Issues       []StaleIssue `json:"issues"`
    TotalCount   int          `json:"total_count"`
    ThresholdDays int         `json:"threshold_days"`
    LLMHints     digest.DigestLLMHints `json:"llm_hints"`
}

type StaleIssue struct {
    IssueKey        string          `json:"issue_key"`
    Summary         string          `json:"summary"`
    Status          *domain.IDName  `json:"status,omitempty"`
    Assignee        *domain.UserRef `json:"assignee,omitempty"`
    DaysSinceUpdate int             `json:"days_since_update"`
    LastUpdated     *time.Time      `json:"last_updated,omitempty"`
    DueDate         *time.Time      `json:"due_date,omitempty"`
    IsOverdue       bool            `json:"is_overdue"`
}
```

---

### M24: Stale Issues CLI + MCP

```bash
logvalet issue stale -k PROJECT
logvalet issue stale -k PROJECT --days 7
logvalet issue stale -k PROJECT --exclude-status "完了,対応済み"
```

---

### M25: BlockerDetector ロジック

#### BlockerResult 構造

```go
type BlockerResult struct {
    Blockers    []Blocker `json:"blockers"`
    TotalCount  int       `json:"total_count"`
    LLMHints    digest.DigestLLMHints `json:"llm_hints"`
}

type Blocker struct {
    IssueKey    string          `json:"issue_key"`
    Summary     string          `json:"summary"`
    Signals     []string        `json:"signals"`  // ["overdue", "no_assignee", "stale_7d"]
    Severity    string          `json:"severity"` // "high", "medium", "low"
    Assignee    *domain.UserRef `json:"assignee,omitempty"`
    DueDate     *time.Time      `json:"due_date,omitempty"`
    DaysSinceUpdate int         `json:"days_since_update"`
}
```

**Blocker シグナル:**
- `overdue` — 期限超過
- `stale_{N}d` — N日以上更新なし
- `no_assignee` — 担当者未設定
- `high_priority_stale` — 高優先度で停滞
- `no_comment` — コメントゼロ

---

### M26: Project Blockers CLI + MCP

```bash
logvalet project blockers PROJECT
logvalet project blockers PROJECT --min-severity medium
```

---

### M27: WorkloadCalculator ロジック

#### Workload 構造

```go
type WorkloadResult struct {
    User        domain.UserRef `json:"user"`
    Summary     WorkloadSummary `json:"summary"`
    ByProject   []ProjectWorkload `json:"by_project"`
    ByPriority  map[string]int `json:"by_priority"`
    StaleCount  int            `json:"stale_count"`
    LLMHints    digest.DigestLLMHints `json:"llm_hints"`
}

type WorkloadSummary struct {
    OpenCount    int `json:"open_count"`
    OverdueCount int `json:"overdue_count"`
    DueSoonCount int `json:"due_soon_count"` // 3日以内
}

type ProjectWorkload struct {
    ProjectKey string `json:"project_key"`
    ProjectName string `json:"project_name"`
    OpenCount   int    `json:"open_count"`
    OverdueCount int   `json:"overdue_count"`
}
```

---

### M28: User Workload CLI + MCP

```bash
logvalet user workload           # 自分の workload
logvalet user workload 12345     # 特定ユーザー
logvalet user workload --me      # 明示的に自分
```

---

### M29-M30: Enhanced Project Digest + Project Health

M29: 既存の project digest に `stale_count`, `blocker_count`, `workload_summary` を追加。
M30: `project health PROJECT` 統合ビュー（stale + blockers + workload を一度に返す）。

---

## 4. テスト戦略

### Unit テスト（全マイルストーン）
- `backlog.MockClient` の Func フィールドパターン
- `now func() time.Time` による clock injection
- Table-driven tests
- JSON shape verification

### E2E テスト（参照系のみ）
- 実 Backlog API に対する統合テスト
- `//go:build e2e` ビルドタグで分離
- `LOGVALET_E2E=1` 環境変数で制御
- 読み取り専用操作のみ（issue get, list, stale, blockers, workload）

### Golden テスト
- analysis 出力の JSON snapshot を `testdata/` に保存
- スキーマ変更を検出

---

## 5. MCP 反映方針

Phase 1 の全分析機能を MCP tool として公開:

| MCP Tool | 対応 M# |
|----------|--------|
| `logvalet_issue_context` | M22 |
| `logvalet_issue_stale` | M24 |
| `logvalet_project_blockers` | M26 |
| `logvalet_user_workload` | M28 |
| `logvalet_project_health` | M30 |

`internal/mcp/tools_analysis.go` に集約。`RegisterAnalysisTools(reg)` を `server.go` に追加。

---

## 6. JSON スキーマ方針

- AnalysisEnvelope は DigestEnvelope と同構造（schema_version, resource, generated_at, warnings）
- `analysis` フィールドに各機能固有の構造体
- analysis 出力の JSON キーは `snake_case`（digest と統一）
- compact モード: description, comment content を省略してトークン削減

---

## 7. リスク と 対策

| リスク | 影響 | 対策 |
|--------|------|------|
| N+1 API 呼び出し（workload で全プロジェクト×全ユーザー） | レート制限 | ListIssues の assigneeId[] フィルタで一括取得 |
| stale 閾値の妥当性 | 誤検出 | デフォルト7日 + --days で上書き可能 |
| blocker 判定の false positive | 信頼性低下 | signals 配列で根拠を明示、severity でフィルタ可能 |
| analysis/ と digest/ の責務重複 | 保守コスト | digest=要約(what)、analysis=洞察(so what) を明確に分離 |

---

## 8. 完了条件

### Phase 1 完了条件
- [ ] `issue context` が1コマンドで課題の判断材料を返せる
- [ ] `issue stale` が停滞課題を検出できる（デフォルト7日閾値）
- [ ] `project blockers` がプロジェクトの進行阻害要因を抽出できる
- [ ] `user workload` が担当者の負荷状況を可視化できる
- [ ] `project health` が統合ビューを返せる
- [ ] 全機能が CLI + MCP 両方で利用可能
- [ ] JSON スキーマが安定（AnalysisEnvelope 統一）
- [ ] 全テストがパス（unit + E2E）
- [ ] README.md / README.ja.md に新コマンドを記載
- [ ] `logvalet` スキル (SKILL.md) に新コマンドを追加
- [ ] `logvalet-health` / `logvalet-context` 新スキル作成
- [ ] 既存スキル（logvalet-my-week, logvalet-my-next, logvalet-report）更新
- [ ] 全コマンドの help テキストが正確

### Phase 2 完了条件
- [ ] AI が「読める」だけでなく「下書きできる」
- [ ] 定期 digest（weekly/daily）が出せる
- [ ] triage で課題の仕分け支援ができる
- [ ] `logvalet-triage` / `logvalet-digest-periodic` 新スキル作成
- [ ] 既存スキル更新（logvalet, logvalet-issue-create）
- [ ] README 更新

### Phase 3 完了条件
- [ ] 意思決定ログが抽出できる
- [ ] アクティビティの異常・偏りを検出できる
- [ ] プロジェクトリスクを構造化して返せる
- [ ] `logvalet-intelligence` 新スキル作成
- [ ] 全スキル最終更新
- [ ] README 完全更新

---

## 9. 実装順序の根拠

1. **M20 (analysis基盤+IssueContext)** を最初にする理由:
   - 全後続機能が依存する共通型を定義
   - IssueContext は最も価値が高く、他機能の土台（signals が stale/blocker の基礎）
   - 既存パターン（BaseDigestBuilder）を踏襲でき、実装リスクが低い

2. **Stale → Blocker → Workload** の順序:
   - Stale は単純（日数計算のみ）で実装しやすい
   - Blocker は Stale のロジックを内部で再利用
   - Workload は独立だが、Phase 1 統合（M29-30）で stale/blocker と合流

3. **CLI → MCP の順序**:
   - CLI で動作検証してから MCP に反映（品質保証）
   - ただしマイルストーンを分けて粒度を細かくする

---

## 10. 検証方法

```bash
# Unit テスト
go test ./internal/analysis/...

# 全テスト
go test ./...

# E2E テスト（実 Backlog API）
LOGVALET_E2E=1 go test ./internal/analysis/... -tags e2e -run TestE2E

# CLI 動作確認
logvalet issue context PROJ-123
logvalet issue context PROJ-123 --compact -f json
logvalet issue stale -k PROJECT
logvalet project blockers PROJECT
logvalet user workload

# MCP 動作確認
# MCP サーバー起動後、logvalet_issue_context tool を呼び出し
```

---

## 11. 次に作成すべき実装プランの章立て案

ロードマップ承認後、各マイルストーンの実装プランを以下の構造で作成:

1. **目的と完了条件**
2. **TDD テストケース一覧**（Red フェーズで書くテスト）
3. **実装ファイル一覧**（新規/変更）
4. **型定義・インターフェース**
5. **実装手順**（テスト → 実装 → リファクタの順）
6. **E2E テスト計画**
7. **検証コマンド**

---

## 12. 出力ファイル計画

このプラン承認後に作成するファイル:

| ファイル | 内容 |
|---------|------|
| `plans/logvalet-roadmap-v3.md` | Phase 1-3 ロードマップ（本プランのセクション2を独立ファイル化） |
| `plans/logvalet-m20-analysis-foundation.md` | M20 詳細実装計画 |

### 各フェーズ末のスキル・ドキュメント対象

| Phase | 新規スキル | 既存スキル更新 | ドキュメント |
|-------|-----------|-------------|------------|
| 1 (M31) | `logvalet-health`, `logvalet-context` | `logvalet`, `logvalet-my-week`, `logvalet-my-next`, `logvalet-report` | README.md, README.ja.md |
| 2 (M40) | `logvalet-triage`, `logvalet-digest-periodic` | `logvalet`, `logvalet-issue-create` | README.md, README.ja.md |
| 3 (M48) | `logvalet-intelligence` | 全スキル最終更新 | README.md, README.ja.md |

> **注意**: `docs/specs/` は初期構想として保存。編集しない。スキルは全て `skills/` ディレクトリに配置（plugin 配布用）。
