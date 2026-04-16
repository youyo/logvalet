package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
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

// MCP-1: NewServer で 29 ツールが登録されること
func TestNewServer_RegistersAllTools(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})

	tools := s.ListTools()
	expectedCount := 42
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

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
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

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
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

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
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
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})

	result := callTool(t, s, "logvalet_issue_get", map[string]any{})

	if !result.IsError {
		t.Error("expected IsError=true for missing issue_key")
	}
}

// MCP-E3: ErrNotFound → IsError: true
func TestIssueGetHandler_NotFound(t *testing.T) {
	mock := backlog.NewMockClient()
	// GetIssueFunc が未設定の場合、MockClient は ErrNotFound を返す

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_get", map[string]any{"issue_key": "NOTFOUND-999"})

	if !result.IsError {
		t.Error("expected IsError=true for ErrNotFound")
	}
}

// MCP-26: logvalet_project_blockers ハンドラーが mock client から JSON を返すこと
func TestProjectBlockersHandler_Basic(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{
			ID:         100,
			ProjectKey: projectKey,
			Name:       "Test Project",
		}, nil
	}
	mock.ListIssuesFunc = func(ctx context.Context, opts backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_project_blockers", map[string]any{"project_keys": "PROJ"})

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
	var envelope map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	analysis, ok := envelope["analysis"].(map[string]any)
	if !ok {
		t.Fatalf("expected analysis field in envelope, got %T", envelope["analysis"])
	}
	if _, ok := analysis["total_count"]; !ok {
		t.Error("expected total_count field in analysis")
	}
}

// MCP-27: project_keys 省略 → IsError: true
func TestProjectBlockersHandler_MissingProjectKeys(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})

	result := callTool(t, s, "logvalet_project_blockers", map[string]any{})

	if !result.IsError {
		t.Error("expected IsError=true for missing project_keys")
	}
}

// MCP-30: logvalet_project_health ハンドラーが mock client から JSON を返すこと
func TestProjectHealthHandler_Basic(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{
			ID:         100,
			ProjectKey: projectKey,
			Name:       "Test Project",
		}, nil
	}
	mock.ListIssuesFunc = func(ctx context.Context, opts backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_project_health", map[string]any{"project_key": "PROJ"})

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
	var envelope map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	analysis, ok := envelope["analysis"].(map[string]any)
	if !ok {
		t.Fatalf("expected analysis field in envelope, got %T", envelope["analysis"])
	}
	if _, ok := analysis["health_score"]; !ok {
		t.Error("expected health_score field in analysis")
	}
	if _, ok := analysis["health_level"]; !ok {
		t.Error("expected health_level field in analysis")
	}
}

// MCP-31: project_key 省略 → IsError: true
func TestProjectHealthHandler_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})

	result := callTool(t, s, "logvalet_project_health", map[string]any{})

	if !result.IsError {
		t.Error("expected IsError=true for missing project_key")
	}
}

// MCP-W1: logvalet_watching_list ハンドラーテスト
func TestWatchingListHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		if userID != 123 {
			t.Errorf("unexpected userID: %d", userID)
		}
		return []domain.Watching{
			{ID: 1, Type: "issue"},
			{ID: 2, Type: "issue"},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_watching_list", map[string]any{"user_id": float64(123)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var watchings []domain.Watching
	if err := json.Unmarshal([]byte(textContent.Text), &watchings); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(watchings) != 2 {
		t.Errorf("expected 2 watchings, got %d", len(watchings))
	}
}

// MCP-W2: logvalet_watching_list の user_id 省略 → IsError: true
func TestWatchingListHandler_MissingUserID(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})

	result := callTool(t, s, "logvalet_watching_list", map[string]any{})
	if !result.IsError {
		t.Error("expected IsError=true for missing user_id")
	}
}

