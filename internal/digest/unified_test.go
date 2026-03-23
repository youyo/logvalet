package digest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// ---- test helpers ----

func newTestTimePtr(t time.Time) *time.Time {
	return &t
}

var (
	testSince = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	testUntil = time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)
)

func newMockActivity(id int, created time.Time) domain.Activity {
	return domain.Activity{
		ID:          int64(id),
		Type:        1, // issue_created
		Created:     &created,
		CreatedUser: &domain.User{ID: 42, Name: "Ishizawa"},
		Content: map[string]interface{}{
			"id":      float64(id * 100),
			"key":     fmt.Sprintf("HEP-%d", id),
			"summary": fmt.Sprintf("テスト課題 %d", id),
		},
	}
}

func newMockIssue(id int, projectID int, assigneeID int) domain.Issue {
	return domain.Issue{
		ID:        id,
		ProjectID: projectID,
		IssueKey:  fmt.Sprintf("HEP-%d", id),
		Summary:   fmt.Sprintf("課題 %d", id),
		Status:    &domain.IDName{ID: 1, Name: "処理中"},
		Assignee:  &domain.User{ID: assigneeID, Name: "Ishizawa"},
	}
}

// ---- D1: project + user scope ----

func TestBuild_projectAndUser(t *testing.T) {
	mc := backlog.NewMockClient()

	project := &domain.Project{ID: 1, ProjectKey: "HEP", Name: "ヘプタゴン"}
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.GetProjectFunc = func(_ context.Context, key string) (*domain.Project, error) {
		if key == "HEP" {
			return project, nil
		}
		return nil, backlog.ErrNotFound
	}
	mc.GetUserFunc = func(_ context.Context, userID string) (*domain.User, error) {
		return &domain.User{ID: 42, Name: "Ishizawa"}, nil
	}
	mc.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{
			newMockIssue(1, 1, 42),
			newMockIssue(2, 1, 42),
		}, nil
	}
	mc.ListUserActivitiesFunc = func(_ context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			newMockActivity(1, actTime),
		}, nil
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		ProjectKeys: []string{"HEP"},
		UserIDs:     []int{42},
		Since:       &testSince,
		Until:       &testUntil,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if env.Resource != "digest" {
		t.Errorf("Resource = %q, want %q", env.Resource, "digest")
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest, got %T", env.Digest)
	}

	if ud.Summary.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2", ud.Summary.IssueCount)
	}
	if ud.Summary.ActivityCount != 1 {
		t.Errorf("ActivityCount = %d, want 1", ud.Summary.ActivityCount)
	}
	if len(env.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", env.Warnings)
	}
}

// ---- D2: space-wide (no flags) ----

func TestBuild_spaceWide(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{newMockIssue(1, 0, 0)}, nil
	}
	mc.ListSpaceActivitiesFunc = func(_ context.Context, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{newMockActivity(1, actTime)}, nil
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		Since: &testSince,
		Until: &testUntil,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if ud.Summary.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", ud.Summary.IssueCount)
	}
	if ud.Summary.ActivityCount != 1 {
		t.Errorf("ActivityCount = %d, want 1", ud.Summary.ActivityCount)
	}
}

// ---- D3: team scope ----

func TestBuild_teamScope(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.GetTeamFunc = func(_ context.Context, teamID int) (*domain.TeamWithMembers, error) {
		if teamID == 173843 {
			return &domain.TeamWithMembers{
				ID:   173843,
				Name: "開発チーム",
				Members: []domain.User{
					{ID: 10, Name: "Alice"},
					{ID: 20, Name: "Bob"},
				},
			}, nil
		}
		return nil, backlog.ErrNotFound
	}
	mc.GetUserFunc = func(_ context.Context, userID string) (*domain.User, error) {
		switch userID {
		case "10":
			return &domain.User{ID: 10, Name: "Alice"}, nil
		case "20":
			return &domain.User{ID: 20, Name: "Bob"}, nil
		}
		return nil, backlog.ErrNotFound
	}
	mc.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		// teamメンバーのIDs（10,20）でフィルタされる
		issues := []domain.Issue{
			newMockIssue(1, 0, 10),
			newMockIssue(2, 0, 20),
			newMockIssue(3, 0, 10),
		}
		return issues, nil
	}
	mc.ListUserActivitiesFunc = func(_ context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		// userID "10" or "20"
		return []domain.Activity{newMockActivity(1, actTime)}, nil
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		TeamIDs: []int{173843},
		Since:   &testSince,
		Until:   &testUntil,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if ud.Summary.IssueCount != 3 {
		t.Errorf("IssueCount = %d, want 3", ud.Summary.IssueCount)
	}
	// 2メンバー各1件 = 2件のアクティビティ
	if ud.Summary.ActivityCount != 2 {
		t.Errorf("ActivityCount = %d, want 2", ud.Summary.ActivityCount)
	}
	if len(env.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", env.Warnings)
	}
}

// ---- D4: issue scope ----

