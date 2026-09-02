package conventions

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestApplyExecutesItemsInOrderAndSkipsNonActions(t *testing.T) {
	ctx := context.Background()
	client := backlog.NewMockClient()
	var calls []string
	client.CreateProjectFunc = func(context.Context, backlog.CreateProjectRequest) (*domain.Project, error) {
		calls = append(calls, "CreateProject")
		return &domain.Project{ID: 101}, nil
	}
	client.AddIssueTypeFunc = func(context.Context, string, backlog.AddIssueTypeRequest) (*domain.IssueType, error) {
		calls = append(calls, "AddIssueType")
		return &domain.IssueType{ID: 201}, nil
	}
	client.UpdateStatusFunc = func(context.Context, string, int, backlog.UpdateStatusRequest) (*domain.Status, error) {
		calls = append(calls, "UpdateStatus")
		return &domain.Status{ID: 301}, nil
	}
	client.UpdateCategoryFunc = func(context.Context, string, int, backlog.UpdateCategoryRequest) (*domain.Category, error) {
		calls = append(calls, "UpdateCategory")
		return &domain.Category{ID: 401}, nil
	}
	client.UpdateIssueFunc = func(context.Context, string, backlog.UpdateIssueRequest) (*domain.Issue, error) {
		calls = append(calls, "UpdateIssue")
		return &domain.Issue{ID: 501}, nil
	}

	plan := &Plan{
		ProjectKey: "PROJ",
		Items: []PlanItem{
			{Resource: KindProject, Action: ActionCreate, Name: "PROJ", createProjectRequest: &backlog.CreateProjectRequest{Key: "PROJ", Name: "Project"}},
			{Resource: KindIssueType, Action: ActionCreate, Name: "案件", createIssueTypeRequest: &backlog.AddIssueTypeRequest{Name: "案件"}},
			{Resource: KindStatus, Action: ActionUpdate, Name: "レビュー中", targetID: 31, updateStatusRequest: &backlog.UpdateStatusRequest{}},
			{Resource: KindCategory, Action: ActionUpdate, Name: "開発", targetID: 41, updateCategoryRequest: &backlog.UpdateCategoryRequest{Name: "開発"}},
			{Resource: KindIssue, Action: ActionUpdate, Name: "[案件] 開発", issueKey: "PROJ-1", targetID: 51, updateIssueRequest: &backlog.UpdateIssueRequest{}},
			{Resource: KindIssueType, Action: ActionUnchanged, Name: "規約"},
			{Resource: KindIssue, Action: ActionSkip, Name: "[案件] 未指定", Reason: "Lead が未指定"},
		},
	}

	result, err := Apply(ctx, client, plan)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if want := []string{"CreateProject", "AddIssueType", "UpdateStatus", "UpdateCategory", "UpdateIssue"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if result.Summary != (ApplySummary{Applied: 6, Skipped: 1}) {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if result.Items[5].Status != StatusApplied || result.Items[6].Status != StatusSkipped {
		t.Fatalf("non-write statuses = %q, %q", result.Items[5].Status, result.Items[6].Status)
	}
	if client.GetCallCount("CreateProject") != 1 || client.GetCallCount("AddIssueType") != 1 {
		t.Fatalf("unexpected write counts: project=%d type=%d", client.GetCallCount("CreateProject"), client.GetCallCount("AddIssueType"))
	}
}

func TestApplyStopsAfterFailureAndMarksRemainingNotReached(t *testing.T) {
	ctx := context.Background()
	client := backlog.NewMockClient()
	wantErr := errors.New("status API failed")
	client.AddIssueTypeFunc = func(context.Context, string, backlog.AddIssueTypeRequest) (*domain.IssueType, error) {
		return &domain.IssueType{ID: 201}, nil
	}
	client.AddStatusFunc = func(context.Context, string, backlog.AddStatusRequest) (*domain.Status, error) {
		return nil, wantErr
	}
	client.AddCategoryFunc = func(context.Context, string, backlog.AddCategoryRequest) (*domain.Category, error) {
		t.Fatal("AddCategory() was called after failure")
		return nil, nil
	}

	plan := &Plan{ProjectKey: "PROJ", Items: []PlanItem{
		{Resource: KindIssueType, Action: ActionCreate, Name: "案件", createIssueTypeRequest: &backlog.AddIssueTypeRequest{Name: "案件"}},
		{Resource: KindStatus, Action: ActionCreate, Name: "レビュー中", createStatusRequest: &backlog.AddStatusRequest{Name: "レビュー中"}},
		{Resource: KindCategory, Action: ActionCreate, Name: "開発", createCategoryRequest: &backlog.AddCategoryRequest{Name: "開発"}},
	}}

	result, err := Apply(ctx, client, plan)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Apply() error = %v, want %v", err, wantErr)
	}
	wantStatuses := []ItemStatus{StatusApplied, StatusFailed, StatusNotReached}
	for i, want := range wantStatuses {
		if result.Items[i].Status != want {
			t.Errorf("items[%d].Status = %q, want %q", i, result.Items[i].Status, want)
		}
	}
	if result.Items[1].Error != wantErr.Error() {
		t.Errorf("failed item error = %q, want %q", result.Items[1].Error, wantErr)
	}
	if result.Summary != (ApplySummary{Applied: 1, Failed: 1, NotReached: 1}) {
		t.Fatalf("summary = %#v", result.Summary)
	}
}

func TestApplyResolvesIDsFromCreatedResources(t *testing.T) {
	ctx := context.Background()
	client := backlog.NewMockClient()
	var got backlog.CreateIssueRequest
	var calls []string
	client.CreateProjectFunc = func(context.Context, backlog.CreateProjectRequest) (*domain.Project, error) {
		calls = append(calls, "CreateProject")
		return &domain.Project{ID: 101}, nil
	}
	client.AddIssueTypeFunc = func(context.Context, string, backlog.AddIssueTypeRequest) (*domain.IssueType, error) {
		calls = append(calls, "AddIssueType")
		return &domain.IssueType{ID: 201, Name: "案件"}, nil
	}
	client.CreateIssueFunc = func(_ context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		calls = append(calls, "CreateIssue")
		got = req
		return &domain.Issue{ID: 301, IssueKey: "PROJ-1"}, nil
	}

	plan := &Plan{ProjectKey: "PROJ", Items: []PlanItem{
		{Resource: KindProject, Action: ActionCreate, Name: "PROJ", createProjectRequest: &backlog.CreateProjectRequest{Key: "PROJ", Name: "Project"}},
		{Resource: KindIssueType, Action: ActionCreate, Name: "案件", createIssueTypeRequest: &backlog.AddIssueTypeRequest{Name: "案件"}},
		{Resource: KindIssue, Action: ActionCreate, Name: "[案件] 顧客A", issueTypeName: "案件", createIssueRequest: &backlog.CreateIssueRequest{Summary: "[案件] 顧客A"}},
	}}

	if _, err := Apply(ctx, client, plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"CreateProject", "AddIssueType", "CreateIssue"}) {
		t.Fatalf("calls = %v", calls)
	}
	if got.ProjectID != 101 || got.IssueTypeID != 201 {
		t.Fatalf("resolved request IDs = project:%d issue_type:%d", got.ProjectID, got.IssueTypeID)
	}
}

