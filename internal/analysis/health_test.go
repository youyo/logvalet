package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ---- ヘルパー ----

// helperHealthIssue はテスト用の domain.Issue を生成するヘルパー。
func helperHealthIssue(
	issueKey string,
	projectID int,
	updatedDaysAgo int,
	statusName string,
	priorityName string,
	assignee *domain.User,
	dueDate *time.Time,
	now time.Time,
) domain.Issue {
	updated := now.Add(-time.Duration(updatedDaysAgo) * 24 * time.Hour)
	issue := domain.Issue{
		ID:        9000,
		ProjectID: projectID,
		IssueKey:  issueKey,
		Summary:   "テスト課題 " + issueKey,
		Updated:   &updated,
		Assignee:  assignee,
		DueDate:   dueDate,
	}
	if statusName != "" {
		issue.Status = &domain.IDName{ID: 1, Name: statusName}
	}
	if priorityName != "" {
		issue.Priority = &domain.IDName{ID: 2, Name: priorityName}
	}
	return issue
}

// ---- T01: 正常系 — stale/blocker/workload が全て返る ----

func TestProjectHealthBuilder_Build_Normal(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	pastDue := now.Add(-2 * 24 * time.Hour)
	userA := &domain.User{ID: 10, Name: "A"}

	issues := []domain.Issue{
		helperHealthIssue("PROJ-1", 1, 10, "処理中", "", userA, nil, now),      // stale + blocker(long_in_progress)
		helperHealthIssue("PROJ-2", 1, 1, "未対応", "", nil, nil, now),         // 正常
		helperHealthIssue("PROJ-3", 1, 1, "未対応", "", userA, &pastDue, now),  // overdue → blocker HIGH
	}

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "PROJ", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if env.Resource != "project_health" {
		t.Errorf("Resource = %q, want %q", env.Resource, "project_health")
	}

	result, ok := env.Analysis.(*ProjectHealthResult)
	if !ok {
		t.Fatalf("Analysis type = %T, want *ProjectHealthResult", env.Analysis)
	}

	if result.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey = %q, want %q", result.ProjectKey, "PROJ")
	}

	if result.StaleSummary.TotalCount != 1 {
		t.Errorf("StaleSummary.TotalCount = %d, want 1", result.StaleSummary.TotalCount)
	}

	if result.BlockerSummary.HighCount != 1 {
		t.Errorf("BlockerSummary.HighCount = %d, want 1", result.BlockerSummary.HighCount)
	}

	if result.HealthLevel != "warning" && result.HealthLevel != "critical" {
		t.Errorf("HealthLevel = %q, want warning or critical", result.HealthLevel)
	}

	if len(env.Warnings) != 0 {
		t.Errorf("Warnings = %v, want empty", env.Warnings)
	}
}

// ---- T02: 正常系 — 課題ゼロのプロジェクト（health_score == 100）----

func TestProjectHealthBuilder_Build_EmptyProject(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 2, ProjectKey: "EMPTY", Name: "空プロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "EMPTY", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*ProjectHealthResult)

	if result.StaleSummary.TotalCount != 0 {
		t.Errorf("StaleSummary.TotalCount = %d, want 0", result.StaleSummary.TotalCount)
	}
	if result.BlockerSummary.TotalCount != 0 {
		t.Errorf("BlockerSummary.TotalCount = %d, want 0", result.BlockerSummary.TotalCount)
	}
	if result.WorkloadSummary.TotalIssues != 0 {
		t.Errorf("WorkloadSummary.TotalIssues = %d, want 0", result.WorkloadSummary.TotalIssues)
	}
	if result.HealthScore != 100 {
		t.Errorf("HealthScore = %d, want 100", result.HealthScore)
	}
	if result.HealthLevel != "healthy" {
		t.Errorf("HealthLevel = %q, want %q", result.HealthLevel, "healthy")
	}
}

// ---- T03: 正常系 — health_score の減点計算 ----
// stale×2, blocker(HIGH)×1, overloaded×1
// 期待: 100 - (2*5) - (1*10) - (1*8) = 72 → "warning"

