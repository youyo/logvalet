package analysis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"golang.org/x/sync/errgroup"
)

// defaultClosedStatus はデフォルトの完了系ステータス名リスト。
var defaultClosedStatus = []string{"完了", "対応済み", "Closed", "Done", "Resolved"}

// unassignedLabel は未割当の担当者ラベル。
const unassignedLabel = "未割当"

// TriageMaterialsOptions は TriageMaterialsBuilder.Build() のオプション。
type TriageMaterialsOptions struct {
	// ClosedStatus は完了とみなすステータス名リスト。
	// 空の場合は defaultClosedStatus を使用。
	ClosedStatus []string
}

// TriageMaterials は triage 用材料の全体構造。
type TriageMaterials struct {
	Issue         TriageIssue        `json:"issue"`
	History       TriageHistory      `json:"history"`
	ProjectStats  TriageProjectStats `json:"project_stats"`
	SimilarIssues TriageSimilar      `json:"similar_issues"`
}

// TriageIssue は対象課題の基本属性。
type TriageIssue struct {
	IssueKey   string          `json:"issue_key"`
	Summary    string          `json:"summary"`
	Status     *domain.IDName  `json:"status,omitempty"`
	Priority   *domain.IDName  `json:"priority,omitempty"`
	IssueType  *domain.IDName  `json:"issue_type,omitempty"`
	Assignee   *domain.UserRef `json:"assignee,omitempty"`
	Reporter   *domain.UserRef `json:"reporter,omitempty"`
	Categories []domain.IDName `json:"categories"`
	Milestones []domain.IDName `json:"milestones"`
	DueDate    *time.Time      `json:"due_date,omitempty"`
	Created    *time.Time      `json:"created,omitempty"`
	Updated    *time.Time      `json:"updated,omitempty"`
}

// TriageHistory は課題の履歴サマリー。
type TriageHistory struct {
	CommentCount     int  `json:"comment_count"`
	DaysSinceCreated int  `json:"days_since_created"`
	DaysSinceUpdated int  `json:"days_since_updated"`
	IsOverdue        bool `json:"is_overdue"`
	IsStale          bool `json:"is_stale"`
}

// TriageProjectStats はプロジェクト全体の統計情報。
type TriageProjectStats struct {
	TotalIssues  int            `json:"total_issues"`
	ByStatus     map[string]int `json:"by_status"`
	ByPriority   map[string]int `json:"by_priority"`
	ByAssignee   map[string]int `json:"by_assignee"`
	AvgCloseDays float64        `json:"avg_close_days"`
}

// TriageSimilar は類似課題の分布情報。
type TriageSimilar struct {
	SameCategoryCount    int            `json:"same_category_count"`
	SameMilestoneCount   int            `json:"same_milestone_count"`
	PriorityDistribution map[string]int `json:"priority_distribution"`
	AssigneeDistribution map[string]int `json:"assignee_distribution"`
}

// TriageMaterialsBuilder は triage 材料を収集・構造化する。
type TriageMaterialsBuilder struct {
	BaseAnalysisBuilder
}

