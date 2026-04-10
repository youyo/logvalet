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

var fixedNow = time.Date(2026, 1, 12, 10, 0, 0, 0, time.UTC) // 月曜日

func helperMyTasksIssue(issueKey string, statusName, priorityName string, dueDaysFromNow *int, now time.Time) domain.Issue {
	issue := domain.Issue{
		ID:       9000,
		IssueKey: issueKey,
		Summary:  "テスト課題 " + issueKey,
	}
	if statusName != "" {
		issue.Status = &domain.IDName{ID: 1, Name: statusName}
	}
	if priorityName != "" {
		issue.Priority = &domain.IDName{ID: 2, Name: priorityName}
	}
	if dueDaysFromNow != nil {
		d := now.AddDate(0, 0, *dueDaysFromNow)
		issue.DueDate = &d
	}
	return issue
}

func helperWatching(issueKey string, statusName string, assigneeName string, dueDaysFromNow *int, lastUpdatedDaysAgo int, now time.Time) domain.Watching {
	issue := &domain.Issue{
		ID:       8000,
		IssueKey: issueKey,
		Summary:  "ウォッチ課題 " + issueKey,
	}
	if statusName != "" {
		issue.Status = &domain.IDName{ID: 1, Name: statusName}
	}
	if assigneeName != "" {
		issue.Assignee = &domain.User{ID: 50, Name: assigneeName}
	}
	if dueDaysFromNow != nil {
		d := now.AddDate(0, 0, *dueDaysFromNow)
		issue.DueDate = &d
	}
	lastUpdated := now.AddDate(0, 0, -lastUpdatedDaysAgo)
	return domain.Watching{
		ID:                 1,
		Issue:              issue,
		LastContentUpdated: &lastUpdated,
	}
}

func newMyTasksBuilder(mc *backlog.MockClient) *MyTasksBuilder {
	return NewMyTasksBuilder(mc, "default", "heptagon", "https://heptagon.backlog.com",
		WithClock(func() time.Time { return fixedNow }),
	)
}

// ---- T2-1: WeekMode_Basic ----
// overdue と upcoming が正しく分離される。日付範囲も確認。

func TestMyTasksBuilder_WeekMode_Basic(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}
	// 今週（2026/1/12 月曜）: 1/12〜1/18
	// overdue: 先週（1/11 以前）の課題
	// upcoming: 今週内（1/12〜1/18）の課題

	overdueIssue := helperMyTasksIssue("PROJ-1", "処理中", "高", nil, fixedNow)
	d := -2 // 2日前
	overdueIssue.DueDate = func() *time.Time { t := fixedNow.AddDate(0, 0, d); return &t }()
	upcomingIssue := helperMyTasksIssue("PROJ-2", "未対応", "中", nil, fixedNow)
	d2 := 3 // 3日後
	upcomingIssue.DueDate = func() *time.Time { t := fixedNow.AddDate(0, 0, d2); return &t }()

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}

	callCount := 0
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		callCount++
		if opt.DueDateUntil != nil && opt.DueDateSince == nil {
			// overdue クエリ
			return []domain.Issue{overdueIssue}, nil
		}
		// upcoming クエリ
		return []domain.Issue{upcomingIssue}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		return []domain.Watching{}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{Mode: "week"})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if env.Resource != "my_tasks" {
		t.Errorf("Resource = %q, want my_tasks", env.Resource)
	}

	result, ok := env.Analysis.(*MyTasksResult)
	if !ok {
		t.Fatalf("Analysis type = %T, want *MyTasksResult", env.Analysis)
	}

	if result.Mode != "week" {
		t.Errorf("Mode = %q, want week", result.Mode)
	}

	// 日付範囲: 2026/1/12 (月) 〜 2026/1/18 (日)
	wantSince := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC)
	if !result.DateRange.Since.Equal(wantSince) {
		t.Errorf("DateRange.Since = %v, want %v", result.DateRange.Since, wantSince)
	}
	if !result.DateRange.Until.Equal(wantUntil) {
		t.Errorf("DateRange.Until = %v, want %v", result.DateRange.Until, wantUntil)
	}

	if len(result.Overdue) != 1 {
		t.Errorf("Overdue count = %d, want 1", len(result.Overdue))
	}
	if len(result.Upcoming) != 1 {
		t.Errorf("Upcoming count = %d, want 1", len(result.Upcoming))
	}
	if result.Summary.OverdueCount != 1 {
		t.Errorf("Summary.OverdueCount = %d, want 1", result.Summary.OverdueCount)
	}
	if result.Summary.UpcomingCount != 1 {
		t.Errorf("Summary.UpcomingCount = %d, want 1", result.Summary.UpcomingCount)
	}
}

