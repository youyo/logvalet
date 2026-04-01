package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// fixedNowPeriodic はテスト用の固定時刻。
var fixedNowPeriodic = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

// helperPeriodicBuilder はテスト用の PeriodicDigestBuilder を生成する。
func helperPeriodicBuilder(mock *backlog.MockClient, now time.Time) *PeriodicDigestBuilder {
	return NewPeriodicDigestBuilder(mock, "test-profile", "test-space", "https://test.backlog.com",
		WithClock(func() time.Time { return now }),
	)
}

// mustTime は time.Parse の結果を unwrap するヘルパー。
func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// TC-01: 正常系 — weekly 集計（完了・開始・ブロック混在）
func TestPeriodicDigest_TC01_WeeklyMixed(t *testing.T) {
	since := mustTime("2026-03-25T00:00:00Z")
	until := mustTime("2026-04-01T00:00:00Z")

	// PROJ-10: 完了 (Updated=2026-03-28, Created=2026-03-20)
	created10 := mustTime("2026-03-20T00:00:00Z")
	updated10 := mustTime("2026-03-28T09:00:00Z")

	// PROJ-20: 処理中・開始 (Updated=2026-03-27, Created=2026-03-27)
	created20 := mustTime("2026-03-27T14:00:00Z")
	updated20 := mustTime("2026-03-27T14:00:00Z")

	// PROJ-30: ブロック (Updated=2026-03-29, Created=2026-03-15)
	created30 := mustTime("2026-03-15T00:00:00Z")
	updated30 := mustTime("2026-03-29T00:00:00Z")

	issues := []domain.Issue{
		{
			ID:       10,
			IssueKey: "PROJ-10",
			Summary:  "ログイン機能実装",
			Status:   &domain.IDName{ID: 3, Name: "完了"},
			Assignee: &domain.User{ID: 101, Name: "田中太郎"},
			Created:  &created10,
			Updated:  &updated10,
		},
		{
			ID:       20,
			IssueKey: "PROJ-20",
			Summary:  "ダッシュボード実装",
			Status:   &domain.IDName{ID: 2, Name: "処理中"},
			Assignee: &domain.User{ID: 102, Name: "佐藤花子"},
			Created:  &created20,
			Updated:  &updated20,
		},
		{
			ID:       30,
			IssueKey: "PROJ-30",
			Summary:  "外部API連携",
			Status:   &domain.IDName{ID: 4, Name: "ブロック"},
			Assignee: &domain.User{ID: 101, Name: "田中太郎"},
			Created:  &created30,
			Updated:  &updated30,
		},
	}

	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ"}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period:          "weekly",
		Since:           &since,
		Until:           &until,
		ClosedStatus:    []string{"完了"},
		BlockedKeywords: []string{"ブロック"},
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if env == nil {
		t.Fatal("Build returned nil envelope")
	}
	if env.Resource != "periodic_digest" {
		t.Errorf("Resource = %q, want %q", env.Resource, "periodic_digest")
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	if result.Summary.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d, want 1", result.Summary.CompletedCount)
	}
	if result.Summary.StartedCount != 1 {
		t.Errorf("StartedCount = %d, want 1", result.Summary.StartedCount)
	}
	if result.Summary.BlockedCount != 1 {
		t.Errorf("BlockedCount = %d, want 1", result.Summary.BlockedCount)
	}
	if len(result.Completed) != 1 || result.Completed[0].IssueKey != "PROJ-10" {
		t.Errorf("Completed[0].IssueKey = %q, want %q", result.Completed[0].IssueKey, "PROJ-10")
	}
	if len(result.Started) != 1 || result.Started[0].IssueKey != "PROJ-20" {
		t.Errorf("Started[0].IssueKey = %q, want %q", result.Started[0].IssueKey, "PROJ-20")
	}
	if len(result.Blocked) != 1 || result.Blocked[0].IssueKey != "PROJ-30" {
		t.Errorf("Blocked[0].IssueKey = %q, want %q", result.Blocked[0].IssueKey, "PROJ-30")
	}
}

