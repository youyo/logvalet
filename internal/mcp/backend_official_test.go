package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// backend_official_test.go は S09 (issue #52) の実装。officialBackend
// (backend_official.go) / ToOfficialSDKTool・ToOfficialSDKResult (tooldef_official.go)
// の単体テストと、公式 Go SDK の StreamableHTTPHandler を httptest で起動した
// in-process E2E テストを提供する。

// --- 単体テスト: ToolDef/ToolResult <-> 公式 SDK 型の変換 ---

// O01: ToOfficialSDKTool は Name/Title/Description/InputSchema/Annotations を
// 変換する。
func TestToolDefToOfficialSDKTool_ConvertsFields(t *testing.T) {
	readOnly := true
	idempotent := false
	destructive := true
	def := ToolDef{
		Name:        "example_tool",
		Title:       "Example",
		Description: "an example tool",
		Params: []ParamSpec{
			{Name: "project_key", Type: ParamTypeString, Description: "project key"},
		},
		Required: []string{"project_key"},
		Annotation: ToolAnnotation{
			Title:           "Example Annotation",
			ReadOnlyHint:    &readOnly,
			DestructiveHint: &destructive,
			IdempotentHint:  &idempotent,
		},
	}

	got := def.ToOfficialSDKTool()

	if got.Name != "example_tool" || got.Title != "Example" || got.Description != "an example tool" {
		t.Fatalf("unexpected tool identity: %+v", got)
	}
	schema, ok := got.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("InputSchema is not map[string]any: %T", got.InputSchema)
	}
	if schema["type"] != "object" {
		t.Errorf("InputSchema[type] = %v, want object", schema["type"])
	}
	if got.Annotations == nil {
		t.Fatal("expected non-nil Annotations")
	}
	if got.Annotations.Title != "Example Annotation" {
		t.Errorf("Annotations.Title = %q", got.Annotations.Title)
	}
	if !got.Annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint=true")
	}
	if got.Annotations.IdempotentHint {
		t.Error("expected IdempotentHint=false")
	}
	if got.Annotations.DestructiveHint == nil || !*got.Annotations.DestructiveHint {
		t.Error("expected DestructiveHint pointer to true")
	}
	if got.Annotations.OpenWorldHint != nil {
		t.Error("expected OpenWorldHint to remain nil (not set on source ToolAnnotation)")
	}
}

// O02: ReadOnlyHint/IdempotentHint が未設定 (nil) の場合、公式 SDK の bare bool には
// false が復元される。logvalet の実ツール定義は両フィールドを常に明示的に設定するため
// (tool_categories.go) 実運用上このケースは発生しないが、変換関数自体の防御的な
// デフォルト値を確認する。
func TestToolDefToOfficialSDKTool_NilHintsDefaultToFalse(t *testing.T) {
	def := ToolDef{Name: "no_hints_tool"}
	got := def.ToOfficialSDKTool()
	if got.Annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint=false for nil source")
	}
	if got.Annotations.IdempotentHint {
		t.Error("expected IdempotentHint=false for nil source")
	}
}

// O03: ToOfficialSDKResult は成功結果 (content/structuredContent) を変換する。
func TestToolResultToOfficialSDKResult_Success(t *testing.T) {
	result := ToolResult{
		Content:           []ToolContent{{Type: ToolContentTypeText, Text: `{"ok":true}`}},
		StructuredContent: map[string]any{"ok": true},
	}
	got := result.ToOfficialSDKResult()
	if got.IsError {
		t.Error("expected IsError=false")
	}
	if len(got.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(got.Content))
	}
	tc, ok := got.Content[0].(*officialmcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is not *officialmcp.TextContent: %T", got.Content[0])
	}
	if tc.Text != `{"ok":true}` {
		t.Errorf("Content[0].Text = %q", tc.Text)
	}
}

// O04: ToOfficialSDKResult はエラー結果 (IsError=true) をそのまま伝播する。
func TestToolResultToOfficialSDKResult_Error(t *testing.T) {
	result := NewErrorToolResult(ToolError{Message: "project_key is required"})
	got := result.ToOfficialSDKResult()
	if !got.IsError {
		t.Error("expected IsError=true")
	}
	if len(got.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(got.Content))
	}
	tc, ok := got.Content[0].(*officialmcp.TextContent)
	if !ok || tc.Text != "project_key is required" {
		t.Fatalf("unexpected error content: %+v", got.Content[0])
	}
}

