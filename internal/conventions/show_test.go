package conventions

import (
	"context"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

const showTestYAML = `schema_version: 1
project:
  key: PROJ
  name: Project
`

func showTestClient() *backlog.MockClient {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return &domain.Project{ID: 42, ProjectKey: "PROJ", Name: "Project"}, nil
	}
	client.ListProjectIssueTypesFunc = func(context.Context, string) ([]domain.IssueType, error) {
		return []domain.IssueType{{ID: 7, Name: IssueTypeRule}}, nil
	}
	return client
}

func showTestIssue(description string, key string) domain.Issue {
	return domain.Issue{
		IssueKey:    key,
		Description: description,
		IssueType:   &domain.IDName{Name: IssueTypeRule},
	}
}

func TestShow_NoRuleIssueReturnsNotAdoptedWithGlossary(t *testing.T) {
	client := showTestClient()
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	got, err := Show(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if got.Adopted {
		t.Error("Adopted = true, want false")
	}
	if got.Conventions != nil {
		t.Fatalf("Conventions = %#v, want nil", got.Conventions)
	}
	if got.IssueKey != "" {
		t.Errorf("IssueKey = %q, want empty", got.IssueKey)
	}
	if len(got.Glossary) == 0 {
		t.Fatal("Glossary is empty")
	}
}

func TestShow_OneRuleIssueLoadsConventions(t *testing.T) {
	client := showTestClient()
	description := BuildRuleIssueDescription([]byte(showTestYAML))
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{showTestIssue(description, "PROJ-1")}, nil
	}

	got, err := Show(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if !got.Adopted || got.Conventions == nil {
		t.Fatalf("Show() result = %#v, want adopted conventions", got)
	}
	if got.ProjectKey != "PROJ" || got.IssueKey != "PROJ-1" {
		t.Errorf("keys = (%q, %q), want (PROJ, PROJ-1)", got.ProjectKey, got.IssueKey)
	}
	if got.Conventions.Project.Key != "PROJ" {
		t.Errorf("conventions project key = %q, want PROJ", got.Conventions.Project.Key)
	}
	if len(got.Glossary) == 0 {
		t.Fatal("Glossary is empty")
	}
}

func TestShow_MultipleRuleIssuesReturnsError(t *testing.T) {
	client := showTestClient()
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{
			showTestIssue("first", "PROJ-1"),
			showTestIssue("second", "PROJ-2"),
		}, nil
	}

	if _, err := Show(context.Background(), client, "PROJ"); err == nil {
		t.Fatal("Show() error = nil, want duplicate rule issue error")
	} else if !strings.Contains(err.Error(), "複数") {
		t.Errorf("Show() error = %v, want duplicate error", err)
	}
}

func TestShow_RuleIssueWithoutYAMLReturnsError(t *testing.T) {
	client := showTestClient()
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{showTestIssue("規約本文のみ", "PROJ-1")}, nil
	}

	if _, err := Show(context.Background(), client, "PROJ"); err == nil {
		t.Fatal("Show() error = nil, want YAML parsing error")
	} else if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("Show() error = %v, want YAML error", err)
	}
}
