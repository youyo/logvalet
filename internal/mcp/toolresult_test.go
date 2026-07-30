package mcp

import (
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// TestNewTextToolResult は成功結果が isError=false・text content 1件で構築されることを確認する。
func TestNewTextToolResult(t *testing.T) {
	r := NewTextToolResult(`{"ok":true}`)
	if r.IsError {
		t.Error("IsError should be false")
	}
	if len(r.Content) != 1 || r.Content[0].Type != ToolContentTypeText || r.Content[0].Text != `{"ok":true}` {
		t.Errorf("Content = %#v, want single text content", r.Content)
	}
}

// TestNewErrorToolResult は ToolError からエラー結果 (isError=true) が構築されることを確認する。
func TestNewErrorToolResult(t *testing.T) {
	r := NewErrorToolResult(ToolError{Message: "issue not found"})
	if !r.IsError {
		t.Error("IsError should be true")
	}
	if len(r.Content) != 1 || r.Content[0].Text != "issue not found" {
		t.Errorf("Content = %#v, want single text content with error message", r.Content)
	}
}

// TestToolResult_ToSDKResult_Success は成功結果を gomcp.CallToolResult に変換した際、
// isError が省略され (false) content が反映されることを確認する。
func TestToolResult_ToSDKResult_Success(t *testing.T) {
	r := NewTextToolResult("hello")
	sdk := r.ToSDKResult()
	if sdk.IsError {
		t.Error("sdk.IsError should be false")
	}
	if len(sdk.Content) != 1 {
		t.Fatalf("len(sdk.Content) = %d, want 1", len(sdk.Content))
	}
	tc, ok := sdk.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("sdk.Content[0] is not TextContent: %#v", sdk.Content[0])
	}
	if tc.Text != "hello" {
		t.Errorf("tc.Text = %q, want %q", tc.Text, "hello")
	}
}

// TestToolResult_ToSDKResult_AuthRequired は認可 URL 付きエラーが _meta に反映されることを確認する。
// toolResultAuthRequired (tools.go) と等価な出力を ToolResult 経由で再現できることを検証する。
func TestToolResult_ToSDKResult_AuthRequired(t *testing.T) {
	r := ToolResult{
		Content: []ToolContent{{Type: ToolContentTypeText, Text: "authorization required"}},
		IsError: true,
		Meta: &ResultMeta{
			AuthorizationRequired: true,
			AuthorizationURL:      "https://example.com/authorize",
		},
	}
	sdk := r.ToSDKResult()
	if !sdk.IsError {
		t.Error("sdk.IsError should be true")
	}
	if sdk.Meta == nil {
		t.Fatal("sdk.Meta should not be nil")
	}
	if sdk.Meta.AdditionalFields["authorization_required"] != true {
		t.Errorf("authorization_required = %v, want true", sdk.Meta.AdditionalFields["authorization_required"])
	}
	if sdk.Meta.AdditionalFields["authorization_url"] != "https://example.com/authorize" {
		t.Errorf("authorization_url = %v, want https://example.com/authorize", sdk.Meta.AdditionalFields["authorization_url"])
	}
}

// TestToolResultFromSDKResult_RoundTrip は ToSDKResult -> ToolResultFromSDKResult の
// 相互変換で情報が失われないことを確認する。
func TestToolResultFromSDKResult_RoundTrip(t *testing.T) {
	original := ToolResult{
		Content:           []ToolContent{{Type: ToolContentTypeText, Text: `{"count":3}`}},
		StructuredContent: map[string]any{"count": float64(3)},
		IsError:           false,
		Meta: &ResultMeta{
			ServerInfo: &ServerInfo{Name: "logvalet", Version: "1.0.0"},
			Extra:      map[string]any{"custom": "value"},
		},
	}

	sdk := original.ToSDKResult()
	roundTripped := ToolResultFromSDKResult(sdk)

	if roundTripped.IsError != original.IsError {
		t.Errorf("IsError = %v, want %v", roundTripped.IsError, original.IsError)
	}
	if len(roundTripped.Content) != 1 || roundTripped.Content[0].Text != original.Content[0].Text {
		t.Errorf("Content = %#v, want %#v", roundTripped.Content, original.Content)
	}
	if roundTripped.Meta == nil {
		t.Fatal("roundTripped.Meta should not be nil")
	}
	if roundTripped.Meta.ServerInfo == nil || roundTripped.Meta.ServerInfo.Name != "logvalet" {
		t.Errorf("roundTripped.Meta.ServerInfo = %#v, want Name=logvalet", roundTripped.Meta.ServerInfo)
	}
	if roundTripped.Meta.Extra["custom"] != "value" {
		t.Errorf("roundTripped.Meta.Extra[custom] = %v, want value", roundTripped.Meta.Extra["custom"])
	}
}

// TestToolResultFromSDKResult_Nil は nil 入力に対して空の ToolResult を返すことを確認する
// (呼び出し側で nil チェックを省略できるようにするためのガード)。
func TestToolResultFromSDKResult_Nil(t *testing.T) {
	r := ToolResultFromSDKResult(nil)
	if r.IsError || len(r.Content) != 0 || r.Meta != nil {
		t.Errorf("ToolResultFromSDKResult(nil) = %#v, want zero value", r)
	}
}

// TestResultMeta_ToMap_OmitsUnsetFields は AuthorizationRequired=false /
// AuthorizationURL="" のとき、対応するキーが出力に含まれないことを確認する
// (optional field 省略と null の同一視の一環)。
func TestResultMeta_ToMap_OmitsUnsetFields(t *testing.T) {
	m := ResultMeta{}
	got := m.ToMap()
	if _, ok := got["authorization_required"]; ok {
		t.Error("authorization_required should be omitted when false")
	}
	if _, ok := got["authorization_url"]; ok {
		t.Error("authorization_url should be omitted when empty")
	}
	if len(got) != 0 {
		t.Errorf("got = %#v, want empty map", got)
	}
}
