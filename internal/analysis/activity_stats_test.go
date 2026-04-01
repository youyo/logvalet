package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// fixedTime はテスト用の固定時刻（2026-04-01T12:00:00Z）。
var fixedTime = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

// helper: テスト用 ActivityStatsBuilder を生成する。
func newTestActivityStatsBuilder(mock *backlog.MockClient, clockFn func() time.Time) *ActivityStatsBuilder {
	opts := []Option{}
	if clockFn != nil {
		opts = append(opts, WithClock(clockFn))
	}
	return NewActivityStatsBuilder(mock, "default", "heptagon", "https://heptagon.backlog.com", opts...)
}

// helper: 固定クロックで ActivityStatsBuilder を生成する。
func newFixedActivityStatsBuilder(mock *backlog.MockClient) *ActivityStatsBuilder {
	return newTestActivityStatsBuilder(mock, func() time.Time { return fixedTime })
}

// makeActivity は指定の type, actor, 時刻でテスト用 Activity を生成する。
func makeActivity(id int64, actType int, actorName string, created time.Time) domain.Activity {
	a := domain.Activity{
		ID:      id,
		Type:    actType,
		Created: &created,
	}
	if actorName != "" {
		a.CreatedUser = &domain.User{ID: int(id), Name: actorName}
	}
	return a
}

// ---- T01: scope=project, 基本集計 ----
func TestActivityStatsBuilder_Build_Project_BasicStats(t *testing.T) {
	mock := backlog.NewMockClient()

	t1 := time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 29, 14, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 3, 30, 14, 0, 0, 0, time.UTC)

	activities := []domain.Activity{
		makeActivity(1, 1, "alice", t1), // issue_created, alice, hour=9
		makeActivity(2, 2, "alice", t2), // issue_updated, alice, hour=14
		makeActivity(3, 2, "bob", t3),   // issue_updated, bob, hour=10
		makeActivity(4, 3, "alice", t4), // issue_commented, alice, hour=14
		makeActivity(5, 3, "bob", t5),   // issue_commented, bob, hour=14
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, projectKey string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		if projectKey != "MYPROJECT" {
			t.Errorf("unexpected projectKey: %s", projectKey)
		}
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "MYPROJECT",
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if env == nil {
		t.Fatal("envelope is nil")
	}
	if env.Resource != "activity_stats" {
		t.Errorf("Resource = %q, want %q", env.Resource, "activity_stats")
	}

	stats, ok := env.Analysis.(*ActivityStats)
	if !ok {
		t.Fatalf("Analysis is not *ActivityStats, got %T", env.Analysis)
	}

	// total_count
	if stats.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", stats.TotalCount)
	}

	// by_type
	if stats.ByType["issue_created"] != 1 {
		t.Errorf("ByType[issue_created] = %d, want 1", stats.ByType["issue_created"])
	}
	if stats.ByType["issue_updated"] != 2 {
		t.Errorf("ByType[issue_updated] = %d, want 2", stats.ByType["issue_updated"])
	}
	if stats.ByType["issue_commented"] != 2 {
		t.Errorf("ByType[issue_commented] = %d, want 2", stats.ByType["issue_commented"])
	}

	// by_actor
	if stats.ByActor["alice"] != 3 {
		t.Errorf("ByActor[alice] = %d, want 3", stats.ByActor["alice"])
	}
	if stats.ByActor["bob"] != 2 {
		t.Errorf("ByActor[bob] = %d, want 2", stats.ByActor["bob"])
	}

	// scope/scope_key
	if stats.Scope != "project" {
		t.Errorf("Scope = %q, want %q", stats.Scope, "project")
	}
	if stats.ScopeKey != "MYPROJECT" {
		t.Errorf("ScopeKey = %q, want %q", stats.ScopeKey, "MYPROJECT")
	}
}

// ---- T02: scope=project, 0件 ----
func TestActivityStatsBuilder_Build_Project_Empty(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "EMPTY",
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	stats, ok := env.Analysis.(*ActivityStats)
	if !ok {
		t.Fatalf("Analysis is not *ActivityStats")
	}

	if stats.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", stats.TotalCount)
	}
	if len(stats.ByType) != 0 {
		t.Errorf("ByType should be empty")
	}
	if len(stats.ByActor) != 0 {
		t.Errorf("ByActor should be empty")
	}
	if len(stats.TopActiveActors) != 0 {
		t.Errorf("TopActiveActors should be empty")
	}
	if len(stats.TopActiveTypes) != 0 {
		t.Errorf("TopActiveTypes should be empty")
	}
}

