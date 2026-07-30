package mcp_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// testserver_test.go は SDK 非依存のテスト用 MCP サーバー代替を提供する。
//
// S11 (issue #52) で旧 SDK backend を削除した際、テストは「公式 SDK のサーバー型を
// 組み立てて内部ハンドラーを取り出す」代わりに、ServerBackend インターフェース
// (backend.go) の fake 実装へ全ツールを登録し、その ToolHandler を直接呼ぶ方式に
// 統一した。これによりテストは logvalet 独自の ToolDef / ToolResult のみを扱い、
// MCP SDK の型・トランスポート・プロトコル層に一切依存しない。
//
// SDK backend を実際に経由する end-to-end な検証 (tools/list が baseline と一致する、
// Stateless=true で initialize なしに tools/call が通る等) は
// backend_official_test.go が httptest 上の StreamableHTTPHandler に対して行う。

// newTestServer は本番同等の全ツールを登録した fake backend を返す。
// mcpinternal.NewServer(client, ver, cfg) のテスト版に相当する。
func newTestServer(t *testing.T, client backlog.Client, cfg mcpinternal.ServerConfig) *fakeBackend {
	t.Helper()
	backend := newFakeBackend()
	mcpinternal.BuildRegistryForTest(backend, client, nil, cfg)
	return backend
}

// newTestServerWithFactory は per-user ClientFactory 構成で全ツールを登録した
// fake backend を返す。mcpinternal.NewServerWithFactory(factory, ver, cfg) の
// テスト版に相当する。
func newTestServerWithFactory(
	t *testing.T,
	factory func(ctx context.Context) (backlog.Client, error),
	cfg mcpinternal.ServerConfig,
) *fakeBackend {
	t.Helper()
	backend := newFakeBackend()
	mcpinternal.BuildRegistryForTest(backend, nil, factory, cfg)
	return backend
}

// toolNames は登録済みツール名の集合を返す。登録ツール数の検証に使う。
func (b *fakeBackend) toolNames() map[string]struct{} {
	names := make(map[string]struct{}, len(b.registered))
	for name := range b.registered {
		names[name] = struct{}{}
	}
	return names
}

// callWithCtx は指定 ctx でツールハンドラーを呼び出す。
// fakeBackend.call の context 指定版。
func (b *fakeBackend) callWithCtx(t *testing.T, ctx context.Context, name string, args map[string]any) mcpinternal.ToolResult {
	t.Helper()
	entry, ok := b.registered[name]
	if !ok {
		t.Fatalf("tool %q not registered on backend", name)
	}
	result, err := entry.handler(ctx, args)
	if err != nil {
		t.Fatalf("handler for %q returned error: %v", name, err)
	}
	return result
}

// callTool は登録済みツールのハンドラーを context.Background() で呼び出す。
func callTool(t *testing.T, s *fakeBackend, toolName string, args map[string]any) mcpinternal.ToolResult {
	t.Helper()
	return s.callWithCtx(t, context.Background(), toolName, args)
}

// callToolWithCtx は登録済みツールのハンドラーを指定 ctx で呼び出す。
func callToolWithCtx(t *testing.T, s *fakeBackend, ctx context.Context, toolName string, args map[string]any) mcpinternal.ToolResult {
	t.Helper()
	return s.callWithCtx(t, ctx, toolName, args)
}

// resultTextContent は ToolResult の先頭 content を返す。
// ToolContent は SDK の Content インターフェースと違い text 専用の具象型なので、
// 旧テストにあった TextContent への型アサーションは不要になり、空チェックのみ行う。
func resultTextContent(t *testing.T, result mcpinternal.ToolResult) mcpinternal.ToolContent {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}
	return result.Content[0]
}

// GetTool は登録済みツールの ToolDef を返す。未登録なら nil。
// 旧テストの MCPServer.GetTool(name) と同じ「登録有無 + 定義参照」の用途に使う。
func (b *fakeBackend) GetTool(name string) *mcpinternal.ToolDef {
	entry, ok := b.registered[name]
	if !ok {
		return nil
	}
	tool := entry.tool
	return &tool
}

// toolProperties は ToolDef の inputSchema.properties を map[string]any で返す。
// 旧テストの SDK Tool.InputSchema.Properties と同じ形 (JSON Schema の properties)。
func toolProperties(t *testing.T, tool *mcpinternal.ToolDef) map[string]any {
	t.Helper()
	if tool == nil {
		t.Fatal("tool is nil")
	}
	props, ok := tool.InputSchemaJSON()["properties"].(map[string]any)
	if !ok {
		t.Fatalf("inputSchema.properties is not map[string]any for tool %q", tool.Name)
	}
	return props
}
