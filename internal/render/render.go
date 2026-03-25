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
func NewRenderer(format string, pretty bool, space string) (Renderer, error) {
	switch format {
	case "json":
		return NewJSONRenderer(pretty), nil
	case "yaml":
		return NewYAMLRenderer(), nil
	case "md", "markdown":
		return NewMarkdownRenderer(), nil
	case "gantt":
		return NewGanttTableRenderer(space), nil
	default:
		return nil, fmt.Errorf("unsupported format: %q (supported: json, yaml, md, markdown, gantt)", format)
	}
}
