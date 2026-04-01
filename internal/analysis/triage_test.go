package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// fixedNowTriage はテスト用の固定時刻。
var fixedNowTriage = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

// helperTriageIssue はテスト用の対象課題を生成する。
func helperTriageIssue() *domain.Issue {
	created := fixedNowTriage.Add(-31 * 24 * time.Hour) // 31日前に作成
	updated := fixedNowTriage.Add(-4 * 24 * time.Hour)  // 4日前に更新
	due := fixedNowTriage.Add(29 * 24 * time.Hour)      // 29日後が期限

	return &domain.Issue{
		ID:        1,
		ProjectID: 100,
		IssueKey:  "PROJ-123",
		Summary:   "テスト課題",
		Description: "テスト説明",
		Status:    &domain.IDName{ID: 1, Name: "未対応"},
		Priority:  &domain.IDName{ID: 2, Name: "高"},
		IssueType: &domain.IDName{ID: 1, Name: "バグ"},
		Assignee: &domain.User{ID: 101, Name: "田中太郎"},
		Reporter: &domain.User{ID: 102, Name: "佐藤花子"},
		Categories: []domain.IDName{{ID: 10, Name: "フロントエンド"}},
		Milestones: []domain.IDName{{ID: 20, Name: "v1.0"}},
		DueDate:   &due,
		Created:   &created,
		Updated:   &updated,
	}
}

// helperProjectIssues はテスト用のプロジェクト課題一覧を生成する。
func helperProjectIssues(now time.Time) []domain.Issue {
	t1 := now.Add(-10 * 24 * time.Hour)
	t2 := now.Add(-20 * 24 * time.Hour)
	t3 := now.Add(-5 * 24 * time.Hour)
	t4 := now.Add(-30 * 24 * time.Hour)
	t5 := now.Add(-15 * 24 * time.Hour)

	c1 := now.Add(-50 * 24 * time.Hour)
	c2 := now.Add(-40 * 24 * time.Hour)
	c3 := now.Add(-60 * 24 * time.Hour)

	return []domain.Issue{
		{
			ID:         1,
			IssueKey:   "PROJ-123",
			Summary:    "対象課題",
			Status:     &domain.IDName{ID: 1, Name: "未対応"},
			Priority:   &domain.IDName{ID: 2, Name: "高"},
			Assignee:   &domain.User{ID: 101, Name: "田中太郎"},
			Categories: []domain.IDName{{ID: 10, Name: "フロントエンド"}},
			Milestones: []domain.IDName{{ID: 20, Name: "v1.0"}},
			Created:    &c1,
			Updated:    &t1,
		},
		{
			ID:         2,
			IssueKey:   "PROJ-124",
			Summary:    "処理中課題1",
			Status:     &domain.IDName{ID: 2, Name: "処理中"},
			Priority:   &domain.IDName{ID: 3, Name: "中"},
			Assignee:   &domain.User{ID: 101, Name: "田中太郎"},
			Categories: []domain.IDName{{ID: 10, Name: "フロントエンド"}},
			Milestones: []domain.IDName{{ID: 21, Name: "v1.1"}},
			Created:    &c2,
			Updated:    &t2,
		},
		{
			ID:         3,
			IssueKey:   "PROJ-125",
			Summary:    "完了課題1",
			Status:     &domain.IDName{ID: 3, Name: "完了"},
			Priority:   &domain.IDName{ID: 3, Name: "中"},
			Assignee:   &domain.User{ID: 102, Name: "佐藤花子"},
			Categories: []domain.IDName{{ID: 11, Name: "バックエンド"}},
			Milestones: []domain.IDName{{ID: 20, Name: "v1.0"}},
			Created:    &c3,
			Updated:    &t3,
		},
		{
			ID:         4,
			IssueKey:   "PROJ-126",
			Summary:    "未割当課題",
			Status:     &domain.IDName{ID: 1, Name: "未対応"},
			Priority:   &domain.IDName{ID: 4, Name: "低"},
			Assignee:   nil, // 未割当
			Categories: []domain.IDName{{ID: 10, Name: "フロントエンド"}},
			Milestones: []domain.IDName{},
			Created:    &c1,
			Updated:    &t4,
		},
		{
			ID:         5,
			IssueKey:   "PROJ-127",
			Summary:    "完了課題2",
			Status:     &domain.IDName{ID: 3, Name: "完了"},
			Priority:   &domain.IDName{ID: 2, Name: "高"},
			Assignee:   &domain.User{ID: 102, Name: "佐藤花子"},
			Categories: []domain.IDName{},
			Milestones: []domain.IDName{{ID: 20, Name: "v1.0"}},
			Created:    &c2,
			Updated:    &t5,
		},
	}
}

