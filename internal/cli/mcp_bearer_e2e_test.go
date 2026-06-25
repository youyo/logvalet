package cli_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

// bearerEchoHandler は Bearer E2E テスト用の簡易 MCP エコーハンドラー。
// 認証が通ったことを確認するため 200 + JSON を返す。
func bearerEchoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"jsonrpc":"2.0","result":"ok"}`))
}

// setupBearerTestServer は Bearer 認証付きテストサーバーを構築する。
// Handler Topology:
//
//	topMux: /healthz → cli.HealthHandler
//	        /       → bearerAuthMiddleware(token)(mcpMux)
//	mcpMux: /mcp    → bearerEchoHandler
func setupBearerTestServer(t *testing.T, token string) *httptest.Server {
	t.Helper()

	mcpMux := http.NewServeMux()
	mcpMux.HandleFunc("/mcp", bearerEchoHandler)

	topMux := http.NewServeMux()
	topMux.HandleFunc("/healthz", cli.HealthHandler)
	topMux.Handle("/", cli.BearerAuthMiddlewareForTest(token)(mcpMux))

	srv := httptest.NewServer(topMux)
	t.Cleanup(srv.Close)
	return srv
}

func TestE2E_BearerAuth_ValidToken(t *testing.T) {
	token := strings.Repeat("e", 32)
	srv := setupBearerTestServer(t, token)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestE2E_BearerAuth_InvalidToken(t *testing.T) {
	token := strings.Repeat("e", 32)
	srv := setupBearerTestServer(t, token)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrongtoken")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestE2E_BearerAuth_NoToken(t *testing.T) {
	token := strings.Repeat("e", 32)
	srv := setupBearerTestServer(t, token)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Authorization ヘッダーなし

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestE2E_BearerAuth_HealthzNoToken(t *testing.T) {
	token := strings.Repeat("e", 32)
	srv := setupBearerTestServer(t, token)

	// トークンなしで /healthz → 200（認証バイパス）
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

func TestE2E_BearerAuth_CaseInsensitive(t *testing.T) {
	token := strings.Repeat("e", 32)
	srv := setupBearerTestServer(t, token)

	// 小文字 "bearer" スキームで認証成功
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp with lowercase bearer failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (lowercase 'bearer' scheme should be accepted)", resp.StatusCode)
	}
}

func TestE2E_BearerAuth_UppercaseSchemeCaseSensitiveToken(t *testing.T) {
	// スキームはcase-insensitive、トークンはcase-sensitiveであることをE2Eレベルで検証する。
	// "BEARER Token123" → スキームは大文字でもOK、トークン "Token123" はcase-sensitiveに比較される。
	token := "Token123" + strings.Repeat("x", 24) // 32文字
	srv := setupBearerTestServer(t, token)

	// 大文字スキーム + 正しいトークン → 200
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "BEARER "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (BEARER scheme + correct token)", resp.StatusCode)
	}

	// 大文字スキーム + トークンを小文字にしたもの → 401（トークンはcase-sensitive）
	req2, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "BEARER "+strings.ToLower(token))

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (token is case-sensitive, lowercase token should fail)", resp2.StatusCode)
	}
}
