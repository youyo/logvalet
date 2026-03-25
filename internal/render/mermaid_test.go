package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/render"
)

func TestMermaidGantt_basicIssues(t *testing.T) {
	start1 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	due1 := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	start2 := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)
	due2 := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "KB反映調査", StartDate: &start1, DueDate: &due1},
		{IssueKey: "CND-8", Summary: "sandbox2構築", StartDate: &start2, DueDate: &due2},
	}

	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	err := r.Render(&buf, issues)
	if err != nil {
		t.Fatalf("Render() エラー: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gantt") {
		t.Error("gantt ヘッダがない")
	}
	if !strings.Contains(out, "dateFormat YYYY-MM-DD") {
		t.Error("dateFormat がない")
	}
	if !strings.Contains(out, "section CND") {
		t.Error("section CND がない")
	}
	if !strings.Contains(out, "CND-7 KB反映調査") {
		t.Error("CND-7 の行がない")
	}
	if !strings.Contains(out, "2026-03-20") {
		t.Error("start date がない")
	}
}

func TestMermaidGantt_nilStartDate(t *testing.T) {
	due := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "test", DueDate: &due},
	}

	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	_ = r.Render(&buf, issues)

	out := buf.String()
	// startDate nil → dueDate を代用して1日タスクとして描画
	if !strings.Contains(out, "CND-7") {
		t.Error("startDate nil の課題が描画されていない（dueDate で代用すべき）")
	}
	if !strings.Contains(out, "2026-03-28, 2026-03-28") {
		t.Error("startDate nil 時は dueDate と同日で1日タスクになるべき")
	}
}

func TestMermaidGantt_nilDueDate(t *testing.T) {
	start := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "test", StartDate: &start},
	}

	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	_ = r.Render(&buf, issues)

	out := buf.String()
	// dueDate nil → startDate を代用して1日タスクとして描画
	if !strings.Contains(out, "CND-7") {
		t.Error("dueDate nil の課題が描画されていない（startDate で代用すべき）")
	}
	if !strings.Contains(out, "2026-03-20, 2026-03-20") {
		t.Error("dueDate nil 時は startDate と同日で1日タスクになるべき")
	}
}

func TestMermaidGantt_mixedNil(t *testing.T) {
	start := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "有効", StartDate: &start, DueDate: &due},
		{IssueKey: "CND-8", Summary: "スキップ"}, // 両方 nil
	}

	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	_ = r.Render(&buf, issues)

	out := buf.String()
	if !strings.Contains(out, "CND-7") {
		t.Error("有効な課題が出力されていない")
	}
	if strings.Contains(out, "CND-8") {
		t.Error("両方 nil の課題がスキップされていない")
	}
}

func TestMermaidGantt_nonIssueData(t *testing.T) {
	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	err := r.Render(&buf, "not an issue slice")
	if err != nil {
		t.Fatalf("non-issue data でエラー: %v", err)
	}

	// JSON フォールバック
	out := buf.String()
	var s string
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Errorf("JSON フォールバックが機能していない: %s", out)
	}
}

func TestMermaidGantt_emptySlice(t *testing.T) {
	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	err := r.Render(&buf, []domain.Issue{})
	if err != nil {
		t.Fatalf("empty slice でエラー: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gantt") {
		t.Error("空スライスでも gantt ヘッダは出力すべき")
	}
}

func TestMermaidGantt_colonInSummary(t *testing.T) {
	start := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "修正: バグ対応", StartDate: &start, DueDate: &due},
	}

	var buf bytes.Buffer
	r := render.NewMermaidGanttRenderer()
	err := r.Render(&buf, issues)
	if err != nil {
		t.Fatalf("Render() エラー: %v", err)
	}

	out := buf.String()
	// summary 中のコロンが除去されていること
	if strings.Contains(out, "修正:") {
		t.Error("summary 中のコロンが除去されていない")
	}
	if !strings.Contains(out, "CND-7") {
		t.Error("課題行が出力されていない")
	}
}

func TestNewRenderer_mermaid(t *testing.T) {
	r, err := render.NewRenderer("mermaid", false)
	if err != nil {
		t.Fatalf("NewRenderer(mermaid) エラー: %v", err)
	}
	if r == nil {
		t.Error("renderer が nil")
	}
}