// ---- T03: scope=user ----
func TestActivityStatsBuilder_Build_User_Scope(t *testing.T) {
	mock := backlog.NewMockClient()
	called := false
	mock.ListUserActivitiesFunc = func(_ context.Context, userID string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		if userID != "alice123" {
			t.Errorf("unexpected userID: %s", userID)
		}
		called = true
		t1 := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
		return []domain.Activity{
			makeActivity(1, 1, "alice", t1),
			makeActivity(2, 2, "alice", t1),
			makeActivity(3, 3, "alice", t1),
		}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	_, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "user",
		ScopeKey: "alice123",
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !called {
		t.Error("ListUserActivities was not called")
	}
	// ListProjectActivities が呼ばれていないことを確認
	if mock.GetCallCount("ListProjectActivities") > 0 {
		t.Error("ListProjectActivities should not be called for user scope")
	}
}

// ---- T04: scope=space ----
func TestActivityStatsBuilder_Build_Space_Scope(t *testing.T) {
	mock := backlog.NewMockClient()
	called := false
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		called = true
		t1 := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
		return []domain.Activity{
			makeActivity(1, 1, "alice", t1),
			makeActivity(2, 2, "bob", t1),
			makeActivity(3, 3, "carol", t1),
		}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	_, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope: "space",
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !called {
		t.Error("ListSpaceActivities was not called")
	}
}

// ---- T05: Since/Until 指定 ----
func TestActivityStatsBuilder_Build_DateRange(t *testing.T) {
	mock := backlog.NewMockClient()
	since := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
		Since:    &since,
		Until:    &until,
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	stats := env.Analysis.(*ActivityStats)
	if !stats.Since.Equal(since) {
		t.Errorf("Since = %v, want %v", stats.Since, since)
	}
	if !stats.Until.Equal(until) {
		t.Errorf("Until = %v, want %v", stats.Until, until)
	}
	if !stats.DateRange.Since.Equal(since) {
		t.Errorf("DateRange.Since = %v, want %v", stats.DateRange.Since, since)
	}
	if !stats.DateRange.Until.Equal(until) {
		t.Errorf("DateRange.Until = %v, want %v", stats.DateRange.Until, until)
	}
}

// ---- T06: by_hour 集計 ----
func TestActivityStatsBuilder_Build_ByHour(t *testing.T) {
	mock := backlog.NewMockClient()

	activities := make([]domain.Activity, 24)
	for i := 0; i < 24; i++ {
		ts := time.Date(2026, 3, 30, i, 0, 0, 0, time.UTC)
		activities[i] = makeActivity(int64(i+1), 1, "alice", ts)
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)

	for h := 0; h < 24; h++ {
		if stats.ByHour[h] != 1 {
			t.Errorf("ByHour[%d] = %d, want 1", h, stats.ByHour[h])
		}
	}
}

// ---- T07: by_date 集計 ----
func TestActivityStatsBuilder_Build_ByDate(t *testing.T) {
	mock := backlog.NewMockClient()

	activities := []domain.Activity{
		makeActivity(1, 1, "alice", time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)),
		makeActivity(2, 1, "alice", time.Date(2026, 3, 29, 14, 0, 0, 0, time.UTC)),
		makeActivity(3, 1, "bob", time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)),
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)

	if stats.ByDate["2026-03-29"] != 2 {
		t.Errorf("ByDate[2026-03-29] = %d, want 2", stats.ByDate["2026-03-29"])
	}
	if stats.ByDate["2026-03-30"] != 1 {
		t.Errorf("ByDate[2026-03-30] = %d, want 1", stats.ByDate["2026-03-30"])
	}
}

