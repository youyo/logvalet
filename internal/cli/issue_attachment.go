package cli

import (
	"context"
	"fmt"
	"os"
)

// IssueAttachmentCmd は issue attachment コマンド群のルート。
type IssueAttachmentCmd struct {
	List     IssueAttachmentListCmd     `cmd:"" help:"list issue attachments"`
	Delete   IssueAttachmentDeleteCmd   `cmd:"" help:"delete issue attachment"`
	Download IssueAttachmentDownloadCmd `cmd:"" help:"download issue attachment"`
}

// IssueAttachmentListCmd は issue attachment list コマンド。
// lv issue attachment list ISSUE-KEY
type IssueAttachmentListCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
}

func (c *IssueAttachmentListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	attachments, err := rc.Client.ListIssueAttachments(ctx, c.IssueIDOrKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, attachments)
}

// IssueAttachmentDeleteCmd は issue attachment delete コマンド。
// lv issue attachment delete ISSUE-KEY ATTACHMENT-ID [--dry-run]
type IssueAttachmentDeleteCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key"`
	AttachmentID int64  `arg:"" required:"" help:"attachment ID"`
}

func (c *IssueAttachmentDeleteCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"issue_key":     c.IssueIDOrKey,
			"attachment_id": c.AttachmentID,
		}
		data, err := formatDryRun("delete_issue_attachment", params)
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
	attachment, err := rc.Client.DeleteIssueAttachment(ctx, c.IssueIDOrKey, c.AttachmentID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, attachment)
}

// IssueAttachmentDownloadCmd は issue attachment download コマンド。
// lv issue attachment download ISSUE-KEY ATTACHMENT-ID [--output PATH]
type IssueAttachmentDownloadCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key"`
	AttachmentID int64  `arg:"" required:"" help:"attachment ID"`
	Output       string `short:"o" help:"output file path (default: current directory with original filename)"`
}

func (c *IssueAttachmentDownloadCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	body, filename, err := rc.Client.DownloadIssueAttachment(ctx, c.IssueIDOrKey, c.AttachmentID)
	if err != nil {
		return err
	}
	dest, err := downloadToFile(body, filename, c.Output)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved: %s\n", dest)
	return nil
}
