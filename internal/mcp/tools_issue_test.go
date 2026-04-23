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

// ===== A1: issue_create 追加パラメータテスト (Red Phase) =====

// TestIssueCreate_WithParentIssueID はリクエスト内で ParentIssueID が設定されることを確認する。
func TestIssueCreate_WithParentIssueID(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":     "TEST",
		"summary":         "Test issue",
		"issue_type_id":   1,
		"parent_issue_id": 100,
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.ParentIssueID != 100 {
		t.Errorf("expected ParentIssueID=100, got %d", capturedReq.ParentIssueID)
	}
}

// TestIssueCreate_WithCategoryIDs_CSV はカテゴリ IDs が CSV 文字列からパースされることを確認する。
func TestIssueCreate_WithCategoryIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"category_ids":  "10,20,30",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.CategoryIDs) != 3 || capturedReq.CategoryIDs[0] != 10 || capturedReq.CategoryIDs[1] != 20 || capturedReq.CategoryIDs[2] != 30 {
		t.Errorf("expected CategoryIDs=[10,20,30], got %v", capturedReq.CategoryIDs)
	}
}

// TestIssueCreate_WithVersionIDs_CSV はバージョン IDs が CSV 文字列からパースされることを確認する。
func TestIssueCreate_WithVersionIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"version_ids":   "5,6",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.VersionIDs) != 2 || capturedReq.VersionIDs[0] != 5 || capturedReq.VersionIDs[1] != 6 {
		t.Errorf("expected VersionIDs=[5,6], got %v", capturedReq.VersionIDs)
	}
}

// TestIssueCreate_WithMilestoneIDs_CSV はマイルストーン IDs が CSV 文字列からパースされることを確認する。
func TestIssueCreate_WithMilestoneIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"milestone_ids": "7,8,9",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.MilestoneIDs) != 3 || capturedReq.MilestoneIDs[0] != 7 || capturedReq.MilestoneIDs[1] != 8 || capturedReq.MilestoneIDs[2] != 9 {
		t.Errorf("expected MilestoneIDs=[7,8,9], got %v", capturedReq.MilestoneIDs)
	}
}

// TestIssueCreate_WithNotifiedUserIDs_CSV は通知対象ユーザー IDs が CSV 文字列からパースされることを確認する。
func TestIssueCreate_WithNotifiedUserIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":        "TEST",
		"summary":            "Test issue",
		"issue_type_id":      1,
		"notified_user_ids":  "100,101,102",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.NotifiedUserIDs) != 3 || capturedReq.NotifiedUserIDs[0] != 100 || capturedReq.NotifiedUserIDs[1] != 101 || capturedReq.NotifiedUserIDs[2] != 102 {
		t.Errorf("expected NotifiedUserIDs=[100,101,102], got %v", capturedReq.NotifiedUserIDs)
	}
}

// TestIssueCreate_WithDueDate は due_date パラメータが DueDate(*time.Time) に設定されることを確認する。
func TestIssueCreate_WithDueDate(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"due_date":      "2026-05-01",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.DueDate == nil {
		t.Fatal("expected DueDate to be set")
	}
	expected := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !capturedReq.DueDate.Equal(expected) {
		t.Errorf("expected DueDate=%v, got %v", expected, capturedReq.DueDate)
	}
}

// TestIssueCreate_WithStartDate は start_date パラメータが StartDate(*time.Time) に設定されることを確認する。
func TestIssueCreate_WithStartDate(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"start_date":    "2026-04-15",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.StartDate == nil {
		t.Fatal("expected StartDate to be set")
	}
	expected := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	if !capturedReq.StartDate.Equal(expected) {
		t.Errorf("expected StartDate=%v, got %v", expected, capturedReq.StartDate)
	}
}

// TestIssueCreate_InvalidDueDate は不正な due_date でエラーを返す。
func TestIssueCreate_InvalidDueDate(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"due_date":      "invalid",
	})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid due_date")
	}
}

// TestIssueCreate_InvalidCategoryIDs は不正なカテゴリ IDs でエラーを返す。
func TestIssueCreate_InvalidCategoryIDs(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"category_ids":  "10,abc,30",
	})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid category_ids")
	}
}

// TestIssueCreate_EmptyCategoryIDs は空の category_ids でも正常に処理される。
func TestIssueCreate_EmptyCategoryIDs(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"category_ids":  "",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.CategoryIDs != nil {
		t.Errorf("expected CategoryIDs=nil for empty string, got %v", capturedReq.CategoryIDs)
	}
}

// TestIssueCreate_SingleCategoryID は単一のカテゴリ ID が正しくパースされる。
func TestIssueCreate_SingleCategoryID(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var capturedReq backlog.CreateIssueRequest
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_create", map[string]any{
		"project_key":   "TEST",
		"summary":       "Test issue",
		"issue_type_id": 1,
		"category_ids":  "10",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.CategoryIDs) != 1 || capturedReq.CategoryIDs[0] != 10 {
		t.Errorf("expected CategoryIDs=[10], got %v", capturedReq.CategoryIDs)
	}
}

