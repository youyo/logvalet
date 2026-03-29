// Package mcp は logvalet MCP サーバーの実装を提供する。
// mark3labs/mcp-go を使用して Streamable HTTP MCP サーバーを起動する。
package mcp

import (
	"context"
	"encoding/json"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
)

// ToolFunc は MCP tool ハンドラーの関数型。
// context.Context と backlog.Client、args map を受け取り、任意の結果またはエラーを返す。
type ToolFunc func(ctx context.Context, client backlog.Client, args map[string]any) (any, error)

// ToolRegistry は MCP サーバーへの tool 登録を管理する。
type ToolRegistry struct {
	server *mcpserver.MCPServer
	client backlog.Client
}

// NewToolRegistry は新しい ToolRegistry を返す。
func NewToolRegistry(s *mcpserver.MCPServer, client backlog.Client) *ToolRegistry {
	return &ToolRegistry{server: s, client: client}
}

// Register は tool を MCPServer に登録する。
// ToolFunc が error を返した場合、自動的に mcp.NewToolResultError に変換する。
func (r *ToolRegistry) Register(tool gomcp.Tool, fn ToolFunc) {
	client := r.client
	r.server.AddTool(tool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		result, err := fn(ctx, client, req.GetArguments())
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return gomcp.NewToolResultError("failed to marshal result: " + err.Error()), nil
		}
		return gomcp.NewToolResultText(string(jsonBytes)), nil
	})
}

// stringArg は args map から文字列引数を取り出すヘルパー。
// ok=false の場合は空文字列を返す。
func stringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// intArg は args map から数値引数を取り出すヘルパー。
// JSON number は float64 として渡されるため int に変換する。
func intArg(args map[string]any, key string) (int, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
