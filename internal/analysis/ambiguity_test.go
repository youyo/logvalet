package analysis

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/conventions"
	"github.com/youyo/logvalet/internal/domain"
)

const ambiguityTestConventions = `
schema_version: 1
initiatives:
  - name: Initiative A
engagements:
  - name: Engagement A
    lead: Lead A
    initiative: Initiative A
`

func ambiguityRuleIssue(description string) domain.Issue {
	return domain.Issue{
		ID:          1,
		IssueKey:    "PROJ-1",
		Summary:     "運用規約",
		IssueType:   &domain.IDName{Name: conventions.IssueTypeRule},
		Description: "```yaml\n" + strings.TrimSpace(description) + "\n```",
	}
}

func ambiguityIssue(issueKey string, now time.Time) domain.Issue {
	return domain.Issue{
		ID:       100,
		IssueKey: issueKey,
		Summary:  "課題 " + issueKey,
		Updated:  timePtr(now),
		Status:   &domain.IDName{Name: "未対応"},
		Priority: &domain.IDName{Name: "中"},
		Assignee: &domain.User{ID: 10, Name: "担当者"},
	}
}

func ambiguityParentIssue(issueKey, engagementName string, assignee *domain.User, dueDate *time.Time) domain.Issue {
	return domain.Issue{
		ID:         200,
		IssueKey:   issueKey,
		Summary:    "[案件] " + engagementName,
		IssueType:  &domain.IDName{Name: conventions.IssueTypeEngagement},
		Assignee:   assignee,
		DueDate:    dueDate,
		Categories: []domain.IDName{{Name: engagementName}},
	}
}

func timePtr(value time.Time) *time.Time { return &value }

func newAmbiguityTestClient(issues []domain.Issue, description string) *backlog.MockClient {
	allIssues := append([]domain.Issue{ambiguityRuleIssue(description)}, issues...)
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ", Name: "テストプロジェクト"}, nil
	}
	mc.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return allIssues, nil
	}
	return mc
}

func detectAmbiguity(t *testing.T, client backlog.Client, now time.Time) *AmbiguityResult {
	t.Helper()
	detector := NewAmbiguityDetector(client, "default", "space", "https://space.backlog.com",
		WithClock(func() time.Time { return now }),
	)
	result, err := detector.Detect(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	return result
}

func TestAmbiguityDetector_NotAdopted(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return &domain.Project{ID: 1, ProjectKey: "PROJ"}, nil
	}
	mc.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return nil, nil
	}

	result := detectAmbiguity(t, mc, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
	if result.Adopted {
		t.Fatal("Adopted = true, want false")
	}
	if result.TotalCount != 0 || len(result.Ambiguities) != 0 {
		t.Fatalf("result = %#v, want an empty result", result)
	}
	if result.Ambiguities == nil {
		t.Fatal("Ambiguities = nil, want an empty list")
	}
}

func TestAmbiguityDetector_Rules(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	assignee := &domain.User{ID: 10, Name: "担当者"}
	dueDate := now.Add(24 * time.Hour)
	parent := ambiguityParentIssue("PROJ-2", "Engagement A", assignee, &dueDate)

	tests := []struct {
		name        string
		description string
		issues      []domain.Issue
		wantKind    AmbiguityKind
		wantTarget  string
	}{
		{
			name:       "案件カテゴリなし",
			issues:     []domain.Issue{parent, ambiguityIssue("PROJ-10", now)},
			wantKind:   AmbiguityNoEngagement,
			wantTarget: "PROJ-10",
		},
		{
			name: "案件カテゴリ複数",
			description: `
schema_version: 1
initiatives:
  - name: Initiative A
engagements:
  - name: Engagement A
    lead: Lead A
    initiative: Initiative A
  - name: Engagement B
    lead: Lead B
    initiative: Initiative A
`,
			issues: []domain.Issue{func() domain.Issue {
				issue := ambiguityIssue("PROJ-10", now)
				issue.Categories = []domain.IDName{{Name: "Engagement A"}, {Name: "Engagement B"}}
				return issue
			}(), parent, ambiguityParentIssue("PROJ-3", "Engagement B", assignee, &dueDate)},
			wantKind:   AmbiguityMultipleEngagements,
			wantTarget: "PROJ-10",
		},
		{
			name:       "案件親課題なし",
			issues:     nil,
			wantKind:   AmbiguityMissingParentIssue,
			wantTarget: "Engagement A",
		},
		{
			name:       "案件親課題の担当者なし",
			issues:     []domain.Issue{ambiguityParentIssue("PROJ-2", "Engagement A", nil, &dueDate)},
			wantKind:   AmbiguityMissingLead,
			wantTarget: "PROJ-2",
		},
		{
			name:       "案件親課題の期限なし",
			issues:     []domain.Issue{ambiguityParentIssue("PROJ-2", "Engagement A", assignee, nil)},
			wantKind:   AmbiguityMissingDueDate,
			wantTarget: "PROJ-2",
		},
		{
			name: "未知のイニシアチブ",
			description: `
schema_version: 1
initiatives:
  - name: Initiative A
engagements:
  - name: Engagement A
    lead: Lead A
    initiative: Missing Initiative
	`,
			issues:     []domain.Issue{parent},
			wantKind:   AmbiguityUnknownInitiative,
			wantTarget: "Engagement A",
		},
		{
			name: "低優先度の長期放置",
			description: `
schema_version: 1
close_policy:
  low_untouched_days: 30
initiatives:
  - name: Initiative A
engagements:
  - name: Engagement A
    lead: Lead A
    initiative: Initiative A
	`,
			issues: func() []domain.Issue {
				issue := ambiguityIssue("PROJ-10", now.Add(-31*24*time.Hour))
				issue.Categories = []domain.IDName{{Name: "Engagement A"}}
				issue.Priority = &domain.IDName{Name: "低"}
				return []domain.Issue{parent, issue}
			}(),
			wantKind:   AmbiguityCloseCandidate,
			wantTarget: "PROJ-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			description := tt.description
			if description == "" {
				description = ambiguityTestConventions
			}
			result := detectAmbiguity(t, newAmbiguityTestClient(tt.issues, description), now)
			if result.TotalCount != 1 || len(result.Ambiguities) != 1 {
				t.Fatalf("result = %#v, want exactly one ambiguity", result)
			}
			got := result.Ambiguities[0]
			if got.Kind != tt.wantKind || got.Target != tt.wantTarget {
				t.Errorf("ambiguity = %#v, want kind=%q target=%q", got, tt.wantKind, tt.wantTarget)
			}
			if got.Summary == "" || got.Detail == "" {
				t.Errorf("ambiguity = %#v, want summary and detail", got)
			}
		})
	}
}

