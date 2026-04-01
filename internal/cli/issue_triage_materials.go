package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/analysis"
)

// IssueTriageMaterialsCmd は issue triage-materials コマンド。
// 単一 issue のトリアージ材料（統計・類似課題・履歴）を収集して返す。
type IssueTriageMaterialsCmd struct {
	IssueKey string `arg:"" required:"" help:"issue key (e.g., PROJ-123)"`
}

// Run は issue triage-materials コマンドを実行する。
func (c *IssueTriageMaterialsCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	builder := analysis.NewTriageMaterialsBuilder(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := builder.Build(ctx, c.IssueKey, analysis.TriageMaterialsOptions{})
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