// NewTriageMaterialsBuilder は TriageMaterialsBuilder を生成する。
func NewTriageMaterialsBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *TriageMaterialsBuilder {
	return &TriageMaterialsBuilder{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Build は issueKey の triage 材料を収集して AnalysisEnvelope を返す。
// GetIssue の失敗は error を返す（必須）。
// GetProject/ListIssues/ListIssueComments の失敗は warnings に追加して部分結果を返す。
func (b *TriageMaterialsBuilder) Build(ctx context.Context, issueKey string, opt TriageMaterialsOptions) (*AnalysisEnvelope, error) {
	// 完了ステータスの解決
	closedStatus := opt.ClosedStatus
	if len(closedStatus) == 0 {
		closedStatus = defaultClosedStatus
	}

	// 1. 対象課題取得（必須 — 失敗したら error 返却）
	issue, err := b.client.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", issueKey, err)
	}

	projectKey := extractProjectKey(issueKey)

	// 2. errgroup で並列取得
	var (
		projectIssues []domain.Issue
		comments      []domain.Comment
		mu            sync.Mutex
		warnings      []domain.Warning
	)

	g, gctx := errgroup.WithContext(ctx)

	// goroutine 1: GetProject → ListIssues
	g.Go(func() error {
		project, err := b.client.GetProject(gctx, projectKey)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "project_fetch_failed",
				Message:   fmt.Sprintf("failed to get project %s: %v", projectKey, err),
				Component: "project_stats",
				Retryable: true,
			})
			mu.Unlock()
			return nil // 部分失敗は warning に留める
		}

		issues, err := b.client.ListIssues(gctx, backlog.ListIssuesOptions{
			ProjectIDs: []int{project.ID},
		})
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "issues_fetch_failed",
				Message:   fmt.Sprintf("failed to list issues for project %s: %v", projectKey, err),
				Component: "project_stats",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}

		mu.Lock()
		projectIssues = issues
		mu.Unlock()
		return nil
	})

	// goroutine 2: ListIssueComments（件数のみ使用）
	g.Go(func() error {
		cs, err := b.client.ListIssueComments(gctx, issueKey, backlog.ListCommentsOptions{})
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "comments_fetch_failed",
				Message:   fmt.Sprintf("failed to list comments for %s: %v", issueKey, err),
				Component: "history.comment_count",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}
		mu.Lock()
		comments = cs
		mu.Unlock()
		return nil
	})

	_ = g.Wait()

	now := b.now()

	// 3. 各セクションを組み立て
	triageIssue := buildTriageIssue(issue)
	history := buildTriageHistory(issue, len(comments), now)
	stats := buildTriageProjectStats(projectIssues, closedStatus)
	similar := buildTriageSimilar(issue, projectIssues)

	result := &TriageMaterials{
		Issue:         triageIssue,
		History:       history,
		ProjectStats:  stats,
		SimilarIssues: similar,
	}

	return b.newEnvelope("issue_triage_materials", result, warnings), nil
}

// buildTriageIssue は domain.Issue から TriageIssue を構築する。
func buildTriageIssue(issue *domain.Issue) TriageIssue {
	categories := issue.Categories
	if categories == nil {
		categories = []domain.IDName{}
	}
	milestones := issue.Milestones
	if milestones == nil {
		milestones = []domain.IDName{}
	}

	return TriageIssue{
		IssueKey:   issue.IssueKey,
		Summary:    issue.Summary,
		Status:     issue.Status,
		Priority:   issue.Priority,
		IssueType:  issue.IssueType,
		Assignee:   toUserRef(issue.Assignee),
		Reporter:   toUserRef(issue.Reporter),
		Categories: categories,
		Milestones: milestones,
		DueDate:    issue.DueDate,
		Created:    issue.Created,
		Updated:    issue.Updated,
	}
}

// buildTriageHistory は課題の履歴サマリーを計算する。
func buildTriageHistory(issue *domain.Issue, commentCount int, now time.Time) TriageHistory {
	h := TriageHistory{
		CommentCount: commentCount,
	}

	if issue.Created != nil {
		h.DaysSinceCreated = int(now.Sub(*issue.Created).Hours() / 24)
	}
	if issue.Updated != nil {
		h.DaysSinceUpdated = int(now.Sub(*issue.Updated).Hours() / 24)
		h.IsStale = h.DaysSinceUpdated >= DefaultStaleDays
	}
	if issue.DueDate != nil && issue.DueDate.Before(now) {
		h.IsOverdue = true
	}

	return h
}

