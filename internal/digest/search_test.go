package digest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestSearchBuilder_Build_AllResources(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{{ID: 1, ProjectKey: "PROJ", Name: "Project"}}, nil
	}
	var capturedIssueOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		capturedIssueOpt = opt
		return []domain.Issue{{
			ID:          10,
			ProjectID:   1,
			IssueKey:    "PROJ-10",
			Summary:     "OAuth issue",
			Description: "OAuth error happens on callback",
		}}, nil
	}
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		return []domain.Document{{
			ID:        "doc-1",
			ProjectID: 1,
			Title:     "OAuth document",
			Plain:     "OAuth setup guide",
		}}, nil
	}
	mock.ListWikisFunc = func(ctx context.Context, projectKey string, opt backlog.ListWikisOptions) ([]domain.WikiPage, error) {
		if projectKey != "PROJ" {
			t.Errorf("projectKey = %q, want PROJ", projectKey)
		}
		if opt.Keyword != "OAuth" {
			t.Errorf("wiki keyword = %q, want OAuth", opt.Keyword)
		}
		return []domain.WikiPage{{
			ID:        20,
			ProjectID: 1,
			Name:      "OAuth Wiki",
			Content:   "OAuth wiki body",
		}}, nil
	}

	builder := NewDefaultSearchBuilder(mock, "work", "space", "https://example.backlog.com")
	env, err := builder.Build(context.Background(), SearchOptions{Keyword: "OAuth", Count: 10})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if env.Resource != "search" {
		t.Errorf("Resource = %q, want search", env.Resource)
	}
	if capturedIssueOpt.Keyword != "OAuth" {
		t.Errorf("issue keyword = %q, want OAuth", capturedIssueOpt.Keyword)
	}

	var d SearchDigest
	b, err := json.Marshal(env.Digest)
	if err != nil {
		t.Fatalf("Marshal digest: %v", err)
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("Unmarshal SearchDigest: %v", err)
	}
	if d.TotalReturned != 3 {
		t.Errorf("TotalReturned = %d, want 3", d.TotalReturned)
	}
	if d.ReturnedByType.Issues != 1 || d.ReturnedByType.Documents != 1 || d.ReturnedByType.Wikis != 1 {
		t.Errorf("ReturnedByType = %+v, want 1 each", d.ReturnedByType)
	}
	if len(d.Items) != 3 {
		t.Fatalf("Items len = %d, want 3", len(d.Items))
	}
	if d.Items[0].ResourceType != "issue" || d.Items[1].ResourceType != "document" || d.Items[2].ResourceType != "wiki" {
		t.Errorf("resource order = %s,%s,%s", d.Items[0].ResourceType, d.Items[1].ResourceType, d.Items[2].ResourceType)
	}
}

func TestSearchBuilder_Build_ProjectFilterResolvesKeys(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 7, ProjectKey: projectKey}, nil
	}
	var issueOpt backlog.ListIssuesOptions
	mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		issueOpt = opt
		return []domain.Issue{}, nil
	}
	var docOpt backlog.SearchDocumentsOptions
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		docOpt = opt
		return []domain.Document{}, nil
	}
	mock.ListWikisFunc = func(ctx context.Context, projectKey string, opt backlog.ListWikisOptions) ([]domain.WikiPage, error) {
		return []domain.WikiPage{}, nil
	}

	builder := NewDefaultSearchBuilder(mock, "", "", "")
	_, err := builder.Build(context.Background(), SearchOptions{Keyword: "auth", ProjectKeys: []string{"PROJ"}})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(issueOpt.ProjectIDs) != 1 || issueOpt.ProjectIDs[0] != 7 {
		t.Errorf("issue ProjectIDs = %v, want [7]", issueOpt.ProjectIDs)
	}
	if len(docOpt.ProjectIDs) != 1 || docOpt.ProjectIDs[0] != 7 {
		t.Errorf("document ProjectIDs = %v, want [7]", docOpt.ProjectIDs)
	}
}
