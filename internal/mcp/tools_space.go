package mcp

import (
	"context"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// RegisterSpaceTools はスペース関連の MCP tools を ToolRegistry に登録する。
func RegisterSpaceTools(r *ToolRegistry, cfg ServerConfig) {
	// logvalet_space_info
	r.Register(gomcp.NewTool("logvalet_space_info",
		gomcp.WithDescription("Get information about the Backlog space"),
		readOnlyAnnotation("スペース情報取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetSpace(ctx)
	})

	// logvalet_space_digest: B7
	r.Register(gomcp.NewTool("logvalet_space_digest",
		gomcp.WithDescription("Generate a digest for the entire Backlog space"),
		readOnlyAnnotation("スペースダイジェスト生成"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		builder := digest.NewDefaultSpaceDigestBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)
		return builder.Build(ctx, digest.SpaceDigestOptions{})
	})

	// logvalet_space_disk_usage: B8
	r.Register(gomcp.NewTool("logvalet_space_disk_usage",
		gomcp.WithDescription("Get disk usage information for the Backlog space"),
		readOnlyAnnotation("スペースディスク使用量取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetSpaceDiskUsage(ctx)
	})
}