func TestApplyResolvesCreatedCategoryForIssueRequests(t *testing.T) {
	ctx := context.Background()
	client := backlog.NewMockClient()
	var createReq backlog.CreateIssueRequest
	var updateReq backlog.UpdateIssueRequest
	client.AddCategoryFunc = func(context.Context, string, backlog.AddCategoryRequest) (*domain.Category, error) {
		return &domain.Category{ID: 401, Name: "顧客A"}, nil
	}
	client.CreateIssueFunc = func(_ context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		createReq = req
		return &domain.Issue{ID: 501}, nil
	}
	client.UpdateIssueFunc = func(_ context.Context, _ string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		updateReq = req
		return &domain.Issue{ID: 502}, nil
	}

	plan := &Plan{ProjectKey: "PROJ", Items: []PlanItem{
		{Resource: KindProject, Action: ActionUnchanged, Name: "PROJ", targetID: 101},
		{Resource: KindCategory, Action: ActionCreate, Name: "顧客A", createCategoryRequest: &backlog.AddCategoryRequest{Name: "顧客A"}},
		{Resource: KindIssue, Action: ActionCreate, Name: "[案件] 新規", categoryName: "顧客A", createIssueRequest: &backlog.CreateIssueRequest{ProjectID: 101, IssueTypeID: 201, Summary: "[案件] 新規"}},
		{Resource: KindIssue, Action: ActionUpdate, Name: "[案件] 既存", categoryName: "顧客A", issueKey: "PROJ-2", updateIssueRequest: &backlog.UpdateIssueRequest{}},
	}}

	if _, err := Apply(ctx, client, plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !reflect.DeepEqual(createReq.CategoryIDs, []int{401}) {
		t.Fatalf("CreateIssue CategoryIDs = %v, want [401]", createReq.CategoryIDs)
	}
	if !reflect.DeepEqual(updateReq.CategoryIDs, []int{401}) {
		t.Fatalf("UpdateIssue CategoryIDs = %v, want [401]", updateReq.CategoryIDs)
	}
}

func TestApplyRoundTripBuildPlanIsIdempotent(t *testing.T) {
	c := testConventions()
	client := backlog.NewMockClient()
	var project *domain.Project
	var issueTypes []domain.IssueType
	var statuses []domain.Status
	var categories []domain.Category
	var issues []domain.Issue

	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		if project == nil {
			return nil, backlog.ErrNotFound
		}
		return project, nil
	}
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) { return issueTypes, nil }
	client.ListProjectStatusesFunc = func(context.Context, string) ([]domain.Status, error) { return statuses, nil }
	client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) { return categories, nil }
	client.ListProjectUsersFunc = func(context.Context, string, backlog.ListProjectUsersOptions) ([]domain.User, error) {
		return testUsers(), nil
	}
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) { return issues, nil }
	client.CreateProjectFunc = func(_ context.Context, req backlog.CreateProjectRequest) (*domain.Project, error) {
		project = &domain.Project{ID: 1, ProjectKey: req.Key, Name: req.Name}
		return project, nil
	}
	client.AddIssueTypeFunc = func(_ context.Context, _ string, req backlog.AddIssueTypeRequest) (*domain.IssueType, error) {
		item := domain.IssueType{ID: 10 + len(issueTypes), Name: req.Name, Color: req.Color, TemplateSummary: req.TemplateSummary, TemplateDescription: req.TemplateDescription}
		issueTypes = append(issueTypes, item)
		return &item, nil
	}
	client.AddStatusFunc = func(_ context.Context, _ string, req backlog.AddStatusRequest) (*domain.Status, error) {
		item := domain.Status{ID: 5 + len(statuses), Name: req.Name, Color: req.Color}
		statuses = append(statuses, item)
		return &item, nil
	}
	client.AddCategoryFunc = func(_ context.Context, _ string, req backlog.AddCategoryRequest) (*domain.Category, error) {
		item := domain.Category{ID: 20 + len(categories), Name: req.Name}
		categories = append(categories, item)
		return &item, nil
	}
	client.CreateIssueFunc = func(_ context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		id := 100 + len(issues)
		issueTypeName := ""
		for _, item := range issueTypes {
			if item.ID == req.IssueTypeID {
				issueTypeName = item.Name
			}
		}
		item := domain.Issue{ID: id, IssueKey: "PROJ-" + string(rune('1'+len(issues))), ProjectID: req.ProjectID, Summary: req.Summary, Description: req.Description, IssueType: &domain.IDName{ID: req.IssueTypeID, Name: issueTypeName}, StartDate: req.StartDate, DueDate: req.DueDate}
		if req.Summary != RuleIssueSummary {
			item.Assignee = &domain.User{ID: 42, Name: c.Engagements[0].Lead}
		}
		for _, categoryID := range req.CategoryIDs {
			for _, category := range categories {
				if category.ID == categoryID {
					item.Categories = append(item.Categories, domain.IDName{ID: category.ID, Name: category.Name})
				}
			}
		}
		issues = append(issues, item)
		return &item, nil
	}

	first, err := BuildPlan(context.Background(), client, c, PlanOptions{Create: true})
	if err != nil {
		t.Fatalf("first BuildPlan() error = %v", err)
	}
	if _, err := Apply(context.Background(), client, first); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	second, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err != nil {
		t.Fatalf("second BuildPlan() error = %v", err)
	}
	if second.Summary != (PlanSummary{Unchanged: len(second.Items)}) {
		t.Fatalf("second summary = %#v, want all unchanged", second.Summary)
	}
	for i, item := range second.Items {
		if item.Action != ActionUnchanged {
			t.Errorf("second items[%d].Action = %q, want unchanged", i, item.Action)
		}
	}
}