// MCP-W3: logvalet_watching_count ハンドラーテスト
func TestWatchingCountHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.CountWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) (int, error) {
		return 7, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_watching_count", map[string]any{"user_id": float64(123)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var out map[string]int
	if err := json.Unmarshal([]byte(textContent.Text), &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if out["count"] != 7 {
		t.Errorf("expected count=7, got %d", out["count"])
	}
}

// MCP-W4: logvalet_watching_get ハンドラーテスト
func TestWatchingGetHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetWatchingFunc = func(ctx context.Context, watchingID int64) (*domain.Watching, error) {
		if watchingID != 42 {
			t.Errorf("unexpected watchingID: %d", watchingID)
		}
		return &domain.Watching{ID: 42, Type: "issue", Note: "my note"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_watching_get", map[string]any{"watching_id": float64(42)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var watching domain.Watching
	if err := json.Unmarshal([]byte(textContent.Text), &watching); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if watching.ID != 42 {
		t.Errorf("expected ID=42, got %d", watching.ID)
	}
	if watching.Note != "my note" {
		t.Errorf("expected note 'my note', got %q", watching.Note)
	}
}

// MCP-W5: logvalet_watching_add ハンドラーテスト
func TestWatchingAddHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.AddWatchingRequest
	mock.AddWatchingFunc = func(ctx context.Context, req backlog.AddWatchingRequest) (*domain.Watching, error) {
		capturedReq = req
		return &domain.Watching{ID: 100, Type: "issue"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_watching_add", map[string]any{
		"issue_id_or_key": "PROJ-1",
		"note":            "watch this",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.IssueIDOrKey != "PROJ-1" {
		t.Errorf("expected IssueIDOrKey=PROJ-1, got %q", capturedReq.IssueIDOrKey)
	}
	if capturedReq.Note != "watch this" {
		t.Errorf("expected Note='watch this', got %q", capturedReq.Note)
	}
}

// MCP-W6: logvalet_watching_mark_as_read ハンドラーテスト
func TestWatchingMarkAsReadHandler(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedWatchingID int64
	mock.MarkWatchingAsReadFunc = func(ctx context.Context, watchingID int64) error {
		capturedWatchingID = watchingID
		return nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_watching_mark_as_read", map[string]any{"watching_id": float64(42)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedWatchingID != 42 {
		t.Errorf("expected watchingID=42, got %d", capturedWatchingID)
	}
}

// callToolWithCtx は指定した context で ServerTool のハンドラーを呼び出すテストヘルパー。
func callToolWithCtx(t *testing.T, s *mcpserver.MCPServer, ctx context.Context, toolName string, args map[string]any) *gomcp.CallToolResult {
	t.Helper()
	serverTool := s.GetTool(toolName)
	if serverTool == nil {
		t.Fatalf("tool %q not found", toolName)
	}

	req := gomcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args

	result, err := serverTool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("tool %q handler returned error: %v", toolName, err)
	}
	return result
}

// M11-1: NewToolRegistryWithFactory でツール登録 → factory(ctx) が呼ばれてツール実行
func TestNewToolRegistryWithFactory_RegisterAndCall(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return &domain.Issue{
			ID:       1,
			IssueKey: issueKey,
			Summary:  "factory test",
		}, nil
	}

	factoryCalled := false
	factory := func(ctx context.Context) (backlog.Client, error) {
		factoryCalled = true
		return mock, nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithFactory(s, factory)

	// 簡単なツールを登録
	tool := gomcp.NewTool("test_tool",
		gomcp.WithDescription("test tool"),
		gomcp.WithString("issue_key", gomcp.Description("issue key"), gomcp.Required()),
	)
	reg.Register(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		key := args["issue_key"].(string)
		return client.GetIssue(ctx, key)
	})

	ctx := context.Background()
	result := callToolWithCtx(t, s, ctx, "test_tool", map[string]any{"issue_key": "TEST-1"})

	if !factoryCalled {
		t.Error("expected factory to be called")
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
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

// M11-2: factory がエラーを返した場合 → IsError: true、ツール関数は呼ばれない
func TestNewToolRegistryWithFactory_FactoryError(t *testing.T) {
	factoryErr := errors.New("user not authenticated")
	factory := func(ctx context.Context) (backlog.Client, error) {
		return nil, factoryErr
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithFactory(s, factory)

	fnCalled := false
	tool := gomcp.NewTool("test_tool",
		gomcp.WithDescription("test tool"),
	)
	reg.Register(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		fnCalled = true
		return nil, nil
	})

	result := callToolWithCtx(t, s, context.Background(), "test_tool", map[string]any{})

	if fnCalled {
		t.Error("expected tool function NOT to be called when factory returns error")
	}
	if !result.IsError {
		t.Error("expected IsError=true when factory returns error")
	}
}

// M11-3: 既存 NewToolRegistry は後方互換で動作する
func TestNewToolRegistry_BackwardCompat(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return &domain.Issue{
			ID:       1,
			IssueKey: issueKey,
			Summary:  "backward compat test",
		}, nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistry(s, mock)

	tool := gomcp.NewTool("test_tool",
		gomcp.WithDescription("test tool"),
		gomcp.WithString("issue_key", gomcp.Description("issue key"), gomcp.Required()),
	)
	reg.Register(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		key := args["issue_key"].(string)
		return client.GetIssue(ctx, key)
	})

	result := callToolWithCtx(t, s, context.Background(), "test_tool", map[string]any{"issue_key": "TEST-1"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
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