// setupTriageMock は共通の MockClient セットアップを行う。
func setupTriageMock(targetIssue *domain.Issue, projectIssues []domain.Issue, comments []domain.Comment) *backlog.MockClient {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return targetIssue, nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return projectIssues, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	return mc
}

// T1: TestTriageMaterials_Build_BasicAttributes は issue 基本属性が正しくマッピングされることを検証する。
func TestTriageMaterials_Build_BasicAttributes(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if env == nil {
		t.Fatal("env is nil")
	}

	result, ok := env.Analysis.(*TriageMaterials)
	if !ok {
		t.Fatalf("Analysis type assertion failed: got %T", env.Analysis)
	}

	if result.Issue.IssueKey != "PROJ-123" {
		t.Errorf("IssueKey = %q, want %q", result.Issue.IssueKey, "PROJ-123")
	}
	if result.Issue.Summary != "テスト課題" {
		t.Errorf("Summary = %q, want %q", result.Issue.Summary, "テスト課題")
	}
	if result.Issue.Status == nil || result.Issue.Status.Name != "未対応" {
		t.Errorf("Status = %v, want 未対応", result.Issue.Status)
	}
	if result.Issue.Priority == nil || result.Issue.Priority.Name != "高" {
		t.Errorf("Priority = %v, want 高", result.Issue.Priority)
	}
	if result.Issue.IssueType == nil || result.Issue.IssueType.Name != "バグ" {
		t.Errorf("IssueType = %v, want バグ", result.Issue.IssueType)
	}
	if result.Issue.Assignee == nil || result.Issue.Assignee.Name != "田中太郎" {
		t.Errorf("Assignee = %v, want 田中太郎", result.Issue.Assignee)
	}
	if result.Issue.Reporter == nil || result.Issue.Reporter.Name != "佐藤花子" {
		t.Errorf("Reporter = %v, want 佐藤花子", result.Issue.Reporter)
	}
	if len(result.Issue.Categories) != 1 || result.Issue.Categories[0].Name != "フロントエンド" {
		t.Errorf("Categories = %v, want [{フロントエンド}]", result.Issue.Categories)
	}
	if len(result.Issue.Milestones) != 1 || result.Issue.Milestones[0].Name != "v1.0" {
		t.Errorf("Milestones = %v, want [{v1.0}]", result.Issue.Milestones)
	}
}

// T2: TestTriageMaterials_Build_History は history（コメント数・経過日数・stale/overdue）が正確であることを検証する。
func TestTriageMaterials_Build_History(t *testing.T) {
	targetIssue := helperTriageIssue()
	comments := []domain.Comment{
		{ID: 1, Content: "コメント1"},
		{ID: 2, Content: "コメント2"},
		{ID: 3, Content: "コメント3"},
	}
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), comments)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	if result.History.CommentCount != 3 {
		t.Errorf("CommentCount = %d, want 3", result.History.CommentCount)
	}
	if result.History.DaysSinceCreated != 31 {
		t.Errorf("DaysSinceCreated = %d, want 31", result.History.DaysSinceCreated)
	}
	if result.History.DaysSinceUpdated != 4 {
		t.Errorf("DaysSinceUpdated = %d, want 4", result.History.DaysSinceUpdated)
	}
	if result.History.IsOverdue {
		t.Error("IsOverdue = true, want false (due date is future)")
	}
	if result.History.IsStale {
		t.Error("IsStale = true, want false (updated 4 days ago, threshold 7)")
	}
}

