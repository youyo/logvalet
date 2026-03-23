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
		if opt.Limit != 20 {
			t.Errorf("Limit = %d, want 20", opt.Limit)
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
		if opt.Since != nil || opt.Until != nil {
			t.Error("zero value should have Since=nil, Until=nil")
		}
	})

	t.Run("set Since and Until", func(t *testing.T) {
		since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		until := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		opt := backlog.ListActivitiesOptions{
			Since: &since,
			Until: &until,
			Limit: 100,
		}
		if opt.Since == nil || !opt.Since.Equal(since) {
			t.Errorf("Since = %v, want %v", opt.Since, since)
		}
		if opt.Until == nil || !opt.Until.Equal(until) {
			t.Errorf("Until = %v, want %v", opt.Until, until)
		}
	})
}

func TestListUserActivitiesOptions(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var opt backlog.ListUserActivitiesOptions
		if opt.Types != nil {
			t.Error("zero value should have Types=nil")
		}
	})

	t.Run("Types slice", func(t *testing.T) {
		opt := backlog.ListUserActivitiesOptions{
			Types:   []string{"issue_created", "issue_commented"},
			Project: "PROJ",
		}
		if len(opt.Types) != 2 {
			t.Errorf("len(Types) = %d, want 2", len(opt.Types))
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
