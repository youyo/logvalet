package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterStarTools はスター関連の MCP tools を ToolRegistry に登録する。
func RegisterStarTools(r *ToolRegistry) {
	// logvalet_star_add
	r.Register(gomcp.NewTool("logvalet_star_add",
		gomcp.WithDescription("Add a star to an issue, comment, wiki, pull request, or pull request comment. Specify exactly one of: issue_id, comment_id, wiki_id, pull_request_id, pull_request_comment_id"),
		gomcp.WithNumber("issue_id", gomcp.Description("Issue ID to star")),
		gomcp.WithNumber("comment_id", gomcp.Description("Comment ID to star")),
		gomcp.WithNumber("wiki_id", gomcp.Description("Wiki ID to star")),
		gomcp.WithNumber("pull_request_id", gomcp.Description("Pull request ID to star")),
		gomcp.WithNumber("pull_request_comment_id", gomcp.Description("Pull request comment ID to star")),
		writeAnnotation("スター追加", true),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		req := backlog.AddStarRequest{}
		count := 0

		if issueID, ok := intArg(args, "issue_id"); ok && issueID != 0 {
			req.IssueID = &issueID
			count++
		}
		if commentID, ok := intArg(args, "comment_id"); ok && commentID != 0 {
			req.CommentID = &commentID
			count++
		}
		if wikiID, ok := intArg(args, "wiki_id"); ok && wikiID != 0 {
			req.WikiID = &wikiID
			count++
		}
		if prID, ok := intArg(args, "pull_request_id"); ok && prID != 0 {
			req.PullRequestID = &prID
			count++
		}
		if prCommentID, ok := intArg(args, "pull_request_comment_id"); ok && prCommentID != 0 {
			req.PullRequestCommentID = &prCommentID
			count++
		}

		if count == 0 {
			return nil, fmt.Errorf("one of issue_id, comment_id, wiki_id, pull_request_id, pull_request_comment_id is required")
		}
		if count > 1 {
			return nil, fmt.Errorf("only one of issue_id, comment_id, wiki_id, pull_request_id, pull_request_comment_id can be specified")
		}

		if err := client.AddStar(ctx, req); err != nil {
			return nil, err
		}
		return map[string]string{"result": "ok"}, nil
	})
}
