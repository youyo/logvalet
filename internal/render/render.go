// Package render はlogvalet CLI の出力フォーマッタを提供する。
// stdout には機械可読な結果のみを出力する。
package render

import (
	"fmt"
	"io"
)

// Renderer は任意のデータを io.Writer に書き出すインターフェース。
type Renderer interface {
	Render(w io.Writer, data any) error
}

// NewRenderer はフォーマット名に対応する Renderer を返す。
// 未知のフォーマットはエラーを返す。
// サポートするフォーマット: json, yaml, md, markdown, text, mermaid
func NewRenderer(format string, pretty bool) (Renderer, error) {
	switch format {
	case "json":
		return NewJSONRenderer(pretty), nil
	case "yaml":
		return NewYAMLRenderer(), nil
	case "md", "markdown":
		return NewMarkdownRenderer(), nil
	case "text":
		return NewTextRenderer(), nil
	case "mermaid":
		return NewMermaidGanttRenderer(), nil
	default:
		return nil, fmt.Errorf("未サポートのフォーマット: %q (サポート: json, yaml, md, markdown, text, mermaid)", format)
	}
}
