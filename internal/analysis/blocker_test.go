package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// helperBlockerIssues はブロッカー検出テスト用の課題スライスを生成するヘルパー。
func helperBlockerIssues(now time.Time, specs []blockerIssueSpec) []domain.Issue {
	issues := make([]domain.Issue, len(specs))
	for i, s := range specs {
		var updated *time.Time
		if !s.updatedNil {
			u := now.Add(-time.Duration(s.daysAgo) * 24 * time.Hour)
			updated = &u
		}
		issue := domain.Issue{
			ID:        2000 + i,
			ProjectID: s.projectID,
			IssueKey:  s.issueKey,
			Summary:   s.summary,
			Updated:   updated,
		}
		if s.statusName != "" {
			issue.Status = &domain.IDName{ID: s.statusID, Name: s.statusName}
		}
		if s.priorityName != "" {
			issue.Priority = &domain.IDName{ID: s.priorityID, Name: s.priorityName}
		}
		if s.assigneeName != "" {
			issue.Assignee = &domain.User{ID: s.assigneeID, Name: s.assigneeName}
		}
		if s.dueDate != nil {
			issue.DueDate = s.dueDate
		}
		issues[i] = issue
	}
	return issues
}

type blockerIssueSpec struct {
	issueKey     string
	summary      string
	projectID    int
	statusID     int
	statusName   string
	priorityID   int
	priorityName string
	assigneeID   int
	assigneeName string
	daysAgo      int
	dueDate      *time.Time
	updatedNil   bool
}

// T1: TestBlockerDetector_Detect_HighPriorityUnassigned は優先度高+未アサインが HIGH で検出されることを検証する。
func TestBlockerDetector_Detect_HighPriorityUnassigned(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "PROJ-1", summary: "高優先未アサイン", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, daysAgo: 3},
		{issueKey: "PROJ-2", summary: "高優先アサイン済", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, assigneeName: "田中", assigneeID: 10, daysAgo: 3},
		{issueKey: "PROJ-3", summary: "低優先未アサイン", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "低", priorityID: 4, daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if env.Resource != "project_blockers" {
		t.Errorf("Resource = %q, want %q", env.Resource, "project_blockers")
	}

	result, ok := env.Analysis.(*BlockerResult)
	if !ok {
		t.Fatalf("Analysis type = %T, want *BlockerResult", env.Analysis)
	}

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}

	bi := result.Issues[0]
	if bi.IssueKey != "PROJ-1" {
		t.Errorf("IssueKey = %q, want %q", bi.IssueKey, "PROJ-1")
	}
	if bi.Severity != "HIGH" {
		t.Errorf("Severity = %q, want HIGH", bi.Severity)
	}

	// シグナルに high_priority_unassigned が含まれる
	found := false
	for _, s := range bi.Signals {
		if s.Code == SignalHighPriorityUnassigned {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Signals does not contain %q, got %v", SignalHighPriorityUnassigned, bi.Signals)
	}
}

// T2: TestBlockerDetector_Detect_LongInProgress は処理中N日超がシグナルに含まれることを検証する。
func TestBlockerDetector_Detect_LongInProgress(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "PROJ-1", summary: "長期処理中", projectID: 100, statusName: "処理中", statusID: 2, daysAgo: 20},
		{issueKey: "PROJ-2", summary: "処理中だが短期", projectID: 100, statusName: "処理中", statusID: 2, daysAgo: 5},
		{issueKey: "PROJ-3", summary: "未対応で長期", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 20},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	// PROJ-1 のみが long_in_progress で検出される
	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}

	bi := result.Issues[0]
	if bi.IssueKey != "PROJ-1" {
		t.Errorf("IssueKey = %q, want %q", bi.IssueKey, "PROJ-1")
	}

	found := false
	for _, s := range bi.Signals {
		if s.Code == SignalLongInProgress {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Signals does not contain %q, got %v", SignalLongInProgress, bi.Signals)
	}
}

