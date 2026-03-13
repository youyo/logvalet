package backlog

import "time"

// ---- Write request types (spec §18.3) ----

// CreateIssueRequest は CreateIssue リクエストのパラメータ。
type CreateIssueRequest struct {
	ProjectKey   string
	Summary      string
	IssueType    string
	Description  string
	Priority     string
	Assignee     string
	Categories   []string
	Versions     []string
	Milestones   []string
	DueDate      *time.Time
	StartDate    *time.Time
	CustomFields map[string]string
}

// UpdateIssueRequest は UpdateIssue リクエストのパラメータ。
// 全フィールドはポインタ型（nil = 変更しない）。
type UpdateIssueRequest struct {
	Summary      *string
	Description  *string
	Status       *string
	Priority     *string
	Assignee     *string
	Categories   []string
	Versions     []string
	Milestones   []string
	DueDate      *time.Time
	StartDate    *time.Time
	CustomFields map[string]string
}

// AddCommentRequest は AddIssueComment リクエストのパラメータ。
type AddCommentRequest struct {
	Content string
}

// UpdateCommentRequest は UpdateIssueComment リクエストのパラメータ。
type UpdateCommentRequest struct {
	Content string
}

// CreateDocumentRequest は CreateDocument リクエストのパラメータ。
type CreateDocumentRequest struct {
	ProjectKey string
	Title      string
	Content    string
	ParentID   *int64
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