// ---- T2-2: NextMode_DateRange ----
// 各曜日の営業日オフセット確認

func TestMyTasksBuilder_NextMode_DateRange(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		wantDays  int // until = now + wantDays
	}{
		{
			name:     "月曜日: +4",
			now:      time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC), // 月曜
			wantDays: 4,
		},
		{
			name:     "火曜日: +6",
			now:      time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC), // 火曜
			wantDays: 6,
		},
		{
			name:     "水曜日: +6",
			now:      time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC), // 水曜
			wantDays: 6,
		},
		{
			name:     "木曜日: +6",
			now:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), // 木曜
			wantDays: 6,
		},
		{
			name:     "金曜日: +6",
			now:      time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC), // 金曜
			wantDays: 6,
		},
		{
			name:     "土曜日: +5",
			now:      time.Date(2026, 1, 17, 0, 0, 0, 0, time.UTC), // 土曜
			wantDays: 5,
		},
		{
			name:     "日曜日: +4",
			now:      time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC), // 日曜
			wantDays: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dr := calcDateRange(tt.now, "next")
			wantUntil := truncateDay(tt.now.AddDate(0, 0, tt.wantDays))
			wantSince := truncateDay(tt.now)
			if !dr.Since.Equal(wantSince) {
				t.Errorf("Since = %v, want %v", dr.Since, wantSince)
			}
			if !dr.Until.Equal(wantUntil) {
				t.Errorf("Until = %v, want %v", dr.Until, wantUntil)
			}
		})
	}
}

// ---- T2-3: WatchingDedup ----
// 担当+ウォッチ重複 → 担当側のみ

func TestMyTasksBuilder_WatchingDedup(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}

	// PROJ-1 は担当（overdue）かつウォッチ中
	overdueIssue := helperMyTasksIssue("PROJ-1", "処理中", "高", nil, fixedNow)

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		if opt.DueDateUntil != nil && opt.DueDateSince == nil {
			return []domain.Issue{overdueIssue}, nil
		}
		return []domain.Issue{}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		// PROJ-1 はウォッチ中かつ担当 → 重複
		w := helperWatching("PROJ-1", "処理中", "別のユーザー", nil, 3, fixedNow)
		return []domain.Watching{w}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*MyTasksResult)

	if len(result.Overdue) != 1 {
		t.Errorf("Overdue count = %d, want 1", len(result.Overdue))
	}
	// Watching には PROJ-1 が出ない
	if len(result.Watching) != 0 {
		t.Errorf("Watching count = %d, want 0 (should be deduped)", len(result.Watching))
	}
}

// ---- T2-4: WatchingClosedExcluded ----
// 完了ステータスのウォッチ課題は除外

func TestMyTasksBuilder_WatchingClosedExcluded(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		w := helperWatching("PROJ-5", "完了", "誰か", nil, 3, fixedNow)
		return []domain.Watching{w}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*MyTasksResult)

	if len(result.Watching) != 0 {
		t.Errorf("Watching count = %d, want 0 (closed should be excluded)", len(result.Watching))
	}
}

// ---- T2-5: WatchingStaleSignal ----
// 7日以上未更新 → is_stale: true

