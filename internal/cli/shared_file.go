package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
)

// SharedFileCmd は shared-file コマンド群のルート。
type SharedFileCmd struct {
	List     SharedFileListCmd     `cmd:"" help:"list shared files"`
	Get      SharedFileGetCmd      `cmd:"" help:"get shared file metadata"`
	Download SharedFileDownloadCmd `cmd:"" help:"download shared file"`
}

// SharedFileListCmd は shared-file list コマンド。
// lv shared-file list --project KEY [--path PATH] [--offset N] [--count N]
type SharedFileListCmd struct {
	ListFlags
	ProjectKey string `required:"" help:"project key"`
	Path       string `help:"directory path to list (default: root)"`
}

func (c *SharedFileListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	opt := backlog.ListSharedFilesOptions{
		Path:   c.Path,
		Limit:  c.Count,
		Offset: c.Offset,
	}
	files, err := rc.Client.ListSharedFiles(ctx, c.ProjectKey, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, files)
}

// SharedFileGetCmd は shared-file get コマンド。
// lv shared-file get --project KEY FILE-ID
type SharedFileGetCmd struct {
	ProjectKey string `required:"" help:"project key"`
	FileID     int64  `arg:"" required:"" help:"shared file ID"`
}

func (c *SharedFileGetCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	file, err := rc.Client.GetSharedFile(ctx, c.ProjectKey, c.FileID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, file)
}

// SharedFileDownloadCmd は shared-file download コマンド。
// lv shared-file download --project KEY FILE-ID [--output PATH]
type SharedFileDownloadCmd struct {
	ProjectKey string `required:"" help:"project key"`
	FileID     int64  `arg:"" required:"" help:"shared file ID"`
	Output     string `short:"o" help:"output file path (default: current directory with original filename)"`
}

func (c *SharedFileDownloadCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	body, filename, err := rc.Client.DownloadSharedFile(ctx, c.ProjectKey, c.FileID)
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
