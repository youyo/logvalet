package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// helperProject はテスト用の domain.Project を返すヘルパー。
func helperProject(key string, id int) *domain.Project {
	return &domain.Project{
		ID:         id,
		ProjectKey: key,
		Name:       "テストプロジェクト " + key,
	}
}

// helperStaleIssues はテスト用の課題スライスを生成するヘルパー。
// daysAgo は各課題の Updated が now から何日前かを指定する。
func helperStaleIssues(now time.Time, specs []staleIssueSpec) []domain.Issue {
	issues := make([]domain.Issue, len(specs))
	for i, s := range specs {
		updated := now.Add(-time.Duration(s.daysAgo) * 24 * time.Hour)
		issue := domain.Issue{
			ID:        1000 + i,
			ProjectID: s.projectID,
			IssueKey:  s.issueKey,
			Summary:   s.summary,
			Updated:   &updated,
		}
		if s.statusName != "" {
			issue.Status = &domain.IDName{ID: s.statusID, Name: s.statusName}
		}
		if s.assigneeName != "" {
			issue.Assignee = &domain.User{ID: s.assigneeID, Name: s.assigneeName}
		}
		if s.dueDate != nil {
			issue.DueDate = s.dueDate
		}
		if s.updatedNil {
			issue.Updated = nil
		}
		issues[i] = issue
	}
	return issues
}

type staleIssueSpec struct {
	issueKey     string
	summary      string
	projectID    int
	statusID     int
	statusName   string
	assigneeID   int
	assigneeName string
	daysAgo      int
	dueDate      *time.Time
	updatedNil   bool
}

// T1: TestStaleDetector_Detect_FiltersStaleOnly は stale な課題のみを抽出することを検証する。
func TestStaleDetector_Detect_FiltersStaleOnly(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "stale課題1", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10},
		{issueKey: "PROJ-2", summary: "stale課題2", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 8},
		{issueKey: "PROJ-3", summary: "fresh課題1", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 3},
		{issueKey: "PROJ-4", summary: "fresh課題2", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 1},
		{issueKey: "PROJ-5", summary: "fresh課題3", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if env.Resource != "stale_issues" {
		t.Errorf("Resource = %q, want %q", env.Resource, "stale_issues")
	}

	result, ok := env.Analysis.(*StaleIssueResult)
	if !ok {
		t.Fatalf("Analysis type = %T, want *StaleIssueResult", env.Analysis)
	}

	if result.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", result.TotalCount)
	}
	if len(result.Issues) != 2 {
		t.Errorf("Issues length = %d, want 2", len(result.Issues))
	}
	if result.ThresholdDays != 7 {
		t.Errorf("ThresholdDays = %d, want 7", result.ThresholdDays)
	}

	// DaysSinceUpdate 降順ソート確認
	if len(result.Issues) >= 2 {
		if result.Issues[0].DaysSinceUpdate < result.Issues[1].DaysSinceUpdate {
			t.Errorf("Issues not sorted by DaysSinceUpdate desc: %d < %d",
				result.Issues[0].DaysSinceUpdate, result.Issues[1].DaysSinceUpdate)
		}
	}
}

// T2: TestStaleDetector_Detect_ExcludeStatus は ExcludeStatus で除外されることを検証する。
func TestStaleDetector_Detect_ExcludeStatus(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "stale完了", projectID: 100, statusName: "完了", statusID: 4, daysAgo: 10},
		{issueKey: "PROJ-2", summary: "stale未対応", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10},
		{issueKey: "PROJ-3", summary: "fresh", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 2},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{
		ExcludeStatus: []string{"完了"},
	})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].IssueKey != "PROJ-2" {
		t.Errorf("Issues[0].IssueKey = %q, want %q", result.Issues[0].IssueKey, "PROJ-2")
	}
}

