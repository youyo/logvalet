package analysis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// MyTasksOptions は MyTasksBuilder のオプション。
type MyTasksOptions struct {
	// Mode は取得モード。"week"（デフォルト）または "next"。
	Mode string
	// StaleDays はウォッチ中課題の stale 判定日数（デフォルト 7）。
	StaleDays int
}

// MyTasksResult は my_tasks 分析の結果。
type MyTasksResult struct {
	User      UserSummaryRef    `json:"user"`
	Mode      string            `json:"mode"`
	DateRange TaskDateRange     `json:"date_range"`
	Overdue   []TaskItem        `json:"overdue"`
	Upcoming  []TaskItem        `json:"upcoming"`
	Watching  []WatchedTaskItem `json:"watching"`
	Summary   MyTasksSummary    `json:"summary"`
}

// UserSummaryRef はユーザーの簡略参照。
type UserSummaryRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TaskDateRange は日付範囲。
type TaskDateRange struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

// TaskItem は課題のサマリー情報。
type TaskItem struct {
	IssueKey   string     `json:"issue_key"`
	Summary    string     `json:"summary"`
	Status     string     `json:"status"`
	Priority   string     `json:"priority"`
	DueDate    *time.Time `json:"due_date"`
	ProjectKey string     `json:"project_key"`
}

// WatchedTaskItem はウォッチ中課題のサマリー情報。
type WatchedTaskItem struct {
	TaskItem
	Assignee        string `json:"assignee"`
	IsOverdue       bool   `json:"is_overdue"`
	IsStale         bool   `json:"is_stale"`
	DaysSinceUpdate int    `json:"days_since_update"`
}

// MyTasksSummary は my_tasks 分析のサマリー。
type MyTasksSummary struct {
	OverdueCount  int `json:"overdue_count"`
	UpcomingCount int `json:"upcoming_count"`
	WatchingCount int `json:"watching_count"`
	TotalCount    int `json:"total_count"`
}

// MyTasksBuilder は自分のタスクダッシュボードを生成する。
type MyTasksBuilder struct {
	BaseAnalysisBuilder
}

