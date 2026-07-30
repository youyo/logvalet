package mcp

import (
	"context"

	"github.com/youyo/logvalet/internal/backlog"
)

// ToolHandler は ServerBackend が tool 呼び出し時に実行する SDK 非依存のハンドラー型。
// プロトコル固有のリクエスト型 (gomcp.CallToolRequest 等) から抽出済みの args map を
// 受け取り、logvalet 独自の ToolResult を返す。
type ToolHandler func(ctx context.Context, args map[string]any) (ToolResult, error)

// ServerBackend は ToolRegistry が tool を登録する先の MCP サーバー実装を抽象化する
// インターフェース。ToolRegistry はこのインターフェースのみに依存し、
// mark3labs/mcp-go (tooldef_mark3labs.go) や将来追加される公式 Go SDK 実装は
// その1実装として並置できる。
type ServerBackend interface {
	// RegisterTool は tool を backend サーバーに登録する。tool 呼び出し時には
	// handler が呼ばれ、その戻り値が backend 固有の形式に変換されて返される。
	RegisterTool(tool ToolDef, handler ToolHandler)
}

// NewToolRegistryWithBackend は任意の ServerBackend 実装を使う ToolRegistry を返す。
// SDK 非依存の fake backend をテストで注入する場合や、mark3labs 以外の backend
// (公式 Go SDK 等) を使う場合の汎用エントリポイント。
func NewToolRegistryWithBackend(backend ServerBackend, client backlog.Client, authorizationURL string) *ToolRegistry {
	return &ToolRegistry{server: backend, client: client, authorizationURL: authorizationURL}
}
