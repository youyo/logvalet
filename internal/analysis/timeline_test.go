package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

var fixedNowTimeline = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

// --- helper functions ---

func helperTimelineIssue() *domain.Issue {
	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	return &domain.Issue{
		ID:       456,
		IssueKey: "PROJ-123",
		Summary:  "テスト課題",
		Created:  &created,
		Updated:  &updated,
	}
}

func helperTimePtr(t time.Time) *time.Time {
	return &t
}

func helperBoolPtr(b bool) *bool {
	return &b
}

func helperMakeActivity(id int64, actType int, issueID int64, created time.Time, user *domain.User, changes []interface{}) domain.Activity {
	content := map[string]interface{}{
		"id":      float64(issueID), // JSON number は float64 で来る
		"key_id":  float64(123),
		"summary": "テスト課題",
	}
	if changes != nil {
		content["changes"] = changes
	}
	return domain.Activity{
		ID:          id,
		Type:        actType,
		Created:     &created,
		CreatedUser: user,
		Content:     content,
	}
}

func helperMakeComment(id int64, content string, created time.Time, user *domain.User) domain.Comment {
	return domain.Comment{
		ID:          id,
		Content:     content,
		CreatedUser: user,
		Created:     &created,
	}
}

func helperUser(id int, name string) *domain.User {
	return &domain.User{ID: id, Name: name}
}

func newTimelineBuilder(mock *backlog.MockClient) *CommentTimelineBuilder {
	return NewCommentTimelineBuilder(mock, "test-profile", "test-space", "https://test.backlog.com",
		WithClock(func() time.Time { return fixedNowTimeline }),
	)
}

// --- T01: コメントと更新履歴が時系列昇順に統合される ---

func TestCommentTimelineBuilder_T01_MergesEventsInOrder(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	user1 := helperUser(1, "田中")
	user2 := helperUser(2, "鈴木")

	comments := []domain.Comment{
		helperMakeComment(1, "最初のコメント", t1, user1),
		helperMakeComment(3, "3番目のコメント", t3, user2),
	}
	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t2, user1, nil),
		helperMakeActivity(11, 2, 456, t4, user2, nil),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(ct.Events))
	}

	// 時系列昇順チェック
	for i := 1; i < len(ct.Events); i++ {
		prev := ct.Events[i-1].Timestamp
		curr := ct.Events[i].Timestamp
		if prev != nil && curr != nil && prev.After(*curr) {
			t.Errorf("events not in ascending order at index %d", i)
		}
	}
}

// --- T02: events の kind が正しく設定される ---

func TestCommentTimelineBuilder_T02_KindsAreCorrect(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	t1 := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC) // issue created と同時刻
	t2 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)

	user1 := helperUser(1, "田中")

	comments := []domain.Comment{
		helperMakeComment(1, "コメント", t3, user1),
	}
	activities := []domain.Activity{
		helperMakeActivity(10, 1, 456, t1, user1, nil), // type=1: issue_created
		helperMakeActivity(11, 2, 456, t2, user1, nil), // type=2: issue_updated
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	kindMap := make(map[TimelineEventKind]int)
	for _, e := range ct.Events {
		kindMap[e.Kind]++
	}

	if kindMap[TimelineEventKindCreated] != 1 {
		t.Errorf("expected 1 'created' event, got %d", kindMap[TimelineEventKindCreated])
	}
	if kindMap[TimelineEventKindUpdate] != 1 {
		t.Errorf("expected 1 'update' event, got %d", kindMap[TimelineEventKindUpdate])
	}
	if kindMap[TimelineEventKindComment] != 1 {
		t.Errorf("expected 1 'comment' event, got %d", kindMap[TimelineEventKindComment])
	}
}

// --- T03: meta のカウントが正確 ---

func TestCommentTimelineBuilder_T03_MetaCountsAreAccurate(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")

	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC)

	comments := []domain.Comment{
		helperMakeComment(1, "C1", t1, user1),
		helperMakeComment(2, "C2", t2, user1),
		helperMakeComment(3, "C3", t3, user1),
	}
	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t4, user1, nil),
		helperMakeActivity(11, 2, 456, t5, user1, nil),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if ct.Meta.CommentCount != 3 {
		t.Errorf("expected comment_count=3, got %d", ct.Meta.CommentCount)
	}
	if ct.Meta.UpdateCount != 2 {
		t.Errorf("expected update_count=2, got %d", ct.Meta.UpdateCount)
	}
	if ct.Meta.TotalEvents != 5 {
		t.Errorf("expected total_events=5, got %d", ct.Meta.TotalEvents)
	}
}

