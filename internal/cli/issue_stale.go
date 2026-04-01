package cli

import (
	"context"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/analysis"
)

// IssueStaleCmd は issue stale コマンド。
// 指定プロジェクトの停滞課題を検出する。
type IssueStaleCmd struct {
	ProjectKey    []string `short:"k" required:"" help:"project key (multiple allowed)"`
	Days          int      `help:"days threshold for stale detection" default:"7"`
	ExcludeStatus string   `help:"comma-separated status names to exclude (e.g. '完了,対応済み')"`
}

// Run は issue stale コマンドを実行する。
func (c *IssueStaleCmd) Run(g *GlobalFlags) error {
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

	cfg := analysis.StaleConfig{
		DefaultDays:   c.Days,
		ExcludeStatus: excludeStatus,
	}

	detector := analysis.NewStaleIssueDetector(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := detector.Detect(ctx, c.ProjectKey, cfg)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
