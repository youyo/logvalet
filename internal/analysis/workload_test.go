package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// helperWorkloadIssues はワークロードテスト用の課題スライスを生成するヘルパー。
func helperWorkloadIssues(now time.Time, specs []workloadIssueSpec) []domain.Issue {
	issues := make([]domain.Issue, len(specs))
	for i, s := range specs {
		updated := now.Add(-time.Duration(s.daysAgo) * 24 * time.Hour)
		issue := domain.Issue{
			ID:        2000 + i,
			ProjectID: s.projectID,
			IssueKey:  s.issueKey,
			Summary:   s.summary,
			Updated:   &updated,
		}
		if s.statusName != "" {
			issue.Status = &domain.IDName{ID: s.statusID, Name: s.statusName}
		}
		if s.priorityName != "" {
			issue.Priority = &domain.IDName{ID: s.priorityID, Name: s.priorityName}
		}
		if s.assigneeID > 0 || s.assigneeName != "" {
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

type workloadIssueSpec struct {
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

// T1: TestWorkloadCalculator_Calculate_Basic は基本的なワークロード集計を検証する。
func TestWorkloadCalculator_Calculate_Basic(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		{issueKey: "PROJ-1", summary: "課題1", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "高", priorityID: 2, assigneeID: 10, assigneeName: "Alice", daysAgo: 1},
		{issueKey: "PROJ-2", summary: "課題2", projectID: 100, statusName: "処理中", statusID: 2, priorityName: "中", priorityID: 3, assigneeID: 10, assigneeName: "Alice", daysAgo: 2},
		{issueKey: "PROJ-3", summary: "課題3", projectID: 100, statusName: "未対応", statusID: 1, priorityName: "低", priorityID: 4, assigneeID: 20, assigneeName: "Bob", daysAgo: 1},
		{issueKey: "PROJ-4", summary: "課題4", projectID: 100, statusName: "処理中", statusID: 2, priorityName: "高", priorityID: 2, assigneeID: 20, assigneeName: "Bob", daysAgo: 3},
		{issueKey: "PROJ-5", summary: "課題5", projectID: 100, statusName: "完了", statusID: 3, priorityName: "中", priorityID: 3, assigneeID: 10, assigneeName: "Alice", daysAgo: 5},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	if env.Resource != "user_workload" {
		t.Errorf("Resource = %q, want %q", env.Resource, "user_workload")
	}

	result, ok := env.Analysis.(*WorkloadResult)
	if !ok {
		t.Fatalf("Analysis type = %T, want *WorkloadResult", env.Analysis)
	}

	if result.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey = %q, want %q", result.ProjectKey, "PROJ")
	}
	// デフォルトで「完了」ステータスが除外されるため 4 件（PROJ-5 の「完了」除外）
	if result.TotalIssues != 4 {
		t.Errorf("TotalIssues = %d, want 4 (完了 is excluded by default)", result.TotalIssues)
	}
	if result.UnassignedCount != 0 {
		t.Errorf("UnassignedCount = %d, want 0", result.UnassignedCount)
	}
	if len(result.Members) != 2 {
		t.Errorf("Members length = %d, want 2", len(result.Members))
	}

	// UserID 昇順ソート確認
	if result.Members[0].UserID != 10 {
		t.Errorf("Members[0].UserID = %d, want 10 (Alice)", result.Members[0].UserID)
	}
	if result.Members[1].UserID != 20 {
		t.Errorf("Members[1].UserID = %d, want 20 (Bob)", result.Members[1].UserID)
	}

	// Alice: 未対応(PROJ-1) + 処理中(PROJ-2) = 2件（完了のPROJ-5 は除外）
	alice := result.Members[0]
	if alice.Total != 2 {
		t.Errorf("Alice.Total = %d, want 2 (完了 excluded by default)", alice.Total)
	}
	if alice.ByStatus["未対応"] != 1 {
		t.Errorf("Alice.ByStatus[未対応] = %d, want 1", alice.ByStatus["未対応"])
	}
	if alice.ByStatus["処理中"] != 1 {
		t.Errorf("Alice.ByStatus[処理中] = %d, want 1", alice.ByStatus["処理中"])
	}
	if alice.ByStatus["完了"] != 0 {
		t.Errorf("Alice.ByStatus[完了] = %d, want 0 (excluded by default)", alice.ByStatus["完了"])
	}
	if alice.ByPriority["高"] != 1 {
		t.Errorf("Alice.ByPriority[高] = %d, want 1", alice.ByPriority["高"])
	}

	bob := result.Members[1]
	if bob.Total != 2 {
		t.Errorf("Bob.Total = %d, want 2", bob.Total)
	}
}

// T2: TestWorkloadCalculator_Calculate_Unassigned は担当者なし課題のカウントを検証する。
func TestWorkloadCalculator_Calculate_Unassigned(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		{issueKey: "PROJ-1", summary: "担当あり", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 1},
		{issueKey: "PROJ-2", summary: "担当なし1", projectID: 100, statusName: "未対応", statusID: 1, daysAgo: 2},
		{issueKey: "PROJ-3", summary: "担当なし2", projectID: 100, statusName: "処理中", statusID: 2, daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)

	if result.UnassignedCount != 2 {
		t.Errorf("UnassignedCount = %d, want 2", result.UnassignedCount)
	}
	if len(result.Members) != 1 {
		t.Errorf("Members length = %d, want 1 (only Alice)", len(result.Members))
	}
}

// T3: TestWorkloadCalculator_Calculate_ExcludeStatus は完了ステータス除外を検証する。
func TestWorkloadCalculator_Calculate_ExcludeStatus(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		{issueKey: "PROJ-1", summary: "未対応", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 1},
		{issueKey: "PROJ-2", summary: "完了", projectID: 100, statusName: "完了", statusID: 3, assigneeID: 10, assigneeName: "Alice", daysAgo: 2},
		{issueKey: "PROJ-3", summary: "処理中", projectID: 100, statusName: "処理中", statusID: 2, assigneeID: 10, assigneeName: "Alice", daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{
		ExcludeStatus: []string{"完了"},
	})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)

	// 完了を除外するので TotalIssues は 2
	if result.TotalIssues != 2 {
		t.Errorf("TotalIssues = %d, want 2", result.TotalIssues)
	}
	if result.Members[0].Total != 2 {
		t.Errorf("Alice.Total = %d, want 2 (完了除外)", result.Members[0].Total)
	}
	// ByStatus に "完了" が含まれない
	if _, ok := result.Members[0].ByStatus["完了"]; ok {
		t.Error("Alice.ByStatus should not contain '完了' (excluded)")
	}
}

// T4: TestWorkloadCalculator_Calculate_OverdueAndStale は期限超過・停滞の検出を検証する。
func TestWorkloadCalculator_Calculate_OverdueAndStale(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-2 * 24 * time.Hour)
	futureDue := fixedNow.Add(5 * 24 * time.Hour)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		// overdue + stale（10日前更新、期限2日前）
		{issueKey: "PROJ-1", summary: "overdue+stale", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 10, dueDate: &pastDue},
		// overdue のみ（2日前更新、期限2日前）
		{issueKey: "PROJ-2", summary: "overdue-only", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 2, dueDate: &pastDue},
		// stale のみ（10日前更新、期限未来）
		{issueKey: "PROJ-3", summary: "stale-only", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 10, dueDate: &futureDue},
		// normal（2日前更新、期限未来）
		{issueKey: "PROJ-4", summary: "normal", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 2, dueDate: &futureDue},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{StaleDays: 7})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)
	alice := result.Members[0]

	// Overdue: PROJ-1, PROJ-2 → 2件
	if alice.Overdue != 2 {
		t.Errorf("Alice.Overdue = %d, want 2", alice.Overdue)
	}
	// Stale: PROJ-1, PROJ-3（10日前更新、閾値7日）→ 2件
	if alice.Stale != 2 {
		t.Errorf("Alice.Stale = %d, want 2", alice.Stale)
	}
}

