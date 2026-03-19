package backlog_test

import (
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
)

func TestCreateIssueRequest(t *testing.T) {
	t.Run("required fields", func(t *testing.T) {
		req := backlog.CreateIssueRequest{
			ProjectID:   42,
			Summary:     "Test issue",
			IssueTypeID: 1,
			PriorityID:  3,
		}
		if req.ProjectID != 42 {
			t.Errorf("ProjectID = %d, want 42", req.ProjectID)
		}
		if req.Summary != "Test issue" {
			t.Errorf("Summary = %q, want %q", req.Summary, "Test issue")
		}
	})

	t.Run("optional fields", func(t *testing.T) {
		due := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		req := backlog.CreateIssueRequest{
			ProjectID:       42,
			Summary:         "Test",
			IssueTypeID:     1,
			PriorityID:      3,
			Description:     "desc",
			AssigneeID:      101,
			CategoryIDs:     []int{1, 2},
			VersionIDs:      []int{10},
			MilestoneIDs:    []int{20},
			DueDate:         &due,
			NotifiedUserIDs: []int{50},
			CustomFields:    map[string]string{"field1": "val1"},
		}
		if req.DueDate == nil || !req.DueDate.Equal(due) {
			t.Errorf("DueDate = %v, want %v", req.DueDate, due)
		}
		if req.CustomFields["field1"] != "val1" {
			t.Error("CustomFields not set correctly")
		}
		if req.AssigneeID != 101 {
			t.Errorf("AssigneeID = %d, want 101", req.AssigneeID)
		}
	})
}

func TestUpdateIssueRequest(t *testing.T) {
	t.Run("all pointer fields are nil by default", func(t *testing.T) {
		var req backlog.UpdateIssueRequest
		if req.Summary != nil {
			t.Error("Summary should be nil by default")
		}
		if req.Description != nil {
			t.Error("Description should be nil by default")
		}
		if req.StatusID != nil {
			t.Error("StatusID should be nil by default")
		}
		if req.PriorityID != nil {
			t.Error("PriorityID should be nil by default")
		}
		if req.AssigneeID != nil {
			t.Error("AssigneeID should be nil by default")
		}
		if req.IssueTypeID != nil {
			t.Error("IssueTypeID should be nil by default")
		}
		if req.Comment != nil {
			t.Error("Comment should be nil by default")
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

	t.Run("set StatusID", func(t *testing.T) {
		statusID := 2
		req := backlog.UpdateIssueRequest{
			StatusID: &statusID,
		}
		if req.StatusID == nil || *req.StatusID != 2 {
			t.Errorf("StatusID = %v, want 2", req.StatusID)
		}
	})
}

func TestAddCommentRequest(t *testing.T) {
	t.Run("content only", func(t *testing.T) {
		req := backlog.AddCommentRequest{Content: "Hello"}
		if req.Content != "Hello" {
			t.Errorf("Content = %q, want %q", req.Content, "Hello")
		}
		if len(req.NotifiedUserIDs) != 0 {
			t.Error("NotifiedUserIDs should be empty by default")
		}
	})

	t.Run("with notified users", func(t *testing.T) {
		req := backlog.AddCommentRequest{
			Content:         "Hello",
			NotifiedUserIDs: []int{1, 2, 3},
		}
		if len(req.NotifiedUserIDs) != 3 {
			t.Errorf("NotifiedUserIDs len = %d, want 3", len(req.NotifiedUserIDs))
		}
	})
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
			ProjectID: 10,
			Title:     "Test Doc",
			Content:   "Content",
		}
		if req.ParentID != nil {
			t.Error("ParentID should be nil")
		}
		if req.Emoji != "" {
			t.Error("Emoji should be empty by default")
		}
		if req.AddLast {
			t.Error("AddLast should be false by default")
		}
	})

	t.Run("with parent and emoji", func(t *testing.T) {
		parentID := "parent-uuid-100"
		req := backlog.CreateDocumentRequest{
			ProjectID: 10,
			Title:     "Child Doc",
			Content:   "Content",
			ParentID:  &parentID,
			Emoji:     "📝",
			AddLast:   true,
		}
		if req.ParentID == nil || *req.ParentID != "parent-uuid-100" {
			t.Errorf("ParentID = %v, want %q", req.ParentID, "parent-uuid-100")
		}
		if req.Emoji != "📝" {
			t.Errorf("Emoji = %q, want %q", req.Emoji, "📝")
		}
		if !req.AddLast {
			t.Error("AddLast should be true")
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
