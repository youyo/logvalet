package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/version"
)

// McpStdioCmd は `logvalet mcp-stdio` サブコマンド。
// stdio トランスポートで MCP サーバーを起動する（Claude Desktop 等向け）。
//
// S10 (issue #52) で公式 Go SDK (github.com/modelcontextprotocol/go-sdk) の
// officialmcp.Server / officialmcp.StdioTransport に移行した。S09 で用意した
// mcpinternal.NewOfficialServer (internal/mcp/backend_official.go) をそのまま再利用する。
type McpStdioCmd struct{}

func (c *McpStdioCmd) Validate() error { return nil }

// Run は stdio MCP サーバーを起動する。
func (c *McpStdioCmd) Run(g *GlobalFlags) error {
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	ver := version.NewInfo().Version
	cfg := mcpinternal.ServerConfig{
		Profile:          rc.Config.Profile,
		Space:            rc.Config.Space,
		BaseURL:          rc.Config.BaseURL,
		DisableFilePaths: true, // stdio モードではローカルファイルアクセスを無効化
	}
	s := mcpinternal.NewOfficialServer(rc.Client, ver, cfg)

	fmt.Fprintln(os.Stderr, "logvalet MCP server (stdio) ready")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return InterpretListenResult(s.Run(ctx, &officialmcp.StdioTransport{}), ctx.Err())
}

// InterpretListenResult は Server.Run の戻り値を正常/異常終了に変換する。
//
// 正常終了条件:
//   - err == nil: 正常完了
//   - ctxErr != nil: SIGINT/SIGTERM またはクライアント切断による停止
//   - errors.Is(err, io.EOF): Claude Desktop 等が stdin を閉じた場合
func InterpretListenResult(err error, ctxErr error) error {
	if err == nil || ctxErr != nil || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