// TC-02: 正常系 — daily 集計（空結果）
func TestPeriodicDigest_TC02_DailyEmpty(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period: "daily",
		// Since/Until はゼロ値 → デフォルト計算
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	if result.Summary.CompletedCount != 0 {
		t.Errorf("CompletedCount = %d, want 0", result.Summary.CompletedCount)
	}
	if result.Summary.StartedCount != 0 {
		t.Errorf("StartedCount = %d, want 0", result.Summary.StartedCount)
	}
	if result.Summary.BlockedCount != 0 {
		t.Errorf("BlockedCount = %d, want 0", result.Summary.BlockedCount)
	}
	if result.Summary.TotalActiveCount != 0 {
		t.Errorf("TotalActiveCount = %d, want 0", result.Summary.TotalActiveCount)
	}
	if len(result.Completed) != 0 {
		t.Errorf("len(Completed) = %d, want 0", len(result.Completed))
	}
	if len(result.Started) != 0 {
		t.Errorf("len(Started) = %d, want 0", len(result.Started))
	}
	if len(result.Blocked) != 0 {
		t.Errorf("len(Blocked) = %d, want 0", len(result.Blocked))
	}
}

// TC-03: 正常系 — Since/Until ゼロ値でデフォルト計算（weekly: 7日前）
func TestPeriodicDigest_TC03_DefaultPeriodRange(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	var capturedOpt backlog.ListIssuesOptions
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	builder := helperPeriodicBuilder(mock, now)
	opt := PeriodicDigestOptions{
		Period: "weekly",
		// Since/Until はゼロ値
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	// 7日前の計算確認
	expectedSince := now.Add(-7 * 24 * time.Hour)
	if capturedOpt.UpdatedSince == nil {
		t.Fatal("UpdatedSince is nil")
	}
	if !capturedOpt.UpdatedSince.Equal(expectedSince) {
		t.Errorf("UpdatedSince = %v, want %v", capturedOpt.UpdatedSince, expectedSince)
	}
	if capturedOpt.UpdatedUntil == nil {
		t.Fatal("UpdatedUntil is nil")
	}
	if !capturedOpt.UpdatedUntil.Equal(now) {
		t.Errorf("UpdatedUntil = %v, want %v", capturedOpt.UpdatedUntil, now)
	}
	// envelope.analysis の since/until も確認
	if !result.Since.Equal(expectedSince) {
		t.Errorf("result.Since = %v, want %v", result.Since, expectedSince)
	}
	if !result.Until.Equal(now) {
		t.Errorf("result.Until = %v, want %v", result.Until, now)
	}
}

// TC-04: 異常系 — GetProject 失敗
func TestPeriodicDigest_TC04_GetProjectFail(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return nil, errors.New("not found")
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{Period: "weekly"}

	_, err := builder.Build(context.Background(), "NONEXIST", opt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TC-05: 異常系 — ListIssues 失敗（partial result: warning を返す）
func TestPeriodicDigest_TC05_ListIssuesFail(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return nil, errors.New("API error")
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{Period: "weekly"}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("expected nil error for partial failure, got: %v", err)
	}
	if env == nil {
		t.Fatal("expected envelope, got nil")
	}

	// warnings に issues_fetch_failed が含まれることを確認
	hasWarning := false
	for _, w := range env.Warnings {
		if w.Code == "issues_fetch_failed" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning with code 'issues_fetch_failed', got: %v", env.Warnings)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}
	if result.Summary.CompletedCount != 0 || result.Summary.StartedCount != 0 || result.Summary.BlockedCount != 0 {
		t.Errorf("expected all zero counts, got: %+v", result.Summary)
	}
}

// TC-06: エッジケース — ClosedStatus 空（デフォルト使用）
func TestPeriodicDigest_TC06_DefaultClosedStatus(t *testing.T) {
	since := mustTime("2026-03-25T00:00:00Z")
	until := mustTime("2026-04-01T00:00:00Z")

	updated := mustTime("2026-03-28T00:00:00Z")
	created := mustTime("2026-03-20T00:00:00Z")

	issues := []domain.Issue{
		{
			ID:       1,
			IssueKey: "PROJ-1",
			Summary:  "完了課題",
			Status:   &domain.IDName{ID: 3, Name: "完了"}, // defaultClosedStatus に含まれる
			Created:  &created,
			Updated:  &updated,
		},
	}

	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period: "weekly",
		Since:  &since,
		Until:  &until,
		// ClosedStatus は空 → defaultClosedStatus を使用
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	// defaultClosedStatus に "完了" が含まれるので completed_count = 1
	if result.Summary.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d, want 1 (using default closed status)", result.Summary.CompletedCount)
	}
}

// TC-07: エッジケース — completed と started の重複除外
func TestPeriodicDigest_TC07_NoCompletedStartedOverlap(t *testing.T) {
	since := mustTime("2026-03-25T00:00:00Z")
	until := mustTime("2026-04-01T00:00:00Z")

	// 期間内に作成され、かつ完了ステータスの課題
	created := mustTime("2026-03-26T00:00:00Z") // since より後（→ started 候補）
	updated := mustTime("2026-03-28T00:00:00Z") // 期間内

	issues := []domain.Issue{
		{
			ID:       1,
			IssueKey: "PROJ-1",
			Summary:  "期間内に完了した課題",
			Status:   &domain.IDName{ID: 3, Name: "完了"},
			Created:  &created,
			Updated:  &updated,
		},
	}

	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period:       "weekly",
		Since:        &since,
		Until:        &until,
		ClosedStatus: []string{"完了"},
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	// completed に含まれ、started には含まれない
	if result.Summary.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d, want 1", result.Summary.CompletedCount)
	}
	if result.Summary.StartedCount != 0 {
		t.Errorf("StartedCount = %d, want 0 (completed should not also be started)", result.Summary.StartedCount)
	}
}

// TC-08: エッジケース — blocked が completed と重複する場合（completed 優先）
func TestPeriodicDigest_TC08_CompletedPriorityOverBlocked(t *testing.T) {
	since := mustTime("2026-03-25T00:00:00Z")
	until := mustTime("2026-04-01T00:00:00Z")

	created := mustTime("2026-03-20T00:00:00Z")
	updated := mustTime("2026-03-28T00:00:00Z")

	// blocked キーワードに一致するが、完了ステータスでもある
	issues := []domain.Issue{
		{
			ID:       1,
			IssueKey: "PROJ-1",
			Summary:  "blocked キーワードを含む完了課題",
			Status:   &domain.IDName{ID: 3, Name: "完了"},
			Created:  &created,
			Updated:  &updated,
		},
	}

	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period:          "weekly",
		Since:           &since,
		Until:           &until,
		ClosedStatus:    []string{"完了"},
		BlockedKeywords: []string{"完了"}, // 完了を blocked キーワードとしても指定
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	// completed 優先: completed に含まれ、blocked には含まれない
	if result.Summary.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d, want 1", result.Summary.CompletedCount)
	}
	if result.Summary.BlockedCount != 0 {
		t.Errorf("BlockedCount = %d, want 0 (completed should take priority over blocked)", result.Summary.BlockedCount)
	}
}

