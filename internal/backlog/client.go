package backlog

import (
	"context"

	"github.com/youyo/logvalet/internal/domain"
)

// Client は Backlog API クライアントのインターフェース。
// spec §18.1 準拠。
//
// 全メソッドは context.Context を受け取り、タイムアウト・キャンセルに対応する。
// エラーは typed errors (ErrNotFound, ErrUnauthorized 等) を返す。
type Client interface {
	// Auth / user identity

	// GetMyself は認証済みユーザー情報を返す。
	// Backlog API: GET /api/v2/users/myself
	GetMyself(ctx context.Context) (*domain.User, error)

	// ListUsers はスペースの全ユーザー一覧を返す。
	// Backlog API: GET /api/v2/users
	ListUsers(ctx context.Context) ([]domain.User, error)

	// GetUser は指定 userID のユーザー情報を返す。
	// Backlog API: GET /api/v2/users/{userID}
	GetUser(ctx context.Context, userID string) (*domain.User, error)

	// ListUserActivities は指定ユーザーのアクティビティ一覧を返す。
	// Backlog API: GET /api/v2/users/{userID}/activities
	ListUserActivities(ctx context.Context, userID string, opt ListUserActivitiesOptions) ([]domain.Activity, error)

	// Issues

	// GetIssue は指定課題キーの課題情報を返す。
	// Backlog API: GET /api/v2/issues/{issueKey}
	GetIssue(ctx context.Context, issueKey string) (*domain.Issue, error)

	// ListIssues は課題一覧を返す。
	// Backlog API: GET /api/v2/issues
	ListIssues(ctx context.Context, opt ListIssuesOptions) ([]domain.Issue, error)

	// CreateIssue は新しい課題を作成する。
	// Backlog API: POST /api/v2/issues
	CreateIssue(ctx context.Context, req CreateIssueRequest) (*domain.Issue, error)

	// UpdateIssue は既存の課題を更新する。
	// Backlog API: PATCH /api/v2/issues/{issueKey}
	UpdateIssue(ctx context.Context, issueKey string, req UpdateIssueRequest) (*domain.Issue, error)

	// Issue comments

	// ListIssueComments は指定課題のコメント一覧を返す。
	// Backlog API: GET /api/v2/issues/{issueKey}/comments
	ListIssueComments(ctx context.Context, issueKey string, opt ListCommentsOptions) ([]domain.Comment, error)

	// AddIssueComment は指定課題にコメントを追加する。
	// Backlog API: POST /api/v2/issues/{issueKey}/comments
	AddIssueComment(ctx context.Context, issueKey string, req AddCommentRequest) (*domain.Comment, error)

	// UpdateIssueComment は指定課題の指定コメントを更新する。
	// Backlog API: PATCH /api/v2/issues/{issueKey}/comments/{commentID}
	UpdateIssueComment(ctx context.Context, issueKey string, commentID int64, req UpdateCommentRequest) (*domain.Comment, error)

	// Projects

	// GetProject は指定プロジェクトキーのプロジェクト情報を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}
	GetProject(ctx context.Context, projectKey string) (*domain.Project, error)

	// ListProjects はスペースの全プロジェクト一覧を返す。
	// Backlog API: GET /api/v2/projects
	ListProjects(ctx context.Context) ([]domain.Project, error)

	// ListProjectActivities は指定プロジェクトのアクティビティ一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/activities
	ListProjectActivities(ctx context.Context, projectKey string, opt ListActivitiesOptions) ([]domain.Activity, error)

	// Space activities

	// ListSpaceActivities はスペースのアクティビティ一覧を返す。
	// Backlog API: GET /api/v2/space/activities
	ListSpaceActivities(ctx context.Context, opt ListActivitiesOptions) ([]domain.Activity, error)

	// Documents

	// GetDocument は指定ドキュメントIDのドキュメントを返す。
	// Backlog API: GET /api/v2/documents/{documentID}
	GetDocument(ctx context.Context, documentID string) (*domain.Document, error)

	// ListDocuments は指定プロジェクトのドキュメント一覧を返す。
	// Backlog API: GET /api/v2/documents?projectId[]={id}&offset=N
	ListDocuments(ctx context.Context, projectID int, opt ListDocumentsOptions) ([]domain.Document, error)

	// GetDocumentTree は指定プロジェクトのドキュメントツリーを返す。
	// Backlog API: GET /api/v2/documents/tree?projectIdOrKey={key}
	GetDocumentTree(ctx context.Context, projectKey string) (*domain.DocumentTree, error)

	// CreateDocument は新しいドキュメントを作成する。
	// Backlog API: POST /api/v2/documents
	CreateDocument(ctx context.Context, req CreateDocumentRequest) (*domain.Document, error)

	// ListDocumentAttachments は指定ドキュメントの添付ファイル一覧を返す。
	// Backlog API: GET /api/v2/documents/{documentID}/attachments
	ListDocumentAttachments(ctx context.Context, documentID string) ([]domain.Attachment, error)

	// Project meta

	// ListProjectStatuses は指定プロジェクトのステータス一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/statuses
	ListProjectStatuses(ctx context.Context, projectKey string) ([]domain.Status, error)

	// ListProjectCategories は指定プロジェクトのカテゴリ一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/categories
	ListProjectCategories(ctx context.Context, projectKey string) ([]domain.Category, error)

	// ListProjectVersions は指定プロジェクトのバージョン一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/versions
	ListProjectVersions(ctx context.Context, projectKey string) ([]domain.Version, error)

	// ListProjectCustomFields は指定プロジェクトのカスタムフィールド定義一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/customFields
	ListProjectCustomFields(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error)

	// ListProjectIssueTypes は指定プロジェクトの課題種別一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/issueTypes
	ListProjectIssueTypes(ctx context.Context, projectKey string) ([]domain.IDName, error)

	// ListPriorities は優先度一覧を返す。
	// Backlog API: GET /api/v2/priorities
	ListPriorities(ctx context.Context) ([]domain.IDName, error)

	// Teams

	// ListTeams はスペースのチーム一覧を返す。
	// Backlog API: GET /api/v2/teams
	ListTeams(ctx context.Context) ([]domain.Team, error)

	// ListProjectTeams は指定プロジェクトのチーム一覧を返す。
	// Backlog API: GET /api/v2/projects/{projectKey}/teams
	ListProjectTeams(ctx context.Context, projectKey string) ([]domain.Team, error)

	// Space

	// GetSpace はスペース情報を返す。
	// Backlog API: GET /api/v2/space
	GetSpace(ctx context.Context) (*domain.Space, error)

	// GetSpaceDiskUsage はスペースのディスク使用量を返す。
	// Backlog API: GET /api/v2/space/diskUsage
	GetSpaceDiskUsage(ctx context.Context) (*domain.DiskUsage, error)
}
