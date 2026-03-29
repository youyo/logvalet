package mcp

import (
	"context"
	"fmt"

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
		gomcp.WithString("project_key", gomcp.Description("Filter by project key (looks up project ID)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of issues (default 20, max 100)")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
		gomcp.WithString("sort", gomcp.Description("Sort field (e.g. updated, created)")),
		gomcp.WithString("order", gomcp.Description("Sort order (asc/desc)")),
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
		// project_key からプロジェクト ID を解決
		if projectKey, ok := stringArg(args, "project_key"); ok && projectKey != "" {
			proj, err := client.GetProject(ctx, projectKey)
			if err != nil {
				return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
			}
			opt.ProjectIDs = []int{proj.ID}
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
