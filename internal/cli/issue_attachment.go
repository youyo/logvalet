package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/render"
)

// IssueAttachmentCmd は issue attachment コマンド群のルート。
type IssueAttachmentCmd struct {
	List     IssueAttachmentListCmd     `cmd:"" help:"list issue attachments"`
	Delete   IssueAttachmentDeleteCmd   `cmd:"" help:"delete issue attachment"`
	Download IssueAttachmentDownloadCmd `cmd:"" help:"download issue attachment"`
	Upload   IssueAttachmentUploadCmd   `cmd:"" help:"upload file(s) and attach to issue"`
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

// IssueAttachmentUploadCmd は issue attachment upload コマンド。
// lv issue attachment upload ISSUE-KEY FILE [FILE...]
type IssueAttachmentUploadCmd struct {
	WriteFlags
	IssueIDOrKey string   `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
	Files        []string `arg:"" required:"" help:"file path(s) to upload"`
}

func (c *IssueAttachmentUploadCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"issue_key": c.IssueIDOrKey,
			"files":     c.Files,
		}
		data, err := formatDryRun("upload_issue_attachment", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	return runIssueAttachmentUploadWithClient(rc.Client, c)
}

// runIssueAttachmentUploadWithClient はテスト可能な実行ヘルパー。
// 各ファイルをアップロードし、取得した ID で UpdateIssue を呼ぶ。
func runIssueAttachmentUploadWithClient(client backlog.Client, cmd *IssueAttachmentUploadCmd) error {
	ctx := context.Background()

	var attachmentIDs []int64
	for _, filePath := range cmd.Files {
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %q: %w", filePath, err)
		}
		defer f.Close() //nolint:errcheck

		filename := filepath.Base(filePath)
		att, err := client.UploadAttachment(ctx, filename, f)
		if err != nil {
			return fmt.Errorf("failed to upload %q: %w", filePath, err)
		}
		attachmentIDs = append(attachmentIDs, att.ID)
		fmt.Fprintf(os.Stderr, "uploaded: %s (id=%d)\n", filename, att.ID)
	}

	// attachmentId[] を指定して UpdateIssue を呼び、課題に添付する
	req := backlog.UpdateIssueRequest{
		AttachmentIDs: attachmentIDs,
	}
	issue, err := client.UpdateIssue(ctx, cmd.IssueIDOrKey, req)
	if err != nil {
		return fmt.Errorf("failed to attach files to %s: %w", cmd.IssueIDOrKey, err)
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		return err
	}
	return renderer.Render(os.Stdout, issue)
}
