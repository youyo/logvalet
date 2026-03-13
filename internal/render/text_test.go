package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/render"
)

func TestTextRenderer_Render(t *testing.T) {
	t.Run("struct をコンパクト JSON で出力する", func(t *testing.T) {
		r := render.NewTextRenderer()
		data := testData{Name: "test", Value: 42}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		// コンパクト形式なので改行が少ない（末尾の1個のみ）
		if strings.Count(output, "\n") > 1 {
			t.Errorf("テキスト出力が複数行になっている（コンパクト形式でない）: %s", output)
		}
		if !strings.Contains(output, `"name"`) {
			t.Errorf("テキスト出力に name フィールドが含まれていない: %s", output)
		}
	})

	t.Run("nil データを渡した場合 null を出力する", func(t *testing.T) {
		r := render.NewTextRenderer()

		var buf bytes.Buffer
		if err := r.Render(&buf, nil); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "null" {
			t.Errorf("got %q, want %q", output, "null")
		}
	})

	t.Run("map をコンパクト JSON で出力する", func(t *testing.T) {
		r := render.NewTextRenderer()
		data := map[string]string{"key": "value"}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != `{"key":"value"}` {
			t.Errorf("got %q, want %q", output, `{"key":"value"}`)
		}
	})

	t.Run("出力の末尾に改行が付く", func(t *testing.T) {
		r := render.NewTextRenderer()
		data := testData{Name: "x", Value: 1}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if !strings.HasSuffix(output, "\n") {
			t.Errorf("出力の末尾に改行が付いていない: %q", output)
		}
	})
}

func TestTextRenderer_ImplementsRenderer(t *testing.T) {
	var _ render.Renderer = render.NewTextRenderer()
}

func TestNewRenderer_Text(t *testing.T) {
	r, err := render.NewRenderer("text", false)
	if err != nil {
		t.Fatalf("NewRenderer(text) エラー: %v", err)
	}
	if r == nil {
		t.Error("NewRenderer(text) は nil を返してはならない")
	}
}

func TestNewRenderer_AllFormats(t *testing.T) {
	formats := []string{"json", "yaml", "md", "markdown", "text"}
	for _, format := range formats {
		r, err := render.NewRenderer(format, false)
		if err != nil {
			t.Errorf("NewRenderer(%q) エラー: %v", format, err)
		}
		if r == nil {
			t.Errorf("NewRenderer(%q) が nil を返した", format)
		}
	}
}
