// Package domain は logvalet のドメインモデルを定義する。
package domain

import (
	"encoding/json"
	"time"
)

// NulabAccount は Nulab アカウント情報（spec §11）。
type NulabAccount struct {
	NulabID string `json:"nulabId"`
}

// User は Backlog のユーザーモデル。
// Backlog API は camelCase を使用する（userId, mailAddress 等）。
type User struct {
	ID           int           `json:"id"`
	UserID       string        `json:"userId"`
	Name         string        `json:"name"`
	MailAddress  string        `json:"mailAddress,omitempty"`
	RoleType     int           `json:"roleType,omitempty"`
	NulabAccount *NulabAccount `json:"nulabAccount,omitempty"`
}

// UserRef はダイジェスト内の簡略ユーザー参照（spec §11 simplified form）。
// assignee, reporter, author, actor, created_user, updated_user で使用する。
type UserRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Issue は Backlog の課題モデル。
// JSON タグは Backlog API レスポンス（camelCase）に準拠。
type Issue struct {
	ID           int           `json:"id"`
	ProjectID    int           `json:"projectId"`
	IssueKey     string        `json:"issueKey"`
	Summary      string        `json:"summary"`
	Description  string        `json:"description"`
	Status       *IDName       `json:"status,omitempty"`
	Priority     *IDName       `json:"priority,omitempty"`
	IssueType    *IDName       `json:"issueType,omitempty"`
	Assignee     *User         `json:"assignee,omitempty"`
	Reporter     *User         `json:"createdUser,omitempty"`
	Categories   []IDName      `json:"category"`
	Versions     []IDName      `json:"versions"`
	Milestones   []IDName      `json:"milestone"`
	DueDate      *time.Time    `json:"dueDate"`
	StartDate    *time.Time    `json:"startDate"`
	Created      *time.Time    `json:"created,omitempty"`
	Updated      *time.Time    `json:"updated,omitempty"`
	CustomFields []CustomField `json:"customFields,omitempty"`
}

// Comment は Backlog のコメントモデル。
type Comment struct {
	ID          int64      `json:"id"`
	Content     string     `json:"content"`
	CreatedUser *User      `json:"createdUser,omitempty"`
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

// Activity は Backlog の生アクティビティモデル（API レスポンス用）。
// 正規化された形式は NormalizedActivity を使用する。
type Activity struct {
	ID          int64                  `json:"id"`
	Type        int                    `json:"type"`
	Created     *time.Time             `json:"created,omitempty"`
	CreatedUser *User                  `json:"createdUser,omitempty"`
	Content     map[string]interface{} `json:"content,omitempty"`
}

// ActivityIssueRef はアクティビティ内の課題参照（簡略形）。
type ActivityIssueRef struct {
	ID      int    `json:"id"`
	Key     string `json:"key"`
	Summary string `json:"summary,omitempty"`
}

// ActivityCommentRef はアクティビティ内のコメント参照（簡略形）。
type ActivityCommentRef struct {
	ID      int64  `json:"id"`
	Content string `json:"content,omitempty"`
}

// NormalizedActivity は spec §12 に準拠した正規化アクティビティ。
// "type" フィールドは Backlog の type 番号を文字列名に変換したもの（例: "issue_commented"）。
type NormalizedActivity struct {
	ID      int64               `json:"id"`
	Type    string              `json:"type"`
	Created *time.Time          `json:"created,omitempty"`
	Actor   *UserRef            `json:"actor,omitempty"`
	Issue   *ActivityIssueRef   `json:"issue,omitempty"`
	Comment *ActivityCommentRef `json:"comment,omitempty"`
}

// Document は Backlog のドキュメントモデル。
type Document struct {
	ID          string       `json:"id"`
	ProjectID   int          `json:"projectId"`
	Title       string       `json:"title"`
	Plain       string           `json:"plain,omitempty"`
	JSON        json.RawMessage  `json:"json,omitempty"`
	StatusID    int          `json:"statusId,omitempty"`
	Emoji       string       `json:"emoji,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Tags        []Tag        `json:"tags,omitempty"`
	CreatedUser *User        `json:"createdUser,omitempty"`
	Created     *time.Time   `json:"created,omitempty"`
	UpdatedUser *User        `json:"updatedUser,omitempty"`
	Updated     *time.Time   `json:"updated,omitempty"`
}

// Tag はドキュメントのタグ。
type Tag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// DocumentTree はドキュメントツリーの API レスポンス。
type DocumentTree struct {
	ProjectID  int          `json:"projectId"`
	ActiveTree DocumentNode `json:"activeTree"`
	TrashTree  DocumentNode `json:"trashTree"`
}

// DocumentNode はドキュメントツリーのノード。
type DocumentNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Emoji    string         `json:"emoji,omitempty"`
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
	ID             int        `json:"id"`
	ProjectID      int        `json:"projectId"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	StartDate      *time.Time `json:"startDate,omitempty"`
	ReleaseDueDate *time.Time `json:"releaseDueDate,omitempty"`
	Archived       bool       `json:"archived"`
}

// CustomFieldDefinition はカスタムフィールド定義。
type CustomFieldDefinition struct {
	ID          int    `json:"id"`
	TypeID      int    `json:"typeId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
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

// TeamWithMembers はメンバー一覧を含むチーム情報。
// Backlog API: GET /api/v2/teams/{teamId} のレスポンスに対応する。
// 既存の Team 型（ID と Name のみ）はリスト取得等で引き続き使用する。
type TeamWithMembers struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	Members      []User     `json:"members"`
	DisplayOrder int        `json:"displayOrder,omitempty"`
	Created      *time.Time `json:"created,omitempty"`
	Updated      *time.Time `json:"updated,omitempty"`
}

// Space は Backlog スペース情報。
type Space struct {
	SpaceKey           string `json:"spaceKey"`
	Name               string `json:"name"`
	OwnerID            int    `json:"ownerId"`
	Lang               string `json:"lang"`
	Timezone           string `json:"timezone"`
	ReportSendTime     string `json:"reportSendTime"`
	TextFormattingRule string `json:"textFormattingRule"`
}

// DiskUsage はスペースのディスク使用量情報。
type DiskUsage struct {
	Capacity   int64 `json:"capacity"`
	Issue      int64 `json:"issue"`
	Wiki       int64 `json:"wiki"`
	File       int64 `json:"file"`
	Subversion int64 `json:"subversion"`
	Git        int64 `json:"git"`
	GitLFS     int64 `json:"gitLFS"`
}

// IDName は ID と名前のペア（ステータス、優先度、課題種別等で使用）。
type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Warning はパーシャルサクセス時の警告（spec §9 warning envelope）。
type Warning struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Component string `json:"component,omitempty"`
	Retryable bool   `json:"retryable"`
}

// ErrorDetail はエラーの詳細情報（spec §9 error envelope 内部）。
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// ErrorEnvelope は完全失敗時のエラーエンベロープ（spec §9）。
// stdout に出力する。
type ErrorEnvelope struct {
	SchemaVersion string      `json:"schema_version"`
	Error         ErrorDetail `json:"error"`
}

// DigestEnvelope はすべての digest コマンドの共通ラッパー（spec §10）。
// Digest フィールドには各リソース固有の digest 構造体を格納する。
type DigestEnvelope struct {
	SchemaVersion string      `json:"schema_version"`
	Resource      string      `json:"resource"`
	GeneratedAt   time.Time   `json:"generated_at"`
	Profile       string      `json:"profile"`
	Space         string      `json:"space"`
	BaseURL       string      `json:"base_url"`
	Warnings      []Warning   `json:"warnings"`
	Digest        interface{} `json:"digest"`
}