// ---- T08: actor_concentration ----
func TestActivityStatsBuilder_Build_ActorConcentration(t *testing.T) {
	mock := backlog.NewMockClient()

	ts := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	// alice: 5, bob: 3, carol: 2, dave: 1, eve: 1 → total=12
	// top3 (alice+bob+carol) = 10, concentration = 10/12 ≈ 0.833
	activities := []domain.Activity{}
	for i := 0; i < 5; i++ {
		activities = append(activities, makeActivity(int64(i+1), 1, "alice", ts))
	}
	for i := 0; i < 3; i++ {
		activities = append(activities, makeActivity(int64(10+i), 2, "bob", ts))
	}
	for i := 0; i < 2; i++ {
		activities = append(activities, makeActivity(int64(20+i), 3, "carol", ts))
	}
	activities = append(activities, makeActivity(30, 1, "dave", ts))
	activities = append(activities, makeActivity(31, 1, "eve", ts))

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)

	// top3 = alice(5) + bob(3) + carol(2) = 10, total = 12
	expected := float64(10) / float64(12)
	if abs64(stats.Patterns.ActorConcentration-expected) > 0.001 {
		t.Errorf("ActorConcentration = %f, want %f", stats.Patterns.ActorConcentration, expected)
	}
}

// ---- T09: peak_hour ----
func TestActivityStatsBuilder_Build_PeakHour(t *testing.T) {
	mock := backlog.NewMockClient()

	activities := []domain.Activity{
		makeActivity(1, 1, "alice", time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)),
		makeActivity(2, 1, "alice", time.Date(2026, 3, 30, 14, 0, 0, 0, time.UTC)),
		makeActivity(3, 1, "alice", time.Date(2026, 3, 30, 14, 30, 0, 0, time.UTC)),
		makeActivity(4, 1, "alice", time.Date(2026, 3, 30, 14, 45, 0, 0, time.UTC)),
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)
	if stats.Patterns.PeakHour != 14 {
		t.Errorf("PeakHour = %d, want 14", stats.Patterns.PeakHour)
	}
}

// ---- T10: peak_day_of_week ----
func TestActivityStatsBuilder_Build_PeakDayOfWeek(t *testing.T) {
	mock := backlog.NewMockClient()

	// 2026-03-30 は月曜日(Mon)
	activities := []domain.Activity{
		makeActivity(1, 1, "alice", time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)),  // Mon
		makeActivity(2, 1, "alice", time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)), // Mon
		makeActivity(3, 1, "alice", time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)),  // Tue
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)
	if stats.Patterns.PeakDayOfWeek != "Mon" {
		t.Errorf("PeakDayOfWeek = %q, want %q", stats.Patterns.PeakDayOfWeek, "Mon")
	}
}

// ---- T11: top_active_actors（TopN=3指定）----
func TestActivityStatsBuilder_Build_TopActiveActors(t *testing.T) {
	mock := backlog.NewMockClient()

	ts := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	activities := []domain.Activity{}
	// alice: 5, bob: 3, carol: 2, dave: 1
	for i := 0; i < 5; i++ {
		activities = append(activities, makeActivity(int64(i+1), 1, "alice", ts))
	}
	for i := 0; i < 3; i++ {
		activities = append(activities, makeActivity(int64(10+i), 1, "bob", ts))
	}
	for i := 0; i < 2; i++ {
		activities = append(activities, makeActivity(int64(20+i), 1, "carol", ts))
	}
	activities = append(activities, makeActivity(30, 1, "dave", ts))

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
		TopN:     3,
	})

	stats := env.Analysis.(*ActivityStats)

	if len(stats.TopActiveActors) != 3 {
		t.Errorf("TopActiveActors len = %d, want 3", len(stats.TopActiveActors))
	}
	if stats.TopActiveActors[0].Name != "alice" {
		t.Errorf("TopActiveActors[0].Name = %q, want %q", stats.TopActiveActors[0].Name, "alice")
	}
	if stats.TopActiveActors[0].Count != 5 {
		t.Errorf("TopActiveActors[0].Count = %d, want 5", stats.TopActiveActors[0].Count)
	}
	// ratio = 5/11
	expectedRatio := float64(5) / float64(11)
	if abs64(stats.TopActiveActors[0].Ratio-expectedRatio) > 0.001 {
		t.Errorf("TopActiveActors[0].Ratio = %f, want %f", stats.TopActiveActors[0].Ratio, expectedRatio)
	}
}

