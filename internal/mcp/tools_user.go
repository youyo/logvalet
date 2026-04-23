package mcp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
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

	// logvalet_user_me: B1
	r.Register(gomcp.NewTool("logvalet_user_me",
		gomcp.WithDescription("Get the authenticated user's information"),
		readOnlyAnnotation("認証ユーザー情報取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetMyself(ctx)
	})

	// logvalet_user_activity: B2
	r.Register(gomcp.NewTool("logvalet_user_activity",
		gomcp.WithDescription("List activities for a specific user"),
		gomcp.WithString("user_id", gomcp.Required(), gomcp.Description("User ID or 'me' for current user")),
		gomcp.WithString("since", gomcp.Description("Start date (YYYY-MM-DD)")),
		gomcp.WithString("until", gomcp.Description("End date (YYYY-MM-DD)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of activities (default 20)")),
		gomcp.WithString("project", gomcp.Description("Filter by project key (client-side filter)")),
		gomcp.WithString("activity_type_ids", gomcp.Description("Comma-separated activity type IDs")),
		readOnlyAnnotation("ユーザーアクティビティ取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userID, ok := stringArg(args, "user_id")
		if !ok || userID == "" {
			return nil, fmt.Errorf("user_id is required")
		}

		// user_id="me" を解決
		if userID == "me" {
			myself, err := client.GetMyself(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get current user: %w", err)
			}
			userID = strconv.Itoa(myself.ID)
		}

		opt := backlog.ListUserActivitiesOptions{
			Count: 20,
		}
		if limit, ok := intArg(args, "limit"); ok && limit > 0 {
			opt.Count = limit
		}
		if activityTypeIDsStr, ok := stringArg(args, "activity_type_ids"); ok && activityTypeIDsStr != "" {
			ids, err := parseCSVIntList(activityTypeIDsStr, "activity_type_ids")
			if err != nil {
				return nil, err
			}
			opt.ActivityTypeIDs = ids
		}

		activities, err := client.ListUserActivities(ctx, userID, opt)
		if err != nil {
			return nil, err
		}

		// since/until のクライアント側フィルタ（API 非対応）
		var sinceTime, untilTime *time.Time
		if sinceStr, ok := stringArg(args, "since"); ok && sinceStr != "" {
			t, err := parseDateStr(sinceStr)
			if err != nil {
				return nil, fmt.Errorf("invalid since: %w", err)
			}
			sinceTime = &t
		}
		if untilStr, ok := stringArg(args, "until"); ok && untilStr != "" {
			t, err := parseDateStr(untilStr)
			if err != nil {
				return nil, fmt.Errorf("invalid until: %w", err)
			}
			untilTime = &t
		}
		projectFilter, hasProjectFilter := stringArg(args, "project")

		// フィルタが不要な場合はそのまま返す
		if sinceTime == nil && untilTime == nil && !hasProjectFilter {
			return activities, nil
		}

		// クライアント側フィルタ適用（since/until のみ。project は Content に構造化情報がないためスキップ）
		_ = projectFilter // project フィルタは活動コンテンツが非構造化のため未適用
		filtered := make([]domain.Activity, 0, len(activities))
		for _, a := range activities {
			if sinceTime != nil && a.Created != nil && a.Created.Before(*sinceTime) {
				continue
			}
			if untilTime != nil && a.Created != nil && a.Created.After(*untilTime) {
				continue
			}
			filtered = append(filtered, a)
		}
		return filtered, nil
	})
}