// T5: TestWorkloadCalculator_Calculate_LoadLevel は負荷レベル判定を検証する（境界値）。
func TestWorkloadCalculator_Calculate_LoadLevel(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		issueCount    int
		expectedLevel string
	}{
		{"low (0)", 0, "low"},
		{"low (4)", 4, "low"},
		{"medium (5)", 5, "medium"},
		{"medium (9)", 9, "medium"},
		{"high (10)", 10, "high"},
		{"high (19)", 19, "high"},
		{"overloaded (20)", 20, "overloaded"},
		{"overloaded (25)", 25, "overloaded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := make([]workloadIssueSpec, tt.issueCount)
			for i := range specs {
				specs[i] = workloadIssueSpec{
					issueKey:     "PROJ-" + string(rune('A'+i)),
					summary:      "課題",
					projectID:    100,
					statusName:   "未対応",
					statusID:     1,
					assigneeID:   10,
					assigneeName: "Alice",
					daysAgo:      1,
				}
			}
			issues := helperWorkloadIssues(fixedNow, specs)

			mc := backlog.NewMockClient()
			mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
				return helperProject("PROJ", 100), nil
			}
			mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
				return issues, nil
			}

			calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
				WithClock(func() time.Time { return fixedNow }))

			env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
			if err != nil {
				t.Fatalf("Calculate() error: %v", err)
			}

			result := env.Analysis.(*WorkloadResult)

			if tt.issueCount == 0 {
				// 課題なし → Members は空
				if len(result.Members) != 0 {
					t.Errorf("Members length = %d, want 0", len(result.Members))
				}
				return
			}

			if len(result.Members) != 1 {
				t.Fatalf("Members length = %d, want 1", len(result.Members))
			}
			alice := result.Members[0]
			if alice.LoadLevel != tt.expectedLevel {
				t.Errorf("LoadLevel = %q, want %q (total=%d)", alice.LoadLevel, tt.expectedLevel, alice.Total)
			}
		})
	}
}