// TC-09: エッジケース — assignee nil の課題
func TestPeriodicDigest_TC09_NilAssignee(t *testing.T) {
	since := mustTime("2026-03-25T00:00:00Z")
	until := mustTime("2026-04-01T00:00:00Z")

	created := mustTime("2026-03-26T00:00:00Z")
	updated := mustTime("2026-03-28T00:00:00Z")

	issues := []domain.Issue{
		{
			ID:       1,
			IssueKey: "PROJ-1",
			Summary:  "担当者なし完了課題",
			Status:   &domain.IDName{ID: 3, Name: "完了"},
			Assignee: nil, // assignee が nil
			Created:  &created,
			Updated:  &updated,
		},
		{
			ID:       2,
			IssueKey: "PROJ-2",
			Summary:  "担当者なし開始課題",
			Status:   &domain.IDName{ID: 2, Name: "処理中"},
			Assignee: nil, // assignee が nil
			Created:  &created,
			Updated:  &updated,
		},
		{
			ID:       3,
			IssueKey: "PROJ-3",
			Summary:  "担当者なしブロック課題",
			Status:   &domain.IDName{ID: 4, Name: "blocked"},
			Assignee: nil, // assignee が nil
			Created:  &created,
			Updated:  &updated,
		},
	}

	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period:          "weekly",
		Since:           &since,
		Until:           &until,
		ClosedStatus:    []string{"完了"},
		BlockedKeywords: []string{"blocked"},
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	// assignee が nil でも各スライスに含まれること
	if len(result.Completed) != 1 {
		t.Errorf("len(Completed) = %d, want 1", len(result.Completed))
	} else if result.Completed[0].Assignee != nil {
		t.Errorf("Completed[0].Assignee = %v, want nil", result.Completed[0].Assignee)
	}

	if len(result.Started) != 1 {
		t.Errorf("len(Started) = %d, want 1", len(result.Started))
	} else if result.Started[0].Assignee != nil {
		t.Errorf("Started[0].Assignee = %v, want nil", result.Started[0].Assignee)
	}

	if len(result.Blocked) != 1 {
		t.Errorf("len(Blocked) = %d, want 1", len(result.Blocked))
	} else if result.Blocked[0].Assignee != nil {
		t.Errorf("Blocked[0].Assignee = %v, want nil", result.Blocked[0].Assignee)
	}
}

