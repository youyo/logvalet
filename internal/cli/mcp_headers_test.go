package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// mcp_headers_test.go は S14 (issue #52) の実装。
//
// MCP-Protocol-Version・Mcp-Method・(tools/call, resources/read, prompts/get での)
// Mcp-Name ヘッダの検証は、S03 スパイク (docs/specs/spike-go-sdk-2026-07-28.md §(c))
// の実測どおり公式 Go SDK が完全に行う (logvalet 側の補完実装は無い。理由は
// mcp_headers.go のコメント参照)。本ファイルはその挙動を、`logvalet mcp`
// (--auth 無効) が実際に組み立てるマウントパス ("/mcp") 上で固定する E2E テスト。

// newHeadersTestServer は cli.NewNoAuthMCPMux (mcp_headers.go) が返す、
// 実運用と同一トポロジーのハンドラーを httptest で起動する。
// client=nil でも tools/list はツール一覧を返すだけで backlog.Client を呼ばないため
// 問題ない (ヘッダ検証はいずれの分岐も method dispatch より前段で完結する)。
func newHeadersTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := cli.NewNoAuthMCPMux(nil, "test-headers", mcpinternal.ServerConfig{})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type jsonrpcErrorEnvelope struct {
	Error *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

func postMCPHeaders(t *testing.T, url, body string, headers map[string]string) (int, jsonrpcErrorEnvelope, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var env jsonrpcErrorEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, b)
	}
	return resp.StatusCode, env, b
}

// H01: Mcp-Protocol-Version ヘッダが欠落し、body._meta.protocolVersion のみが
// 新プロトコルを名乗るケース → -32020 (HeaderMismatch)。
func TestE2E_MCPHeaders_MissingProtocolVersionHeader(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Method": "tools/list",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil {
		t.Fatalf("expected JSON-RPC error, got success: %s", raw)
	}
	if env.Error.Code != -32020 {
		t.Errorf("code = %d, want -32020; message=%q", env.Error.Code, env.Error.Message)
	}
}

// H02: Mcp-Protocol-Version ヘッダと body._meta.protocolVersion が不一致 → -32020。
func TestE2E_MCPHeaders_ProtocolVersionMismatch(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2025-11-25",
		"Mcp-Method":           "tools/list",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil || env.Error.Code != -32020 {
		t.Errorf("code = %+v, want -32020; body=%s", env.Error, raw)
	}
}

// H03: Mcp-Protocol-Version は正しいが Mcp-Method ヘッダが欠落 → -32020。
func TestE2E_MCPHeaders_MissingMcpMethodHeader(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil || env.Error.Code != -32020 {
		t.Errorf("code = %+v, want -32020; body=%s", env.Error, raw)
	}
}

// H04: Mcp-Method ヘッダの値が body.method と不一致 → -32020。
func TestE2E_MCPHeaders_MethodHeaderMismatch(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil || env.Error.Code != -32020 {
		t.Errorf("code = %+v, want -32020; body=%s", env.Error, raw)
	}
}

// H05: tools/call で Mcp-Name ヘッダが欠落 → -32020。
func TestE2E_MCPHeaders_ToolsCall_MissingMcpNameHeader(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_meta_tool_categories","arguments":{},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil || env.Error.Code != -32020 {
		t.Errorf("code = %+v, want -32020; body=%s", env.Error, raw)
	}
}

// H06: resources/read で Mcp-Name ヘッダが欠落 → -32020。logvalet は
// resources/read を一切登録していないが、ヘッダ検証は method dispatch より
// 前段で完結するため、未登録 method でも同じく -32020 になる
// (mcp_headers.go コメント参照)。
func TestE2E_MCPHeaders_ResourcesRead_MissingMcpNameHeader(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{` +
		`"uri":"file:///dummy","_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "resources/read",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil || env.Error.Code != -32020 {
		t.Errorf("code = %+v, want -32020; body=%s", env.Error, raw)
	}
}

// H07: prompts/get で Mcp-Name ヘッダが欠落 → -32020 (H06 と同じ理由で
// 未登録 method でも発火する)。
func TestE2E_MCPHeaders_PromptsGet_MissingMcpNameHeader(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{` +
		`"name":"dummy","_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "prompts/get",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil || env.Error.Code != -32020 {
		t.Errorf("code = %+v, want -32020; body=%s", env.Error, raw)
	}
}

// H08: _meta.protocolVersion が SDK の未知バージョン文字列 →
// -32022 (UnsupportedProtocolVersionError)。data.supported にサポート
// バージョン一覧が構造化データとして載ることも併せて固定する。
func TestE2E_MCPHeaders_UnsupportedProtocolVersion(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_meta_tool_categories","arguments":{},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2099-01-01",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2099-01-01",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "logvalet_meta_tool_categories",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, raw)
	}
	if env.Error == nil {
		t.Fatalf("expected JSON-RPC error, got success: %s", raw)
	}
	if env.Error.Code != -32022 {
		t.Errorf("code = %d, want -32022 (UnsupportedProtocolVersionError); message=%q", env.Error.Code, env.Error.Message)
	}
	var data struct {
		Supported []string `json:"supported"`
		Requested string   `json:"requested"`
	}
	if err := json.Unmarshal(env.Error.Data, &data); err != nil {
		t.Fatalf("unmarshal error.data: %v; raw=%s", err, env.Error.Data)
	}
	if data.Requested != "2099-01-01" {
		t.Errorf("data.requested = %q, want 2099-01-01", data.Requested)
	}
	if len(data.Supported) == 0 {
		t.Error("data.supported is empty, want the server's supported protocol version list")
	}
}

// H09: 旧プロトコル (Mcp-Protocol-Version ヘッダなし、body._meta なし) の
// tools/list は Mcp-Method/Mcp-Name の要求を一切受けず 200 で成功する
// (後方互換性の回帰固定。S03 スパイク (c-5) の再現)。
func TestE2E_MCPHeaders_LegacyRequest_NoHeadersRequired(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, raw)
	}
	if env.Error != nil {
		t.Fatalf("unexpected error: %+v; body=%s", env.Error, raw)
	}
}

// H10: 新プロトコルでも Mcp-Method のみが必要な method (tools/list は
// Mcp-Name 対象外) では、正しいヘッダを揃えれば 200 で成功する
// (新プロトコル・正常系の回帰固定)。
func TestE2E_MCPHeaders_ValidNewProtocolHeaders_ToolsList_Succeeds(t *testing.T) {
	srv := newHeadersTestServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, env, raw := postMCPHeaders(t, srv.URL, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/list",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, raw)
	}
	if env.Error != nil {
		t.Fatalf("unexpected error: %+v; body=%s", env.Error, raw)
	}
}
