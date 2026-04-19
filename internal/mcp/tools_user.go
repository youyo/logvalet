package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterUserTools はユーザー関連の MCP tools を ToolRegistry に登録する。
func RegisterUserTools(r *ToolRegistry) {
	// logvalet_user_list
	r.Register(gomcp.NewTool("logvalet_user_list",
		gomcp.WithDescription("List all users in the space"),
		readOnlyAnnotation("ユーザー一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.ListUsers(ctx)
	})

	// logvalet_user_get
	r.Register(gomcp.NewTool("logvalet_user_get",
		gomcp.WithDescription("Get user details by user ID"),
		gomcp.WithString("user_id", gomcp.Required(), gomcp.Description("User ID")),
		readOnlyAnnotation("ユーザー詳細取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userID, ok := stringArg(args, "user_id")
		if !ok || userID == "" {
			return nil, fmt.Errorf("user_id is required")
		}
		return client.GetUser(ctx, userID)
	})
}
