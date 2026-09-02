package conventions

import (
	"context"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestResolveEngagement_ResolvesCategoryAndParentIssue(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)

	got, err := ResolveEngagement(context.Background(), client, "PROJ", "  "+c.Engagements[0].Name+"  ")
	if err != nil {
		t.Fatalf("ResolveEngagement() error = %v", err)
	}

	want := &EngagementRef{
		Name:          c.Engagements[0].Name,
		CategoryID:    20,
		ParentIssueID: 101,
	}
	if *got != *want {
		t.Fatalf("ResolveEngagement() = %#v, want %#v", got, want)
	}
}

func TestResolveEngagement_WithoutParentIssueSucceedsWithZeroID(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	ruleIssue := testIssues(c)[0]
	client.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{ruleIssue}, nil
	}

	got, err := ResolveEngagement(context.Background(), client, "PROJ", c.Engagements[0].Name)
	if err != nil {
		t.Fatalf("ResolveEngagement() error = %v", err)
	}
	if got.ParentIssueID != 0 {
		t.Errorf("ParentIssueID = %d, want 0", got.ParentIssueID)
	}
}

func TestResolveEngagement_NotAdoptedReturnsError(t *testing.T) {
	client := showTestClient()

	_, err := ResolveEngagement(context.Background(), client, "PROJ", "顧客A")
	if err == nil {
		t.Fatal("ResolveEngagement() error = nil, want not-adopted error")
	}
	if !strings.Contains(err.Error(), "プロジェクト \"PROJ\" には運用規約が導入されていません") {
		t.Errorf("ResolveEngagement() error = %v, want not-adopted message", err)
	}
}

func TestResolveEngagement_UnknownEngagementReturnsAvailableNames(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)

	_, err := ResolveEngagement(context.Background(), client, "PROJ", "存在しない案件")
	if err == nil {
		t.Fatal("ResolveEngagement() error = nil, want unknown-engagement error")
	}
	if !strings.Contains(err.Error(), "案件 \"存在しない案件\" は規約にありません") {
		t.Errorf("ResolveEngagement() error = %v, want unknown-engagement message", err)
	}
	if !strings.Contains(err.Error(), c.Engagements[0].Name) {
		t.Errorf("ResolveEngagement() error = %v, want available engagement name", err)
	}
}

func TestResolveEngagement_MissingCategoryReturnsError(t *testing.T) {
	c := testConventions()
	client := configuredExistingClient(c)
	client.ListProjectCategoriesFunc = func(context.Context, string) ([]domain.Category, error) {
		return nil, nil
	}

	_, err := ResolveEngagement(context.Background(), client, "PROJ", c.Engagements[0].Name)
	if err == nil {
		t.Fatal("ResolveEngagement() error = nil, want missing-category error")
	}
	if !strings.Contains(err.Error(), "案件 \""+c.Engagements[0].Name+"\" に対応するカテゴリがプロジェクトにありません") {
		t.Errorf("ResolveEngagement() error = %v, want missing-category message", err)
	}
}

func TestCountEngagementCategories(t *testing.T) {
	c := testConventions()
	name := c.Engagements[0].Name

	tests := []struct {
		name       string
		categories []domain.IDName
		want       int
	}{
		{name: "zero"},
		{name: "one", categories: []domain.IDName{{ID: 20, Name: "  " + name + "  "}}, want: 1},
		{name: "two", categories: []domain.IDName{{ID: 20, Name: name}, {ID: 21, Name: name}}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountEngagementCategories(c, tt.categories); got != tt.want {
				t.Errorf("CountEngagementCategories() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountEngagementCategories_NilConventionsReturnsZero(t *testing.T) {
	categories := []domain.IDName{{ID: 20, Name: "顧客A 基盤更改"}}
	if got := CountEngagementCategories(nil, categories); got != 0 {
		t.Errorf("CountEngagementCategories(nil, categories) = %d, want 0", got)
	}
}
