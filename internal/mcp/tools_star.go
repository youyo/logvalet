package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterStarTools はスター関連の MCP tools を ToolRegistry に登録する。
func RegisterStarTools(r *ToolRegistry) {
	// logvalet_star_add
	r.RegisterWithSpacesWrite(NewToolDef("logvalet_star_add",
		WithDesc("Add a star to an issue, comment, wiki, pull request, or pull request comment. Specify exactly one of: issue_id, comment_id, wiki_id, pull_request_id, pull_request_comment_id"),
		WithNumberParam("issue_id", false, "Issue ID to star"),
		WithNumberParam("comment_id", false, "Comment ID to star"),
		WithNumberParam("wiki_id", false, "Wiki ID to star"),
		WithNumberParam("pull_request_id", false, "Pull request ID to star"),
		WithNumberParam("pull_request_comment_id", false, "Pull request comment ID to star"),
		WithAnnotation(writeAnnotation("スター追加", true)),
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
