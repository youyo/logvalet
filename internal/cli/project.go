package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/space"
)

// ProjectCmd は project コマンド群のルート。
type ProjectCmd struct {
	Get      ProjectGetCmd      `cmd:"" help:"get project"`
	List     ProjectListCmd     `cmd:"" help:"list projects"`
	Blockers ProjectBlockersCmd `cmd:"" help:"detect project blockers"`
	Health   ProjectHealthCmd   `cmd:"" help:"show project health summary"`
}

// ProjectGetCmd は project get コマンド。
type ProjectGetCmd struct {
	ProjectKeyOrID string `arg:"" required:"" help:"project key or ID"`
}

func (c *ProjectGetCmd) Run(g *GlobalFlags) error {
	fanoutDone, err := runFanout(g, func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (*domain.Project, error) {
		return client.GetProject(ctx, c.ProjectKeyOrID)
	})
	if fanoutDone {
		return err
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	project, err := rc.Client.GetProject(ctx, c.ProjectKeyOrID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, project)
}

// ProjectListCmd は project list コマンド。
type ProjectListCmd struct {
	ListFlags
}

func (c *ProjectListCmd) Run(g *GlobalFlags) error {
	fanoutDone, err := runFanout(g, func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) ([]domain.Project, error) {
		return client.ListProjects(ctx)
	})
	if fanoutDone {
		return err
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	projects, err := rc.Client.ListProjects(ctx)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, projects)
}

