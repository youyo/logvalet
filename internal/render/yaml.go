package render

import (
	"io"

	"gopkg.in/yaml.v3"
)

// YAMLRenderer は YAML 形式で data を出力する Renderer 実装。
type YAMLRenderer struct{}

// NewYAMLRenderer は YAMLRenderer を返す。
func NewYAMLRenderer() *YAMLRenderer {
	return &YAMLRenderer{}
}

// Render は data を YAML にエンコードして w に書き込む。
func (r *YAMLRenderer) Render(w io.Writer, data any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(data)
}