// T6: TestWorkloadCalculator_Calculate_CustomThresholds はカスタム閾値での負荷レベル判定を検証する。
func TestWorkloadCalculator_Calculate_CustomThresholds(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	specs := make([]workloadIssueSpec, 3)
	for i := range specs {
		specs[i] = workloadIssueSpec{
			issueKey:     "PROJ-" + string(rune('A'+i)),
			summary:      "課題",
			projectID:    100,
			statusName:   "未対応",
			statusID:     1,
			assigneeID:   10,
			assigneeName: "Alice",
			daysAgo:      1,
		}
	}
	issues := helperWorkloadIssues(fixedNow, specs)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	// カスタム閾値: medium=2, high=3, overloaded=5
	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{
		MediumThreshold:     2,
		HighThreshold:       3,
		OverloadedThreshold: 5,
	})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)
	// Alice の total=3 → high（閾値3）
	if result.Members[0].LoadLevel != "high" {
		t.Errorf("LoadLevel = %q, want %q", result.Members[0].LoadLevel, "high")
	}
}

// T7: TestWorkloadCalculator_Calculate_ProjectFetchError はプロジェクト取得失敗時の warning を検証する。
func TestWorkloadCalculator_Calculate_ProjectFetchError(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return nil, errors.New("project not found")
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() should not return error on project fetch failure, got: %v", err)
	}

	// warning が含まれる
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

	result := env.Analysis.(*WorkloadResult)
	if result.TotalIssues != 0 {
		t.Errorf("TotalIssues = %d, want 0", result.TotalIssues)
	}
	if len(result.Members) != 0 {
		t.Errorf("Members length = %d, want 0", len(result.Members))
	}
}

// T8: TestWorkloadCalculator_Calculate_IssuesFetchError は課題取得失敗時の warning を検証する。
func TestWorkloadCalculator_Calculate_IssuesFetchError(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return nil, errors.New("API error: list issues failed")
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() should not return error on issues fetch failure, got: %v", err)
	}

	// warning が含まれる
	foundWarning := false
	for _, w := range env.Warnings {
		if w.Code == "issues_fetch_failed" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Warnings does not contain code 'issues_fetch_failed'")
	}

	result := env.Analysis.(*WorkloadResult)
	if result.TotalIssues != 0 {
		t.Errorf("TotalIssues = %d, want 0", result.TotalIssues)
	}
}

// T9: TestWorkloadCalculator_Calculate_EmptyProject は課題0件のプロジェクトを検証する。
func TestWorkloadCalculator_Calculate_EmptyProject(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)

	if result.TotalIssues != 0 {
		t.Errorf("TotalIssues = %d, want 0", result.TotalIssues)
	}
	if result.Members == nil {
		t.Error("Members = nil, want empty slice")
	}
	if len(result.Members) != 0 {
		t.Errorf("Members length = %d, want 0", len(result.Members))
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings length = %d, want 0", len(env.Warnings))
	}
}

