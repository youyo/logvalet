package analysis

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// StaleConfig は停滞判定の設定。
type StaleConfig struct {
	DefaultDays   int            // デフォルト閾値（日数）。0以下の場合 DefaultStaleDays を使用。
	StatusDays    map[string]int // ステータス名→独自閾値（例: "処理中": 3）
	ExcludeStatus []string       // 除外ステータス名（例: "完了", "対応済み"）
}

// StaleIssueResult は停滞課題検出の結果。
type StaleIssueResult struct {
	Issues        []StaleIssue          `json:"issues"`
	TotalCount    int                   `json:"total_count"`
	ThresholdDays int                   `json:"threshold_days"`
	LLMHints      digest.DigestLLMHints `json:"llm_hints"`
}

// StaleIssue は停滞と判定された個別課題。
type StaleIssue struct {
	IssueKey        string          `json:"issue_key"`
	Summary         string          `json:"summary"`
	Status          string          `json:"status"`
	Assignee        *domain.UserRef `json:"assignee,omitempty"`
	DaysSinceUpdate int             `json:"days_since_update"`
	LastUpdated     *time.Time      `json:"last_updated,omitempty"`
	DueDate         *time.Time      `json:"due_date,omitempty"`
	IsOverdue       bool            `json:"is_overdue"`
}

// StaleIssueDetector は停滞課題を検出する。
type StaleIssueDetector struct {
	BaseAnalysisBuilder
}

// NewStaleIssueDetector は StaleIssueDetector を生成する。
func NewStaleIssueDetector(client backlog.Client, profile, space, baseURL string, opts ...Option) *StaleIssueDetector {
	return &StaleIssueDetector{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Detect は指定プロジェクトの停滞課題を検出する。
func (d *StaleIssueDetector) Detect(ctx context.Context, projectKeys []string, config StaleConfig) (*AnalysisEnvelope, error) {
	// DefaultDays のフォールバック
	defaultDays := config.DefaultDays
	if defaultDays <= 0 {
		defaultDays = DefaultStaleDays
	}

	// ExcludeStatus のデフォルト適用（未指定時は「完了」を除外）
	if len(config.ExcludeStatus) == 0 {
		config.ExcludeStatus = DefaultExcludeStatus
	}

	now := d.now()
	var staleIssues []StaleIssue
	var warnings []domain.Warning

	// ExcludeStatus を set に変換
	excludeSet := buildExcludeSet(config.ExcludeStatus)

	// プロジェクトごとに GetProject → ListIssues
	for _, projectKey := range projectKeys {
		issues, ws := d.fetchProjectIssues(ctx, projectKey)
		warnings = append(warnings, ws...)
		if issues == nil {
			continue
		}

		for i := range issues {
			si, ok := classifyIssue(&issues[i], now, defaultDays, config.StatusDays, excludeSet)
			if ok {
				staleIssues = append(staleIssues, si)
			}
		}
	}

	// nil → 空スライス
	if staleIssues == nil {
		staleIssues = []StaleIssue{}
	}

	// DaysSinceUpdate 降順ソート
	sort.Slice(staleIssues, func(i, j int) bool {
		return staleIssues[i].DaysSinceUpdate > staleIssues[j].DaysSinceUpdate
	})

	result := &StaleIssueResult{
		Issues:        staleIssues,
		TotalCount:    len(staleIssues),
		ThresholdDays: defaultDays,
		LLMHints:      buildStaleLLMHints(projectKeys, staleIssues),
	}

	return d.newEnvelope("stale_issues", result, warnings), nil
}

// fetchProjectIssues はプロジェクトの課題一覧を取得する。エラー時は warnings を返す。
func (d *StaleIssueDetector) fetchProjectIssues(ctx context.Context, projectKey string) ([]domain.Issue, []domain.Warning) {
	project, err := d.client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, []domain.Warning{{
			Code:      "project_fetch_failed",
			Message:   fmt.Sprintf("failed to get project %s: %v", projectKey, err),
			Component: "project",
			Retryable: true,
		}}
	}

	issues, err := d.client.ListIssues(ctx, backlog.ListIssuesOptions{
		ProjectIDs: []int{project.ID},
	})
	if err != nil {
		return nil, []domain.Warning{{
			Code:      "issues_fetch_failed",
			Message:   fmt.Sprintf("failed to list issues for project %s: %v", projectKey, err),
			Component: "issues",
			Retryable: true,
		}}
	}

	return issues, nil
}

// classifyIssue は個別課題を stale 判定し、該当する場合は StaleIssue を返す。
func classifyIssue(issue *domain.Issue, now time.Time, defaultDays int, statusDays map[string]int, excludeSet map[string]bool) (StaleIssue, bool) {
	// Updated が nil → スキップ
	if issue.Updated == nil {
		return StaleIssue{}, false
	}

	// ステータス名を取得（nil 安全）
	statusName := ""
	if issue.Status != nil {
		statusName = issue.Status.Name
	}

	// ExcludeStatus チェック（Status nil の場合はスキップしない）
	if statusName != "" && excludeSet[statusName] {
		return StaleIssue{}, false
	}

	// threshold 決定: StatusDays[statusName] || defaultDays
	threshold := defaultDays
	if statusName != "" && statusDays != nil {
		if days, ok := statusDays[statusName]; ok {
			threshold = days
		}
	}

	// daysSinceUpdate 計算
	daysSinceUpdate := int(now.Sub(*issue.Updated).Hours() / 24)

	// stale 判定
	if daysSinceUpdate < threshold {
		return StaleIssue{}, false
	}

	// IsOverdue 判定
	isOverdue := issue.DueDate != nil && issue.DueDate.Before(now)

	return StaleIssue{
		IssueKey:        issue.IssueKey,
		Summary:         issue.Summary,
		Status:          statusName,
		Assignee:        toUserRef(issue.Assignee),
		DaysSinceUpdate: daysSinceUpdate,
		LastUpdated:     issue.Updated,
		DueDate:         issue.DueDate,
		IsOverdue:       isOverdue,
	}, true
}

// buildExcludeSet は ExcludeStatus スライスを set（map）に変換する。
func buildExcludeSet(excludeStatus []string) map[string]bool {
	set := make(map[string]bool, len(excludeStatus))
	for _, s := range excludeStatus {
		set[s] = true
	}
	return set
}

// buildStaleLLMHints は stale 課題検出結果から LLMHints を生成する。
func buildStaleLLMHints(projectKeys []string, staleIssues []StaleIssue) digest.DigestLLMHints {
	primaryEntities := make([]string, 0, len(projectKeys))
	for _, pk := range projectKeys {
		primaryEntities = append(primaryEntities, fmt.Sprintf("project:%s", pk))
	}

	openQuestions := []string{}
	if len(staleIssues) > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d件の停滞課題があります。対応状況を確認してください", len(staleIssues)))
	}

	// overdue な stale 課題がある場合
	overdueCount := 0
	for _, si := range staleIssues {
		if si.IsOverdue {
			overdueCount++
		}
	}
	if overdueCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d件の停滞課題が期限超過しています", overdueCount))
	}

	return digest.DigestLLMHints{
		PrimaryEntities:      primaryEntities,
		OpenQuestions:        openQuestions,
		SuggestedNextActions: []string{},
	}
}
