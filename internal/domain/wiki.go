package domain

import "time"

// WikiTag は Wiki ページのタグ。
// Backlog API: GET /api/v2/wikis/tags
type WikiTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// WikiPage は Backlog の Wiki ページモデル。
// Backlog API: GET /api/v2/wikis/{wikiId}
type WikiPage struct {
	ID          int64          `json:"id"`
	ProjectID   int            `json:"projectId"`
	Name        string         `json:"name"`
	Content     string         `json:"content"`
	Tags        []WikiTag      `json:"tag"`
	Attachments []Attachment   `json:"attachments,omitempty"`
	SharedFiles []SharedFile   `json:"sharedFiles,omitempty"`
	Stars       []WikiStar     `json:"stars,omitempty"`
	CreatedUser *User          `json:"createdUser,omitempty"`
	Created     *time.Time     `json:"created,omitempty"`
	UpdatedUser *User          `json:"updatedUser,omitempty"`
	Updated     *time.Time     `json:"updated,omitempty"`
}

// WikiHistory は Wiki ページの変更履歴。
// Backlog API: GET /api/v2/wikis/{wikiId}/history
type WikiHistory struct {
	PageID      int64      `json:"pageId"`
	Version     int        `json:"version"`
	Name        string     `json:"name"`
	Content     string     `json:"content"`
	CreatedUser *User      `json:"createdUser,omitempty"`
	Created     *time.Time `json:"created,omitempty"`
}

// WikiStar は Wiki ページのスター情報。
// Backlog API: GET /api/v2/wikis/{wikiId}/stars
type WikiStar struct {
	ID        int64      `json:"id"`
	Comment   string     `json:"comment,omitempty"`
	URL       string     `json:"url"`
	Title     string     `json:"title"`
	Presenter *User      `json:"presenter,omitempty"`
	Created   *time.Time `json:"created,omitempty"`
}

// WikiCount は Wiki ページ件数レスポンス。
// Backlog API: GET /api/v2/wikis/count
type WikiCount struct {
	Count int `json:"count"`
}
