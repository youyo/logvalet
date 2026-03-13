package digest_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

func TestUserDigestBuilder_Build_success(t *testing.T) {
	now := time.Now().UTC()
	mock := backlog.NewMockClient()

	mock.GetUserFunc = func(_ context.Context, userID string) (*domain.User, error) {
		if userID != "12345" {
			t.Errorf("GetUser userID = %q, want %q", userID, "12345")
		}
		return &domain.User{ID: 12345, Name: "Naoto Ishizawa"}, nil
	}
	mock.ListUserActivitiesFunc = func(_ context.Context, userID string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{
				ID:          10,
				Type:        1, // issue_created
				Created:     &now,
				CreatedUser: &domain.User{ID: 12345, Name: "Naoto Ishizawa"},
				Content: map[string]interface{}{
					"id":      float64(500),
					"key":     "PROJ-123",
					"summary": "新規課題",
				},
			},
			{
				ID:          11,
				Type:        3, // issue_commented
				Created:     &now,
				CreatedUser: &domain.User{ID: 12345, Name: "Naoto Ishizawa"},
				Content: map[string]interface{}{
					"id":      float64(501),
					"key":     "PROJ-140",
					"summary": "コメント課題",
					"comment": map[string]interface{}{
						"id":      float64(777),
						"content": "進捗コメント",
					},
				},
			},
		}, nil
	}

	builder := digest.NewDefaultUserDigestBuilder(mock, "work", "example-space", "https://example.backlog.com")
	opt := digest.UserDigestOptions{Limit: 20}

	env, err := builder.Build(context.Background(), "12345", opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	if env.Resource != "user" {
		t.Errorf("Resource = %q, want %q", env.Resource, "user")
	}

	ud, ok := env.Digest.(*digest.UserDigest)
	if !ok {
		t.Fatalf("Digest が *UserDigest でない: %T", env.Digest)
	}

	if ud.User.ID != 12345 {
		t.Errorf("User.ID = %d, want 12345", ud.User.ID)
	}
	if ud.User.Name != "Naoto Ishizawa" {
		t.Errorf("User.Name = %q, want %q", ud.User.Name, "Naoto Ishizawa")
	}

	if len(ud.Activities) != 2 {
		t.Errorf("Activities len = %d, want 2", len(ud.Activities))
	}
	if ud.Activities[0].Type != "issue_created" {
		t.Errorf("Activities[0].Type = %q, want %q", ud.Activities[0].Type, "issue_created")
	}

	if len(ud.Comments) != 1 {
		t.Errorf("Comments len = %d, want 1", len(ud.Comments))
	}
	if ud.Comments[0].Content != "進捗コメント" {
		t.Errorf("Comments[0].Content = %q, want %q", ud.Comments[0].Content, "進捗コメント")
	}

	if ud.Summary.TotalActivity != 2 {
		t.Errorf("Summary.TotalActivity = %d, want 2", ud.Summary.TotalActivity)
	}
	if ud.Summary.CommentCount != 1 {
		t.Errorf("Summary.CommentCount = %d, want 1", ud.Summary.CommentCount)
	}
	if ud.Summary.Headline == "" {
		t.Error("Summary.Headline が空")
	}
}

func TestUserDigestBuilder_Build_user_not_found(t *testing.T) {
	mock := backlog.NewMockClient()
	// GetUserFunc を設定しない（デフォルトで ErrNotFound）

	builder := digest.NewDefaultUserDigestBuilder(mock, "work", "example-space", "https://example.backlog.com")
	opt := digest.UserDigestOptions{Limit: 20}

	_, err := builder.Build(context.Background(), "99999", opt)
	if err == nil {
		t.Fatal("Build() が error を返さなかった（GetUser 失敗は必須エラーのはず）")
	}
}

func TestUserDigestBuilder_Build_activities_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetUserFunc = func(_ context.Context, _ string) (*domain.User, error) {
		return &domain.User{ID: 1, Name: "TestUser"}, nil
	}
	// ListUserActivitiesFunc を設定しない（デフォルトで ErrNotFound）

	builder := digest.NewDefaultUserDigestBuilder(mock, "work", "example-space", "https://example.backlog.com")
	opt := digest.UserDigestOptions{Limit: 20}

	env, err := builder.Build(context.Background(), "1", opt)
	if err != nil {
		t.Fatalf("Build() エラー（partial success のはず）: %v", err)
	}

	if len(env.Warnings) == 0 {
		t.Error("warnings が空（アクティビティ取得失敗の warning が期待される）")
	}

	ud, ok := env.Digest.(*digest.UserDigest)
	if !ok {
		t.Fatalf("Digest が *UserDigest でない: %T", env.Digest)
	}
	if len(ud.Activities) != 0 {
		t.Errorf("Activities len = %d, want 0", len(ud.Activities))
	}
}

func TestUserDigestBuilder_Build_empty_activities(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetUserFunc = func(_ context.Context, _ string) (*domain.User, error) {
		return &domain.User{ID: 2, Name: "EmptyUser"}, nil
	}
	mock.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	builder := digest.NewDefaultUserDigestBuilder(mock, "work", "example-space", "https://example.backlog.com")
	opt := digest.UserDigestOptions{Limit: 20}

	env, err := builder.Build(context.Background(), "2", opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	ud, ok := env.Digest.(*digest.UserDigest)
	if !ok {
		t.Fatalf("Digest が *UserDigest でない: %T", env.Digest)
	}
	if len(ud.Activities) != 0 {
		t.Errorf("Activities len = %d, want 0", len(ud.Activities))
	}
	if len(ud.Comments) != 0 {
		t.Errorf("Comments len = %d, want 0", len(ud.Comments))
	}
	if ud.Summary.TotalActivity != 0 {
		t.Errorf("Summary.TotalActivity = %d, want 0", ud.Summary.TotalActivity)
	}
}

func TestUserDigestBuilder_Build_with_comments(t *testing.T) {
	now := time.Now().UTC()
	mock := backlog.NewMockClient()
	mock.GetUserFunc = func(_ context.Context, _ string) (*domain.User, error) {
		return &domain.User{ID: 3, Name: "Commenter"}, nil
	}
	mock.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{
			{
				ID:          20,
				Type:        3, // issue_commented
				Created:     &now,
				CreatedUser: &domain.User{ID: 3, Name: "Commenter"},
				Content: map[string]interface{}{
					"id":      float64(600),
					"key":     "OPS-5",
					"summary": "障害対応",
					"comment": map[string]interface{}{
						"id":      float64(888),
						"content": "修正完了しました",
					},
				},
			},
		}, nil
	}

	builder := digest.NewDefaultUserDigestBuilder(mock, "work", "example-space", "https://example.backlog.com")
	opt := digest.UserDigestOptions{Limit: 20}

	env, err := builder.Build(context.Background(), "3", opt)
	if err != nil {
		t.Fatalf("Build() エラー: %v", err)
	}

	ud, ok := env.Digest.(*digest.UserDigest)
	if !ok {
		t.Fatalf("Digest が *UserDigest でない: %T", env.Digest)
	}

	if len(ud.Comments) != 1 {
		t.Fatalf("Comments len = %d, want 1", len(ud.Comments))
	}
	if ud.Comments[0].Content != "修正完了しました" {
		t.Errorf("Comments[0].Content = %q, want %q", ud.Comments[0].Content, "修正完了しました")
	}
	if ud.Comments[0].ID != 888 {
		t.Errorf("Comments[0].ID = %d, want 888", ud.Comments[0].ID)
	}
}
