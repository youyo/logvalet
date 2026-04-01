# M20: analysis 基盤 + IssueContext ロジック

Plan: plans/tidy-tickling-moon.md
Roadmap: plans/logvalet-roadmap-v3.md

## 目的

`internal/analysis/` パッケージを新設し、全 Phase 1-3 分析機能の共通基盤と、
最初の分析機能 IssueContextBuilder を TDD で実装する。

## 完了条件

- [ ] `internal/analysis/analysis.go` — AnalysisEnvelope, BaseAnalysisBuilder
- [ ] `internal/analysis/analysis_test.go` — envelope JSON shape テスト
- [ ] `internal/analysis/context.go` — IssueContextBuilder + 全型定義
- [ ] `internal/analysis/context_test.go` — 8 テストケース全パス
- [ ] `go test ./internal/analysis/...` パス
- [ ] `go vet ./...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### analysis_test.go

```
T1: TestNewAnalysisEnvelope_JSONShape
    - NewAnalysisEnvelope() で生成 → JSON Marshal
    - schema_version, resource, generated_at, analysis フィールドが存在
    - warnings が空配列（nil でない）

T2: TestBaseAnalysisBuilder_NewEnvelope
    - newEnvelope() が profile, space, base_url を正しくセット
    - now() の clock injection が反映される
```

### context_test.go

```
T3: TestIssueContextBuilder_Build_Success
    入力: MockClient に GetIssue, ListIssueComments, ListProjectStatuses をセット
    期待:
    - issue snapshot の全フィールドが正しくマッピング
    - recent_comments が MaxComments 件（デフォルト10）
    - meta.statuses が取得済み
    - signals.has_assignee = true（assignee あり）
    - signals.comment_count = コメント総数
    - llm_hints.primary_entities に "issue:PROJ-123" 含む
    - AnalysisEnvelope.resource = "issue_context"

T4: TestIssueContextBuilder_Build_Compact
    入力: Compact=true
    期待:
    - issue.description が空文字列
    - recent_comments[*].content が空文字列
    - それ以外のフィールドは正常

T5: TestIssueContextBuilder_Build_MaxComments
    入力: MaxComments=3, MockClient に10件のコメントを返す
    期待:
    - recent_comments の件数 = 3
    - signals.comment_count = 10（制限前の総数）

T6: TestIssueContextBuilder_Build_OverdueSignal
    入力: DueDate = now() の2日前
    期待:
    - signals.is_overdue = true

T7: TestIssueContextBuilder_Build_StaleSignal
    入力: Updated = now() の10日前
    期待:
    - signals.is_stale = true
    - signals.days_since_update = 10

T8: TestIssueContextBuilder_Build_NotStale
    入力: Updated = now() の3日前
    期待:
    - signals.is_stale = false
    - signals.days_since_update = 3

T9: TestIssueContextBuilder_Build_IssueNotFound
    入力: GetIssueFunc が ErrNotFound を返す
    期待:
    - Build() がエラーを返す
    - errors.Is(err, backlog.ErrNotFound) = true

T10: TestIssueContextBuilder_Build_PartialFailure
    入力: GetIssue は成功、ListIssueComments が error を返す
    期待:
    - Build() はエラーを返さない
    - AnalysisEnvelope.warnings に "comments_fetch_failed" 含む
    - recent_comments は空配列
    - signals.comment_count = 0
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/analysis/analysis.go` | AnalysisEnvelope, BaseAnalysisBuilder, Option 関数 |
| `internal/analysis/analysis_test.go` | T1, T2 |
| `internal/analysis/context.go` | IssueContextBuilder, IssueContext, IssueSnapshot, IssueSignals 等 |
| `internal/analysis/context_test.go` | T3-T10 |

### 変更なし

M20 では CLI/MCP/既存パッケージへの変更はない。

---

## 3. 型定義

### analysis.go

```go
package analysis

import (
    "time"

    "github.com/youyo/logvalet/internal/backlog"
    "github.com/youyo/logvalet/internal/domain"
)

