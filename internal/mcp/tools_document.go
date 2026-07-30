package mcp

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// RegisterDocumentTools はドキュメント関連の MCP tools を ToolRegistry に登録する。
func RegisterDocumentTools(r *ToolRegistry, cfg ServerConfig) {
	// logvalet_document_get
	r.RegisterWithSpaces(NewToolDef("logvalet_document_get",
		WithDesc("Get document by document ID"),
		WithStringParam("document_id", true, "Document ID"),
		WithAnnotation(readOnlyAnnotation("ドキュメント取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		documentID, ok := stringArg(args, "document_id")
		if !ok || documentID == "" {
			return nil, fmt.Errorf("document_id is required")
		}
		return client.GetDocument(ctx, documentID)
	})

	// logvalet_document_list
	r.RegisterWithSpaces(NewToolDef("logvalet_document_list",
		WithDesc("List documents in a project"),
		WithStringParam("project_key", true, "Project key (e.g. PROJ)"),
		WithNumberParam("count", false, "Max number of documents"),
		WithNumberParam("offset", false, "Offset for pagination"),
		WithAnnotation(readOnlyAnnotation("ドキュメント一覧取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		proj, err := client.GetProject(ctx, projectKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
		}
		opt := backlog.ListDocumentsOptions{}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Limit = count
		}
		if offset, ok := intArg(args, "offset"); ok {
			opt.Offset = offset
		}
		return client.ListDocuments(ctx, proj.ID, opt)
	})

	// logvalet_document_create
	r.RegisterWithSpacesWrite(NewToolDef("logvalet_document_create",
		WithDesc("Create a new document in a project"),
		WithNumberParam("project_id", true, "Project ID (numeric)"),
		WithStringParam("title", true, "Document title"),
		WithStringParam("content", true, "Document content (markdown)"),
		WithStringParam("parent_id", false, "Parent document ID (optional)"),
		WithAnnotation(writeAnnotation("ドキュメント作成", false)),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectID, ok := intArg(args, "project_id")
		if !ok || projectID == 0 {
			return nil, fmt.Errorf("project_id is required")
		}
		title, ok := stringArg(args, "title")
		if !ok || title == "" {
			return nil, fmt.Errorf("title is required")
		}
		content, ok := stringArg(args, "content")
		if !ok || content == "" {
			return nil, fmt.Errorf("content is required")
		}

		req := backlog.CreateDocumentRequest{
			ProjectID: projectID,
			Title:     title,
			Content:   content,
		}
		if parentID, ok := stringArg(args, "parent_id"); ok && parentID != "" {
			req.ParentID = &parentID
		}

		return client.CreateDocument(ctx, req)
	})

	// logvalet_document_tree: B5
	r.RegisterWithSpaces(NewToolDef("logvalet_document_tree",
		WithDesc("Get the document tree for a project"),
		WithStringParam("project_key", true, "Project key"),
		WithAnnotation(readOnlyAnnotation("ドキュメントツリー取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectKey, ok := stringArg(args, "project_key")
		if !ok || projectKey == "" {
			return nil, fmt.Errorf("project_key is required")
		}
		return client.GetDocumentTree(ctx, projectKey)
	})

	// logvalet_document_digest: B6
	r.RegisterWithSpaces(NewToolDef("logvalet_document_digest",
		WithDesc("Generate a digest for a document"),
		WithStringParam("document_id", true, "Document ID"),
		WithAnnotation(readOnlyAnnotation("ドキュメントダイジェスト生成")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		documentID, ok := stringArg(args, "document_id")
		if !ok || documentID == "" {
			return nil, fmt.Errorf("document_id is required")
		}
		spaceAlias, spaceBaseURL := spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)
		builder := digest.NewDefaultDocumentDigestBuilder(client, cfg.Profile, spaceAlias, spaceBaseURL)
		return builder.Build(ctx, documentID, digest.DocumentDigestOptions{})
	})

	// logvalet_document_search
	r.RegisterWithSpaces(NewToolDef("logvalet_document_search",
		WithDesc("Search documents by keyword within a Backlog space"),
		WithStringParam("keyword", true, "Search keyword"),
		WithStringParam("project_keys", false, "Comma-separated project keys to filter (e.g. PROJ1,PROJ2)"),
		WithNumberParam("count", false, "Max results (1-100, default 100)"),
		WithNumberParam("offset", false, "Pagination offset (default 0)"),
		WithStringParam("sort", false, "Sort field: created | updated"),
		WithStringParam("order", false, "Sort order: asc | desc"),
		WithStringParam("detail", false, "Verbosity: snippet | meta | full (default: snippet)"),
		WithAnnotation(readOnlyAnnotation("ドキュメント検索")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		keyword, ok := stringArg(args, "keyword")
		if !ok || keyword == "" {
			return nil, fmt.Errorf("keyword is required")
		}
		// count クランプ（上限 100）
		count := 100
		if c, ok := intArg(args, "count"); ok && c > 0 {
			count = c
		}
		if count > 100 {
			count = 100
		}
		// project_keys をカンマ区切り文字列から解決（任意）
		var projectIDs []int
		if projectKeys, ok := stringArg(args, "project_keys"); ok && projectKeys != "" {
			for _, key := range parseCSVStringList(projectKeys) {
				proj, err := client.GetProject(ctx, key)
				if err != nil {
					return nil, fmt.Errorf("failed to get project %s: %w", key, err)
				}
				projectIDs = append(projectIDs, proj.ID)
			}
		}
		offset := 0
		if o, ok := intArg(args, "offset"); ok && o > 0 {
			offset = o
		}
		opt := backlog.SearchDocumentsOptions{
			Keyword:    keyword,
			ProjectIDs: projectIDs,
			Count:      count,
			Offset:     offset,
		}
		if sort, ok := stringArg(args, "sort"); ok {
			opt.Sort = sort
		}
		if order, ok := stringArg(args, "order"); ok {
			opt.Order = order
		}
		docs, err := client.SearchDocuments(ctx, opt)
		if err != nil {
			return nil, err
		}
		detail := "snippet"
		if d, ok := stringArg(args, "detail"); ok && d != "" {
			detail = d
		}
		spaceAlias, spaceBaseURL := spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)
		builder := digest.NewDefaultDocumentSearchBuilder(client, cfg.Profile, spaceAlias, spaceBaseURL)
		return builder.Build(ctx, docs, digest.DocumentSearchOptions{
			Keyword: keyword,
			Detail:  detail,
			// RequestedCount は API の Count と同値（不一致だと possibly_more が偽陰性に戻る・AD11）
			RequestedCount: count,
			Offset:         offset,
		}), nil
	})
}
