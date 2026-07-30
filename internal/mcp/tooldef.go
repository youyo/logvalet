package mcp

import (
	"sort"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// ParamType は ToolDef パラメータが表す JSON Schema の "type" 値。
type ParamType string

const (
	ParamTypeString  ParamType = "string"
	ParamTypeNumber  ParamType = "number"
	ParamTypeInteger ParamType = "integer"
	ParamTypeBoolean ParamType = "boolean"
	ParamTypeArray   ParamType = "array"
	ParamTypeObject  ParamType = "object"
)

// space injection によって全 read/write ツールに注入されるパラメータ名。
const (
	ParamNameSpaces    = "spaces"
	ParamNameAllSpaces = "all_spaces"
)

// ParamSpec は ToolDef が持つ1パラメータ (JSON Schema property) の logvalet 独自表現。
// mark3labs/mcp-go は property を map[string]any として扱うが、ParamSpec は構造化された
// 型として保持することで、JSON Schema への変換ロジックを一箇所に集約する。
type ParamSpec struct {
	Name        string
	Type        ParamType
	Description string
	// Enum は Type == ParamTypeString のときのみ意味を持つ列挙候補。
	Enum []string
	// Items は Type == ParamTypeArray のときの要素定義。
	Items *ParamSpec
	// Properties は Type == ParamTypeObject のときの子プロパティ定義。
	Properties []ParamSpec
}

// ToJSONSchema は ParamSpec を tools/list の inputSchema.properties[name] に相当する
// JSON Schema 表現 (map[string]any) に変換する。
func (p ParamSpec) ToJSONSchema() map[string]any {
	schema := map[string]any{"type": string(p.Type)}
	if p.Description != "" {
		schema["description"] = p.Description
	}
	if len(p.Enum) > 0 {
		enum := make([]any, len(p.Enum))
		for i, v := range p.Enum {
			enum[i] = v
		}
		schema["enum"] = enum
	}
	if p.Type == ParamTypeArray && p.Items != nil {
		schema["items"] = p.Items.ToJSONSchema()
	}
	if p.Type == ParamTypeObject && len(p.Properties) > 0 {
		props := make(map[string]any, len(p.Properties))
		for _, child := range p.Properties {
			props[child.Name] = child.ToJSONSchema()
		}
		schema["properties"] = props
	}
	return schema
}

// ParamSpecFromJSONSchema は JSON Schema のプロパティ表現から ParamSpec を復元する。
// name はスキーマ自体に含まれない (呼び出し元がプロパティマップのキーとして持っている) ため、
// 引数で明示的に渡す。
func ParamSpecFromJSONSchema(name string, schema map[string]any) ParamSpec {
	p := ParamSpec{Name: name}
	if schema == nil {
		return p
	}
	if t, ok := schema["type"].(string); ok {
		p.Type = ParamType(t)
	}
	if d, ok := schema["description"].(string); ok {
		p.Description = d
	}
	if enumRaw, ok := schema["enum"].([]any); ok {
		for _, v := range enumRaw {
			if s, ok := v.(string); ok {
				p.Enum = append(p.Enum, s)
			}
		}
	}
	if p.Type == ParamTypeArray {
		if itemsRaw, ok := schema["items"].(map[string]any); ok {
			items := ParamSpecFromJSONSchema("", itemsRaw)
			p.Items = &items
		}
	}
	if p.Type == ParamTypeObject {
		if propsRaw, ok := schema["properties"].(map[string]any); ok {
			names := make([]string, 0, len(propsRaw))
			for k := range propsRaw {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				childSchema, _ := propsRaw[k].(map[string]any)
				p.Properties = append(p.Properties, ParamSpecFromJSONSchema(k, childSchema))
			}
		}
	}
	return p
}

// ToolAnnotation は MCP tool の behavior hint (tools/list の annotations フィールド) を表す
// logvalet 独自型。mark3labs/mcp-go の gomcp.ToolAnnotation と等価な情報を保持するが、
// SDK 型そのものへの依存はしない。
//
// ReadOnlyHint 等を *bool で保持するのは、SDK 同様「未設定 (omitempty で省略)」と
// 「明示的に false」を区別する必要があるため。
type ToolAnnotation struct {
	Title           string
	ReadOnlyHint    *bool
	DestructiveHint *bool
	IdempotentHint  *bool
	OpenWorldHint   *bool
}

// ToolDef は logvalet が所有するツール定義。tools/list レスポンスの1ツールぶんの情報
// (名前・説明・パラメータ・annotation) を SDK 非依存で表現する。
type ToolDef struct {
	Name        string
	Title       string
	Description string
	Params      []ParamSpec
	Required    []string
	Annotation  ToolAnnotation
}

// InputSchemaJSON は Params/Required を tools/list の inputSchema 相当の
// JSON Schema (type=object) に変換する。
func (t ToolDef) InputSchemaJSON() map[string]any {
	props := make(map[string]any, len(t.Params))
	for _, p := range t.Params {
		props[p.Name] = p.ToJSONSchema()
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   append([]string(nil), t.Required...),
	}
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

// ClientInfo は MCP initialize/request meta に含まれるクライアント識別情報
// (gomcp.Implementation 相当) の logvalet 独自表現。
type ClientInfo struct {
	Name    string
	Version string
	Title   string
}

// RequestMeta は MCP リクエストに付随するメタ情報の logvalet 独自表現。
// protocolVersion・clientInfo に加え、呼び出し元 (MCP client) が付加した
// progressToken 等の任意フィールドを Extra に保持する。
type RequestMeta struct {
	ProtocolVersion string
	ClientInfo      ClientInfo
	Extra           map[string]any
}

// SpacesParamSpec は read 系ツールに注入する "spaces" パラメータ (複数指定可) の定義を返す。
// RegisterWithSpaces / injectSpaceParams が gomcp.Tool へ直接注入している内容の
// logvalet 型表現。
func SpacesParamSpec(description string) ParamSpec {
	return ParamSpec{
		Name:        ParamNameSpaces,
		Type:        ParamTypeArray,
		Description: description,
		Items:       &ParamSpec{Type: ParamTypeString},
	}
}

// AllSpacesParamSpec は read 系ツールに注入する "all_spaces" パラメータの定義を返す。
func AllSpacesParamSpec(description string) ParamSpec {
	return ParamSpec{Name: ParamNameAllSpaces, Type: ParamTypeBoolean, Description: description}
}

// SpacesWriteParamSpec は write 系ツールに注入する "spaces" パラメータ (単一指定) の定義を返す。
// RegisterWithSpacesWrite / injectSpaceParamWrite が注入している内容の logvalet 型表現。
func SpacesWriteParamSpec(description string) ParamSpec {
	return ParamSpec{
		Name:        ParamNameSpaces,
		Type:        ParamTypeArray,
		Description: description,
		Items:       &ParamSpec{Type: ParamTypeString},
	}
}