// T3: TestTriageMaterials_Build_ProjectStats_ByStatus は by_status の集計が正確であることを検証する。
func TestTriageMaterials_Build_ProjectStats_ByStatus(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	// 5課題: 未対応x2, 処理中x1, 完了x2
	if result.ProjectStats.ByStatus["未対応"] != 2 {
		t.Errorf("ByStatus[未対応] = %d, want 2", result.ProjectStats.ByStatus["未対応"])
	}
	if result.ProjectStats.ByStatus["処理中"] != 1 {
		t.Errorf("ByStatus[処理中] = %d, want 1", result.ProjectStats.ByStatus["処理中"])
	}
	if result.ProjectStats.ByStatus["完了"] != 2 {
		t.Errorf("ByStatus[完了] = %d, want 2", result.ProjectStats.ByStatus["完了"])
	}
	if result.ProjectStats.TotalIssues != 5 {
		t.Errorf("TotalIssues = %d, want 5", result.ProjectStats.TotalIssues)
	}
}

// T4: TestTriageMaterials_Build_ProjectStats_ByPriority は by_priority の集計が正確であることを検証する。
func TestTriageMaterials_Build_ProjectStats_ByPriority(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	// 5課題: 高x2, 中x2, 低x1
	if result.ProjectStats.ByPriority["高"] != 2 {
		t.Errorf("ByPriority[高] = %d, want 2", result.ProjectStats.ByPriority["高"])
	}
	if result.ProjectStats.ByPriority["中"] != 2 {
		t.Errorf("ByPriority[中] = %d, want 2", result.ProjectStats.ByPriority["中"])
	}
	if result.ProjectStats.ByPriority["低"] != 1 {
		t.Errorf("ByPriority[低] = %d, want 1", result.ProjectStats.ByPriority["低"])
	}
}

// T5: TestTriageMaterials_Build_ProjectStats_ByAssignee は by_assignee（未割当含む）が正確であることを検証する。
func TestTriageMaterials_Build_ProjectStats_ByAssignee(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	// 5課題: 田中太郎x2, 佐藤花子x2, 未割当x1
	if result.ProjectStats.ByAssignee["田中太郎"] != 2 {
		t.Errorf("ByAssignee[田中太郎] = %d, want 2", result.ProjectStats.ByAssignee["田中太郎"])
	}
	if result.ProjectStats.ByAssignee["佐藤花子"] != 2 {
		t.Errorf("ByAssignee[佐藤花子] = %d, want 2", result.ProjectStats.ByAssignee["佐藤花子"])
	}
	if result.ProjectStats.ByAssignee["未割当"] != 1 {
		t.Errorf("ByAssignee[未割当] = %d, want 1", result.ProjectStats.ByAssignee["未割当"])
	}
}

// T6: TestTriageMaterials_Build_ProjectStats_AvgCloseDays は avg_close_days が完了課題のみ計算されることを検証する。
func TestTriageMaterials_Build_ProjectStats_AvgCloseDays(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	// 完了課題2件:
	//   PROJ-125: created=-60d, updated=-5d → 55日
	//   PROJ-127: created=-40d, updated=-15d → 25日
	// 平均: (55+25)/2 = 40日
	if result.ProjectStats.AvgCloseDays != 40.0 {
		t.Errorf("AvgCloseDays = %f, want 40.0", result.ProjectStats.AvgCloseDays)
	}
}

// T7: TestTriageMaterials_Build_SimilarIssues_Category は同カテゴリ課題の分布が正確であることを検証する。
func TestTriageMaterials_Build_SimilarIssues_Category(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	// 対象課題のカテゴリ: フロントエンド(ID=10)
	// 同カテゴリ: PROJ-124(フロントエンド), PROJ-126(フロントエンド) → 対象課題自身(PROJ-123)を除く2件
	if result.SimilarIssues.SameCategoryCount != 2 {
		t.Errorf("SameCategoryCount = %d, want 2", result.SimilarIssues.SameCategoryCount)
	}
}

