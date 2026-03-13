package backlog

import (
	"context"
	"sync"

	"github.com/youyo/logvalet/internal/domain"
)

// MockClient はテスト用の Client 実装。
// 各メソッドに対して Func フィールドをセットすることで動作を制御する。
// セットされていない Func は ErrNotFound を返す。
// CallCounts は mu (sync.Mutex) で保護されスレッドセーフ。
type MockClient struct {
	// Auth / user identity
	GetMyselfFunc          func(ctx context.Context) (*domain.User, error)
	ListUsersFunc          func(ctx context.Context) ([]domain.User, error)
	GetUserFunc            func(ctx context.Context, userID string) (*domain.User, error)
	ListUserActivitiesFunc func(ctx context.Context, userID string, opt ListUserActivitiesOptions) ([]domain.Activity, error)

	// Issues
	GetIssueFunc    func(ctx context.Context, issueKey string) (*domain.Issue, error)
	ListIssuesFunc  func(ctx context.Context, opt ListIssuesOptions) ([]domain.Issue, error)
	CreateIssueFunc func(ctx context.Context, req CreateIssueRequest) (*domain.Issue, error)
	UpdateIssueFunc func(ctx context.Context, issueKey string, req UpdateIssueRequest) (*domain.Issue, error)

	// Issue comments
	ListIssueCommentsFunc  func(ctx context.Context, issueKey string, opt ListCommentsOptions) ([]domain.Comment, error)
	AddIssueCommentFunc    func(ctx context.Context, issueKey string, req AddCommentRequest) (*domain.Comment, error)
	UpdateIssueCommentFunc func(ctx context.Context, issueKey string, commentID int64, req UpdateCommentRequest) (*domain.Comment, error)

	// Projects
	GetProjectFunc           func(ctx context.Context, projectKey string) (*domain.Project, error)
	ListProjectsFunc         func(ctx context.Context) ([]domain.Project, error)
	ListProjectActivitiesFunc func(ctx context.Context, projectKey string, opt ListActivitiesOptions) ([]domain.Activity, error)

	// Space activities
	ListSpaceActivitiesFunc func(ctx context.Context, opt ListActivitiesOptions) ([]domain.Activity, error)

	// Documents
	GetDocumentFunc            func(ctx context.Context, documentID int64) (*domain.Document, error)
	ListDocumentsFunc          func(ctx context.Context, projectKey string, opt ListDocumentsOptions) ([]domain.Document, error)
	GetDocumentTreeFunc        func(ctx context.Context, projectKey string) ([]domain.DocumentNode, error)
	CreateDocumentFunc         func(ctx context.Context, req CreateDocumentRequest) (*domain.Document, error)
	ListDocumentAttachmentsFunc func(ctx context.Context, documentID int64) ([]domain.Attachment, error)

	// Project meta
	ListProjectStatusesFunc      func(ctx context.Context, projectKey string) ([]domain.Status, error)
	ListProjectCategoriesFunc    func(ctx context.Context, projectKey string) ([]domain.Category, error)
	ListProjectVersionsFunc      func(ctx context.Context, projectKey string) ([]domain.Version, error)
	ListProjectCustomFieldsFunc  func(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error)

	// Teams
	ListTeamsFunc        func(ctx context.Context) ([]domain.Team, error)
	ListProjectTeamsFunc func(ctx context.Context, projectKey string) ([]domain.Team, error)

	// Space
	GetSpaceFunc          func(ctx context.Context) (*domain.Space, error)
	GetSpaceDiskUsageFunc func(ctx context.Context) (*domain.DiskUsage, error)

	mu         sync.Mutex
	callCounts map[string]int
}

// NewMockClient は新しい MockClient を返す。
func NewMockClient() *MockClient {
	return &MockClient{
		callCounts: make(map[string]int),
	}
}

// GetCallCount は指定メソッドの呼び出し回数を返す（スレッドセーフ）。
func (m *MockClient) GetCallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCounts[method]
}

// increment はメソッド呼び出し回数をインクリメントする（内部用）。
func (m *MockClient) increment(method string) {
	m.mu.Lock()
	m.callCounts[method]++
	m.mu.Unlock()
}

// ---- Client interface 実装 ----

