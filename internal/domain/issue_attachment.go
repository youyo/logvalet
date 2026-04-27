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

// UploadedAttachment は POST /api/v2/space/attachment のレスポンス。
// アップロード後に発行される一時的な添付ファイル ID を保持する。
// この ID を CreateIssue / UpdateIssue の attachmentId[] に渡すことで課題に添付できる。
type UploadedAttachment struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}
