package mcp

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterSharedFileTools は共有ファイル関連の MCP tools を ToolRegistry に登録する。
func RegisterSharedFileTools(r *ToolRegistry) {
	// logvalet_shared_file_list
	r.RegisterWithSpaces(NewToolDef("logvalet_shared_file_list",
		WithDesc("List shared files in a project"),
		WithStringParam("project_key", true, "Project key"),
		WithStringParam("path", false, "Directory path within the project (default: root)"),
		WithNumberParam("count", false, "Max number of files"),
		WithNumberParam("offset", false, "Offset for pagination"),
		WithAnnotation(readOnlyAnnotation("共有ファイル一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		opt := backlog.ListSharedFilesOptions{}
		if path, ok := stringArg(args, "path"); ok {
			opt.Path = path
		}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Limit = count
		}
		if offset, ok := intArg(args, "offset"); ok {
			opt.Offset = offset
		}
		return client.ListSharedFiles(ctx, projectKey, opt)
	})

	// logvalet_shared_file_download: B14
	r.RegisterWithSpaces(NewToolDef("logvalet_shared_file_download",
		WithDesc("Download a shared file (max 20MB, returned as base64)"),
		WithStringParam("project_key", true, "Project key"),
		WithNumberParam("file_id", true, "Shared file ID"),
		WithAnnotation(readOnlyAnnotation("共有ファイルダウンロード")),
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
