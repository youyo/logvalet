package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterWatchingTools はウォッチ関連の MCP tools を ToolRegistry に登録する。
func RegisterWatchingTools(r *ToolRegistry) {
	// logvalet_watching_list
	r.Register(gomcp.NewTool("logvalet_watching_list",
		gomcp.WithDescription("List watchings for a user. Returns issues being watched by the specified user."),
		gomcp.WithNumber("user_id", gomcp.Description("User ID (required)"), gomcp.Required()),
		gomcp.WithNumber("count", gomcp.Description("Max number of items (default: 20, max: 100)")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination (default: 0)")),
		gomcp.WithString("order", gomcp.Description("Sort order: asc or desc (default: desc)")),
		gomcp.WithString("sort", gomcp.Description("Sort key: created, updated, or issueUpdated")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		userID, ok := intArg(args, "user_id")
		if !ok || userID == 0 {
			return nil, fmt.Errorf("user_id is required")
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
	r.Register(gomcp.NewTool("logvalet_watching_count",
		gomcp.WithDescription("Get the count of watchings for a user."),
		gomcp.WithNumber("user_id", gomcp.Description("User ID (required)"), gomcp.Required()),
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
	r.Register(gomcp.NewTool("logvalet_watching_get",
		gomcp.WithDescription("Get watching detail by watching ID."),
		gomcp.WithNumber("watching_id", gomcp.Description("Watching ID (required)"), gomcp.Required()),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		watchingID, ok := intArg(args, "watching_id")
		if !ok || watchingID == 0 {
			return nil, fmt.Errorf("watching_id is required")
		}
		return client.GetWatching(ctx, int64(watchingID))
	})

	// logvalet_watching_add
	r.Register(gomcp.NewTool("logvalet_watching_add",
		gomcp.WithDescription("Add a watching for an issue. Returns the created watching."),
		gomcp.WithString("issue_id_or_key", gomcp.Description("Issue ID or key (e.g., PROJ-123) (required)"), gomcp.Required()),
		gomcp.WithString("note", gomcp.Description("Optional note for the watching")),
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
	r.Register(gomcp.NewTool("logvalet_watching_update",
		gomcp.WithDescription("Update the note of a watching."),
		gomcp.WithNumber("watching_id", gomcp.Description("Watching ID (required)"), gomcp.Required()),
		gomcp.WithString("note", gomcp.Description("New note for the watching (required)"), gomcp.Required()),
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
	r.Register(gomcp.NewTool("logvalet_watching_delete",
		gomcp.WithDescription("Delete a watching by watching ID. Returns the deleted watching."),
		gomcp.WithNumber("watching_id", gomcp.Description("Watching ID (required)"), gomcp.Required()),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		watchingID, ok := intArg(args, "watching_id")
		if !ok || watchingID == 0 {
			return nil, fmt.Errorf("watching_id is required")
		}
		return client.DeleteWatching(ctx, int64(watchingID))
	})

	// logvalet_watching_mark_as_read
	r.Register(gomcp.NewTool("logvalet_watching_mark_as_read",
		gomcp.WithDescription("Mark a watching as read by watching ID."),
		gomcp.WithNumber("watching_id", gomcp.Description("Watching ID (required)"), gomcp.Required()),
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
