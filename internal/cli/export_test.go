package cli

import (
	"net/http"

	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

var HealthHandler = healthHandler
var APIKeyAuthMiddlewareForTest = apiKeyAuthMiddleware

func ResolvedAuthModeForTest(c *McpCmd) string { return c.resolvedAuthMode() }

func APIKeyValueForTest(c *McpCmd) string { return c.apiKeyValue() }

// BuildMCPHTTPHandlerForTest は McpCmd.buildHTTPHandler (mcp.go) を公開する。
// S30: Bearer passthrough の本番配線 (apikey → identity → passthrough) を
// 実際に listen せずに httptest 上で E2E 検証するためのテスト専用フック。
func BuildMCPHTTPHandlerForTest(c *McpCmd, ver string, cfg mcpinternal.ServerConfig) http.Handler {
	return c.buildHTTPHandler(ver, cfg)
}
