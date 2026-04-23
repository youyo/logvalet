package cli

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// StarCmd は star コマンド群のルート。
type StarCmd struct {
	Add StarAddCmd `cmd:"" help:"add star to issue, comment, wiki, or pull request"`
}

// StarAddCmd は star add コマンド。
// lv star add (--issue-id ID | --comment-id ID | --wiki-id ID | --pr-id ID | --pr-comment-id ID)
// 指定されたフラグはちょうど1つでなければならない。
type StarAddCmd struct {
	IssueID     *int `help:"issue ID to star"`
	CommentID   *int `help:"issue comment ID to star"`
	WikiID      *int `help:"wiki ID to star"`
	PrID        *int `help:"pull request ID to star" name:"pull-request-id" aliases:"pr-id"`
	PrCommentID *int `help:"pull request comment ID to star" name:"pr-comment-id"`
}

func (c *StarAddCmd) Run(g *GlobalFlags) error {
	// 排他バリデーション: ちょうど1つ指定されている必要がある
	count := 0
	if c.IssueID != nil {
		count++
	}
	if c.CommentID != nil {
		count++
	}
	if c.WikiID != nil {
		count++
	}
	if c.PrID != nil {
		count++
	}
	if c.PrCommentID != nil {
		count++
	}
	if count == 0 {
		return fmt.Errorf("at least one of --issue-id, --comment-id, --wiki-id, --pull-request-id, --pr-comment-id must be specified")
	}
	if count > 1 {
		return fmt.Errorf("only one of --issue-id, --comment-id, --wiki-id, --pull-request-id, --pr-comment-id can be specified")
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	req := backlog.AddStarRequest{
		IssueID:              c.IssueID,
		CommentID:            c.CommentID,
		WikiID:               c.WikiID,
		PullRequestID:        c.PrID,
		PullRequestCommentID: c.PrCommentID,
	}
	return rc.Client.AddStar(ctx, req)
}
