package cli

import (
	"net/http"

	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

var HealthHandler = healthHandler

// BuildMCPHTTPHandlerForTest は McpCmd.buildHTTPHandler (mcp.go) を公開する。
// HTTP モード唯一の構成（Bearer passthrough）の本番配線を、実際に listen せずに
// httptest 上で E2E 検証するためのテスト専用フック。
func BuildMCPHTTPHandlerForTest(c *McpCmd, ver string, cfg mcpinternal.ServerConfig) http.Handler {
	return c.buildHTTPHandler(ver, cfg)
}
