package cli

import (
	"context"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/analysis"
)

// UserWorkloadCmd は user workload コマンド。
// 指定プロジェクトのユーザーごとの課題負荷を計算する。
type UserWorkloadCmd struct {
	ProjectKey    string `arg:"" required:"" help:"project key"`
	Days          int    `help:"days threshold for stale detection" default:"7"`
	ExcludeStatus string `help:"comma-separated status names to exclude (e.g. '完了,対応済み')"`
}

// Run は user workload コマンドを実行する。
func (c *UserWorkloadCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	var excludeStatus []string
	if c.ExcludeStatus != "" {
		excludeStatus = strings.Split(c.ExcludeStatus, ",")
	}

	cfg := analysis.WorkloadConfig{
		StaleDays:     c.Days,
		ExcludeStatus: excludeStatus,
	}

	calculator := analysis.NewWorkloadCalculator(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := calculator.Calculate(ctx, c.ProjectKey, cfg)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
