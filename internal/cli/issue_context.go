package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/analysis"
)

// IssueContextCmd は issue context コマンド。
// 単一 issue の総合コンテキストを AI 分析用に取得する。
type IssueContextCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
	Comments     int    `help:"max recent comments to include" default:"10"`
	Compact      bool   `help:"omit description and comment bodies"`
}

// Run は issue context コマンドを実行する。
func (c *IssueContextCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	builder := analysis.NewIssueContextBuilder(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := builder.Build(ctx, c.IssueIDOrKey, analysis.IssueContextOptions{
		MaxComments: c.Comments,
		Compact:     c.Compact,
	})
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