// ---- T12: WithClock で固定時刻 ----
func TestActivityStatsBuilder_Build_WithClock(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope: "space",
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !env.GeneratedAt.Equal(fixedTime) {
		t.Errorf("GeneratedAt = %v, want %v", env.GeneratedAt, fixedTime)
	}
}

// ---- E01: ListProjectActivities エラー ----
func TestActivityStatsBuilder_Build_Error_ProjectActivities(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return nil, errors.New("API error")
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "FAIL",
	})

	// エラーではなく部分成功
	if err != nil {
		t.Fatalf("Build should not return error, got: %v", err)
	}
	if len(env.Warnings) == 0 {
		t.Error("Warnings should not be empty on API failure")
	}
	hasCode := false
	for _, w := range env.Warnings {
		if w.Code == "activities_fetch_failed" {
			hasCode = true
			break
		}
	}
	if !hasCode {
		t.Error("Warnings should contain 'activities_fetch_failed'")
	}

	stats := env.Analysis.(*ActivityStats)
	if stats.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0 on error", stats.TotalCount)
	}
}

// ---- E02: ListUserActivities エラー ----
func TestActivityStatsBuilder_Build_Error_UserActivities(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return nil, errors.New("API error")
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "user",
		ScopeKey: "user1",
	})

	if err != nil {
		t.Fatalf("Build should not return error, got: %v", err)
	}
	hasCode := false
	for _, w := range env.Warnings {
		if w.Code == "activities_fetch_failed" {
			hasCode = true
			break
		}
	}
	if !hasCode {
		t.Error("Warnings should contain 'activities_fetch_failed'")
	}
}

// ---- E03: scope="" → space スコープとして動作 ----
func TestActivityStatsBuilder_Build_EmptyScope_DefaultsToSpace(t *testing.T) {
	mock := backlog.NewMockClient()
	called := false
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		called = true
		return []domain.Activity{}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	_, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope: "", // 空文字 → space にフォールバック
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !called {
		t.Error("ListSpaceActivities should be called for empty scope")
	}
}

// ---- E04: アクター名が空 → "unknown" としてカウント ----
func TestActivityStatsBuilder_Build_EmptyActorName(t *testing.T) {
	mock := backlog.NewMockClient()

	ts := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	// CreatedUser が nil のアクティビティ
	a1 := domain.Activity{ID: 1, Type: 1, Created: &ts}
	// CreatedUser の Name が空のアクティビティ
	a2 := domain.Activity{ID: 2, Type: 1, Created: &ts, CreatedUser: &domain.User{ID: 99, Name: ""}}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{a1, a2}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	stats := env.Analysis.(*ActivityStats)
	if stats.ByActor["unknown"] != 2 {
		t.Errorf("ByActor[unknown] = %d, want 2", stats.ByActor["unknown"])
	}
}

// ---- EC01: 全て同一アクター → actor_concentration = 1.0 ----
func TestActivityStatsBuilder_Build_SingleActor_ConcentrationOne(t *testing.T) {
	mock := backlog.NewMockClient()

	ts := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	activities := []domain.Activity{
		makeActivity(1, 1, "alice", ts),
		makeActivity(2, 1, "alice", ts),
		makeActivity(3, 1, "alice", ts),
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)
	if abs64(stats.Patterns.ActorConcentration-1.0) > 0.001 {
		t.Errorf("ActorConcentration = %f, want 1.0", stats.Patterns.ActorConcentration)
	}
}

// ---- EC02: Since/Until が nil → デフォルト期間（now-7days, now）----
func TestActivityStatsBuilder_Build_NilSinceUntil_DefaultPeriod(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, err := b.Build(context.Background(), ActivityStatsOptions{
		Scope: "space",
		// Since/Until は nil
	})

	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	stats := env.Analysis.(*ActivityStats)
	expectedSince := fixedTime.AddDate(0, 0, -7)
	expectedUntil := fixedTime

	if !stats.Since.Equal(expectedSince) {
		t.Errorf("Since = %v, want %v", stats.Since, expectedSince)
	}
	if !stats.Until.Equal(expectedUntil) {
		t.Errorf("Until = %v, want %v", stats.Until, expectedUntil)
	}
}

