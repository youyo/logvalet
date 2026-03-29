package mcp

import (
	"context"
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
}