// ===== A2: issue_update 追加パラメータテスト =====

// TestIssueUpdate_WithIssueTypeID は issue_type_id が IssueTypeID に設定されることを確認する。
func TestIssueUpdate_WithIssueTypeID(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key":     "PROJ-1",
		"issue_type_id": 5,
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.IssueTypeID == nil || *capturedReq.IssueTypeID != 5 {
		t.Errorf("expected IssueTypeID=&5, got %v", capturedReq.IssueTypeID)
	}
}

// TestIssueUpdate_WithCategoryIDs_CSV は category_ids が CategoryIDs に設定されることを確認する。
func TestIssueUpdate_WithCategoryIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key":    "PROJ-1",
		"category_ids": "10,20",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.CategoryIDs) != 2 || capturedReq.CategoryIDs[0] != 10 || capturedReq.CategoryIDs[1] != 20 {
		t.Errorf("expected CategoryIDs=[10,20], got %v", capturedReq.CategoryIDs)
	}
}

// TestIssueUpdate_WithVersionIDs_CSV は version_ids が VersionIDs に設定されることを確認する。
func TestIssueUpdate_WithVersionIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key":   "PROJ-1",
		"version_ids": "5,6",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.VersionIDs) != 2 || capturedReq.VersionIDs[0] != 5 || capturedReq.VersionIDs[1] != 6 {
		t.Errorf("expected VersionIDs=[5,6], got %v", capturedReq.VersionIDs)
	}
}

// TestIssueUpdate_WithMilestoneIDs_CSV は milestone_ids が MilestoneIDs に設定されることを確認する。
func TestIssueUpdate_WithMilestoneIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key":     "PROJ-1",
		"milestone_ids": "7,8,9",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.MilestoneIDs) != 3 || capturedReq.MilestoneIDs[0] != 7 || capturedReq.MilestoneIDs[1] != 8 || capturedReq.MilestoneIDs[2] != 9 {
		t.Errorf("expected MilestoneIDs=[7,8,9], got %v", capturedReq.MilestoneIDs)
	}
}

// TestIssueUpdate_WithNotifiedUserIDs_CSV は notified_user_ids が NotifiedUserIDs に設定されることを確認する。
func TestIssueUpdate_WithNotifiedUserIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key":         "PROJ-1",
		"notified_user_ids": "11,22",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.NotifiedUserIDs) != 2 || capturedReq.NotifiedUserIDs[0] != 11 || capturedReq.NotifiedUserIDs[1] != 22 {
		t.Errorf("expected NotifiedUserIDs=[11,22], got %v", capturedReq.NotifiedUserIDs)
	}
}

// TestIssueUpdate_WithDueDate は due_date が DueDate に設定されることを確認する。
func TestIssueUpdate_WithDueDate(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key": "PROJ-1",
		"due_date":  "2026-05-01",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	expected := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if capturedReq.DueDate == nil || !capturedReq.DueDate.Equal(expected) {
		t.Errorf("expected DueDate=%v, got %v", expected, capturedReq.DueDate)
	}
}

// TestIssueUpdate_WithStartDate は start_date が StartDate に設定されることを確認する。
func TestIssueUpdate_WithStartDate(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key":  "PROJ-1",
		"start_date": "2026-04-15",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	expected := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	if capturedReq.StartDate == nil || !capturedReq.StartDate.Equal(expected) {
		t.Errorf("expected StartDate=%v, got %v", expected, capturedReq.StartDate)
	}
}

// TestIssueUpdate_InvalidDueDate は不正な due_date でエラーを返す。
func TestIssueUpdate_InvalidDueDate(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_update", map[string]any{
		"issue_key": "PROJ-1",
		"due_date":  "invalid",
	})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid due_date")
	}
}

// ===== A3: issue_list 追加パラメータテスト =====

// TestIssueList_StartDateThisWeek は start_date "this-week" が StartDateSince/Until に設定されることを確認する。
func TestIssueList_StartDateThisWeek(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"start_date": "this-week"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.StartDateSince == nil {
		t.Fatal("expected StartDateSince to be set for this-week")
	}
	if capturedOpt.StartDateUntil == nil {
		t.Fatal("expected StartDateUntil to be set for this-week")
	}
	if capturedOpt.StartDateSince.Weekday() != time.Monday {
		t.Errorf("expected StartDateSince to be Monday, got %v", capturedOpt.StartDateSince.Weekday())
	}
	if capturedOpt.StartDateUntil.Weekday() != time.Sunday {
		t.Errorf("expected StartDateUntil to be Sunday, got %v", capturedOpt.StartDateUntil.Weekday())
	}
}

