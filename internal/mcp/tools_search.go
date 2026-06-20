package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// RegisterSearchTools は横断検索 MCP tool を登録する。
func RegisterSearchTools(r *ToolRegistry, cfg ServerConfig) {
	r.RegisterWithSpaces(gomcp.NewTool("logvalet_search",
		gomcp.WithDescription("Search issues, documents, and wiki pages by keyword"),
		gomcp.WithString("keyword", gomcp.Required(), gomcp.Description("Search keyword")),
		gomcp.WithString("project_keys", gomcp.Description("Comma-separated project keys to filter (optional)")),
		gomcp.WithNumber("count", gomcp.Description("Max results per resource (1-100, default 20)")),
		gomcp.WithNumber("offset", gomcp.Description("Pagination offset per resource (default 0)")),
		gomcp.WithString("detail", gomcp.Description("Verbosity: snippet | meta (default: snippet)")),
		readOnlyAnnotation("横断検索"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		keyword, ok := stringArg(args, "keyword")
		if !ok || keyword == "" {
			return nil, fmt.Errorf("keyword is required")
		}
		count := 20
		if c, ok := intArg(args, "count"); ok {
			count = c
		}
		offset := 0
		if o, ok := intArg(args, "offset"); ok {
			offset = o
		}
		detail := "snippet"
		if d, ok := stringArg(args, "detail"); ok && d != "" {
			detail = d
		}
		projectKeys := []string(nil)
		if raw, ok := stringArg(args, "project_keys"); ok {
			projectKeys = parseCSVStringList(raw)
		}

		spaceAlias, spaceBaseURL := spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)
		builder := digest.NewDefaultSearchBuilder(client, cfg.Profile, spaceAlias, spaceBaseURL)
		return builder.Build(ctx, digest.SearchOptions{
			Keyword:     keyword,
			ProjectKeys: projectKeys,
			Count:       count,
			Offset:      offset,
			Detail:      detail,
		})
	})
}
