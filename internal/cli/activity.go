package cli

import (
	"context"
	"os"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// ActivityCmd は activity コマンド群のルート。
type ActivityCmd struct {
	List   ActivityListCmd   `cmd:"" help:"list activities"`
	Digest ActivityDigestCmd `cmd:"" help:"generate activity digest"`
	Stats  ActivityStatsCmd  `cmd:"" help:"show activity statistics (by type, actor, date, hour, patterns)"`
}

// ActivityListCmd は activity list コマンド（spec §14.12）。
type ActivityListCmd struct {
	ListFlags
	// Project はプロジェクトキーでフィルタ（指定時はプロジェクトのアクティビティのみ取得）。
	Project string `help:"filter by project key" env:"LOGVALET_PROJECT"`
	// Since は取得開始日時（ISO 8601 または duration: 30d, 1w 等）。
	Since string `help:"start date/time (ISO 8601 or duration)" env:"LOGVALET_SINCE"`
}

func (c *ActivityListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	opt := backlog.ListActivitiesOptions{
		Limit:  c.Count,
		Offset: c.Offset,
	}
	if c.Since != "" {
		t, parseErr := time.Parse(time.RFC3339, c.Since)
		if parseErr == nil {
			opt.Since = &t
		}
	}

	var activities interface{}
	if c.Project != "" {
		activities, err = rc.Client.ListProjectActivities(ctx, c.Project, opt)
	} else {
		activities, err = rc.Client.ListSpaceActivities(ctx, opt)
	}
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, activities)
}

// ActivityDigestCmd は activity digest コマンド（spec §14.13）。
// Since/Until/Limit は DigestFlags から継承する。
type ActivityDigestCmd struct {
	DigestFlags
	// Project はプロジェクトキーでフィルタ（指定時はプロジェクトのアクティビティのみ取得）。
	Project string `help:"filter by project key" env:"LOGVALET_PROJECT"`
}

func (c *ActivityDigestCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	builder := digest.NewDefaultActivityDigestBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	opt := digest.ActivityDigestOptions{
		Limit:   c.Limit,
		Project: c.Project,
	}
	if c.Since != "" {
		t, parseErr := time.Parse(time.RFC3339, c.Since)
		if parseErr == nil {
			opt.Since = &t
		}
	}
	if c.Until != "" {
		t, parseErr := time.Parse(time.RFC3339, c.Until)
		if parseErr == nil {
			opt.Until = &t
		}
	}
	envelope, err := builder.Build(ctx, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, envelope)
}
