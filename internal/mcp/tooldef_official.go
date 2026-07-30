package mcp

import (
	"sort"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// このファイルは internal/mcp パッケージにおける公式 Go SDK
// (github.com/modelcontextprotocol/go-sdk) アダプタの集約先。ToolDef/ToolResult
// (tooldef.go/toolresult.go) と officialmcp.Tool/officialmcp.CallToolResult との
// 相互変換を一箇所にまとめ、パッケージ内の他ファイルが SDK 型を直接扱わずに済むようにする
// (S07 の役割分担を公式 SDK backend にも適用したもの)。

// ToOfficialSDKTool は ToolDef を公式 Go SDK の *officialmcp.Tool に変換する。
// InputSchema には ToolDef.InputSchemaJSON() が返す map[string]any をそのまま渡す
// (officialmcp.Server.AddTool は json.RawMessage 以外にも「JSON へ marshal できる
// 任意の値」を InputSchema として受け付ける低レベル API のため、jsonschema.Schema 型
// への変換は不要)。
func (t ToolDef) ToOfficialSDKTool() *officialmcp.Tool {
	return &officialmcp.Tool{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		InputSchema: normalizedInputSchema(t),
		Annotations: t.Annotation.toOfficialSDKAnnotations(),
	}
}

// normalizedInputSchema は ToolDef.InputSchemaJSON() の結果のうち "required" を、
// 空でも null ではなく空配列 ([]string{}) に正規化して返す。
// InputSchemaJSON() は t.Required が空のとき append([]string(nil), ...) の結果 (nil)
// をそのまま格納するため、素の json.Marshal では "required": null になる。
// tools_list_baseline.json は空の Required を "required": [] として記録しているため、
// この差異を埋めないと baseline との比較で余計な差分が出てしまう。
func normalizedInputSchema(t ToolDef) map[string]any {
	schema := t.InputSchemaJSON()
	if req, ok := schema["required"].([]string); ok && len(req) == 0 {
		schema["required"] = []string{}
	}
	return schema
}

// toOfficialSDKAnnotations は ToolAnnotation を公式 SDK の *officialmcp.ToolAnnotations
// に変換する。
//
// 公式 SDK (v1.7.0) の ToolAnnotations.ReadOnlyHint / IdempotentHint は ToolAnnotation の
// *bool ではなく素の bool (false 値も省略されず出力される) であるため、nil の場合は
// false として復元する。logvalet の全 72 ツール定義は ReadOnlyHint/IdempotentHint を常に明示的に
// 設定しており (tool_categories.go)、nil になるケースは実運用上存在しないため、
// この変換で tools_list_baseline.json との差分は生じない。
func (a ToolAnnotation) toOfficialSDKAnnotations() *officialmcp.ToolAnnotations {
	readOnly := false
	if a.ReadOnlyHint != nil {
		readOnly = *a.ReadOnlyHint
	}
	idempotent := false
	if a.IdempotentHint != nil {
		idempotent = *a.IdempotentHint
	}
	return &officialmcp.ToolAnnotations{
		Title:           a.Title,
		ReadOnlyHint:    readOnly,
		DestructiveHint: a.DestructiveHint,
		IdempotentHint:  idempotent,
		OpenWorldHint:   a.OpenWorldHint,
	}
}

// ToOfficialSDKResult は ToolResult を公式 SDK の *officialmcp.CallToolResult に変換する。
func (r ToolResult) ToOfficialSDKResult() *officialmcp.CallToolResult {
	content := make([]officialmcp.Content, 0, len(r.Content))
	for _, c := range r.Content {
		content = append(content, &officialmcp.TextContent{Text: c.Text})
	}
	result := &officialmcp.CallToolResult{
		Content:           content,
		StructuredContent: r.StructuredContent,
		IsError:           r.IsError,
	}
	if r.Meta != nil {
		if fields := r.Meta.ToMap(); len(fields) > 0 {
			meta := make(officialmcp.Meta, len(fields))
			for k, v := range fields {
				meta[k] = v
			}
			result.Meta = meta
		}
	}
	return result
}

// ToolDefFromOfficialSDKTool は公式 SDK の *officialmcp.Tool から ToolDef を復元する
// (ToOfficialSDKTool の逆変換)。
//
// 公式 SDK の Tool.InputSchema は any 型で、ToOfficialSDKTool が格納するのは
// normalizedInputSchema が返す map[string]any (type/properties/required)。
// それ以外の型 (jsonschema.Schema や json.RawMessage 等) が入っている場合は
// パラメータを復元できないため Params/Required は空のまま返す。
func ToolDefFromOfficialSDKTool(t *officialmcp.Tool) ToolDef {
	if t == nil {
		return ToolDef{}
	}
	def := ToolDef{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
	}
	if schema, ok := t.InputSchema.(map[string]any); ok {
		if propsRaw, ok := schema["properties"].(map[string]any); ok {
			names := make([]string, 0, len(propsRaw))
			for k := range propsRaw {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, name := range names {
				childSchema, _ := propsRaw[name].(map[string]any)
				def.Params = append(def.Params, ParamSpecFromJSONSchema(name, childSchema))
			}
		}
		def.Required = requiredFromSchema(schema["required"])
	}
	if t.Annotations != nil {
		readOnly := t.Annotations.ReadOnlyHint
		idempotent := t.Annotations.IdempotentHint
		def.Annotation = ToolAnnotation{
			Title:           t.Annotations.Title,
			ReadOnlyHint:    &readOnly,
			DestructiveHint: t.Annotations.DestructiveHint,
			IdempotentHint:  &idempotent,
			OpenWorldHint:   t.Annotations.OpenWorldHint,
		}
	}
	return def
}

// requiredFromSchema は JSON Schema の "required" 値を []string に正規化する。
// InputSchemaJSON() が直接構築した場合は []string、JSON をデコードして得た場合は
// []any になるため両方を受け付ける。
func requiredFromSchema(v any) []string {
	switch req := v.(type) {
	case []string:
		return append([]string(nil), req...)
	case []any:
		out := make([]string, 0, len(req))
		for _, item := range req {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// ToolResultFromOfficialSDKResult は公式 SDK の *officialmcp.CallToolResult から
// ToolResult を復元する (ToOfficialSDKResult の逆変換)。
// text content 以外 (image/audio/embedded resource) は現時点で扱わないため無視する。
func ToolResultFromOfficialSDKResult(r *officialmcp.CallToolResult) ToolResult {
	if r == nil {
		return ToolResult{}
	}
	content := make([]ToolContent, 0, len(r.Content))
	for _, c := range r.Content {
		if tc, ok := c.(*officialmcp.TextContent); ok {
			content = append(content, ToolContent{Type: ToolContentTypeText, Text: tc.Text})
		}
	}
	result := ToolResult{
		Content:           content,
		StructuredContent: r.StructuredContent,
		IsError:           r.IsError,
	}
	if len(r.Meta) > 0 {
		meta := &ResultMeta{Extra: map[string]any{}}
		for k, v := range r.Meta {
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
