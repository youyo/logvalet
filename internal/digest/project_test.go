package digest_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

func newTestProjectForDigest() *domain.Project {
	return &domain.Project{
		ID:         1000,
		ProjectKey: "PROJ",
		Name:       "Example Project",
		Archived:   false,
	}
}

// TestDefaultProjectDigestBuilder_Build_success は正常系のテスト。
func TestDefaultProjectDigestBuilder_Build_success(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		if projectKey != "PROJ" {
			t.Errorf("unexpected projectKey: %s", projectKey)
		}
		return newTestProjectForDigest(), nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{
			{ID: 1, ProjectID: 1000, Name: "処理中", DisplayOrder: 1},
			{ID: 2, ProjectID: 1000, Name: "完了", DisplayOrder: 2},
		}, nil
	}
	mc.ListProjectCategoriesFunc = func(ctx context.Context, projectKey string) ([]domain.Category, error) {
		return []domain.Category{{ID: 10, Name: "frontend", DisplayOrder: 1}}, nil
	}
	mc.ListProjectVersionsFunc = func(ctx context.Context, projectKey string) ([]domain.Version, error) {
		return []domain.Version{{ID: 20, ProjectID: 1000, Name: "v1.0", Archived: false}}, nil
	}
	mc.ListProjectCustomFieldsFunc = func(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
		return []domain.CustomFieldDefinition{{ID: 5, TypeID: 1, Name: "担当チーム", Required: false}}, nil
	}
	mc.ListProjectTeamsFunc = func(ctx context.Context, projectKey string) ([]domain.Team, error) {
		return []domain.Team{{ID: 1, Name: "Alpha Team"}}, nil
	}
	mc.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	builder := digest.NewDefaultProjectDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.ProjectDigestOptions{}

	env, err := builder.Build(context.Background(), "PROJ", opts)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if env.Resource != "project" {
		t.Errorf("envelope.Resource = %q; want %q", env.Resource, "project")
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

	d, ok := env.Digest.(*digest.ProjectDigest)
	if !ok {
		t.Fatalf("env.Digest type assertion to *digest.ProjectDigest failed; got %T", env.Digest)
	}

	if d.Project.Key != "PROJ" {
		t.Errorf("ProjectDigest.Project.Key = %q; want %q", d.Project.Key, "PROJ")
	}
	if d.Project.Name != "Example Project" {
		t.Errorf("ProjectDigest.Project.Name = %q; want %q", d.Project.Name, "Example Project")
	}
	if len(d.Teams) != 1 {
		t.Errorf("ProjectDigest.Teams len = %d; want 1", len(d.Teams))
	}
	if len(d.Meta.Statuses) != 2 {
		t.Errorf("ProjectDigest.Meta.Statuses len = %d; want 2", len(d.Meta.Statuses))
	}
	if len(d.Meta.Categories) != 1 {
		t.Errorf("ProjectDigest.Meta.Categories len = %d; want 1", len(d.Meta.Categories))
	}
	if len(d.Meta.Versions) != 1 {
		t.Errorf("ProjectDigest.Meta.Versions len = %d; want 1", len(d.Meta.Versions))
	}
	if len(d.Meta.CustomFields) != 1 {
		t.Errorf("ProjectDigest.Meta.CustomFields len = %d; want 1", len(d.Meta.CustomFields))
	}
	if d.Summary.TeamCount != 1 {
		t.Errorf("ProjectDigest.Summary.TeamCount = %d; want 1", d.Summary.TeamCount)
	}
	if d.Summary.StatusCount != 2 {
		t.Errorf("ProjectDigest.Summary.StatusCount = %d; want 2", d.Summary.StatusCount)
	}
	// headline が プロジェクトキーを含むこと
	if d.Summary.Headline == "" {
		t.Error("ProjectDigest.Summary.Headline should not be empty")
	}
}

