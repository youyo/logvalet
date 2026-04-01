package backlog_test

import (
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
)

func TestListIssuesOptions(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var opt backlog.ListIssuesOptions
		if opt.Limit != 0 || opt.Offset != 0 {
			t.Error("zero value should have Limit=0, Offset=0")
		}
	})

	t.Run("all fields settable", func(t *testing.T) {
		since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		until := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
		opt := backlog.ListIssuesOptions{
			ProjectIDs:   []int{1, 2},
			AssigneeIDs:  []int{10, 11},
			StatusIDs:    []int{1, 2},
			DueDateSince: &since,
			DueDateUntil: &until,
			Sort:         "dueDate",
			Order:        "asc",
			Limit:        20,
			Offset:       10,
		}
		if len(opt.ProjectIDs) != 2 || opt.ProjectIDs[0] != 1 {
			t.Errorf("ProjectIDs = %v, want [1 2]", opt.ProjectIDs)
		}
		if len(opt.AssigneeIDs) != 2 || opt.AssigneeIDs[0] != 10 {
			t.Errorf("AssigneeIDs = %v, want [10 11]", opt.AssigneeIDs)
		}
		if len(opt.StatusIDs) != 2 {
			t.Errorf("StatusIDs = %v, want [1 2]", opt.StatusIDs)
		}
		if opt.DueDateSince == nil || !opt.DueDateSince.Equal(since) {
			t.Errorf("DueDateSince = %v, want %v", opt.DueDateSince, since)
		}
		if opt.Sort != "dueDate" {
			t.Errorf("Sort = %q, want %q", opt.Sort, "dueDate")
		}
		if opt.Order != "asc" {
			t.Errorf("Order = %q, want %q", opt.Order, "asc")
		}
		if opt.Limit != 20 {
			t.Errorf("Limit = %d, want 20", opt.Limit)
		}
	})

	t.Run("Sort and Order zero value is empty string", func(t *testing.T) {
		var opt backlog.ListIssuesOptions
		if opt.Sort != "" {
			t.Errorf("Sort zero value = %q, want empty string", opt.Sort)
		}
		if opt.Order != "" {
			t.Errorf("Order zero value = %q, want empty string", opt.Order)
		}
	})
}

func TestListCommentsOptions(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var opt backlog.ListCommentsOptions
		if opt.Limit != 0 || opt.Offset != 0 {
			t.Error("zero value should have Limit=0, Offset=0")
		}
	})

	t.Run("set fields", func(t *testing.T) {
		opt := backlog.ListCommentsOptions{
			Limit:  5,
			Offset: 0,
		}
		if opt.Limit != 5 {
			t.Errorf("Limit = %d, want 5", opt.Limit)
		}
	})
}

func TestListActivitiesOptions(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var opt backlog.ListActivitiesOptions
		if opt.Count != 0 || opt.MinId != 0 || opt.MaxId != 0 {
			t.Error("zero value should have Count=0, MinId=0, MaxId=0")
		}
		if opt.ActivityTypeIDs != nil {
			t.Error("zero value should have ActivityTypeIDs=nil")
		}
	})

	t.Run("set Count and MaxId", func(t *testing.T) {
		opt := backlog.ListActivitiesOptions{
			Count: 100,
			MaxId: 12345,
			Order: "desc",
		}
		if opt.Count != 100 {
			t.Errorf("Count = %d, want 100", opt.Count)
		}
		if opt.MaxId != 12345 {
			t.Errorf("MaxId = %d, want 12345", opt.MaxId)
		}
		if opt.Order != "desc" {
			t.Errorf("Order = %q, want %q", opt.Order, "desc")
		}
	})

	t.Run("set ActivityTypeIDs", func(t *testing.T) {
		opt := backlog.ListActivitiesOptions{
			ActivityTypeIDs: []int{1, 2, 3},
		}
		if len(opt.ActivityTypeIDs) != 3 {
			t.Errorf("len(ActivityTypeIDs) = %d, want 3", len(opt.ActivityTypeIDs))
		}
	})
}

func TestListUserActivitiesOptions(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var opt backlog.ListUserActivitiesOptions
		if opt.ActivityTypeIDs != nil {
			t.Error("zero value should have ActivityTypeIDs=nil")
		}
		if opt.Count != 0 || opt.MinId != 0 || opt.MaxId != 0 {
			t.Error("zero value should have Count=0, MinId=0, MaxId=0")
		}
	})

	t.Run("set ActivityTypeIDs", func(t *testing.T) {
		opt := backlog.ListUserActivitiesOptions{
			ActivityTypeIDs: []int{1, 3},
			Count:           50,
		}
		if len(opt.ActivityTypeIDs) != 2 {
			t.Errorf("len(ActivityTypeIDs) = %d, want 2", len(opt.ActivityTypeIDs))
		}
		if opt.Count != 50 {
			t.Errorf("Count = %d, want 50", opt.Count)
		}
	})
}

func TestListDocumentsOptions(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var opt backlog.ListDocumentsOptions
		if opt.Limit != 0 {
			t.Errorf("Limit = %d, want 0", opt.Limit)
		}
	})
}