func TestProjectHealthBuilder_Build_HealthScoreCalculation(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	// overloaded: DefaultOverloadedThreshold=20 なので 20+ 課題担当が必要
	// 簡単にするために WorkloadConfig.OverloadedThreshold=3 を設定
	// stale×2: daysAgo=10 で DefaultStaleDays=7 を超える課題
	// blocker(HIGH)×1: overdue_open シグナルの課題
	userB := &domain.User{ID: 20, Name: "B"}
	pastDue := now.Add(-1 * 24 * time.Hour)

	issues := []domain.Issue{}
	// stale×2（daysAgo=10）— unassigned ratio を 20% 以下に抑えるため担当者を設定
	userA := &domain.User{ID: 10, Name: "A"}
	for i := 0; i < 2; i++ {
		issues = append(issues, helperHealthIssue("PROJ-100", 1, 10, "未対応", "", userA, nil, now))
	}
	issues[0].IssueKey = "PROJ-100"
	issues[1].IssueKey = "PROJ-101"

	// blocker HIGH × 1（overdue）
	overdueIssue := helperHealthIssue("PROJ-200", 1, 1, "未対応", "", userB, &pastDue, now)
	issues = append(issues, overdueIssue)

	// overloaded member: userB に OverloadedThreshold=3 以上の課題を割り当て
	for i := 0; i < 3; i++ {
		extra := helperHealthIssue("PROJ-300", 1, 1, "未対応", "", userB, nil, now)
		extra.IssueKey = "PROJ-30" + string(rune('0'+i))
		issues = append(issues, extra)
	}

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	cfg := ProjectHealthConfig{
		WorkloadConfig: WorkloadConfig{
			OverloadedThreshold: 3, // userB は 4 課題なので overloaded
		},
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "PROJ", cfg)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*ProjectHealthResult)

	// stale=2, blockerHigh=1, overloaded=1
	// 100 - 2*5 - 1*10 - 1*8 = 72
	expectedScore := 72
	if result.HealthScore != expectedScore {
		t.Errorf("HealthScore = %d, want %d (stale×2 -10, blockerHigh×1 -10, overloaded×1 -8)", result.HealthScore, expectedScore)
	}
	if result.HealthLevel != "warning" {
		t.Errorf("HealthLevel = %q, want %q", result.HealthLevel, "warning")
	}
}

// ---- T04: 異常系 — GetProject 失敗 ----

func TestProjectHealthBuilder_Build_GetProjectFailed(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return nil, errors.New("API error")
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "FAIL", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() should not return error, got: %v", err)
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
		t.Errorf("Warnings does not contain project_fetch_failed, got: %v", env.Warnings)
	}

	result := env.Analysis.(*ProjectHealthResult)
	if result.HealthScore != 0 {
		t.Errorf("HealthScore = %d, want 0 on failure", result.HealthScore)
	}
	if result.HealthLevel != "critical" {
		t.Errorf("HealthLevel = %q, want %q", result.HealthLevel, "critical")
	}
}

// ---- T05: 異常系 — ListIssues 失敗 ----

func TestProjectHealthBuilder_Build_ListIssuesFailed(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return nil, errors.New("API error: list issues failed")
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "PROJ", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() should not return error, got: %v", err)
	}

	// warnings に issues_fetch_failed が含まれる
	foundWarning := false
	for _, w := range env.Warnings {
		if w.Code == "issues_fetch_failed" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Warnings does not contain issues_fetch_failed, got: %v", env.Warnings)
	}

	result := env.Analysis.(*ProjectHealthResult)
	if result.StaleSummary.TotalCount != 0 {
		t.Errorf("StaleSummary.TotalCount = %d, want 0", result.StaleSummary.TotalCount)
	}
}

// ---- T06: WithClock オプション — 時刻注入 ----

func TestProjectHealthBuilder_Build_WithClock(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	// Updated = 2026-01-01 → daysSince=9 >= 7 → stale
	updated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	issue := domain.Issue{
		ID:        1,
		ProjectID: 1,
		IssueKey:  "PROJ-1",
		Summary:   "stale",
		Updated:   &updated,
		Status:    &domain.IDName{ID: 1, Name: "未対応"},
	}

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{issue}, nil
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "PROJ", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*ProjectHealthResult)

	// 固定時刻 2026-01-10 で Updated=2026-01-01 は 9日経過 → stale
	if result.StaleSummary.TotalCount != 1 {
		t.Errorf("StaleSummary.TotalCount = %d, want 1 (clock injection test)", result.StaleSummary.TotalCount)
	}
}

