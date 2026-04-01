package mcp

import (
	"context"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterActivityTools はアクティビティ関連の MCP tools を ToolRegistry に登録する。
func RegisterActivityTools(r *ToolRegistry) {
	// logvalet_activity_list
	r.Register(gomcp.NewTool("logvalet_activity_list",
		gomcp.WithDescription("List space activities"),
		gomcp.WithNumber("count", gomcp.Description("Max number of activities (default 20, max 100)")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		opt := backlog.ListActivitiesOptions{}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Count = count
		}
		return client.ListSpaceActivities(ctx, opt)
	})
}