// --- T04: IncludeUpdates=false でコメントのみ返る ---

func TestCommentTimelineBuilder_T04_IncludeUpdatesFalseReturnsOnlyComments(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	comments := []domain.Comment{
		helperMakeComment(1, "C1", t1, user1),
		helperMakeComment(2, "C2", t2, user1),
	}
	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t3, user1, nil),
		helperMakeActivity(11, 2, 456, t4, user1, nil),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	builder := newTimelineBuilder(mock)
	falseVal := false
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{
		IncludeUpdates: &falseVal,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	for _, e := range ct.Events {
		if e.Kind != TimelineEventKindComment {
			t.Errorf("expected only comment events, got kind=%s", e.Kind)
		}
	}
	if len(ct.Events) != 2 {
		t.Errorf("expected 2 events (comments only), got %d", len(ct.Events))
	}
}

// --- T05: Since/Until フィルタが適用される ---

func TestCommentTimelineBuilder_T05_SinceUntilFilter(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	comments := []domain.Comment{
		helperMakeComment(1, "C1", t1, user1),
		helperMakeComment(2, "C2", t2, user1),
		helperMakeComment(3, "C3", t3, user1),
		helperMakeComment(4, "C4", t4, user1),
		helperMakeComment(5, "C5", t5, user1),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	since := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{
		Since: &since,
		Until: &until,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	// t2, t3, t4 が範囲内（since=3/4, until=3/16）
	if len(ct.Events) != 3 {
		t.Errorf("expected 3 events in range, got %d", len(ct.Events))
	}
}

// --- T06: participant_count がユニーク集計される ---

func TestCommentTimelineBuilder_T06_ParticipantCountIsUnique(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)

	comments := []domain.Comment{
		helperMakeComment(1, "C1", t1, user1),
		helperMakeComment(2, "C2", t2, user1),
		helperMakeComment(3, "C3", t3, user1),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if ct.Meta.ParticipantCount != 1 {
		t.Errorf("expected participant_count=1, got %d", ct.Meta.ParticipantCount)
	}
}

// --- T07: changes が TimelineChange に変換される ---

func TestCommentTimelineBuilder_T07_ChangesAreConverted(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	changes := []interface{}{
		map[string]interface{}{
			"field":     "status",
			"old_value": "未対応",
			"new_value": "処理中",
		},
	}
	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t1, user1, changes),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	var updateEvent *TimelineEvent
	for i := range ct.Events {
		if ct.Events[i].Kind == TimelineEventKindUpdate {
			updateEvent = &ct.Events[i]
			break
		}
	}
	if updateEvent == nil {
		t.Fatal("expected update event not found")
	}
	if len(updateEvent.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(updateEvent.Changes))
	}
	if updateEvent.Changes[0].Field != "status" {
		t.Errorf("expected field='status', got '%s'", updateEvent.Changes[0].Field)
	}
	if updateEvent.Changes[0].OldValue != "未対応" {
		t.Errorf("expected old_value='未対応', got '%s'", updateEvent.Changes[0].OldValue)
	}
	if updateEvent.Changes[0].NewValue != "処理中" {
		t.Errorf("expected new_value='処理中', got '%s'", updateEvent.Changes[0].NewValue)
	}
}

// --- T08: AnalysisEnvelope で包まれる ---

func TestCommentTimelineBuilder_T08_WrappedInAnalysisEnvelope(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Resource != "issue_timeline" {
		t.Errorf("expected resource='issue_timeline', got '%s'", env.Resource)
	}
	if _, ok := env.Analysis.(*CommentTimeline); !ok {
		t.Errorf("expected Analysis to be *CommentTimeline")
	}
}

// --- T09: GetIssue 失敗は error を返す ---

func TestCommentTimelineBuilder_T09_GetIssueFailureReturnsError(t *testing.T) {
	mock := backlog.NewMockClient()

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return nil, errors.New("not found")
	}

	builder := newTimelineBuilder(mock)
	_, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- T10: ListIssueComments 失敗は warning に留まる ---

func TestCommentTimelineBuilder_T10_ListIssueCommentsFailureIsWarning(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t1, user1, nil),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, errors.New("comments fetch error")
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// warnings に含まれること
	hasWarning := false
	for _, w := range env.Warnings {
		if w.Code == "comments_fetch_failed" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected 'comments_fetch_failed' warning")
	}

	// activities のイベントは含まれること
	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) == 0 {
		t.Error("expected activities events to be present")
	}
}

// --- T11: ListProjectActivities 失敗は warning に留まる ---

