package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/render"
)

// IssueRelatedCmd は issue related コマンド群のルート。
type IssueRelatedCmd struct {
	List   IssueRelatedListCmd   `cmd:"" help:"list related issues"`
	Add    IssueRelatedAddCmd    `cmd:"" help:"add related issue"`
	Remove IssueRelatedRemoveCmd `cmd:"" help:"remove related issue"`
}

// IssueRelatedListCmd は issue related list コマンド。
// lv issue related list ISSUE-KEY
type IssueRelatedListCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
}

func (c *IssueRelatedListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	return c.run(ctx, rc.Client, rc.Renderer, os.Stdout)
}

func (c *IssueRelatedListCmd) run(ctx context.Context, client backlog.Client, renderer render.Renderer, w io.Writer) error {
	related, err := client.ListRelatedIssues(ctx, c.IssueIDOrKey)
	if err != nil {
		return err
	}
	return renderer.Render(w, related)
}

// IssueRelatedAddCmd は issue related add コマンド。
// lv issue related add ISSUE-KEY TARGET-ISSUE-ID [--dry-run]
type IssueRelatedAddCmd struct {
	WriteFlags
	IssueIDOrKey  string `arg:"" required:"" help:"issue ID or key"`
	TargetIssueID int64  `arg:"" required:"" help:"target issue ID to relate"`
}

func (c *IssueRelatedAddCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"issue_key":       c.IssueIDOrKey,
			"target_issue_id": c.TargetIssueID,
		}
		data, err := formatDryRun("add_related_issue", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	return c.run(ctx, rc.Client, rc.Renderer, os.Stdout)
}

func (c *IssueRelatedAddCmd) run(ctx context.Context, client backlog.Client, renderer render.Renderer, w io.Writer) error {
	related, err := client.AddRelatedIssue(ctx, c.IssueIDOrKey, backlog.AddRelatedIssueRequest{
		TargetIssueID: c.TargetIssueID,
	})
	if err != nil {
		return err
	}
	return renderer.Render(w, related)
}

// IssueRelatedRemoveCmd は issue related remove コマンド。
// lv issue related remove ISSUE-KEY RELATED-ISSUE-ID [--dry-run]
type IssueRelatedRemoveCmd struct {
	WriteFlags
	IssueIDOrKey   string `arg:"" required:"" help:"issue ID or key"`
	RelatedIssueID int64  `arg:"" required:"" help:"related issue ID to remove"`
}

func (c *IssueRelatedRemoveCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"issue_key":        c.IssueIDOrKey,
			"related_issue_id": c.RelatedIssueID,
		}
		data, err := formatDryRun("remove_related_issue", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	return c.run(ctx, rc.Client, rc.Renderer, os.Stdout)
}

func (c *IssueRelatedRemoveCmd) run(ctx context.Context, client backlog.Client, renderer render.Renderer, w io.Writer) error {
	related, err := client.DeleteRelatedIssue(ctx, c.IssueIDOrKey, c.RelatedIssueID)
	if err != nil {
		return err
	}
	return renderer.Render(w, related)
}
