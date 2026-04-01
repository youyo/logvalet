package analysis

import (
	"context"
	"fmt"
	"sort"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// デフォルト負荷レベル閾値
const (
	DefaultOverloadedThreshold = 20
	DefaultHighThreshold       = 10
	DefaultMediumThreshold     = 5
)

// WorkloadConfig はワークロード計算の設定。
type WorkloadConfig struct {
	// StaleDays は停滞判定の閾値（日数）。0以下の場合 DefaultStaleDays を使用。
	StaleDays int
	// ExcludeStatus は除外ステータス名（例: "完了", "対応済み"）。
	ExcludeStatus []string
	// OverloadedThreshold は "overloaded" 判定の閾値（デフォルト: DefaultOverloadedThreshold）。
	OverloadedThreshold int
	// HighThreshold は "high" 判定の閾値（デフォルト: DefaultHighThreshold）。
	HighThreshold int
	// MediumThreshold は "medium" 判定の閾値（デフォルト: DefaultMediumThreshold）。
	MediumThreshold int
}

// WorkloadResult はワークロード計算の結果。
type WorkloadResult struct {
	ProjectKey      string                `json:"project_key"`
	TotalIssues     int                   `json:"total_issues"`
	UnassignedCount int                   `json:"unassigned_count"`
	StaleDays       int                   `json:"stale_threshold_days"`
	Members         []MemberWorkload      `json:"members"`
	LLMHints        digest.DigestLLMHints `json:"llm_hints"`
}

// MemberWorkload は個別メンバーのワークロード情報。
type MemberWorkload struct {
	UserID     int            `json:"user_id"`
	Name       string         `json:"name"`
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"by_status"`
	ByPriority map[string]int `json:"by_priority"`
	Overdue    int            `json:"overdue"`
	Stale      int            `json:"stale"`
	LoadLevel  string         `json:"load_level"` // "low" | "medium" | "high" | "overloaded"
}

// WorkloadCalculator はユーザーごとの課題負荷を計算する。
type WorkloadCalculator struct {
	BaseAnalysisBuilder
}

// NewWorkloadCalculator は WorkloadCalculator を生成する。
func NewWorkloadCalculator(client backlog.Client, profile, space, baseURL string, opts ...Option) *WorkloadCalculator {
	return &WorkloadCalculator{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Calculate は指定プロジェクトのワークロードを計算する。
func (c *WorkloadCalculator) Calculate(ctx context.Context, projectKey string, config WorkloadConfig) (*AnalysisEnvelope, error) {
	// デフォルト値の解決
	staleDays := config.StaleDays
	if staleDays <= 0 {
		staleDays = DefaultStaleDays
	}

	// ExcludeStatus のデフォルト適用（未指定時は「完了」を除外）
	if len(config.ExcludeStatus) == 0 {
		config.ExcludeStatus = DefaultExcludeStatus
	}

	overloadedThreshold := config.OverloadedThreshold
	if overloadedThreshold <= 0 {
		overloadedThreshold = DefaultOverloadedThreshold
	}
	highThreshold := config.HighThreshold
	if highThreshold <= 0 {
		highThreshold = DefaultHighThreshold
	}
	mediumThreshold := config.MediumThreshold
	if mediumThreshold <= 0 {
		mediumThreshold = DefaultMediumThreshold
	}

	excludeSet := buildExcludeSet(config.ExcludeStatus)
	now := c.now()

	var warnings []domain.Warning

	// emptyResult はエラー時に返す空の WorkloadResult を生成するヘルパー。
	emptyResult := func() *WorkloadResult {
		return &WorkloadResult{
			ProjectKey: projectKey,
			StaleDays:  staleDays,
			Members:    []MemberWorkload{},
			LLMHints:   buildWorkloadLLMHints(projectKey, []MemberWorkload{}),
		}
	}

	// GetProject
	project, err := c.client.GetProject(ctx, projectKey)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "project_fetch_failed",
			Message:   fmt.Sprintf("failed to get project %s: %v", projectKey, err),
			Component: "project",
			Retryable: true,
		})
		return c.newEnvelope("user_workload", emptyResult(), warnings), nil
	}

	// ListIssues
	issues, err := c.client.ListIssues(ctx, backlog.ListIssuesOptions{
		ProjectIDs: []int{project.ID},
	})
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "issues_fetch_failed",
			Message:   fmt.Sprintf("failed to list issues for project %s: %v", projectKey, err),
			Component: "issues",
			Retryable: true,
		})
		return c.newEnvelope("user_workload", emptyResult(), warnings), nil
	}

	// 課題を走査してメンバー別集計
	memberMap := map[int]*MemberWorkload{}
	totalIssues := 0
	unassignedCount := 0

	for i := range issues {
		issue := &issues[i]

		// ExcludeStatus チェック
		statusName := ""
		if issue.Status != nil {
			statusName = issue.Status.Name
		}
		if statusName != "" && excludeSet[statusName] {
			continue
		}

		totalIssues++

		// 担当者なし
		if issue.Assignee == nil {
			unassignedCount++
			continue
		}

		userID := issue.Assignee.ID
		mw, ok := memberMap[userID]
		if !ok {
			mw = &MemberWorkload{
				UserID:     userID,
				Name:       issue.Assignee.Name,
				ByStatus:   map[string]int{},
				ByPriority: map[string]int{},
			}
			memberMap[userID] = mw
		}

		mw.Total++

		// ByStatus
		if statusName != "" {
			mw.ByStatus[statusName]++
		}

		// ByPriority
		if issue.Priority != nil {
			mw.ByPriority[issue.Priority.Name]++
		}

		// Overdue 判定
		if issue.DueDate != nil && issue.DueDate.Before(now) {
			mw.Overdue++
		}

		// Stale 判定（Updated != nil の場合のみ）
		if issue.Updated != nil {
			daysSince := int(now.Sub(*issue.Updated).Hours() / 24)
			if daysSince >= staleDays {
				mw.Stale++
			}
		}
	}

	// MemberWorkload スライス化（UserID 昇順ソート）
	members := make([]MemberWorkload, 0, len(memberMap))
	for _, mw := range memberMap {
		mw.LoadLevel = calcLoadLevel(mw.Total, overloadedThreshold, highThreshold, mediumThreshold)
		members = append(members, *mw)
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].UserID < members[j].UserID
	})

	result := &WorkloadResult{
		ProjectKey:      projectKey,
		TotalIssues:     totalIssues,
		UnassignedCount: unassignedCount,
		StaleDays:       staleDays,
		Members:         members,
		LLMHints:        buildWorkloadLLMHints(projectKey, members),
	}

	return c.newEnvelope("user_workload", result, warnings), nil
}

// calcLoadLevel はメンバーの課題数から負荷レベルを計算する。
func calcLoadLevel(total, overloaded, high, medium int) string {
	switch {
	case total >= overloaded:
		return "overloaded"
	case total >= high:
		return "high"
	case total >= medium:
		return "medium"
	default:
		return "low"
	}
}

// buildWorkloadLLMHints はワークロード計算結果から LLMHints を生成する。
func buildWorkloadLLMHints(projectKey string, members []MemberWorkload) digest.DigestLLMHints {
	primaryEntities := []string{fmt.Sprintf("project:%s", projectKey)}

	openQuestions := []string{}

	highLoadCount := 0
	overloadedCount := 0
	for _, m := range members {
		if m.LoadLevel == "high" {
			highLoadCount++
		}
		if m.LoadLevel == "overloaded" {
			overloadedCount++
		}
	}

	if overloadedCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d人のメンバーがoverloadedです。タスクの再配分を検討してください", overloadedCount))
	}
	if highLoadCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d人のメンバーが高負荷（high）です", highLoadCount))
	}

	return digest.DigestLLMHints{
		PrimaryEntities:      primaryEntities,
		OpenQuestions:        openQuestions,
		SuggestedNextActions: []string{},
	}
}