// ---- T07: エッジケース — health_score が 0 を下回らない ----

func TestProjectHealthBuilder_Build_HealthScoreFloor(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	// stale×20 → 減点 100 → health_score = max(0, 0) = 0
	issues := make([]domain.Issue, 20)
	for i := range issues {
		issues[i] = helperHealthIssue("PROJ-999", 1, 10, "未対応", "", nil, nil, now)
		issues[i].IssueKey = "PROJ-" + string(rune('A'+i))
	}

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "PROJ", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*ProjectHealthResult)

	if result.HealthScore < 0 {
		t.Errorf("HealthScore = %d, want >= 0 (floor at 0)", result.HealthScore)
	}
	if result.HealthScore != 0 {
		t.Errorf("HealthScore = %d, want 0 (stale×20 = -100)", result.HealthScore)
	}
	if result.HealthLevel != "critical" {
		t.Errorf("HealthLevel = %q, want %q", result.HealthLevel, "critical")
	}
}

// ---- T08: エッジケース — unassigned_ratio > 20% ----

func TestProjectHealthBuilder_Build_UnassignedRatio(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	userC := &domain.User{ID: 30, Name: "C"}

	// total=10, unassigned=3 → 30% > 20% → -10
	issues := make([]domain.Issue, 10)
	for i := range issues {
		u := userC
		if i < 3 {
			u = nil // 最初の3件は unassigned
		}
		issues[i] = helperHealthIssue("PROJ-1", 1, 1, "未対応", "", u, nil, now)
		issues[i].IssueKey = "PROJ-U" + string(rune('0'+i))
	}

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := NewProjectHealthBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return now }),
	)

	env, err := builder.Build(context.Background(), "PROJ", ProjectHealthConfig{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*ProjectHealthResult)

	// unassigned 3/10=30% > 20% → -10 反映
	if result.WorkloadSummary.UnassignedCount != 3 {
		t.Errorf("WorkloadSummary.UnassignedCount = %d, want 3", result.WorkloadSummary.UnassignedCount)
	}

	// score = 100 - 10 = 90
	if result.HealthScore != 90 {
		t.Errorf("HealthScore = %d, want 90 (unassigned_ratio > 20%% = -10)", result.HealthScore)
	}
}

// ---- calcHealthScore 単体テスト ----

func TestCalcHealthScore(t *testing.T) {
	tests := []struct {
		name          string
		staleCount    int
		blockerHigh   int
		blockerMedium int
		overloaded    int
		unassigned    int
		total         int
		want          int
	}{
		{
			name:        "全て0",
			staleCount:  0,
			blockerHigh: 0,
			overloaded:  0,
			unassigned:  0,
			total:       0,
			want:        100,
		},
		{
			name:        "stale×2, blockerHigh×1, overloaded×1",
			staleCount:  2,
			blockerHigh: 1,
			overloaded:  1,
			total:       10,
			want:        72, // 100 - 10 - 10 - 8
		},
		{
			name:          "全て減点して0未満にならない",
			staleCount:    20,
			blockerHigh:   5,
			blockerMedium: 3,
			overloaded:    2,
			unassigned:    5,
			total:         10,
			want:          0, // max(0, ...)
		},
		{
			name:       "unassigned_ratio > 20%",
			unassigned: 3,
			total:      10,
			want:       90,
		},
		{
			name:       "unassigned_ratio = 20% (境界)",
			unassigned: 2,
			total:      10,
			want:       100, // 20% 以下は減点なし
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcHealthScore(tt.staleCount, tt.blockerHigh, tt.blockerMedium, tt.overloaded, tt.unassigned, tt.total)
			if got != tt.want {
				t.Errorf("calcHealthScore(%d, %d, %d, %d, %d, %d) = %d, want %d",
					tt.staleCount, tt.blockerHigh, tt.blockerMedium, tt.overloaded, tt.unassigned, tt.total, got, tt.want)
			}
		})
	}
}

// ---- calcHealthLevel 単体テスト ----

func TestCalcHealthLevel(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, "healthy"},
		{80, "healthy"},
		{79, "warning"},
		{60, "warning"},
		{59, "critical"},
		{0, "critical"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := calcHealthLevel(tt.score)
			if got != tt.want {
				t.Errorf("calcHealthLevel(%d) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}
