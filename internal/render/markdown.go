package render

import (
	"encoding/json"
	"fmt"
	"io"
)

// MarkdownRenderer は Markdown コードブロック形式で data を出力する Renderer 実装。
// 汎用実装として JSON をコードブロックで包む。
// リソース固有のリッチな Markdown 変換は M06 以降で digest builder 層に追加する。
type MarkdownRenderer struct{}

// NewMarkdownRenderer は MarkdownRenderer を返す。
func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{}
}

// Render は data を JSON にエンコードして Markdown のコードブロックとして w に書き込む。
func (r *MarkdownRenderer) Render(w io.Writer, data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "```json\n%s\n```\n", b)
	return err
}
