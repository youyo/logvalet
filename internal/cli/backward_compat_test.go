package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// BC5: NewServer が単一クライアントで動作し、ツールが tools/list に列挙されることを
// 確認する（multi-space 撤去後も単一スペース経路が壊れていないことの回帰テスト）。
func TestBC5_MCPTool_SingleClient(t *testing.T) {
	t.Parallel()

	mock := backlog.NewMockClient()

	// NewServer(client, ver, config) は従来シグネチャで動作する（BC確認）
	s := mcpinternal.NewServer(mock, "test-space", mcpinternal.ServerConfig{})
	if s == nil {
		t.Fatal("BC5: NewServer returned nil")
	}

	// 登録済みツールは公式 SDK の StreamableHTTP ハンドラー経由で tools/list を叩いて確認する
	// (S11 で SDK 移行後、サーバー型は登録ツールを列挙する公開 API を持たないため)。
	handler := mcpinternal.NewOfficialStreamableHTTPHandler(mock, "test-space", mcpinternal.ServerConfig{})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if err != nil {
		t.Fatalf("BC5: NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("BC5: tools/list request failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("BC5: read body: %v", err)
	}

	var parsed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("BC5: unmarshal tools/list response: %v; body=%s", err, body)
	}
	if len(parsed.Result.Tools) == 0 {
		t.Fatal("BC5: expected tools to be registered")
	}

	found := false
	for _, tool := range parsed.Result.Tools {
		if tool.Name == "logvalet_space_disk_usage" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("BC5: tool logvalet_space_disk_usage not found")
	}
}
