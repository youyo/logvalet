package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterWikiTools は Wiki 関連の MCP tools を ToolRegistry に登録する。
func RegisterWikiTools(r *ToolRegistry) {
	// logvalet_wiki_list
	r.Register(gomcp.NewTool("logvalet_wiki_list",
		gomcp.WithDescription("List wiki pages in a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key (e.g. PROJ)")),
		gomcp.WithString("keyword", gomcp.Description("Keyword to search in wiki pages")),
		readOnlyAnnotation("Wiki ページ一覧取得"),
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
	r.Register(gomcp.NewTool("logvalet_wiki_get",
		gomcp.WithDescription("Get a wiki page by ID"),
		gomcp.WithNumber("wiki_id", gomcp.Required(), gomcp.Description("Wiki page ID")),
		readOnlyAnnotation("Wiki ページ取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.GetWiki(ctx, int64(wikiIDInt))
	})

	// logvalet_wiki_count
	r.Register(gomcp.NewTool("logvalet_wiki_count",
		gomcp.WithDescription("Count wiki pages in a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key (e.g. PROJ)")),
		readOnlyAnnotation("Wiki ページ件数取得"),
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
	r.Register(gomcp.NewTool("logvalet_wiki_tags",
		gomcp.WithDescription("List wiki tags in a project"),
		gomcp.WithString("project_key", gomcp.Required(), gomcp.Description("Project key (e.g. PROJ)")),
		readOnlyAnnotation("Wiki タグ一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.ListWikiTags(ctx, projectKey)
	})

	// logvalet_wiki_history
	r.Register(gomcp.NewTool("logvalet_wiki_history",
		gomcp.WithDescription("Get wiki page history"),
		gomcp.WithNumber("wiki_id", gomcp.Required(), gomcp.Description("Wiki page ID")),
		gomcp.WithNumber("min_id", gomcp.Description("Minimum history ID")),
		gomcp.WithNumber("max_id", gomcp.Description("Maximum history ID")),
		gomcp.WithNumber("count", gomcp.Description("Number of records (1-100, default 20)")),
		gomcp.WithString("order", gomcp.Description("Sort order: asc or desc (default desc)")),
		readOnlyAnnotation("Wiki 履歴取得"),
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
	r.Register(gomcp.NewTool("logvalet_wiki_stars",
		gomcp.WithDescription("List stars on a wiki page"),
		gomcp.WithNumber("wiki_id", gomcp.Required(), gomcp.Description("Wiki page ID")),
		readOnlyAnnotation("Wiki スター一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.GetWikiStars(ctx, int64(wikiIDInt))
	})

	// logvalet_wiki_attachment_list
	r.Register(gomcp.NewTool("logvalet_wiki_attachment_list",
		gomcp.WithDescription("List attachments on a wiki page"),
		gomcp.WithNumber("wiki_id", gomcp.Required(), gomcp.Description("Wiki page ID")),
		readOnlyAnnotation("Wiki 添付ファイル一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.ListWikiAttachments(ctx, int64(wikiIDInt))
	})

	// logvalet_wiki_sharedfile_list
	r.Register(gomcp.NewTool("logvalet_wiki_sharedfile_list",
		gomcp.WithDescription("List shared files on a wiki page"),
		gomcp.WithNumber("wiki_id", gomcp.Required(), gomcp.Description("Wiki page ID")),
		readOnlyAnnotation("Wiki 共有ファイル一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		wikiIDInt, ok := intArg(args, "wiki_id")
		if !ok || wikiIDInt == 0 {
			return nil, fmt.Errorf("wiki_id is required")
		}
		return client.ListWikiSharedFiles(ctx, int64(wikiIDInt))
	})
}