// T3: TestStaleDetector_Detect_StatusDays はステータス別閾値を検証する。
func TestStaleDetector_Detect_StatusDays(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "処理中4日前", projectID: 100, statusName: "処理中", statusID: 2, daysAgo: 4},
		{issueKey: "PROJ-2", summary: "未対応4日前", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 4},
		{issueKey: "PROJ-3", summary: "fresh", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 1},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{
		StatusDays: map[string]int{"処理中": 3},
	})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].IssueKey != "PROJ-1" {
		t.Errorf("Issues[0].IssueKey = %q, want %q", result.Issues[0].IssueKey, "PROJ-1")
	}
}

// T4: TestStaleDetector_Detect_NoStaleIssues は stale 課題がない場合を検証する。
func TestStaleDetector_Detect_NoStaleIssues(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "fresh1", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 1},
		{issueKey: "PROJ-2", summary: "fresh2", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 3},
		{issueKey: "PROJ-3", summary: "fresh3", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

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

// T5: TestStaleDetector_Detect_Overdue は IsOverdue の判定を検証する。
func TestStaleDetector_Detect_Overdue(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-2 * 24 * time.Hour)
	futureDue := fixedNow.Add(5 * 24 * time.Hour)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "stale+overdue", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10, dueDate: &pastDue},
		{issueKey: "PROJ-2", summary: "stale+not-overdue", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 8, dueDate: &futureDue},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if len(result.Issues) != 2 {
		t.Fatalf("Issues length = %d, want 2", len(result.Issues))
	}

	// DaysSinceUpdate 降順なので PROJ-1(10日) が先
	overdueFound := false
	notOverdueFound := false
	for _, si := range result.Issues {
		if si.IssueKey == "PROJ-1" {
			if !si.IsOverdue {
				t.Errorf("PROJ-1 IsOverdue = false, want true")
			}
			overdueFound = true
		}
		if si.IssueKey == "PROJ-2" {
			if si.IsOverdue {
				t.Errorf("PROJ-2 IsOverdue = true, want false")
			}
			notOverdueFound = true
		}
	}
	if !overdueFound {
		t.Error("PROJ-1 not found in results")
	}
	if !notOverdueFound {
		t.Error("PROJ-2 not found in results")
	}
}

// T6: TestStaleDetector_Detect_MultiProject は複数プロジェクトの統合を検証する。
func TestStaleDetector_Detect_MultiProject(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	proj1Issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "P1-1", summary: "P1 stale", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10},
	})
	proj2Issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "P2-1", summary: "P2 stale", projectID: 200, statusName: "未対応", statusID: 1, daysAgo: 9},
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

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"P1", "P2"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if result.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", result.TotalCount)
	}
	if len(result.Issues) != 2 {
		t.Errorf("Issues length = %d, want 2", len(result.Issues))
	}

	// 両プロジェクトの課題が含まれていることを確認
	keys := map[string]bool{}
	for _, si := range result.Issues {
		keys[si.IssueKey] = true
	}
	if !keys["P1-1"] {
		t.Error("P1-1 not found in results")
	}
	if !keys["P2-1"] {
		t.Error("P2-1 not found in results")
	}
}

// T7: TestStaleDetector_Detect_LLMHints は LLMHints の生成を検証する。
func TestStaleDetector_Detect_LLMHints(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "stale1", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10},
		{issueKey: "PROJ-2", summary: "stale2", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 8},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	// primary_entities に "project:PROJ" が含まれる
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

	// open_questions に件数に関する情報が含まれる
	if len(result.LLMHints.OpenQuestions) == 0 {
		t.Error("LLMHints.OpenQuestions is empty, want at least one entry")
	}
}

// T8: TestStaleDetector_Detect_ProjectError は部分失敗（GetProject エラー）を検証する。
func TestStaleDetector_Detect_ProjectError(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	staleIssues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "P2-1", summary: "stale", projectID: 200, statusName: "未対応", statusID: 1, daysAgo: 10},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		if projectKey == "FAIL" {
			return nil, errors.New("API error: project not found")
		}
		return helperProject("P2", 200), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return staleIssues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"FAIL", "P2"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() should not return error on partial failure, got: %v", err)
	}

	// warnings に project_fetch_failed が含まれる
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

	// 成功したプロジェクトの結果は返る
	result := env.Analysis.(*StaleIssueResult)
	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}
}

