package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/render"
)

func TestMarkdown_table_basic(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "name": "Alice", "email": "alice@example.com"},
		{"id": 2, "name": "Bob", "email": "bob@example.com"},
	}
	var buf bytes.Buffer
	r := render.NewMarkdownRenderer()
	if err := r.Render(&buf, data); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "| id") {
		t.Error("テーブルヘッダーに id がない")
	}
	if !strings.Contains(out, "| name") {
		t.Error("テーブルヘッダーに name がない")
	}
	if !strings.Contains(out, "Alice") {
		t.Error("データ行に Alice がない")
	}
	if !strings.Contains(out, "Bob") {
		t.Error("データ行に Bob がない")
	}
	// セパレーター行の確認
	if !strings.Contains(out, "----") {
		t.Error("セパレーター行がない")
	}
}

func TestMarkdown_table_nested(t *testing.T) {
	data := []map[string]any{
		{"issueKey": "CND-7", "status": map[string]any{"id": 1, "name": "処理中"}},
	}
	var buf bytes.Buffer
	r := render.NewMarkdownRenderer()
	if err := r.Render(&buf, data); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	// ネストした map で name キーがあれば Name を表示
	if !strings.Contains(out, "処理中") {
		t.Error("ネストした map の name が表示されていない")
	}
}

func TestMarkdown_table_empty(t *testing.T) {
	data := []map[string]any{}
	var buf bytes.Buffer
	r := render.NewMarkdownRenderer()
	if err := r.Render(&buf, data); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(データなし)") {
		t.Errorf("空配列で (データなし) が出力されない: %q", out)
	}
}

func TestMarkdown_kv_basic(t *testing.T) {
	data := map[string]any{
		"profile":   "heptagon",
		"auth_type": "api_key",
		"expired":   false,
	}
	var buf bytes.Buffer
	r := render.NewMarkdownRenderer()
	if err := r.Render(&buf, data); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "- **profile**: heptagon") {
		t.Error("キー・値リストに profile がない")
	}
	if !strings.Contains(out, "- **auth_type**: api_key") {
		t.Error("キー・値リストに auth_type がない")
	}
}

func TestMarkdown_kv_nested(t *testing.T) {
	data := map[string]any{
		"profile": "heptagon",
		"user":    map[string]any{"id": 1, "name": "Naoto Ishizawa"},
	}
	var buf bytes.Buffer
	r := render.NewMarkdownRenderer()
	if err := r.Render(&buf, data); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Naoto Ishizawa") {
		t.Error("ネストした user の name が表示されていない")
	}
}

func TestNewRenderer_md(t *testing.T) {
	r, err := render.NewRenderer("md", false, "")
	if err != nil {
		t.Fatalf("NewRenderer(md) error: %v", err)
	}
	if r == nil {
		t.Error("renderer が nil")
	}
}

func TestNewRenderer_markdown(t *testing.T) {
	r, err := render.NewRenderer("markdown", false, "")
	if err != nil {
		t.Fatalf("NewRenderer(markdown) error: %v", err)
	}
	if r == nil {
		t.Error("renderer が nil")
	}
}
