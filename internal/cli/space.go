package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/space"
)

// SpaceCmd は space コマンド群のルート。
type SpaceCmd struct {
	Info      SpaceInfoCmd      `cmd:"" help:"get space information"`
	DiskUsage SpaceDiskUsageCmd `cmd:"" help:"get space disk usage"`
	Digest    SpaceDigestCmd    `cmd:"" help:"generate space digest"`
}

// SpaceInfoCmd は space info コマンド。
type SpaceInfoCmd struct{}

func (c *SpaceInfoCmd) Run(g *GlobalFlags) error {
	fanoutDone, err := runFanout(g, func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (*domain.Space, error) {
		return client.GetSpace(ctx)
	})
	if fanoutDone {
		return err
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	spaceInfo, err := rc.Client.GetSpace(ctx)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, spaceInfo)
}

// SpaceDiskUsageCmd は space disk-usage コマンド。
type SpaceDiskUsageCmd struct{}

func (c *SpaceDiskUsageCmd) Run(g *GlobalFlags) error {
	fanoutDone, err := runFanout(g, func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (*domain.DiskUsage, error) {
		return client.GetSpaceDiskUsage(ctx)
	})
	if fanoutDone {
		return err
	}

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
	fanoutDone, err := runFanout(g, func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (*domain.DigestEnvelope, error) {
		builder := digest.NewDefaultSpaceDigestBuilder(client, reg.Alias, reg.Tenant, reg.BaseURL)
		return builder.Build(ctx, digest.SpaceDigestOptions{})
	})
	if fanoutDone {
		return err
	}

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