// T9: TestStaleDetector_Detect_AllProjectsError は全プロジェクトがエラーの場合を検証する。
func TestStaleDetector_Detect_AllProjectsError(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return nil, errors.New("API error")
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"P1", "P2"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() should not return error even on all failures, got: %v", err)
	}

	if len(env.Warnings) != 2 {
		t.Errorf("Warnings length = %d, want 2", len(env.Warnings))
	}

	result := env.Analysis.(*StaleIssueResult)
	if result.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.TotalCount)
	}
	if result.Issues == nil {
		t.Error("Issues = nil, want empty slice")
	}
}

// T10: TestStaleDetector_Detect_EmptyProjectKeys は空の projectKeys を検証する。
func TestStaleDetector_Detect_EmptyProjectKeys(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if result.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.TotalCount)
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues length = %d, want 0", len(result.Issues))
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings length = %d, want 0", len(env.Warnings))
	}
}

// T11: TestStaleDetector_Detect_NilUpdated は Updated=nil の課題がスキップされることを検証する。
func TestStaleDetector_Detect_NilUpdated(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "nil updated", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 0, updatedNil: true},
		{issueKey: "PROJ-2", summary: "stale", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1 (nil updated should be skipped)", result.TotalCount)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].IssueKey != "PROJ-2" {
		t.Errorf("Issues[0].IssueKey = %q, want %q", result.Issues[0].IssueKey, "PROJ-2")
	}
}

// T12: TestStaleDetector_Detect_DefaultDaysZero は DefaultDays=0 が DefaultStaleDays にフォールバックすることを検証する。
func TestStaleDetector_Detect_DefaultDaysZero(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "stale", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 8},
		{issueKey: "PROJ-2", summary: "fresh", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 5},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{DefaultDays: 0})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	// DefaultDays=0 → DefaultStaleDays(7) にフォールバック
	if result.ThresholdDays != DefaultStaleDays {
		t.Errorf("ThresholdDays = %d, want %d (DefaultStaleDays)", result.ThresholdDays, DefaultStaleDays)
	}
	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}
}

// T13: TestStaleDetector_Detect_NilStatus は Status=nil の課題が DefaultDays で判定されることを検証する。
func TestStaleDetector_Detect_NilStatus(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "nil status stale", projectID: 100, statusName: "", statusID: 0, daysAgo: 10},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{
		ExcludeStatus: []string{"完了"},
		StatusDays:    map[string]int{"処理中": 3},
	})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	// Status=nil でも DefaultDays(7) で stale 判定される
	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].DaysSinceUpdate != 10 {
		t.Errorf("DaysSinceUpdate = %d, want 10", result.Issues[0].DaysSinceUpdate)
	}
}

// TestStaleIssueDetector_DefaultExcludeStatus は ExcludeStatus 未指定時に「完了」課題が除外されることを検証する。
func TestStaleIssueDetector_DefaultExcludeStatus(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperStaleIssues(fixedNow, []staleIssueSpec{
		{issueKey: "PROJ-1", summary: "stale未対応", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 10},
		{issueKey: "PROJ-2", summary: "stale完了", projectID: 100, statusName: "完了", statusID: 4, daysAgo: 10},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	detector := NewStaleIssueDetector(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	// ExcludeStatus を指定しない → デフォルトで「完了」が除外される
	env, err := detector.Detect(context.Background(), []string{"PROJ"}, StaleConfig{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	result := env.Analysis.(*StaleIssueResult)

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1 (完了 excluded by default)", result.TotalCount)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("Issues length = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].IssueKey != "PROJ-1" {
		t.Errorf("Issues[0].IssueKey = %q, want PROJ-1 (PROJ-2 の完了 is excluded)", result.Issues[0].IssueKey)
	}
}
