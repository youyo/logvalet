# M25 BlockerDetector ロジック — 詳細実装計画

## 概要

BlockerDetector は Backlog プロジェクト内の課題の進行阻害要因を検出する分析機能。
`internal/analysis/blocker.go` に実装し、`StaleIssueDetector` と同パターンを踏襲する。

依存: M20 (analysis 基盤)  
次: M26 (Project Blockers CLI + MCP)

---

## 検出シグナル

Backlog API から取得できる情報に基づき、以下を deterministic に検出する:

| シグナル | severity | 検出ロジック |
|---------|----------|------------|
| `high_priority_unassigned` | HIGH | 優先度が「高」かつ担当者なし |
| `long_in_progress` | MEDIUM | ステータスが「処理中」で `updated` から N 日経過 |
| `overdue_open` | HIGH | DueDate が過去かつ未完了ステータス |
| `blocked_by_keyword` | MEDIUM | コメント本文に「ブロック」「待ち」「blocked」「pending」「依存」等を含む（最新コメント確認） |

> **設計判断**:
> - 親子課題・依存リンクは Backlog API に標準エンドポイントがないため今回スコープ外
> - コメントキーワード検出は `ListIssueComments` を使用。N+1 問題があるため opt-in フラグ `IncludeComments bool` で制御

---

## データ構造

```go
// BlockerConfig はブロッカー検出の設定
type BlockerConfig struct {
    InProgressDays   int      // 「処理中」ステータスの停滞閾値（日数）。0以下の場合 DefaultInProgressDays を使用
    InProgressStatus []string // 「処理中」とみなすステータス名リスト（デフォルト: ["処理中"]）
    HighPriority     []string // 「高優先度」とみなす優先度名リスト（デフォルト: ["高", "最高"]）
    ExcludeStatus    []string // 除外ステータス名（完了系）
    IncludeComments  bool     // コメントキーワード検出を有効化するか
    MaxCommentCount  int      // コメント取得件数上限（0の場合 DefaultMaxCommentCount を使用）
}

// BlockerResult はブロッカー検出の結果
type BlockerResult struct {
    Issues     []BlockerIssue        `json:"issues"`
    TotalCount int                   `json:"total_count"`
    BySeverity map[string]int        `json:"by_severity"`
    LLMHints   digest.DigestLLMHints `json:"llm_hints"`
}

// BlockerIssue は進行阻害と判定された個別課題
type BlockerIssue struct {
    IssueKey   string          `json:"issue_key"`
    Summary    string          `json:"summary"`
    Status     string          `json:"status"`
    Priority   string          `json:"priority,omitempty"`
    Assignee   *domain.UserRef `json:"assignee,omitempty"`
    DueDate    *time.Time      `json:"due_date,omitempty"`
    Signals    []BlockerSignal `json:"signals"`
    Severity   string          `json:"severity"` // "HIGH" | "MEDIUM" | "LOW"
}

// BlockerSignal は個別の阻害要因
type BlockerSignal struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

---

## 定数

```go
const (
    DefaultInProgressDays  = 14  // 処理中ステータス停滞閾値（デフォルト14日）
    DefaultMaxCommentCount = 5   // コメント取得件数上限（デフォルト5件）

    // シグナルコード定数
    SignalHighPriorityUnassigned = "high_priority_unassigned"
    SignalLongInProgress         = "long_in_progress"
    SignalOverdueOpen            = "overdue_open"
    SignalBlockedByKeyword       = "blocked_by_keyword"
)

// defaultInProgressStatus はデフォルトの処理中ステータス名リスト
var defaultInProgressStatus = []string{"処理中"}