// T8: TestTriageMaterials_Build_SimilarIssues_Milestone は同マイルストーン課題の分布が正確であることを検証する。
func TestTriageMaterials_Build_SimilarIssues_Milestone(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)

	// 対象課題のマイルストーン: v1.0(ID=20)
	// 同マイルストーン: PROJ-125(v1.0), PROJ-127(v1.0) → 対象課題自身(PROJ-123)を除く2件
	if result.SimilarIssues.SameMilestoneCount != 2 {
		t.Errorf("SameMilestoneCount = %d, want 2", result.SimilarIssues.SameMilestoneCount)
	}
}

// T9: TestTriageMaterials_Build_NilFields は assignee/category/milestone が nil でも panic しないことを検証する。
func TestTriageMaterials_Build_NilFields(t *testing.T) {
	created := fixedNowTriage.Add(-10 * 24 * time.Hour)
	updated := fixedNowTriage.Add(-2 * 24 * time.Hour)

	minimalIssue := &domain.Issue{
		ID:          2,
		ProjectID:   100,
		IssueKey:    "PROJ-200",
		Summary:     "最小課題",
		Status:      nil,   // nil
		Priority:    nil,   // nil
		IssueType:   nil,   // nil
		Assignee:    nil,   // nil
		Reporter:    nil,   // nil
		Categories:  nil,   // nil
		Milestones:  nil,   // nil
		DueDate:     nil,   // nil
		Created:     &created,
		Updated:     &updated,
	}

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return minimalIssue, nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{*minimalIssue}, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-200", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if env == nil {
		t.Fatal("env is nil")
	}

	result := env.Analysis.(*TriageMaterials)
	// nil フィールドが空スライスに変換されることを確認
	if result.Issue.Categories == nil {
		t.Error("Categories is nil, want empty slice")
	}
	if result.Issue.Milestones == nil {
		t.Error("Milestones is nil, want empty slice")
	}
}

// T10: TestTriageMaterials_Build_Envelope は resource と warnings が正しく設定されることを検証する。
func TestTriageMaterials_Build_Envelope(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if env.Resource != "issue_triage_materials" {
		t.Errorf("Resource = %q, want %q", env.Resource, "issue_triage_materials")
	}
	if env.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", env.SchemaVersion, "1")
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings = %v, want empty", env.Warnings)
	}
	if env.Profile != "default" {
		t.Errorf("Profile = %q, want %q", env.Profile, "default")
	}
	if env.Space != "heptagon" {
		t.Errorf("Space = %q, want %q", env.Space, "heptagon")
	}
}

// E1: TestTriageMaterials_Build_IssueGetError は GetIssue 失敗時に error を返すことを検証する。
func TestTriageMaterials_Build_IssueGetError(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return nil, errors.New("not found")
	}

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-999", TriageMaterialsOptions{})
	if err == nil {
		t.Fatal("Build() error = nil, want error")
	}
	if env != nil {
		t.Error("env should be nil on GetIssue error")
	}
}

// E2: TestTriageMaterials_Build_ProjectFetchError は GetProject 失敗時に warning に追加し部分結果を返すことを検証する。
func TestTriageMaterials_Build_ProjectFetchError(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return targetIssue, nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return nil, errors.New("project not found")
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v, want nil (partial result)", err)
	}
	if env == nil {
		t.Fatal("env is nil")
	}
	if len(env.Warnings) == 0 {
		t.Error("Warnings is empty, want at least one warning for project fetch failure")
	}
}

