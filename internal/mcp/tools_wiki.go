package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterWikiTools は Wiki 関連の MCP tools を ToolRegistry に登録する。
func RegisterWikiTools(r *ToolRegistry) {
	// logvalet_wiki_list
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_list",
		WithDesc("List wiki pages in a project"),
		WithStringParam("project_key", true, "Project key (e.g. PROJ)"),
		WithStringParam("keyword", false, "Keyword to search in wiki pages"),
		WithAnnotation(readOnlyAnnotation("Wiki ページ一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		opt := backlog.ListWikisOptions{}
		if keyword, ok := stringArg(args, "keyword"); ok {
			opt.Keyword = keyword
		}
		return client.ListWikis(ctx, projectKey, opt)
	})

	// logvalet_wiki_get
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_get",
		WithDesc("Get a wiki page by ID"),
		WithNumberParam("wiki_id", true, "Wiki page ID"),
		WithAnnotation(readOnlyAnnotation("Wiki ページ取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.GetWiki(ctx, int64(wikiIDInt))
	})

	// logvalet_wiki_count
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_count",
		WithDesc("Count wiki pages in a project"),
		WithStringParam("project_key", true, "Project key (e.g. PROJ)"),
		WithAnnotation(readOnlyAnnotation("Wiki ページ件数取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		count, err := client.CountWikis(ctx, projectKey)
		if err != nil {
			return nil, err
		}
		return map[string]int{"count": count}, nil
	})

	// logvalet_wiki_tags
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_tags",
		WithDesc("List wiki tags in a project"),
		WithStringParam("project_key", true, "Project key (e.g. PROJ)"),
		WithAnnotation(readOnlyAnnotation("Wiki タグ一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListWikiTags(ctx, projectKey)
	})

	// logvalet_wiki_history
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_history",
		WithDesc("Get wiki page history"),
		WithNumberParam("wiki_id", true, "Wiki page ID"),
		WithNumberParam("min_id", false, "Minimum history ID"),
		WithNumberParam("max_id", false, "Maximum history ID"),
		WithNumberParam("count", false, "Number of records (1-100, default 20)"),
		WithStringParam("order", false, "Sort order: asc or desc (default desc)"),
		WithAnnotation(readOnlyAnnotation("Wiki 履歴取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		opt := backlog.ListWikiHistoryOptions{}
		if minID, ok := intArg(args, "min_id"); ok {
			opt.MinID = minID
		}
		if maxID, ok := intArg(args, "max_id"); ok {
			opt.MaxID = maxID
		}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Count = count
		}
		if order, ok := stringArg(args, "order"); ok {
			opt.Order = order
		}
		return client.GetWikiHistory(ctx, int64(wikiIDInt), opt)
	})

	// logvalet_wiki_stars
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_stars",
		WithDesc("List stars on a wiki page"),
		WithNumberParam("wiki_id", true, "Wiki page ID"),
		WithAnnotation(readOnlyAnnotation("Wiki スター一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.GetWikiStars(ctx, int64(wikiIDInt))
	})

	// logvalet_wiki_attachment_list
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_attachment_list",
		WithDesc("List attachments on a wiki page"),
		WithNumberParam("wiki_id", true, "Wiki page ID"),
		WithAnnotation(readOnlyAnnotation("Wiki 添付ファイル一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.ListWikiAttachments(ctx, int64(wikiIDInt))
	})

	// logvalet_wiki_sharedfile_list
	r.RegisterWithSpaces(NewToolDef("logvalet_wiki_sharedfile_list",
		WithDesc("List shared files on a wiki page"),
		WithNumberParam("wiki_id", true, "Wiki page ID"),
		WithAnnotation(readOnlyAnnotation("Wiki 共有ファイル一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.ListWikiSharedFiles(ctx, int64(wikiIDInt))
	})
}
