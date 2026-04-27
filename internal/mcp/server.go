package mcp

import (
	"context"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
)

// ServerConfig は MCP サーバーの設定。
// analysis 系ツールが IssueContextBuilder に渡す情報を保持する。
type ServerConfig struct {
	Profile          string
	Space            string
	BaseURL          string
	AuthorizationURL string
}

// NewServer は logvalet MCP サーバーを単一 client で初期化して返す。
// すべての tool を登録済みの MCPServer を返す。
// 既存パス（CLI profile / API key 認証）で使用する。
func NewServer(client backlog.Client, ver string, cfg ServerConfig) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"logvalet",
		ver,
		mcpserver.WithToolCapabilities(true),
	)
	reg := NewToolRegistry(s, client, cfg.AuthorizationURL)
	registerAllTools(reg, cfg)
	return s
}

// NewServerWithFactory は per-user ClientFactory を使って logvalet MCP サーバーを初期化して返す。
// MCP ツール呼び出し時にリクエストの context.Context からユーザーを特定し、
// そのユーザーの Backlog OAuth トークンで backlog.Client を生成する。
// OAuth モード（--auth かつ LOGVALET_BACKLOG_CLIENT_ID 設定時）で使用する。
//
// factory には `auth.NewClientFactory(...)` で生成した ClientFactory を渡す。
// mcp → auth の import cycle を避けるため、引数型は匿名関数型で表現する。
func NewServerWithFactory(factory func(ctx context.Context) (backlog.Client, error), ver string, cfg ServerConfig) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"logvalet",
		ver,
		mcpserver.WithToolCapabilities(true),
	)
	reg := NewToolRegistryWithFactory(s, factory, cfg.AuthorizationURL)
	registerAllTools(reg, cfg)
	return s
}

// registerAllTools は MCP サーバーに全ツールを登録する共通ヘルパー。
// NewServer / NewServerWithFactory から呼ばれ、両者で同一のツールセットを保証する。
func registerAllTools(reg *ToolRegistry, cfg ServerConfig) {
	RegisterIssueTools(reg)
	RegisterProjectTools(reg)
	RegisterUserTools(reg)
	RegisterActivityTools(reg, cfg)
	RegisterDocumentTools(reg, cfg)
	RegisterTeamTools(reg)
	RegisterSpaceTools(reg, cfg)
	RegisterMetaTools(reg)
	RegisterSharedFileTools(reg)
	RegisterStarTools(reg)
	RegisterWatchingTools(reg)
	RegisterWikiTools(reg)
	RegisterAnalysisTools(reg, cfg)
}
