package render

import (
	"encoding/json"
	"fmt"
	"io"
)

// TextRenderer はコンパクトなターミナル向け JSON 形式で data を出力する Renderer 実装。
type TextRenderer struct{}

// NewTextRenderer は TextRenderer を返す。
func NewTextRenderer() *TextRenderer {
	return &TextRenderer{}
}

// Render は data をコンパクト JSON にエンコードして w に書き込む。
// 出力の末尾には改行を付加する。
func (r *TextRenderer) Render(w io.Writer, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}
