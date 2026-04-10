package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterIssueTools は課題関連の MCP tools を ToolRegistry に登録する。
// logvalet_issue_get, list, create, update, comment 系, attachment 系 を含む。
func RegisterIssueTools(r *ToolRegistry) {
	// logvalet_issue_get
	r.Register(gomcp.NewTool("logvalet_issue_get",
		gomcp.WithDescription("Get issue details by issue key"),
		gomcp.WithString("issue_key",
			gomcp.Required(),
			gomcp.Description("Issue key (e.g. PROJECT-123)"),
		),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}
		return client.GetIssue(ctx, issueKey)
	})

	// logvalet_issue_list
	r.Register(gomcp.NewTool("logvalet_issue_list",
		gomcp.WithDescription("List issues with optional filters"),
		gomcp.WithString("project_key", gomcp.Description("Filter by single project key (legacy; use project_keys for multiple)")),
		gomcp.WithString("project_keys", gomcp.Description("Comma-separated project keys (e.g. PROJ1,PROJ2)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of issues (default 20, max 100)")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
		gomcp.WithString("sort", gomcp.Description("Sort field (e.g. updated, created)")),
		gomcp.WithString("order", gomcp.Description("Sort order (asc/desc)")),
		gomcp.WithString("assignee_id", gomcp.Description("Assignee filter: me (resolved via GetMyself) or numeric user ID")),
		gomcp.WithString("status_id", gomcp.Description("Status filter: not-closed (IDs 1,2,3) or comma-separated numeric IDs")),
		gomcp.WithString("due_date", gomcp.Description("Due date filter: overdue, this-week, today, this-month, YYYY-MM-DD, or YYYY-MM-DD:YYYY-MM-DD")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		opt := backlog.ListIssuesOptions{}

		if limit, ok := intArg(args, "limit"); ok && limit > 0 {
			opt.Limit = limit
		}
		if offset, ok := intArg(args, "offset"); ok {
			opt.Offset = offset
		}
		if sort, ok := stringArg(args, "sort"); ok {
			opt.Sort = sort
		}
		if order, ok := stringArg(args, "order"); ok {
			opt.Order = order
		}

		// project_key from legacy param
		if projectKey, ok := stringArg(args, "project_key"); ok && projectKey != "" {
			proj, err := client.GetProject(ctx, projectKey)
			if err != nil {
				return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
			}
			opt.ProjectIDs = append(opt.ProjectIDs, proj.ID)
		}

		// project_keys from comma-separated list
		if projectKeys, ok := stringArg(args, "project_keys"); ok && projectKeys != "" {
			for _, key := range strings.Split(projectKeys, ",") {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				proj, err := client.GetProject(ctx, key)
				if err != nil {
					return nil, fmt.Errorf("failed to get project %s: %w", key, err)
				}
				opt.ProjectIDs = append(opt.ProjectIDs, proj.ID)
			}
		}

		// assignee_id: "me" -> GetMyself, numeric -> use directly
		if assigneeIDStr, ok := stringArg(args, "assignee_id"); ok && assigneeIDStr != "" {
			ids, err := resolveAssigneeIDForMCP(ctx, assigneeIDStr, client)
			if err != nil {
				return nil, err
			}
			opt.AssigneeIDs = ids
		}

		// status_id: "not-closed" -> [1,2,3], comma-separated numeric IDs
		if statusIDStr, ok := stringArg(args, "status_id"); ok && statusIDStr != "" {
			ids, err := resolveStatusIDsForMCP(statusIDStr)
			if err != nil {
				return nil, err
			}
			opt.StatusIDs = ids
		}

		// due_date: keyword or date range
		if dueDateStr, ok := stringArg(args, "due_date"); ok && dueDateStr != "" {
			since, until, err := resolveDueDateForMCP(dueDateStr)
			if err != nil {
				return nil, err
			}
			opt.DueDateSince = since
			opt.DueDateUntil = until
		}

		return client.ListIssues(ctx, opt)
	})

	// logvalet_issue_create
	r.Register(gomcp.NewTool("logvalet_issue_create",
		gomcp.WithDescription("Create a new issue"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key")),
		gomcp.WithString("summary", gomcp.Required(), gomcp.Description("Issue summary")),
		gomcp.WithNumber("issue_type_id", gomcp.Required(), gomcp.Description("Issue type ID")),
		gomcp.WithString("description", gomcp.Description("Issue description")),
		gomcp.WithNumber("priority_id", gomcp.Description("Priority ID")),
		gomcp.WithNumber("assignee_id", gomcp.Description("Assignee user ID")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		summary, ok := stringArg(args, "summary")
		if !ok || summary == "" {
			return nil, fmt.Errorf("summary is required")
		}
		issueTypeID, ok := intArg(args, "issue_type_id")
		if !ok || issueTypeID == 0 {
			return nil, fmt.Errorf("issue_type_id is required")
		}

		proj, err := client.GetProject(ctx, projectKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
		}

		req := backlog.CreateIssueRequest{
			ProjectID:   proj.ID,
			Summary:     summary,
			IssueTypeID: issueTypeID,
		}
		if desc, ok := stringArg(args, "description"); ok {
			req.Description = desc
		}
		if priorityID, ok := intArg(args, "priority_id"); ok {
			req.PriorityID = priorityID
		}
		if assigneeID, ok := intArg(args, "assignee_id"); ok {
			req.AssigneeID = assigneeID
		}

		return client.CreateIssue(ctx, req)
	})

	// logvalet_issue_update
	r.Register(gomcp.NewTool("logvalet_issue_update",
		gomcp.WithDescription("Update an existing issue"),
		gomcp.WithString("issue_key", gomcp.Required(), gomcp.Description("Issue key (e.g. PROJECT-123)")),
		gomcp.WithString("summary", gomcp.Description("New summary")),
		gomcp.WithString("description", gomcp.Description("New description")),
		gomcp.WithNumber("status_id", gomcp.Description("Status ID")),
		gomcp.WithNumber("priority_id", gomcp.Description("Priority ID")),
		gomcp.WithNumber("assignee_id", gomcp.Description("Assignee user ID")),
		gomcp.WithString("comment", gomcp.Description("Comment to add with the update")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}

		req := backlog.UpdateIssueRequest{}
		if summary, ok := stringArg(args, "summary"); ok {
			req.Summary = &summary
		}
		if desc, ok := stringArg(args, "description"); ok {
			req.Description = &desc
		}
		if statusID, ok := intArg(args, "status_id"); ok {
			req.StatusID = &statusID
		}
		if priorityID, ok := intArg(args, "priority_id"); ok {
			req.PriorityID = &priorityID
		}
		if assigneeID, ok := intArg(args, "assignee_id"); ok {
			req.AssigneeID = &assigneeID
		}
		if comment, ok := stringArg(args, "comment"); ok {
			req.Comment = &comment
		}

		return client.UpdateIssue(ctx, issueKey, req)
	})

	// logvalet_issue_comment_list
	r.Register(gomcp.NewTool("logvalet_issue_comment_list",
		gomcp.WithDescription("List comments for an issue"),
		gomcp.WithString("issue_key", gomcp.Required(), gomcp.Description("Issue key (e.g. PROJECT-123)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of comments")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}
		opt := backlog.ListCommentsOptions{}
		if limit, ok := intArg(args, "limit"); ok {
			opt.Limit = limit
		}
		if offset, ok := intArg(args, "offset"); ok {
			opt.Offset = offset
		}
		return client.ListIssueComments(ctx, issueKey, opt)
	})

	// logvalet_issue_comment_add
	r.Register(gomcp.NewTool("logvalet_issue_comment_add",
		gomcp.WithDescription("Add a comment to an issue"),
		gomcp.WithString("issue_key", gomcp.Required(), gomcp.Description("Issue key (e.g. PROJECT-123)")),
		gomcp.WithString("content", gomcp.Required(), gomcp.Description("Comment content")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}
		content, ok := stringArg(args, "content")
		if !ok || content == "" {
			return nil, fmt.Errorf("content is required")
		}
		req := backlog.AddCommentRequest{Content: content}
		return client.AddIssueComment(ctx, issueKey, req)
	})

	// logvalet_issue_comment_update
	r.Register(gomcp.NewTool("logvalet_issue_comment_update",
		gomcp.WithDescription("Update a comment on an issue"),
		gomcp.WithString("issue_key", gomcp.Required(), gomcp.Description("Issue key (e.g. PROJECT-123)")),
		gomcp.WithNumber("comment_id", gomcp.Required(), gomcp.Description("Comment ID")),
		gomcp.WithString("content", gomcp.Required(), gomcp.Description("New comment content")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}
		commentID, ok := intArg(args, "comment_id")
		if !ok || commentID == 0 {
			return nil, fmt.Errorf("comment_id is required")
		}
		content, ok := stringArg(args, "content")
		if !ok || content == "" {
			return nil, fmt.Errorf("content is required")
		}
		req := backlog.UpdateCommentRequest{Content: content}
		return client.UpdateIssueComment(ctx, issueKey, int64(commentID), req)
	})

	// logvalet_issue_attachment_list
	r.Register(gomcp.NewTool("logvalet_issue_attachment_list",
		gomcp.WithDescription("List attachments for an issue"),
		gomcp.WithString("issue_key", gomcp.Required(), gomcp.Description("Issue key (e.g. PROJECT-123)")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}
		return client.ListIssueAttachments(ctx, issueKey)
	})
}

// resolveAssigneeIDForMCP resolves assignee_id param to AssigneeIDs for MCP.
// "me" -> GetMyself, numeric string -> ID directly, other -> error.
func resolveAssigneeIDForMCP(ctx context.Context, input string, client backlog.Client) ([]int, error) {
	if input == "me" {
		user, err := client.GetMyself(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get myself: %w", err)
		}
		return []int{user.ID}, nil
	}
	id, err := strconv.Atoi(input)
	if err != nil {
		return nil, fmt.Errorf("assignee_id must be me or a numeric user ID, got %q", input)
	}
	return []int{id}, nil
}

// resolveStatusIDsForMCP resolves status_id param to StatusIDs for MCP.
// "not-closed" -> [1,2,3], comma-separated numeric IDs -> parsed IDs.
func resolveStatusIDsForMCP(input string) ([]int, error) {
	if input == "not-closed" {
		return []int{1, 2, 3}, nil
	}
	parts := strings.Split(input, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("status_id must be not-closed or comma-separated numeric IDs, got %q", input)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("status_id must be not-closed or comma-separated numeric IDs, got %q", input)
	}
	return ids, nil
}

// resolveDueDateForMCP resolves due_date param to DueDateSince/DueDateUntil for MCP.
// Keywords: overdue, this-week, today, this-month. Date: YYYY-MM-DD or YYYY-MM-DD:YYYY-MM-DD.
func resolveDueDateForMCP(input string) (*time.Time, *time.Time, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	switch input {
	case "today":
		return &today, &today, nil
	case "overdue":
		yesterday := today.AddDate(0, 0, -1)
		return nil, &yesterday, nil
	case "this-week":
		monday := weekStartMCP(today)
		sunday := monday.AddDate(0, 0, 6)
		return &monday, &sunday, nil
	case "this-month":
		firstDay := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		lastDay := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location())
		return &firstDay, &lastDay, nil
	}

	if strings.Contains(input, ":") {
		since, until, err := parseDateRangeMCP(input)
		if err != nil {
			return nil, nil, fmt.Errorf("due_date must be one of: today, overdue, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD: %q", input)
		}
		return since, until, nil
	}

	t, err := time.Parse("2006-01-02", input)
	if err != nil {
		return nil, nil, fmt.Errorf("due_date must be one of: today, overdue, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD: %q", input)
	}
	return &t, &t, nil
}

// weekStartMCP returns the Monday of the week containing t.
func weekStartMCP(t time.Time) time.Time {
	weekday := t.Weekday()
	var offset int
	if weekday == time.Sunday {
		offset = -6
	} else {
		offset = -int(weekday - time.Monday)
	}
	return t.AddDate(0, 0, offset)
}

// parseDateRangeMCP parses a colon-separated date range.
// "A:B" -> Since=A, Until=B; "A:" -> Since=A, Until=nil; ":B" -> Since=nil, Until=B.
func parseDateRangeMCP(input string) (*time.Time, *time.Time, error) {
	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid colon-separated range: %q", input)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" && right == "" {
		return nil, nil, fmt.Errorf("range ':' has both sides empty")
	}
	var since, until *time.Time
	if left != "" {
		t, err := time.Parse("2006-01-02", left)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid date format (must be YYYY-MM-DD): %q", left)
		}
		since = &t
	}
	if right != "" {
		t, err := time.Parse("2006-01-02", right)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid date format (must be YYYY-MM-DD): %q", right)
		}
		until = &t
	}
	return since, until, nil
}
