package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterMetaTools はプロジェクトメタデータ関連の MCP tools を ToolRegistry に登録する。
func RegisterMetaTools(r *ToolRegistry) {
	// logvalet_meta_statuses
	r.RegisterWithSpaces(NewToolDef("logvalet_meta_statuses",
		WithDesc("List statuses for a project"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("ステータス一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectStatuses(ctx, projectKey)
	})

	// logvalet_meta_issue_types
	r.RegisterWithSpaces(NewToolDef("logvalet_meta_issue_types",
		WithDesc("List issue types for a project"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("課題種別一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectIssueTypes(ctx, projectKey)
	})

	// logvalet_meta_categories
	r.RegisterWithSpaces(NewToolDef("logvalet_meta_categories",
		WithDesc("List categories for a project"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("カテゴリ一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectCategories(ctx, projectKey)
	})

	// logvalet_meta_version: B9
	r.RegisterWithSpaces(NewToolDef("logvalet_meta_version",
		WithDesc("List versions for a project"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("バージョン一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectVersions(ctx, projectKey)
	})

	// logvalet_meta_custom_field: B10
	r.RegisterWithSpaces(NewToolDef("logvalet_meta_custom_field",
		WithDesc("List custom field definitions for a project"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("カスタムフィールド一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListProjectCustomFields(ctx, projectKey)
	})
}
