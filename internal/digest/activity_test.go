package digest_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

func TestActivityDigestBuilder_Build_space_activities(t *testing.T) {
	now := time.Now().UTC()
	mock := backlog.NewMockClient()
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{
				ID:      1,
				Type:    1, // issue_created
				Created: &now,
				CreatedUser: &domain.User{ID: 10, Name: "Alice"},
				Content: map[string]interface{}{
					"id":      float64(100),
					"key":     "PROJ-1",
					"summary": "テスト課題",
				},
			},
			{
				ID:      2,
				Type:    3, // issue_commented
				Created: &now,
				CreatedUser: &domain.User{ID: 10, Name: "Alice"},
				Content: map[string]interface{}{
					"id":      float64(101),
					"key":     "PROJ-2",
					"summary": "コメント付き課題",
					"comment": map[string]interface{}{
						"id":      float64(999),
						"content": "テストコメント",
					},
				},
			},
		}, nil
	}

	builder := digest.NewDefaultActivityDigestBuilder(mock, "default", "example-space", "https://example.backlog.com")
	opt := digest.ActivityDigestOptions{Limit: 10}

	env, err := builder.Build(context.Background(), opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	if env.Resource != "activity" {
		t.Errorf("Resource = %q, want %q", env.Resource, "activity")
	}

	ad, ok := env.Digest.(*digest.ActivityDigest)
	if !ok {
		t.Fatalf("Digest が *ActivityDigest でない: %T", env.Digest)
	}

	if len(ad.Activities) != 2 {
		t.Errorf("Activities len = %d, want 2", len(ad.Activities))
	}
	// type 番号が文字列に変換されているか
	if ad.Activities[0].Type != "issue_created" {
		t.Errorf("Activities[0].Type = %q, want %q", ad.Activities[0].Type, "issue_created")
	}
	// コメントが抽出されているか
	if len(ad.Comments) != 1 {
		t.Errorf("Comments len = %d, want 1", len(ad.Comments))
	}
	if ad.Summary.TotalActivity != 2 {
		t.Errorf("Summary.TotalActivity = %d, want 2", ad.Summary.TotalActivity)
	}
	if ad.Summary.CommentCount != 1 {
		t.Errorf("Summary.CommentCount = %d, want 1", ad.Summary.CommentCount)
	}
}

func TestActivityDigestBuilder_Build_project_activities(t *testing.T) {
	now := time.Now().UTC()
	mock := backlog.NewMockClient()
	mock.ListProjectActivitiesFunc = func(_ context.Context, projectKey string, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		if projectKey != "PROJ" {
			t.Errorf("projectKey = %q, want %q", projectKey, "PROJ")
		}
		return []domain.Activity{
			{
				ID:          3,
				Type:        2, // issue_updated
				Created:     &now,
				CreatedUser: &domain.User{ID: 11, Name: "Bob"},
				Content: map[string]interface{}{
					"id":  float64(200),
					"key": "PROJ-5",
				},
			},
		}, nil
	}

	builder := digest.NewDefaultActivityDigestBuilder(mock, "default", "example-space", "https://example.backlog.com")
	opt := digest.ActivityDigestOptions{Project: "PROJ", Limit: 10}

	env, err := builder.Build(context.Background(), opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	ad, ok := env.Digest.(*digest.ActivityDigest)
	if !ok {
		t.Fatalf("Digest が *ActivityDigest でない: %T", env.Digest)
	}

	if len(ad.Activities) != 1 {
		t.Errorf("Activities len = %d, want 1", len(ad.Activities))
	}
	if ad.Activities[0].Type != "issue_updated" {
		t.Errorf("Activities[0].Type = %q, want %q", ad.Activities[0].Type, "issue_updated")
	}
	if ad.Scope.Project != "PROJ" {
		t.Errorf("Scope.Project = %q, want %q", ad.Scope.Project, "PROJ")
	}
	// ListSpaceActivities が呼ばれていないことを確認
	if mock.GetCallCount("ListSpaceActivities") != 0 {
		t.Error("ListSpaceActivities が呼ばれた（Project 指定時は呼ばれるべきでない）")
	}
}

func TestActivityDigestBuilder_Build_activities_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()
	// ListSpaceActivities は設定しない（デフォルトで ErrNotFound を返す）

	builder := digest.NewDefaultActivityDigestBuilder(mock, "default", "example-space", "https://example.backlog.com")
	opt := digest.ActivityDigestOptions{Limit: 10}

	// アクティビティ取得失敗は partial success として DigestEnvelope を返す
	env, err := builder.Build(context.Background(), opt)
	if err != nil {
		t.Fatalf("Build() エラー（partial success のため error にならないはず）: %v", err)
	}

	if len(env.Warnings) == 0 {
		t.Error("warnings が空（アクティビティ取得失敗の warning が期待される）")
	}

	ad, ok := env.Digest.(*digest.ActivityDigest)
	if !ok {
		t.Fatalf("Digest が *ActivityDigest でない: %T", env.Digest)
	}
	if len(ad.Activities) != 0 {
		t.Errorf("Activities len = %d, want 0", len(ad.Activities))
	}
}

func TestActivityDigestBuilder_Build_empty_activities(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	builder := digest.NewDefaultActivityDigestBuilder(mock, "default", "example-space", "https://example.backlog.com")
	opt := digest.ActivityDigestOptions{Limit: 10}

	env, err := builder.Build(context.Background(), opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	ad, ok := env.Digest.(*digest.ActivityDigest)
	if !ok {
		t.Fatalf("Digest が *ActivityDigest でない: %T", env.Digest)
	}
	if len(ad.Activities) != 0 {
		t.Errorf("Activities len = %d, want 0", len(ad.Activities))
	}
	if len(ad.Comments) != 0 {
		t.Errorf("Comments len = %d, want 0", len(ad.Comments))
	}
	if ad.Summary.TotalActivity != 0 {
		t.Errorf("Summary.TotalActivity = %d, want 0", ad.Summary.TotalActivity)
	}
}

func TestActivityDigestBuilder_Build_with_comments(t *testing.T) {
	now := time.Now().UTC()
	mock := backlog.NewMockClient()
	mock.ListSpaceActivitiesFunc = func(_ context.Context, _ backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{
				ID:          5,
				Type:        3, // issue_commented
				Created:     &now,
				CreatedUser: &domain.User{ID: 20, Name: "Charlie"},
				Content: map[string]interface{}{
					"id":      float64(300),
					"key":     "TEST-10",
					"summary": "コメント課題",
					"comment": map[string]interface{}{
						"id":      float64(555),
						"content": "レビューコメント",
					},
				},
			},
		}, nil
	}

	builder := digest.NewDefaultActivityDigestBuilder(mock, "default", "example-space", "https://example.backlog.com")
	opt := digest.ActivityDigestOptions{Limit: 10}

	env, err := builder.Build(context.Background(), opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	ad, ok := env.Digest.(*digest.ActivityDigest)
	if !ok {
		t.Fatalf("Digest が *ActivityDigest でない: %T", env.Digest)
	}

	if len(ad.Comments) != 1 {
		t.Fatalf("Comments len = %d, want 1", len(ad.Comments))
	}
	if ad.Comments[0].Content != "レビューコメント" {
		t.Errorf("Comments[0].Content = %q, want %q", ad.Comments[0].Content, "レビューコメント")
	}
	if ad.Comments[0].ID != 555 {
		t.Errorf("Comments[0].ID = %d, want 555", ad.Comments[0].ID)
	}
}
