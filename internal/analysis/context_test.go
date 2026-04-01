package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// helperIssue は テスト用の domain.Issue を返すヘルパー。
func helperIssue(now time.Time) *domain.Issue {
	updated := now.Add(-2 * 24 * time.Hour) // 2日前
	created := now.Add(-10 * 24 * time.Hour)
	dueDate := now.Add(5 * 24 * time.Hour) // 5日後
	return &domain.Issue{
		ID:        1001,
		ProjectID: 100,
		IssueKey:  "PROJ-123",
		Summary:   "テストの課題",
		Description: "テストの説明文",
		Status:    &domain.IDName{ID: 1, Name: "未対応"},
		Priority:  &domain.IDName{ID: 2, Name: "高"},
		IssueType: &domain.IDName{ID: 3, Name: "バグ"},
		Assignee:  &domain.User{ID: 10, UserID: "user1", Name: "テストユーザー"},
		Reporter:  &domain.User{ID: 20, UserID: "user2", Name: "報告者"},
		Categories: []domain.IDName{{ID: 1, Name: "カテゴリA"}},
		Milestones: []domain.IDName{{ID: 1, Name: "v1.0"}},
		DueDate:    &dueDate,
		Created:    &created,
		Updated:    &updated,
	}
}

// helperComments は テスト用のコメント一覧を返すヘルパー。
func helperComments(count int) []domain.Comment {
	comments := make([]domain.Comment, count)
	for i := 0; i < count; i++ {
		t := time.Date(2026, 3, 20+i, 10, 0, 0, 0, time.UTC)
		comments[i] = domain.Comment{
			ID:          int64(i + 1),
			Content:     "コメント内容",
			CreatedUser: &domain.User{ID: 10, UserID: "user1", Name: "テストユーザー"},
			Created:     &t,
		}
	}
	return comments
}

// helperStatuses は テスト用のステータス一覧を返すヘルパー。
func helperStatuses() []domain.Status {
	return []domain.Status{
		{ID: 1, Name: "未対応", DisplayOrder: 0},
		{ID: 2, Name: "処理中", DisplayOrder: 1},
		{ID: 3, Name: "処理済み", DisplayOrder: 2},
		{ID: 4, Name: "完了", DisplayOrder: 3},
	}
}

// T3: TestIssueContextBuilder_Build_Success は正常系を検証する。
func TestIssueContextBuilder_Build_Success(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)
	comments := helperComments(5)
	statuses := helperStatuses()

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return statuses, nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if env.Resource != "issue_context" {
		t.Errorf("Resource = %q, want %q", env.Resource, "issue_context")
	}

	ic, ok := env.Analysis.(*IssueContext)
	if !ok {
		t.Fatalf("Analysis type = %T, want *IssueContext", env.Analysis)
	}

	// issue snapshot
	if ic.Issue.IssueKey != "PROJ-123" {
		t.Errorf("IssueKey = %q, want %q", ic.Issue.IssueKey, "PROJ-123")
	}
	if ic.Issue.Summary != "テストの課題" {
		t.Errorf("Summary = %q, want %q", ic.Issue.Summary, "テストの課題")
	}

	// recent_comments (デフォルト MaxComments=10、コメント5件)
	if len(ic.RecentComments) != 5 {
		t.Errorf("RecentComments length = %d, want 5", len(ic.RecentComments))
	}

	// meta.statuses
	if len(ic.Meta.Statuses) != 4 {
		t.Errorf("Meta.Statuses length = %d, want 4", len(ic.Meta.Statuses))
	}

	// signals
	if !ic.Signals.HasAssignee {
		t.Error("Signals.HasAssignee = false, want true")
	}
	if ic.Signals.CommentCount != 5 {
		t.Errorf("Signals.CommentCount = %d, want 5", ic.Signals.CommentCount)
	}

	// llm_hints
	found := false
	for _, e := range ic.LLMHints.PrimaryEntities {
		if e == "issue:PROJ-123" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("LLMHints.PrimaryEntities does not contain %q", "issue:PROJ-123")
	}
}

// T4: TestIssueContextBuilder_Build_Compact は Compact モードを検証する。
func TestIssueContextBuilder_Build_Compact(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)
	comments := helperComments(3)

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperStatuses(), nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{Compact: true})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ic := env.Analysis.(*IssueContext)

	if ic.Issue.Description != "" {
		t.Errorf("Compact: Issue.Description = %q, want empty", ic.Issue.Description)
	}
	for i, c := range ic.RecentComments {
		if c.Content != "" {
			t.Errorf("Compact: RecentComments[%d].Content = %q, want empty", i, c.Content)
		}
	}

	// その他フィールドは正常
	if ic.Issue.IssueKey != "PROJ-123" {
		t.Errorf("IssueKey = %q, want %q", ic.Issue.IssueKey, "PROJ-123")
	}
}

