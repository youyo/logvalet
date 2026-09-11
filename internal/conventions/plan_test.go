package conventions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"gopkg.in/yaml.v3"
)

func testConventions() *Conventions {
	return &Conventions{
		SchemaVersion: SchemaVersion,
		Project:       Project{Key: "PROJ", Name: "Project"},
		IssueTypes: []IssueType{
			{Name: IssueTypeRule, Color: IssueTypeColors[0]},
			{Name: IssueTypeEngagement, Color: IssueTypeColors[1], TemplateDescription: "Context & Goals\n"},
		},
		Statuses:    []Status{{Name: "レビュー中", Color: StatusColors[0]}},
		Initiatives: []Initiative{{Name: "運用保守"}},
		Engagements: []Engagement{{
			Name:       "顧客A 基盤更改",
			Lead:       "山田 太郎",
			Initiative: "運用保守",
			StartDate:  "2026-10-01",
			DueDate:    "2026-10-31",
		}},
	}
}

func testUsers() []domain.User {
	return []domain.User{{ID: 42, Name: "山田 太郎"}}
}

func testIssueTypes(c *Conventions) []domain.IssueType {
	return []domain.IssueType{
		{ID: 10, Name: IssueTypeRule, Color: c.IssueTypes[0].Color},
		{ID: 11, Name: IssueTypeEngagement, Color: c.IssueTypes[1].Color, TemplateDescription: c.IssueTypes[1].TemplateDescription},
	}
}

func testStatuses(c *Conventions) []domain.Status {
	return []domain.Status{{ID: 5, Name: c.Statuses[0].Name, Color: c.Statuses[0].Color}}
}

func testIssues(c *Conventions) []domain.Issue {
	start, _ := time.Parse("2006-01-02", c.Engagements[0].StartDate)
	due, _ := time.Parse("2006-01-02", c.Engagements[0].DueDate)
	return []domain.Issue{
		{ID: 100, IssueKey: "PROJ-1", Summary: RuleIssueSummary, IssueType: &domain.IDName{ID: 10, Name: IssueTypeRule}, Description: testRuleDescription(c)},
		{ID: 101, IssueKey: "PROJ-2", Summary: "[案件] " + c.Engagements[0].Name, IssueType: &domain.IDName{ID: 11, Name: IssueTypeEngagement}, Assignee: &domain.User{ID: 42, Name: c.Engagements[0].Lead}, StartDate: &start, DueDate: &due, Categories: []domain.IDName{{ID: 20, Name: c.Engagements[0].Name}}},
	}
}

func testRuleDescription(c *Conventions) string {
	yamlSource, _ := yaml.Marshal(c)
	return BuildRuleIssueDescription(yamlSource)
}

func configuredExistingClient(c *Conventions) *backlog.MockClient {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: c.Project.Key, Name: c.Project.Name}, nil
	}
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) { return testIssueTypes(c), nil }
	client.ListProjectStatusesFunc = func(context.Context, string) ([]domain.Status, error) { return testStatuses(c), nil }
	client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) {
		return []domain.Category{{ID: 20, Name: c.Engagements[0].Name}}, nil
	}
	client.ListProjectUsersFunc = func(context.Context, string, backlog.ListProjectUsersOptions) ([]domain.User, error) {
		return testUsers(), nil
	}
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) { return testIssues(c), nil }
	return client
}

func TestBuildPlan_EmptyProjectCreatesResourcesInOrder(t *testing.T) {
	c := testConventions()
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: c.Project.Key, Name: c.Project.Name}, nil
	}
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) { return nil, nil }
	client.ListProjectStatusesFunc = func(context.Context, string) ([]domain.Status, error) { return nil, nil }
	client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) { return nil, nil }
	client.ListProjectUsersFunc = func(context.Context, string, backlog.ListProjectUsersOptions) ([]domain.User, error) {
		return testUsers(), nil
	}
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) { return nil, nil }

	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	want := []struct {
		resource ResourceKind
		action   Action
	}{
		{KindProject, ActionUnchanged},
		{KindIssueType, ActionCreate}, {KindIssueType, ActionCreate},
		{KindStatus, ActionCreate}, {KindCategory, ActionCreate},
		{KindIssue, ActionCreate}, {KindIssue, ActionCreate},
	}
	if len(plan.Items) != len(want) {
		t.Fatalf("items = %d, want %d: %#v", len(plan.Items), len(want), plan.Items)
	}
	for i, expected := range want {
		if plan.Items[i].Resource != expected.resource || plan.Items[i].Action != expected.action {
			t.Errorf("items[%d] = (%s, %s), want (%s, %s)", i, plan.Items[i].Resource, plan.Items[i].Action, expected.resource, expected.action)
		}
	}
	if plan.Summary.Create != 6 || plan.Summary.Unchanged != 1 {
		t.Fatalf("summary = %#v", plan.Summary)
	}
	if writes := client.GetCallCount("CreateIssue") + client.GetCallCount("UpdateIssue") + client.GetCallCount("CreateProject") + client.GetCallCount("AddIssueType") + client.GetCallCount("UpdateIssueType") + client.GetCallCount("AddStatus") + client.GetCallCount("UpdateStatus") + client.GetCallCount("AddCategory") + client.GetCallCount("UpdateCategory"); writes != 0 {
		t.Fatalf("write calls = %d", writes)
	}
}

