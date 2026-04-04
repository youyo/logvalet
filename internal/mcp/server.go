package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
)

// ServerConfig は MCP サーバーの設定。
// analysis 系ツールが IssueContextBuilder に渡す情報を保持する。
type ServerConfig struct {
	Profile string
	Space   string
	BaseURL string
}

// NewServer は logvalet MCP サーバーを初期化して返す。
// すべての tool を登録済みの MCPServer を返す。
func NewServer(client backlog.Client, ver string, cfg ServerConfig) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"logvalet",
		ver,
		mcpserver.WithToolCapabilities(true),
	)
	reg := NewToolRegistry(s, client)

	RegisterIssueTools(reg)
	RegisterProjectTools(reg)
	RegisterUserTools(reg)
	RegisterActivityTools(reg)
	RegisterDocumentTools(reg)
	RegisterTeamTools(reg)
	RegisterSpaceTools(reg)
	RegisterMetaTools(reg)
	RegisterSharedFileTools(reg)
	RegisterStarTools(reg)
	RegisterWatchingTools(reg)
	RegisterAnalysisTools(reg, cfg)

	return s
}
