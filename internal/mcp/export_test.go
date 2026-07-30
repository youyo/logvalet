package mcp

import (
	"context"

	"github.com/youyo/logvalet/internal/backlog"
)

// export_test.go: 同パッケージ内部の unexported ヘルパーを
// package mcp_test 側のテストから利用するための test-only エクスポート。

// SpaceInfoFromContextForTest は spaceInfoFromContext を外部テストへ公開する。
var SpaceInfoFromContextForTest = spaceInfoFromContext

// BuildRegistryForTest は buildRegistry (server.go) を外部テストへ公開する。
//
// テストは fake ServerBackend (backend_test.go) を渡すことで、公式 Go SDK の
// サーバー型やトランスポートを一切経由せずに NewServer / NewServerWithFactory と
// 同一のツールセット・同一の ToolRegistry 構成を得られる。
// client と factory はどちらか一方を渡す (factory != nil が優先)。
func BuildRegistryForTest(
	backend ServerBackend,
	client backlog.Client,
	factory func(ctx context.Context) (backlog.Client, error),
	cfg ServerConfig,
) *ToolRegistry {
	return buildRegistry(backend, client, factory, cfg)
}

// captureBackend は登録された ToolDef を name 順に保持するだけの ServerBackend 実装。
// package mcp 内のテストが SDK 型を経由せずに「本番で登録される全ツール定義」を
// 取り出すために使う (package mcp_test 側の fakeBackend と同じ役割)。
type captureBackend struct {
	tools map[string]ToolDef
}

func newCaptureBackend() *captureBackend {
	return &captureBackend{tools: map[string]ToolDef{}}
}

func (b *captureBackend) RegisterTool(tool ToolDef, _ ToolHandler) {
	b.tools[tool.Name] = tool
}

// registeredToolDefs は NewServer と同一の登録経路 (buildRegistry) を通して
// 登録される全ツール定義を name -> ToolDef のマップで返す。
func registeredToolDefs(client backlog.Client, cfg ServerConfig) map[string]ToolDef {
	backend := newCaptureBackend()
	buildRegistry(backend, client, nil, cfg)
	return backend.tools
}