func (m *MockClient) GetMyself(ctx context.Context) (*domain.User, error) {
	m.increment("GetMyself")
	if m.GetMyselfFunc != nil {
		return m.GetMyselfFunc(ctx)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListUsers(ctx context.Context) ([]domain.User, error) {
	m.increment("ListUsers")
	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(ctx)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	m.increment("GetUser")
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, userID)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListUserActivities(ctx context.Context, userID string, opt ListUserActivitiesOptions) ([]domain.Activity, error) {
	m.increment("ListUserActivities")
	if m.ListUserActivitiesFunc != nil {
		return m.ListUserActivitiesFunc(ctx, userID, opt)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetIssue(ctx context.Context, issueKey string) (*domain.Issue, error) {
	m.increment("GetIssue")
	if m.GetIssueFunc != nil {
		return m.GetIssueFunc(ctx, issueKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListIssues(ctx context.Context, opt ListIssuesOptions) ([]domain.Issue, error) {
	m.increment("ListIssues")
	if m.ListIssuesFunc != nil {
		return m.ListIssuesFunc(ctx, opt)
	}
	return nil, ErrNotFound
}

func (m *MockClient) CreateIssue(ctx context.Context, req CreateIssueRequest) (*domain.Issue, error) {
	m.increment("CreateIssue")
	if m.CreateIssueFunc != nil {
		return m.CreateIssueFunc(ctx, req)
	}
	return nil, ErrNotFound
}

func (m *MockClient) UpdateIssue(ctx context.Context, issueKey string, req UpdateIssueRequest) (*domain.Issue, error) {
	m.increment("UpdateIssue")
	if m.UpdateIssueFunc != nil {
		return m.UpdateIssueFunc(ctx, issueKey, req)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListIssueComments(ctx context.Context, issueKey string, opt ListCommentsOptions) ([]domain.Comment, error) {
	m.increment("ListIssueComments")
	if m.ListIssueCommentsFunc != nil {
		return m.ListIssueCommentsFunc(ctx, issueKey, opt)
	}
	return nil, ErrNotFound
}

func (m *MockClient) AddIssueComment(ctx context.Context, issueKey string, req AddCommentRequest) (*domain.Comment, error) {
	m.increment("AddIssueComment")
	if m.AddIssueCommentFunc != nil {
		return m.AddIssueCommentFunc(ctx, issueKey, req)
	}
	return nil, ErrNotFound
}

func (m *MockClient) UpdateIssueComment(ctx context.Context, issueKey string, commentID int64, req UpdateCommentRequest) (*domain.Comment, error) {
	m.increment("UpdateIssueComment")
	if m.UpdateIssueCommentFunc != nil {
		return m.UpdateIssueCommentFunc(ctx, issueKey, commentID, req)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetProject(ctx context.Context, projectKey string) (*domain.Project, error) {
	m.increment("GetProject")
	if m.GetProjectFunc != nil {
		return m.GetProjectFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjects(ctx context.Context) ([]domain.Project, error) {
	m.increment("ListProjects")
	if m.ListProjectsFunc != nil {
		return m.ListProjectsFunc(ctx)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjectActivities(ctx context.Context, projectKey string, opt ListActivitiesOptions) ([]domain.Activity, error) {
	m.increment("ListProjectActivities")
	if m.ListProjectActivitiesFunc != nil {
		return m.ListProjectActivitiesFunc(ctx, projectKey, opt)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListSpaceActivities(ctx context.Context, opt ListActivitiesOptions) ([]domain.Activity, error) {
	m.increment("ListSpaceActivities")
	if m.ListSpaceActivitiesFunc != nil {
		return m.ListSpaceActivitiesFunc(ctx, opt)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetDocument(ctx context.Context, documentID int64) (*domain.Document, error) {
	m.increment("GetDocument")
	if m.GetDocumentFunc != nil {
		return m.GetDocumentFunc(ctx, documentID)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListDocuments(ctx context.Context, projectKey string, opt ListDocumentsOptions) ([]domain.Document, error) {
	m.increment("ListDocuments")
	if m.ListDocumentsFunc != nil {
		return m.ListDocumentsFunc(ctx, projectKey, opt)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetDocumentTree(ctx context.Context, projectKey string) ([]domain.DocumentNode, error) {
	m.increment("GetDocumentTree")
	if m.GetDocumentTreeFunc != nil {
		return m.GetDocumentTreeFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) CreateDocument(ctx context.Context, req CreateDocumentRequest) (*domain.Document, error) {
	m.increment("CreateDocument")
	if m.CreateDocumentFunc != nil {
		return m.CreateDocumentFunc(ctx, req)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListDocumentAttachments(ctx context.Context, documentID int64) ([]domain.Attachment, error) {
	m.increment("ListDocumentAttachments")
	if m.ListDocumentAttachmentsFunc != nil {
		return m.ListDocumentAttachmentsFunc(ctx, documentID)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjectStatuses(ctx context.Context, projectKey string) ([]domain.Status, error) {
	m.increment("ListProjectStatuses")
	if m.ListProjectStatusesFunc != nil {
		return m.ListProjectStatusesFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjectCategories(ctx context.Context, projectKey string) ([]domain.Category, error) {
	m.increment("ListProjectCategories")
	if m.ListProjectCategoriesFunc != nil {
		return m.ListProjectCategoriesFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjectVersions(ctx context.Context, projectKey string) ([]domain.Version, error) {
	m.increment("ListProjectVersions")
	if m.ListProjectVersionsFunc != nil {
		return m.ListProjectVersionsFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjectCustomFields(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
	m.increment("ListProjectCustomFields")
	if m.ListProjectCustomFieldsFunc != nil {
		return m.ListProjectCustomFieldsFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListTeams(ctx context.Context) ([]domain.Team, error) {
	m.increment("ListTeams")
	if m.ListTeamsFunc != nil {
		return m.ListTeamsFunc(ctx)
	}
	return nil, ErrNotFound
}

func (m *MockClient) ListProjectTeams(ctx context.Context, projectKey string) ([]domain.Team, error) {
	m.increment("ListProjectTeams")
	if m.ListProjectTeamsFunc != nil {
		return m.ListProjectTeamsFunc(ctx, projectKey)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetSpace(ctx context.Context) (*domain.Space, error) {
	m.increment("GetSpace")
	if m.GetSpaceFunc != nil {
		return m.GetSpaceFunc(ctx)
	}
	return nil, ErrNotFound
}

func (m *MockClient) GetSpaceDiskUsage(ctx context.Context) (*domain.DiskUsage, error) {
	m.increment("GetSpaceDiskUsage")
	if m.GetSpaceDiskUsageFunc != nil {
		return m.GetSpaceDiskUsageFunc(ctx)
	}
	return nil, ErrNotFound
}