// AnalysisEnvelope は全 analysis コマンドの共通ラッパー。
// DigestEnvelope と同構造（LLM が同パターンで処理可能）。
type AnalysisEnvelope struct {
    SchemaVersion string           `json:"schema_version"`
    Resource      string           `json:"resource"`
    GeneratedAt   time.Time        `json:"generated_at"`
    Profile       string           `json:"profile"`
    Space         string           `json:"space"`
    BaseURL       string           `json:"base_url"`
    Warnings      []domain.Warning `json:"warnings"`
    Analysis      any              `json:"analysis"`
}

// BaseAnalysisBuilder は全 AnalysisBuilder に共通するフィールドと helper を提供する。
type BaseAnalysisBuilder struct {
    client  backlog.Client
    profile string
    space   string
    baseURL string
    now     func() time.Time
}

// Option は BaseAnalysisBuilder のオプション設定関数型。
type Option func(*BaseAnalysisBuilder)

// WithClock はテスト用の clock injection オプション。
func WithClock(now func() time.Time) Option {
    return func(b *BaseAnalysisBuilder) {
        b.now = now
    }
}

// NewBaseAnalysisBuilder は BaseAnalysisBuilder を生成する。
func NewBaseAnalysisBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) BaseAnalysisBuilder {
    b := BaseAnalysisBuilder{
        client:  client,
        profile: profile,
        space:   space,
        baseURL: baseURL,
        now:     time.Now,
    }
    for _, opt := range opts {
        opt(&b)
    }
    return b
}

// newEnvelope は AnalysisEnvelope を組み立てる共通 helper。
func (b *BaseAnalysisBuilder) newEnvelope(resource string, analysis any, warnings []domain.Warning) *AnalysisEnvelope {
    if warnings == nil {
        warnings = []domain.Warning{}
    }
    return &AnalysisEnvelope{
        SchemaVersion: "1",
        Resource:      resource,
        GeneratedAt:   b.now().UTC(),
        Profile:       b.profile,
        Space:         b.space,
        BaseURL:       b.baseURL,
        Warnings:      warnings,
        Analysis:      analysis,
    }
}

// toUserRef は domain.User を domain.UserRef に変換する（nil 安全）。
func toUserRef(user *domain.User) *domain.UserRef {
    if user == nil {
        return nil
    }
    return &domain.UserRef{ID: user.ID, Name: user.Name}
}
```

### context.go

```go
package analysis

import (
    "context"
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/youyo/logvalet/internal/backlog"
    "github.com/youyo/logvalet/internal/digest"
    "github.com/youyo/logvalet/internal/domain"
    "golang.org/x/sync/errgroup"
)

// DefaultStaleDays は stale 判定のデフォルト閾値（日数）。
const DefaultStaleDays = 7

// IssueContextOptions は IssueContextBuilder.Build() のオプション。
type IssueContextOptions struct {
    MaxComments int  // default: 10
    Compact     bool // true: description, comment content を省略
}

// IssueContext は issue context 分析の結果。
type IssueContext struct {
    Issue          IssueSnapshot         `json:"issue"`
    Meta           ContextMeta           `json:"meta"`
    RecentComments []ContextComment      `json:"recent_comments"`
    Signals        IssueSignals          `json:"signals"`
    LLMHints       digest.DigestLLMHints `json:"llm_hints"`
}

// IssueSnapshot は issue の正規化スナップショット（snake_case JSON）。
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

// ContextMeta はプロジェクトメタ情報。
type ContextMeta struct {
    Statuses []domain.Status `json:"statuses"`
}

// ContextComment はコンテキスト内のコメント表現。
type ContextComment struct {
    ID      int64           `json:"id"`
    Content string          `json:"content"`
    Author  *domain.UserRef `json:"author,omitempty"`
    Created *time.Time      `json:"created,omitempty"`
}

// IssueSignals は issue の状態シグナル。
type IssueSignals struct {
    IsOverdue       bool `json:"is_overdue"`
    DaysSinceUpdate int  `json:"days_since_update"`
    IsStale         bool `json:"is_stale"`
    HasAssignee     bool `json:"has_assignee"`
    CommentCount    int  `json:"comment_count"`
}

// IssueContextBuilder は IssueContext を構築する。
type IssueContextBuilder struct {
    BaseAnalysisBuilder
}

// NewIssueContextBuilder は IssueContextBuilder を生成する。
func NewIssueContextBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *IssueContextBuilder {
    return &IssueContextBuilder{
        BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
    }
}

