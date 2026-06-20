package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

func TestSearch_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{{ID: 1, ProjectKey: "PROJ"}}, nil
	}
	var capturedIssueOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedIssueOpt = opt
		return []domain.Issue{{ID: 1, ProjectID: 1, IssueKey: "PROJ-1", Summary: "OAuth issue"}}, nil
	}
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		return []domain.Document{}, nil
	}
	mock.ListWikisFunc = func(ctx context.Context, projectKey string, opt backlog.ListWikisOptions) ([]domain.WikiPage, error) {
		return []domain.WikiPage{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_search", map[string]any{
		"keyword": "OAuth",
		"count":   float64(5),
	})
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedIssueOpt.Keyword != "OAuth" {
		t.Errorf("issue Keyword = %q, want OAuth", capturedIssueOpt.Keyword)
	}
	if capturedIssueOpt.Limit != 5 {
		t.Errorf("issue Limit = %d, want 5", capturedIssueOpt.Limit)
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var envelope domain.DigestEnvelope
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	digestBytes, err := json.Marshal(envelope.Digest)
	if err != nil {
		t.Fatalf("marshal digest: %v", err)
	}
	var d digest.SearchDigest
	if err := json.Unmarshal(digestBytes, &d); err != nil {
		t.Fatalf("unmarshal SearchDigest: %v", err)
	}
	if d.Keyword != "OAuth" || d.ReturnedByType.Issues != 1 {
		t.Errorf("digest = %+v", d)
	}
}

func TestSearch_MissingKeyword(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_search", map[string]any{})
	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}
