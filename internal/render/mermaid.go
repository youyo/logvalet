package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/domain"
)

// MermaidGanttRenderer は Mermaid gantt 形式で出力するレンダラー。
type MermaidGanttRenderer struct{}

// NewMermaidGanttRenderer は MermaidGanttRenderer を生成する。
func NewMermaidGanttRenderer() *MermaidGanttRenderer {
	return &MermaidGanttRenderer{}
}

// Render は data を Mermaid gantt 形式で w に書き出す。
// data が []domain.Issue の場合のみ gantt を生成し、それ以外は JSON フォールバック。
func (r *MermaidGanttRenderer) Render(w io.Writer, data any) error {
	issues, ok := data.([]domain.Issue)
	if !ok {
		// JSON フォールバック
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	fmt.Fprintln(w, "gantt")
	fmt.Fprintln(w, "  title Issues")
	fmt.Fprintln(w, "  dateFormat YYYY-MM-DD")

	// projectKey でグループ化
	sections := make(map[string][]domain.Issue)
	var sectionOrder []string
	skipped := 0

	for _, issue := range issues {
		if issue.StartDate == nil || issue.DueDate == nil {
			skipped++
			continue
		}
		prefix := extractProjectKey(issue.IssueKey)
		if _, exists := sections[prefix]; !exists {
			sectionOrder = append(sectionOrder, prefix)
		}
		sections[prefix] = append(sections[prefix], issue)
	}

	for _, section := range sectionOrder {
		fmt.Fprintf(w, "  section %s\n", section)
		for _, issue := range sections[section] {
			summary := strings.ReplaceAll(issue.Summary, ":", "")
			fmt.Fprintf(w, "    %s %s : %s, %s\n",
				issue.IssueKey,
				summary,
				issue.StartDate.Format("2006-01-02"),
				issue.DueDate.Format("2006-01-02"),
			)
		}
	}

	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "警告: %d 件の課題は日付未設定のためスキップしました\n", skipped)
	}

	return nil
}

// extractProjectKey は issueKey (e.g. "CND-7") からプロジェクトキー ("CND") を抽出する。
func extractProjectKey(issueKey string) string {
	if idx := strings.LastIndex(issueKey, "-"); idx > 0 {
		return issueKey[:idx]
	}
	return issueKey
}
