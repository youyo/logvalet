package render

import (
	"encoding/json"
	"io"
)

// JSONRenderer は JSON 形式で data を出力する Renderer 実装。
type JSONRenderer struct {
	pretty bool
}

// NewJSONRenderer は JSONRenderer を返す。
// pretty=true の場合はインデント付きで出力する。
func NewJSONRenderer(pretty bool) *JSONRenderer {
	return &JSONRenderer{pretty: pretty}
}

// Render は data を JSON にエンコードして w に書き込む。
// 出力の末尾には改行を付加する。
func (r *JSONRenderer) Render(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	if r.pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(data)
}
