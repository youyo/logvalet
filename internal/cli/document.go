package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// DocumentCmd は document コマンド群のルート。
type DocumentCmd struct {
	Get    DocumentGetCmd    `cmd:"" help:"get document"`
	List   DocumentListCmd   `cmd:"" help:"list documents"`
	Tree   DocumentTreeCmd   `cmd:"" help:"get document tree"`
	Digest DocumentDigestCmd `cmd:"" help:"generate document digest"`
	Create DocumentCreateCmd `cmd:"" help:"create document"`
}

// DocumentGetCmd は document get コマンド（spec §14.18）。
// lv document get <document_id>
type DocumentGetCmd struct {
	DocumentID string `arg:"" required:"" help:"document ID"`
}

func (c *DocumentGetCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	doc, err := rc.Client.GetDocument(ctx, c.DocumentID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, doc)
}

// DocumentListCmd は document list コマンド（spec §14.19）。
// lv document list --project <key>
type DocumentListCmd struct {
	ListFlags
	ProjectKey string `required:"" help:"project key"`
}

func (c *DocumentListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	// projectKey → projectID 変換
	proj, err := rc.Client.GetProject(ctx, c.ProjectKey)
	if err != nil {
		return fmt.Errorf("failed to resolve project key %q: %w", c.ProjectKey, err)
	}
	opt := backlog.ListDocumentsOptions{
		Limit:  c.Count,
		Offset: c.Offset,
	}
	docs, err := rc.Client.ListDocuments(ctx, proj.ID, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, docs)
}

// DocumentTreeCmd は document tree コマンド（spec §14.20）。
// lv document tree --project <key>
type DocumentTreeCmd struct {
	ProjectKey string `required:"" help:"project key"`
}

func (c *DocumentTreeCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	tree, err := rc.Client.GetDocumentTree(ctx, c.ProjectKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, tree)
}

// DocumentDigestCmd は document digest コマンド（spec §14.21）。
// lv document digest <document_id>
type DocumentDigestCmd struct {
	DigestFlags
	DocumentID string `arg:"" required:"" help:"document ID"`
}

func (c *DocumentDigestCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	builder := digest.NewDefaultDocumentDigestBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	envelope, err := builder.Build(ctx, c.DocumentID, digest.DocumentDigestOptions{})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, envelope)
}

// DocumentCreateCmd は document create コマンド（spec §14.22）。
// lv document create --project <key> --title <text> (--content <text> | --content-file <path>)
type DocumentCreateCmd struct {
	WriteFlags
	ProjectKey  string  `required:"" help:"project key"`
	Title       string  `required:"" help:"document title"`
	Content     string  `help:"document body (mutually exclusive with --content-file)"`
	ContentFile string  `help:"document body file path (mutually exclusive with --content)" type:"existingfile"`
	ParentID    *string `help:"parent document ID (optional)"`
	Emoji       string  `help:"emoji for title"`
	AddLast     bool    `help:"add at the end"`
}

func (c *DocumentCreateCmd) Run(g *GlobalFlags) error {
	// --content と --content-file の排他バリデーション（どちらか必須）
	if err := validateContentFlags(c.Content, c.ContentFile); err != nil {
		return err
	}

	// --content-file が指定された場合はファイルから本文を読み込む
	content := c.Content
	if c.ContentFile != "" {
		fileContent, err := readContentFromFile(c.ContentFile)
		if err != nil {
			return fmt.Errorf("failed to read --content-file: %w", err)
		}
		content = fileContent
	}

	if c.DryRun {
		params := map[string]interface{}{
			"project_key": c.ProjectKey,
			"title":       c.Title,
			"content":     content,
			"emoji":       nilIfEmpty(c.Emoji),
			"add_last":    c.AddLast,
		}
		if c.ParentID != nil {
			params["parent_id"] = *c.ParentID
		}
		renderer, rerr := buildRenderer(g)
		if rerr != nil {
			return rerr
		}
		return renderer.Render(os.Stdout, map[string]interface{}{
			"dry_run":   true,
			"operation": "document_create",
			"params":    params,
		})
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	// projectKey → projectID 変換
	proj, err := rc.Client.GetProject(ctx, c.ProjectKey)
	if err != nil {
		return fmt.Errorf("failed to resolve project key %q: %w", c.ProjectKey, err)
	}
	req := backlog.CreateDocumentRequest{
		ProjectID: proj.ID,
		Title:     c.Title,
		Content:   content,
		ParentID:  c.ParentID,
		Emoji:     c.Emoji,
		AddLast:   c.AddLast,
	}
	doc, err := rc.Client.CreateDocument(ctx, req)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, doc)
}