func TestBuildPlan_AlreadyAppliedIsIdempotent(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	for i, item := range plan.Items {
		if item.Action != ActionUnchanged {
			t.Errorf("items[%d] action = %s, want unchanged", i, item.Action)
		}
	}
	if plan.Summary != (PlanSummary{Unchanged: len(plan.Items)}) {
		t.Fatalf("summary = %#v", plan.Summary)
	}
}

func TestBuildPlan_IssueTypeTemplateDifferenceIsOneUpdate(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) {
		got := testIssueTypes(c)
		got[1].TemplateDescription = "old template"
		return got, nil
	}
	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.Summary != (PlanSummary{Update: 1, Unchanged: 6}) {
		t.Fatalf("summary = %#v", plan.Summary)
	}
	item := plan.Items[2]
	if item.Resource != KindIssueType || item.Action != ActionUpdate || len(item.Changes) != 1 || item.Changes[0] != (FieldChange{Field: "template_description", From: "old template", To: "(変更あり)"}) {
		t.Fatalf("issue type item = %#v", item)
	}
}

func TestBuildPlan_DuplicateCategoryReturnsError(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) {
		return []domain.Category{{ID: 1, Name: c.Engagements[0].Name}, {ID: 2, Name: " " + c.Engagements[0].Name + " "}}, nil
	}
	assertPlanErrorContains(t, client, c, "カテゴリ", c.Engagements[0].Name)
}

func TestBuildPlan_DuplicateRuleIssueReturnsError(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		issues := testIssues(c)
		issues = append(issues, domain.Issue{ID: 102, IssueKey: "PROJ-3", Summary: "別件", IssueType: &domain.IDName{Name: IssueTypeRule}})
		return issues, nil
	}
	assertPlanErrorContains(t, client, c, "規約課題", "2")
}

func TestBuildPlan_LeadConditionsSkipOnlyParentIssue(t *testing.T) {
	tests := []struct {
		name       string
		lead       string
		users      []domain.User
		wantReason string
	}{
		{name: "empty", lead: "", wantReason: "Lead が未指定のため案件親課題を作成しません"},
		{name: "missing", lead: "不在", users: []domain.User{{ID: 1, Name: "別人"}}, wantReason: `Lead "不在" がプロジェクトメンバーに見つかりません`},
		{name: "multiple", lead: "山田 太郎", users: []domain.User{{ID: 1, Name: "山田 太郎"}, {ID: 2, Name: " 山田 太郎 "}}, wantReason: `Lead "山田 太郎" が複数のメンバーに一致します`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testConventions()
			c.Engagements[0].Lead = tt.lead
			client := configuredExistingClient(c)
			client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) { return nil, nil }
			client.ListProjectUsersFunc = func(context.Context, string, backlog.ListProjectUsersOptions) ([]domain.User, error) {
				return tt.users, nil
			}
			plan, err := BuildPlan(context.Background(), client, c, PlanOptions{})
			if err != nil {
				t.Fatalf("BuildPlan() error = %v", err)
			}
			if plan.Items[4].Action != ActionCreate {
				t.Errorf("category action = %s, want create", plan.Items[4].Action)
			}
			item := plan.Items[len(plan.Items)-1]
			if item.Action != ActionSkip || item.Reason != tt.wantReason {
				t.Errorf("parent item = %#v, want skip %q", item, tt.wantReason)
			}
		})
	}
}

