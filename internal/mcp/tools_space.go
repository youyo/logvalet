package mcp

import (
	"context"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// RegisterSpaceTools はスペース関連の MCP tools を ToolRegistry に登録する。
func RegisterSpaceTools(r *ToolRegistry, cfg ServerConfig) {
	// logvalet_space_info
	r.RegisterWithSpaces(NewToolDef("logvalet_space_info",
		WithDesc("Get information about the Backlog space"),
		WithAnnotation(readOnlyAnnotation("スペース情報取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetSpace(ctx)
	})

	// logvalet_space_digest: B7
	r.RegisterWithSpaces(NewToolDef("logvalet_space_digest",
		WithDesc("Generate a digest for the entire Backlog space"),
		WithAnnotation(readOnlyAnnotation("スペースダイジェスト生成")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		spaceAlias, spaceBaseURL := spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)
		builder := digest.NewDefaultSpaceDigestBuilder(client, cfg.Profile, spaceAlias, spaceBaseURL)
		return builder.Build(ctx, digest.SpaceDigestOptions{})
	})

	// logvalet_space_disk_usage: B8
	r.RegisterWithSpaces(NewToolDef("logvalet_space_disk_usage",
		WithDesc("Get disk usage information for the Backlog space"),
		WithAnnotation(readOnlyAnnotation("スペースディスク使用量取得")),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.GetSpaceDiskUsage(ctx)
	})
}
