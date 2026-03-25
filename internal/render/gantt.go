package render

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/youyo/logvalet/internal/domain"
)

type GanttTableRenderer struct {
	space string
}

func NewGanttTableRenderer(space string) *GanttTableRenderer {
	return &GanttTableRenderer{space: space}
}

func (r *GanttTableRenderer) Render(w io.Writer, data any) error {
	issues, ok := data.([]domain.Issue)
	if !ok {
		return fmt.Errorf("gantt format is only available for issue list")
	}

	if len(issues) == 0 {
		_, err := fmt.Fprintln(w, "(no data)")
		return err
	}

	// nil 日付処理 + フィルタ
	var valid []domain.Issue
	skipped := 0
	for _, issue := range issues {
		if issue.StartDate == nil && issue.DueDate == nil {
			skipped++
			continue
		}
		if issue.StartDate == nil {
			issue.StartDate = issue.DueDate
		}
		if issue.DueDate == nil {
			issue.DueDate = issue.StartDate
		}
		valid = append(valid, issue)
	}

	if len(valid) == 0 {
		fmt.Fprintln(w, "(no data)")
		if skipped > 0 {
			fmt.Fprintf(os.Stderr, "warning: %d issue(s) skipped (missing dates)\n", skipped)
		}
		return nil
	}

	// ソート: startDate 昇順 → dueDate 昇順
	sort.Slice(valid, func(i, j int) bool {
		if valid[i].StartDate.Equal(*valid[j].StartDate) {
			return valid[i].DueDate.Before(*valid[j].DueDate)
		}
		return valid[i].StartDate.Before(*valid[j].StartDate)
	})

	// min/max 算出
	minDate := *valid[0].StartDate
	maxDate := *valid[0].DueDate
	for _, issue := range valid {
		if issue.StartDate.Before(minDate) {
			minDate = *issue.StartDate
		}
		if issue.DueDate.After(maxDate) {
			maxDate = *issue.DueDate
		}
	}

	// 日付列生成
	dates := generateDateRange(minDate, maxDate)
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	// タイトル
	fmt.Fprintf(w, "📅 Tasks (%d/%d – %d/%d)\n\n",
		int(minDate.Month()), minDate.Day(),
		int(maxDate.Month()), maxDate.Day())

	// ヘッダー
	headers := make([]string, len(dates))
	for i, d := range dates {
		headers[i] = fmt.Sprintf("%d/%d", int(d.Month()), d.Day())
	}
	fmt.Fprintf(w, "| Issue | %s |\n", strings.Join(headers, " | "))

	// セパレーター
	seps := make([]string, len(dates)+1)
	seps[0] = "------"
	for i := 1; i <= len(dates); i++ {
		seps[i] = "----"
	}
	fmt.Fprintf(w, "|%s|\n", strings.Join(seps, "|"))

	// データ行
	for _, issue := range valid {
		label := r.formatIssueLabel(issue)
		cells := make([]string, len(dates))
		for i, d := range dates {
			dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
			issueStart := time.Date(issue.StartDate.Year(), issue.StartDate.Month(), issue.StartDate.Day(), 0, 0, 0, 0, issue.StartDate.Location())
			issueEnd := time.Date(issue.DueDate.Year(), issue.DueDate.Month(), issue.DueDate.Day(), 0, 0, 0, 0, issue.DueDate.Location())

			if dayStart.Before(issueStart) || dayStart.After(issueEnd) {
				cells[i] = "  "
			} else if dayStart.Before(today) {
				cells[i] = " ░░ "
			} else {
				cells[i] = " ██ "
			}
		}
		fmt.Fprintf(w, "| %s | %s |\n", label, strings.Join(cells, " | "))
	}

	// 凡例
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Legend: ░░ elapsed  ██ remaining")

	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d issue(s) skipped (missing dates)\n", skipped)
	}

	return nil
}

func (r *GanttTableRenderer) formatIssueLabel(issue domain.Issue) string {
	summary := issue.Summary
	if utf8.RuneCountInString(summary) > 30 {
		runes := []rune(summary)
		summary = string(runes[:30]) + "…"
	}
	if r.space != "" {
		return fmt.Sprintf("[%s %s](https://%s.backlog.com/view/%s)",
			issue.IssueKey, summary, r.space, issue.IssueKey)
	}
	return fmt.Sprintf("%s %s", issue.IssueKey, summary)
}

func generateDateRange(start, end time.Time) []time.Time {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	var dates []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	return dates
}