func TestCommentTimelineBuilder_T11_ListProjectActivitiesFailureIsWarning(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return []domain.Comment{helperMakeComment(1, "C1", t1, user1)}, nil
	}
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return nil, errors.New("activities fetch error")
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// warnings に含まれること
	hasWarning := false
	for _, w := range env.Warnings {
		if w.Code == "activities_fetch_failed" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected 'activities_fetch_failed' warning")
	}

	// comments のイベントは含まれること
	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) == 0 {
		t.Error("expected comment events to be present")
	}
}

// --- T12: コメント・アクティビティが空の場合 ---

func TestCommentTimelineBuilder_T12_EmptyCommentsAndActivities(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if ct.Events == nil {
		t.Error("expected Events to be non-nil (empty slice)")
	}
	if len(ct.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(ct.Events))
	}
	if ct.Meta.TotalEvents != 0 {
		t.Errorf("expected total_events=0, got %d", ct.Meta.TotalEvents)
	}
}

// --- T13: activity.Content["id"] が issue.ID に一致しないものはスキップ ---

func TestCommentTimelineBuilder_T13_ActivitiesFilteredByIssueID(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue() // issue.ID = 456

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)

	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t1, user1, nil), // 対象課題
		helperMakeActivity(11, 2, 999, t2, user1, nil), // 他課題（スキップ）
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) != 1 {
		t.Errorf("expected 1 event (only matching issue), got %d", len(ct.Events))
	}
}

// --- T14: Timestamp が nil のイベントはソートで末尾に寄る ---

func TestCommentTimelineBuilder_T14_NilTimestampGoesToEnd(t *testing.T) {
	// Timestamp nil のイベントはソートで安全に処理される
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	// Timestamp が nil のコメントを作成
	commentWithNilTimestamp := domain.Comment{
		ID:          99,
		Content:     "nil timestamp comment",
		CreatedUser: user1,
		Created:     nil, // nil timestamp
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return []domain.Comment{
			helperMakeComment(1, "C1", t1, user1),
			commentWithNilTimestamp,
		}, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	// パニックが起きないことを確認
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(ct.Events))
	}

	// nil timestamp は末尾に来ること
	lastEvent := ct.Events[len(ct.Events)-1]
	if lastEvent.Timestamp != nil {
		t.Errorf("expected last event to have nil timestamp")
	}
}

// --- T15: IncludeUpdates=nil（未指定）でも更新履歴が含まれる ---

func TestCommentTimelineBuilder_T15_IncludeUpdatesNilDefaultsToTrue(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t1, user1, nil),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	// IncludeUpdates は nil（未指定）
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{
		IncludeUpdates: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	hasUpdate := false
	for _, e := range ct.Events {
		if e.Kind == TimelineEventKindUpdate {
			hasUpdate = true
			break
		}
	}
	if !hasUpdate {
		t.Error("expected update events when IncludeUpdates=nil (default true)")
	}
}

// --- T16: MaxComments=0 は全件取得 ---

func TestCommentTimelineBuilder_T16_MaxCommentsZeroReturnsAll(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	var comments []domain.Comment
	for i := 0; i < 10; i++ {
		ts := time.Date(2026, 3, 1+i, 10, 0, 0, 0, time.UTC)
		comments = append(comments, helperMakeComment(int64(i+1), "C", ts, user1))
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{
		MaxComments: 0, // 全件
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) != 10 {
		t.Errorf("expected 10 events, got %d", len(ct.Events))
	}
}

// --- T17: warnings が nil でなく空スライスで返る ---

func TestCommentTimelineBuilder_T17_WarningsIsEmptySliceNotNil(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Warnings == nil {
		t.Error("expected Warnings to be non-nil empty slice, got nil")
	}
}

// --- T18: 同時刻イベントのソートが決定的 ---

func TestCommentTimelineBuilder_T18_SameTimestampSortIsDeterministic(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	sameTime := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	// 同時刻の created, update, comment
	activities := []domain.Activity{
		helperMakeActivity(10, 1, 456, sameTime, user1, nil), // created
		helperMakeActivity(11, 2, 456, sameTime, user1, nil), // update
	}
	comments := []domain.Comment{
		helperMakeComment(1, "C1", sameTime, user1), // comment
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(ct.Events))
	}

	// created → update → comment の順
	if ct.Events[0].Kind != TimelineEventKindCreated {
		t.Errorf("expected first event kind='created', got '%s'", ct.Events[0].Kind)
	}
	if ct.Events[1].Kind != TimelineEventKindUpdate {
		t.Errorf("expected second event kind='update', got '%s'", ct.Events[1].Kind)
	}
	if ct.Events[2].Kind != TimelineEventKindComment {
		t.Errorf("expected third event kind='comment', got '%s'", ct.Events[2].Kind)
	}
}

