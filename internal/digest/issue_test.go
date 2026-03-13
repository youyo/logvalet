package digest_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// testTime は固定の時刻（テストで決定論的な出力を得るため）。
var testTime = time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)

func newTestIssue() *domain.Issue {
	status := &domain.IDName{ID: 1, Name: "処理中"}
	priority := &domain.IDName{ID: 2, Name: "高"}
	issueType := &domain.IDName{ID: 3, Name: "バグ"}
	assignee := &domain.User{ID: 100, UserID: "taro", Name: "Taro Yamada"}
	reporter := &domain.User{ID: 101, UserID: "hanako", Name: "Hanako Suzuki"}
	return &domain.Issue{
		ID:          42,
		ProjectID:   1000,
		IssueKey:    "PROJ-123",
		Summary:     "テストバグ修正",
		Description: "バグの詳細説明",
		Status:      status,
		Priority:    priority,
		IssueType:   issueType,
		Assignee:    assignee,
		Reporter:    reporter,
		Categories:  []domain.IDName{{ID: 10, Name: "frontend"}},
		Versions:    []domain.IDName{{ID: 20, Name: "v1.0"}},
		Milestones:  []domain.IDName{{ID: 30, Name: "v1.2.0"}},
		Created:     &testTime,
		Updated:     &testTime,
	}
}

func newTestProject() *domain.Project {
	return &domain.Project{
		ID:         1000,
		ProjectKey: "PROJ",
		Name:       "Example Project",
		Archived:   false,
	}
}

func newTestComments() []domain.Comment {
	t1 := testTime.Add(-1 * time.Hour)
	t2 := testTime.Add(-2 * time.Hour)
	return []domain.Comment{
		{ID: 1, Content: "最初のコメント", CreatedUser: &domain.User{ID: 100, Name: "Taro Yamada"}, Created: &t1},
		{ID: 2, Content: "2番目のコメント", CreatedUser: &domain.User{ID: 101, Name: "Hanako Suzuki"}, Created: &t2},
	}
}

// TestDefaultIssueDigestBuilder_Build_success は正常系のテスト。
func TestDefaultIssueDigestBuilder_Build_success(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		if issueKey != "PROJ-123" {
			t.Errorf("unexpected issueKey: %s", issueKey)
		}
		return newTestIssue(), nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProject(), nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return newTestComments(), nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{{ID: 1, Name: "処理中"}, {ID: 2, Name: "完了"}}, nil
	}
	mc.ListProjectCategoriesFunc = func(ctx context.Context, projectKey string) ([]domain.Category, error) {
		return []domain.Category{{ID: 10, Name: "frontend"}}, nil
	}
	mc.ListProjectVersionsFunc = func(ctx context.Context, projectKey string) ([]domain.Version, error) {
		return []domain.Version{{ID: 20, Name: "v1.0"}, {ID: 30, Name: "v1.2.0"}}, nil
	}
	mc.ListProjectCustomFieldsFunc = func(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
		return nil, nil
	}

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5, IncludeActivity: false}

	env, err := builder.Build(context.Background(), "PROJ-123", opts)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if env.Resource != "issue" {
		t.Errorf("envelope.Resource = %q; want %q", env.Resource, "issue")
	}
	if env.Profile != "work" {
		t.Errorf("envelope.Profile = %q; want %q", env.Profile, "work")
	}
	if env.Space != "example-space" {
		t.Errorf("envelope.Space = %q; want %q", env.Space, "example-space")
	}
	if len(env.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(env.Warnings))
	}

	d, ok := env.Digest.(*digest.IssueDigest)
	if !ok {
		t.Fatalf("env.Digest type assertion to *digest.IssueDigest failed; got %T", env.Digest)
	}

	if d.Issue.Key != "PROJ-123" {
		t.Errorf("IssueDigest.Issue.Key = %q; want %q", d.Issue.Key, "PROJ-123")
	}
	if d.Issue.Summary != "テストバグ修正" {
		t.Errorf("IssueDigest.Issue.Summary = %q; want %q", d.Issue.Summary, "テストバグ修正")
	}
	if d.Project.Key != "PROJ" {
		t.Errorf("IssueDigest.Project.Key = %q; want %q", d.Project.Key, "PROJ")
	}
	if len(d.Comments) != 2 {
		t.Errorf("IssueDigest.Comments len = %d; want 2", len(d.Comments))
	}
	if d.Summary.HasAssignee != true {
		t.Error("IssueDigest.Summary.HasAssignee should be true")
	}
	if d.Summary.StatusName != "処理中" {
		t.Errorf("IssueDigest.Summary.StatusName = %q; want %q", d.Summary.StatusName, "処理中")
	}
}

