package mcp

import (
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// このファイルは internal/mcp パッケージにおける公式 Go SDK
// (github.com/modelcontextprotocol/go-sdk) アダプタの集約先。ToolDef/ToolResult
// (tooldef.go/toolresult.go) と officialmcp.Tool/officialmcp.CallToolResult との
// 相互変換を一箇所にまとめる。tooldef_mark3labs.go と同じ役割分担 (S07 の方針) を
// 公式 SDK backend にも適用する。

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
// mark3labs/mcp-go 側の gomcp.ToolInputSchema.MarshalJSON は空の Required を意図的に
// "required": [] として出力する (toolArgumentsSchemaMarshalJSON) ため、この差異を
// 埋めないと tools_list_baseline.json との比較で余計な差分が出てしまう。
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
// 公式 SDK (v1.7.0) の ToolAnnotations.ReadOnlyHint / IdempotentHint は mark3labs の
// *bool ではなく素の bool (デフォルトで false 値も省略されず出力される、
// MCPGODEBUG=hintomitempty=1 未設定時の挙動) であるため、nil の場合は false として
// 復元する。logvalet の全 72 ツール定義は ReadOnlyHint/IdempotentHint を常に明示的に
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