func TestAmbiguityDetector_ExcludesRuleAndEngagementIssues(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	old := now.Add(-31 * 24 * time.Hour)
	parent := ambiguityParentIssue("PROJ-2", "Engagement A", &domain.User{ID: 10}, timePtr(now.Add(24*time.Hour)))
	parent.Priority = &domain.IDName{Name: "低"}
	rule := ambiguityRuleIssue(ambiguityTestConventions)
	rule.Updated = &old
	rule.Priority = &domain.IDName{Name: "低"}

	issue := ambiguityIssue("PROJ-10", old)
	issue.Priority = &domain.IDName{Name: "中"}
	mc := newAmbiguityTestClient([]domain.Issue{parent, issue}, ambiguityTestConventions)
	// Keep the rule's old/low values in the returned issue list as well.
	mc.ListIssuesFunc = func(context.Context, backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{rule, parent, issue}, nil
	}

	result := detectAmbiguity(t, mc, now)
	if result.TotalCount != 1 || result.Ambiguities[0].Kind != AmbiguityNoEngagement {
		t.Fatalf("result = %#v, want only no_engagement for PROJ-10", result)
	}
	if result.Ambiguities[0].Target != "PROJ-10" {
		t.Errorf("target = %q, want PROJ-10", result.Ambiguities[0].Target)
	}
}

func TestAmbiguityDetector_CloseCandidateNilPolicy(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	parent := ambiguityParentIssue("PROJ-2", "Engagement A", &domain.User{ID: 10}, timePtr(now.Add(24*time.Hour)))
	issue := ambiguityIssue("PROJ-10", now.Add(-31*24*time.Hour))
	issue.Categories = []domain.IDName{{Name: "Engagement A"}}
	issue.Priority = &domain.IDName{Name: "低"}

	result := detectAmbiguity(t, newAmbiguityTestClient([]domain.Issue{parent, issue}, ambiguityTestConventions), now)
	if result.TotalCount != 0 || len(result.Ambiguities) != 0 {
		t.Fatalf("result = %#v, want no ambiguity", result)
	}
}

func TestAmbiguityDetector_DeterministicOrder(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	description := `
schema_version: 1
initiatives:
  - name: Initiative A
engagements:
  - name: Beta
    lead: Lead B
    initiative: Initiative A
  - name: Alpha
    lead: Lead A
    initiative: Initiative A
`
	noEngagement20 := ambiguityIssue("PROJ-20", now)
	noEngagement10 := ambiguityIssue("PROJ-10", now)
	multiple30 := ambiguityIssue("PROJ-30", now)
	multiple30.Categories = []domain.IDName{{Name: "Alpha"}, {Name: "Beta"}}
	multiple05 := ambiguityIssue("PROJ-05", now)
	multiple05.Categories = []domain.IDName{{Name: "Alpha"}, {Name: "Beta"}}

	result := detectAmbiguity(t, newAmbiguityTestClient([]domain.Issue{
		noEngagement20, multiple30, noEngagement10, multiple05,
	}, description), now)
	want := []struct {
		kind   AmbiguityKind
		target string
	}{
		{AmbiguityNoEngagement, "PROJ-10"},
		{AmbiguityNoEngagement, "PROJ-20"},
		{AmbiguityMultipleEngagements, "PROJ-05"},
		{AmbiguityMultipleEngagements, "PROJ-30"},
		{AmbiguityMissingParentIssue, "Alpha"},
		{AmbiguityMissingParentIssue, "Beta"},
	}
	got := make([]struct {
		kind   AmbiguityKind
		target string
	}, 0, len(result.Ambiguities))
	for _, ambiguity := range result.Ambiguities {
		got = append(got, struct {
			kind   AmbiguityKind
			target string
		}{ambiguity.Kind, ambiguity.Target})
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestAmbiguityDetector_Zero(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	parent := ambiguityParentIssue("PROJ-2", "Engagement A", &domain.User{ID: 10}, timePtr(now.Add(24*time.Hour)))
	issue := ambiguityIssue("PROJ-10", now)
	issue.Categories = []domain.IDName{{Name: "Engagement A"}}

	result := detectAmbiguity(t, newAmbiguityTestClient([]domain.Issue{parent, issue}, ambiguityTestConventions), now)
	if result.TotalCount != 0 || len(result.Ambiguities) != 0 {
		t.Fatalf("result = %#v, want no ambiguity", result)
	}
}

func TestAmbiguityDetector_Error(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetProjectFunc = func(context.Context, string) (*domain.Project, error) {
		return nil, errors.New("project unavailable")
	}
	detector := NewAmbiguityDetector(mc, "default", "space", "https://space.backlog.com")
	if _, err := detector.Detect(context.Background(), "PROJ"); err == nil {
		t.Fatal("Detect() error = nil, want error")
	}
}