// O05: ResultMeta (authorization_required/authorization_url 等) が officialmcp.Meta に
// 変換される。
func TestToolResultToOfficialSDKResult_Meta(t *testing.T) {
	result := ToolResult{
		Content: []ToolContent{{Type: ToolContentTypeText, Text: "auth required"}},
		IsError: true,
		Meta:    &ResultMeta{AuthorizationRequired: true, AuthorizationURL: "https://example.test/authorize"},
	}
	got := result.ToOfficialSDKResult()
	if got.Meta == nil {
		t.Fatal("expected non-nil Meta")
	}
	if got.Meta["authorization_required"] != true {
		t.Errorf("Meta[authorization_required] = %v", got.Meta["authorization_required"])
	}
	if got.Meta["authorization_url"] != "https://example.test/authorize" {
		t.Errorf("Meta[authorization_url] = %v", got.Meta["authorization_url"])
	}
}

// --- in-process E2E テスト: 公式 SDK の StreamableHTTPHandler 経由 ---

// newOfficialTestHTTPServer は NewOfficialStreamableHTTPHandler を httptest でラップして
// 起動する。Stateless=true + JSONResponse=true (S09 done_criteria) で構築される。
func newOfficialTestHTTPServer(t *testing.T, client backlog.Client) *httptest.Server {
	t.Helper()
	handler := NewOfficialStreamableHTTPHandler(client, "test", ServerConfig{})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// rawPost は生の HTTP POST を送り、ステータス・ボディを返す。SDK クライアントを経由せず
// ワイヤ上の応答をそのまま検証するため、S03 スパイク (spike_test.go) と同じ手法を使う。
func rawPost(t *testing.T, url, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp.StatusCode, b
}

// O06: Stateless=true の officialBackend サーバーは、initialize なしで直接
// tools/list が通ることを確認する (S09 done_criteria: StreamableHTTPOptions.Stateless=true
// で動作すること)。
func TestOfficialServer_ToolsList_NoInitializeRequired(t *testing.T) {
	mock := backlog.NewMockClient()
	srv := newOfficialTestHTTPServer(t, mock)

	status, body := rawPost(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}

	var parsed struct {
		Result struct {
			Tools []json.RawMessage `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, body)
	}
	if parsed.Error != nil {
		t.Fatalf("unexpected error: %+v", parsed.Error)
	}
	const wantCount = 76
	if len(parsed.Result.Tools) != wantCount {
		t.Errorf("tools count = %d, want %d", len(parsed.Result.Tools), wantCount)
	}
}

// stripOfficialSDKOnlyResultFields は正規化済み tools/list レスポンスから
// "result.cacheScope" / "result.ttlMs" を取り除いて返す。
//
// SDK 間表現差 (意図した差分。baseline は書き換えない):
// 公式 Go SDK v1.7.0 の ListToolsResult は SEP-2575 の Cacheable ミックスイン
// (cacheScope/ttlMs) を全ての list 系レスポンスに無条件で含める
// (github.com/modelcontextprotocol/go-sdk/mcp/protocol.go の Cacheable 構造体。
// ttlMs は json タグに omitempty が無いため値が 0 でも必ず出力される)。
// baseline 採取時の旧 SDK は SEP-2575 に対応しておらずこのフィールド自体が存在しないため、
// tools_list_baseline.json にも含まれない。
// これはプロトコルバージョンのアップグレードに伴う SDK の仕様差であり、
// logvalet 側の ToolDef/ToolResult 変換ロジックの不整合ではないため、
// テスト側でこの2フィールドのみを許容差分として除外した上で baseline と比較する。
func stripOfficialSDKOnlyResultFields(t *testing.T, normalized []byte) []byte {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("unmarshal normalized response: %v", err)
	}
	if result, ok := parsed["result"].(map[string]any); ok {
		delete(result, "cacheScope")
		delete(result, "ttlMs")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(parsed); err != nil {
		t.Fatalf("re-encode stripped response: %v", err)
	}
	return buf.Bytes()
}

// O07: tools/list の正規化後のレスポンスが testdata/tools_list_baseline.json
// (移行前の旧 SDK backend による golden) と一致することを確認する。
//
// baseline はツール追加のたびに再生成が必要な golden。直近の再生成は関連課題ツール
// (logvalet_issue_related_list/_add/_delete) 追加時に、本テストが書き出す
// 正規化済み実応答で上書きする形で行った (72 → 75 ツール)。SDK 由来の表現差は
// stripOfficialSDKOnlyResultFields で除外済みのため、再生成しても「旧 SDK backend
// 由来の golden」という位置づけは維持される
// (S09 done_criteria)。SDK 間表現差については stripOfficialSDKOnlyResultFields の
// コメントを参照。annotations.readOnlyHint/idempotentHint については、logvalet の
// 全75ツール定義が両フィールドを常に明示設定しているため (tool_categories.go)、
// *bool→bool のデフォルト値変換に起因する意図しない差分は観測されていない。
func TestOfficialServer_ToolsList_MatchesBaseline(t *testing.T) {
	mock := backlog.NewMockClient()
	srv := newOfficialTestHTTPServer(t, mock)

	status, body := rawPost(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}

	normalized, err := normalizeToolsListResponse(body)
	if err != nil {
		t.Fatalf("normalizeToolsListResponse: %v", err)
	}
	got := stripOfficialSDKOnlyResultFields(t, normalized)

	// baseline は jsonrpc/id フィールドも含めて正規化された golden なので、レスポンスの
	// id をそろえてから比較する (baseline 生成時の id=1 に合わせて上記リクエストも id=1
	// にしている)。
	baseline, err := os.ReadFile("testdata/tools_list_baseline.json")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !bytes.Equal(got, baseline) {
		_ = os.WriteFile("/tmp/claude-501/got_tools_list.json", got, 0o644)
		t.Errorf("official backend tools/list response does not match baseline (wrote diff candidate to /tmp/claude-501/got_tools_list.json)")
	}
}

// O08: Stateless=true では initialize を経ずに tools/call が成功する
// (S03 スパイク (a) の再現。S09 done_criteria: Stateless=true で動作すること)。
func TestOfficialServer_ToolsCall_NoInitializeRequired_Success(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		if projectKey != "TESTPROJ" {
			t.Fatalf("unexpected projectKey: %q", projectKey)
		}
		return []domain.Status{{ID: 1, ProjectID: 100, Name: "Open", DisplayOrder: 1000}}, nil
	}
	srv := newOfficialTestHTTPServer(t, mock)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"logvalet_meta_statuses","arguments":{"project_key":"TESTPROJ"}}}`
	status, resp := rawPost(t, srv.URL, body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, resp)
	}

	var parsed struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, resp)
	}
	if parsed.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", parsed.Error)
	}
	if parsed.Result.IsError {
		t.Fatalf("unexpected tool error: %+v", parsed.Result)
	}
	if len(parsed.Result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(parsed.Result.Content))
	}
	var statuses []domain.Status
	if err := json.Unmarshal([]byte(parsed.Result.Content[0].Text), &statuses); err != nil {
		t.Fatalf("unmarshal content text: %v; text=%s", err, parsed.Result.Content[0].Text)
	}
	if len(statuses) != 1 || statuses[0].Name != "Open" {
		t.Errorf("statuses = %+v", statuses)
	}
}

// O09: ツールハンドラーのドメインエラーは JSON-RPC レベルのエラーではなく
// CallToolResult.IsError=true + content[0].text のエラーメッセージとして表現される
// (S09 done_criteria: エラー結果が JSON エンベロープ形式(spec §9)を保つこと)。
func TestOfficialServer_ToolsCall_ErrorEnvelope(t *testing.T) {
	mock := backlog.NewMockClient()
	srv := newOfficialTestHTTPServer(t, mock)

	// project_key を省略してエラーを誘発する。
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"logvalet_meta_statuses","arguments":{}}}`
	status, resp := rawPost(t, srv.URL, body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (tool errors are not protocol errors); body=%s", status, resp)
	}

	var parsed struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, resp)
	}
	if parsed.Error != nil {
		t.Fatalf("unexpected protocol-level error (should be a tool-level error): %+v", parsed.Error)
	}
	if !parsed.Result.IsError {
		t.Fatalf("expected IsError=true, got %+v", parsed.Result)
	}
	if len(parsed.Result.Content) != 1 || parsed.Result.Content[0].Text != "project_key is required" {
		t.Errorf("unexpected error content: %+v", parsed.Result.Content)
	}
}
