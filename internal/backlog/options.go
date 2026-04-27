package backlog

import "time"

// ListIssuesOptions は GetIssues リクエストのオプション。
// spec §18.2 準拠。
type ListIssuesOptions struct {
	ProjectIDs   []int
	AssigneeIDs  []int
	StatusIDs    []int
	DueDateSince   *time.Time
	DueDateUntil   *time.Time
	StartDateSince *time.Time
	StartDateUntil *time.Time
	UpdatedSince *time.Time
	UpdatedUntil *time.Time
	Sort         string
	Order        string
	Limit        int
	Offset       int
}

// ListCommentsOptions は ListIssueComments リクエストのオプション。
// spec §18.2 準拠。
type ListCommentsOptions struct {
	Limit  int
	Offset int
}

// ListActivitiesOptions は ListProjectActivities / ListSpaceActivities リクエストのオプション。
// Backlog API 仕様に準拠: activityTypeId[], minId, maxId, count, order のみサポート。
// offset/since/until は API 非サポートのためクライアント側でフィルタリングすること。
type ListActivitiesOptions struct {
	ActivityTypeIDs []int  // activityTypeId[] フィルタ
	MinId           int    // minId: この ID 以上の活動を取得（0 = 制限なし）
	MaxId           int    // maxId: この ID 以下の活動を取得（0 = 制限なし）
	Count           int    // count: 取得件数（最大100）
	Order           string // "asc" or "desc"（空文字 = API デフォルト desc）
}

// ListUserActivitiesOptions は ListUserActivities リクエストのオプション。
// Backlog API 仕様に準拠: activityTypeId[], minId, maxId, count, order のみサポート。
// offset/since/until は API 非サポートのためクライアント側でフィルタリングすること。
type ListUserActivitiesOptions struct {
	ActivityTypeIDs []int  // activityTypeId[] フィルタ
	MinId           int    // minId
	MaxId           int    // maxId
	Count           int    // count: 取得件数（最大100）
	Order           string // "asc" or "desc"
}

// ListDocumentsOptions は ListDocuments リクエストのオプション。
// spec §18.2 準拠。
type ListDocumentsOptions struct {
	Limit  int
	Offset int
}

// ListSharedFilesOptions は ListSharedFiles リクエストのオプション。
type ListSharedFilesOptions struct {
	Path   string
	Limit  int
	Offset int
}

// ListTeamsOptions は ListTeams リクエストのオプション。
// Backlog API: GET /api/v2/teams
type ListTeamsOptions struct {
	Order  string // "asc" or "desc"（空文字 = API デフォルト）
	Offset int    // 0=未指定
	Count  int    // 取得件数（最大100、0=未指定=API デフォルト）
}

// ListWatchingsOptions は ListWatchings リクエストのオプション。
// Backlog API: GET /api/v2/users/{userId}/watchings
type ListWatchingsOptions struct {
	Order                    string // "asc" or "desc"（空文字 = API デフォルト）
	Sort                     string // "created", "updated", "issueUpdated"
	Count                    int    // 取得件数（最大100）
	Offset                   int
	ResourceAlreadyRead      *bool  // true=既読のみ、false=未読のみ、nil=全件
	IssueID                  int    // 特定課題 ID でフィルタ（0=フィルタなし）
}

// ListWikisOptions は ListWikis リクエストのオプション。
// Backlog API: GET /api/v2/wikis
type ListWikisOptions struct {
	Keyword string // キーワード検索
}

// ListWikiHistoryOptions は GetWikiHistory リクエストのオプション。
// Backlog API: GET /api/v2/wikis/{wikiId}/history
type ListWikiHistoryOptions struct {
	MinID int    // この ID 以上の履歴を取得（0 = 制限なし）
	MaxID int    // この ID 以下の履歴を取得（0 = 制限なし）
	Count int    // 取得件数（1-100、デフォルト 20）
	Order string // "asc" or "desc"（空文字 = API デフォルト desc）
}