// T5: TestIssueContextBuilder_Build_MaxComments は MaxComments 制限を検証する。
func TestIssueContextBuilder_Build_MaxComments(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)
	comments := helperComments(10)

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperStatuses(), nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{MaxComments: 3})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ic := env.Analysis.(*IssueContext)

	if len(ic.RecentComments) != 3 {
		t.Errorf("RecentComments length = %d, want 3", len(ic.RecentComments))
	}
	if ic.Signals.CommentCount != 10 {
		t.Errorf("Signals.CommentCount = %d, want 10 (制限前の総数)", ic.Signals.CommentCount)
	}
}

// T6: TestIssueContextBuilder_Build_OverdueSignal は期限超過シグナルを検証する。
func TestIssueContextBuilder_Build_OverdueSignal(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)
	overdueDue := fixedNow.Add(-2 * 24 * time.Hour) // 2日前
	issue.DueDate = &overdueDue

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperStatuses(), nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ic := env.Analysis.(*IssueContext)

	if !ic.Signals.IsOverdue {
		t.Error("Signals.IsOverdue = false, want true")
	}
}

// T7: TestIssueContextBuilder_Build_StaleSignal は stale シグナル（10日更新なし）を検証する。
func TestIssueContextBuilder_Build_StaleSignal(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)
	staleUpdated := fixedNow.Add(-10 * 24 * time.Hour) // 10日前
	issue.Updated = &staleUpdated

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperStatuses(), nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ic := env.Analysis.(*IssueContext)

	if !ic.Signals.IsStale {
		t.Error("Signals.IsStale = false, want true")
	}
	if ic.Signals.DaysSinceUpdate != 10 {
		t.Errorf("Signals.DaysSinceUpdate = %d, want 10", ic.Signals.DaysSinceUpdate)
	}
}

// T8: TestIssueContextBuilder_Build_NotStale は stale でない場合を検証する。
func TestIssueContextBuilder_Build_NotStale(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)
	recentUpdated := fixedNow.Add(-3 * 24 * time.Hour) // 3日前
	issue.Updated = &recentUpdated

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperStatuses(), nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ic := env.Analysis.(*IssueContext)

	if ic.Signals.IsStale {
		t.Error("Signals.IsStale = true, want false")
	}
	if ic.Signals.DaysSinceUpdate != 3 {
		t.Errorf("Signals.DaysSinceUpdate = %d, want 3", ic.Signals.DaysSinceUpdate)
	}
}

// T9: TestIssueContextBuilder_Build_IssueNotFound は課題が見つからない場合のエラーを検証する。
func TestIssueContextBuilder_Build_IssueNotFound(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return nil, backlog.ErrNotFound
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com")

	_, err := builder.Build(context.Background(), "PROJ-999", IssueContextOptions{})
	if err == nil {
		t.Fatal("Build() error = nil, want error")
	}
	if !errors.Is(err, backlog.ErrNotFound) {
		t.Errorf("Build() error = %v, want errors.Is(err, backlog.ErrNotFound)", err)
	}
}

// T10: TestIssueContextBuilder_Build_PartialFailure はコメント取得失敗時の部分成功を検証する。
func TestIssueContextBuilder_Build_PartialFailure(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	issue := helperIssue(fixedNow)

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, errors.New("API error: comments unavailable")
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperStatuses(), nil
	}

	builder := NewIssueContextBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := builder.Build(context.Background(), "PROJ-123", IssueContextOptions{})
	if err != nil {
		t.Fatalf("Build() should not return error on partial failure, got: %v", err)
	}

	// warnings に "comments_fetch_failed" が含まれる
	foundWarning := false
	for _, w := range env.Warnings {
		if w.Code == "comments_fetch_failed" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Warnings does not contain code 'comments_fetch_failed'")
	}

	ic := env.Analysis.(*IssueContext)

	// recent_comments は空配列
	if ic.RecentComments == nil {
		t.Error("RecentComments = nil, want empty slice")
	}
	if len(ic.RecentComments) != 0 {
		t.Errorf("RecentComments length = %d, want 0", len(ic.RecentComments))
	}

	// comment_count = 0
	if ic.Signals.CommentCount != 0 {
		t.Errorf("Signals.CommentCount = %d, want 0", ic.Signals.CommentCount)
	}
}