// E3: TestTriageMaterials_Build_CommentsFetchError は ListIssueComments 失敗時に warning に追加し comment_count=0 になることを検証する。
func TestTriageMaterials_Build_CommentsFetchError(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, helperProjectIssues(fixedNowTriage), nil)
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, errors.New("comments fetch failed")
	}

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v, want nil (partial result)", err)
	}
	if env == nil {
		t.Fatal("env is nil")
	}
	if len(env.Warnings) == 0 {
		t.Error("Warnings is empty, want at least one warning for comments fetch failure")
	}
	result := env.Analysis.(*TriageMaterials)
	if result.History.CommentCount != 0 {
		t.Errorf("CommentCount = %d, want 0 on comments fetch failure", result.History.CommentCount)
	}
}

// E4: TestTriageMaterials_Build_ListIssuesError は ListIssues 失敗時に warning に追加し stats が零値になることを検証する。
func TestTriageMaterials_Build_ListIssuesError(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return targetIssue, nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return nil, errors.New("list issues failed")
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v, want nil (partial result)", err)
	}
	if env == nil {
		t.Fatal("env is nil")
	}
	if len(env.Warnings) == 0 {
		t.Error("Warnings is empty, want at least one warning for list issues failure")
	}
	result := env.Analysis.(*TriageMaterials)
	if result.ProjectStats.TotalIssues != 0 {
		t.Errorf("TotalIssues = %d, want 0 on list issues failure", result.ProjectStats.TotalIssues)
	}
}

// EC1: TestTriageMaterials_Build_EmptyProject はプロジェクト課題 0件の場合 by_status が空マップになることを検証する。
func TestTriageMaterials_Build_EmptyProject(t *testing.T) {
	targetIssue := helperTriageIssue()
	mc := setupTriageMock(targetIssue, []domain.Issue{}, nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)
	if result.ProjectStats.TotalIssues != 0 {
		t.Errorf("TotalIssues = %d, want 0", result.ProjectStats.TotalIssues)
	}
	if result.ProjectStats.ByStatus == nil {
		t.Error("ByStatus is nil, want empty map")
	}
	if len(result.ProjectStats.ByStatus) != 0 {
		t.Errorf("ByStatus length = %d, want 0", len(result.ProjectStats.ByStatus))
	}
}

// EC2: TestTriageMaterials_Build_NoCategoryMatch は同カテゴリなしの場合 same_category_count=0 になることを検証する。
func TestTriageMaterials_Build_NoCategoryMatch(t *testing.T) {
	targetIssue := helperTriageIssue() // カテゴリ: フロントエンド(ID=10)

	// フロントエンド(ID=10) を持つ課題は対象課題のみ（他はバックエンドのみ）
	t1 := fixedNowTriage.Add(-5 * 24 * time.Hour)
	c1 := fixedNowTriage.Add(-10 * 24 * time.Hour)
	issues := []domain.Issue{
		{
			ID:         1,
			IssueKey:   "PROJ-123",
			Summary:    "対象課題",
			Status:     &domain.IDName{ID: 1, Name: "未対応"},
			Categories: []domain.IDName{{ID: 10, Name: "フロントエンド"}},
			Milestones: []domain.IDName{},
			Created:    &c1,
			Updated:    &t1,
		},
		{
			ID:         2,
			IssueKey:   "PROJ-124",
			Summary:    "別カテゴリ課題",
			Status:     &domain.IDName{ID: 1, Name: "未対応"},
			Categories: []domain.IDName{{ID: 11, Name: "バックエンド"}},
			Milestones: []domain.IDName{},
			Created:    &c1,
			Updated:    &t1,
		},
	}

	mc := setupTriageMock(targetIssue, issues, nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)
	if result.SimilarIssues.SameCategoryCount != 0 {
		t.Errorf("SameCategoryCount = %d, want 0", result.SimilarIssues.SameCategoryCount)
	}
}

