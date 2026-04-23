package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
)


// TeamCmd は team コマンド群のルート。
type TeamCmd struct {
	List    TeamListCmd    `cmd:"" help:"list teams"`
	Project TeamProjectCmd `cmd:"" help:"list project teams"`
}

// TeamListCmd は team list コマンド。
type TeamListCmd struct {
	ListFlags
	NoMembers bool `help:"exclude member information" name:"no-members"`
}

func (c *TeamListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	teams, err := rc.Client.ListTeams(ctx, backlog.ListTeamsOptions{})
	if err != nil {
		return err
	}
	if c.NoMembers {
		for i := range teams {
			teams[i].Members = nil
		}
	}
	return rc.Renderer.Render(os.Stdout, teams)
}

// TeamProjectCmd は team project コマンド。
type TeamProjectCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key"`
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