// ---- EC03: TopN = 0 → デフォルト TopN=5 ----
func TestActivityStatsBuilder_Build_TopNZero_DefaultFive(t *testing.T) {
	mock := backlog.NewMockClient()

	ts := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	activities := []domain.Activity{}
	actors := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}
	for i, name := range actors {
		activities = append(activities, makeActivity(int64(i+1), 1, name, ts))
	}

	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
		TopN:     0, // デフォルト 5
	})

	stats := env.Analysis.(*ActivityStats)
	if len(stats.TopActiveActors) != 5 {
		t.Errorf("TopActiveActors len = %d, want 5 (default TopN)", len(stats.TopActiveActors))
	}
}

// ---- EC04: by_hour のキーが int（0-23）----
func TestActivityStatsBuilder_Build_ByHour_IntKeys(t *testing.T) {
	mock := backlog.NewMockClient()

	ts := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC) // hour=0
	mock.ListProjectActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{makeActivity(1, 1, "alice", ts)}, nil
	}

	b := newFixedActivityStatsBuilder(mock)
	env, _ := b.Build(context.Background(), ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: "PROJ",
	})

	stats := env.Analysis.(*ActivityStats)
	if stats.ByHour[0] != 1 {
		t.Errorf("ByHour[0] = %d, want 1", stats.ByHour[0])
	}
}

// ---- ヘルパー関数の単体テスト ----

func TestBuildByDate(t *testing.T) {
	ts1 := time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 29, 23, 59, 0, 0, time.UTC)
	ts3 := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)

	activities := []domain.Activity{
		makeActivity(1, 1, "alice", ts1),
		makeActivity(2, 1, "alice", ts2),
		makeActivity(3, 1, "alice", ts3),
	}

	result := buildByDate(activities)
	if result["2026-03-29"] != 2 {
		t.Errorf("buildByDate[2026-03-29] = %d, want 2", result["2026-03-29"])
	}
	if result["2026-03-30"] != 1 {
		t.Errorf("buildByDate[2026-03-30] = %d, want 1", result["2026-03-30"])
	}
}

func TestBuildByHour(t *testing.T) {
	ts0 := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	ts14 := time.Date(2026, 3, 30, 14, 0, 0, 0, time.UTC)
	ts14b := time.Date(2026, 3, 30, 14, 30, 0, 0, time.UTC)

	activities := []domain.Activity{
		makeActivity(1, 1, "alice", ts0),
		makeActivity(2, 1, "alice", ts14),
		makeActivity(3, 1, "alice", ts14b),
	}

	result := buildByHour(activities)
	if result[0] != 1 {
		t.Errorf("buildByHour[0] = %d, want 1", result[0])
	}
	if result[14] != 2 {
		t.Errorf("buildByHour[14] = %d, want 2", result[14])
	}
}

func TestCalcConcentration(t *testing.T) {
	tests := []struct {
		name     string
		counts   map[string]int
		total    int
		expected float64
	}{
		{
			name:     "single actor",
			counts:   map[string]int{"alice": 10},
			total:    10,
			expected: 1.0,
		},
		{
			name:     "three actors top3",
			counts:   map[string]int{"alice": 5, "bob": 3, "carol": 2},
			total:    10,
			expected: 1.0,
		},
		{
			name:     "five actors top3 partial",
			counts:   map[string]int{"a": 5, "b": 3, "c": 2, "d": 1, "e": 1},
			total:    12,
			expected: float64(10) / float64(12),
		},
		{
			name:     "zero total",
			counts:   map[string]int{},
			total:    0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcConcentration(tt.counts, tt.total)
			if abs64(got-tt.expected) > 0.001 {
				t.Errorf("calcConcentration() = %f, want %f", got, tt.expected)
			}
		})
	}
}

func TestCalcPeakHour(t *testing.T) {
	byHour := map[int]int{
		9:  2,
		14: 5,
		17: 3,
	}
	got := calcPeakHour(byHour)
	if got != 14 {
		t.Errorf("calcPeakHour() = %d, want 14", got)
	}

	// 空の場合
	got2 := calcPeakHour(map[int]int{})
	if got2 != 0 {
		t.Errorf("calcPeakHour(empty) = %d, want 0", got2)
	}
}

// abs64 は float64 の絶対値を返す。
func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
