package cli

import (
	"io"
	"os"

	"github.com/youyo/logvalet/internal/render"
	"github.com/youyo/logvalet/internal/version"
)

// VersionCmd はバージョン情報を出力するサブコマンド。
// 認証不要で、buildRunContext を呼ばない。
type VersionCmd struct {
	// Stdout はテスト用の出力先。nil の場合 os.Stdout にフォールバック。
	Stdout io.Writer `kong:"-"`
}

// Run はバージョン情報を指定フォーマットで出力する。
func (c *VersionCmd) Run(g *GlobalFlags) error {
	info := version.NewInfo()

	format := g.Format
	if format == "" {
		format = "json"
	}

	renderer, err := render.NewRenderer(format, g.Pretty, "")
	if err != nil {
		return err
	}

	w := c.Stdout
	if w == nil {
		w = os.Stdout
	}

	return renderer.Render(w, info)
}
