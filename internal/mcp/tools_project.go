package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterProjectTools はプロジェクト関連の MCP tools を ToolRegistry に登録する。
func RegisterProjectTools(r *ToolRegistry) {
	// logvalet_project_get
	r.Register(gomcp.NewTool("logvalet_project_get",
		gomcp.WithDescription("Get project details by project key"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key (e.g. PROJECT)")),
		readOnlyAnnotation("プロジェクト詳細取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.GetProject(ctx, projectKey)
	})

	// logvalet_project_list
	r.Register(gomcp.NewTool("logvalet_project_list",
		gomcp.WithDescription("List all projects in the space"),
		readOnlyAnnotation("プロジェクト一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.ListProjects(ctx)
	})
}