func TestBuild_issueScope(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.GetIssueFunc = func(_ context.Context, issueKey string) (*domain.Issue, error) {
		if issueKey == "HEP-1" {
			issue := newMockIssue(1, 1, 42)
			return &issue, nil
		}
		return nil, backlog.ErrNotFound
	}
	mc.GetProjectFunc = func(_ context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "HEP", Name: "ヘプタゴン"}, nil
	}
	mc.ListProjectActivitiesFunc = func(_ context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{newMockActivity(1, actTime)}, nil
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		IssueKeys: []string{"HEP-1"},
		Since:     &testSince,
		Until:     &testUntil,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if ud.Summary.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", ud.Summary.IssueCount)
	}
	if ud.Summary.ActivityCount != 1 {
		t.Errorf("ActivityCount = %d, want 1", ud.Summary.ActivityCount)
	}
}

// ---- D5: mixed (project + user AND condition) ----

func TestBuild_mixed(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.GetProjectFunc = func(_ context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "HEP", Name: "ヘプタゴン"}, nil
	}
	mc.GetUserFunc = func(_ context.Context, _ string) (*domain.User, error) {
		return &domain.User{ID: 42, Name: "Ishizawa"}, nil
	}
	mc.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		// project=HEP(ID=1), assignee=42 の AND 条件
		if len(opt.ProjectIDs) > 0 && len(opt.AssigneeIDs) > 0 {
			return []domain.Issue{newMockIssue(5, 1, 42)}, nil
		}
		return []domain.Issue{}, nil
	}
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{newMockActivity(10, actTime)}, nil
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		ProjectKeys: []string{"HEP"},
		UserIDs:     []int{42},
		Since:       &testSince,
		Until:       &testUntil,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if ud.Summary.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", ud.Summary.IssueCount)
	}
}

// ---- D5b: activities フルデータ保持確認 ----

// TestBuild_activityFullData は activities に type, content, createdUser が含まれることを確認する。
// activitiesToInterface が id と created しか含めない場合にこのテストが失敗する（Red）。
func TestBuild_activityFullData(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 23, 7, 24, 56, 0, time.UTC)

	mc.ListSpaceActivitiesFunc = func(_ context.Context, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{
				ID:   914382588,
				Type: 3,
				Content: map[string]interface{}{
					"id":      float64(141884975),
					"key_id":  float64(1158),
					"summary": "生成AI利用制度 再設計",
					"comment": map[string]interface{}{
						"id":      float64(706275735),
						"content": "team契約進める...",
					},
				},
				CreatedUser: &domain.User{ID: 1537084, Name: "Naoto Ishizawa"},
				Created:     &actTime,
			},
		}, nil
	}
	mc.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	since := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 3, 23, 23, 59, 59, 0, time.UTC)
	scope := digest.UnifiedDigestScope{
		Since: &since,
		Until: &until,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if len(ud.Activities) != 1 {
		t.Fatalf("Activities count = %d, want 1", len(ud.Activities))
	}

	m, ok := ud.Activities[0].(map[string]interface{})
	if !ok {
		t.Fatalf("activity is not map[string]interface{}, got %T", ud.Activities[0])
	}

	// type フィールドが存在することを確認
	if _, exists := m["type"]; !exists {
		t.Error("activity missing 'type' field")
	}
	if typeVal, ok := m["type"]; ok {
		if typeVal != 3 && typeVal != float64(3) {
			t.Errorf("activity 'type' = %v, want 3", typeVal)
		}
	}

	// content フィールドが存在することを確認
	if _, exists := m["content"]; !exists {
		t.Error("activity missing 'content' field")
	}
	if content, ok := m["content"].(map[string]interface{}); ok {
		if summary, ok := content["summary"].(string); !ok || summary != "生成AI利用制度 再設計" {
			t.Errorf("activity content.summary = %v, want '生成AI利用制度 再設計'", content["summary"])
		}
	} else {
		t.Errorf("activity 'content' is not map[string]interface{}, got %T", m["content"])
	}

	// createdUser フィールドが存在することを確認
	if _, exists := m["createdUser"]; !exists {
		t.Error("activity missing 'createdUser' field")
	}
}

// ---- D6: partial success (issues OK, activities NG) ----

func TestBuild_partialSuccess(t *testing.T) {
	mc := backlog.NewMockClient()

	mc.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{newMockIssue(1, 0, 0)}, nil
	}
	// ListSpaceActivitiesFunc をセットしない → ErrNotFound
	mc.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return nil, errors.New("activities API error")
	}

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		Since: &testSince,
		Until: &testUntil,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() should return envelope even on partial failure, got error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if ud.Summary.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", ud.Summary.IssueCount)
	}
	if ud.Summary.ActivityCount != 0 {
		t.Errorf("ActivityCount = %d, want 0 (activities failed)", ud.Summary.ActivityCount)
	}
	if len(env.Warnings) == 0 {
		t.Error("expected at least 1 warning for activities failure")
	}
}
