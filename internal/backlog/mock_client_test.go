package backlog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestMockClientGetMyself(t *testing.T) {
	t.Run("returns value from func", func(t *testing.T) {
		want := &domain.User{ID: 1, Name: "Test User"}
		mock := backlog.NewMockClient()
		mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
			return want, nil
		}
		got, err := mock.GetMyself(context.Background())
		if err != nil {
			t.Fatalf("GetMyself() error = %v", err)
		}
		if got.ID != want.ID {
			t.Errorf("GetMyself() ID = %d, want %d", got.ID, want.ID)
		}
		if mock.GetCallCount("GetMyself") != 1 {
			t.Errorf("GetCallCount(GetMyself) = %d, want 1", mock.GetCallCount("GetMyself"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.GetMyself(context.Background())
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("GetMyself() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientGetIssue(t *testing.T) {
	t.Run("returns issue from func", func(t *testing.T) {
		want := &domain.Issue{IssueKey: "PROJ-123", Summary: "Test Issue"}
		mock := backlog.NewMockClient()
		mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
			if issueKey != "PROJ-123" {
				return nil, backlog.ErrNotFound
			}
			return want, nil
		}
		got, err := mock.GetIssue(context.Background(), "PROJ-123")
		if err != nil {
			t.Fatalf("GetIssue() error = %v", err)
		}
		if got.IssueKey != "PROJ-123" {
			t.Errorf("GetIssue() IssueKey = %q, want %q", got.IssueKey, "PROJ-123")
		}
	})

	t.Run("call count increments", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
			return &domain.Issue{IssueKey: issueKey}, nil
		}
		_, _ = mock.GetIssue(context.Background(), "A-1")
		_, _ = mock.GetIssue(context.Background(), "A-2")
		if mock.GetCallCount("GetIssue") != 2 {
			t.Errorf("GetCallCount(GetIssue) = %d, want 2", mock.GetCallCount("GetIssue"))
		}
	})
}

func TestMockClientCreateProject(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "Project"}
		mock := backlog.NewMockClient()
		mock.CreateProjectFunc = func(ctx context.Context, req backlog.CreateProjectRequest) (*domain.Project, error) {
			if req.Name != "Project" || req.Key != "PROJ" {
				t.Errorf("request = %+v, want project name/key", req)
			}
			return want, nil
		}

		got, err := mock.CreateProject(context.Background(), backlog.CreateProjectRequest{Name: "Project", Key: "PROJ"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}
		if got != want {
			t.Errorf("CreateProject() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("CreateProject") != 1 {
			t.Errorf("GetCallCount(CreateProject) = %d, want 1", mock.GetCallCount("CreateProject"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.CreateProject(context.Background(), backlog.CreateProjectRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("CreateProject() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("CreateProject") != 1 {
			t.Errorf("GetCallCount(CreateProject) = %d, want 1", mock.GetCallCount("CreateProject"))
		}
	})
}

func TestMockClientAddCategory(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.Category{ID: 10, Name: "Backend"}
		mock := backlog.NewMockClient()
		mock.AddCategoryFunc = func(ctx context.Context, projectKey string, req backlog.AddCategoryRequest) (*domain.Category, error) {
			if projectKey != "PROJ" || req.Name != "Backend" {
				t.Errorf("arguments = %q, %+v, want PROJ and Backend", projectKey, req)
			}
			return want, nil
		}

		got, err := mock.AddCategory(context.Background(), "PROJ", backlog.AddCategoryRequest{Name: "Backend"})
		if err != nil {
			t.Fatalf("AddCategory() error = %v", err)
		}
		if got != want {
			t.Errorf("AddCategory() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("AddCategory") != 1 {
			t.Errorf("GetCallCount(AddCategory) = %d, want 1", mock.GetCallCount("AddCategory"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.AddCategory(context.Background(), "PROJ", backlog.AddCategoryRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("AddCategory() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("AddCategory") != 1 {
			t.Errorf("GetCallCount(AddCategory) = %d, want 1", mock.GetCallCount("AddCategory"))
		}
	})
}

func TestMockClientUpdateCategory(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.Category{ID: 42, Name: "Platform"}
		mock := backlog.NewMockClient()
		mock.UpdateCategoryFunc = func(ctx context.Context, projectKey string, categoryID int, req backlog.UpdateCategoryRequest) (*domain.Category, error) {
			if projectKey != "PROJ" || categoryID != 42 || req.Name != "Platform" {
				t.Errorf("arguments = %q, %d, %+v, want PROJ, 42, and Platform", projectKey, categoryID, req)
			}
			return want, nil
		}

		got, err := mock.UpdateCategory(context.Background(), "PROJ", 42, backlog.UpdateCategoryRequest{Name: "Platform"})
		if err != nil {
			t.Fatalf("UpdateCategory() error = %v", err)
		}
		if got != want {
			t.Errorf("UpdateCategory() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("UpdateCategory") != 1 {
			t.Errorf("GetCallCount(UpdateCategory) = %d, want 1", mock.GetCallCount("UpdateCategory"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.UpdateCategory(context.Background(), "PROJ", 42, backlog.UpdateCategoryRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("UpdateCategory() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("UpdateCategory") != 1 {
			t.Errorf("GetCallCount(UpdateCategory) = %d, want 1", mock.GetCallCount("UpdateCategory"))
		}
	})
}

func TestMockClientAddIssueType(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.IssueType{ID: 1, Name: "Bug", Color: "#990000"}
		mock := backlog.NewMockClient()
		mock.AddIssueTypeFunc = func(ctx context.Context, projectKey string, req backlog.AddIssueTypeRequest) (*domain.IssueType, error) {
			if projectKey != "PROJ" || req.Name != "Bug" || req.Color != "#990000" {
				t.Errorf("arguments = %q, %+v, want PROJ, Bug, and #990000", projectKey, req)
			}
			return want, nil
		}

		got, err := mock.AddIssueType(context.Background(), "PROJ", backlog.AddIssueTypeRequest{Name: "Bug", Color: "#990000"})
		if err != nil {
			t.Fatalf("AddIssueType() error = %v", err)
		}
		if got != want {
			t.Errorf("AddIssueType() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("AddIssueType") != 1 {
			t.Errorf("GetCallCount(AddIssueType) = %d, want 1", mock.GetCallCount("AddIssueType"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.AddIssueType(context.Background(), "PROJ", backlog.AddIssueTypeRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("AddIssueType() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("AddIssueType") != 1 {
			t.Errorf("GetCallCount(AddIssueType) = %d, want 1", mock.GetCallCount("AddIssueType"))
		}
	})
}

func TestMockClientUpdateIssueType(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.IssueType{ID: 42, Name: "Bug"}
		mock := backlog.NewMockClient()
		mock.UpdateIssueTypeFunc = func(ctx context.Context, projectKey string, issueTypeID int, req backlog.UpdateIssueTypeRequest) (*domain.IssueType, error) {
			if projectKey != "PROJ" || issueTypeID != 42 || req.Name == nil || *req.Name != "Bug" {
				t.Errorf("arguments = %q, %d, %+v, want PROJ, 42, and Bug", projectKey, issueTypeID, req)
			}
			return want, nil
		}
		name := "Bug"

		got, err := mock.UpdateIssueType(context.Background(), "PROJ", 42, backlog.UpdateIssueTypeRequest{Name: &name})
		if err != nil {
			t.Fatalf("UpdateIssueType() error = %v", err)
		}
		if got != want {
			t.Errorf("UpdateIssueType() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("UpdateIssueType") != 1 {
			t.Errorf("GetCallCount(UpdateIssueType) = %d, want 1", mock.GetCallCount("UpdateIssueType"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.UpdateIssueType(context.Background(), "PROJ", 42, backlog.UpdateIssueTypeRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("UpdateIssueType() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("UpdateIssueType") != 1 {
			t.Errorf("GetCallCount(UpdateIssueType) = %d, want 1", mock.GetCallCount("UpdateIssueType"))
		}
	})
}

func TestMockClientAddStatus(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.Status{ID: 101, Name: "Review", Color: "#e87758"}
		mock := backlog.NewMockClient()
		mock.AddStatusFunc = func(ctx context.Context, projectKey string, req backlog.AddStatusRequest) (*domain.Status, error) {
			if projectKey != "PROJ" || req.Name != "Review" || req.Color != "#e87758" {
				t.Errorf("arguments = %q, %+v, want PROJ, Review, and #e87758", projectKey, req)
			}
			return want, nil
		}

		got, err := mock.AddStatus(context.Background(), "PROJ", backlog.AddStatusRequest{Name: "Review", Color: "#e87758"})
		if err != nil {
			t.Fatalf("AddStatus() error = %v", err)
		}
		if got != want {
			t.Errorf("AddStatus() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("AddStatus") != 1 {
			t.Errorf("GetCallCount(AddStatus) = %d, want 1", mock.GetCallCount("AddStatus"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.AddStatus(context.Background(), "PROJ", backlog.AddStatusRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("AddStatus() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("AddStatus") != 1 {
			t.Errorf("GetCallCount(AddStatus) = %d, want 1", mock.GetCallCount("AddStatus"))
		}
	})
}

func TestMockClientUpdateStatus(t *testing.T) {
	t.Run("returns value from func and increments call count", func(t *testing.T) {
		want := &domain.Status{ID: 101, Name: "Review"}
		mock := backlog.NewMockClient()
		mock.UpdateStatusFunc = func(ctx context.Context, projectKey string, statusID int, req backlog.UpdateStatusRequest) (*domain.Status, error) {
			if projectKey != "PROJ" || statusID != 101 || req.Name == nil || *req.Name != "Review" {
				t.Errorf("arguments = %q, %d, %+v, want PROJ, 101, and Review", projectKey, statusID, req)
			}
			return want, nil
		}
		name := "Review"

		got, err := mock.UpdateStatus(context.Background(), "PROJ", 101, backlog.UpdateStatusRequest{Name: &name})
		if err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if got != want {
			t.Errorf("UpdateStatus() = %+v, want %+v", got, want)
		}
		if mock.GetCallCount("UpdateStatus") != 1 {
			t.Errorf("GetCallCount(UpdateStatus) = %d, want 1", mock.GetCallCount("UpdateStatus"))
		}
	})

	t.Run("returns ErrNotFound and increments when func is not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.UpdateStatus(context.Background(), "PROJ", 101, backlog.UpdateStatusRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("UpdateStatus() error = %v, want ErrNotFound", err)
		}
		if mock.GetCallCount("UpdateStatus") != 1 {
			t.Errorf("GetCallCount(UpdateStatus) = %d, want 1", mock.GetCallCount("UpdateStatus"))
		}
	})
}

func TestMockClientListIssues(t *testing.T) {
	t.Run("returns issues from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
			return []domain.Issue{{IssueKey: "A-1"}, {IssueKey: "A-2"}}, nil
		}
		got, err := mock.ListIssues(context.Background(), backlog.ListIssuesOptions{ProjectIDs: []int{1}})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("ListIssues() len = %d, want 2", len(got))
		}
	})
}

func TestMockClientListProjectIssueTypes(t *testing.T) {
	want := []domain.IssueType{{
		ID:                  1,
		ProjectID:           42,
		Name:                "課題",
		Color:               "#990000",
		DisplayOrder:        0,
		TemplateSummary:     "Subject",
		TemplateDescription: "Description",
	}}
	mock := backlog.NewMockClient()
	mock.ListProjectIssueTypesFunc = func(ctx context.Context, projectKey string) ([]domain.IssueType, error) {
		return want, nil
	}

	got, err := mock.ListProjectIssueTypes(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("ListProjectIssueTypes() error = %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("ListProjectIssueTypes() = %+v, want %+v", got, want)
	}
	if mock.GetCallCount("ListProjectIssueTypes") != 1 {
		t.Errorf("GetCallCount(ListProjectIssueTypes) = %d, want 1", mock.GetCallCount("ListProjectIssueTypes"))
	}
}

func TestMockClientCallCountThreadSafe(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 1}, nil
	}
	// 並列呼び出し
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mock.GetMyself(context.Background())
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if mock.GetCallCount("GetMyself") != 10 {
		t.Errorf("GetCallCount(GetMyself) = %d, want 10", mock.GetCallCount("GetMyself"))
	}
}

func TestMockClientGetTeam(t *testing.T) {
	t.Run("returns TeamWithMembers from func", func(t *testing.T) {
		want := &domain.TeamWithMembers{
			ID:   173843,
			Name: "Test Team",
			Members: []domain.User{
				{ID: 10, Name: "User Ten"},
			},
		}
		mock := backlog.NewMockClient()
		mock.GetTeamFunc = func(ctx context.Context, teamID int) (*domain.TeamWithMembers, error) {
			if teamID != 173843 {
				return nil, backlog.ErrNotFound
			}
			return want, nil
		}
		got, err := mock.GetTeam(context.Background(), 173843)
		if err != nil {
			t.Fatalf("GetTeam() error = %v", err)
		}
		if got.ID != 173843 {
			t.Errorf("ID = %d, want 173843", got.ID)
		}
		if len(got.Members) != 1 {
			t.Fatalf("len(Members) = %d, want 1", len(got.Members))
		}
		if mock.GetCallCount("GetTeam") != 1 {
			t.Errorf("GetCallCount(GetTeam) = %d, want 1", mock.GetCallCount("GetTeam"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.GetTeam(context.Background(), 173843)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("GetTeam() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientListTeams_withMembers(t *testing.T) {
	t.Run("returns TeamWithMembers slice from func", func(t *testing.T) {
		want := []domain.TeamWithMembers{
			{
				ID:   173843,
				Name: "ヘプタゴン",
				Members: []domain.User{
					{ID: 10, Name: "Alice"},
					{ID: 11, Name: "Bob"},
				},
			},
		}
		mock := backlog.NewMockClient()
		mock.ListTeamsFunc = func(ctx context.Context, opt backlog.ListTeamsOptions) ([]domain.TeamWithMembers, error) {
			return want, nil
		}
		got, err := mock.ListTeams(context.Background(), backlog.ListTeamsOptions{})
		if err != nil {
			t.Fatalf("ListTeams() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != 173843 {
			t.Errorf("ID = %d, want 173843", got[0].ID)
		}
		if len(got[0].Members) != 2 {
			t.Errorf("len(Members) = %d, want 2", len(got[0].Members))
		}
		if mock.GetCallCount("ListTeams") != 1 {
			t.Errorf("GetCallCount(ListTeams) = %d, want 1", mock.GetCallCount("ListTeams"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.ListTeams(context.Background(), backlog.ListTeamsOptions{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("ListTeams() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientListSharedFiles(t *testing.T) {
	t.Run("returns files from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.ListSharedFilesFunc = func(ctx context.Context, projectKey string, opt backlog.ListSharedFilesOptions) ([]domain.SharedFile, error) {
			return []domain.SharedFile{{ID: 1, Name: "test.txt"}}, nil
		}
		got, err := mock.ListSharedFiles(context.Background(), "PROJ", backlog.ListSharedFilesOptions{})
		if err != nil {
			t.Fatalf("ListSharedFiles() error = %v", err)
		}
		if len(got) != 1 || got[0].Name != "test.txt" {
			t.Errorf("unexpected result: %+v", got)
		}
		if mock.GetCallCount("ListSharedFiles") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("ListSharedFiles"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.ListSharedFiles(context.Background(), "PROJ", backlog.ListSharedFilesOptions{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientDownloadSharedFile(t *testing.T) {
	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, _, err := mock.DownloadSharedFile(context.Background(), "PROJ", 1)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientListIssueAttachments(t *testing.T) {
	t.Run("returns attachments from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.ListIssueAttachmentsFunc = func(ctx context.Context, issueKey string) ([]domain.IssueAttachment, error) {
			return []domain.IssueAttachment{{ID: 10, Name: "file.png"}}, nil
		}
		got, err := mock.ListIssueAttachments(context.Background(), "PROJ-1")
		if err != nil {
			t.Fatalf("ListIssueAttachments() error = %v", err)
		}
		if len(got) != 1 || got[0].Name != "file.png" {
			t.Errorf("unexpected result: %+v", got)
		}
		if mock.GetCallCount("ListIssueAttachments") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("ListIssueAttachments"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.ListIssueAttachments(context.Background(), "PROJ-1")
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientDeleteIssueAttachment(t *testing.T) {
	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.DeleteIssueAttachment(context.Background(), "PROJ-1", 10)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientDownloadIssueAttachment(t *testing.T) {
	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, _, err := mock.DownloadIssueAttachment(context.Background(), "PROJ-1", 10)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientAddStar(t *testing.T) {
	t.Run("calls func with request", func(t *testing.T) {
		mock := backlog.NewMockClient()
		issueID := 42
		var gotReq backlog.AddStarRequest
		mock.AddStarFunc = func(ctx context.Context, req backlog.AddStarRequest) error {
			gotReq = req
			return nil
		}
		err := mock.AddStar(context.Background(), backlog.AddStarRequest{IssueID: &issueID})
		if err != nil {
			t.Fatalf("AddStar() error = %v", err)
		}
		if gotReq.IssueID == nil || *gotReq.IssueID != 42 {
			t.Errorf("IssueID = %v, want 42", gotReq.IssueID)
		}
		if mock.GetCallCount("AddStar") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("AddStar"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		err := mock.AddStar(context.Background(), backlog.AddStarRequest{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientAllMethodsDefaultToErrNotFound(t *testing.T) {
	mock := backlog.NewMockClient()
	ctx := context.Background()

	t.Run("ListUsers", func(t *testing.T) {
		_, err := mock.ListUsers(ctx)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("GetIssue", func(t *testing.T) {
		_, err := mock.GetIssue(ctx, "A-1")
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("ListIssues", func(t *testing.T) {
		_, err := mock.ListIssues(ctx, backlog.ListIssuesOptions{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("GetSpace", func(t *testing.T) {
		_, err := mock.GetSpace(ctx)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

// TestMockClientWatchings は Watching 系メソッドのモックテスト。
func TestMockClientListWatchings(t *testing.T) {
	t.Run("returns watchings from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
			return []domain.Watching{{ID: 1, Type: "issue"}}, nil
		}
		got, err := mock.ListWatchings(context.Background(), 123, backlog.ListWatchingsOptions{})
		if err != nil {
			t.Fatalf("ListWatchings() error = %v", err)
		}
		if len(got) != 1 || got[0].ID != 1 {
			t.Errorf("unexpected result: %+v", got)
		}
		if mock.GetCallCount("ListWatchings") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("ListWatchings"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.ListWatchings(context.Background(), 123, backlog.ListWatchingsOptions{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientCountWatchings(t *testing.T) {
	t.Run("returns count from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.CountWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) (int, error) {
			return 5, nil
		}
		count, err := mock.CountWatchings(context.Background(), 123, backlog.ListWatchingsOptions{})
		if err != nil {
			t.Fatalf("CountWatchings() error = %v", err)
		}
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})
}

func TestMockClientGetWatching(t *testing.T) {
	t.Run("returns watching from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.GetWatchingFunc = func(ctx context.Context, watchingID int64) (*domain.Watching, error) {
			return &domain.Watching{ID: watchingID, Type: "issue"}, nil
		}
		got, err := mock.GetWatching(context.Background(), 42)
		if err != nil {
			t.Fatalf("GetWatching() error = %v", err)
		}
		if got.ID != 42 {
			t.Errorf("ID = %d, want 42", got.ID)
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.GetWatching(context.Background(), 42)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientAddWatching(t *testing.T) {
	t.Run("calls func with request", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var capturedReq backlog.AddWatchingRequest
		mock.AddWatchingFunc = func(ctx context.Context, req backlog.AddWatchingRequest) (*domain.Watching, error) {
			capturedReq = req
			return &domain.Watching{ID: 100, Type: "issue"}, nil
		}
		got, err := mock.AddWatching(context.Background(), backlog.AddWatchingRequest{IssueIDOrKey: "PROJ-1", Note: "test"})
		if err != nil {
			t.Fatalf("AddWatching() error = %v", err)
		}
		if capturedReq.IssueIDOrKey != "PROJ-1" {
			t.Errorf("IssueIDOrKey = %q, want PROJ-1", capturedReq.IssueIDOrKey)
		}
		if got.ID != 100 {
			t.Errorf("ID = %d, want 100", got.ID)
		}
	})
}

func TestMockClientMarkWatchingAsRead(t *testing.T) {
	t.Run("calls func and returns nil", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var capturedID int64
		mock.MarkWatchingAsReadFunc = func(ctx context.Context, watchingID int64) error {
			capturedID = watchingID
			return nil
		}
		err := mock.MarkWatchingAsRead(context.Background(), 42)
		if err != nil {
			t.Fatalf("MarkWatchingAsRead() error = %v", err)
		}
		if capturedID != 42 {
			t.Errorf("watchingID = %d, want 42", capturedID)
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		err := mock.MarkWatchingAsRead(context.Background(), 42)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientListProjectUsers(t *testing.T) {
	m := backlog.NewMockClient()
	if _, err := m.ListProjectUsers(context.Background(), "PROJ", backlog.ListProjectUsersOptions{}); !errors.Is(err, backlog.ErrNotFound) {
		t.Errorf("Func 未設定時の error = %v, want ErrNotFound", err)
	}

	m.ListProjectUsersFunc = func(ctx context.Context, projectKey string, opt backlog.ListProjectUsersOptions) ([]domain.User, error) {
		return []domain.User{{ID: 1, Name: "山田 太郎"}}, nil
	}
	users, err := m.ListProjectUsers(context.Background(), "PROJ", backlog.ListProjectUsersOptions{})
	if err != nil || len(users) != 1 {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	if got := m.GetCallCount("ListProjectUsers"); got != 2 {
		t.Errorf("GetCallCount = %d, want 2", got)
	}
}
