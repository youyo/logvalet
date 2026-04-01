package analysis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
	"golang.org/x/sync/errgroup"
)

// DefaultStaleDays は stale 判定のデフォルト閾値（日数）。
const DefaultStaleDays = 7

// DefaultMaxComments は MaxComments のデフォルト値。
const DefaultMaxComments = 10

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
	IssueKey    string          `json:"issue_key"`
	Summary     string          `json:"summary"`
	Description string          `json:"description,omitempty"`
	Status      *domain.IDName  `json:"status,omitempty"`
	Priority    *domain.IDName  `json:"priority,omitempty"`
	IssueType   *domain.IDName  `json:"issue_type,omitempty"`
	Assignee    *domain.UserRef `json:"assignee,omitempty"`
	Reporter    *domain.UserRef `json:"reporter,omitempty"`
	Categories  []domain.IDName `json:"categories"`
	Milestones  []domain.IDName `json:"milestones"`
	DueDate     *time.Time      `json:"due_date,omitempty"`
	StartDate   *time.Time      `json:"start_date,omitempty"`
	Created     *time.Time      `json:"created,omitempty"`
	Updated     *time.Time      `json:"updated,omitempty"`
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
	// デフォルト値
	if opt.MaxComments <= 0 {
		opt.MaxComments = DefaultMaxComments
	}

	// 1. issue 取得（必須 — 失敗したらエラー返却）
	issue, err := b.client.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", issueKey, err)
	}

	// 2. errgroup で comments, statuses を並行取得（部分失敗は warnings）
	var (
		comments []domain.Comment
		statuses []domain.Status
		mu       sync.Mutex
		warnings []domain.Warning
	)

	projectKey := extractProjectKey(issueKey)

	g, gctx := errgroup.WithContext(ctx)

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
			return nil // 部分失敗は warning に留める
		}
		mu.Lock()
		comments = cs
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		ss, err := b.client.ListProjectStatuses(gctx, projectKey)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "statuses_fetch_failed",
				Message:   fmt.Sprintf("failed to list statuses: %v", err),
				Component: "meta.statuses",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}
		mu.Lock()
		statuses = ss
		mu.Unlock()
		return nil
	})

	_ = g.Wait()

	// 3. snapshot, signals, llm_hints を組み立て
	snapshot := buildSnapshot(issue, opt.Compact)
	recentComments := buildComments(comments, opt.MaxComments, opt.Compact)
	totalCommentCount := len(comments)
	signals := buildSignals(issue, totalCommentCount, b.now())
	hints := buildLLMHints(issueKey, signals)
	meta := buildMeta(statuses)

	ic := &IssueContext{
		Issue:          snapshot,
		Meta:           meta,
		RecentComments: recentComments,
		Signals:        signals,
		LLMHints:       hints,
	}

	// 4. envelope で包んで返却
	return b.newEnvelope("issue_context", ic, warnings), nil
}

// buildSnapshot は domain.Issue から IssueSnapshot を構築する。
func buildSnapshot(issue *domain.Issue, compact bool) IssueSnapshot {
	categories := issue.Categories
	if categories == nil {
		categories = []domain.IDName{}
	}
	milestones := issue.Milestones
	if milestones == nil {
		milestones = []domain.IDName{}
	}

	s := IssueSnapshot{
		IssueKey:   issue.IssueKey,
		Summary:    issue.Summary,
		Status:     issue.Status,
		Priority:   issue.Priority,
		IssueType:  issue.IssueType,
		Assignee:   toUserRef(issue.Assignee),
		Reporter:   toUserRef(issue.Reporter),
		Categories: categories,
		Milestones: milestones,
		DueDate:    issue.DueDate,
		StartDate:  issue.StartDate,
		Created:    issue.Created,
		Updated:    issue.Updated,
	}

	if !compact {
		s.Description = issue.Description
	}

	return s
}

// buildComments は domain.Comment スライスから ContextComment スライスを構築する。
func buildComments(comments []domain.Comment, maxComments int, compact bool) []ContextComment {
	if comments == nil {
		return []ContextComment{}
	}

	limit := len(comments)
	if limit > maxComments {
		limit = maxComments
	}

	result := make([]ContextComment, limit)
	for i := 0; i < limit; i++ {
		c := comments[i]
		cc := ContextComment{
			ID:      c.ID,
			Author:  toUserRef(c.CreatedUser),
			Created: c.Created,
		}
		if !compact {
			cc.Content = c.Content
		}
		result[i] = cc
	}
	return result
}

// buildSignals は issue の状態シグナルを計算する。
func buildSignals(issue *domain.Issue, totalCommentCount int, now time.Time) IssueSignals {
	signals := IssueSignals{
		HasAssignee:  issue.Assignee != nil,
		CommentCount: totalCommentCount,
	}

	if issue.DueDate != nil && issue.DueDate.Before(now) {
		signals.IsOverdue = true
	}

	if issue.Updated != nil {
		days := int(now.Sub(*issue.Updated).Hours() / 24)
		signals.DaysSinceUpdate = days
		signals.IsStale = days >= DefaultStaleDays
	}

	return signals
}

// buildLLMHints は LLM 向けヒント情報を構築する。
func buildLLMHints(issueKey string, signals IssueSignals) digest.DigestLLMHints {
	hints := digest.DigestLLMHints{
		PrimaryEntities:      []string{fmt.Sprintf("issue:%s", issueKey)},
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
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

	return hints
}

// buildMeta はプロジェクトメタ情報を構築する。
func buildMeta(statuses []domain.Status) ContextMeta {
	if statuses == nil {
		statuses = []domain.Status{}
	}
	return ContextMeta{
		Statuses: statuses,
	}
}