// NewMyTasksBuilder は MyTasksBuilder を生成する。
func NewMyTasksBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *MyTasksBuilder {
	return &MyTasksBuilder{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Build は自分のタスクダッシュボードを構築する。
// GetMyself + 3並列 API 呼び出し（overdue/upcoming ListIssues + ListWatchings）。
// 部分失敗時は成功分の結果 + warnings を返す。
func (b *MyTasksBuilder) Build(ctx context.Context, opts MyTasksOptions) (*AnalysisEnvelope, error) {
	// デフォルト値設定
	if opts.Mode == "" {
		opts.Mode = "week"
	}
	if opts.StaleDays <= 0 {
		opts.StaleDays = 7
	}

	// GetMyself でユーザー ID 取得
	myself, err := b.client.GetMyself(ctx)
	if err != nil {
		return nil, fmt.Errorf("get myself failed: %w", err)
	}

	now := b.now()
	dateRange := calcDateRange(now, opts.Mode)

	// 昨日の日付（overdue 用）
	yesterday := now.AddDate(0, 0, -1)

	// 3並列 API 呼び出し
	var (
		overdueIssues  []domain.Issue
		upcomingIssues []domain.Issue
		watchings      []domain.Watching
		allWarnings    []domain.Warning
		mu             sync.Mutex
		wg             sync.WaitGroup
	)

	// goroutine 1: overdue（期限が昨日以前の未完了課題）
	wg.Add(1)
	go func() {
		defer wg.Done()
		overdueOpts := backlog.ListIssuesOptions{
			AssigneeIDs:  []int{myself.ID},
			StatusIDs:    []int{1, 2, 3},
			DueDateUntil: &yesterday,
		}
		issues, apiErr := b.client.ListIssues(ctx, overdueOpts)
		mu.Lock()
		defer mu.Unlock()
		if apiErr != nil {
			allWarnings = append(allWarnings, domain.Warning{
				Code:      "overdue_fetch_failed",
				Message:   fmt.Sprintf("overdue issues fetch failed: %v", apiErr),
				Component: "overdue",
				Retryable: true,
			})
			return
		}
		overdueIssues = issues
	}()

	// goroutine 2: upcoming（日付範囲内の未完了課題）
	wg.Add(1)
	go func() {
		defer wg.Done()
		since := dateRange.Since
		until := dateRange.Until
		upcomingOpts := backlog.ListIssuesOptions{
			AssigneeIDs:  []int{myself.ID},
			StatusIDs:    []int{1, 2, 3},
			DueDateSince: &since,
			DueDateUntil: &until,
		}
		issues, apiErr := b.client.ListIssues(ctx, upcomingOpts)
		mu.Lock()
		defer mu.Unlock()
		if apiErr != nil {
			allWarnings = append(allWarnings, domain.Warning{
				Code:      "upcoming_fetch_failed",
				Message:   fmt.Sprintf("upcoming issues fetch failed: %v", apiErr),
				Component: "upcoming",
				Retryable: true,
			})
			return
		}
		upcomingIssues = issues
	}()

	// goroutine 3: watching
	wg.Add(1)
	go func() {
		defer wg.Done()
		watchOpts := backlog.ListWatchingsOptions{
			Count: 100,
		}
		ws, apiErr := b.client.ListWatchings(ctx, myself.ID, watchOpts)
		mu.Lock()
		defer mu.Unlock()
		if apiErr != nil {
			allWarnings = append(allWarnings, domain.Warning{
				Code:      "watching_fetch_failed",
				Message:   fmt.Sprintf("watchings fetch failed: %v", apiErr),
				Component: "watching",
				Retryable: true,
			})
			return
		}
		watchings = ws
	}()

	wg.Wait()

	// 担当課題キーセット（重複排除用）
	assignedKeys := buildAssignedKeySet(overdueIssues, upcomingIssues)

	// TaskItem に変換
	overdueItems := toTaskItems(overdueIssues)
	upcomingItems := toTaskItems(upcomingIssues)

	// ウォッチ中課題のフィルタリングと変換
	watchedItems := buildWatchedItems(watchings, assignedKeys, now, opts.StaleDays)

	result := &MyTasksResult{
		User: UserSummaryRef{
			ID:   myself.ID,
			Name: myself.Name,
		},
		Mode:      opts.Mode,
		DateRange: dateRange,
		Overdue:   overdueItems,
		Upcoming:  upcomingItems,
		Watching:  watchedItems,
		Summary: MyTasksSummary{
			OverdueCount:  len(overdueItems),
			UpcomingCount: len(upcomingItems),
			WatchingCount: len(watchedItems),
			TotalCount:    len(overdueItems) + len(upcomingItems) + len(watchedItems),
		},
	}

	return b.newEnvelope("my_tasks", result, allWarnings), nil
}

// calcDateRange はモードに応じた日付範囲を計算する。
// "week": 今週月曜〜日曜
// "next": 今日〜今日+営業日4日
func calcDateRange(now time.Time, mode string) TaskDateRange {
	switch mode {
	case "next":
		since := now
		offset := nextModeOffset(now.Weekday())
		until := now.AddDate(0, 0, offset)
		return TaskDateRange{
			Since: truncateDay(since),
			Until: truncateDay(until),
		}
	default: // "week"
		monday := weekMonday(now)
		sunday := monday.AddDate(0, 0, 6)
		return TaskDateRange{
			Since: truncateDay(monday),
			Until: truncateDay(sunday),
		}
	}
}

// nextModeOffset は曜日に応じた "next" モードの日数オフセットを返す。
// 月:+4, 火〜金:+6, 土:+5, 日:+4
func nextModeOffset(wd time.Weekday) int {
	switch wd {
	case time.Monday:
		return 4
	case time.Saturday:
		return 5
	case time.Sunday:
		return 4
	default: // 火〜金
		return 6
	}
}

// weekMonday は指定日の週の月曜日を返す。
func weekMonday(t time.Time) time.Time {
	wd := t.Weekday()
	if wd == time.Sunday {
		wd = 7
	}
	diff := int(wd) - int(time.Monday)
	return t.AddDate(0, 0, -diff)
}

// truncateDay は時刻部分を切り捨てて日付のみにする。
func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// buildAssignedKeySet は overdue/upcoming 課題のキーセットを返す。
func buildAssignedKeySet(overdue, upcoming []domain.Issue) map[string]struct{} {
	set := make(map[string]struct{}, len(overdue)+len(upcoming))
	for _, issue := range overdue {
		set[issue.IssueKey] = struct{}{}
	}
	for _, issue := range upcoming {
		set[issue.IssueKey] = struct{}{}
	}
	return set
}

// toTaskItems は domain.Issue のスライスを []TaskItem に変換する。
func toTaskItems(issues []domain.Issue) []TaskItem {
	if len(issues) == 0 {
		return []TaskItem{}
	}
	items := make([]TaskItem, 0, len(issues))
	for _, issue := range issues {
		item := TaskItem{
			IssueKey:   issue.IssueKey,
			Summary:    issue.Summary,
			ProjectKey: extractProjectKey(issue.IssueKey),
			DueDate:    issue.DueDate,
		}
		if issue.Status != nil {
			item.Status = issue.Status.Name
		}
		if issue.Priority != nil {
			item.Priority = issue.Priority.Name
		}
		items = append(items, item)
	}
	return items
}

// buildWatchedItems はウォッチ中課題を変換・フィルタリングする。
// 担当済み課題とステータスが完了の課題は除外する。
func buildWatchedItems(watchings []domain.Watching, assignedKeys map[string]struct{}, now time.Time, staleDays int) []WatchedTaskItem {
	if len(watchings) == 0 {
		return []WatchedTaskItem{}
	}
	items := make([]WatchedTaskItem, 0, len(watchings))
	for _, w := range watchings {
		if w.Issue == nil {
			continue
		}
		issue := w.Issue

		// 担当済みは除外
		if _, ok := assignedKeys[issue.IssueKey]; ok {
			continue
		}

		// 完了は除外（Status.ID == 4）
		const closedStatusID = 4
		if issue.Status != nil && issue.Status.ID == closedStatusID {
			continue
		}

		// overdue シグナル
		isOverdue := false
		if issue.DueDate != nil && issue.DueDate.Before(now) {
			isOverdue = true
		}

		// stale シグナル
		daysSinceUpdate := 0
		isStale := false
		if w.LastContentUpdated != nil {
			diff := now.Sub(*w.LastContentUpdated)
			daysSinceUpdate = int(diff.Hours() / 24)
			if daysSinceUpdate >= staleDays {
				isStale = true
			}
		} else if issue.Updated != nil {
			diff := now.Sub(*issue.Updated)
			daysSinceUpdate = int(diff.Hours() / 24)
			if daysSinceUpdate >= staleDays {
				isStale = true
			}
		}

		item := WatchedTaskItem{
			TaskItem: TaskItem{
				IssueKey:   issue.IssueKey,
				Summary:    issue.Summary,
				ProjectKey: extractProjectKey(issue.IssueKey),
				DueDate:    issue.DueDate,
			},
			IsOverdue:       isOverdue,
			IsStale:         isStale,
			DaysSinceUpdate: daysSinceUpdate,
		}
		if issue.Status != nil {
			item.Status = issue.Status.Name
		}
		if issue.Priority != nil {
			item.Priority = issue.Priority.Name
		}
		if issue.Assignee != nil {
			item.Assignee = issue.Assignee.Name
		}

		items = append(items, item)
	}
	return items
}
