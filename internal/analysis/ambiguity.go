package analysis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/conventions"
	"github.com/youyo/logvalet/internal/domain"
)

// AmbiguityKind は曖昧さの種類。
type AmbiguityKind string

const (
	// AmbiguityNoEngagement は案件カテゴリを持たない課題。Linear でいう「溜まり」。
	AmbiguityNoEngagement AmbiguityKind = "no_engagement"
	// AmbiguityMultipleEngagements は案件カテゴリを 2 つ以上持つ課題。
	AmbiguityMultipleEngagements AmbiguityKind = "multiple_engagements"
	// AmbiguityMissingParentIssue は案件親課題が存在しない案件カテゴリ。
	AmbiguityMissingParentIssue AmbiguityKind = "missing_parent_issue"
	// AmbiguityMissingLead は担当者のいない案件親課題。
	AmbiguityMissingLead AmbiguityKind = "missing_lead"
	// AmbiguityMissingDueDate は期限のない案件親課題。
	AmbiguityMissingDueDate AmbiguityKind = "missing_due_date"
	// AmbiguityUnknownInitiative は conventions の Initiative に紐づかない案件。
	AmbiguityUnknownInitiative AmbiguityKind = "unknown_initiative"
	// AmbiguityCloseCandidate は長期間放置された低優先度課題。クローズも決断のうち。
	AmbiguityCloseCandidate AmbiguityKind = "close_candidate"
)

// Ambiguity は規約に照らして決まっていないこと 1 件。
type Ambiguity struct {
	Kind    AmbiguityKind `json:"kind"`
	Target  string        `json:"target"`
	Summary string        `json:"summary"`
	Detail  string        `json:"detail"`
}

// AmbiguityResult は曖昧さ検知の結果。
type AmbiguityResult struct {
	Adopted     bool        `json:"adopted"`
	TotalCount  int         `json:"total_count"`
	Ambiguities []Ambiguity `json:"ambiguities"`
}

// AmbiguityDetector は運用規約に照らした曖昧さを検知する。
type AmbiguityDetector struct {
	BaseAnalysisBuilder
}