// T3: TestBlockerDetector_Detect_OverdueOpen は期限超過+未完了が HIGH で検出されることを検証する。
func TestBlockerDetector_Detect_OverdueOpen(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-3 * 24 * time.Hour)
	futureDue := fixedNow.Add(5 * 24 * time.Hour)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "PROJ-1", summary: "期限超過未完了", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5, dueDate: &pastDue},
		{issueKey: "PROJ-2", summary: "期限超過完了", projectID: 100, statusName: "完了", statusID: 4, daysAgo: 5, dueDate: &pastDue},
		{issueKey: "PROJ-3", summary: "期限内未完了", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5, dueDate: &futureDue},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{
		ExcludeStatus: []string{"完了"},
	})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}

	bi := result.Issues[0]
	if bi.IssueKey != "PROJ-1" {
		t.Errorf("IssueKey = %q, want %q", bi.IssueKey, "PROJ-1")
	}
	if bi.Severity != "HIGH" {
		t.Errorf("Severity = %q, want HIGH", bi.Severity)
	}

	found := false
	for _, s := range bi.Signals {
		if s.Code == SignalOverdueOpen {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Signals does not contain %q, got %v", SignalOverdueOpen, bi.Signals)
	}
}

// T4: TestBlockerDetector_Detect_MultipleSignals は1課題に複数シグナルが付くことを検証する。
func TestBlockerDetector_Detect_MultipleSignals(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-3 * 24 * time.Hour)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		// 期限超過 + 高優先未アサイン の両方
		{issueKey: "PROJ-1", summary: "複合ブロッカー", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, daysAgo: 5, dueDate: &pastDue},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", result.TotalCount)
	}

	bi := result.Issues[0]
	if len(bi.Signals) < 2 {
		t.Errorf("Signals length = %d, want >= 2 (multiple signals)", len(bi.Signals))
	}

	codes := map[string]bool{}
	for _, s := range bi.Signals {
		codes[s.Code] = true
	}
	if !codes[SignalHighPriorityUnassigned] {
		t.Errorf("Signals missing %q", SignalHighPriorityUnassigned)
	}
	if !codes[SignalOverdueOpen] {
		t.Errorf("Signals missing %q", SignalOverdueOpen)
	}
}

// T5: TestBlockerDetector_Detect_ExcludeStatus は完了ステータスが除外されることを検証する。
func TestBlockerDetector_Detect_ExcludeStatus(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "PROJ-1", summary: "完了高優先未アサイン", projectID: 100, statusName: "完了", statusID: 4, priorityName: "高", priorityID: 2, daysAgo: 3},
		{issueKey: "PROJ-2", summary: "未完了高優先未アサイン", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{
		ExcludeStatus: []string{"完了"},
	})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].IssueKey != "PROJ-2" {
		t.Errorf("IssueKey = %q, want PROJ-2", result.Issues[0].IssueKey)
	}
}

// T6: TestBlockerDetector_Detect_NoBlockers は阻害なし → issues 空スライスを検証する。
func TestBlockerDetector_Detect_NoBlockers(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	futureDue := fixedNow.Add(5 * 24 * time.Hour)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		// 低優先、担当あり、期限あり（未来）、処理中だが短期
		{issueKey: "PROJ-1", summary: "正常課題", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "低", priorityID: 4, assigneeName: "田中", assigneeID: 10, daysAgo: 3, dueDate: &futureDue},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.TotalCount)
	}
	if result.Issues == nil {
		t.Error("Issues = nil, want empty slice")
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues length = %d, want 0", len(result.Issues))
	}
}

// T7: TestBlockerDetector_Detect_CommentKeyword は IncludeComments=true でキーワード検出されることを検証する。
func TestBlockerDetector_Detect_CommentKeyword(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "PROJ-1", summary: "コメントあり課題", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 3},
		{issueKey: "PROJ-2", summary: "コメントなし課題", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		if issueKey == "PROJ-1" {
			return []domain.Comment{
				{ID: 1, Content: "この課題はAシステムの対応待ちでブロックされています"},
			}, nil
		}
		return []domain.Comment{
			{ID: 2, Content: "通常のコメントです"},
		}, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{
		IncludeComments: true,
	})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}

	bi := result.Issues[0]
	if bi.IssueKey != "PROJ-1" {
		t.Errorf("IssueKey = %q, want PROJ-1", bi.IssueKey)
	}

	found := false
	for _, s := range bi.Signals {
		if s.Code == SignalBlockedByKeyword {
			found = true
			// メッセージにキーワードが含まれる
			if !strings.Contains(s.Message, "ブロック") {
				t.Errorf("Signal message %q does not contain keyword", s.Message)
			}
			break
		}
	}
	if !found {
		t.Errorf("Signals does not contain %q, got %v", SignalBlockedByKeyword, bi.Signals)
	}
}

