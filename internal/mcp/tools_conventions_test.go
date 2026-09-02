package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/conventions"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

func TestProjectConventionsTool_IsRegisteredWithRequiredProjectKey(t *testing.T) {
	server := newTestServer(t, backlog.NewMockClient(), mcpinternal.ServerConfig{})
	tool := server.GetTool("logvalet_project_conventions")
	if tool == nil {
		t.Fatal("logvalet_project_conventions is not registered")
	}
	if len(tool.Required) != 1 || tool.Required[0] != "project_key" {
		t.Fatalf("required = %v, want [project_key]", tool.Required)
	}
}

func TestProjectConventionsTool_MissingProjectKeyReturnsError(t *testing.T) {
	server := newTestServer(t, backlog.NewMockClient(), mcpinternal.ServerConfig{})
	result := callTool(t, server, "logvalet_project_conventions", map[string]any{})
	if !result.IsError {
		t.Fatal("missing project_key did not return an error")
	}
}

func TestProjectConventionsTool_ReturnsShowResult(t *testing.T) {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return &domain.Project{ID: 42, ProjectKey: "PROJ", Name: "Project"}, nil
	}
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) {
		return []domain.IssueType{{ID: 7, Name: conventions.IssueTypeRule}}, nil
	}
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		description := conventions.BuildRuleIssueDescription([]byte("schema_version: 1\nproject:\n  key: PROJ\n  name: Project\n"))
		return []domain.Issue{{
			IssueKey:    "PROJ-1",
			Description: description,
			IssueType:   &domain.IDName{Name: conventions.IssueTypeRule},
		}}, nil
	}

	server := newTestServer(t, client, mcpinternal.ServerConfig{})
	result := callTool(t, server, "logvalet_project_conventions", map[string]any{"project_key": "PROJ"})
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	var got conventions.ShowResult
	if err := json.Unmarshal([]byte(resultTextContent(t, result).Text), &got); err != nil {
		t.Fatalf("result JSON decode error: %v", err)
	}
	if !got.Adopted || got.IssueKey != "PROJ-1" || got.Conventions == nil {
		t.Fatalf("result = %#v, want adopted PROJ-1 conventions", got)
	}
}
