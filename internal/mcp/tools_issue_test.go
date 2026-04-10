package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// --- T1-1: assignee_id "me" -> GetMyself 呼出、opts.AssigneeIDs にユーザーID設定 ---
func TestIssueList_AssigneeMe(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 99, UserID: "taro", Name: "Taro"}, nil
	}
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"assignee_id": "me"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.AssigneeIDs) != 1 || capturedOpt.AssigneeIDs[0] != 99 {
		t.Errorf("expected AssigneeIDs=[99], got %v", capturedOpt.AssigneeIDs)
	}
}

// --- T1-2: assignee_id "12345" -> opts.AssigneeIDs = [12345] ---
func TestIssueList_AssigneeNumericID(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"assignee_id": "12345"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.AssigneeIDs) != 1 || capturedOpt.AssigneeIDs[0] != 12345 {
		t.Errorf("expected AssigneeIDs=[12345], got %v", capturedOpt.AssigneeIDs)
	}
}

// --- T1-3: status_id "not-closed" -> opts.StatusIDs = [1,2,3] ---
func TestIssueList_StatusNotClosed(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"status_id": "not-closed"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	expected := []int{1, 2, 3}
	if len(capturedOpt.StatusIDs) != len(expected) {
		t.Fatalf("expected StatusIDs=%v, got %v", expected, capturedOpt.StatusIDs)
	}
	for i, v := range expected {
		if capturedOpt.StatusIDs[i] != v {
			t.Errorf("StatusIDs[%d]: expected %d, got %d", i, v, capturedOpt.StatusIDs[i])
		}
	}
}

// --- T1-4: status_id "1,2" -> opts.StatusIDs = [1,2] ---
func TestIssueList_StatusCommaSeparated(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"status_id": "1,2"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.StatusIDs) != 2 || capturedOpt.StatusIDs[0] != 1 || capturedOpt.StatusIDs[1] != 2 {
		t.Errorf("expected StatusIDs=[1,2], got %v", capturedOpt.StatusIDs)
	}
}

// --- T1-5: due_date "overdue" -> opts.DueDateUntil = 昨日末 ---
func TestIssueList_DueDateOverdue(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"due_date": "overdue"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.DueDateSince != nil {
		t.Errorf("expected DueDateSince=nil for overdue, got %v", capturedOpt.DueDateSince)
	}
	if capturedOpt.DueDateUntil == nil {
		t.Fatal("expected DueDateUntil to be set for overdue")
	}
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	if capturedOpt.DueDateUntil.Equal(yesterday) == false {
		t.Errorf("expected DueDateUntil=%v (yesterday), got %v", yesterday, capturedOpt.DueDateUntil)
	}
}

// --- T1-6: due_date "this-week" -> since=月曜, until=日曜 ---
func TestIssueList_DueDateThisWeek(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"due_date": "this-week"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.DueDateSince == nil {
		t.Fatal("expected DueDateSince to be set for this-week")
	}
	if capturedOpt.DueDateUntil == nil {
		t.Fatal("expected DueDateUntil to be set for this-week")
	}
	if capturedOpt.DueDateSince.Weekday() != time.Monday {
		t.Errorf("expected DueDateSince to be Monday, got %v", capturedOpt.DueDateSince.Weekday())
	}
	if capturedOpt.DueDateUntil.Weekday() != time.Sunday {
		t.Errorf("expected DueDateUntil to be Sunday, got %v", capturedOpt.DueDateUntil.Weekday())
	}
}

// --- T1-7: due_date "2026-04-01:2026-04-10" -> 両日付設定 ---
func TestIssueList_DueDateRange(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"due_date": "2026-04-01:2026-04-10"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.DueDateSince == nil {
		t.Fatal("expected DueDateSince to be set")
	}
	if capturedOpt.DueDateUntil == nil {
		t.Fatal("expected DueDateUntil to be set")
	}
	expectedSince := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	expectedUntil := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	if capturedOpt.DueDateSince.Equal(expectedSince) == false {
		t.Errorf("expected DueDateSince=%v, got %v", expectedSince, capturedOpt.DueDateSince)
	}
	if capturedOpt.DueDateUntil.Equal(expectedUntil) == false {
		t.Errorf("expected DueDateUntil=%v, got %v", expectedUntil, capturedOpt.DueDateUntil)
	}
}