// NewAmbiguityDetector は AmbiguityDetector を生成する。
func NewAmbiguityDetector(client backlog.Client, profile, space, baseURL string, opts ...Option) *AmbiguityDetector {
	return &AmbiguityDetector{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Detect は指定プロジェクトの曖昧さを検知する。
// 規約が未導入なら Adopted: false と空のリストを返す。
func (d *AmbiguityDetector) Detect(ctx context.Context, projectKey string) (*AmbiguityResult, error) {
	show, err := conventions.Show(ctx, d.client, projectKey)
	if err != nil {
		return nil, fmt.Errorf("規約の取得に失敗しました: %w", err)
	}

	result := &AmbiguityResult{
		Adopted:     show.Adopted,
		Ambiguities: []Ambiguity{},
	}
	if !show.Adopted {
		return result, nil
	}
	if show.Conventions == nil {
		return nil, fmt.Errorf("規約が導入済みですが内容がありません")
	}

	project, err := d.client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("プロジェクト %q の取得に失敗しました: %w", projectKey, err)
	}
	issues, err := d.client.ListIssues(ctx, backlog.ListIssuesOptions{ProjectIDs: []int{project.ID}})
	if err != nil {
		return nil, fmt.Errorf("プロジェクト %q の課題取得に失敗しました: %w", projectKey, err)
	}

	c := show.Conventions
	engagementNames := make(map[string]struct{}, len(c.Engagements))
	initiativeNames := make(map[string]struct{}, len(c.Initiatives))
	for _, initiative := range c.Initiatives {
		initiativeNames[normalizeAmbiguityName(initiative.Name)] = struct{}{}
	}
	for _, engagement := range c.Engagements {
		engagementNames[normalizeAmbiguityName(engagement.Name)] = struct{}{}
	}

	now := d.now()
	for i := range issues {
		issue := &issues[i]
		if isRuleOrEngagementIssue(issue) {
			continue
		}

		matchedEngagements := matchingEngagementCategories(issue.Categories, engagementNames)
		switch len(matchedEngagements) {
		case 0:
			result.Ambiguities = append(result.Ambiguities, issueAmbiguity(
				AmbiguityNoEngagement, issue,
				"案件カテゴリが付いていません",
			))
		case 1:
			// 案件カテゴリがちょうど1つなら、この課題については明確。
		default:
			result.Ambiguities = append(result.Ambiguities, issueAmbiguity(
				AmbiguityMultipleEngagements, issue,
				fmt.Sprintf("案件カテゴリが %d つ付いています", len(matchedEngagements)),
			))
		}

		if isCloseCandidate(issue, c.ClosePolicy.LowUntouchedDays, now) {
			result.Ambiguities = append(result.Ambiguities, issueAmbiguity(
				AmbiguityCloseCandidate, issue,
				"低優先度のまま長期間更新されていません",
			))
		}
	}

	for _, engagement := range c.Engagements {
		name := normalizeAmbiguityName(engagement.Name)
		parentSummary := "[案件] " + name
		parents := matchingParentIssues(issues, parentSummary)
		if len(parents) == 0 {
			result.Ambiguities = append(result.Ambiguities, Ambiguity{
				Kind:    AmbiguityMissingParentIssue,
				Target:  name,
				Summary: parentSummary,
				Detail:  fmt.Sprintf("案件「%s」に対応する親課題がありません", name),
			})
		} else {
			for _, parent := range parents {
				if parent.Assignee == nil {
					result.Ambiguities = append(result.Ambiguities, issueAmbiguity(
						AmbiguityMissingLead, &parent,
						"案件親課題に担当者がいません",
					))
				}
				if parent.DueDate == nil {
					result.Ambiguities = append(result.Ambiguities, issueAmbiguity(
						AmbiguityMissingDueDate, &parent,
						"案件親課題に期限日がありません",
					))
				}
			}
		}

		if _, ok := initiativeNames[normalizeAmbiguityName(engagement.Initiative)]; !ok {
			result.Ambiguities = append(result.Ambiguities, Ambiguity{
				Kind:    AmbiguityUnknownInitiative,
				Target:  name,
				Summary: engagement.Initiative,
				Detail:  fmt.Sprintf("案件「%s」のイニシアチブ「%s」が定義されていません", name, engagement.Initiative),
			})
		}
	}

	sortAmbiguities(result.Ambiguities)
	result.TotalCount = len(result.Ambiguities)
	return result, nil
}

func normalizeAmbiguityName(value string) string {
	return strings.TrimSpace(value)
}

func isRuleOrEngagementIssue(issue *domain.Issue) bool {
	if issue.IssueType == nil {
		return false
	}
	name := normalizeAmbiguityName(issue.IssueType.Name)
	return name == conventions.IssueTypeRule || name == conventions.IssueTypeEngagement
}

func matchingEngagementCategories(categories []domain.IDName, engagementNames map[string]struct{}) []string {
	matched := make([]string, 0, len(categories))
	seen := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		name := normalizeAmbiguityName(category.Name)
		if _, ok := engagementNames[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		matched = append(matched, name)
	}
	return matched
}

func matchingParentIssues(issues []domain.Issue, summary string) []domain.Issue {
	parents := make([]domain.Issue, 0)
	for _, issue := range issues {
		if issue.IssueType == nil || normalizeAmbiguityName(issue.IssueType.Name) != conventions.IssueTypeEngagement {
			continue
		}
		if issue.Summary == summary {
			parents = append(parents, issue)
		}
	}
	return parents
}

func issueAmbiguity(kind AmbiguityKind, issue *domain.Issue, detail string) Ambiguity {
	return Ambiguity{
		Kind:    kind,
		Target:  issue.IssueKey,
		Summary: issue.Summary,
		Detail:  detail,
	}
}

func isCloseCandidate(issue *domain.Issue, lowUntouchedDays *int, now time.Time) bool {
	if lowUntouchedDays == nil || issue.Updated == nil || issue.Priority == nil {
		return false
	}
	if issue.Priority.Name != "低" {
		return false
	}
	return now.Sub(*issue.Updated) > time.Duration(*lowUntouchedDays)*24*time.Hour
}

var ambiguityKindOrder = map[AmbiguityKind]int{
	AmbiguityNoEngagement:        0,
	AmbiguityMultipleEngagements: 1,
	AmbiguityMissingParentIssue:  2,
	AmbiguityMissingLead:         3,
	AmbiguityMissingDueDate:      4,
	AmbiguityUnknownInitiative:   5,
	AmbiguityCloseCandidate:      6,
}

func sortAmbiguities(ambiguities []Ambiguity) {
	sort.SliceStable(ambiguities, func(i, j int) bool {
		leftOrder := ambiguityKindOrder[ambiguities[i].Kind]
		rightOrder := ambiguityKindOrder[ambiguities[j].Kind]
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if ambiguities[i].Target != ambiguities[j].Target {
			return ambiguities[i].Target < ambiguities[j].Target
		}
		return ambiguities[i].Summary < ambiguities[j].Summary
	})
}
