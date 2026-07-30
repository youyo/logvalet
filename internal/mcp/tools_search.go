package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// RegisterSearchTools は横断検索 MCP tool を登録する。
func RegisterSearchTools(r *ToolRegistry, cfg ServerConfig) {
	r.RegisterWithSpaces(NewToolDef("logvalet_search",
		WithDesc("Search issues, documents, and wiki pages by keyword"),
		WithStringParam("keyword", true, "Search keyword"),
		WithStringParam("project_keys", false, "Comma-separated project keys to filter (optional)"),
		WithNumberParam("count", false, "Max results per resource (1-100, default 20)"),
		WithNumberParam("offset", false, "Pagination offset per resource (default 0)"),
		WithStringParam("detail", false, "Verbosity: snippet | meta (default: snippet)"),
		WithAnnotation(readOnlyAnnotation("横断検索")),
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
