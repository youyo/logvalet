package backlog

import "time"

// ListIssuesOptions は GetIssues リクエストのオプション。
// spec §18.2 準拠。
type ListIssuesOptions struct {
	ProjectKey string
	Assignee   string
	Status     string
	Limit      int
	Offset     int
}

// ListCommentsOptions は ListIssueComments リクエストのオプション。
// spec §18.2 準拠。
type ListCommentsOptions struct {
	Limit  int
	Offset int
}

// ListActivitiesOptions は ListProjectActivities / ListSpaceActivities リクエストのオプション。
// spec §18.2 準拠。
type ListActivitiesOptions struct {
	ProjectKey string
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// ListUserActivitiesOptions は ListUserActivities リクエストのオプション。
// spec §18.2 準拠。
type ListUserActivitiesOptions struct {
	Since   *time.Time
	Until   *time.Time
	Limit   int
	Offset  int
	Project string
	Types   []string
}

// ListDocumentsOptions は ListDocuments リクエストのオプション。
// spec §18.2 準拠。
type ListDocumentsOptions struct {
	Limit  int
	Offset int
}
