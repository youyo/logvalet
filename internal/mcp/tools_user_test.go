package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== B1: logvalet_user_me =====

// TestUserMe_Normal は GetMyself が呼ばれてユーザーが返ることを確認する。
func TestUserMe_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 42, UserID: "testuser", Name: "テストユーザー"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_me", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetMyself") != 1 {
		t.Errorf("expected GetMyself called 1 time, got %d", mock.GetCallCount("GetMyself"))
	}
}

// TestUserMe_Error は GetMyself がエラーを返した場合に IsError=true になることを確認する。
func TestUserMe_Error(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return nil, backlog.ErrNotFound
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_me", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== B2: logvalet_user_activity =====

// TestUserActivity_Normal は user_id と limit が正しく渡されることを確認する。
func TestUserActivity_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedUserID string
	var capturedOpt backlog.ListUserActivitiesOptions
	now := time.Now()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		capturedUserID = userID
		capturedOpt = opt
		return []domain.Activity{{ID: 1, Type: 1, Created: &now}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{
		"user_id": "12345",
		"limit":   float64(10),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedUserID != "12345" {
		t.Errorf("userID = %q, want %q", capturedUserID, "12345")
	}
	if capturedOpt.Count != 10 {
		t.Errorf("Count = %d, want 10", capturedOpt.Count)
	}
}

// TestUserActivity_Me は user_id="me" のとき GetMyself → ListUserActivities が呼ばれることを確認する。
func TestUserActivity_Me(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 99, UserID: "me_user", Name: "自分"}, nil
	}
	var capturedUserID string
	now := time.Now()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		capturedUserID = userID
		return []domain.Activity{{ID: 2, Type: 2, Created: &now}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{"user_id": "me"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetMyself") != 1 {
		t.Errorf("expected GetMyself called 1 time, got %d", mock.GetCallCount("GetMyself"))
	}
	if capturedUserID != "99" {
		t.Errorf("userID = %q, want %q", capturedUserID, "99")
	}
}

// TestUserActivity_MissingUserID は user_id 未指定で IsError=true になることを確認する。
func TestUserActivity_MissingUserID(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}
