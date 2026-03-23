package cli

import (
	"context"
	"os"
)

// TeamCmd は team コマンド群のルート。
type TeamCmd struct {
	List    TeamListCmd    `cmd:"" help:"チーム一覧を取得する"`
	Project TeamProjectCmd `cmd:"" help:"プロジェクトのチーム一覧を取得する"`
}

// TeamListCmd は team list コマンド。
type TeamListCmd struct {
	ListFlags
}

func (c *TeamListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	teams, err := rc.Client.ListTeams(ctx)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, teams)
}

// TeamProjectCmd は team project コマンド。
type TeamProjectCmd struct {
	ProjectKey string `arg:"" required:"" help:"プロジェクトキー"`
}

func (c *TeamProjectCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	teams, err := rc.Client.ListProjectTeams(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, teams)
}

