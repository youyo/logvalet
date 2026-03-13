package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/render"
)

func TestMarkdownRenderer_Render(t *testing.T) {
	t.Run("struct を Markdown コードブロックで出力する", func(t *testing.T) {
		r := render.NewMarkdownRenderer()
		data := testData{Name: "test", Value: 42}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "```") {
			t.Errorf("Markdown 出力にコードブロック ``` が含まれていない: %s", output)
		}
		if !strings.Contains(output, `"name"`) {
			t.Errorf("Markdown 出力に name フィールドが含まれていない: %s", output)
		}
	})

	t.Run("nil データを渡した場合 null を含む出力を返す", func(t *testing.T) {
		r := render.NewMarkdownRenderer()

		var buf bytes.Buffer
		if err := r.Render(&buf, nil); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "null") {
			t.Errorf("Markdown 出力に null が含まれていない: %s", output)
		}
	})

	t.Run("コードブロックは json タグ付きで出力される", func(t *testing.T) {
		r := render.NewMarkdownRenderer()
		data := map[string]int{"count": 3}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if !strings.HasPrefix(output, "```json") {
			t.Errorf("Markdown コードブロックが ```json で始まっていない: %s", output)
		}
	})
}

func TestMarkdownRenderer_ImplementsRenderer(t *testing.T) {
	var _ render.Renderer = render.NewMarkdownRenderer()
}

func TestNewRenderer_Markdown(t *testing.T) {
	t.Run("md フォーマットで MarkdownRenderer を返す", func(t *testing.T) {
		r, err := render.NewRenderer("md", false)
		if err != nil {
			t.Fatalf("NewRenderer(md) エラー: %v", err)
		}
		if r == nil {
			t.Error("NewRenderer(md) は nil を返してはならない")
		}
	})

	t.Run("markdown フォーマットで MarkdownRenderer を返す", func(t *testing.T) {
		r, err := render.NewRenderer("markdown", false)
		if err != nil {
			t.Fatalf("NewRenderer(markdown) エラー: %v", err)
		}
		if r == nil {
			t.Error("NewRenderer(markdown) は nil を返してはならない")
		}
	})
}