// TestDefaultIssueDigestBuilder_Build_issueNotFound は課題が見つからない場合のテスト。
func TestDefaultIssueDigestBuilder_Build_issueNotFound(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return nil, backlog.ErrNotFound
	}

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5}

	_, err := builder.Build(context.Background(), "PROJ-999", opts)
	if err == nil {
		t.Fatal("Build() should return error when issue not found")
	}
}

// TestDefaultIssueDigestBuilder_Build_projectNotFound はプロジェクトが見つからない場合のテスト。
func TestDefaultIssueDigestBuilder_Build_projectNotFound(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return newTestIssue(), nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return nil, backlog.ErrNotFound
	}

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5}

	_, err := builder.Build(context.Background(), "PROJ-123", opts)
	if err == nil {
		t.Fatal("Build() should return error when project not found")
	}
}

// TestDefaultIssueDigestBuilder_Build_commentsWarning はコメント取得失敗時のwarningテスト。
func TestDefaultIssueDigestBuilder_Build_commentsWarning(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return newTestIssue(), nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProject(), nil
	}
	// ListIssueCommentsFunc をセットしない → ErrNotFound が返る

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5}

	env, err := builder.Build(context.Background(), "PROJ-123", opts)
	if err != nil {
		t.Fatalf("Build() should succeed with partial data, got error: %v", err)
	}

	// コメントが空であること
	d, ok := env.Digest.(*digest.IssueDigest)
	if !ok {
		t.Fatalf("type assertion failed")
	}
	if len(d.Comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(d.Comments))
	}

	// warningが少なくとも1件あること
	if len(env.Warnings) == 0 {
		t.Error("expected at least 1 warning when comments fetch fails")
	}
}

// TestDefaultIssueDigestBuilder_Build_metaWarnings はメタ情報取得失敗時のwarningテスト。
func TestDefaultIssueDigestBuilder_Build_metaWarnings(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return newTestIssue(), nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProject(), nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return nil, nil
	}
	// ステータス/カテゴリ/バージョン/カスタムフィールドをセットしない → warning

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5}

	env, err := builder.Build(context.Background(), "PROJ-123", opts)
	if err != nil {
		t.Fatalf("Build() should succeed with partial meta, got error: %v", err)
	}

	// warningが少なくとも1件あること（ステータス/カテゴリ/バージョンのいずれかが失敗）
	if len(env.Warnings) == 0 {
		t.Error("expected warnings when meta fetches fail")
	}
}

// TestDefaultIssueDigestBuilder_Build_noAssignee は担当者なしのテスト。
func TestDefaultIssueDigestBuilder_Build_noAssignee(t *testing.T) {
	mc := backlog.NewMockClient()
	issue := newTestIssue()
	issue.Assignee = nil
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return issue, nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProject(), nil
	}

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5}

	env, err := builder.Build(context.Background(), "PROJ-123", opts)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	d := env.Digest.(*digest.IssueDigest)
	if d.Summary.HasAssignee != false {
		t.Error("IssueDigest.Summary.HasAssignee should be false when no assignee")
	}
}

// TestGolden は Golden test。testdata/issue_digest.golden と出力を比較する。
// UPDATE_GOLDEN=1 環境変数があればファイルを更新する。
func TestGolden(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return newTestIssue(), nil
	}
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProject(), nil
	}
	mc.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return newTestComments(), nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{{ID: 1, Name: "処理中"}, {ID: 2, Name: "完了"}}, nil
	}
	mc.ListProjectCategoriesFunc = func(ctx context.Context, projectKey string) ([]domain.Category, error) {
		return []domain.Category{{ID: 10, Name: "frontend"}}, nil
	}
	mc.ListProjectVersionsFunc = func(ctx context.Context, projectKey string) ([]domain.Version, error) {
		return []domain.Version{{ID: 20, Name: "v1.0"}, {ID: 30, Name: "v1.2.0"}}, nil
	}
	mc.ListProjectCustomFieldsFunc = func(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
		return nil, nil
	}

	builder := digest.NewDefaultIssueDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.IssueDigestOptions{MaxComments: 5, IncludeActivity: false}

	env, err := builder.Build(context.Background(), "PROJ-123", opts)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// GeneratedAt を固定値に上書き（決定論的な Golden test のため）
	env.GeneratedAt = testTime

	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "issue_digest.golden")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, out, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("golden file updated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v (run with UPDATE_GOLDEN=1 to create)", goldenPath, err)
	}

	if string(out) != string(want) {
		t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", out, want)
	}
}