// T8: TestBlockerDetector_Detect_BySeverity は by_severity カウントが正確であることを検証する。
func TestBlockerDetector_Detect_BySeverity(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-3 * 24 * time.Hour)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		// HIGH: 期限超過
		{issueKey: "PROJ-1", summary: "overdue", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5, dueDate: &pastDue},
		// HIGH: 高優先未アサイン
		{issueKey: "PROJ-2", summary: "high prio unassigned", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, daysAgo: 3},
		// MEDIUM: 処理中長期
		{issueKey: "PROJ-3", summary: "long in progress", projectID: 100, statusName: "処理中", statusID: 2, daysAgo: 20},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", result.TotalCount)
	}

	if result.BySeverity["HIGH"] != 2 {
		t.Errorf("BySeverity[HIGH] = %d, want 2", result.BySeverity["HIGH"])
	}
	if result.BySeverity["MEDIUM"] != 1 {
		t.Errorf("BySeverity[MEDIUM] = %d, want 1", result.BySeverity["MEDIUM"])
	}

	// ソート順: HIGH が先
	if len(result.Issues) >= 2 {
		if result.Issues[0].Severity != "HIGH" {
			t.Errorf("Issues[0].Severity = %q, want HIGH (HIGH should be sorted first)", result.Issues[0].Severity)
		}
	}
}

// T9: TestBlockerDetector_Detect_MultiProject は複数プロジェクト統合を検証する。
func TestBlockerDetector_Detect_MultiProject(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	proj1Issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "P1-1", summary: "P1 高優先未アサイン", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, daysAgo: 3},
	})
	proj2Issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "P2-1", summary: "P2 長期処理中", projectID: 200, statusName: "処理中", statusID: 2, daysAgo: 20},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		switch projectKey {
		case "P1":
			return helperProject("P1", 100), nil
		case "P2":
			return helperProject("P2", 200), nil
		}
		return nil, backlog.ErrNotFound
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		if len(opt.ProjectIDs) > 0 {
			switch opt.ProjectIDs[0] {
			case 100:
				return proj1Issues, nil
			case 200:
				return proj2Issues, nil
			}
		}
		return nil, backlog.ErrNotFound
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"P1", "P2"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", result.TotalCount)
	}

	keys := map[string]bool{}
	for _, bi := range result.Issues {
		keys[bi.IssueKey] = true
	}
	if !keys["P1-1"] {
		t.Error("P1-1 not found in results")
	}
	if !keys["P2-1"] {
		t.Error("P2-1 not found in results")
	}
}

// T10: TestBlockerDetector_Detect_ProjectError は部分失敗 → warning + 残りプロジェクト結果返却を検証する。
func TestBlockerDetector_Detect_ProjectError(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	proj2Issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "P2-1", summary: "P2 高優先未アサイン", projectID: 200, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		if projectKey == "FAIL" {
			return nil, errors.New("API error: project not found")
		}
		return helperProject("P2", 200), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return proj2Issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"FAIL", "P2"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() should not return error on partial failure, got: %v", err)
	}

	foundWarning := false
	for _, w := range env.Warnings {
		if w.Code == "project_fetch_failed" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Warnings does not contain code 'project_fetch_failed'")
	}

	result := env.Analysis.(*BlockerResult)
	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}
}

// T11: TestBlockerDetector_Detect_LLMHints は LLMHints の生成を検証する。
func TestBlockerDetector_Detect_LLMHints(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-3 * 24 * time.Hour)

	issues := helperBlockerIssues(fixedNow, []blockerIssueSpec{
		{issueKey: "PROJ-1", summary: "期限超過", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5, dueDate: &pastDue},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	// PrimaryEntities に project:PROJ が含まれる
	foundProject := false
	for _, e := range result.LLMHints.PrimaryEntities {
		if e == "project:PROJ" {
			foundProject = true
			break
		}
	}
	if !foundProject {
		t.Errorf("LLMHints.PrimaryEntities does not contain %q, got %v", "project:PROJ", result.LLMHints.PrimaryEntities)
	}

	// open_questions に件数情報が含まれる
	if len(result.LLMHints.OpenQuestions) == 0 {
		t.Error("LLMHints.OpenQuestions is empty, want at least one entry")
	}
}

// T12: TestBlockerDetector_Detect_EmptyProjectKeys は空プロジェクトキー → 空結果を検証する。
func TestBlockerDetector_Detect_EmptyProjectKeys(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()

	detector := NewBlockerDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{}, BlockerConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*BlockerResult)

	if result.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.TotalCount)
	}
	if result.Issues == nil {
		t.Error("Issues = nil, want empty slice")
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues length = %d, want 0", len(result.Issues))
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings length = %d, want 0", len(env.Warnings))
	}
}