func TestMyTasksBuilder_WatchingStaleSignal(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		// 8日前に更新 → stale
		w := helperWatching("PROJ-6", "処理中", "誰か", nil, 8, fixedNow)
		return []domain.Watching{w}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{StaleDays: 7})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*MyTasksResult)

	if len(result.Watching) != 1 {
		t.Fatalf("Watching count = %d, want 1", len(result.Watching))
	}
	if !result.Watching[0].IsStale {
		t.Error("IsStale = false, want true (8 days >= 7)")
	}
	if result.Watching[0].DaysSinceUpdate < 7 {
		t.Errorf("DaysSinceUpdate = %d, want >= 7", result.Watching[0].DaysSinceUpdate)
	}
}

// ---- T2-6: WatchingOverdueSignal ----
// 期限超過 → is_overdue: true

func TestMyTasksBuilder_WatchingOverdueSignal(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		// 期限が3日前 → overdue
		d := -3
		w := helperWatching("PROJ-7", "処理中", "誰か", &d, 1, fixedNow)
		return []domain.Watching{w}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*MyTasksResult)

	if len(result.Watching) != 1 {
		t.Fatalf("Watching count = %d, want 1", len(result.Watching))
	}
	if !result.Watching[0].IsOverdue {
		t.Error("IsOverdue = false, want true (due 3 days ago)")
	}
}

// ---- T2-7: EmptyResults ----
// 全0件 → Summary 全0

func TestMyTasksBuilder_EmptyResults(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		return []domain.Watching{}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	result := env.Analysis.(*MyTasksResult)

	if result.Summary.OverdueCount != 0 {
		t.Errorf("OverdueCount = %d, want 0", result.Summary.OverdueCount)
	}
	if result.Summary.UpcomingCount != 0 {
		t.Errorf("UpcomingCount = %d, want 0", result.Summary.UpcomingCount)
	}
	if result.Summary.WatchingCount != 0 {
		t.Errorf("WatchingCount = %d, want 0", result.Summary.WatchingCount)
	}
	if result.Summary.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.Summary.TotalCount)
	}
	if len(result.Overdue) != 0 {
		t.Errorf("Overdue = %v, want empty slice", result.Overdue)
	}
	if len(result.Upcoming) != 0 {
		t.Errorf("Upcoming = %v, want empty slice", result.Upcoming)
	}
	if len(result.Watching) != 0 {
		t.Errorf("Watching = %v, want empty slice", result.Watching)
	}
}

// ---- T2-8: PartialFailure ----
// ListIssues(overdue) 失敗 → upcoming + watching は返却 + warning

func TestMyTasksBuilder_PartialFailure(t *testing.T) {
	myself := &domain.User{ID: 1, Name: "テストユーザー"}
	upcomingIssue := helperMyTasksIssue("PROJ-10", "未対応", "中", nil, fixedNow)

	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return myself, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		if opt.DueDateUntil != nil && opt.DueDateSince == nil {
			// overdue クエリ → 失敗
			return nil, errors.New("API error for overdue")
		}
		// upcoming クエリ → 成功
		return []domain.Issue{upcomingIssue}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		return []domain.Watching{}, nil
	}

	builder := newMyTasksBuilder(mc)
	env, err := builder.Build(context.Background(), MyTasksOptions{})
	if err != nil {
		t.Fatalf("Build() should not return error for partial failure: %v", err)
	}

	result := env.Analysis.(*MyTasksResult)

	// overdue は空
	if len(result.Overdue) != 0 {
		t.Errorf("Overdue count = %d, want 0 (failed)", len(result.Overdue))
	}

	// upcoming は返却される
	if len(result.Upcoming) != 1 {
		t.Errorf("Upcoming count = %d, want 1", len(result.Upcoming))
	}

	// warnings に overdue_fetch_failed が含まれる
	foundWarning := false
	for _, w := range env.Warnings {
		if w.Code == "overdue_fetch_failed" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Warnings does not contain overdue_fetch_failed, got: %v", env.Warnings)
	}
}
