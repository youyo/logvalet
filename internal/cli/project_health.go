package cli

import (
	"context"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/analysis"
)

// ProjectHealthCmd は project health コマンド。
// 指定プロジェクトの健全性（stale/blocker/workload を統合した health_score）を評価する。
type ProjectHealthCmd struct {
	ProjectKey      string `arg:"" required:"" help:"project key"`
	Days            int    `help:"days threshold for stale/blocker detection" default:"7"`
	IncludeComments bool   `help:"enable blocked-by-keyword detection via comments"`
	ExcludeStatus   string `help:"comma-separated status names to exclude (e.g. '完了,対応済み')"`
}

// Run は project health コマンドを実行する。
func (c *ProjectHealthCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	// --exclude-status をカンマ分割
	var excludeStatus []string
	if c.ExcludeStatus != "" {
		excludeStatus = strings.Split(c.ExcludeStatus, ",")
	}

	cfg := analysis.ProjectHealthConfig{
		StaleConfig: analysis.StaleConfig{
			DefaultDays:   c.Days,
			ExcludeStatus: excludeStatus,
		},
		BlockerConfig: analysis.BlockerConfig{
			InProgressDays:  c.Days,
			ExcludeStatus:   excludeStatus,
			IncludeComments: c.IncludeComments,
		},
		WorkloadConfig: analysis.WorkloadConfig{
			StaleDays:     c.Days,
			ExcludeStatus: excludeStatus,
		},
	}

	builder := analysis.NewProjectHealthBuilder(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := builder.Build(ctx, c.ProjectKey, cfg)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