// T10: TestWorkloadCalculator_Calculate_LLMHints は LLMHints の生成を検証する。
func TestWorkloadCalculator_Calculate_LLMHints(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	pastDue := fixedNow.Add(-2 * 24 * time.Hour)

	specs := make([]workloadIssueSpec, 10)
	for i := range specs {
		specs[i] = workloadIssueSpec{
			issueKey:     "PROJ-" + string(rune('A'+i)),
			summary:      "課題",
			projectID:    100,
			statusName:   "処理中",
			statusID:     2,
			assigneeID:   10,
			assigneeName: "Alice",
			daysAgo:      1,
		}
	}
	// 1件を overdue にする
	specs[0].dueDate = &pastDue

	issues := helperWorkloadIssues(fixedNow, specs)

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)

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

	// open_questions に high/overloaded メンバーに関する情報が含まれる（Alice は total=10 → high）
	if len(result.LLMHints.OpenQuestions) == 0 {
		t.Error("LLMHints.OpenQuestions is empty, want at least one entry")
	}
}

// T11: TestWorkloadCalculator_Calculate_StaleDaysDefault は StaleDays=0 が DefaultStaleDays にフォールバックすることを検証する。
func TestWorkloadCalculator_Calculate_StaleDaysDefault(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		{issueKey: "PROJ-1", summary: "stale", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 8},
		{issueKey: "PROJ-2", summary: "fresh", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 3},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	// StaleDays=0 → DefaultStaleDays(7) にフォールバック
	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{StaleDays: 0})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)
	if result.StaleDays != DefaultStaleDays {
		t.Errorf("StaleDays = %d, want %d (DefaultStaleDays)", result.StaleDays, DefaultStaleDays)
	}
	alice := result.Members[0]
	// 8日前更新 → stale (閾値7日)
	if alice.Stale != 1 {
		t.Errorf("Alice.Stale = %d, want 1", alice.Stale)
	}
}

// T12: TestWorkloadCalculator_Calculate_NilUpdatedStale は Updated=nil の課題が stale カウントされないことを検証する。
func TestWorkloadCalculator_Calculate_NilUpdatedStale(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		{issueKey: "PROJ-1", summary: "nil updated", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", updatedNil: true},
		{issueKey: "PROJ-2", summary: "stale", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 10},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)
	alice := result.Members[0]
	// Updated=nil の課題は stale 判定しない
	if alice.Stale != 1 {
		t.Errorf("Alice.Stale = %d, want 1 (nil updated should not be counted as stale)", alice.Stale)
	}
	// TotalIssues は2件（除外なし）
	if result.TotalIssues != 2 {
		t.Errorf("TotalIssues = %d, want 2", result.TotalIssues)
	}
}

// TestWorkloadCalculator_DefaultExcludeStatus は ExcludeStatus 未指定時に「完了」課題が除外されることを検証する。
func TestWorkloadCalculator_DefaultExcludeStatus(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	issues := helperWorkloadIssues(fixedNow, []workloadIssueSpec{
		{issueKey: "PROJ-1", summary: "未対応", projectID: 100, statusName: "未対応", statusID: 1, assigneeID: 10, assigneeName: "Alice", daysAgo: 1},
		{issueKey: "PROJ-2", summary: "完了", projectID: 100, statusName: "完了", statusID: 4, assigneeID: 10, assigneeName: "Alice", daysAgo: 5},
	})

	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return helperProject("PROJ", 100), nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	calc := NewWorkloadCalculator(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }))

	// ExcludeStatus を指定しない → デフォルトで「完了」が除外される
	env, err := calc.Calculate(context.Background(), "PROJ", WorkloadConfig{})
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	result := env.Analysis.(*WorkloadResult)

	// PROJ-2 の「完了」は除外されるため TotalIssues = 1
	if result.TotalIssues != 1 {
		t.Errorf("TotalIssues = %d, want 1 (完了 excluded by default)", result.TotalIssues)
	}
	if len(result.Members) != 1 {
		t.Fatalf("Members length = %d, want 1", len(result.Members))
	}
	alice := result.Members[0]
	if alice.Total != 1 {
		t.Errorf("Alice.Total = %d, want 1 (完了 excluded by default)", alice.Total)
	}
	if alice.ByStatus["完了"] != 0 {
		t.Errorf("Alice.ByStatus[完了] = %d, want 0 (excluded by default)", alice.ByStatus["完了"])
	}
}
