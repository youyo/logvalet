package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/render"
)

func TestYAMLRenderer_Render(t *testing.T) {
	t.Run("struct を YAML に変換する", func(t *testing.T) {
		r := render.NewYAMLRenderer()
		data := testData{Name: "test", Value: 42}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "name: test") {
			t.Errorf("YAML 出力に name: test が含まれていない: %s", output)
		}
		if !strings.Contains(output, "value: 42") {
			t.Errorf("YAML 出力に value: 42 が含まれていない: %s", output)
		}
	})

	t.Run("nil データを渡した場合 null を出力する", func(t *testing.T) {
		r := render.NewYAMLRenderer()

		var buf bytes.Buffer
		if err := r.Render(&buf, nil); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "null" {
			t.Errorf("got %q, want %q", output, "null")
		}
	})

	t.Run("map を YAML に変換する", func(t *testing.T) {
		r := render.NewYAMLRenderer()
		data := map[string]string{"key": "value"}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "key: value") {
			t.Errorf("YAML 出力に key: value が含まれていない: %s", output)
		}
	})
}

func TestYAMLRenderer_ImplementsRenderer(t *testing.T) {
	var _ render.Renderer = render.NewYAMLRenderer()
}

func TestNewRenderer_YAML(t *testing.T) {
	r, err := render.NewRenderer("yaml", false)
	if err != nil {
		t.Fatalf("NewRenderer(yaml) エラー: %v", err)
	}
	if r == nil {
		t.Error("NewRenderer(yaml) は nil を返してはならない")
	}
}
