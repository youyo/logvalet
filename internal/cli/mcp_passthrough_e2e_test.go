package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// mcp_passthrough_e2e_test.go は S30 (issue #52) attempt 2 の実装。
//
// verifier F1: PassthroughAuthMiddleware / NewPassthroughClientFactory /
// NewOfficialStreamableHTTPHandlerWithFactory は実装済みだったが本番呼び出し元が
// 0 件だった。ここでは cli.BuildMCPHTTPHandlerForTest 経由で mcp.go の
// buildHTTPHandler (Run() が実際に組み立てるのと同一のトポロジー) を httptest で
// 起動し、Authorization: Bearer ヘッダーが Backlog モックへ per-request で
// 到達することを検証する。

// passthroughBacklogMock は GET /api/v2/users/myself のみを実装する Backlog API の
// 最小モック。受信した Authorization ヘッダーを記録する。
type passthroughBacklogMock struct {
	mu       sync.Mutex
	observed []string
}

func newPassthroughBacklogMock(t *testing.T) (*httptest.Server, *passthroughBacklogMock) {
	t.Helper()
	m := &passthroughBacklogMock{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/users/myself" {
			http.NotFound(w, r)
			return
		}
		m.mu.Lock()
		m.observed = append(m.observed, r.Header.Get("Authorization"))
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"userId":"alice","name":"Alice"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, m
}

func (m *passthroughBacklogMock) lastObserved() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.observed) == 0 {
		return ""
	}
	return m.observed[len(m.observed)-1]
}

func (m *passthroughBacklogMock) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.observed)
}

// postToolCall は logvalet_user_me tools/call リクエストを送る (旧プロトコル。
// Mcp-Protocol-Version ヘッダー無しなので SEP-2575 ヘッダー検証の対象外)。
func postToolCall(t *testing.T, srv *httptest.Server, headers map[string]string) *http.Response {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"logvalet_user_me","arguments":{}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp failed: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// mergeHeaders は複数の header map を 1 つに合成する (後勝ち)。
func mergeHeaders(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

type passthroughErrorEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	Error         struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
	} `json:"error"`
}

func decodeErrorEnvelope(t *testing.T, resp *http.Response) passthroughErrorEnvelope {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	var env passthroughErrorEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("unmarshal error envelope failed: %v; body=%s", err, b)
	}
	return env
}

// auth-mode=none: Authorization: Bearer <token> が Backlog モックまで届く。
func TestE2E_Passthrough_NoneMode_BearerReachesBacklogClient(t *testing.T) {
	backlogSrv, mock := newPassthroughBacklogMock(t)

	cmd := &cli.McpCmd{}
	cfg := mcpinternal.ServerConfig{BaseURL: backlogSrv.URL}
	handler := cli.BuildMCPHTTPHandlerForTest(cmd, "test-passthrough", cfg)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	token := "user-specific-backlog-token"
	resp := postToolCall(t, srv, map[string]string{"Authorization": "Bearer " + token})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, b)
	}
	if mock.callCount() != 1 {
		t.Fatalf("backlog mock call count = %d, want 1", mock.callCount())
	}
	if want := "Bearer " + token; mock.lastObserved() != want {
		t.Errorf("backlog observed Authorization = %q, want %q", mock.lastObserved(), want)
	}
}

// auth-mode=none: Authorization ヘッダー欠落は Backlog を呼ばず 401 + エラーエンベロープ。
func TestE2E_Passthrough_NoneMode_MissingBearerReturnsErrorEnvelope(t *testing.T) {
	backlogSrv, mock := newPassthroughBacklogMock(t)

	cmd := &cli.McpCmd{}
	cfg := mcpinternal.ServerConfig{BaseURL: backlogSrv.URL}
	handler := cli.BuildMCPHTTPHandlerForTest(cmd, "test-passthrough", cfg)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	resp := postToolCall(t, srv, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	env := decodeErrorEnvelope(t, resp)
	if env.Error.Code != "authentication_error" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "authentication_error")
	}
	if mock.callCount() != 0 {
		t.Errorf("backlog mock should not be called, count = %d", mock.callCount())
	}
}

// auth-mode=apikey: apikey → identity → passthrough の順。apikey が通っても
// Authorization (passthrough 用) が無ければ Backlog は呼ばれず 401。
func TestE2E_Passthrough_APIKeyMode_ChainOrder(t *testing.T) {
	backlogSrv, mock := newPassthroughBacklogMock(t)

	key := strings.Repeat("k", 32)
	cmd := &cli.McpCmd{AuthMode: "apikey", ApiKey: key}
	cfg := mcpinternal.ServerConfig{BaseURL: backlogSrv.URL}
	handler := cli.BuildMCPHTTPHandlerForTest(cmd, "test-passthrough", cfg)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	identityHeaders := map[string]string{
		"X-Logvalet-Identity-Issuer":  "https://login.example.com/",
		"X-Logvalet-Identity-Subject": "user-001",
	}

	// apikey OK + identity OK + Bearer あり → Backlog まで届く。
	token := "gateway-injected-backlog-token"
	resp := postToolCall(t, srv, mergeHeaders(map[string]string{
		"X-Logvalet-Api-Key": key,
		"Authorization":      "Bearer " + token,
	}, identityHeaders))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, b)
	}
	if want := "Bearer " + token; mock.lastObserved() != want {
		t.Errorf("backlog observed Authorization = %q, want %q", mock.lastObserved(), want)
	}

	// apikey OK + identity OK + Bearer 無し → passthrough の 401 (authentication_error)。
	// apikey 自体のエラー (code=unauthorized) とは区別できる = passthrough 層まで
	// 到達している証拠。
	resp2 := postToolCall(t, srv, mergeHeaders(map[string]string{"X-Logvalet-Api-Key": key}, identityHeaders))
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp2.StatusCode)
	}
	env := decodeErrorEnvelope(t, resp2)
	if env.Error.Code != "authentication_error" {
		t.Errorf("error.code = %q, want %q (passthrough, not apikey's %q)", env.Error.Code, "authentication_error", "unauthorized")
	}

	// apikey 自体が無効 → apikey 層で弾かれ、passthrough/Backlog には到達しない。
	resp3 := postToolCall(t, srv, map[string]string{"Authorization": "Bearer " + token})
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp3.StatusCode)
	}
	env3 := decodeErrorEnvelope(t, resp3)
	if env3.Error.Code != "unauthorized" {
		t.Errorf("error.code = %q, want %q (apikey layer)", env3.Error.Code, "unauthorized")
	}

	if mock.callCount() != 1 {
		t.Errorf("backlog mock should be called exactly once (only the valid request), count = %d", mock.callCount())
	}
}
