package backlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"strings"

	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/domain"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "logvalet/1.0"
)

// ClientConfig は HTTPClient の設定。
type ClientConfig struct {
	// BaseURL は Backlog スペースの URL (例: "https://example.backlog.com")
	BaseURL string
	// Credential は認証情報 (M03 credentials.ResolvedCredential)
	Credential *credentials.ResolvedCredential
	// HTTPClient は内部で使用する *http.Client。nil の場合はデフォルト (タイムアウト 30s)
	HTTPClient *http.Client
	// UserAgent は User-Agent ヘッダの値。空文字はデフォルト値を使用。
	UserAgent string
}

// HTTPClient は Client interface の標準実装。
// Backlog REST API v2 を呼び出す。
type HTTPClient struct {
	baseURL    string
	cred       *credentials.ResolvedCredential
	httpClient *http.Client
	userAgent  string
}

// NewHTTPClient は ClientConfig から HTTPClient を生成する。
func NewHTTPClient(cfg ClientConfig) *HTTPClient {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultTimeout,
		}
	}
	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &HTTPClient{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		cred:       cfg.Credential,
		httpClient: httpClient,
		userAgent:  userAgent,
	}
}

// ---- リクエストヘルパー ----

// newRequest は認証情報付きの *http.Request を生成する。
func (c *HTTPClient) newRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("backlog: invalid URL %q: %w", c.baseURL+path, err)
	}

	if query == nil {
		query = url.Values{}
	}

	// API key 認証: クエリパラメータ apiKey を付与
	if c.cred != nil && c.cred.AuthType == credentials.AuthTypeAPIKey && c.cred.APIKey != "" {
		query.Set("apiKey", c.cred.APIKey)
	}

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("backlog: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	// OAuth 認証: Authorization: Bearer ヘッダを設定
	if c.cred != nil && c.cred.AuthType == credentials.AuthTypeOAuth && c.cred.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cred.AccessToken)
	}

	return req, nil
}

// backlogAPIError は Backlog API のエラーレスポンス構造体。
type backlogAPIError struct {
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors"`
}

// do はリクエストを実行し、レスポンスボディを out にデシリアライズする。
// HTTP エラーは typed errors に変換する。
func (c *HTTPClient) do(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("backlog: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("backlog: failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return c.normalizeError(resp.StatusCode, body)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("backlog: failed to parse response: %w", err)
		}
	}
	return nil
}

