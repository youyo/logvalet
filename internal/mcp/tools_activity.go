package mcp

import (
	"context"
	"fmt"
	"strconv"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterActivityTools はアクティビティ関連の MCP tools を ToolRegistry に登録する。
func RegisterActivityTools(r *ToolRegistry) {
	// logvalet_activity_list
	r.Register(gomcp.NewTool("logvalet_activity_list",
		gomcp.WithDescription("List activities by scope (space, project, or user)"),
		gomcp.WithString("user_id", gomcp.Description("User ID or 'me' for current user")),
		gomcp.WithString("project_key", gomcp.Description("Project key")),
		gomcp.WithNumber("count", gomcp.Description("Max number of activities (default 20, max 100)")),
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
}
