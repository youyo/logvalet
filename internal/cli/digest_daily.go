package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/analysis"
)

// DigestDailyCmd は lv digest daily コマンド。
// 指定プロジェクトの日次ペリオディックダイジェストを生成する。
type DigestDailyCmd struct {
	// ProjectKey はプロジェクトキー（必須）。
	ProjectKey string `short:"k" required:"" help:"project key"`
	// Since は集計開始日（YYYY-MM-DD, 省略時は 1 日前）。
	Since string `help:"since date (YYYY-MM-DD, default: 1 day ago)"`
	// Until は集計終了日（YYYY-MM-DD, 省略時は now）。
	Until string `help:"until date (YYYY-MM-DD, default: now)"`
}

// Run は digest daily コマンドを実行する。
func (c *DigestDailyCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	opts := analysis.PeriodicDigestOptions{Period: "daily"}

	if c.Since != "" {
		t, err := parseDate(c.Since)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}
		opts.Since = t
	}
	if c.Until != "" {
		t, err := parseDate(c.Until)
		if err != nil {
			return fmt.Errorf("invalid --until: %w", err)
		}
		opts.Until = t
	}

	builder := analysis.NewPeriodicDigestBuilder(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := builder.Build(ctx, c.ProjectKey, opts)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}
