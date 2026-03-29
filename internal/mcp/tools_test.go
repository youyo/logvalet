package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// callTool は ServerTool のハンドラーを直接呼び出すテストヘルパー。
// MCPServer.GetTool で tool を取得し、Handler を実行する。
func callTool(t *testing.T, s *mcpserver.MCPServer, toolName string, args map[string]any) *gomcp.CallToolResult {
	t.Helper()
	serverTool := s.GetTool(toolName)
	if serverTool == nil {
		t.Fatalf("tool %q not found", toolName)
	}

	req := gomcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args

	result, err := serverTool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("tool %q handler returned error: %v", toolName, err)
	}
	return result
}

// MCP-1: NewServer で 24 ツールが登録されること
func TestNewServer_RegistersAllTools(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test")

	tools := s.ListTools()
	expectedCount := 24
	if len(tools) != expectedCount {
		t.Errorf("expected %d tools, got %d", expectedCount, len(tools))
		for name := range tools {
			t.Logf("  tool: %s", name)
		}
	}
}

// MCP-2: logvalet_issue_get ハンドラーが mock client から JSON を返すこと
func TestIssueGetHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		if issueKey != "TEST-1" {
			t.Errorf("unexpected issueKey: %s", issueKey)
		}
		return &domain.Issue{
			ID:       1,
			IssueKey: "TEST-1",
			Summary:  "Test issue",
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test")
	result := callTool(t, s, "logvalet_issue_get", map[string]any{"issue_key": "TEST-1"})

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
	var issue domain.Issue
	if err := json.Unmarshal([]byte(textContent.Text), &issue); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if issue.IssueKey != "TEST-1" {
		t.Errorf("expected issue_key TEST-1, got %s", issue.IssueKey)
	}
}

// MCP-5: logvalet_project_get ハンドラーテスト
func TestProjectGetHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{
			ID:         42,
			ProjectKey: projectKey,
			Name:       "Test Project",
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test")
	result := callTool(t, s, "logvalet_project_get", map[string]any{"project_key": "TESTPROJECT"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var proj domain.Project
	if err := json.Unmarshal([]byte(textContent.Text), &proj); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if proj.ProjectKey != "TESTPROJECT" {
		t.Errorf("expected project_key TESTPROJECT, got %s", proj.ProjectKey)
	}
}

// MCP-12: logvalet_star_add ハンドラーテスト
func TestStarAddHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.AddStarRequest
	mock.AddStarFunc = func(ctx context.Context, req backlog.AddStarRequest) error {
		capturedReq = req
		return nil
	}

	s := mcpinternal.NewServer(mock, "test")
	result := callTool(t, s, "logvalet_star_add", map[string]any{"issue_id": float64(100)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.IssueID == nil || *capturedReq.IssueID != 100 {
		t.Errorf("expected issue_id=100, got %v", capturedReq.IssueID)
	}
}

// MCP-E2: 必須パラメータ欠落 → IsError: true
func TestIssueGetHandler_MissingIssueKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test")

	result := callTool(t, s, "logvalet_issue_get", map[string]any{})

	if !result.IsError {
		t.Error("expected IsError=true for missing issue_key")
	}
}

// MCP-E3: ErrNotFound → IsError: true
func TestIssueGetHandler_NotFound(t *testing.T) {
	mock := backlog.NewMockClient()
	// GetIssueFunc が未設定の場合、MockClient は ErrNotFound を返す

	s := mcpinternal.NewServer(mock, "test")
	result := callTool(t, s, "logvalet_issue_get", map[string]any{"issue_key": "NOTFOUND-999"})

	if !result.IsError {
		t.Error("expected IsError=true for ErrNotFound")
	}
}
