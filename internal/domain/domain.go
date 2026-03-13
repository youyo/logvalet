// Package domain は logvalet のドメインモデルを定義する。
//
// M04 で最小限の型のみを先行定義する。
// M05（Domain models & full rendering）で拡張予定。
package domain

import "time"

// User は Backlog のユーザーモデル。
// Backlog API は camelCase を使用する（userId, mailAddress 等）。
type User struct {
	ID          int    `json:"id"`
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	MailAddress string `json:"mailAddress,omitempty"`
	RoleType    int    `json:"roleType,omitempty"`
}

// Issue は Backlog の課題モデル。
type Issue struct {
	ID          int        `json:"id"`
	ProjectID   int        `json:"project_id"`
	IssueKey    string     `json:"issueKey"`
	Summary     string     `json:"summary"`
	Description string     `json:"description"`
	Status      *IDName    `json:"status,omitempty"`
	Priority    *IDName    `json:"priority,omitempty"`
	IssueType   *IDName    `json:"issue_type,omitempty"`
	Assignee    *User      `json:"assignee,omitempty"`
	Reporter    *User      `json:"created_user,omitempty"`
	Categories  []IDName   `json:"category"`
	Versions    []IDName   `json:"versions"`
	Milestones  []IDName   `json:"milestone"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	Created     *time.Time `json:"created,omitempty"`
	Updated     *time.Time `json:"updated,omitempty"`
	CustomFields []CustomField `json:"customFields,omitempty"`
}

// Comment は Backlog のコメントモデル。
type Comment struct {
	ID          int64      `json:"id"`
	Content     string     `json:"content"`
	CreatedUser *User      `json:"created_user,omitempty"`
	Created     *time.Time `json:"created,omitempty"`
	Updated     *time.Time `json:"updated,omitempty"`
}

// Project は Backlog のプロジェクトモデル。
type Project struct {
	ID         int    `json:"id"`
	ProjectKey string `json:"projectKey"`
	Name       string `json:"name"`
	Archived   bool   `json:"archived"`
}

// Activity は Backlog のアクティビティモデル。
type Activity struct {
	ID      int64      `json:"id"`
	Type    int        `json:"type"`
	Created *time.Time `json:"created,omitempty"`
	CreatedUser *User  `json:"createdUser,omitempty"`
	Content map[string]interface{} `json:"content,omitempty"`
}

// Document は Backlog のドキュメントモデル。
type Document struct {
	ID        int64      `json:"id"`
	ProjectID int        `json:"project_id"`
	Title     string     `json:"title"`
	Content   string     `json:"content,omitempty"`
	Created   *time.Time `json:"created,omitempty"`
	Updated   *time.Time `json:"updated,omitempty"`
	CreatedUser *User    `json:"created_user,omitempty"`
}

// DocumentNode はドキュメントツリーのノード。
type DocumentNode struct {
	ID       int64          `json:"id"`
	Title    string         `json:"title"`
	Children []DocumentNode `json:"children,omitempty"`
}

// Attachment はドキュメントの添付ファイル。
type Attachment struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Status はステータス情報。
type Status struct {
	ID           int    `json:"id"`
	ProjectID    int    `json:"projectId"`
	Name         string `json:"name"`
	Color        string `json:"color,omitempty"`
	DisplayOrder int    `json:"displayOrder"`
}

// Category はカテゴリ情報。
type Category struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"displayOrder"`
}

// Version はバージョン情報。
type Version struct {
	ID           int        `json:"id"`
	ProjectID    int        `json:"projectId"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	StartDate    *time.Time `json:"startDate,omitempty"`
	ReleaseDueDate *time.Time `json:"releaseDueDate,omitempty"`
	Archived     bool       `json:"archived"`
}

// CustomFieldDefinition はカスタムフィールド定義。
type CustomFieldDefinition struct {
	ID           int    `json:"id"`
	TypeID       int    `json:"typeId"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Required     bool   `json:"required"`
}

// CustomField はカスタムフィールドの値。
type CustomField struct {
	ID    int         `json:"id"`
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// Team はチーム情報。
type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Space は Backlog スペース情報。
type Space struct {
	SpaceKey      string `json:"spaceKey"`
	Name          string `json:"name"`
	OwnerID       int    `json:"ownerId"`
	Lang          string `json:"lang"`
	Timezone      string `json:"timezone"`
	ReportSendTime string `json:"reportSendTime"`
	TextFormattingRule string `json:"textFormattingRule"`
}

// DiskUsage はスペースのディスク使用量情報。
type DiskUsage struct {
	Capacity  int64 `json:"capacity"`
	Issue     int64 `json:"issue"`
	Wiki      int64 `json:"wiki"`
	File      int64 `json:"file"`
	Subversion int64 `json:"subversion"`
	Git       int64 `json:"git"`
	GitLFS    int64 `json:"gitLFS"`
}

// IDName は ID と名前のペア（ステータス、優先度、課題種別等で使用）。
type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
