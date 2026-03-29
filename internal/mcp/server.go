package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
)

// NewServer は logvalet MCP サーバーを初期化して返す。
// すべての tool を登録済みの MCPServer を返す。
func NewServer(client backlog.Client, ver string) *mcpserver.MCPServer {
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

	return s
}