// EC3: TestTriageMaterials_Build_NoMilestoneMatch は同マイルストーンなしの場合 same_milestone_count=0 になることを検証する。
func TestTriageMaterials_Build_NoMilestoneMatch(t *testing.T) {
	targetIssue := helperTriageIssue() // マイルストーン: v1.0(ID=20)

	t1 := fixedNowTriage.Add(-5 * 24 * time.Hour)
	c1 := fixedNowTriage.Add(-10 * 24 * time.Hour)
	issues := []domain.Issue{
		{
			ID:         1,
			IssueKey:   "PROJ-123",
			Summary:    "対象課題",
			Status:     &domain.IDName{ID: 1, Name: "未対応"},
			Categories: []domain.IDName{},
			Milestones: []domain.IDName{{ID: 20, Name: "v1.0"}},
			Created:    &c1,
			Updated:    &t1,
		},
		{
			ID:         2,
			IssueKey:   "PROJ-124",
			Summary:    "別マイルストーン課題",
			Status:     &domain.IDName{ID: 1, Name: "未対応"},
			Categories: []domain.IDName{},
			Milestones: []domain.IDName{{ID: 21, Name: "v1.1"}},
			Created:    &c1,
			Updated:    &t1,
		},
	}

	mc := setupTriageMock(targetIssue, issues, nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)
	if result.SimilarIssues.SameMilestoneCount != 0 {
		t.Errorf("SameMilestoneCount = %d, want 0", result.SimilarIssues.SameMilestoneCount)
	}
}

// EC4: TestTriageMaterials_Build_NoClosedIssues は完了課題なしの場合 avg_close_days=0 になることを検証する。
func TestTriageMaterials_Build_NoClosedIssues(t *testing.T) {
	targetIssue := helperTriageIssue()

	t1 := fixedNowTriage.Add(-5 * 24 * time.Hour)
	c1 := fixedNowTriage.Add(-10 * 24 * time.Hour)
	issues := []domain.Issue{
		{
			ID:       1,
			IssueKey: "PROJ-123",
			Summary:  "未対応課題",
			Status:   &domain.IDName{ID: 1, Name: "未対応"},
			Created:  &c1,
			Updated:  &t1,
		},
		{
			ID:       2,
			IssueKey: "PROJ-124",
			Summary:  "処理中課題",
			Status:   &domain.IDName{ID: 2, Name: "処理中"},
			Created:  &c1,
			Updated:  &t1,
		},
	}

	mc := setupTriageMock(targetIssue, issues, nil)

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNowTriage }))

	env, err := builder.Build(context.Background(), "PROJ-123", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)
	if result.ProjectStats.AvgCloseDays != 0.0 {
		t.Errorf("AvgCloseDays = %f, want 0.0", result.ProjectStats.AvgCloseDays)
	}
}

// EC5: TestTriageMaterials_Build_ClockInjection は WithClock でテスト時刻を固定して days_since_updated を検証する。
func TestTriageMaterials_Build_ClockInjection(t *testing.T) {
	customNow := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) // 1ヶ月後の時刻

	created := customNow.Add(-40 * 24 * time.Hour)
	updated := customNow.Add(-10 * 24 * time.Hour) // 10日前に更新

	issue := &domain.Issue{
		ID:        1,
		ProjectID: 100,
		IssueKey:  "PROJ-300",
		Summary:   "clock test",
		Status:    &domain.IDName{ID: 1, Name: "未対応"},
		Created:   &created,
		Updated:   &updated,
	}

	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{*issue}, nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}

	builder := NewTriageMaterialsBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return customNow }))

	env, err := builder.Build(context.Background(), "PROJ-300", TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result := env.Analysis.(*TriageMaterials)
	if result.History.DaysSinceUpdated != 10 {
		t.Errorf("DaysSinceUpdated = %d, want 10", result.History.DaysSinceUpdated)
	}
	if result.History.DaysSinceCreated != 40 {
		t.Errorf("DaysSinceCreated = %d, want 40", result.History.DaysSinceCreated)
	}
	// 10日前更新 → stale (>=7日) = true
	if !result.History.IsStale {
		t.Error("IsStale = false, want true (updated 10 days ago, threshold 7)")
	}
}
