package cli

import (
	"context"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/render"
)

// WikiCmd は wiki コマンド群のルート。
type WikiCmd struct {
	List       WikiListCmd       `cmd:"" help:"list wiki pages"`
	Get        WikiGetCmd        `cmd:"" help:"get wiki page"`
	Count      WikiCountCmd      `cmd:"" help:"count wiki pages"`
	Tags       WikiTagsCmd       `cmd:"" help:"list wiki tags"`
	History    WikiHistoryCmd    `cmd:"" help:"get wiki page history"`
	Stars      WikiStarsCmd      `cmd:"" help:"list wiki page stars"`
	Attachment WikiAttachmentCmd `cmd:"" help:"manage wiki attachments"`
	SharedFile WikiSharedFileCmd `cmd:"" help:"manage wiki shared files"`
}

// WikiAttachmentCmd は wiki attachment コマンド群のルート。
type WikiAttachmentCmd struct {
	List WikiAttachmentListCmd `cmd:"" help:"list wiki attachments"`
}

// WikiSharedFileCmd は wiki sharedfile コマンド群のルート。
type WikiSharedFileCmd struct {
	List WikiSharedFileListCmd `cmd:"" help:"list wiki shared files"`
}

// WikiListCmd は wiki list コマンド。
// lv wiki list PROJECT-KEY [--keyword KEYWORD]
type WikiListCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key (e.g., PROJ)"`
	Keyword    string `short:"k" help:"keyword to search"`
}

func (c *WikiListCmd) Run(g *GlobalFlags) error {
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	return runWikiListWithClient(rc.Client, c, rc.Renderer)
}

// runWikiListWithClient はテスト可能な実行ヘルパー。
func runWikiListWithClient(client backlog.Client, cmd *WikiListCmd, renderer ...render.Renderer) error {
	ctx := context.Background()
	opt := backlog.ListWikisOptions{Keyword: cmd.Keyword}
	pages, err := client.ListWikis(ctx, cmd.ProjectKey, opt)
	if err != nil {
		return err
	}
	r := defaultRenderer(renderer)
	return r.Render(os.Stdout, pages)
}

// WikiGetCmd は wiki get コマンド。
// lv wiki get WIKI-ID
type WikiGetCmd struct {
	WikiID int64 `arg:"" required:"" help:"wiki page ID"`
}

func (c *WikiGetCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	page, err := rc.Client.GetWiki(ctx, c.WikiID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, page)
}

// WikiCountCmd は wiki count コマンド。
// lv wiki count PROJECT-KEY
type WikiCountCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key (e.g., PROJ)"`
}

func (c *WikiCountCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	count, err := rc.Client.CountWikis(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, map[string]int{"count": count})
}

// WikiTagsCmd は wiki tags コマンド。
// lv wiki tags PROJECT-KEY
type WikiTagsCmd struct {
	ProjectKey string `arg:"" required:"" help:"project key (e.g., PROJ)"`
}

func (c *WikiTagsCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	tags, err := rc.Client.ListWikiTags(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, tags)
}

// WikiHistoryCmd は wiki history コマンド。
// lv wiki history WIKI-ID [--min-id N] [--max-id N] [--count N] [--order asc|desc]
type WikiHistoryCmd struct {
	WikiID int64  `arg:"" required:"" help:"wiki page ID"`
	MinID  int    `help:"minimum history ID"`
	MaxID  int    `help:"maximum history ID"`
	Count  int    `help:"number of records (1-100, default 20)" default:"0"`
	Order  string `help:"sort order (asc|desc)" default:"" enum:"asc,desc,"`
}

func (c *WikiHistoryCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	opt := backlog.ListWikiHistoryOptions{
		MinID: c.MinID,
		MaxID: c.MaxID,
		Count: c.Count,
		Order: c.Order,
	}
	histories, err := rc.Client.GetWikiHistory(ctx, c.WikiID, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, histories)
}

// WikiStarsCmd は wiki stars コマンド。
// lv wiki stars WIKI-ID
type WikiStarsCmd struct {
	WikiID int64 `arg:"" required:"" help:"wiki page ID"`
}

func (c *WikiStarsCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	stars, err := rc.Client.GetWikiStars(ctx, c.WikiID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, stars)
}

// WikiAttachmentListCmd は wiki attachment list コマンド。
// lv wiki attachment list WIKI-ID
type WikiAttachmentListCmd struct {
	WikiID int64 `arg:"" required:"" help:"wiki page ID"`
}

func (c *WikiAttachmentListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	attachments, err := rc.Client.ListWikiAttachments(ctx, c.WikiID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, attachments)
}

// WikiSharedFileListCmd は wiki sharedfile list コマンド。
// lv wiki sharedfile list WIKI-ID
type WikiSharedFileListCmd struct {
	WikiID int64 `arg:"" required:"" help:"wiki page ID"`
}

func (c *WikiSharedFileListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	files, err := rc.Client.ListWikiSharedFiles(ctx, c.WikiID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, files)
}

// defaultRenderer はテスト用レンダラーが渡された場合はそれを使い、なければ JSON レンダラーを返す。
func defaultRenderer(renderers []render.Renderer) render.Renderer {
	if len(renderers) > 0 {
		return renderers[0]
	}
	r, _ := render.NewRenderer("json", false, "")
	return r
}
