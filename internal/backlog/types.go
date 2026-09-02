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
	ParentIssueID   int // 0 = 未指定（UpdateIssueRequest.ParentIssueID の 0=解除とは意味が異なる）
	NotifiedUserIDs []int
	CustomFields    map[string]string
	AttachmentIDs   []int64 // UploadAttachment で取得した添付 ID
}

// CreateProjectRequest は CreateProject リクエストのパラメータ。
// bool はポインタ（nil = 未指定。API の既定に委ねる）。
type CreateProjectRequest struct {
	Name                              string // 必須
	Key                               string // 必須。大文字英数字と _ のみ
	ChartEnabled                      *bool
	SubtaskingEnabled                 *bool // nil のときは true を送る
	GrandchildIssueEnabled            *bool
	ProjectLeaderCanEditProjectLeader *bool
	UseDevAttributes                  *bool
	TextFormattingRule                string // "backlog" | "markdown"。空なら送らない
}

// AddCategoryRequest はカテゴリ追加のパラメータ。
type AddCategoryRequest struct {
	Name string // 必須
}

// UpdateCategoryRequest はカテゴリ更新のパラメータ。
type UpdateCategoryRequest struct {
	Name string // 必須
}

// AddIssueTypeRequest は課題種別追加のパラメータ。
type AddIssueTypeRequest struct {
	Name                string // 必須
	Color               string // 必須。Backlog が許可する色コードのみ
	TemplateSummary     string
	TemplateDescription string
}

// UpdateIssueTypeRequest は課題種別更新のパラメータ。
// 全フィールドはポインタ型（nil = 変更しない）。
type UpdateIssueTypeRequest struct {
	Name                *string
	Color               *string
	TemplateSummary     *string
	TemplateDescription *string
}

// AddStatusRequest は状態追加のパラメータ。
type AddStatusRequest struct {
	Name  string // 必須
	Color string // 必須。状態用の許可色コードのみ
}

// UpdateStatusRequest は状態更新のパラメータ。
// 全フィールドはポインタ型（nil = 変更しない）。
type UpdateStatusRequest struct {
	Name  *string
	Color *string
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
	ParentIssueID   *int // 0 = 親課題解除、nil = 変更しない（CreateIssueRequest.ParentIssueID の 0=未指定とは意味が異なる）
	CategoryIDs     []int
	VersionIDs      []int
	MilestoneIDs    []int
	DueDate         *time.Time
	StartDate       *time.Time
	NotifiedUserIDs []int
	Comment         *string
	CustomFields    map[string]string
	AttachmentIDs   []int64 // UploadAttachment で取得した添付 ID
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

// AddWatchingRequest は AddWatching リクエストのパラメータ。
// issueIdOrKey は必須。note はオプション。
type AddWatchingRequest struct {
	IssueIDOrKey string
	Note         string
}

// UpdateWatchingRequest は UpdateWatching リクエストのパラメータ。
// note のみ更新可能。
type UpdateWatchingRequest struct {
	Note string
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
