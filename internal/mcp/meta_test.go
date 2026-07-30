package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// rawPostNewProtocolToolCall は params._meta.protocolVersion 付きの tools/call を
// 送るための生 HTTP POST。discover_test.go の rawPostDiscover と同様、公式 SDK は
// protocolVersion >= 2026-07-28 のリクエストに Mcp-Protocol-Version / Mcp-Method /
// (tools/call の場合) Mcp-Name ヘッダの一致を要求する (streamable_headers.go
// validateMcpHeaders。無いと -32020 HeaderMismatch になる) ため、
// rawPost (backend_official_test.go) とは別にヘッダを付与する。
func rawPostNewProtocolToolCall(t *testing.T, url, toolName, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", toolName)
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

// meta_test.go は S13 (issue #52) の実装。per-request _meta の
// protocolVersion/clientInfo が handler (ToolFunc/ToolHandler 経路) から
// 参照できること、および結果 _meta.serverInfo / 既存の authorization_url
// メタ付与が壊れていないことを検証する。

// --- 単体テスト: RequestMetaFromOfficialSDKMeta ---

// M01: MetaKeyProtocolVersion/MetaKeyClientInfo を RequestMeta.ProtocolVersion/
// ClientInfo に変換し、それ以外のキーは Extra にそのまま残す。
func TestRequestMetaFromOfficialSDKMeta_DecodesKnownKeys(t *testing.T) {
	meta := map[string]any{
		officialmcp.MetaKeyProtocolVersion: "2026-07-28",
		officialmcp.MetaKeyClientInfo:      &officialmcp.Implementation{Name: "acme-client", Version: "1.2.3", Title: "Acme"},
		"progressToken":                    "tok-1",
	}
	got := RequestMetaFromOfficialSDKMeta(meta)
	if got.ProtocolVersion != "2026-07-28" {
		t.Errorf("ProtocolVersion = %q, want %q", got.ProtocolVersion, "2026-07-28")
	}
	if got.ClientInfo != (ClientInfo{Name: "acme-client", Version: "1.2.3", Title: "Acme"}) {
		t.Errorf("ClientInfo = %+v, want acme-client/1.2.3/Acme", got.ClientInfo)
	}
	if got.Extra["progressToken"] != "tok-1" {
		t.Errorf("Extra[progressToken] = %v, want tok-1", got.Extra["progressToken"])
	}
}

// M02: clientInfo が生の map[string]any (JSON デコード直後相当) の場合も変換できる。
func TestRequestMetaFromOfficialSDKMeta_ClientInfoAsMap(t *testing.T) {
	meta := map[string]any{
		officialmcp.MetaKeyClientInfo: map[string]any{"name": "raw-client", "version": "0.1.0"},
	}
	got := RequestMetaFromOfficialSDKMeta(meta)
	if got.ClientInfo.Name != "raw-client" || got.ClientInfo.Version != "0.1.0" {
		t.Errorf("ClientInfo = %+v, want raw-client/0.1.0", got.ClientInfo)
	}
}

// M03: meta が空/nil の場合はゼロ値の RequestMeta を返す。
func TestRequestMetaFromOfficialSDKMeta_Empty(t *testing.T) {
	got := RequestMetaFromOfficialSDKMeta(nil)
	if got.ProtocolVersion != "" || got.ClientInfo != (ClientInfo{}) || got.Extra != nil {
		t.Errorf("got = %+v, want zero value", got)
	}
}

// --- 単体テスト: ContextWithRequestMeta / RequestMetaFromContext ---

// M04: ctx に埋め込んだ RequestMeta をそのまま取り出せる。
func TestRequestMetaFromContext_RoundTrip(t *testing.T) {
	want := RequestMeta{ProtocolVersion: "2026-07-28", ClientInfo: ClientInfo{Name: "c"}}
	ctx := ContextWithRequestMeta(context.Background(), want)
	got, ok := RequestMetaFromContext(ctx)
	if !ok {
		t.Fatal("RequestMetaFromContext ok = false, want true")
	}
	if got.ProtocolVersion != want.ProtocolVersion || got.ClientInfo != want.ClientInfo {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

// M05: 埋め込まれていない ctx では ok=false を返す。
func TestRequestMetaFromContext_Absent(t *testing.T) {
	_, ok := RequestMetaFromContext(context.Background())
	if ok {
		t.Error("RequestMetaFromContext ok = true, want false")
	}
}

// --- E2E: officialBackend 経由で ToolHandler が params._meta を参照できる ---

// M06: tools/call の params._meta.protocolVersion/clientInfo が、
// backend_official.go の RegisterTool 経由で ToolHandler の ctx から
// RequestMetaFromContext で読み出せる (done_criteria #1)。
func TestOfficialBackend_ToolHandler_ReadsRequestMeta(t *testing.T) {
	s := newOfficialMCPServer("test")
	backend := NewOfficialBackend(s)

	var gotMeta RequestMeta
	var gotOK bool
	backend.RegisterTool(ToolDef{Name: "logvalet_test_meta_echo", Description: "test-only echo tool"},
		func(ctx context.Context, args map[string]any) (ToolResult, error) {
			gotMeta, gotOK = RequestMetaFromContext(ctx)
			return NewTextToolResult("ok"), nil
		})

	handler := newStreamableHTTPHandler(s)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_test_meta_echo","arguments":{},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientInfo":{"name":"acme-client","version":"1.2.3"},` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, resp := rawPostNewProtocolToolCall(t, srv.URL, "logvalet_test_meta_echo", body)
	if status != 200 {
		t.Fatalf("status = %d, want 200; body=%s", status, resp)
	}
	var parsed struct {
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

	if !gotOK {
		t.Fatal("RequestMetaFromContext ok = false, want true (meta not propagated to handler)")
	}
	if gotMeta.ProtocolVersion != "2026-07-28" {
		t.Errorf("ProtocolVersion = %q, want 2026-07-28", gotMeta.ProtocolVersion)
	}
	if gotMeta.ClientInfo.Name != "acme-client" || gotMeta.ClientInfo.Version != "1.2.3" {
		t.Errorf("ClientInfo = %+v, want acme-client/1.2.3", gotMeta.ClientInfo)
	}
}

// M07: params._meta が無いレガシー経路 (S09 done_criteria: initialize なし tools/call) でも
// backend_official.go は毎回 ContextWithRequestMeta を呼ぶため RequestMetaFromContext は
// ok=true を返すが、中身はゼロ値の RequestMeta になる (protocolVersion/clientInfo 無し)。
func TestOfficialBackend_ToolHandler_NoRequestMeta_ReturnsZeroValue(t *testing.T) {
	s := newOfficialMCPServer("test")
	backend := NewOfficialBackend(s)

	var gotMeta RequestMeta
	var gotOK bool
	backend.RegisterTool(ToolDef{Name: "logvalet_test_meta_absent", Description: "test-only echo tool"},
		func(ctx context.Context, args map[string]any) (ToolResult, error) {
			gotMeta, gotOK = RequestMetaFromContext(ctx)
			return NewTextToolResult("ok"), nil
		})

	handler := newStreamableHTTPHandler(s)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"logvalet_test_meta_absent","arguments":{}}}`
	status, resp := rawPost(t, srv.URL, body)
	if status != 200 {
		t.Fatalf("status = %d, want 200; body=%s", status, resp)
	}
	if !gotOK {
		t.Fatal("RequestMetaFromContext ok = false, want true (backend_official.go always injects RequestMeta)")
	}
	if gotMeta.ProtocolVersion != "" || gotMeta.ClientInfo != (ClientInfo{}) || gotMeta.Extra != nil {
		t.Errorf("gotMeta = %+v, want zero value", gotMeta)
	}
}

// --- 結果側: serverInfo (SDK 自動付与) / authorization_url の _meta が壊れていない ---

// M08: authorization_url の _meta (S06/既存実装) は、per-request _meta 対応後も
// 変わらず authorization_required/authorization_url を含む。ServerInfo は
// logvalet 側で設定しない (SDK の annotateServerInfo (server.go) が結果 _meta に
// serverInfo を自動付与するため、二重に実装しない)。
func TestToolResultAuthRequired_MetaUnaffectedByRequestMeta(t *testing.T) {
	result := toolResultAuthRequired(errTestAuth, "https://example.test/authorize")
	if result.Meta == nil {
		t.Fatal("Meta is nil")
	}
	if !result.Meta.AuthorizationRequired {
		t.Error("AuthorizationRequired = false, want true")
	}
	if result.Meta.AuthorizationURL != "https://example.test/authorize" {
		t.Errorf("AuthorizationURL = %q, want https://example.test/authorize", result.Meta.AuthorizationURL)
	}
	if result.Meta.ServerInfo != nil {
		t.Errorf("ServerInfo = %+v, want nil (SDK annotates serverInfo automatically)", result.Meta.ServerInfo)
	}
	toMap := result.Meta.ToMap()
	if _, ok := toMap["serverInfo"]; ok {
		t.Error(`ToMap()["serverInfo"] present, want absent so the SDK's annotateServerInfo can add it`)
	}
}

// M09: tools/call の結果 _meta には、authorization_url 付きエラーであっても
// SDK が自動付与する serverInfo (SEP-2575) が乗る (公式 SDK の
// annotateServerInfo が既に serverInfo を付与済みでない場合のみ付与する挙動 (server.go) の固定)。
func TestOfficialBackend_ToolsCall_ResultMetaHasServerInfo(t *testing.T) {
	s := newOfficialMCPServer("meta-e2e-test")
	backend := NewOfficialBackend(s)
	backend.RegisterTool(ToolDef{Name: "logvalet_test_meta_result", Description: "test-only tool"},
		func(ctx context.Context, args map[string]any) (ToolResult, error) {
			result := toolResultAuthRequired(errTestAuth, "https://example.test/authorize")
			return result, nil
		})

	handler := newStreamableHTTPHandler(s)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_test_meta_result","arguments":{},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, resp := rawPostNewProtocolToolCall(t, srv.URL, "logvalet_test_meta_result", body)
	if status != 200 {
		t.Fatalf("status = %d, want 200; body=%s", status, resp)
	}

	var parsed struct {
		Result struct {
			Meta map[string]json.RawMessage `json:"_meta"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, resp)
	}
	if _, ok := parsed.Result.Meta["authorization_required"]; !ok {
		t.Errorf("_meta.authorization_required missing; body=%s", resp)
	}
	serverInfoRaw, ok := parsed.Result.Meta["io.modelcontextprotocol/serverInfo"]
	if !ok {
		t.Fatalf("_meta.io.modelcontextprotocol/serverInfo missing; body=%s", resp)
	}
	var serverInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(serverInfoRaw, &serverInfo); err != nil {
		t.Fatalf("unmarshal serverInfo: %v", err)
	}
	if serverInfo.Name != "logvalet" || serverInfo.Version != "meta-e2e-test" {
		t.Errorf("serverInfo = %+v, want name=logvalet version=meta-e2e-test", serverInfo)
	}
}

// errTestAuth は M08/M09 用のダミーエラー (メッセージ内容は検証対象外)。
var errTestAuth = &testAuthError{}

type testAuthError struct{}

func (*testAuthError) Error() string { return strings.Repeat("x", 1) }