// --- T1-8: project_keys "PROJ1,PROJ2" -> opts.ProjectIDs 2件 ---
func TestIssueList_ProjectKeysMultiple(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		switch projectKey {
		case "PROJ1":
			return &domain.Project{ID: 101, ProjectKey: "PROJ1"}, nil
		case "PROJ2":
			return &domain.Project{ID: 102, ProjectKey: "PROJ2"}, nil
		}
		return nil, fmt.Errorf("project not found: %s", projectKey)
	}
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"project_keys": "PROJ1,PROJ2"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.ProjectIDs) != 2 {
		t.Fatalf("expected 2 ProjectIDs, got %v", capturedOpt.ProjectIDs)
	}
	if capturedOpt.ProjectIDs[0] != 101 || capturedOpt.ProjectIDs[1] != 102 {
		t.Errorf("expected ProjectIDs=[101,102], got %v", capturedOpt.ProjectIDs)
	}
}

// --- T1-9: project_key "PROJ"（旧パラメータ）-> 動作変更なし ---
func TestIssueList_ProjectKeyLegacy(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 200, ProjectKey: projectKey}, nil
	}
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.ProjectIDs) != 1 || capturedOpt.ProjectIDs[0] != 200 {
		t.Errorf("expected ProjectIDs=[200], got %v", capturedOpt.ProjectIDs)
	}
}

// --- T1-10: assignee_id "me" + GetMyself 失敗 -> エラー返却 ---
func TestIssueList_AssigneeMe_GetMyselfFails(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return nil, fmt.Errorf("authentication failed")
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"assignee_id": "me"})

	if result.IsError == false {
		t.Error("expected IsError=true when GetMyself fails")
	}
}

// --- T1-11: assignee_id "abc"（非数値・非"me"）-> エラー返却 ---
func TestIssueList_AssigneeInvalidString(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"assignee_id": "abc"})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid assignee_id 'abc'")
	}
}

// --- T1-12: due_date "invalid-format" -> エラー返却 ---
func TestIssueList_DueDateInvalidFormat(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"due_date": "invalid-format"})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid due_date format")
	}
}

// --- T1-13: status_id "abc"（非数値・非"not-closed"）-> エラー返却 ---
func TestIssueList_StatusInvalidString(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"status_id": "abc"})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid status_id 'abc'")
	}
}

// --- T1-bonus: project_key と project_keys 両方指定 -> 両方のプロジェクトIDが設定される ---
func TestIssueList_BothProjectKeyAndProjectKeys(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		switch projectKey {
		case "PROJ1":
			return &domain.Project{ID: 101, ProjectKey: "PROJ1"}, nil
		case "SINGLE":
			return &domain.Project{ID: 300, ProjectKey: "SINGLE"}, nil
		}
		return nil, fmt.Errorf("project not found: %s", projectKey)
	}
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{
		"project_key":  "SINGLE",
		"project_keys": "PROJ1",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.ProjectIDs) != 2 {
		t.Errorf("expected 2 ProjectIDs (from both params), got %v", capturedOpt.ProjectIDs)
	}
}

// --- T1-result: issue_list が正しく JSON を返すこと ---
func TestIssueList_ReturnsJSON(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{
			{ID: 1, IssueKey: "TEST-1", Summary: "Test issue"},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if ok == false {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var issues []domain.Issue
	if err := json.Unmarshal([]byte(textContent.Text), &issues); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(issues) != 1 || issues[0].IssueKey != "TEST-1" {
		t.Errorf("unexpected issues: %v", issues)
	}
}
