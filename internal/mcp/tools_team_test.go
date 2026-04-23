package mcp_test

import (
	"context"
	"strings"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== A6: team_list 追加パラメータテスト =====

// TestTeamList_Default は既存動作（パラメータなし）が正常に動作することを確認する。
func TestTeamList_Default(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListTeamsFunc = func(ctx context.Context, opt backlog.ListTeamsOptions) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 1, Name: "チームA", Members: []domain.User{{ID: 10, Name: "Alice"}}},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_list", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("ListTeams") != 1 {
		t.Errorf("expected ListTeams called 1 time, got %d", mock.GetCallCount("ListTeams"))
	}
}

// TestTeamList_WithCount は count が ListTeamsOptions.Count に設定されることを確認する。
func TestTeamList_WithCount(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListTeamsOptions
	mock.ListTeamsFunc = func(ctx context.Context, opt backlog.ListTeamsOptions) ([]domain.TeamWithMembers, error) {
		capturedOpt = opt
		return []domain.TeamWithMembers{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_list", map[string]any{"count": 50})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Count != 50 {
		t.Errorf("expected Count=50, got %d", capturedOpt.Count)
	}
}

// TestTeamList_WithOffset は offset が ListTeamsOptions.Offset に設定されることを確認する。
func TestTeamList_WithOffset(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListTeamsOptions
	mock.ListTeamsFunc = func(ctx context.Context, opt backlog.ListTeamsOptions) ([]domain.TeamWithMembers, error) {
		capturedOpt = opt
		return []domain.TeamWithMembers{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_list", map[string]any{"offset": 10})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Offset != 10 {
		t.Errorf("expected Offset=10, got %d", capturedOpt.Offset)
	}
}

// TestTeamList_WithNoMembers_True は no_members=true でメンバー情報が除外されることを確認する。
// 返却される JSON に "members" キーが含まれないこと。
func TestTeamList_WithNoMembers_True(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListTeamsFunc = func(ctx context.Context, opt backlog.ListTeamsOptions) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 1, Name: "チームA", Members: []domain.User{{ID: 10, Name: "Alice"}}},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_list", map[string]any{"no_members": true})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	// no_members=true の場合、JSON に "members" キーが含まれないこと
	if strings.Contains(textContent.Text, `"members"`) {
		t.Errorf("expected no 'members' key in output when no_members=true, got: %s", textContent.Text)
	}
}

// TestTeamList_WithNoMembers_False はデフォルト動作と同一（メンバー情報含む）。
func TestTeamList_WithNoMembers_False(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListTeamsFunc = func(ctx context.Context, opt backlog.ListTeamsOptions) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 1, Name: "チームA", Members: []domain.User{{ID: 10, Name: "Alice"}}},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_list", map[string]any{"no_members": false})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("ListTeams") != 1 {
		t.Errorf("expected ListTeams called 1 time, got %d", mock.GetCallCount("ListTeams"))
	}
}

// ===== B11: logvalet_team_project =====

// TestTeamProject_Normal は project_key 指定で ListProjectTeams が呼ばれることを確認する。
func TestTeamProject_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedProjectKey string
	mock.ListProjectTeamsFunc = func(ctx context.Context, projectKey string) ([]domain.Team, error) {
		capturedProjectKey = projectKey
		return []domain.Team{{ID: 1, Name: "チームA"}, {ID: 2, Name: "チームB"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_project", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedProjectKey != "PROJ" {
		t.Errorf("projectKey = %q, want %q", capturedProjectKey, "PROJ")
	}
	if mock.GetCallCount("ListProjectTeams") != 1 {
		t.Errorf("expected ListProjectTeams called 1 time, got %d", mock.GetCallCount("ListProjectTeams"))
	}
}

// TestTeamProject_MissingProjectKey は project_key 未指定で IsError=true になることを確認する。
func TestTeamProject_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_team_project", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}
