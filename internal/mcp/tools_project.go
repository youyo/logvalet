package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterProjectTools はプロジェクト関連の MCP tools を ToolRegistry に登録する。
func RegisterProjectTools(r *ToolRegistry) {
	// logvalet_project_get
	r.RegisterWithSpaces(NewToolDef("logvalet_project_get",
		WithDesc("Get project details by project key"),
		WithStringParam("project_key", true, "Project key (e.g. PROJECT)"),
		WithAnnotation(readOnlyAnnotation("プロジェクト詳細取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.GetProject(ctx, projectKey)
	})

	// logvalet_project_list
	r.RegisterWithSpaces(NewToolDef("logvalet_project_list",
		WithDesc("List all projects in the space"),
		WithAnnotation(readOnlyAnnotation("プロジェクト一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.ListProjects(ctx)
	})
}
