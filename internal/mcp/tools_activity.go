package mcp

import (
	"context"
	"fmt"
	"strconv"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// RegisterActivityTools はアクティビティ関連の MCP tools を ToolRegistry に登録する。
func RegisterActivityTools(r *ToolRegistry, cfg ServerConfig) {
	// logvalet_activity_list
	r.Register(gomcp.NewTool("logvalet_activity_list",
		gomcp.WithDescription("List activities by scope (space, project, or user)"),
		gomcp.WithString("user_id", gomcp.Description("User ID or 'me' for current user")),
		gomcp.WithString("project_key", gomcp.Description("Project key")),
		gomcp.WithNumber("count", gomcp.Description("Max number of activities (default 20, max 100)")),
		gomcp.WithString("activity_type_ids", gomcp.Description("Comma-separated activity type IDs to filter")),
		gomcp.WithString("order", gomcp.Description("Sort order: asc or desc (default: desc)")),
		readOnlyAnnotation("アクティビティ一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userID, hasUserID := stringArg(args, "user_id")
		projectKey, hasProjectKey := stringArg(args, "project_key")

		// user_id と project_key は排他的
		if hasUserID && hasProjectKey {
			return nil, fmt.Errorf("user_id and project_key are mutually exclusive")
		}

		// common options
		opt := backlog.ListActivitiesOptions{
			Count: 20, // default
		}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Count = count
		}
		if activityTypeIDsStr, ok := stringArg(args, "activity_type_ids"); ok && activityTypeIDsStr != "" {
			ids, err := parseCSVIntList(activityTypeIDsStr, "activity_type_ids")
			if err != nil {
				return nil, err
			}
			opt.ActivityTypeIDs = ids
		}
		if order, ok := stringArg(args, "order"); ok && order != "" {
			opt.Order = order
		}

		// ユーザー別アクティビティ
		if hasUserID {
			if userID != "me" {
				if _, err := strconv.Atoi(userID); err != nil {
					return nil, fmt.Errorf("user_id must be 'me' or a numeric ID, got: %s", userID)
				}
			}
			actualUserID := userID
			if userID == "me" {
				myself, err := client.GetMyself(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to get current user: %w", err)
				}
				actualUserID = strconv.Itoa(myself.ID)
			}

			userOpt := backlog.ListUserActivitiesOptions{
				ActivityTypeIDs: opt.ActivityTypeIDs,
				MinId:           opt.MinId,
				MaxId:           opt.MaxId,
				Count:           opt.Count,
				Order:           opt.Order,
			}
			return client.ListUserActivities(ctx, actualUserID, userOpt)
		}

		// プロジェクト別アクティビティ
		if hasProjectKey {
			return client.ListProjectActivities(ctx, projectKey, opt)
		}

		// デフォルト: スペースアクティビティ（後方互換）
		return client.ListSpaceActivities(ctx, opt)
	})

	// logvalet_activity_digest: B4
	r.Register(gomcp.NewTool("logvalet_activity_digest",
		gomcp.WithDescription("Generate an activity digest for a space or project"),
		gomcp.WithString("since", gomcp.Description("Start date (YYYY-MM-DD)")),
		gomcp.WithString("until", gomcp.Description("End date (YYYY-MM-DD)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of activities (default 20)")),
		gomcp.WithString("project", gomcp.Description("Filter by project key")),
		readOnlyAnnotation("アクティビティダイジェスト生成"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		opt := digest.ActivityDigestOptions{
			Limit: 20,
		}
		if limit, ok := intArg(args, "limit"); ok && limit > 0 {
			opt.Limit = limit
		}
		if project, ok := stringArg(args, "project"); ok {
			opt.Project = project
		}
		if sinceStr, ok := stringArg(args, "since"); ok && sinceStr != "" {
			t, err := parseDateStr(sinceStr)
			if err != nil {
				return nil, fmt.Errorf("invalid since: %w", err)
			}
			opt.Since = &t
		}
		if untilStr, ok := stringArg(args, "until"); ok && untilStr != "" {
			t, err := parseDateStr(untilStr)
			if err != nil {
				return nil, fmt.Errorf("invalid until: %w", err)
			}
			opt.Until = &t
		}

		builder := digest.NewDefaultActivityDigestBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)
		return builder.Build(ctx, opt)
	})
}