// defaultHighPriority はデフォルトの高優先度名リスト
var defaultHighPriority = []string{"高", "最高"}
```

---

## TDD 設計（Red → Green → Refactor）

### テストケース一覧

| TC | テスト名 | 検証内容 |
|----|---------|---------|
| T1 | `TestBlockerDetector_Detect_HighPriorityUnassigned` | 優先度高+未アサインが HIGH で検出される |
| T2 | `TestBlockerDetector_Detect_LongInProgress` | 処理中 N 日超がシグナルに含まれる |
| T3 | `TestBlockerDetector_Detect_OverdueOpen` | 期限超過+未完了が HIGH で検出される |
| T4 | `TestBlockerDetector_Detect_MultipleSignals` | 1課題に複数シグナルが付く |
| T5 | `TestBlockerDetector_Detect_ExcludeStatus` | 完了ステータスが除外される |
| T6 | `TestBlockerDetector_Detect_NoBlockers` | 阻害なし → issues 空スライス |
| T7 | `TestBlockerDetector_Detect_CommentKeyword` | IncludeComments=true でキーワード検出 |
| T8 | `TestBlockerDetector_Detect_BySeverity` | by_severity カウントが正確 |
| T9 | `TestBlockerDetector_Detect_MultiProject` | 複数プロジェクト統合 |
| T10 | `TestBlockerDetector_Detect_ProjectError` | 部分失敗 → warning + 残りプロジェクト結果返却 |
| T11 | `TestBlockerDetector_Detect_LLMHints` | LLMHints の生成 |
| T12 | `TestBlockerDetector_Detect_EmptyProjectKeys` | 空プロジェクトキー → 空結果 |

---

## 実装ステップ

### Step 0: 定数定義
`internal/analysis/context.go` に `DefaultInProgressDays = 14` を追加

### Step 1: Red（全テストを先に書く）
`internal/analysis/blocker_test.go` を作成。全12テストケースが全て FAIL することを確認。

### Step 2: Green（最小実装）
`internal/analysis/blocker.go` を作成:
1. `BlockerConfig`, `BlockerResult`, `BlockerIssue`, `BlockerSignal` 型定義
2. `BlockerDetector` 構造体（`BaseAnalysisBuilder` 埋め込み）
3. `NewBlockerDetector()` コンストラクタ
4. `Detect()` メソッド: プロジェクト課題取得 → 各シグナル判定 → ソート → Envelope 返却
5. シグナル検出ヘルパー関数群:
   - `classifyBlocker()` — 課題をブロッカー判定
   - `detectHighPriorityUnassigned()` — 優先度高+未アサイン
   - `detectLongInProgress()` — 処理中停滞
   - `detectOverdueOpen()` — 期限超過未完了
   - `detectCommentKeyword()` — コメントキーワード
   - `calcSeverity()` — シグナルから severity 計算（HIGH優先）
   - `buildBlockerLLMHints()` — LLMHints 生成
6. `go test ./internal/analysis/...` で全テスト GREEN を確認

### Step 3: Refactor
- 重複ロジックをヘルパー化
- `isCompletedStatus()` を `stale.go` の `buildExcludeSet()` と共通化検討
- コメント整理

---

## シグナル判定ロジック詳細

### high_priority_unassigned
```
HighPriority スライス（デフォルト: ["高", "最高"]）に issue.Priority.Name が含まれる
&& issue.Assignee == nil
→ signal: {code: "high_priority_unassigned", message: "優先度「高」で担当者未設定"}
```

### long_in_progress
```
ステータス名が InProgressStatus（デフォルト: ["処理中"]）に含まれる
&& issue.Updated != nil && daysSince(issue.Updated) >= InProgressDays
→ signal: {code: "long_in_progress", message: "処理中のまま XX 日経過"}
```

### overdue_open
```
issue.DueDate != nil && issue.DueDate.Before(now)
&& ステータス名が ExcludeStatus に含まれない（未完了）
→ signal: {code: "overdue_open", message: "期限を XX 日超過"}
```

### blocked_by_keyword（IncludeComments=true 時のみ）
```
ListIssueComments で最新 MaxCommentCount 件（デフォルト5件）を取得
最新コメント（コメント一覧の先頭）のみをキーワード検出対象とする（解消済みを誤検出防止）
コメント本文に以下を含む（case-insensitive）:
  "ブロック", "待ち", "blocked", "pending", "依存", "待機", "対応待ち"
→ signal: {code: "blocked_by_keyword", message: "コメントに阻害キーワード: 「XXX」"}
```

### severity 計算
```
シグナルに HIGH シグナル（high_priority_unassigned, overdue_open）があれば → "HIGH"
シグナルに MEDIUM シグナル（long_in_progress, blocked_by_keyword）のみ → "MEDIUM"
```

---

## ソート順

`Severity` 優先（HIGH → MEDIUM → LOW）、同一 severity 内は `IssueKey` 辞書順。

---

## API 呼び出し

```
Detect(projectKeys):
  for each projectKey:
    GetProject(projectKey) → project.ID
    ListIssues({ProjectIDs: [project.ID]}) → issues
    for each issue:
      classifyBlocker(issue)
      if IncludeComments && is potential blocker:
        ListIssueComments(issue.IssueKey, {Count: 5}) → comments
```

---

## LLMHints 生成

```
PrimaryEntities: ["project:PROJ", ...]
OpenQuestions:
  - "{N}件のブロッカー課題があります" (N > 0 の場合)
  - "{N}件が HIGH severity です" (HIGH > 0 の場合)
SuggestedNextActions: []
```

---

## ファイル構成

| ファイル | 役割 |
|---------|------|
| `internal/analysis/blocker.go` | BlockerDetector 実装 |
| `internal/analysis/blocker_test.go` | TDD テスト（12ケース） |

---

## リスク評価

| リスク | 対策 |
|--------|------|
| N+1 API（コメント取得） | `IncludeComments` で opt-in、デフォルト false |
| 優先度名が日本語/英語混在 | 優先度 ID でも判定（ID=2 が「高」の場合が多い）、将来的に設定可能に |
| 処理中ステータス名が多様 | `InProgressStatus` スライスでカスタマイズ可能 |
| false positive | `signals` 配列で根拠明示、`severity` フィルタで絞り込み可能 |

---

## 完了条件

- [ ] `go test ./internal/analysis/...` で 12 テストケースが全て GREEN
- [ ] `go test ./...` で既存テストが引き続き GREEN
- [ ] `go vet ./...` でエラーなし
- [ ] `blocker.go` のコメントが適切

Plan: plans/logvalet-m25-blocker-detector.md
