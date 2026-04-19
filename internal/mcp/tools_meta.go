package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterMetaTools はプロジェクトメタデータ関連の MCP tools を ToolRegistry に登録する。
func RegisterMetaTools(r *ToolRegistry) {
	// logvalet_meta_statuses
	r.Register(gomcp.NewTool("logvalet_meta_statuses",
		gomcp.WithDescription("List statuses for a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key")),
		readOnlyAnnotation("ステータス一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectStatuses(ctx, projectKey)
	})

	// logvalet_meta_issue_types
	r.Register(gomcp.NewTool("logvalet_meta_issue_types",
		gomcp.WithDescription("List issue types for a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key")),
		readOnlyAnnotation("課題種別一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectIssueTypes(ctx, projectKey)
	})

	// logvalet_meta_categories
	r.Register(gomcp.NewTool("logvalet_meta_categories",
		gomcp.WithDescription("List categories for a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key")),
		readOnlyAnnotation("カテゴリ一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectCategories(ctx, projectKey)
	})
}