// TestIssueList_StartDateToday は start_date "today" が Since=Until=今日に設定されることを確認する。
func TestIssueList_StartDateToday(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"start_date": "today"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.StartDateSince == nil {
		t.Fatal("expected StartDateSince to be set for today")
	}
	if capturedOpt.StartDateUntil == nil {
		t.Fatal("expected StartDateUntil to be set for today")
	}
	if !capturedOpt.StartDateSince.Equal(*capturedOpt.StartDateUntil) {
		t.Errorf("expected StartDateSince == StartDateUntil for today")
	}
}

// TestIssueList_StartDateSingleDate は start_date "2026-04-01" が Since=Until に設定されることを確認する。
func TestIssueList_StartDateSingleDate(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"start_date": "2026-04-01"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	expected := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if capturedOpt.StartDateSince == nil || !capturedOpt.StartDateSince.Equal(expected) {
		t.Errorf("expected StartDateSince=%v, got %v", expected, capturedOpt.StartDateSince)
	}
	if capturedOpt.StartDateUntil == nil || !capturedOpt.StartDateUntil.Equal(expected) {
		t.Errorf("expected StartDateUntil=%v, got %v", expected, capturedOpt.StartDateUntil)
	}
}

// TestIssueList_UpdatedSince は updated_since が UpdatedSince に設定されることを確認する。
func TestIssueList_UpdatedSince(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"updated_since": "2026-04-01"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	expected := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if capturedOpt.UpdatedSince == nil || !capturedOpt.UpdatedSince.Equal(expected) {
		t.Errorf("expected UpdatedSince=%v, got %v", expected, capturedOpt.UpdatedSince)
	}
	if capturedOpt.UpdatedUntil != nil {
		t.Errorf("expected UpdatedUntil=nil, got %v", capturedOpt.UpdatedUntil)
	}
}

// TestIssueList_UpdatedUntil は updated_until が UpdatedUntil に設定されることを確認する。
func TestIssueList_UpdatedUntil(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedOpt = opt
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"updated_until": "2026-04-30"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	expected := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	if capturedOpt.UpdatedUntil == nil || !capturedOpt.UpdatedUntil.Equal(expected) {
		t.Errorf("expected UpdatedUntil=%v, got %v", expected, capturedOpt.UpdatedUntil)
	}
	if capturedOpt.UpdatedSince != nil {
		t.Errorf("expected UpdatedSince=nil, got %v", capturedOpt.UpdatedSince)
	}
}

// TestIssueList_UpdatedSinceInvalid は不正な updated_since でエラーを返す。
func TestIssueList_UpdatedSinceInvalid(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_list", map[string]any{"updated_since": "invalid"})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid updated_since")
	}
}

// ===== A4: issue_comment_add 追加パラメータテスト =====

// TestIssueCommentAdd_WithNotifiedUserIDs_CSV は notified_user_ids が NotifiedUserIDs に設定されることを確認する。
func TestIssueCommentAdd_WithNotifiedUserIDs_CSV(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.AddCommentRequest
	mock.AddIssueCommentFunc = func(ctx context.Context, issueKey string, req backlog.AddCommentRequest) (*domain.Comment, error) {
		capturedReq = req
		return &domain.Comment{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_comment_add", map[string]any{
		"issue_key":         "PROJ-1",
		"content":           "テストコメント",
		"notified_user_ids": "11,22",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedReq.NotifiedUserIDs) != 2 || capturedReq.NotifiedUserIDs[0] != 11 || capturedReq.NotifiedUserIDs[1] != 22 {
		t.Errorf("expected NotifiedUserIDs=[11,22], got %v", capturedReq.NotifiedUserIDs)
	}
}

// TestIssueCommentAdd_EmptyNotifiedUserIDs は空文字列で NotifiedUserIDs が nil になることを確認する。
func TestIssueCommentAdd_EmptyNotifiedUserIDs(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.AddCommentRequest
	mock.AddIssueCommentFunc = func(ctx context.Context, issueKey string, req backlog.AddCommentRequest) (*domain.Comment, error) {
		capturedReq = req
		return &domain.Comment{ID: 1}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_comment_add", map[string]any{
		"issue_key":         "PROJ-1",
		"content":           "テストコメント",
		"notified_user_ids": "",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.NotifiedUserIDs != nil {
		t.Errorf("expected NotifiedUserIDs=nil for empty string, got %v", capturedReq.NotifiedUserIDs)
	}
}

// TestIssueCommentAdd_InvalidNotifiedUserIDs は不正な notified_user_ids でエラーを返す。
func TestIssueCommentAdd_InvalidNotifiedUserIDs(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_comment_add", map[string]any{
		"issue_key":         "PROJ-1",
		"content":           "テストコメント",
		"notified_user_ids": "11,abc",
	})

	if result.IsError == false {
		t.Error("expected IsError=true for invalid notified_user_ids")
	}
}
