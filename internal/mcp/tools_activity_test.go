package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// T3-1: WithUserId — user_id: "12345" → ListUserActivities 呼出確認
func TestActivityListWithUserId(t *testing.T) {
	mock := backlog.NewMockClient()
	now := time.Now()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		if userID != "12345" {
			t.Errorf("expected userID 12345, got %s", userID)
		}
		if opt.Count != 20 {
			t.Errorf("expected count 20, got %d", opt.Count)
		}
		return []domain.Activity{
			{
				ID:      1,
				Type:    1,
				Created: &now,
			},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_activity_list", map[string]any{"user_id": "12345"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	var activities []domain.Activity
	if err := json.Unmarshal([]byte(textContent.Text), &activities); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(activities) != 1 {
		t.Errorf("expected 1 activity, got %d", len(activities))
	}

	if mock.GetCallCount("ListUserActivities") != 1 {
		t.Errorf("expected ListUserActivities to be called 1 time, got %d", mock.GetCallCount("ListUserActivities"))
	}
}

// T3-2: WithUserIdMe — user_id: "me" → GetMyself → ListUserActivities 呼出
func TestActivityListWithUserIdMe(t *testing.T) {
	mock := backlog.NewMockClient()
	now := time.Now()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{
			ID:   99,
			Name: "Test User",
		}, nil
	}
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		if userID != "99" {
			t.Errorf("expected userID 99 (converted from 'me'), got %s", userID)
		}
		return []domain.Activity{
			{
				ID:      2,
				Type:    2,
				Created: &now,
			},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_activity_list", map[string]any{"user_id": "me"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	if mock.GetCallCount("GetMyself") != 1 {
		t.Errorf("expected GetMyself to be called 1 time, got %d", mock.GetCallCount("GetMyself"))
	}
	if mock.GetCallCount("ListUserActivities") != 1 {
		t.Errorf("expected ListUserActivities to be called 1 time, got %d", mock.GetCallCount("ListUserActivities"))
	}
}

// T3-3: WithProjectKey — project_key: "PROJ" → ListProjectActivities 呼出
func TestActivityListWithProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()
	now := time.Now()
	mock.ListProjectActivitiesFunc = func(ctx context.Context, projectKey string, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		if projectKey != "PROJ" {
			t.Errorf("expected projectKey PROJ, got %s", projectKey)
		}
		if opt.Count != 20 {
			t.Errorf("expected count 20, got %d", opt.Count)
		}
		return []domain.Activity{
			{
				ID:      3,
				Type:    1,
				Created: &now,
			},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_activity_list", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	var activities []domain.Activity
	if err := json.Unmarshal([]byte(textContent.Text), &activities); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(activities) != 1 {
		t.Errorf("expected 1 activity, got %d", len(activities))
	}

	if mock.GetCallCount("ListProjectActivities") != 1 {
		t.Errorf("expected ListProjectActivities to be called 1 time, got %d", mock.GetCallCount("ListProjectActivities"))
	}
}

// T3-4: Default — パラメータなし → ListSpaceActivities（後方互換）
func TestActivityListDefault(t *testing.T) {
	mock := backlog.NewMockClient()
	now := time.Now()
	mock.ListSpaceActivitiesFunc = func(ctx context.Context, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		if opt.Count != 20 {
			t.Errorf("expected count 20, got %d", opt.Count)
		}
		return []domain.Activity{
			{
				ID:      4,
				Type:    1,
				Created: &now,
			},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_activity_list", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	if mock.GetCallCount("ListSpaceActivities") != 1 {
		t.Errorf("expected ListSpaceActivities to be called 1 time, got %d", mock.GetCallCount("ListSpaceActivities"))
	}
}

// T3-5: BothParams_Error — 両方指定 → エラー
func TestActivityListBothParamsError(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_activity_list", map[string]any{
		"user_id":     "12345",
		"project_key": "PROJ",
	})

	if !result.IsError {
		t.Fatal("expected tool error, got success")
	}

	// Verify no API calls were made
	if mock.GetCallCount("ListUserActivities") != 0 {
		t.Errorf("expected ListUserActivities not to be called, got %d calls", mock.GetCallCount("ListUserActivities"))
	}
	if mock.GetCallCount("ListProjectActivities") != 0 {
		t.Errorf("expected ListProjectActivities not to be called, got %d calls", mock.GetCallCount("ListProjectActivities"))
	}
	if mock.GetCallCount("ListSpaceActivities") != 0 {
		t.Errorf("expected ListSpaceActivities not to be called, got %d calls", mock.GetCallCount("ListSpaceActivities"))
	}
}

// T3-6: UserIdMe_GetMyselfError — user_id: "me" + GetMyself 失敗 → エラー
func TestActivityListUserIdMeGetMyselfError(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return nil, backlog.ErrNotFound
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_activity_list", map[string]any{"user_id": "me"})

	if !result.IsError {
		t.Fatal("expected tool error, got success")
	}

	if mock.GetCallCount("GetMyself") != 1 {
		t.Errorf("expected GetMyself to be called 1 time, got %d", mock.GetCallCount("GetMyself"))
	}
	if mock.GetCallCount("ListUserActivities") != 0 {
		t.Errorf("expected ListUserActivities not to be called, got %d calls", mock.GetCallCount("ListUserActivities"))
	}
}
