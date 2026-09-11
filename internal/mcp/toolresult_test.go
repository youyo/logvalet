package mcp

import (
	"testing"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// sdkTextContent は公式 SDK の CallToolResult から先頭の text content を取り出す。
func sdkTextContent(t *testing.T, r *officialmcp.CallToolResult) *officialmcp.TextContent {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("expected non-empty SDK content")
	}
	tc, ok := r.Content[0].(*officialmcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is not *officialmcp.TextContent: %T", r.Content[0])
	}
	return tc
}

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

// TestToolResult_ToOfficialSDKResult_Success は成功結果を公式 SDK の CallToolResult に
// 変換した際、isError が false のまま content が反映されることを確認する。
func TestToolResult_ToOfficialSDKResult_Success(t *testing.T) {
	r := NewTextToolResult("hello")
	sdk := r.ToOfficialSDKResult()
	if sdk.IsError {
		t.Error("sdk.IsError should be false")
	}
	if len(sdk.Content) != 1 {
		t.Fatalf("len(sdk.Content) = %d, want 1", len(sdk.Content))
	}
	tc := sdkTextContent(t, sdk)
	if tc.Text != "hello" {
		t.Errorf("tc.Text = %q, want %q", tc.Text, "hello")
	}
}

// TestToolResultFromOfficialSDKResult_RoundTrip は ToOfficialSDKResult -> ToolResultFromOfficialSDKResult の
// 相互変換で情報が失われないことを確認する。
func TestToolResultFromOfficialSDKResult_RoundTrip(t *testing.T) {
	original := ToolResult{
		Content:           []ToolContent{{Type: ToolContentTypeText, Text: `{"count":3}`}},
		StructuredContent: map[string]any{"count": float64(3)},
		IsError:           false,
		Meta: &ResultMeta{
			ServerInfo: &ServerInfo{Name: "logvalet", Version: "1.0.0"},
			Extra:      map[string]any{"custom": "value"},
		},
	}

	sdk := original.ToOfficialSDKResult()
	roundTripped := ToolResultFromOfficialSDKResult(sdk)

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

// TestToolResultFromOfficialSDKResult_Nil は nil 入力に対して空の ToolResult を返すことを確認する
// (呼び出し側で nil チェックを省略できるようにするためのガード)。
func TestToolResultFromOfficialSDKResult_Nil(t *testing.T) {
	r := ToolResultFromOfficialSDKResult(nil)
	if r.IsError || len(r.Content) != 0 || r.Meta != nil {
		t.Errorf("ToolResultFromOfficialSDKResult(nil) = %#v, want zero value", r)
	}
}
