package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// TestIssueRelatedTools_Registered は関連課題 3 ツールが ToolRegistry に登録されることを確認する。
func TestIssueRelatedTools_Registered(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(t, mock, mcpinternal.ServerConfig{})

	tools := s.toolNames()
	for _, name := range []string{
		"logvalet_issue_related_list",
		"logvalet_issue_related_add",
		"logvalet_issue_related_delete",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q が登録されていない", name)
		}
	}
}

// TestIssueRelatedList_Normal は list ハンドラーが ListRelatedIssues へ委譲することを確認する。
func TestIssueRelatedList_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
		if issueKey != "PROJ-1" {
			t.Errorf("issueKey = %q, want PROJ-1", issueKey)
		}
		return []domain.RelatedIssue{{Issue: domain.Issue{ID: 30, IssueKey: "PROJ-2", Summary: "related"}, Type: "RELATES"}}, nil
	}

	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_list", map[string]any{"issue_key": "PROJ-1"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	textContent := resultTextContent(t, result)
	var got []domain.RelatedIssue
	if err := json.Unmarshal([]byte(textContent.Text), &got); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(got) != 1 || got[0].IssueKey != "PROJ-2" {
		t.Errorf("unexpected result: %+v", got)
	}
	if mock.GetCallCount("ListRelatedIssues") != 1 {
		t.Errorf("ListRelatedIssues call count = %d, want 1", mock.GetCallCount("ListRelatedIssues"))
	}
}

// TestIssueRelatedList_MissingIssueKey は issue_key 必須を確認する。
func TestIssueRelatedList_MissingIssueKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_list", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestIssueRelatedAdd_Normal は add ハンドラーが AddRelatedIssue へ委譲することを確認する。
func TestIssueRelatedAdd_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedReq backlog.AddRelatedIssueRequest
	mock.AddRelatedIssueFunc = func(ctx context.Context, issueKey string, req backlog.AddRelatedIssueRequest) (*domain.RelatedIssue, error) {
		if issueKey != "PROJ-1" {
			t.Errorf("issueKey = %q, want PROJ-1", issueKey)
		}
		capturedReq = req
		return &domain.RelatedIssue{Issue: domain.Issue{ID: 30, IssueKey: "PROJ-2"}, Type: "RELATES"}, nil
	}

	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_add", map[string]any{
		"issue_key":       "PROJ-1",
		"target_issue_id": float64(12345),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedReq.TargetIssueID != 12345 {
		t.Errorf("TargetIssueID = %d, want 12345", capturedReq.TargetIssueID)
	}
	if mock.GetCallCount("AddRelatedIssue") != 1 {
		t.Errorf("AddRelatedIssue call count = %d, want 1", mock.GetCallCount("AddRelatedIssue"))
	}
}

// TestIssueRelatedAdd_MissingTargetIssueID は target_issue_id 必須を確認する。
func TestIssueRelatedAdd_MissingTargetIssueID(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_add", map[string]any{"issue_key": "PROJ-1"})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestIssueRelatedAdd_MissingIssueKey は issue_key 必須を確認する。
func TestIssueRelatedAdd_MissingIssueKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_add", map[string]any{"target_issue_id": float64(12345)})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestIssueRelatedDelete_Normal は delete ハンドラーが DeleteRelatedIssue へ委譲することを確認する。
func TestIssueRelatedDelete_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedID int64
	mock.DeleteRelatedIssueFunc = func(ctx context.Context, issueKey string, relatedIssueID int64) (*domain.RelatedIssue, error) {
		if issueKey != "PROJ-1" {
			t.Errorf("issueKey = %q, want PROJ-1", issueKey)
		}
		capturedID = relatedIssueID
		return &domain.RelatedIssue{Issue: domain.Issue{ID: 30, IssueKey: "PROJ-2"}, Type: "RELATES"}, nil
	}

	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_delete", map[string]any{
		"issue_key":        "PROJ-1",
		"related_issue_id": float64(30),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedID != 30 {
		t.Errorf("relatedIssueID = %d, want 30", capturedID)
	}
	if mock.GetCallCount("DeleteRelatedIssue") != 1 {
		t.Errorf("DeleteRelatedIssue call count = %d, want 1", mock.GetCallCount("DeleteRelatedIssue"))
	}
}

// TestIssueRelatedDelete_MissingRelatedIssueID は related_issue_id 必須を確認する。
func TestIssueRelatedDelete_MissingRelatedIssueID(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_delete", map[string]any{"issue_key": "PROJ-1"})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestIssueRelatedDelete_MissingIssueKey は issue_key 必須を確認する。
func TestIssueRelatedDelete_MissingIssueKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_related_delete", map[string]any{"related_issue_id": float64(30)})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}
