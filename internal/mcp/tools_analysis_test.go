package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/analysis"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// helperMCPIssue は MCP テスト用の domain.Issue を返すヘルパー。
func helperMCPIssue() *domain.Issue {
	updated := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	created := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	return &domain.Issue{
		ID:          1001,
		ProjectID:   100,
		IssueKey:    "PROJ-123",
		Summary:     "テストの課題",
		Description: "テストの説明文",
		Status:      &domain.IDName{ID: 1, Name: "未対応"},
		Priority:    &domain.IDName{ID: 2, Name: "高"},
		IssueType:   &domain.IDName{ID: 3, Name: "バグ"},
		Assignee:    &domain.User{ID: 10, UserID: "user1", Name: "テストユーザー"},
		Reporter:    &domain.User{ID: 20, UserID: "user2", Name: "報告者"},
		Categories:  []domain.IDName{{ID: 1, Name: "カテゴリA"}},
		Milestones:  []domain.IDName{{ID: 1, Name: "v1.0"}},
		DueDate:     &dueDate,
		Created:     &created,
		Updated:     &updated,
	}
}

// helperMCPComments は MCP テスト用のコメント一覧を返すヘルパー。
func helperMCPComments(count int) []domain.Comment {
	comments := make([]domain.Comment, count)
	for i := 0; i < count; i++ {
		t := time.Date(2026, 3, 20+i, 10, 0, 0, 0, time.UTC)
		comments[i] = domain.Comment{
			ID:          int64(i + 1),
			Content:     "コメント内容",
			CreatedUser: &domain.User{ID: 10, UserID: "user1", Name: "テストユーザー"},
			Created:     &t,
		}
	}
	return comments
}

// helperMCPStatuses は MCP テスト用のステータス一覧を返すヘルパー。
func helperMCPStatuses() []domain.Status {
	return []domain.Status{
		{ID: 1, Name: "未対応", DisplayOrder: 0},
		{ID: 2, Name: "処理中", DisplayOrder: 1},
		{ID: 3, Name: "処理済み", DisplayOrder: 2},
		{ID: 4, Name: "完了", DisplayOrder: 3},
	}
}

// newTestServer は ServerConfig 付きの MCP サーバーを返すテストヘルパー。
func newTestServer(mock *backlog.MockClient) *mcpserver.MCPServer {
	cfg := mcpinternal.ServerConfig{
		Profile: "default",
		Space:   "heptagon",
		BaseURL: "https://heptagon.backlog.com",
	}
	return mcpinternal.NewServer(mock, "test", cfg)
}

// T1: RegisterAnalysisTools で logvalet_issue_context が登録されること
func TestRegisterAnalysisTools_ToolRegistered(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(mock)

	tool := s.GetTool("logvalet_issue_context")
	if tool == nil {
		t.Fatal("logvalet_issue_context tool not registered")
	}
}

// T2: logvalet_issue_context ハンドラーが正常に AnalysisEnvelope を返すこと
func TestIssueContextHandler_Success(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return helperMCPIssue(), nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return helperMCPComments(5), nil
	}
	mock.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperMCPStatuses(), nil
	}

	s := newTestServer(mock)
	result := callTool(t, s, "logvalet_issue_context", map[string]any{"issue_key": "PROJ-123"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	var envelope analysis.AnalysisEnvelope
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if envelope.Resource != "issue_context" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "issue_context")
	}
	if envelope.Profile != "default" {
		t.Errorf("Profile = %q, want %q", envelope.Profile, "default")
	}
	if envelope.Space != "heptagon" {
		t.Errorf("Space = %q, want %q", envelope.Space, "heptagon")
	}
}

// T3: issue_key が空の場合にエラーを返すこと
func TestIssueContextHandler_MissingIssueKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(mock)

	result := callTool(t, s, "logvalet_issue_context", map[string]any{})

	if !result.IsError {
		t.Error("expected IsError=true for missing issue_key")
	}
}

// T4: comments パラメータが MaxComments に反映されること
func TestIssueContextHandler_WithComments(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return helperMCPIssue(), nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return helperMCPComments(10), nil
	}
	mock.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperMCPStatuses(), nil
	}

	s := newTestServer(mock)
	result := callTool(t, s, "logvalet_issue_context", map[string]any{
		"issue_key": "PROJ-123",
		"comments":  float64(3),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	textContent := result.Content[0].(gomcp.TextContent)
	var envelope analysis.AnalysisEnvelope
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Analysis を map として解析して recent_comments の件数を確認
	analysisBytes, _ := json.Marshal(envelope.Analysis)
	var ic map[string]any
	if err := json.Unmarshal(analysisBytes, &ic); err != nil {
		t.Fatalf("failed to unmarshal analysis: %v", err)
	}
	recentComments, ok := ic["recent_comments"].([]any)
	if !ok {
		t.Fatalf("recent_comments not found or wrong type")
	}
	if len(recentComments) != 3 {
		t.Errorf("recent_comments length = %d, want 3", len(recentComments))
	}
}

// T5: compact パラメータが反映されること
func TestIssueContextHandler_WithCompact(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return helperMCPIssue(), nil
	}
	mock.ListIssueCommentsFunc = func(ctx context.Context, issueKey string, opt backlog.ListCommentsOptions) ([]domain.Comment, error) {
		return helperMCPComments(3), nil
	}
	mock.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return helperMCPStatuses(), nil
	}

	s := newTestServer(mock)
	result := callTool(t, s, "logvalet_issue_context", map[string]any{
		"issue_key": "PROJ-123",
		"compact":   true,
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	textContent := result.Content[0].(gomcp.TextContent)
	var envelope analysis.AnalysisEnvelope
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	analysisBytes, _ := json.Marshal(envelope.Analysis)
	var ic map[string]any
	if err := json.Unmarshal(analysisBytes, &ic); err != nil {
		t.Fatalf("failed to unmarshal analysis: %v", err)
	}

	issueMap := ic["issue"].(map[string]any)
	if desc, ok := issueMap["description"]; ok && desc != "" {
		t.Errorf("compact mode: description should be empty, got %q", desc)
	}
}

// T6: GetIssue がエラーを返した場合に IsError になること
func TestIssueContextHandler_GetIssueError(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return nil, errors.New("API error")
	}

	s := newTestServer(mock)
	result := callTool(t, s, "logvalet_issue_context", map[string]any{"issue_key": "PROJ-999"})

	if !result.IsError {
		t.Error("expected IsError=true for GetIssue error")
	}
}