// --- T19: activity_pagination_truncated warning が出る ---

func TestCommentTimelineBuilder_T19_PaginationTruncatedWarning(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")

	// MaxActivityPages=1 で 100 件返した後もまだ続くように見せる
	// ページ1: 100件（limit=100 で 100件返ってきた → まだある可能性）
	page1Activities := make([]domain.Activity, 100)
	for i := range page1Activities {
		ts := time.Date(2026, 3, 1, 0, 0, i, 0, time.UTC)
		page1Activities[i] = helperMakeActivity(int64(i+1), 2, 456, ts, user1, nil)
	}

	callCount := 0
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return page1Activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{
		MaxActivityPages: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasWarning := false
	for _, w := range env.Warnings {
		if w.Code == "activity_pagination_truncated" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected 'activity_pagination_truncated' warning")
	}
}

// --- T20: content["id"] が float64 でも正しくフィルタされる ---

func TestCommentTimelineBuilder_T20_ContentIDAsFloat64(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue() // issue.ID = 456

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	// float64 型で ID を設定（JSON unmarshal の挙動を再現）
	activity := domain.Activity{
		ID:          10,
		Type:        2,
		Created:     &t1,
		CreatedUser: user1,
		Content: map[string]interface{}{
			"id": float64(456), // float64 として設定
		},
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return []domain.Activity{activity}, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if len(ct.Events) != 1 {
		t.Errorf("expected 1 event (float64 id should match), got %d", len(ct.Events))
	}
}

// --- T21: changes parse 失敗時に activity_changes_parse_failed warning が出る ---

func TestCommentTimelineBuilder_T21_ChangesParseFailed(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	// malformed changes（文字列の配列）
	activity := domain.Activity{
		ID:          10,
		Type:        2,
		Created:     &t1,
		CreatedUser: user1,
		Content: map[string]interface{}{
			"id":      float64(456),
			"changes": "not a slice", // malformed
		},
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return []domain.Activity{activity}, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasWarning := false
	for _, w := range env.Warnings {
		if w.Code == "activity_changes_parse_failed" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected 'activity_changes_parse_failed' warning")
	}
}

// --- T22: AnalysisEnvelope のメタデータが正しく設定される ---

func TestCommentTimelineBuilder_T22_EnvelopeMetadataIsCorrect(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return nil, nil
	}

	builder := NewCommentTimelineBuilder(mock, "my-profile", "my-space", "https://example.backlog.com",
		WithClock(func() time.Time { return fixedNowTimeline }),
	)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Profile != "my-profile" {
		t.Errorf("expected Profile='my-profile', got '%s'", env.Profile)
	}
	if env.Space != "my-space" {
		t.Errorf("expected Space='my-space', got '%s'", env.Space)
	}
	if env.BaseURL != "https://example.backlog.com" {
		t.Errorf("expected BaseURL='https://example.backlog.com', got '%s'", env.BaseURL)
	}
	if !env.GeneratedAt.Equal(fixedNowTimeline.UTC()) {
		t.Errorf("expected GeneratedAt=%v, got %v", fixedNowTimeline.UTC(), env.GeneratedAt)
	}
}

// --- T23: participant_count は mixed events（コメント + 更新）のユニーク actor 数 ---

func TestCommentTimelineBuilder_T23_ParticipantCountMixedEvents(t *testing.T) {
	mock := backlog.NewMockClient()
	issue := helperTimelineIssue()

	user1 := helperUser(1, "田中")
	user2 := helperUser(2, "鈴木")

	t1 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	comments := []domain.Comment{
		helperMakeComment(1, "C1 by user1", t1, user1),
		helperMakeComment(2, "C2 by user2", t2, user2),
	}
	activities := []domain.Activity{
		helperMakeActivity(10, 2, 456, t3, user1, nil),
		helperMakeActivity(11, 2, 456, t4, user2, nil),
	}

	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return comments, nil
	}
	callCount := 0
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if callCount == 1 {
			return activities, nil
		}
		return nil, nil
	}

	builder := newTimelineBuilder(mock)
	env, err := builder.Build(context.Background(), "PROJ-123", CommentTimelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := env.Analysis.(*CommentTimeline)
	if ct.Meta.ParticipantCount != 2 {
		t.Errorf("expected participant_count=2, got %d", ct.Meta.ParticipantCount)
	}
}