func TestBuildPlan_EightExistingCustomStatusesSkipNewStatus(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	client.ListProjectStatusesFunc = func(context.Context, string) ([]domain.Status, error) {
		statuses := make([]domain.Status, 8)
		for i := range statuses {
			statuses[i] = domain.Status{ID: i + 5, Name: "既存状態" + string(rune('A'+i)), Color: StatusColors[0]}
		}
		return statuses, nil
	}
	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	item := plan.Items[3]
	if item.Action != ActionSkip || item.Reason != "カスタム状態は最大 8 個までです" {
		t.Fatalf("status item = %#v", item)
	}
}

func TestBuildPlan_ProjectMissingWithoutCreateReturnsError(t *testing.T) {
	c := testConventions()
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) { return nil, backlog.ErrNotFound }
	if _, err := BuildPlan(context.Background(), client, c, PlanOptions{}); err == nil {
		t.Fatal("存在しないプロジェクトでエラーになりませんでした")
	}
}

func TestBuildPlan_ProjectMissingWithCreateDoesNotFetchResources(t *testing.T) {
	c := testConventions()
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) { return nil, backlog.ErrNotFound }
	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{Create: true})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.Items[0].Action != ActionCreate {
		t.Fatalf("project action = %s, want create", plan.Items[0].Action)
	}
	if client.GetCallCount("GetProject") != 1 || client.GetCallCount("ListProjectIssueTypes") != 0 || client.GetCallCount("ListProjectStatuses") != 0 || client.GetCallCount("ListProjectCategories") != 0 || client.GetCallCount("ListProjectUsers") != 0 || client.GetCallCount("ListIssues") != 0 {
		t.Fatalf("read calls = project:%d types:%d statuses:%d categories:%d users:%d issues:%d", client.GetCallCount("GetProject"), client.GetCallCount("ListProjectIssueTypes"), client.GetCallCount("ListProjectStatuses"), client.GetCallCount("ListProjectCategories"), client.GetCallCount("ListProjectUsers"), client.GetCallCount("ListIssues"))
	}
}

func TestBuildPlan_ErrorViolationStopsBeforeClient(t *testing.T) {
	c := testConventions()
	c.SchemaVersion++
	client := backlog.NewMockClient()
	if _, err := BuildPlan(context.Background(), client, c, PlanOptions{}); err == nil {
		t.Fatal("error violation でエラーになりませんでした")
	}
	if client.GetCallCount("GetProject") != 0 {
		t.Fatal("validation error 後に GetProject が呼ばれました")
	}
}

func TestBuildPlan_ProjectKeyOverride(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	var gotKey string
	client.GetProjectFunc = func(_ context.Context, key string) (*domain.Project, error) {
		gotKey = key
		return &domain.Project{ID: 1, ProjectKey: key, Name: "Other"}, nil
	}
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) { return testIssueTypes(c), nil }
	client.ListProjectStatusesFunc = func(context.Context, string) ([]domain.Status, error) { return testStatuses(c), nil }
	client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) {
		return []domain.Category{{ID: 20, Name: c.Engagements[0].Name}}, nil
	}
	client.ListProjectUsersFunc = func(context.Context, string, backlog.ListProjectUsersOptions) ([]domain.User, error) {
		return testUsers(), nil
	}
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) { return testIssues(c), nil }
	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{ProjectKey: " OTHER "})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if gotKey != "OTHER" || plan.ProjectKey != "OTHER" {
		t.Fatalf("project key = %q, plan key = %q", gotKey, plan.ProjectKey)
	}
}

func assertPlanErrorContains(t *testing.T, client *backlog.MockClient, c *Conventions, parts ...string) {
	t.Helper()
	_, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err == nil {
		t.Fatal("重複でエラーになりませんでした")
	}
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error = %q, want contains %q", err, part)
		}
	}
}

func TestBuildPlan_ExposesApplyMetadataWithoutJSON(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	plan, err := BuildPlan(context.Background(), client, c, PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.Items[1].targetID != 10 || plan.Items[5].issueKey != "PROJ-1" {
		t.Fatalf("metadata is missing: type=%d issue=%q", plan.Items[1].targetID, plan.Items[5].issueKey)
	}
}

func TestBuildPlan_ListErrorIsReturned(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	want := errors.New("network")
	client.ListProjectStatusesFunc = func(context.Context, string) ([]domain.Status, error) { return nil, want }
	if _, err := BuildPlan(context.Background(), client, c, PlanOptions{}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
