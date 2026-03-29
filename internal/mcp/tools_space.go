package mcp

import (
	"context"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterSpaceTools はスペース関連の MCP tools を ToolRegistry に登録する。
func RegisterSpaceTools(r *ToolRegistry) {
	// logvalet_space_info
	r.Register(gomcp.NewTool("logvalet_space_info",
		gomcp.WithDescription("Get information about the Backlog space"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetSpace(ctx)
	})
}
