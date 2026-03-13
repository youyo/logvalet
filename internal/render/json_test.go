package render_test

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/youyo/logvalet/internal/render"
)

type testData struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestJSONRenderer_Render(t *testing.T) {
	t.Run("struct を JSON に変換する", func(t *testing.T) {
		r := render.NewJSONRenderer(false)
		data := testData{Name: "test", Value: 42}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		var got testData
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("出力が有効な JSON でない: %v, output: %s", err, buf.String())
		}

		if got.Name != data.Name || got.Value != data.Value {
			t.Errorf("got %+v, want %+v", got, data)
		}
	})

	t.Run("pretty-print モードでインデント付き JSON を出力する", func(t *testing.T) {
		r := render.NewJSONRenderer(true)
		data := testData{Name: "pretty", Value: 1}

		var buf bytes.Buffer
		if err := r.Render(&buf, data); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		// pretty-print は改行を含む
		if !bytes.Contains(buf.Bytes(), []byte("\n")) {
			t.Errorf("pretty-print モードで改行が含まれていない: %s", output)
		}
		// インデントが含まれる
		if !bytes.Contains(buf.Bytes(), []byte("  ")) {
			t.Errorf("pretty-print モードでインデントが含まれていない: %s", output)
		}
	})

	t.Run("nil データを渡した場合 null を出力する", func(t *testing.T) {
		r := render.NewJSONRenderer(false)

		var buf bytes.Buffer
		if err := r.Render(&buf, nil); err != nil {
			t.Fatalf("Render() エラー: %v", err)
		}

		output := buf.String()
		if output != "null\n" {
			t.Errorf("got %q, want %q", output, "null\n")
		}
	})
}

func TestJSONRenderer_ImplementsRenderer(t *testing.T) {
	// コンパイル時に Renderer interface を実装しているか確認
	var _ render.Renderer = render.NewJSONRenderer(false)
}

func TestNewRenderer(t *testing.T) {
	t.Run("json フォーマットで JSONRenderer を返す", func(t *testing.T) {
		r, err := render.NewRenderer("json", false)
		if err != nil {
			t.Fatalf("NewRenderer() エラー: %v", err)
		}
		if r == nil {
			t.Error("NewRenderer() は nil を返してはならない")
		}
	})

	t.Run("不明なフォーマットはエラーを返す", func(t *testing.T) {
		_, err := render.NewRenderer("unknown", false)
		if err == nil {
			t.Error("不明なフォーマットでエラーが返されなかった")
		}
	})
}

// Renderer interface を使ったポリモーフィックなテスト
func TestRenderer_interface(t *testing.T) {
	r := render.NewJSONRenderer(false)
	data := map[string]string{"key": "value"}

	// io.Writer インターフェースとして渡せることを確認
	var w io.Writer = &bytes.Buffer{}
	if err := r.Render(w, data); err != nil {
		t.Fatalf("Render() エラー: %v", err)
	}
}
