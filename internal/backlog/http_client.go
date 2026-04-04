package backlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

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
	for _, id := range opt.ActivityTypeIDs {
		q.Add("activityTypeId[]", strconv.Itoa(id))
	}
	if opt.MinId > 0 {
		q.Set("minId", strconv.Itoa(opt.MinId))
	}
	if opt.MaxId > 0 {
		q.Set("maxId", strconv.Itoa(opt.MaxId))
	}
	if opt.Count > 0 {
		q.Set("count", strconv.Itoa(opt.Count))
	}
	if opt.Order != "" {
		q.Set("order", opt.Order)
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
	for _, id := range opt.ProjectIDs {
		q.Add("projectId[]", strconv.Itoa(id))
	}
	for _, id := range opt.AssigneeIDs {
		q.Add("assigneeId[]", strconv.Itoa(id))
	}
	for _, id := range opt.StatusIDs {
		q.Add("statusId[]", strconv.Itoa(id))
	}
	if opt.DueDateSince != nil {
		q.Set("dueDateSince", opt.DueDateSince.Format("2006-01-02"))
	}
	if opt.DueDateUntil != nil {
		q.Set("dueDateUntil", opt.DueDateUntil.Format("2006-01-02"))
	}
	if opt.StartDateSince != nil {
		q.Set("startDateSince", opt.StartDateSince.Format("2006-01-02"))
	}
	if opt.StartDateUntil != nil {
		q.Set("startDateUntil", opt.StartDateUntil.Format("2006-01-02"))
	}
	if opt.UpdatedSince != nil {
		q.Set("updatedSince", opt.UpdatedSince.Format("2006-01-02"))
	}
	if opt.UpdatedUntil != nil {
		q.Set("updatedUntil", opt.UpdatedUntil.Format("2006-01-02"))
	}
	if opt.Sort != "" {
		q.Set("sort", opt.Sort)
	}
	if opt.Order != "" {
		q.Set("order", opt.Order)
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
	q.Set("projectId", strconv.Itoa(reqBody.ProjectID))
	q.Set("summary", reqBody.Summary)
	q.Set("issueTypeId", strconv.Itoa(reqBody.IssueTypeID))
	q.Set("priorityId", strconv.Itoa(reqBody.PriorityID))
	if reqBody.Description != "" {
		q.Set("description", reqBody.Description)
	}
	if reqBody.AssigneeID > 0 {
		q.Set("assigneeId", strconv.Itoa(reqBody.AssigneeID))
	}
	if reqBody.ParentIssueID > 0 {
		q.Set("parentIssueId", strconv.Itoa(reqBody.ParentIssueID))
	}
	for _, id := range reqBody.CategoryIDs {
		q.Add("categoryId[]", strconv.Itoa(id))
	}
	for _, id := range reqBody.VersionIDs {
		q.Add("versionId[]", strconv.Itoa(id))
	}
	for _, id := range reqBody.MilestoneIDs {
		q.Add("milestoneId[]", strconv.Itoa(id))
	}
	for _, id := range reqBody.NotifiedUserIDs {
		q.Add("notifiedUserId[]", strconv.Itoa(id))
	}
	if reqBody.DueDate != nil {
		q.Set("dueDate", reqBody.DueDate.Format("2006-01-02"))
	}
	if reqBody.StartDate != nil {
		q.Set("startDate", reqBody.StartDate.Format("2006-01-02"))
	}

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
	if reqBody.StatusID != nil {
		q.Set("statusId", strconv.Itoa(*reqBody.StatusID))
	}
	if reqBody.PriorityID != nil {
		q.Set("priorityId", strconv.Itoa(*reqBody.PriorityID))
	}
	if reqBody.AssigneeID != nil {
		q.Set("assigneeId", strconv.Itoa(*reqBody.AssigneeID))
	}
	if reqBody.IssueTypeID != nil {
		q.Set("issueTypeId", strconv.Itoa(*reqBody.IssueTypeID))
	}
	for _, id := range reqBody.CategoryIDs {
		q.Add("categoryId[]", strconv.Itoa(id))
	}
	for _, id := range reqBody.VersionIDs {
		q.Add("versionId[]", strconv.Itoa(id))
	}
	for _, id := range reqBody.MilestoneIDs {
		q.Add("milestoneId[]", strconv.Itoa(id))
	}
	for _, id := range reqBody.NotifiedUserIDs {
		q.Add("notifiedUserId[]", strconv.Itoa(id))
	}
	if reqBody.DueDate != nil {
		q.Set("dueDate", reqBody.DueDate.Format("2006-01-02"))
	}
	if reqBody.StartDate != nil {
		q.Set("startDate", reqBody.StartDate.Format("2006-01-02"))
	}
	if reqBody.Comment != nil {
		q.Set("comment", *reqBody.Comment)
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
	for _, id := range reqBody.NotifiedUserIDs {
		q.Add("notifiedUserId[]", strconv.Itoa(id))
	}
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
	for _, id := range opt.ActivityTypeIDs {
		q.Add("activityTypeId[]", strconv.Itoa(id))
	}
	if opt.MinId > 0 {
		q.Set("minId", strconv.Itoa(opt.MinId))
	}
	if opt.MaxId > 0 {
		q.Set("maxId", strconv.Itoa(opt.MaxId))
	}
	if opt.Count > 0 {
		q.Set("count", strconv.Itoa(opt.Count))
	}
	if opt.Order != "" {
		q.Set("order", opt.Order)
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
	for _, id := range opt.ActivityTypeIDs {
		q.Add("activityTypeId[]", strconv.Itoa(id))
	}
	if opt.MinId > 0 {
		q.Set("minId", strconv.Itoa(opt.MinId))
	}
	if opt.MaxId > 0 {
		q.Set("maxId", strconv.Itoa(opt.MaxId))
	}
	if opt.Count > 0 {
		q.Set("count", strconv.Itoa(opt.Count))
	}
	if opt.Order != "" {
		q.Set("order", opt.Order)
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
func (c *HTTPClient) GetDocument(ctx context.Context, documentID string) (*domain.Document, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/documents/"+url.PathEscape(documentID), nil)
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
// GET /api/v2/documents?projectId[]={id}&offset=N
func (c *HTTPClient) ListDocuments(ctx context.Context, projectID int, opt ListDocumentsOptions) ([]domain.Document, error) {
	q := url.Values{}
	q.Add("projectId[]", strconv.Itoa(projectID))
	q.Set("offset", strconv.Itoa(opt.Offset))
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/documents", q)
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
// GET /api/v2/documents/tree?projectIdOrKey={key}
func (c *HTTPClient) GetDocumentTree(ctx context.Context, projectKey string) (*domain.DocumentTree, error) {
	q := url.Values{}
	q.Set("projectIdOrKey", projectKey)
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/documents/tree", q)
	if err != nil {
		return nil, err
	}
	var tree domain.DocumentTree
	if err := c.do(req, &tree); err != nil {
		return nil, err
	}
	return &tree, nil
}

// CreateDocument は新しいドキュメントを作成する。
// POST /api/v2/documents
func (c *HTTPClient) CreateDocument(ctx context.Context, reqBody CreateDocumentRequest) (*domain.Document, error) {
	q := url.Values{}
	q.Set("projectId", strconv.Itoa(reqBody.ProjectID))
	q.Set("title", reqBody.Title)
	q.Set("content", reqBody.Content)
	if reqBody.ParentID != nil {
		q.Set("parentId", *reqBody.ParentID)
	}
	if reqBody.Emoji != "" {
		q.Set("emoji", reqBody.Emoji)
	}
	if reqBody.AddLast {
		q.Set("addLast", "true")
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
func (c *HTTPClient) ListDocumentAttachments(ctx context.Context, documentID string) ([]domain.Attachment, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/documents/"+url.PathEscape(documentID)+"/attachments", nil)
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

// ListProjectIssueTypes は指定プロジェクトの課題種別一覧を返す。
// GET /api/v2/projects/{projectKey}/issueTypes
func (c *HTTPClient) ListProjectIssueTypes(ctx context.Context, projectKey string) ([]domain.IDName, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects/"+url.PathEscape(projectKey)+"/issueTypes", nil)
	if err != nil {
		return nil, err
	}
	var issueTypes []domain.IDName
	if err := c.do(req, &issueTypes); err != nil {
		return nil, err
	}
	return issueTypes, nil
}

// ListPriorities は優先度一覧を返す。
// GET /api/v2/priorities
func (c *HTTPClient) ListPriorities(ctx context.Context) ([]domain.IDName, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/priorities", nil)
	if err != nil {
		return nil, err
	}
	var priorities []domain.IDName
	if err := c.do(req, &priorities); err != nil {
		return nil, err
	}
	return priorities, nil
}

// ListTeams はスペースのチーム一覧を返す。
// GET /api/v2/teams
// Backlog API は members[] を含むレスポンスを返すため TeamWithMembers にデシリアライズする。
func (c *HTTPClient) ListTeams(ctx context.Context) ([]domain.TeamWithMembers, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/teams", nil)
	if err != nil {
		return nil, err
	}
	var teams []domain.TeamWithMembers
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

// GetTeam は指定チーム ID のチーム情報（メンバー一覧含む）を返す。
// GET /api/v2/teams/{teamId}
func (c *HTTPClient) GetTeam(ctx context.Context, teamID int) (*domain.TeamWithMembers, error) {
	path := fmt.Sprintf("/api/v2/teams/%d", teamID)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var team domain.TeamWithMembers
	if err := c.do(req, &team); err != nil {
		return nil, err
	}
	return &team, nil
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

// ---- ダウンロードヘルパー ----

// doDownload はリクエストを実行し、レスポンス Body を io.ReadCloser としてそのまま返す。
// ファイル名は Content-Disposition ヘッダから取得する。取得できない場合は URL パス末尾を使用する。
func (c *HTTPClient) doDownload(req *http.Request) (io.ReadCloser, string, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("backlog: HTTP request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, "", c.normalizeError(resp.StatusCode, body)
	}

	// ファイル名を Content-Disposition から取得
	filename := filenameFromResponse(resp)

	return resp.Body, filename, nil
}

// filenameFromResponse は HTTP レスポンスからファイル名を取得する。
// Content-Disposition ヘッダを優先し、取得できない場合は URL パス末尾を使用する。
func filenameFromResponse(resp *http.Response) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		_, params, err := mime.ParseMediaType(cd)
		if err == nil {
			if fn, ok := params["filename"]; ok && fn != "" {
				return fn
			}
		}
	}
	// フォールバック: URL パスの末尾
	return path.Base(resp.Request.URL.Path)
}

// ---- Shared files ----

// ListSharedFiles は指定プロジェクトの共有ファイル一覧を返す。
// GET /api/v2/projects/{projectIdOrKey}/files/metadata/{path}
func (c *HTTPClient) ListSharedFiles(ctx context.Context, projectKey string, opt ListSharedFilesOptions) ([]domain.SharedFile, error) {
	p := opt.Path
	p = strings.TrimPrefix(p, "/")
	apiPath := fmt.Sprintf("/api/v2/projects/%s/files/metadata/%s", url.PathEscape(projectKey), p)

	q := url.Values{}
	if opt.Limit > 0 {
		q.Set("count", strconv.Itoa(opt.Limit))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}

	req, err := c.newRequest(ctx, http.MethodGet, apiPath, q)
	if err != nil {
		return nil, err
	}
	var files []domain.SharedFile
	if err := c.do(req, &files); err != nil {
		return nil, err
	}
	return files, nil
}


// DownloadSharedFile は指定共有ファイルのコンテンツを返す。
// GET /api/v2/projects/{projectIdOrKey}/files/{sharedFileId}
func (c *HTTPClient) DownloadSharedFile(ctx context.Context, projectKey string, fileID int64) (io.ReadCloser, string, error) {
	apiPath := fmt.Sprintf("/api/v2/projects/%s/files/%d", url.PathEscape(projectKey), fileID)
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, "", err
	}
	return c.doDownload(req)
}

// ---- Issue attachments ----

// ListIssueAttachments は指定課題の添付ファイル一覧を返す。
// GET /api/v2/issues/{issueIdOrKey}/attachments
func (c *HTTPClient) ListIssueAttachments(ctx context.Context, issueKey string) ([]domain.IssueAttachment, error) {
	apiPath := fmt.Sprintf("/api/v2/issues/%s/attachments", url.PathEscape(issueKey))
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	var attachments []domain.IssueAttachment
	if err := c.do(req, &attachments); err != nil {
		return nil, err
	}
	return attachments, nil
}

// DeleteIssueAttachment は指定課題の添付ファイルを削除し、削除した添付ファイル情報を返す。
// DELETE /api/v2/issues/{issueIdOrKey}/attachments/{attachmentId}
func (c *HTTPClient) DeleteIssueAttachment(ctx context.Context, issueKey string, attachmentID int64) (*domain.IssueAttachment, error) {
	apiPath := fmt.Sprintf("/api/v2/issues/%s/attachments/%d", url.PathEscape(issueKey), attachmentID)
	req, err := c.newRequest(ctx, http.MethodDelete, apiPath, nil)
	if err != nil {
		return nil, err
	}
	var attachment domain.IssueAttachment
	if err := c.do(req, &attachment); err != nil {
		return nil, err
	}
	return &attachment, nil
}

// DownloadIssueAttachment は指定課題の添付ファイルコンテンツを返す。
// GET /api/v2/issues/{issueIdOrKey}/attachments/{attachmentId}
func (c *HTTPClient) DownloadIssueAttachment(ctx context.Context, issueKey string, attachmentID int64) (io.ReadCloser, string, error) {
	apiPath := fmt.Sprintf("/api/v2/issues/%s/attachments/%d", url.PathEscape(issueKey), attachmentID)
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, "", err
	}
	return c.doDownload(req)
}

// ---- Watchings ----

// ListWatchings は指定ユーザーのウォッチ一覧を返す。
// GET /api/v2/users/{userId}/watchings
func (c *HTTPClient) ListWatchings(ctx context.Context, userID int, opt ListWatchingsOptions) ([]domain.Watching, error) {
	q := url.Values{}
	if opt.Order != "" {
		q.Set("order", opt.Order)
	}
	if opt.Sort != "" {
		q.Set("sort", opt.Sort)
	}
	if opt.Count > 0 {
		q.Set("count", strconv.Itoa(opt.Count))
	}
	if opt.Offset > 0 {
		q.Set("offset", strconv.Itoa(opt.Offset))
	}
	if opt.ResourceAlreadyRead != nil {
		if *opt.ResourceAlreadyRead {
			q.Set("resourceAlreadyRead", "true")
		} else {
			q.Set("resourceAlreadyRead", "false")
		}
	}
	if opt.IssueID > 0 {
		q.Set("issueId", strconv.Itoa(opt.IssueID))
	}
	apiPath := fmt.Sprintf("/api/v2/users/%d/watchings", userID)
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, q)
	if err != nil {
		return nil, err
	}
	var watchings []domain.Watching
	if err := c.do(req, &watchings); err != nil {
		return nil, err
	}
	return watchings, nil
}

// CountWatchings は指定ユーザーのウォッチ件数を返す。
// GET /api/v2/users/{userId}/watchings/count
func (c *HTTPClient) CountWatchings(ctx context.Context, userID int, opt ListWatchingsOptions) (int, error) {
	q := url.Values{}
	if opt.ResourceAlreadyRead != nil {
		if *opt.ResourceAlreadyRead {
			q.Set("resourceAlreadyRead", "true")
		} else {
			q.Set("resourceAlreadyRead", "false")
		}
	}
	if opt.IssueID > 0 {
		q.Set("issueId", strconv.Itoa(opt.IssueID))
	}
	apiPath := fmt.Sprintf("/api/v2/users/%d/watchings/count", userID)
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, q)
	if err != nil {
		return 0, err
	}
	var result struct {
		Count int `json:"count"`
	}
	if err := c.do(req, &result); err != nil {
		return 0, err
	}
	return result.Count, nil
}

// GetWatching は指定ウォッチの詳細を返す。
// GET /api/v2/watchings/{watchingId}
func (c *HTTPClient) GetWatching(ctx context.Context, watchingID int64) (*domain.Watching, error) {
	apiPath := fmt.Sprintf("/api/v2/watchings/%d", watchingID)
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	var watching domain.Watching
	if err := c.do(req, &watching); err != nil {
		return nil, err
	}
	return &watching, nil
}

// AddWatching は課題をウォッチ登録する。
// POST /api/v2/watchings
func (c *HTTPClient) AddWatching(ctx context.Context, reqBody AddWatchingRequest) (*domain.Watching, error) {
	q := url.Values{}
	q.Set("issueIdOrKey", reqBody.IssueIDOrKey)
	if reqBody.Note != "" {
		q.Set("note", reqBody.Note)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v2/watchings", q)
	if err != nil {
		return nil, err
	}
	var watching domain.Watching
	if err := c.do(req, &watching); err != nil {
		return nil, err
	}
	return &watching, nil
}

// UpdateWatching はウォッチのノートを更新する。
// PATCH /api/v2/watchings/{watchingId}
func (c *HTTPClient) UpdateWatching(ctx context.Context, watchingID int64, reqBody UpdateWatchingRequest) (*domain.Watching, error) {
	q := url.Values{}
	q.Set("note", reqBody.Note)
	apiPath := fmt.Sprintf("/api/v2/watchings/%d", watchingID)
	req, err := c.newRequest(ctx, http.MethodPatch, apiPath, q)
	if err != nil {
		return nil, err
	}
	var watching domain.Watching
	if err := c.do(req, &watching); err != nil {
		return nil, err
	}
	return &watching, nil
}

// DeleteWatching は指定ウォッチを削除する。
// DELETE /api/v2/watchings/{watchingId}
func (c *HTTPClient) DeleteWatching(ctx context.Context, watchingID int64) (*domain.Watching, error) {
	apiPath := fmt.Sprintf("/api/v2/watchings/%d", watchingID)
	req, err := c.newRequest(ctx, http.MethodDelete, apiPath, nil)
	if err != nil {
		return nil, err
	}
	var watching domain.Watching
	if err := c.do(req, &watching); err != nil {
		return nil, err
	}
	return &watching, nil
}

// MarkWatchingAsRead は指定ウォッチを既読化する。
// POST /api/v2/watchings/{watchingId}/markAsRead (レスポンス: 204 No Content)
func (c *HTTPClient) MarkWatchingAsRead(ctx context.Context, watchingID int64) error {
	apiPath := fmt.Sprintf("/api/v2/watchings/%d/markAsRead", watchingID)
	req, err := c.newRequest(ctx, http.MethodPost, apiPath, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// ---- Stars ----

// AddStar は課題・コメント・Wiki 等にスターを追加する。
// POST /api/v2/stars (レスポンス: 204 No Content)
func (c *HTTPClient) AddStar(ctx context.Context, reqBody AddStarRequest) error {
	q := url.Values{}
	if reqBody.IssueID != nil {
		q.Set("issueId", strconv.Itoa(*reqBody.IssueID))
	}
	if reqBody.CommentID != nil {
		q.Set("commentId", strconv.Itoa(*reqBody.CommentID))
	}
	if reqBody.WikiID != nil {
		q.Set("wikiId", strconv.Itoa(*reqBody.WikiID))
	}
	if reqBody.PullRequestID != nil {
		q.Set("pullRequestId", strconv.Itoa(*reqBody.PullRequestID))
	}
	if reqBody.PullRequestCommentID != nil {
		q.Set("pullRequestCommentId", strconv.Itoa(*reqBody.PullRequestCommentID))
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v2/stars", q)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}