// Build は issueKey の IssueContext を構築する。
func (b *IssueContextBuilder) Build(ctx context.Context, issueKey string, opt IssueContextOptions) (*AnalysisEnvelope, error) {
    // 1. issue 取得（必須 — 失敗したらエラー返却）
    // 2. errgroup で comments, statuses を並行取得（部分失敗は warnings）
    // 3. snapshot, signals, llm_hints を組み立て
    // 4. envelope で包んで返却
    ...
}
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red — テストを先に書く

1. `internal/analysis/analysis_test.go` を作成（T1, T2）
2. `internal/analysis/context_test.go` を作成（T3-T10）
3. `go test ./internal/analysis/...` → コンパイルエラー（型未定義）

### Step 2: Green — 最小限の実装

1. `internal/analysis/analysis.go` を実装
   - AnalysisEnvelope, BaseAnalysisBuilder, Option, WithClock, NewBaseAnalysisBuilder, newEnvelope, toUserRef
2. `go test ./internal/analysis/...` → T1, T2 パス
3. `internal/analysis/context.go` を実装
   - 型定義（IssueContext, IssueSnapshot, ContextMeta, ContextComment, IssueSignals）
   - IssueContextBuilder.Build() の本体
4. `go test ./internal/analysis/...` → T3-T10 パス

### Step 3: Refactor

1. コードの整理（不要なコメント削除、関数の抽出）
2. `go test ./internal/analysis/...` → 全テストパス
3. `go vet ./...` → クリーン

---

## 5. 実装の要点

### extractProjectKey

`issueKey` (e.g., "PROJ-123") からプロジェクトキー "PROJ" を抽出。
`digest/common.go` に既存の `extractProjectKey()` があるが、package private。
analysis パッケージにも同じヘルパーを追加（重複は軽微）。

### MaxComments のデフォルト

```go
if opt.MaxComments <= 0 {
    opt.MaxComments = 10
}
```

### Compact モード

```go
if opt.Compact {
    snapshot.Description = ""
    for i := range comments {
        comments[i].Content = ""
    }
}
```

### Signals 計算

```go
now := b.now()

signals.HasAssignee = issue.Assignee != nil
signals.CommentCount = totalCommentCount  // 制限前の総数

if issue.DueDate != nil && issue.DueDate.Before(now) {
    signals.IsOverdue = true
}

if issue.Updated != nil {
    days := int(now.Sub(*issue.Updated).Hours() / 24)
    signals.DaysSinceUpdate = days
    signals.IsStale = days >= DefaultStaleDays
}
```

### LLMHints 生成

```go
hints := digest.DigestLLMHints{
    PrimaryEntities: []string{fmt.Sprintf("issue:%s", issueKey)},
}

if signals.IsOverdue {
    hints.OpenQuestions = append(hints.OpenQuestions, "期限超過 — 対応状況を確認してください")
}
if signals.IsStale {
    hints.OpenQuestions = append(hints.OpenQuestions, fmt.Sprintf("%d日間更新なし — 停滞の原因を確認してください", signals.DaysSinceUpdate))
}
if !signals.HasAssignee {
    hints.SuggestedNextActions = append(hints.SuggestedNextActions, "担当者を設定してください")
}
```

### 部分失敗処理

```go
g, gctx := errgroup.WithContext(ctx)
var mu sync.Mutex
var warnings []domain.Warning

// comments 取得
g.Go(func() error {
    cs, err := b.client.ListIssueComments(gctx, issueKey, backlog.ListCommentsOptions{})
    if err != nil {
        mu.Lock()
        warnings = append(warnings, domain.Warning{
            Code:      "comments_fetch_failed",
            Message:   fmt.Sprintf("failed to list comments: %v", err),
            Component: "recent_comments",
            Retryable: true,
        })
        mu.Unlock()
        return nil  // 部分失敗は warning に留める
    }
    // ... comments をセット
    return nil
})
```

---

## 6. 検証コマンド

```bash
# テスト実行
go test ./internal/analysis/... -v

# 全テスト
go test ./...

# Lint
go vet ./...
```

---

## 7. 次のマイルストーン

M20 完了後 → M21（IssueContext CLI コマンド）へ進む。
M21 では `internal/cli/issue.go` に `Context IssueContextCmd` を追加し、
`internal/cli/issue_context.go` で CLI コマンドを実装する。
