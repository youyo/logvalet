package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/analysis"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterAnalysisTools は分析系の MCP tools を ToolRegistry に登録する。
func RegisterAnalysisTools(r *ToolRegistry, cfg ServerConfig) {
	// logvalet_issue_context
	r.Register(gomcp.NewTool("logvalet_issue_context",
		gomcp.WithDescription("Get structured issue context with signals and LLM hints for analysis"),
		gomcp.WithString("issue_key",
			gomcp.Required(),
			gomcp.Description("Issue key (e.g. PROJ-123)"),
		),
		gomcp.WithNumber("comments",
			gomcp.Description("Max number of recent comments to include (default 10)"),
		),
		gomcp.WithBoolean("compact",
			gomcp.Description("Omit description and comment bodies (default false)"),
		),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		issueKey, ok := stringArg(args, "issue_key")
		if !ok || issueKey == "" {
			return nil, fmt.Errorf("issue_key is required")
		}

		opts := analysis.IssueContextOptions{}
		if comments, ok := intArg(args, "comments"); ok && comments > 0 {
			opts.MaxComments = comments
		}
		if compact, ok := boolArg(args, "compact"); ok {
			opts.Compact = compact
		}

		builder := analysis.NewIssueContextBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)
		return builder.Build(ctx, issueKey, opts)
	})
}
