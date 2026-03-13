package backlog_test

import (
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
)

func TestCreateIssueRequest(t *testing.T) {
	t.Run("required fields", func(t *testing.T) {
		req := backlog.CreateIssueRequest{
			ProjectKey: "PROJ",
			Summary:    "Test issue",
			IssueType:  "Bug",
		}
		if req.ProjectKey != "PROJ" {
			t.Errorf("ProjectKey = %q, want %q", req.ProjectKey, "PROJ")
		}
		if req.Summary != "Test issue" {
			t.Errorf("Summary = %q, want %q", req.Summary, "Test issue")
		}
	})

	t.Run("optional fields", func(t *testing.T) {
		due := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		req := backlog.CreateIssueRequest{
			ProjectKey:   "PROJ",
			Summary:      "Test",
			IssueType:    "Task",
			Description:  "desc",
			Priority:     "high",
			Assignee:     "user1",
			Categories:   []string{"cat1"},
			Versions:     []string{"v1"},
			Milestones:   []string{"m1"},
			DueDate:      &due,
			CustomFields: map[string]string{"field1": "val1"},
		}
		if req.DueDate == nil || !req.DueDate.Equal(due) {
			t.Errorf("DueDate = %v, want %v", req.DueDate, due)
		}
		if req.CustomFields["field1"] != "val1" {
			t.Error("CustomFields not set correctly")
		}
	})
}

func TestUpdateIssueRequest(t *testing.T) {
	t.Run("all fields are pointers (optional)", func(t *testing.T) {
		var req backlog.UpdateIssueRequest
		// 全フィールドがポインタ型のため、ゼロ値では nil
		if req.Summary != nil {
			t.Error("Summary should be nil by default")
		}
		if req.Description != nil {
			t.Error("Description should be nil by default")
		}
		if req.Status != nil {
			t.Error("Status should be nil by default")
		}
		if req.Priority != nil {
			t.Error("Priority should be nil by default")
		}
		if req.Assignee != nil {
			t.Error("Assignee should be nil by default")
		}
	})

	t.Run("partial update: set only Summary", func(t *testing.T) {
		summary := "Updated summary"
		req := backlog.UpdateIssueRequest{
			Summary: &summary,
		}
		if req.Summary == nil || *req.Summary != summary {
			t.Errorf("Summary = %v, want %q", req.Summary, summary)
		}
	})
}

func TestAddCommentRequest(t *testing.T) {
	req := backlog.AddCommentRequest{Content: "Hello"}
	if req.Content != "Hello" {
		t.Errorf("Content = %q, want %q", req.Content, "Hello")
	}
}

func TestUpdateCommentRequest(t *testing.T) {
	req := backlog.UpdateCommentRequest{Content: "Updated"}
	if req.Content != "Updated" {
		t.Errorf("Content = %q, want %q", req.Content, "Updated")
	}
}

func TestCreateDocumentRequest(t *testing.T) {
	t.Run("without parent", func(t *testing.T) {
		req := backlog.CreateDocumentRequest{
			ProjectKey: "PROJ",
			Title:      "Test Doc",
			Content:    "Content",
		}
		if req.ParentID != nil {
			t.Error("ParentID should be nil")
		}
	})

	t.Run("with parent", func(t *testing.T) {
		parentID := int64(100)
		req := backlog.CreateDocumentRequest{
			ProjectKey: "PROJ",
			Title:      "Child Doc",
			Content:    "Content",
			ParentID:   &parentID,
		}
		if req.ParentID == nil || *req.ParentID != 100 {
			t.Errorf("ParentID = %v, want 100", req.ParentID)
		}
	})
}

func TestPagination(t *testing.T) {
	p := backlog.Pagination{
		Total:  100,
		Offset: 20,
		Limit:  20,
	}
	if p.Total != 100 {
		t.Errorf("Total = %d, want 100", p.Total)
	}
}

func TestRateLimitInfo(t *testing.T) {
	r := backlog.RateLimitInfo{
		Limit:     600,
		Remaining: 599,
		Reset:     1800000000,
	}
	if r.Limit != 600 {
		t.Errorf("Limit = %d, want 600", r.Limit)
	}
	if r.Remaining != 599 {
		t.Errorf("Remaining = %d, want 599", r.Remaining)
	}
}
