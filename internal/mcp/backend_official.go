package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// backend_official.go は ServerBackend (backend.go) の公式 Go SDK
// (github.com/modelcontextprotocol/go-sdk) 実装を提供する。
//
// S03 スパイク (docs/specs/spike-go-sdk-2026-07-28.md) の実測により、
// StreamableHTTPOptions.Stateless=true にすると同一の *officialmcp.Server /
// officialmcp.NewStreamableHTTPHandler が旧initialize/sessionベースのフローと
// 新sessionless(SEP-2575)プロトコルを並行してサポートすることが確認されている。
// そのため officialBackend は「新旧で別のサーバー型を用意する」必要が無く、
// RegisterTool ベースの単一実装で足りる。
//
// S11 で旧 SDK backend は削除され、officialBackend が logvalet で唯一の
// 本番用 ServerBackend 実装となった (テストは backend_test.go の fake backend を使う)。

// officialBackend は公式 Go SDK の *officialmcp.Server を使う ServerBackend 実装。
type officialBackend struct {
	server *officialmcp.Server
}

// NewOfficialBackend は既存の *officialmcp.Server を ServerBackend として包む。
func NewOfficialBackend(s *officialmcp.Server) ServerBackend {
	return &officialBackend{server: s}
}

// RegisterTool は ServerBackend を実装する。tool を *officialmcp.Tool に変換して
// Server.AddTool に登録する。
//
// officialmcp.Server.AddTool は「低レベル API」(generic な officialmcp.AddTool とは
// 異なり、入力の unmarshal/validation を SDK 側で行わない) ため、
// req.Params.Arguments (json.RawMessage) を自前で map[string]any へ unmarshal してから
// handler に渡す。handler の戻り値の error は「呼び出し元が回復不能な protocol
// error」を意味する (JSON エンベロープ形式のツールエラーは ToolResult.IsError=true
// として表現され、ToolFunc/callWithDefaultClient (tools.go) 側で既に組み立て済みの
// ため、ここでは Go の error を返すのは Arguments の unmarshal に失敗した場合のみ)。
func (b *officialBackend) RegisterTool(tool ToolDef, handler ToolHandler) {
	b.server.AddTool(tool.ToOfficialSDKTool(), func(ctx context.Context, req *officialmcp.CallToolRequest) (*officialmcp.CallToolResult, error) {
		args := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
		}
		result, err := handler(ctx, args)
		if err != nil {
			return NewErrorToolResult(ToolError{Message: err.Error()}).ToOfficialSDKResult(), nil
		}
		return result.ToOfficialSDKResult(), nil
	})
}

// NewOfficialServer は公式 Go SDK の *officialmcp.Server を生成し、logvalet の全ツールを
// 登録して返す。実体は NewServer (server.go) と同一で、stdio トランスポート
// (internal/cli/mcp_stdio.go) からの呼び出し名として維持している。
func NewOfficialServer(client backlog.Client, ver string, cfg ServerConfig) *officialmcp.Server {
	return NewServer(client, ver, cfg)
}

// newStreamableHTTPHandler は *officialmcp.Server を StreamableHTTPOptions.Stateless=true
// (+ JSONResponse=true) で StreamableHTTPHandler にラップする。
//
// Stateless=true により initialize / Mcp-Session-Id を要求せずに tools/list・tools/call を
// 受け付けられる (S03 スパイクの実測結果)。これは Lambda 等の複数インスタンス構成で
// セッション状態を共有できない環境で必須の設定。
func newStreamableHTTPHandler(s *officialmcp.Server) *officialmcp.StreamableHTTPHandler {
	return officialmcp.NewStreamableHTTPHandler(func(*http.Request) *officialmcp.Server {
		return s
	}, &officialmcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

// NewOfficialStreamableHTTPHandler は単一 client 構成 (CLI profile / API key 認証) の
// MCP サーバーを Stateless な StreamableHTTPHandler として返す。
func NewOfficialStreamableHTTPHandler(client backlog.Client, ver string, cfg ServerConfig) *officialmcp.StreamableHTTPHandler {
	return newStreamableHTTPHandler(NewServer(client, ver, cfg))
}

// NewOfficialStreamableHTTPHandlerWithFactory は per-user ClientFactory 構成
// (OAuth モード) の MCP サーバーを Stateless な StreamableHTTPHandler として返す。
//
// Stateless=true のため、ツール呼び出しごとに HTTP リクエストの context がハンドラーへ
// 渡り、factory がそこから user を解決する。セッションに紐づく状態を持たないので、
// idproxy が注入する userID は常に「そのリクエストの」ユーザーになる。
func NewOfficialStreamableHTTPHandlerWithFactory(
	factory func(ctx context.Context) (backlog.Client, error),
	ver string,
	cfg ServerConfig,
) *officialmcp.StreamableHTTPHandler {
	return newStreamableHTTPHandler(NewServerWithFactory(factory, ver, cfg))
}
