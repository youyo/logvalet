package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// backend_official.go は S09 (issue #52) の実装。ServerBackend (backend.go) の
// 公式 Go SDK (github.com/modelcontextprotocol/go-sdk) 実装を提供する。
//
// S03 スパイク (docs/specs/spike-go-sdk-2026-07-28.md) の実測により、
// StreamableHTTPOptions.Stateless=true にすると同一の *officialmcp.Server /
// officialmcp.NewStreamableHTTPHandler が旧initialize/sessionベースのフローと
// 新sessionless(SEP-2575)プロトコルを並行してサポートすることが確認されている。
// そのため officialBackend は「新旧で別のサーバー型を用意する」必要が無く、
// mark3labsBackend (tooldef_mark3labs.go) と同じ RegisterTool ベースの
// 単一実装で足りる。
//
// この時点 (S09) では mark3labsBackend と officialBackend は併存し、呼び出し側が
// どちらを ServerBackend として ToolRegistry に注入するかで切替できる
// (NewMark3labsBackend / NewOfficialBackend のどちらを NewToolRegistryWithBackend
// に渡すか)。

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
// StreamableHTTPOptions.Stateless=true 前提で登録する。mark3labsBackend 版の NewServer
// (server.go) と同じ構成 (registerAllTools) を officialBackend 経由で行う。
func NewOfficialServer(client backlog.Client, ver string, cfg ServerConfig) *officialmcp.Server {
	s := officialmcp.NewServer(&officialmcp.Implementation{Name: "logvalet", Version: ver}, nil)
	backend := NewOfficialBackend(s)
	reg := NewToolRegistryWithBackend(backend, client, cfg.AuthorizationURL)
	reg.disableFilePaths = cfg.DisableFilePaths
	registerAllTools(reg, cfg)
	return s
}

// NewOfficialStreamableHTTPHandler は NewOfficialServer が返す *officialmcp.Server を
// StreamableHTTPOptions.Stateless=true で StreamableHTTPHandler にラップして返す。
func NewOfficialStreamableHTTPHandler(client backlog.Client, ver string, cfg ServerConfig) *officialmcp.StreamableHTTPHandler {
	s := NewOfficialServer(client, ver, cfg)
	return officialmcp.NewStreamableHTTPHandler(func(*http.Request) *officialmcp.Server {
		return s
	}, &officialmcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}