// TestDefaultProjectDigestBuilder_Build_projectNotFound はプロジェクトが見つからない場合のテスト。
func TestDefaultProjectDigestBuilder_Build_projectNotFound(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return nil, backlog.ErrNotFound
	}

	builder := digest.NewDefaultProjectDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.ProjectDigestOptions{}

	_, err := builder.Build(context.Background(), "NOTEXIST", opts)
	if err == nil {
		t.Fatal("Build() should return error when project not found")
	}
}

// TestDefaultProjectDigestBuilder_Build_teamsWarning はチーム取得失敗時のwarningテスト。
func TestDefaultProjectDigestBuilder_Build_teamsWarning(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProjectForDigest(), nil
	}
	// ListProjectTeamsFunc をセットしない → ErrNotFound → warning

	builder := digest.NewDefaultProjectDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.ProjectDigestOptions{}

	env, err := builder.Build(context.Background(), "PROJ", opts)
	if err != nil {
		t.Fatalf("Build() should succeed with partial data, got error: %v", err)
	}

	// チームが空であること
	d, ok := env.Digest.(*digest.ProjectDigest)
	if !ok {
		t.Fatalf("type assertion failed")
	}
	if len(d.Teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(d.Teams))
	}

	// warningが少なくとも1件あること
	if len(env.Warnings) == 0 {
		t.Error("expected at least 1 warning when teams fetch fails")
	}
}

// TestDefaultProjectDigestBuilder_Build_metaWarnings はメタ情報取得失敗時のwarningテスト。
func TestDefaultProjectDigestBuilder_Build_metaWarnings(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProjectForDigest(), nil
	}
	mc.ListProjectTeamsFunc = func(ctx context.Context, projectKey string) ([]domain.Team, error) {
		return []domain.Team{}, nil
	}
	// ステータス/カテゴリ/バージョン/カスタムフィールドをセットしない → warning

	builder := digest.NewDefaultProjectDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.ProjectDigestOptions{}

	env, err := builder.Build(context.Background(), "PROJ", opts)
	if err != nil {
		t.Fatalf("Build() should succeed with partial meta, got error: %v", err)
	}

	// warningが少なくとも1件あること（ステータス/カテゴリ/バージョンのいずれかが失敗）
	if len(env.Warnings) == 0 {
		t.Error("expected warnings when meta fetches fail")
	}
}

// TestProjectGolden は Golden test。testdata/project_digest.golden と出力を比較する。
// UPDATE_GOLDEN=1 環境変数があればファイルを更新する。
func TestProjectGolden(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return newTestProjectForDigest(), nil
	}
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{
			{ID: 1, ProjectID: 1000, Name: "処理中", DisplayOrder: 1},
			{ID: 2, ProjectID: 1000, Name: "完了", DisplayOrder: 2},
		}, nil
	}
	mc.ListProjectCategoriesFunc = func(ctx context.Context, projectKey string) ([]domain.Category, error) {
		return []domain.Category{{ID: 10, Name: "frontend", DisplayOrder: 1}}, nil
	}
	mc.ListProjectVersionsFunc = func(ctx context.Context, projectKey string) ([]domain.Version, error) {
		return []domain.Version{{ID: 20, ProjectID: 1000, Name: "v1.0", Archived: false}}, nil
	}
	mc.ListProjectCustomFieldsFunc = func(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
		return []domain.CustomFieldDefinition{{ID: 5, TypeID: 1, Name: "担当チーム", Required: false}}, nil
	}
	mc.ListProjectTeamsFunc = func(ctx context.Context, projectKey string) ([]domain.Team, error) {
		return []domain.Team{{ID: 1, Name: "Alpha Team"}}, nil
	}
	mc.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	builder := digest.NewDefaultProjectDigestBuilder(mc, "work", "example-space", "https://example-space.backlog.com")
	opts := digest.ProjectDigestOptions{}

	env, err := builder.Build(context.Background(), "PROJ", opts)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// GeneratedAt を固定値に上書き（決定論的な Golden test のため）
	env.GeneratedAt = testTime

	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "project_digest.golden")

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
