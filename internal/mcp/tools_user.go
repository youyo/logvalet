package mcp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterUserTools はユーザー関連の MCP tools を ToolRegistry に登録する。
func RegisterUserTools(r *ToolRegistry) {
	// logvalet_user_list
	r.RegisterWithSpaces(NewToolDef("logvalet_user_list",
		WithDesc("List all users in the space"),
		WithAnnotation(readOnlyAnnotation("ユーザー一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.ListUsers(ctx)
	})

	// logvalet_user_get
	r.RegisterWithSpaces(NewToolDef("logvalet_user_get",
		WithDesc("Get user details by user ID"),
		WithStringParam("user_id", true, "User ID"),
		WithAnnotation(readOnlyAnnotation("ユーザー詳細取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userID, ok := stringArg(args, "user_id")
		if !ok || userID == "" {
			return nil, fmt.Errorf("user_id is required")
		}
		return client.GetUser(ctx, userID)
	})

	// logvalet_user_me: B1
	r.RegisterWithSpaces(NewToolDef("logvalet_user_me",
		WithDesc("Get the authenticated user's information"),
		WithAnnotation(readOnlyAnnotation("認証ユーザー情報取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetMyself(ctx)
	})

	// logvalet_user_activity: B2
	r.RegisterWithSpaces(NewToolDef("logvalet_user_activity",
		WithDesc("List activities for a specific user"),
		WithStringParam("user_id", true, "User ID or 'me' for current user"),
		WithStringParam("since", false, "Start date (YYYY-MM-DD)"),
		WithStringParam("until", false, "End date (YYYY-MM-DD)"),
		WithNumberParam("limit", false, "Max number of activities (default 20)"),
		WithStringParam("project", false, "Filter by project key (client-side filter)"),
		WithStringParam("activity_type_ids", false, "Comma-separated activity type IDs"),
		WithAnnotation(readOnlyAnnotation("ユーザーアクティビティ取得")),
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

		limit := 20
		if l, ok := intArg(args, "limit"); ok && l > 0 {
			limit = l
		}

		// activity_type_ids をパース
		fetchOpt := backlog.ListUserActivitiesOptions{}
		if activityTypeIDsStr, ok := stringArg(args, "activity_type_ids"); ok && activityTypeIDsStr != "" {
			ids, err := parseCSVIntList(activityTypeIDsStr, "activity_type_ids")
			if err != nil {
				return nil, err
			}
			fetchOpt.ActivityTypeIDs = ids
		}

		// since/until をパース（until は end-of-day に拡張）
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
			// YYYY-MM-DD は 00:00:00 になるため、その日の終わり（23:59:59）に拡張する
			eod := t.Add(24*time.Hour - time.Second)
			untilTime = &eod
		}

		// project フィルタは活動コンテンツが非構造化のため未適用
		_, _ = stringArg(args, "project")

		return backlog.FetchUserActivities(ctx, client, userID, sinceTime, untilTime, limit, fetchOpt)
	})
}
