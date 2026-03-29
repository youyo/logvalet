package backlog

import "time"

// ---- Write request types (spec §18.3) ----

// CreateIssueRequest は CreateIssue リクエストのパラメータ。
type CreateIssueRequest struct {
	ProjectID       int
	Summary         string
	IssueTypeID     int
	Description     string
	PriorityID      int // 0 = 未指定
	AssigneeID      int // 0 = 未指定
	CategoryIDs     []int
	VersionIDs      []int
	MilestoneIDs    []int
	DueDate         *time.Time
	StartDate       *time.Time
	ParentIssueID   int // 0 = 未指定
	NotifiedUserIDs []int
	CustomFields    map[string]string
}

// UpdateIssueRequest は UpdateIssue リクエストのパラメータ。
// 全フィールドはポインタ型（nil = 変更しない）。
type UpdateIssueRequest struct {
	Summary         *string
	Description     *string
	StatusID        *int
	PriorityID      *int
	AssigneeID      *int
	IssueTypeID     *int
	CategoryIDs     []int
	VersionIDs      []int
	MilestoneIDs    []int
	DueDate         *time.Time
	StartDate       *time.Time
	NotifiedUserIDs []int
	Comment         *string
	CustomFields    map[string]string
}

// AddCommentRequest は AddIssueComment リクエストのパラメータ。
type AddCommentRequest struct {
	Content         string
	NotifiedUserIDs []int
}

// UpdateCommentRequest は UpdateIssueComment リクエストのパラメータ。
type UpdateCommentRequest struct {
	Content string
}

// CreateDocumentRequest は CreateDocument リクエストのパラメータ。
type CreateDocumentRequest struct {
	ProjectID int
	Title     string
	Content   string
	ParentID  *string
	Emoji     string
	AddLast   bool
}

// AddStarRequest は AddStar リクエストのパラメータ。
// 各フィールドはポインタ型（nil = 指定なし）。
// issueId, commentId, wikiId, pullRequestId, pullRequestCommentId のいずれか1つを指定する。
type AddStarRequest struct {
	IssueID              *int `json:"issueId,omitempty"`
	CommentID            *int `json:"commentId,omitempty"`
	WikiID               *int `json:"wikiId,omitempty"`
	PullRequestID        *int `json:"pullRequestId,omitempty"`
	PullRequestCommentID *int `json:"pullRequestCommentId,omitempty"`
}

// ---- Response metadata ----

// Pagination はリスト API のページネーション情報。
type Pagination struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// RateLimitInfo は Backlog API のレートリミット情報。
// X-Ratelimit-* ヘッダから解析する。
type RateLimitInfo struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"` // unix timestamp
}