// TC-10: 正常系 — LLMHints 生成
func TestPeriodicDigest_TC10_LLMHints(t *testing.T) {
	since := mustTime("2026-03-25T00:00:00Z")
	until := mustTime("2026-04-01T00:00:00Z")

	// 3 completed, 2 started, 1 blocked
	c1 := mustTime("2026-03-20T00:00:00Z")
	u1 := mustTime("2026-03-26T00:00:00Z")
	c2 := mustTime("2026-03-26T00:00:00Z")
	u2 := mustTime("2026-03-27T00:00:00Z")

	issues := []domain.Issue{
		{ID: 1, IssueKey: "PROJ-1", Summary: "完了1", Status: &domain.IDName{ID: 3, Name: "完了"}, Created: &c1, Updated: &u1},
		{ID: 2, IssueKey: "PROJ-2", Summary: "完了2", Status: &domain.IDName{ID: 3, Name: "完了"}, Created: &c1, Updated: &u1},
		{ID: 3, IssueKey: "PROJ-3", Summary: "完了3", Status: &domain.IDName{ID: 3, Name: "完了"}, Created: &c1, Updated: &u1},
		{ID: 4, IssueKey: "PROJ-4", Summary: "開始1", Status: &domain.IDName{ID: 2, Name: "処理中"}, Created: &c2, Updated: &u2},
		{ID: 5, IssueKey: "PROJ-5", Summary: "開始2", Status: &domain.IDName{ID: 2, Name: "処理中"}, Created: &c2, Updated: &u2},
		{ID: 6, IssueKey: "PROJ-6", Summary: "ブロック1", Status: &domain.IDName{ID: 4, Name: "ブロック"}, Created: &c1, Updated: &u1},
	}

	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(_ context.Context, _ string) (*domain.Project, error) {
		return &domain.Project{ID: 1}, nil
	}
	mock.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return issues, nil
	}

	builder := helperPeriodicBuilder(mock, fixedNowPeriodic)
	opt := PeriodicDigestOptions{
		Period:          "weekly",
		Since:           &since,
		Until:           &until,
		ClosedStatus:    []string{"完了"},
		BlockedKeywords: []string{"ブロック"},
	}

	env, err := builder.Build(context.Background(), "PROJ", opt)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	result, ok := env.Analysis.(*PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis is not *PeriodicDigest: %T", env.Analysis)
	}

	// LLMHints.primary_entities に "project:PROJ" が含まれること
	hasPrimaryEntity := false
	for _, e := range result.LLMHints.PrimaryEntities {
		if e == "project:PROJ" {
			hasPrimaryEntity = true
			break
		}
	}
	if !hasPrimaryEntity {
		t.Errorf("LLMHints.PrimaryEntities does not contain 'project:PROJ': %v", result.LLMHints.PrimaryEntities)
	}

	// blocked > 0 の場合は open_questions に警告が含まれること
	if len(result.LLMHints.OpenQuestions) == 0 {
		t.Errorf("LLMHints.OpenQuestions is empty, expected at least one question for blocked issues")
	}
}