// buildTriageProjectStats はプロジェクト全体の統計情報を集計する。
func buildTriageProjectStats(issues []domain.Issue, closedStatus []string) TriageProjectStats {
	// 完了ステータスの set を構築
	closedSet := make(map[string]bool, len(closedStatus))
	for _, s := range closedStatus {
		closedSet[s] = true
	}

	byStatus := make(map[string]int)
	byPriority := make(map[string]int)
	byAssignee := make(map[string]int)

	var totalCloseDays float64
	var closedCount int

	for i := range issues {
		issue := &issues[i]

		// ステータス集計
		statusName := ""
		if issue.Status != nil {
			statusName = issue.Status.Name
		}
		if statusName != "" {
			byStatus[statusName]++
		}

		// 優先度集計
		if issue.Priority != nil && issue.Priority.Name != "" {
			byPriority[issue.Priority.Name]++
		}

		// 担当者集計
		if issue.Assignee != nil {
			byAssignee[issue.Assignee.Name]++
		} else {
			byAssignee[unassignedLabel]++
		}

		// avg_close_days 計算（完了ステータスのみ、Created と Updated の差分を近似値として使用）
		// 注意: Backlog API では完了日の専用フィールドがないため、Updated を完了日の近似値として使用する。
		if statusName != "" && closedSet[statusName] && issue.Created != nil && issue.Updated != nil {
			days := issue.Updated.Sub(*issue.Created).Hours() / 24
			if days >= 0 {
				totalCloseDays += days
				closedCount++
			}
		}
	}

	avgCloseDays := 0.0
	if closedCount > 0 {
		avgCloseDays = totalCloseDays / float64(closedCount)
	}

	return TriageProjectStats{
		TotalIssues:  len(issues),
		ByStatus:     byStatus,
		ByPriority:   byPriority,
		ByAssignee:   byAssignee,
		AvgCloseDays: avgCloseDays,
	}
}

// buildTriageSimilar は類似課題（同カテゴリ・同マイルストーン）の分布を集計する。
// 対象課題自身は集計から除外する。
// PriorityDistribution と AssigneeDistribution は同カテゴリ課題の分布を表す。
// SameMilestoneCount は件数のみ（分布は PriorityDistribution と共有すると混乱するため省略）。
func buildTriageSimilar(target *domain.Issue, issues []domain.Issue) TriageSimilar {
	// 対象課題のカテゴリIDとマイルストーンIDを set に変換
	targetCategoryIDs := make(map[int]bool, len(target.Categories))
	for _, c := range target.Categories {
		targetCategoryIDs[c.ID] = true
	}

	targetMilestoneIDs := make(map[int]bool, len(target.Milestones))
	for _, m := range target.Milestones {
		targetMilestoneIDs[m.ID] = true
	}

	categoryPriorityDist := make(map[string]int)
	categoryAssigneeDist := make(map[string]int)
	sameCategoryCount := 0
	sameMilestoneCount := 0

	for i := range issues {
		issue := &issues[i]

		// 対象課題自身を除外
		if issue.IssueKey == target.IssueKey {
			continue
		}

		// 同カテゴリ判定（一つでもマッチするカテゴリがあれば同カテゴリ）
		hasCategoryMatch := false
		for _, c := range issue.Categories {
			if targetCategoryIDs[c.ID] {
				hasCategoryMatch = true
				break
			}
		}
		if hasCategoryMatch {
			sameCategoryCount++
			// 同カテゴリ課題の優先度・担当者分布を集計
			if issue.Priority != nil && issue.Priority.Name != "" {
				categoryPriorityDist[issue.Priority.Name]++
			}
			if issue.Assignee != nil {
				categoryAssigneeDist[issue.Assignee.Name]++
			}
		}

		// 同マイルストーン判定（一つでもマッチするマイルストーンがあれば同マイルストーン）
		for _, m := range issue.Milestones {
			if targetMilestoneIDs[m.ID] {
				sameMilestoneCount++
				break
			}
		}
	}

	return TriageSimilar{
		SameCategoryCount:    sameCategoryCount,
		SameMilestoneCount:   sameMilestoneCount,
		PriorityDistribution: categoryPriorityDist,
		AssigneeDistribution: categoryAssigneeDist,
	}
}
