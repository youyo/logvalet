package cli_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

// apiKeyEchoHandler は apikey E2E テスト用の簡易 MCP エコーハンドラー。
// 認証が通ったことを確認するため 200 + JSON を返す。
func apiKeyEchoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"jsonrpc":"2.0","result":"ok"}`))
}

// setupAPIKeyTestServer は apikey 認証付きテストサーバーを構築する。
// Handler Topology:
//
//	topMux: /healthz → cli.HealthHandler（認証対象外・契約 §1.5）
//	        /       → apiKeyAuthMiddleware(key)(mcpMux)
//	mcpMux: /mcp    → apiKeyEchoHandler
func setupAPIKeyTestServer(t *testing.T, key string) *httptest.Server {
	t.Helper()

	mcpMux := http.NewServeMux()
	mcpMux.HandleFunc("/mcp", apiKeyEchoHandler)

	topMux := http.NewServeMux()
	topMux.HandleFunc("/healthz", cli.HealthHandler)
	topMux.Handle("/", cli.APIKeyAuthMiddlewareForTest(key)(mcpMux))

	srv := httptest.NewServer(topMux)
	t.Cleanup(srv.Close)
	return srv
}

func postMCP(t *testing.T, srv *httptest.Server, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestE2E_APIKeyAuth_ValidKey(t *testing.T) {
	key := strings.Repeat("e", 32)
	srv := setupAPIKeyTestServer(t, key)

	resp := postMCP(t, srv, map[string]string{apiKeyHeader: key})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestE2E_APIKeyAuth_InvalidKey(t *testing.T) {
	srv := setupAPIKeyTestServer(t, strings.Repeat("e", 32))

	resp := postMCP(t, srv, map[string]string{apiKeyHeader: "wrongkey"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestE2E_APIKeyAuth_NoKey(t *testing.T) {
	srv := setupAPIKeyTestServer(t, strings.Repeat("e", 32))

	resp := postMCP(t, srv, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// Authorization は Backlog credential passthrough 専用であり、apikey としては受理しない。
func TestE2E_APIKeyAuth_AuthorizationHeaderRejected(t *testing.T) {
	key := strings.Repeat("e", 32)
	srv := setupAPIKeyTestServer(t, key)

	resp := postMCP(t, srv, map[string]string{"Authorization": "Bearer " + key})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (Authorization must not authenticate the gateway)", resp.StatusCode)
	}
}

// apikey が通れば Authorization は passthrough 用にそのまま backend へ届く。
func TestE2E_APIKeyAuth_AuthorizationPassedThrough(t *testing.T) {
	key := strings.Repeat("e", 32)
	backlogToken := "backlog-oauth-access-token"

	var seen string
	mcpMux := http.NewServeMux()
	mcpMux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
	topMux := http.NewServeMux()
	topMux.Handle("/", cli.APIKeyAuthMiddlewareForTest(key)(mcpMux))
	srv := httptest.NewServer(topMux)
	t.Cleanup(srv.Close)

	resp := postMCP(t, srv, map[string]string{
		apiKeyHeader:    key,
		"Authorization": "Bearer " + backlogToken,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if seen != "Bearer "+backlogToken {
		t.Errorf("backend saw Authorization = %q, want %q", seen, "Bearer "+backlogToken)
	}
}

// 契約 §1.5: /healthz は apikey 検証の対象外。
func TestE2E_APIKeyAuth_HealthzNoKey(t *testing.T) {
	srv := setupAPIKeyTestServer(t, strings.Repeat("e", 32))

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", string(body), `{"status":"ok"}`)
	}
}

func TestE2E_APIKeyAuth_CaseSensitiveKey(t *testing.T) {
	key := "Token123" + strings.Repeat("x", 24)
	srv := setupAPIKeyTestServer(t, key)

	if resp := postMCP(t, srv, map[string]string{apiKeyHeader: key}); resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (exact key)", resp.StatusCode)
	}
	if resp := postMCP(t, srv, map[string]string{apiKeyHeader: strings.ToLower(key)}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (key is case-sensitive)", resp.StatusCode)
	}
}
