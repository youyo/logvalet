package cli

import (
	"context"
	"os"
)

// MetaCmd は meta コマンド群のルート。
type MetaCmd struct {
	Status      MetaStatusCmd      `cmd:"" help:"list issue statuses"`
	Category    MetaCategoryCmd    `cmd:"" help:"list issue categories"`
	Version     MetaVersionCmd     `cmd:"" help:"list versions"`
	CustomField MetaCustomFieldCmd `cmd:"" help:"list custom fields"`
}

// MetaStatusCmd は meta status コマンド。
type MetaStatusCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key"`
}

func (c *MetaStatusCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	statuses, err := rc.Client.ListProjectStatuses(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, statuses)
}

// MetaCategoryCmd は meta category コマンド。
type MetaCategoryCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key"`
}

func (c *MetaCategoryCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	categories, err := rc.Client.ListProjectCategories(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, categories)
}

// MetaVersionCmd は meta version コマンド。
type MetaVersionCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key"`
}

func (c *MetaVersionCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	versions, err := rc.Client.ListProjectVersions(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, versions)
}

// MetaCustomFieldCmd は meta custom-field コマンド。
type MetaCustomFieldCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key"`
}

func (c *MetaCustomFieldCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	customFields, err := rc.Client.ListProjectCustomFields(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, customFields)
}
