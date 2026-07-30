package mcp

import (
	"context"
	"sort"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// このファイルは internal/mcp パッケージにおける mark3labs/mcp-go SDK アダプタの
// 集約先。ToolDef/ToolResult (tooldef.go/toolresult.go) と gomcp.Tool/gomcp.CallToolResult
// との相互変換、および ServerBackend (backend.go) の mark3labs 実装を一箇所にまとめる
// ことで、S07 以降 gomcp import を持つ非テストファイルを tooldef_mark3labs.go /
// tools.go(コンストラクタの引数型のみ) / server.go の3箇所に限定する。

// mark3labsBackend は mark3labs/mcp-go の *mcpserver.MCPServer を使う ServerBackend 実装。
type mark3labsBackend struct {
	server *mcpserver.MCPServer
}

// NewMark3labsBackend は既存の *mcpserver.MCPServer を ServerBackend として包む。
func NewMark3labsBackend(s *mcpserver.MCPServer) ServerBackend {
	return &mark3labsBackend{server: s}
}

// RegisterTool は ServerBackend を実装する。tool を gomcp.Tool に変換して
// AddTool に登録し、呼び出し時は req.GetArguments() を handler に渡した上で
// ToolResult を gomcp.CallToolResult に変換して返す。
func (b *mark3labsBackend) RegisterTool(tool ToolDef, handler ToolHandler) {
	b.server.AddTool(tool.ToSDKTool(), func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		result, err := handler(ctx, req.GetArguments())
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return result.ToSDKResult(), nil
	})
}

// ToSDKTool は ToolDef を mark3labs/mcp-go の gomcp.Tool に変換する。
// マイグレーション期間中、logvalet 型で組み立てたツール定義を既存の
// ToolRegistry.Register 系 API (gomcp.Tool を要求する) に橋渡しするために使う。
func (t ToolDef) ToSDKTool() gomcp.Tool {
	props := make(map[string]any, len(t.Params))
	for _, p := range t.Params {
		props[p.Name] = p.ToJSONSchema()
	}
	return gomcp.Tool{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		InputSchema: gomcp.ToolInputSchema{
			Type:       "object",
			Properties: props,
			Required:   append([]string(nil), t.Required...),
		},
		Annotations: gomcp.ToolAnnotation{
			Title:           t.Annotation.Title,
			ReadOnlyHint:    t.Annotation.ReadOnlyHint,
			DestructiveHint: t.Annotation.DestructiveHint,
			IdempotentHint:  t.Annotation.IdempotentHint,
			OpenWorldHint:   t.Annotation.OpenWorldHint,
		},
	}
}

// ToolDefFromSDKTool は gomcp.Tool から ToolDef を復元する (ToSDKTool の逆変換)。
// tools_list_baseline.json のような SDK 生成物を logvalet 型に取り込む経路として使う。
func ToolDefFromSDKTool(t gomcp.Tool) ToolDef {
	names := make([]string, 0, len(t.InputSchema.Properties))
	for k := range t.InputSchema.Properties {
		names = append(names, k)
	}
	sort.Strings(names)
	params := make([]ParamSpec, 0, len(names))
	for _, name := range names {
		schema, _ := t.InputSchema.Properties[name].(map[string]any)
		params = append(params, ParamSpecFromJSONSchema(name, schema))
	}
	return ToolDef{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		Params:      params,
		Required:    append([]string(nil), t.InputSchema.Required...),
		Annotation: ToolAnnotation{
			Title:           t.Annotations.Title,
			ReadOnlyHint:    t.Annotations.ReadOnlyHint,
			DestructiveHint: t.Annotations.DestructiveHint,
			IdempotentHint:  t.Annotations.IdempotentHint,
			OpenWorldHint:   t.Annotations.OpenWorldHint,
		},
	}
}

// ToSDKResult は ToolResult を mark3labs/mcp-go の gomcp.CallToolResult に変換する。
func (r ToolResult) ToSDKResult() *gomcp.CallToolResult {
	content := make([]gomcp.Content, 0, len(r.Content))
	for _, c := range r.Content {
		content = append(content, gomcp.TextContent{Type: string(c.Type), Text: c.Text})
	}
	result := &gomcp.CallToolResult{
		Content:           content,
		StructuredContent: r.StructuredContent,
		IsError:           r.IsError,
	}
	if r.Meta != nil {
		if fields := r.Meta.ToMap(); len(fields) > 0 {
			result.Meta = &gomcp.Meta{AdditionalFields: fields}
		}
	}
	return result
}

// ToolResultFromSDKResult は gomcp.CallToolResult から ToolResult を復元する
// (ToSDKResult の逆変換)。text content 以外 (image/audio/embedded resource) は
// 現時点で扱わないため無視する。
func ToolResultFromSDKResult(r *gomcp.CallToolResult) ToolResult {
	if r == nil {
		return ToolResult{}
	}
	content := make([]ToolContent, 0, len(r.Content))
	for _, c := range r.Content {
		if tc, ok := c.(gomcp.TextContent); ok {
			content = append(content, ToolContent{Type: ToolContentType(tc.Type), Text: tc.Text})
		}
	}
	result := ToolResult{
		Content:           content,
		StructuredContent: r.StructuredContent,
		IsError:           r.IsError,
	}
	if r.Meta != nil && len(r.Meta.AdditionalFields) > 0 {
		meta := &ResultMeta{Extra: map[string]any{}}
		for k, v := range r.Meta.AdditionalFields {
			switch k {
			case "authorization_required":
				if b, ok := v.(bool); ok {
					meta.AuthorizationRequired = b
				}
			case "authorization_url":
				if s, ok := v.(string); ok {
					meta.AuthorizationURL = s
				}
			case "serverInfo":
				if si, ok := v.(map[string]any); ok {
					name, _ := si["name"].(string)
					version, _ := si["version"].(string)
					title, _ := si["title"].(string)
					meta.ServerInfo = &ServerInfo{Name: name, Version: version, Title: title}
				}
			default:
				meta.Extra[k] = v
			}
		}
		if len(meta.Extra) == 0 {
			meta.Extra = nil
		}
		result.Meta = meta
	}
	return result
}
