package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterDocumentTools はドキュメント関連の MCP tools を ToolRegistry に登録する。
func RegisterDocumentTools(r *ToolRegistry) {
	// logvalet_document_get
	r.Register(gomcp.NewTool("logvalet_document_get",
		gomcp.WithDescription("Get document by document ID"),
		gomcp.WithString("document_id", gomcp.Required(), gomcp.Description("Document ID")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		documentID, ok := stringArg(args, "document_id")
		if !ok || documentID == "" {
			return nil, fmt.Errorf("document_id is required")
		}
		return client.GetDocument(ctx, documentID)
	})

	// logvalet_document_list
	r.Register(gomcp.NewTool("logvalet_document_list",
		gomcp.WithDescription("List documents in a project"),
		gomcp.WithNumber("project_id", gomcp.Required(), gomcp.Description("Project ID (numeric)")),
		gomcp.WithNumber("limit", gomcp.Description("Max number of documents")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		projectID, ok := intArg(args, "project_id")
		if !ok || projectID == 0 {
			return nil, fmt.Errorf("project_id is required")
		}
		opt := backlog.ListDocumentsOptions{}
		if limit, ok := intArg(args, "limit"); ok && limit > 0 {
			opt.Limit = limit
		}
		if offset, ok := intArg(args, "offset"); ok {
			opt.Offset = offset
		}
		return client.ListDocuments(ctx, projectID, opt)
	})

	// logvalet_document_create
	r.Register(gomcp.NewTool("logvalet_document_create",
		gomcp.WithDescription("Create a new document in a project"),
		gomcp.WithNumber("project_id", gomcp.Required(), gomcp.Description("Project ID (numeric)")),
		gomcp.WithString("title", gomcp.Required(), gomcp.Description("Document title")),
		gomcp.WithString("content", gomcp.Required(), gomcp.Description("Document content (markdown)")),
		gomcp.WithString("parent_id", gomcp.Description("Parent document ID (optional)")),
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
}