// normalizeError は HTTP ステータスコードを typed errors に変換する。
func (c *HTTPClient) normalizeError(statusCode int, body []byte) error {
	// レスポンスボディからエラーメッセージを取得（失敗しても無視）
	var apiErr backlogAPIError
	var code string
	var message string
	if err := json.Unmarshal(body, &apiErr); err == nil && len(apiErr.Errors) > 0 {
		message = apiErr.Errors[0].Message
		code = strconv.Itoa(apiErr.Errors[0].Code)
	}

	var sentinel error
	switch statusCode {
	case http.StatusNotFound:
		sentinel = ErrNotFound
	case http.StatusUnauthorized:
		sentinel = ErrUnauthorized
	case http.StatusForbidden:
		sentinel = ErrForbidden
	case http.StatusUnprocessableEntity:
		sentinel = ErrValidation
	case http.StatusTooManyRequests:
		sentinel = ErrRateLimited
	default:
		sentinel = ErrAPI
	}

	return &BacklogError{
		Err:        sentinel,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// ---- Client interface 実装 ----

// GetMyself は認証済みユーザー情報を返す。
// GET /api/v2/users/myself
func (c *HTTPClient) GetMyself(ctx context.Context) (*domain.User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/users/myself", nil)
	if err != nil {
		return nil, err
	}
	var user domain.User
	if err := c.do(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers はスペースの全ユーザー一覧を返す。
// GET /api/v2/users
func (c *HTTPClient) ListUsers(ctx context.Context) ([]domain.User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/users", nil)
	if err != nil {
		return nil, err
	}
	var users []domain.User
	if err := c.do(req, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetUser は指定 userID のユーザー情報を返す。
// GET /api/v2/users/{userID}
func (c *HTTPClient) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/users/"+url.PathEscape(userID), nil)
	if err != nil {
		return nil, err
	}
	var user domain.User
	if err := c.do(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUserActivities は指定ユーザーのアクティビティ一覧を返す。
// GET /api/v2/users/{userID}/activities
func (c *HTTPClient) ListUserActivities(ctx context.Context, userID string, opt ListUserActivitiesOptions) ([]domain.Activity, error) {
	q := url.Values{}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}
	if opt.Since != nil {
		q.Set("since", opt.Since.Format(time.RFC3339))
	}
	if opt.Until != nil {
		q.Set("until", opt.Until.Format(time.RFC3339))
	}
	for _, t := range opt.Types {
		q.Add("activityTypeId[]", t)
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/users/"+url.PathEscape(userID)+"/activities", q)
	if err != nil {
		return nil, err
	}
	var activities []domain.Activity
	if err := c.do(req, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

// GetIssue は指定課題キーの課題情報を返す。
// GET /api/v2/issues/{issueKey}
func (c *HTTPClient) GetIssue(ctx context.Context, issueKey string) (*domain.Issue, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/issues/"+url.PathEscape(issueKey), nil)
	if err != nil {
		return nil, err
	}
	var issue domain.Issue
	if err := c.do(req, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// ListIssues は課題一覧を返す。
// GET /api/v2/issues
func (c *HTTPClient) ListIssues(ctx context.Context, opt ListIssuesOptions) ([]domain.Issue, error) {
	q := url.Values{}
	if opt.ProjectKey != "" {
		q.Set("projectKey[]", opt.ProjectKey)
	}
	if opt.Assignee != "" {
		q.Set("assigneeUserId[]", opt.Assignee)
	}
	if opt.Status != "" {
		q.Set("statusId[]", opt.Status)
	}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/issues", q)
	if err != nil {
		return nil, err
	}
	var issues []domain.Issue
	if err := c.do(req, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

// CreateIssue は新しい課題を作成する。
// POST /api/v2/issues
func (c *HTTPClient) CreateIssue(ctx context.Context, reqBody CreateIssueRequest) (*domain.Issue, error) {
	q := url.Values{}
	q.Set("projectKey", reqBody.ProjectKey)
	q.Set("summary", reqBody.Summary)
	q.Set("issueTypeId", reqBody.IssueType)
	if reqBody.Description != "" {
		q.Set("description", reqBody.Description)
	}
	if reqBody.Priority != "" {
		q.Set("priorityId", reqBody.Priority)
	}
	if reqBody.Assignee != "" {
		q.Set("assigneeUserId", reqBody.Assignee)
	}

	// POST なので query param ではなく form body が本来望ましいが、
	// M04 では最小限の実装として URL に設定する（M06 以降で整備）
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v2/issues", q)
	if err != nil {
		return nil, err
	}
	var issue domain.Issue
	if err := c.do(req, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// UpdateIssue は既存の課題を更新する。
// PATCH /api/v2/issues/{issueKey}
func (c *HTTPClient) UpdateIssue(ctx context.Context, issueKey string, reqBody UpdateIssueRequest) (*domain.Issue, error) {
	q := url.Values{}
	if reqBody.Summary != nil {
		q.Set("summary", *reqBody.Summary)
	}
	if reqBody.Description != nil {
		q.Set("description", *reqBody.Description)
	}
	if reqBody.Status != nil {
		q.Set("statusId", *reqBody.Status)
	}

	req, err := c.newRequest(ctx, http.MethodPatch, "/api/v2/issues/"+url.PathEscape(issueKey), q)
	if err != nil {
		return nil, err
	}
	var issue domain.Issue
	if err := c.do(req, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// ListIssueComments は指定課題のコメント一覧を返す。
// GET /api/v2/issues/{issueKey}/comments
func (c *HTTPClient) ListIssueComments(ctx context.Context, issueKey string, opt ListCommentsOptions) ([]domain.Comment, error) {
	q := url.Values{}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/issues/"+url.PathEscape(issueKey)+"/comments", q)
	if err != nil {
		return nil, err
	}
	var comments []domain.Comment
	if err := c.do(req, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// AddIssueComment は指定課題にコメントを追加する。
// POST /api/v2/issues/{issueKey}/comments
func (c *HTTPClient) AddIssueComment(ctx context.Context, issueKey string, reqBody AddCommentRequest) (*domain.Comment, error) {
	q := url.Values{}
	q.Set("content", reqBody.Content)
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v2/issues/"+url.PathEscape(issueKey)+"/comments", q)
	if err != nil {
		return nil, err
	}
	var comment domain.Comment
	if err := c.do(req, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// UpdateIssueComment は指定課題の指定コメントを更新する。
// PATCH /api/v2/issues/{issueKey}/comments/{commentID}
func (c *HTTPClient) UpdateIssueComment(ctx context.Context, issueKey string, commentID int64, reqBody UpdateCommentRequest) (*domain.Comment, error) {
	q := url.Values{}
	q.Set("content", reqBody.Content)
	path := fmt.Sprintf("/api/v2/issues/%s/comments/%d", url.PathEscape(issueKey), commentID)
	req, err := c.newRequest(ctx, http.MethodPatch, path, q)
	if err != nil {
		return nil, err
	}
	var comment domain.Comment
	if err := c.do(req, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// GetProject は指定プロジェクトキーのプロジェクト情報を返す。
// GET /api/v2/projects/{projectKey}
func (c *HTTPClient) GetProject(ctx context.Context, projectKey string) (*domain.Project, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey), nil)
	if err != nil {
		return nil, err
	}
	var project domain.Project
	if err := c.do(req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ListProjects はスペースの全プロジェクト一覧を返す。
// GET /api/v2/projects
func (c *HTTPClient) ListProjects(ctx context.Context) ([]domain.Project, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects", nil)
	if err != nil {
		return nil, err
	}
	var projects []domain.Project
	if err := c.do(req, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// ListProjectActivities は指定プロジェクトのアクティビティ一覧を返す。
// GET /api/v2/projects/{projectKey}/activities
func (c *HTTPClient) ListProjectActivities(ctx context.Context, projectKey string, opt ListActivitiesOptions) ([]domain.Activity, error) {
	q := url.Values{}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}
	if opt.Since != nil {
		q.Set("since", opt.Since.Format(time.RFC3339))
	}
	if opt.Until != nil {
		q.Set("until", opt.Until.Format(time.RFC3339))
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/activities", q)
	if err != nil {
		return nil, err
	}
	var activities []domain.Activity
	if err := c.do(req, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

// ListSpaceActivities はスペースのアクティビティ一覧を返す。
// GET /api/v2/space/activities
func (c *HTTPClient) ListSpaceActivities(ctx context.Context, opt ListActivitiesOptions) ([]domain.Activity, error) {
	q := url.Values{}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}
	if opt.Since != nil {
		q.Set("since", opt.Since.Format(time.RFC3339))
	}
	if opt.Until != nil {
		q.Set("until", opt.Until.Format(time.RFC3339))
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/space/activities", q)
	if err != nil {
		return nil, err
	}
	var activities []domain.Activity
	if err := c.do(req, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

// GetDocument は指定ドキュメントIDのドキュメントを返す。
// GET /api/v2/documents/{documentID}
func (c *HTTPClient) GetDocument(ctx context.Context, documentID int64) (*domain.Document, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v2/documents/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	var doc domain.Document
	if err := c.do(req, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListDocuments は指定プロジェクトのドキュメント一覧を返す。
// GET /api/v2/projects/{projectKey}/documents
func (c *HTTPClient) ListDocuments(ctx context.Context, projectKey string, opt ListDocumentsOptions) ([]domain.Document, error) {
	q := url.Values{}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/documents", q)
	if err != nil {
		return nil, err
	}
	var docs []domain.Document
	if err := c.do(req, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// GetDocumentTree は指定プロジェクトのドキュメントツリーを返す。
// GET /api/v2/projects/{projectKey}/documents/tree
func (c *HTTPClient) GetDocumentTree(ctx context.Context, projectKey string) ([]domain.DocumentNode, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/documents/tree", nil)
	if err != nil {
		return nil, err
	}
	var tree []domain.DocumentNode
	if err := c.do(req, &tree); err != nil {
		return nil, err
	}
	return tree, nil
}

// CreateDocument は新しいドキュメントを作成する。
// POST /api/v2/documents
func (c *HTTPClient) CreateDocument(ctx context.Context, reqBody CreateDocumentRequest) (*domain.Document, error) {
	q := url.Values{}
	q.Set("projectKey", reqBody.ProjectKey)
	q.Set("name", reqBody.Title)
	q.Set("content", reqBody.Content)
	if reqBody.ParentID != nil {
		q.Set("parentDocumentId", strconv.FormatInt(*reqBody.ParentID, 10))
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v2/documents", q)
	if err != nil {
		return nil, err
	}
	var doc domain.Document
	if err := c.do(req, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListDocumentAttachments は指定ドキュメントの添付ファイル一覧を返す。
// GET /api/v2/documents/{documentID}/attachments
func (c *HTTPClient) ListDocumentAttachments(ctx context.Context, documentID int64) ([]domain.Attachment, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v2/documents/%d/attachments", documentID), nil)
	if err != nil {
		return nil, err
	}
	var attachments []domain.Attachment
	if err := c.do(req, &attachments); err != nil {
		return nil, err
	}
	return attachments, nil
}

// ListProjectStatuses は指定プロジェクトのステータス一覧を返す。
// GET /api/v2/projects/{projectKey}/statuses
func (c *HTTPClient) ListProjectStatuses(ctx context.Context, projectKey string) ([]domain.Status, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/statuses", nil)
	if err != nil {
		return nil, err
	}
	var statuses []domain.Status
	if err := c.do(req, &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

// ListProjectCategories は指定プロジェクトのカテゴリ一覧を返す。
// GET /api/v2/projects/{projectKey}/categories
func (c *HTTPClient) ListProjectCategories(ctx context.Context, projectKey string) ([]domain.Category, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/categories", nil)
	if err != nil {
		return nil, err
	}
	var categories []domain.Category
	if err := c.do(req, &categories); err != nil {
		return nil, err
	}
	return categories, nil
}

// ListProjectVersions は指定プロジェクトのバージョン一覧を返す。
// GET /api/v2/projects/{projectKey}/versions
func (c *HTTPClient) ListProjectVersions(ctx context.Context, projectKey string) ([]domain.Version, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/versions", nil)
	if err != nil {
		return nil, err
	}
	var versions []domain.Version
	if err := c.do(req, &versions); err != nil {
		return nil, err
	}
	return versions, nil
}

// ListProjectCustomFields は指定プロジェクトのカスタムフィールド定義一覧を返す。
// GET /api/v2/projects/{projectKey}/customFields
func (c *HTTPClient) ListProjectCustomFields(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/customFields", nil)
	if err != nil {
		return nil, err
	}
	var fields []domain.CustomFieldDefinition
	if err := c.do(req, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// ListTeams はスペースのチーム一覧を返す。
// GET /api/v2/teams
func (c *HTTPClient) ListTeams(ctx context.Context) ([]domain.Team, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/teams", nil)
	if err != nil {
		return nil, err
	}
	var teams []domain.Team
	if err := c.do(req, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// ListProjectTeams は指定プロジェクトのチーム一覧を返す。
// GET /api/v2/projects/{projectKey}/teams
func (c *HTTPClient) ListProjectTeams(ctx context.Context, projectKey string) ([]domain.Team, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/teams", nil)
	if err != nil {
		return nil, err
	}
	var teams []domain.Team
	if err := c.do(req, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// GetSpace はスペース情報を返す。
// GET /api/v2/space
func (c *HTTPClient) GetSpace(ctx context.Context) (*domain.Space, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/space", nil)
	if err != nil {
		return nil, err
	}
	var space domain.Space
	if err := c.do(req, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// GetSpaceDiskUsage はスペースのディスク使用量を返す。
// GET /api/v2/space/diskUsage
func (c *HTTPClient) GetSpaceDiskUsage(ctx context.Context) (*domain.DiskUsage, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/space/diskUsage", nil)
	if err != nil {
		return nil, err
	}
	var usage domain.DiskUsage
	if err := c.do(req, &usage); err != nil {
		return nil, err
	}
	return &usage, nil
}
