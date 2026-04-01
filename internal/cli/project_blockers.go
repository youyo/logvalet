package cli

import (
	"context"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/analysis"
)

// ProjectBlockersCmd は project blockers コマンド。
// 指定プロジェクトの進行阻害課題を検出する。
type ProjectBlockersCmd struct {
	ProjectKey      string `arg:"" required:"" help:"project key"`
	Days            int    `help:"days threshold for in-progress stagnation detection" default:"14"`
	IncludeComments bool   `help:"enable blocked-by-keyword detection via comments"`
	ExcludeStatus   string `help:"comma-separated status names to exclude (e.g. '完了,対応済み')"`
}

// Run は project blockers コマンドを実行する。
func (c *ProjectBlockersCmd) Run(g *GlobalFlags) error {
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

	cfg := analysis.BlockerConfig{
		InProgressDays:  c.Days,
		ExcludeStatus:   excludeStatus,
		IncludeComments: c.IncludeComments,
	}

	detector := analysis.NewBlockerDetector(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := detector.Detect(ctx, []string{c.ProjectKey}, cfg)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
