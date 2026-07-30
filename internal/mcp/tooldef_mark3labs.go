package mcp

import (
	"sort"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// このファイルは internal/mcp パッケージにおける mark3labs/mcp-go SDK アダプタの
// 集約先。ToolDef/ToolResult (tooldef.go/toolresult.go) と gomcp.Tool/gomcp.CallToolResult
// との相互変換を一箇所にまとめることで、S07 以降 gomcp import を持つファイルを
// tooldef_mark3labs.go / tools.go / server.go の3箇所に限定する。

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
