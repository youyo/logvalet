package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/conventions"
)

func init() {
	toolCategories["logvalet_project_conventions"] = ToolCategorySpec{
		Category: CategoryReadOnly,
		Title:    "運用規約取得",
	}
}

// RegisterConventionsTools は運用規約関連の MCP tools を ToolRegistry に登録する。
func RegisterConventionsTools(r *ToolRegistry) {
	// logvalet_project_conventions
	r.RegisterWithSpaces(NewToolDef("logvalet_project_conventions",
		WithDesc("Show the operating conventions adopted by a project (rule issue), with a glossary"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("運用規約取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return conventions.Show(ctx, client, projectKey)
	})
}
