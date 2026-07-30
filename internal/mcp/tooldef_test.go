package mcp

import (
	"reflect"
	"testing"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func boolPtr(b bool) *bool { return &b }

// TestParamSpec_ToJSONSchema_String は string 型パラメータの JSON Schema 変換を確認する。
func TestParamSpec_ToJSONSchema_String(t *testing.T) {
	p := ParamSpec{Name: "project", Type: ParamTypeString, Description: "Project key"}
	got := p.ToJSONSchema()
	want := map[string]any{"type": "string", "description": "Project key"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToJSONSchema() = %#v, want %#v", got, want)
	}
}

// TestParamSpec_ToJSONSchema_Number は number 型パラメータの JSON Schema 変換を確認する。
func TestParamSpec_ToJSONSchema_Number(t *testing.T) {
	p := ParamSpec{Name: "limit", Type: ParamTypeNumber, Description: "Max count"}
	got := p.ToJSONSchema()
	want := map[string]any{"type": "number", "description": "Max count"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToJSONSchema() = %#v, want %#v", got, want)
	}
}

// TestParamSpec_ToJSONSchema_Boolean は boolean 型パラメータの JSON Schema 変換を確認する。
func TestParamSpec_ToJSONSchema_Boolean(t *testing.T) {
	p := ParamSpec{Name: "all_spaces", Type: ParamTypeBoolean}
	got := p.ToJSONSchema()
	want := map[string]any{"type": "boolean"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToJSONSchema() = %#v, want %#v", got, want)
	}
}

// TestParamSpec_ToJSONSchema_Enum は enum 付き string パラメータの JSON Schema 変換を確認する。
func TestParamSpec_ToJSONSchema_Enum(t *testing.T) {
	p := ParamSpec{Name: "status", Type: ParamTypeString, Enum: []string{"open", "closed"}}
	got := p.ToJSONSchema()
	want := map[string]any{"type": "string", "enum": []any{"open", "closed"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToJSONSchema() = %#v, want %#v", got, want)
	}
}

// TestParamSpec_ToJSONSchema_Array は array 型 (items 付き) パラメータの JSON Schema 変換を確認する。
func TestParamSpec_ToJSONSchema_Array(t *testing.T) {
	p := ParamSpec{
		Name:  "spaces",
		Type:  ParamTypeArray,
		Items: &ParamSpec{Type: ParamTypeString},
	}
	got := p.ToJSONSchema()
	want := map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToJSONSchema() = %#v, want %#v", got, want)
	}
}

// TestToolDef_InputSchemaJSON_Required は Required フィールドが inputSchema.required に
// 反映されることを確認する。
func TestToolDef_InputSchemaJSON_Required(t *testing.T) {
	td := ToolDef{
		Name: "logvalet_issue_get",
		Params: []ParamSpec{
			{Name: "issue_key", Type: ParamTypeString},
		},
		Required: []string{"issue_key"},
	}
	schema := td.InputSchemaJSON()
	if schema["type"] != "object" {
		t.Errorf("schema[type] = %v, want object", schema["type"])
	}
	required, ok := schema["required"].([]string)
	if !ok || !reflect.DeepEqual(required, []string{"issue_key"}) {
		t.Errorf("schema[required] = %#v, want [issue_key]", schema["required"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema[properties] is not map[string]any: %#v", schema["properties"])
	}
	if _, ok := props["issue_key"]; !ok {
		t.Error("properties[issue_key] not found")
	}
}

// TestToolDef_SDKRoundTrip は ToolDef -> 公式 SDK Tool -> ToolDef の相互変換で
// 情報が失われないことを確認する。
func TestToolDef_SDKRoundTrip(t *testing.T) {
	original := ToolDef{
		Name:        "logvalet_activity_digest",
		Title:       "アクティビティダイジェスト生成",
		Description: "Generate an activity digest for a space or project",
		Params: []ParamSpec{
			{Name: "project", Type: ParamTypeString, Description: "Filter by project key"},
			{Name: "limit", Type: ParamTypeNumber, Description: "Max number of activities"},
			{Name: "spaces", Type: ParamTypeArray, Description: "対象スペース", Items: &ParamSpec{Type: ParamTypeString}},
		},
		Required: []string{},
		Annotation: ToolAnnotation{
			Title:          "アクティビティダイジェスト生成",
			ReadOnlyHint:   boolPtr(true),
			IdempotentHint: boolPtr(true),
			OpenWorldHint:  boolPtr(true),
		},
	}

	sdkTool := original.ToOfficialSDKTool()
	if sdkTool.Name != original.Name {
		t.Errorf("sdkTool.Name = %q, want %q", sdkTool.Name, original.Name)
	}
	if sdkTool.Annotations.Title != original.Annotation.Title {
		t.Errorf("sdkTool.Annotations.Title = %q, want %q", sdkTool.Annotations.Title, original.Annotation.Title)
	}

	roundTripped := ToolDefFromOfficialSDKTool(sdkTool)

	if roundTripped.Name != original.Name {
		t.Errorf("roundTripped.Name = %q, want %q", roundTripped.Name, original.Name)
	}
	if roundTripped.Title != original.Title {
		t.Errorf("roundTripped.Title = %q, want %q", roundTripped.Title, original.Title)
	}
	if roundTripped.Description != original.Description {
		t.Errorf("roundTripped.Description = %q, want %q", roundTripped.Description, original.Description)
	}
	if len(roundTripped.Params) != len(original.Params) {
		t.Fatalf("roundTripped.Params has %d entries, want %d", len(roundTripped.Params), len(original.Params))
	}
	paramsByName := make(map[string]ParamSpec, len(roundTripped.Params))
	for _, p := range roundTripped.Params {
		paramsByName[p.Name] = p
	}
	for _, want := range original.Params {
		got, ok := paramsByName[want.Name]
		if !ok {
			t.Errorf("param %q not found after round trip", want.Name)
			continue
		}
		if got.Type != want.Type {
			t.Errorf("param %q Type = %q, want %q", want.Name, got.Type, want.Type)
		}
		if got.Description != want.Description {
			t.Errorf("param %q Description = %q, want %q", want.Name, got.Description, want.Description)
		}
	}
	if roundTripped.Annotation.ReadOnlyHint == nil || *roundTripped.Annotation.ReadOnlyHint != true {
		t.Error("roundTripped.Annotation.ReadOnlyHint should be true")
	}
	if roundTripped.Annotation.DestructiveHint != nil {
		t.Error("roundTripped.Annotation.DestructiveHint should remain unset (nil)")
	}
}

// TestParamSpecFromJSONSchema_Object はネストした object 型プロパティを復元できることを確認する。
func TestParamSpecFromJSONSchema_Object(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"b": map[string]any{"type": "string"},
			"a": map[string]any{"type": "number"},
		},
	}
	p := ParamSpecFromJSONSchema("filter", schema)
	if p.Type != ParamTypeObject {
		t.Fatalf("p.Type = %q, want object", p.Type)
	}
	if len(p.Properties) != 2 {
		t.Fatalf("len(p.Properties) = %d, want 2", len(p.Properties))
	}
	// ParamSpecFromJSONSchema はキー名でソートして復元するため、a, b の順になる。
	if p.Properties[0].Name != "a" || p.Properties[1].Name != "b" {
		t.Errorf("p.Properties order = [%s, %s], want [a, b]", p.Properties[0].Name, p.Properties[1].Name)
	}
}

// TestToolAnnotation_ToSDK_PreservesUnsetHints は未設定 (nil) の hint が
// SDK 型変換後も nil のまま (false に化けない) ことを確認する。
func TestToolAnnotation_ToSDK_PreservesUnsetHints(t *testing.T) {
	td := ToolDef{
		Name: "logvalet_issue_delete",
		Annotation: ToolAnnotation{
			ReadOnlyHint:    boolPtr(false),
			DestructiveHint: boolPtr(true),
			IdempotentHint:  boolPtr(true),
			OpenWorldHint:   boolPtr(true),
			// Title は未設定のまま
		},
	}
	sdkTool := td.ToOfficialSDKTool()
	if sdkTool.Annotations.Title != "" {
		t.Errorf("sdkTool.Annotations.Title = %q, want empty", sdkTool.Annotations.Title)
	}
	if sdkTool.Annotations.ReadOnlyHint {
		t.Error("sdkTool.Annotations.ReadOnlyHint should be false")
	}
	var _ *officialmcp.Tool = sdkTool // 型が公式 SDK の *Tool であることの静的確認
}
