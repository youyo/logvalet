package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/youyo/logvalet/internal/analysis"
)

// IssueTimelineCmd は issue timeline コマンド。
// 指定 issue のコメント・更新履歴を時系列で返す。
type IssueTimelineCmd struct {
	IssueKey         string `arg:"" required:"" help:"issue key (e.g., PROJ-123)"`
	MaxComments      int    `help:"max number of comments to include (0 = all)" default:"0"`
	IncludeUpdates   bool   `help:"include update history events" default:"true" negatable:""`
	MaxActivityPages int    `help:"max pages for activity pagination" default:"5"`
	Since            string `help:"filter events since date (YYYY-MM-DD)"`
	Until            string `help:"filter events until date (YYYY-MM-DD)"`
}

// Run は issue timeline コマンドを実行する。
func (c *IssueTimelineCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	opts := analysis.CommentTimelineOptions{
		MaxComments:      c.MaxComments,
		MaxActivityPages: c.MaxActivityPages,
	}

	// --include-updates / --no-include-updates
	opts.IncludeUpdates = &c.IncludeUpdates

	// --since
	if c.Since != "" {
		t, err := parseTimelineDate(c.Since)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}
		opts.Since = &t
	}

	// --until
	if c.Until != "" {
		t, err := parseTimelineDate(c.Until)
		if err != nil {
			return fmt.Errorf("invalid --until: %w", err)
		}
		opts.Until = &t
	}

	builder := analysis.NewCommentTimelineBuilder(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := builder.Build(ctx, c.IssueKey, opts)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}

// parseTimelineDate は "YYYY-MM-DD" 形式の文字列を time.Time に変換する。
func parseTimelineDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
