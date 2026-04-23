// Package mcp は logvalet MCP サーバーの実装を提供する。
// mark3labs/mcp-go を使用して Streamable HTTP MCP サーバーを起動する。
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
)

// ToolFunc は MCP tool ハンドラーの関数型。
// context.Context と backlog.Client、args map を受け取り、任意の結果またはエラーを返す。
type ToolFunc func(ctx context.Context, client backlog.Client, args map[string]any) (any, error)

// ToolRegistry は MCP サーバーへの tool 登録を管理する。
type ToolRegistry struct {
	server           *mcpserver.MCPServer
	client           backlog.Client
	factory          func(ctx context.Context) (backlog.Client, error)
	authorizationURL string
}

// NewToolRegistry は新しい ToolRegistry を返す。
// authorizationURL は OAuth 未接続エラー時に _meta に付与する認可 URL。
// 空文字列の場合は従来挙動（Meta なし）。
func NewToolRegistry(s *mcpserver.MCPServer, client backlog.Client, authorizationURL string) *ToolRegistry {
	return &ToolRegistry{server: s, client: client, authorizationURL: authorizationURL}
}

// NewToolRegistryWithFactory は ClientFactory を使って per-user の backlog.Client を
// 動的に生成する ToolRegistry を返す。
// factory は MCP ツール呼び出し時に context.Context からユーザーを特定し、
// そのユーザー用の backlog.Client を返す。
// authorizationURL は OAuth 未接続エラー時に _meta に付与する認可 URL。
// 空文字列の場合は従来挙動（Meta なし）。
func NewToolRegistryWithFactory(s *mcpserver.MCPServer, factory func(ctx context.Context) (backlog.Client, error), authorizationURL string) *ToolRegistry {
	return &ToolRegistry{server: s, factory: factory, authorizationURL: authorizationURL}
}

// Register は tool を MCPServer に登録する。
// ToolFunc が error を返した場合、自動的に mcp.NewToolResultError に変換する。
// factory が設定されている場合、リクエストの context から per-user クライアントを生成する。
// factory が ErrProviderNotConnected / ErrTokenRefreshFailed / ErrTokenExpired を返し、
// authorizationURL が設定されている場合は _meta.authorization_url を付与する。
func (r *ToolRegistry) Register(tool gomcp.Tool, fn ToolFunc) {
	r.server.AddTool(tool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		var c backlog.Client
		if r.factory != nil {
			var err error
			c, err = r.factory(ctx)
			if err != nil {
				if needsAuthorization(err) && r.authorizationURL != "" {
					return toolResultAuthRequired(err, r.authorizationURL), nil
				}
				return gomcp.NewToolResultError(err.Error()), nil
			}
		} else {
			c = r.client
		}
		result, err := fn(ctx, c, req.GetArguments())
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

// needsAuthorization は GetValidToken / factory のエラーが OAuth 認可を要求するかを判定する。
// ホワイトリスト方式で判定する。
func needsAuthorization(err error) bool {
	return errors.Is(err, auth.ErrProviderNotConnected) ||
		errors.Is(err, auth.ErrTokenRefreshFailed) ||
		errors.Is(err, auth.ErrTokenExpired)
}

// toolResultAuthRequired は認可 URL 付きのツールエラー結果を返す。
// _meta.authorization_required と _meta.authorization_url を含む。
func toolResultAuthRequired(err error, url string) *gomcp.CallToolResult {
	text := fmt.Sprintf(
		"Backlog authorization required. Open the following URL in your browser to connect:\n%s",
		url,
	)
	result := gomcp.NewToolResultError(text)
	result.Meta = &gomcp.Meta{
		AdditionalFields: map[string]any{
			"authorization_required": true,
			"authorization_url":      url,
		},
	}
	return result
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

// boolArg は args map から bool 引数を取り出すヘルパー。
// ok=false の場合は false を返す。
func boolArg(args map[string]any, key string) (bool, bool) {
	v, ok := args[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
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

// parseDateStr は "YYYY-MM-DD" 形式の文字列を time.Time に変換するヘルパー。
// MCP ツールの since/until 引数パース用。
func parseDateStr(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format (must be YYYY-MM-DD): %q", s)
	}
	return t, nil
}

// parseCSVIntList は "1,2,3" 形式の文字列を []int に変換する。
// 空文字列は nil を返す（未指定扱い）。
// 無効な整数が含まれる場合はエラー。
func parseCSVIntList(input, paramName string) ([]int, error) {
	if input == "" {
		return nil, nil
	}
	parts := strings.Split(input, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: must be comma-separated integers, got %q", paramName, input)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
