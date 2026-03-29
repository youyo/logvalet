package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/version"
)

// McpCmd は `logvalet mcp` サブコマンド。
// Streamable HTTP MCP サーバーを起動する。
type McpCmd struct {
	Port int    `help:"listen port" default:"8080"`
	Host string `help:"listen host" default:"127.0.0.1"`
}

// Run は MCP サーバーを起動する。
func (c *McpCmd) Run(g *GlobalFlags) error {
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	ver := version.NewInfo().Version
	s := mcpinternal.NewServer(rc.Client, ver)
	h := mcpserver.NewStreamableHTTPServer(s, mcpserver.WithEndpointPath("/mcp"))

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	fmt.Fprintf(os.Stderr, "logvalet MCP server listening on %s/mcp\n", addr)

	mux := http.NewServeMux()
	mux.Handle("/mcp", h)

	ctx := context.Background()
	_ = ctx // context は将来の graceful shutdown 用
	return http.ListenAndServe(addr, mux)
}
