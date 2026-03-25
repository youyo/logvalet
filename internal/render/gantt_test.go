package render_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/render"
)

func TestGantt_basic(t *testing.T) {
	start1 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	due1 := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)
	start2 := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)
	due2 := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)

	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "KB反映調査", StartDate: &start1, DueDate: &due1},
		{IssueKey: "CND-8", Summary: "sandbox2構築", StartDate: &start2, DueDate: &due2},
	}

	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	if err := r.Render(&buf, issues); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "📅 タスク一覧") {
		t.Error("タイトルがない")
	}
	if !strings.Contains(out, "3/20") {
		t.Error("日付列 3/20 がない")
	}
	if !strings.Contains(out, "3/23") {
		t.Error("日付列 3/23 がない")
	}
	if !strings.Contains(out, "heptagon.backlog.com/view/CND-7") {
		t.Error("Backlog URL がない")
	}
	if !strings.Contains(out, "凡例") {
		t.Error("凡例がない")
	}
}

func TestGantt_nilStart(t *testing.T) {
	due := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "test", DueDate: &due},
	}
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	if err := r.Render(&buf, issues); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CND-7") {
		t.Error("startDate nil の課題が描画されていない")
	}
}

func TestGantt_bothNil(t *testing.T) {
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "test"},
	}
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	_ = r.Render(&buf, issues)
	out := buf.String()
	if !strings.Contains(out, "(データなし)") {
		t.Error("両方nil で (データなし) が出力されない")
	}
}

func TestGantt_truncate(t *testing.T) {
	start := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "これは三十一文字を超える非常に長い件名のサマリーテストです12345678", StartDate: &start, DueDate: &due},
	}
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	if err := r.Render(&buf, issues); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "…") {
		t.Error("30文字超の件名が切り詰められていない")
	}
}

func TestGantt_sort(t *testing.T) {
	d1 := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "A-1", Summary: "third", StartDate: &d1, DueDate: &d1},
		{IssueKey: "A-2", Summary: "first", StartDate: &d2, DueDate: &d2},
		{IssueKey: "A-3", Summary: "second", StartDate: &d3, DueDate: &d3},
	}
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	if err := r.Render(&buf, issues); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	firstIdx := strings.Index(out, "first")
	secondIdx := strings.Index(out, "second")
	thirdIdx := strings.Index(out, "third")
	if firstIdx > secondIdx || secondIdx > thirdIdx {
		t.Errorf("ソート順が正しくない: first=%d, second=%d, third=%d", firstIdx, secondIdx, thirdIdx)
	}
}

func TestGantt_empty(t *testing.T) {
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	if err := r.Render(&buf, []domain.Issue{}); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(buf.String(), "(データなし)") {
		t.Error("空スライスで (データなし) が出力されない")
	}
}

func TestGantt_singleDay(t *testing.T) {
	d := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "test", StartDate: &d, DueDate: &d},
	}
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	if err := r.Render(&buf, issues); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	// 日付列は1列のみ
	if strings.Count(out, "3/25") < 1 {
		t.Error("single day で日付列がない")
	}
}

func TestGantt_nonIssue(t *testing.T) {
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("heptagon")
	err := r.Render(&buf, "not issues")
	if err == nil {
		t.Error("非 Issue データでエラーが返らない")
	}
	if !strings.Contains(err.Error(), "gantt フォーマットは issue list でのみ使用できます") {
		t.Errorf("エラーメッセージが不正: %v", err)
	}
}

func TestGantt_noSpace(t *testing.T) {
	d := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	issues := []domain.Issue{
		{IssueKey: "CND-7", Summary: "test", StartDate: &d, DueDate: &d},
	}
	var buf bytes.Buffer
	r := render.NewGanttTableRenderer("") // space 空
	if err := r.Render(&buf, issues); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "backlog.com") {
		t.Error("space 空なのに URL が生成されている")
	}
	if !strings.Contains(out, "CND-7") {
		t.Error("課題キーが出力されていない")
	}
}

func TestNewRenderer_gantt(t *testing.T) {
	r, err := render.NewRenderer("gantt", false, "heptagon")
	if err != nil {
		t.Fatalf("NewRenderer(gantt) error: %v", err)
	}
	if r == nil {
		t.Error("renderer が nil")
	}
}

func TestNewRenderer_text_removed(t *testing.T) {
	_, err := render.NewRenderer("text", false, "")
	if err == nil {
		t.Error("text フォーマットが削除されていない")
	}
}

func TestNewRenderer_mermaid_removed(t *testing.T) {
	_, err := render.NewRenderer("mermaid", false, "")
	if err == nil {
		t.Error("mermaid フォーマットが削除されていない")
	}
}
