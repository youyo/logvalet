package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/digest"
)

// ProjectCmd は project コマンド群のルート。
type ProjectCmd struct {
	Get    ProjectGetCmd    `cmd:"" help:"プロジェクトを取得する"`
	List   ProjectListCmd   `cmd:"" help:"プロジェクト一覧を取得する"`
	Digest ProjectDigestCmd `cmd:"" help:"プロジェクトのダイジェストを生成する"`
}

// ProjectGetCmd は project get コマンド。
type ProjectGetCmd struct {
	ProjectKeyOrID string `arg:"" required:"" help:"プロジェクトキー または ID"`
}

func (c *ProjectGetCmd) Run(g *GlobalFlags) error {
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

// ProjectDigestCmd は project digest コマンド。
type ProjectDigestCmd struct {
	DigestFlags
	ProjectKeyOrID string `arg:"" required:"" help:"プロジェクトキー または ID"`
}

func (c *ProjectDigestCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	builder := digest.NewDefaultProjectDigestBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	envelope, err := builder.Build(ctx, c.ProjectKeyOrID, digest.ProjectDigestOptions{})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, envelope)
}
