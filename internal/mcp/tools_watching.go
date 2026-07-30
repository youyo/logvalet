package mcp

import (
	"context"
	"fmt"
	"strconv"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterWatchingTools はウォッチ関連の MCP tools を ToolRegistry に登録する。
func RegisterWatchingTools(r *ToolRegistry) {
	// logvalet_watching_list
	r.RegisterWithSpaces(NewToolDef("logvalet_watching_list",
		WithDesc("List watchings for a user. Returns issues being watched by the specified user."),
		WithStringParam("user_id", true, `User ID: "me" (resolved via GetMyself) or numeric user ID (e.g. "12345")`),
		WithNumberParam("count", false, "Max number of items (default: 20, max: 100)"),
		WithNumberParam("offset", false, "Offset for pagination (default: 0)"),
		WithStringParam("order", false, "Sort order: asc or desc (default: desc)"),
		WithStringParam("sort", false, "Sort key: created, updated, or issueUpdated"),
		WithAnnotation(readOnlyAnnotation("ウォッチ一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userIDStr, ok := stringArg(args, "user_id")
		if !ok || userIDStr == "" {
			return nil, fmt.Errorf("user_id is required")
		}
		userID, err := resolveUserIDForMCP(ctx, userIDStr, client)
		if err != nil {
			return nil, err
		}
		opt := backlog.ListWatchingsOptions{}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Count = count
		}
		if offset, ok := intArg(args, "offset"); ok && offset > 0 {
			opt.Offset = offset
		}
		if order, ok := stringArg(args, "order"); ok {
			opt.Order = order
		}
		if sort, ok := stringArg(args, "sort"); ok {
			opt.Sort = sort
		}
		return client.ListWatchings(ctx, userID, opt)
	})

	// logvalet_watching_count
	r.RegisterWithSpaces(NewToolDef("logvalet_watching_count",
		WithDesc("Get the count of watchings for a user."),
		WithNumberParam("user_id", true, "User ID (required)"),
		WithAnnotation(readOnlyAnnotation("ウォッチ数取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userID, ok := intArg(args, "user_id")
		if !ok || userID == 0 {
			return nil, fmt.Errorf("user_id is required")
		}
		count, err := client.CountWatchings(ctx, userID, backlog.ListWatchingsOptions{})
		if err != nil {
			return nil, err
		}
		return map[string]int{"count": count}, nil
	})

	// logvalet_watching_get
	r.RegisterWithSpaces(NewToolDef("logvalet_watching_get",
		WithDesc("Get watching detail by watching ID."),
		WithNumberParam("watching_id", true, "Watching ID (required)"),
		WithAnnotation(readOnlyAnnotation("ウォッチ詳細取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		watchingID, ok := intArg(args, "watching_id")
		if !ok || watchingID == 0 {
			return nil, fmt.Errorf("watching_id is required")
		}
		return client.GetWatching(ctx, int64(watchingID))
	})

	// logvalet_watching_add
	r.RegisterWithSpacesWrite(NewToolDef("logvalet_watching_add",
		WithDesc("Add a watching for an issue. Returns the created watching."),
		WithStringParam("issue_id_or_key", true, "Issue ID or key (e.g., PROJ-123) (required)"),
		WithStringParam("note", false, "Optional note for the watching"),
		WithAnnotation(writeAnnotation("ウォッチ追加", true)),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueIDOrKey, ok := stringArg(args, "issue_id_or_key")
		if !ok || issueIDOrKey == "" {
			return nil, fmt.Errorf("issue_id_or_key is required")
		}
		req := backlog.AddWatchingRequest{IssueIDOrKey: issueIDOrKey}
		if note, ok := stringArg(args, "note"); ok {
			req.Note = note
		}
		return client.AddWatching(ctx, req)
	})

	// logvalet_watching_update
	r.RegisterWithSpacesWrite(NewToolDef("logvalet_watching_update",
		WithDesc("Update the note of a watching."),
		WithNumberParam("watching_id", true, "Watching ID (required)"),
		WithStringParam("note", true, "New note for the watching (required)"),
		WithAnnotation(writeAnnotation("ウォッチ更新", true)),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		watchingID, ok := intArg(args, "watching_id")
		if !ok || watchingID == 0 {
			return nil, fmt.Errorf("watching_id is required")
		}
		note, ok := stringArg(args, "note")
		if !ok {
			return nil, fmt.Errorf("note is required")
		}
		req := backlog.UpdateWatchingRequest{Note: note}
		return client.UpdateWatching(ctx, int64(watchingID), req)
	})

	// logvalet_watching_delete
	r.RegisterWithSpacesWrite(NewToolDef("logvalet_watching_delete",
		WithDesc("Delete a watching by watching ID. Returns the deleted watching."),
		WithNumberParam("watching_id", true, "Watching ID (required)"),
		WithAnnotation(destructiveAnnotation("ウォッチ削除")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		watchingID, ok := intArg(args, "watching_id")
		if !ok || watchingID == 0 {
			return nil, fmt.Errorf("watching_id is required")
		}
		return client.DeleteWatching(ctx, int64(watchingID))
	})

	// logvalet_watching_mark_as_read
	r.RegisterWithSpacesWrite(NewToolDef("logvalet_watching_mark_as_read",
		WithDesc("Mark a watching as read by watching ID."),
		WithNumberParam("watching_id", true, "Watching ID (required)"),
		WithAnnotation(writeAnnotation("ウォッチ既読化", true)),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		watchingID, ok := intArg(args, "watching_id")
		if !ok || watchingID == 0 {
			return nil, fmt.Errorf("watching_id is required")
		}
		if err := client.MarkWatchingAsRead(ctx, int64(watchingID)); err != nil {
			return nil, err
		}
		return map[string]string{"result": "ok"}, nil
	})
}

// resolveUserIDForMCP は user_id 文字列を int の userID に解決する。
// "me" -> GetMyself を呼び出してユーザーIDを取得。
// 数値文字列 -> strconv.Atoi で変換。
// それ以外 -> エラー。
func resolveUserIDForMCP(ctx context.Context, input string, client backlog.Client) (int, error) {
	if input == "me" {
		user, err := client.GetMyself(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get myself: %w", err)
		}
		return user.ID, nil
	}
	id, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("user_id must be \"me\" or a numeric user ID, got %q", input)
	}
	return id, nil
}
