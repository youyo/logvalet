package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/space"
)

// SearchCmd は Backlog リソースを keyword で横断検索するコマンド。
type SearchCmd struct {
	Keyword     string   `arg:"" required:"" help:"search keyword"`
	ProjectKeys []string `name:"project" help:"project key(s) to filter (optional, multiple)"`
	Count       int      `default:"20" help:"max results per resource (1-100)"`
	Offset      int      `default:"0" help:"pagination offset per resource"`
	Detail      string   `default:"snippet" help:"verbosity: snippet | meta"`
}

func (c *SearchCmd) Run(g *GlobalFlags) error {
	fanoutDone, err := runFanout(g, func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (*domain.DigestEnvelope, error) {
		builder := digest.NewDefaultSearchBuilder(client, g.Profile, reg.Alias, reg.BaseURL)
		return builder.Build(ctx, c.options())
	})
	if fanoutDone {
		return err
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	builder := digest.NewDefaultSearchBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	envelope, err := builder.Build(ctx, c.options())
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, envelope)
}

func (c *SearchCmd) options() digest.SearchOptions {
	return digest.SearchOptions{
		Keyword:     c.Keyword,
		ProjectKeys: c.ProjectKeys,
		Count:       c.Count,
		Offset:      c.Offset,
		Detail:      c.Detail,
	}
}
