package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
)

// discover_test.go は server/discover (SEP-2575 JSON-RPC メソッド) のレスポンスを
// golden として固定する (S12, issue #52)。discover.go のコメントの通り、logvalet 側の
// 上書き実装は無く、公式 Go SDK の挙動 (S03 スパイクで実測済み) をそのままロックする。

// discoverVer は本テスト専用のバージョン文字列。実運用では internal/version.Version
// (cli/mcp.go 経由) が同じ経路 (NewOfficialStreamableHTTPHandler の ver 引数 →
// newOfficialMCPServer の Implementation.Version) で serverInfo.version に渡る。
const discoverVer = "v1.2.3-discover-test"

// newDiscoverTestHTTPServer は server/discover 検証用に、ver を明示指定した
// Stateless=true の StreamableHTTPHandler を httptest で起動する。
func newDiscoverTestHTTPServer(t *testing.T, client backlog.Client) *httptest.Server {
	t.Helper()
	handler := NewOfficialStreamableHTTPHandler(client, discoverVer, ServerConfig{})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// rawPostDiscover は server/discover 用の SEP-2575 ヘッダ (Mcp-Protocol-Version /
// Mcp-Method) を付与した生 HTTP POST を送る。server/discover は常に新プロトコル扱い
// (S03 スパイク (b)) のため、このヘッダが無いと -32020 (HeaderMismatch) になる
// (S03 スパイク (c) 参照)。
func rawPostDiscover(t *testing.T, url string) (int, []byte) {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "server/discover")
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

// discoverResponse は server/discover レスポンスのうち本テストで固定する部分。
type discoverResponse struct {
	Result struct {
		SupportedVersions []string                   `json:"supportedVersions"`
		Capabilities       map[string]json.RawMessage `json:"capabilities"`
		Meta               map[string]json.RawMessage `json:"_meta"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// D01: server/discover が supportedVersions・tools capability・serverInfo
// (name=logvalet, version=渡された ver) を返すことを固定する
// (S12 done_criteria)。
func TestOfficialServer_Discover_FixesResponse(t *testing.T) {
	mock := backlog.NewMockClient()
	srv := newDiscoverTestHTTPServer(t, mock)

	status, body := rawPostDiscover(t, srv.URL)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}

	var parsed discoverResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if parsed.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", parsed.Error)
	}

	// supportedVersions: Stateless=true + 新プロトコルの discover は SDK 組み込みの
	// 全バージョンをそのまま返す (S03 スパイク (b) の実測ログと同一)。
	wantVersions := []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}
	if !slices.Equal(parsed.Result.SupportedVersions, wantVersions) {
		t.Errorf("supportedVersions = %v, want %v", parsed.Result.SupportedVersions, wantVersions)
	}

	// capabilities: tools を登録しているため tools.listChanged=true が自動付与される
	// (SDK Server.capabilities()、S03 スパイク (b) の実測ログと同一)。
	toolsRaw, ok := parsed.Result.Capabilities["tools"]
	if !ok {
		t.Fatal("capabilities.tools missing")
	}
	var toolsCap struct {
		ListChanged bool `json:"listChanged"`
	}
	if err := json.Unmarshal(toolsRaw, &toolsCap); err != nil {
		t.Fatalf("unmarshal capabilities.tools: %v", err)
	}
	if !toolsCap.ListChanged {
		t.Error("capabilities.tools.listChanged = false, want true")
	}
	if _, ok := parsed.Result.Capabilities["logging"]; !ok {
		t.Error("capabilities.logging missing (SDK default capability)")
	}

	// serverInfo: _meta["io.modelcontextprotocol/serverInfo"] に name=logvalet と
	// NewOfficialStreamableHTTPHandler に渡した ver がそのまま入ることを固定する。
	serverInfoRaw, ok := parsed.Result.Meta["io.modelcontextprotocol/serverInfo"]
	if !ok {
		t.Fatal("_meta.io.modelcontextprotocol/serverInfo missing")
	}
	var serverInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(serverInfoRaw, &serverInfo); err != nil {
		t.Fatalf("unmarshal serverInfo: %v", err)
	}
	if serverInfo.Name != "logvalet" {
		t.Errorf("serverInfo.name = %q, want %q", serverInfo.Name, "logvalet")
	}
	if serverInfo.Version != discoverVer {
		t.Errorf("serverInfo.version = %q, want %q", serverInfo.Version, discoverVer)
	}
}

// D02: NewOfficialStreamableHTTPHandlerWithFactory (OAuth モード) 経由でも discover の
// serverInfo が同様に固定されることを確認する。buildRegistry 経由の factory 分岐が
// serverInfo に影響しないこと (S12 done_criteria の対象は client/factory 両経路)
// を保証する。
func TestOfficialServer_Discover_FixesResponse_WithFactory(t *testing.T) {
	mock := backlog.NewMockClient()
	factory := func(context.Context) (backlog.Client, error) { return mock, nil }

	handler := NewOfficialStreamableHTTPHandlerWithFactory(factory, discoverVer, ServerConfig{})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	status, body := rawPostDiscover(t, srv.URL)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}

	var parsed discoverResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if parsed.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", parsed.Error)
	}

	serverInfoRaw, ok := parsed.Result.Meta["io.modelcontextprotocol/serverInfo"]
	if !ok {
		t.Fatal("_meta.io.modelcontextprotocol/serverInfo missing")
	}
	var serverInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(serverInfoRaw, &serverInfo); err != nil {
		t.Fatalf("unmarshal serverInfo: %v", err)
	}
	if serverInfo.Name != "logvalet" {
		t.Errorf("serverInfo.name = %q, want %q", serverInfo.Name, "logvalet")
	}
	if serverInfo.Version != discoverVer {
		t.Errorf("serverInfo.version = %q, want %q", serverInfo.Version, discoverVer)
	}
}
