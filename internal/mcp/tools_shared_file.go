package mcp

import (
	"context"
	"encoding/base64"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterSharedFileTools は共有ファイル関連の MCP tools を ToolRegistry に登録する。
func RegisterSharedFileTools(r *ToolRegistry) {
	// logvalet_shared_file_list
	r.Register(gomcp.NewTool("logvalet_shared_file_list",
		gomcp.WithDescription("List shared files in a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key")),
		gomcp.WithString("path", gomcp.Description("Directory path within the project (default: root)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of files")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
		readOnlyAnnotation("共有ファイル一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		opt := backlog.ListSharedFilesOptions{}
		if path, ok := stringArg(args, "path"); ok {
			opt.Path = path
		}
		if limit, ok := intArg(args, "limit"); ok && limit > 0 {
			opt.Limit = limit
		}
		if offset, ok := intArg(args, "offset"); ok {
			opt.Offset = offset
		}
		return client.ListSharedFiles(ctx, projectKey, opt)
	})

	// logvalet_shared_file_download: B14
	r.Register(gomcp.NewTool("logvalet_shared_file_download",
		gomcp.WithDescription("Download a shared file (max 20MB, returned as base64)"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key")),
		gomcp.WithNumber("file_id", gomcp.Required(), gomcp.Description("Shared file ID")),
		readOnlyAnnotation("共有ファイルダウンロード"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		fileID, ok := intArg(args, "file_id")
		if !ok || fileID == 0 {
			return nil, fmt.Errorf("file_id is required")
		}
		const maxBytes = 20 * 1024 * 1024
		content, filename, contentType, err := client.DownloadSharedFileBounded(ctx, projectKey, int64(fileID), maxBytes)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(content),
			"filename":       filename,
			"content_type":   contentType,
			"size_bytes":     len(content),
		}, nil
	})
}
