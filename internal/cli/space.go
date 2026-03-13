package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/digest"
)

// SpaceCmd は space コマンド群のルート。
type SpaceCmd struct {
	Info      SpaceInfoCmd      `cmd:"" help:"スペース情報を取得する"`
	DiskUsage SpaceDiskUsageCmd `cmd:"" help:"スペースのディスク使用量を取得する"`
	Digest    SpaceDigestCmd    `cmd:"" help:"スペースのダイジェストを生成する"`
}

// SpaceInfoCmd は space info コマンド。
type SpaceInfoCmd struct{}

func (c *SpaceInfoCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	space, err := rc.Client.GetSpace(ctx)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, space)
}

// SpaceDiskUsageCmd は space disk-usage コマンド。
type SpaceDiskUsageCmd struct{}

func (c *SpaceDiskUsageCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	usage, err := rc.Client.GetSpaceDiskUsage(ctx)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, usage)
}

// SpaceDigestCmd は space digest コマンド。
type SpaceDigestCmd struct {
	DigestFlags
}

func (c *SpaceDigestCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	builder := digest.NewDefaultSpaceDigestBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	opt := digest.SpaceDigestOptions{}
	envelope, err := builder.Build(ctx, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, envelope)
}
