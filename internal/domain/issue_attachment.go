package domain

import "time"

// IssueAttachment は Backlog の課題添付ファイルモデル。
// Backlog API: GET /api/v2/issues/{issueIdOrKey}/attachments
type IssueAttachment struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Size        int64      `json:"size"`
	CreatedUser *User      `json:"createdUser,omitempty"`
	Created     *time.Time `json:"created,omitempty"`
}
