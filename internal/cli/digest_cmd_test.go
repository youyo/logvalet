package cli_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// TestDigestCmd_buildScope_projectOnly は --project フラグのみのスコープ構築テスト。
// DigestCmd は直接インスタンス化できないため、
// UnifiedDigestBuilder を使ったインテグレーションテストで動作を検証する。
func TestDigestCmd_buildScope_projectOnly(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.GetProjectFunc = func(_ context.Context, key string) (*domain.Project, error) {
		if key == "HEP" {
			return &domain.Project{ID: 1, ProjectKey: "HEP", Name: "ヘプタゴン"}, nil
		}
		return nil, backlog.ErrNotFound
	}
	mc.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		if len(opt.ProjectIDs) > 0 && opt.ProjectIDs[0] == 1 {
			return []domain.Issue{
				{ID: 1, IssueKey: "HEP-1", Summary: "課題1", ProjectID: 1},
			}, nil
		}
		return []domain.Issue{}, nil
	}
	mc.ListProjectActivitiesFunc = func(_ context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{ID: 100, Type: 1, Created: &actTime},
		}, nil
	}

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")

	// GetProject でプロジェクトを解決してから scope を構築
	proj, err := mc.GetProject(context.Background(), "HEP")
	if err != nil {
		t.Fatalf("GetProject error: %v", err)
	}

	scope := digest.UnifiedDigestScope{
		ProjectKeys: []string{"HEP"},
		ProjectIDs:  []int{proj.ID},
		Since:       &since,
		Until:       &until,
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

	if ud.Summary.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", ud.Summary.IssueCount)
	}
	if ud.Summary.ActivityCount != 1 {
		t.Errorf("ActivityCount = %d, want 1", ud.Summary.ActivityCount)
	}
	if ud.Scope.Since != "2026-03-01" {
		t.Errorf("Scope.Since = %q, want %q", ud.Scope.Since, "2026-03-01")
	}
}

// TestDigestCmd_buildScope_userAndProject は --user + --project のテスト。
func TestDigestCmd_buildScope_userAndProject(t *testing.T) {
	mc := backlog.NewMockClient()
	actTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	mc.GetProjectFunc = func(_ context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ID: 2, ProjectKey: "DEV", Name: "Dev"}, nil
	}
	mc.GetUserFunc = func(_ context.Context, userID string) (*domain.User, error) {
		return &domain.User{ID: 99, Name: "Taro"}, nil
	}
	mc.ListIssuesFunc = func(_ context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{
			{ID: 10, IssueKey: "DEV-10", Summary: "課題10", ProjectID: 2},
			{ID: 11, IssueKey: "DEV-11", Summary: "課題11", ProjectID: 2},
		}, nil
	}
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{ID: 200, Type: 2, Created: &actTime},
		}, nil
	}

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)

	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{
		ProjectKeys: []string{"DEV"},
		UserIDs:     []int{99},
		Since:       &since,
		Until:       &until,
	}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	ud, ok := env.Digest.(*digest.UnifiedDigest)
	if !ok {
		t.Fatalf("env.Digest is not *digest.UnifiedDigest")
	}

	if ud.Summary.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2", ud.Summary.IssueCount)
	}
	if ud.Summary.ActivityCount != 1 {
		t.Errorf("ActivityCount = %d, want 1", ud.Summary.ActivityCount)
	}
}

// TestDigestCmd_outputIsJSON は出力 JSON が正しく marshal できることを確認する。
func TestDigestCmd_outputIsJSON(t *testing.T) {
	mc := backlog.NewMockClient()

	mc.ListIssuesFunc = func(_ context.Context, _ backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}
	mc.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	b := digest.NewUnifiedDigestBuilder(mc, "work", "heptagon", "https://heptagon.backlog.com")
	scope := digest.UnifiedDigestScope{Since: &since}

	env, err := b.Build(context.Background(), scope)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if parsed["resource"] != "digest" {
		t.Errorf("JSON resource = %v, want %q", parsed["resource"], "digest")
	}
	if _, ok := parsed["digest"]; !ok {
		t.Error("JSON missing 'digest' field")
	}
	if _, ok := parsed["schema_version"]; !ok {
		t.Error("JSON missing 'schema_version' field")
	}
}
