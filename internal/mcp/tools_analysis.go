package mcp

import (
	"context"
	"fmt"
	"strings"

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

	// logvalet_project_blockers
	r.Register(gomcp.NewTool("logvalet_project_blockers",
		gomcp.WithDescription("Detect project blocker issues (high priority unassigned, long in-progress, overdue)"),
		gomcp.WithString("project_keys",
			gomcp.Required(),
			gomcp.Description("Comma-separated project keys (e.g. 'PROJ1,PROJ2')"),
		),
		gomcp.WithNumber("days",
			gomcp.Description("Days threshold for in-progress stagnation (default 14)"),
		),
		gomcp.WithBoolean("include_comments",
			gomcp.Description("Enable blocked-by-keyword detection via latest comment (default false)"),
		),
		gomcp.WithString("exclude_status",
			gomcp.Description("Comma-separated status names to exclude (e.g. '完了,対応済み')"),
		),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKeysStr, ok := stringArg(args, "project_keys")
		if !ok || projectKeysStr == "" {
			return nil, fmt.Errorf("project_keys is required")
		}

		projectKeys := strings.Split(projectKeysStr, ",")

		blockerCfg := analysis.BlockerConfig{}
		if days, ok := intArg(args, "days"); ok && days > 0 {
			blockerCfg.InProgressDays = days
		}
		if includeComments, ok := boolArg(args, "include_comments"); ok {
			blockerCfg.IncludeComments = includeComments
		}
		if excludeStatusStr, ok := stringArg(args, "exclude_status"); ok && excludeStatusStr != "" {
			blockerCfg.ExcludeStatus = strings.Split(excludeStatusStr, ",")
		}

		detector := analysis.NewBlockerDetector(client, cfg.Profile, cfg.Space, cfg.BaseURL)
		return detector.Detect(ctx, projectKeys, blockerCfg)
	})

	// logvalet_issue_stale
	r.Register(gomcp.NewTool("logvalet_issue_stale",
		gomcp.WithDescription("Detect stale issues in specified projects"),
		gomcp.WithString("project_keys",
			gomcp.Required(),
			gomcp.Description("Comma-separated project keys (e.g. 'PROJ1,PROJ2')"),
		),
		gomcp.WithNumber("days",
			gomcp.Description("Days threshold for stale detection (default 7)"),
		),
		gomcp.WithString("exclude_status",
			gomcp.Description("Comma-separated status names to exclude (e.g. '完了,対応済み')"),
		),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKeysStr, ok := stringArg(args, "project_keys")
		if !ok || projectKeysStr == "" {
			return nil, fmt.Errorf("project_keys is required")
		}

		projectKeys := strings.Split(projectKeysStr, ",")

		staleCfg := analysis.StaleConfig{}
		if days, ok := intArg(args, "days"); ok && days > 0 {
			staleCfg.DefaultDays = days
		}
		if excludeStatusStr, ok := stringArg(args, "exclude_status"); ok && excludeStatusStr != "" {
			staleCfg.ExcludeStatus = strings.Split(excludeStatusStr, ",")
		}

		detector := analysis.NewStaleIssueDetector(client, cfg.Profile, cfg.Space, cfg.BaseURL)
		return detector.Detect(ctx, projectKeys, staleCfg)
	})
}
