package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// apiKeyHeader は Gateway ⇔ logvalet 契約（gateway-request-contract.md §1.2）で確定した
// apikey ヘッダー名。Authorization は Backlog credential passthrough 専用に予約されている。
const apiKeyHeader = "X-Logvalet-Api-Key"

func apiKeyOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func serveWithAPIKey(t *testing.T, key string, setup func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if setup != nil {
		setup(req)
	}
	rr := httptest.NewRecorder()
	cli.APIKeyAuthMiddlewareForTest(key)(apiKeyOKHandler()).ServeHTTP(rr, req)
	return rr
}

// --- apiKeyAuthMiddleware テスト群 ---

func TestAPIKeyAuthMiddleware_ValidKey(t *testing.T) {
	key := strings.Repeat("a", 32)
	rr := serveWithAPIKey(t, key, func(r *http.Request) {
		r.Header.Set(apiKeyHeader, key)
	})
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestAPIKeyAuthMiddleware_MissingHeader(t *testing.T) {
	rr := serveWithAPIKey(t, strings.Repeat("a", 32), nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestAPIKeyAuthMiddleware_WrongKey(t *testing.T) {
	rr := serveWithAPIKey(t, strings.Repeat("a", 32), func(r *http.Request) {
		r.Header.Set(apiKeyHeader, "wrongkey")
	})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

// 契約 §1.3: 値はスキームプレフィックスを持たない生のトークン。
func TestAPIKeyAuthMiddleware_BearerPrefixRejected(t *testing.T) {
	key := strings.Repeat("a", 32)
	rr := serveWithAPIKey(t, key, func(r *http.Request) {
		r.Header.Set(apiKeyHeader, "Bearer "+key)
	})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (scheme prefix must not be accepted)", rr.Code)
	}
}

// 契約 §1.2/§4.1: Authorization は Backlog passthrough 専用。apikey として受理してはならない。
func TestAPIKeyAuthMiddleware_AuthorizationHeaderNotAccepted(t *testing.T) {
	key := strings.Repeat("a", 32)
	for _, v := range []string{"Bearer " + key, key} {
		rr := serveWithAPIKey(t, key, func(r *http.Request) {
			r.Header.Set("Authorization", v)
		})
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Authorization: %q → status = %d, want 401 (Authorization is reserved for Backlog passthrough)", v, rr.Code)
		}
	}
}

// 契約 §1.3: 大小文字を区別する完全一致比較。
func TestAPIKeyAuthMiddleware_CaseSensitive(t *testing.T) {
	key := "Token123" + strings.Repeat("x", 24)
	rr := serveWithAPIKey(t, key, func(r *http.Request) {
		r.Header.Set(apiKeyHeader, strings.ToLower(key))
	})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (api key is case-sensitive)", rr.Code)
	}
}

// fail-closed: 設定鍵が空の場合はすべて拒否する（空ヘッダーとの一致で素通りさせない）。
func TestAPIKeyAuthMiddleware_EmptyConfiguredKeyDeniesAll(t *testing.T) {
	for _, v := range []string{"", "anything"} {
		rr := serveWithAPIKey(t, "", func(r *http.Request) {
			r.Header.Set(apiKeyHeader, v)
		})
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("header %q → status = %d, want 401 (empty configured key must deny all)", v, rr.Code)
		}
	}
}

// 契約 §1.4: WWW-Authenticate は付与しない（Bearer スキームではないため RFC 6750 対象外）。
// ボディは spec §9 のエラーエンベロープ形式。
func TestAPIKeyAuthMiddleware_UnauthorizedResponseShape(t *testing.T) {
	rr := serveWithAPIKey(t, strings.Repeat("a", 32), nil)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if wa := rr.Header().Get("WWW-Authenticate"); wa != "" {
		t.Errorf("WWW-Authenticate = %q, want empty (contract §1.4)", wa)
	}

	var env struct {
		SchemaVersion string `json:"schema_version"`
		Error         struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not a JSON error envelope: %v (body=%s)", err, rr.Body.String())
	}
	if env.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want \"1\"", env.SchemaVersion)
	}
	if env.Error.Code != "unauthorized" {
		t.Errorf("error.code = %q, want \"unauthorized\"", env.Error.Code)
	}
	if env.Error.Message == "" {
		t.Error("error.message must not be empty")
	}
	if env.Error.Retryable {
		t.Error("error.retryable = true, want false")
	}
}

// 定数時間比較の使用確認: 鍵比較は crypto/subtle.ConstantTimeCompare で行う。
func TestAPIKeyAuthMiddleware_UsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("mcp_apikey.go")
	if err != nil {
		t.Fatalf("read mcp_apikey.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, `"crypto/subtle"`) || !strings.Contains(body, "subtle.ConstantTimeCompare") {
		t.Error("api key comparison must use crypto/subtle.ConstantTimeCompare")
	}
	if strings.Contains(body, "provided == ") || strings.Contains(body, "== key") {
		t.Error("api key must not be compared with ==")
	}
}

// --- resolvedAuthMode テスト群 ---

func TestMcpCmd_ResolvedAuthMode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "none"},
		{"none", "none"},
		{"NONE", "none"},
		{" none ", "none"},
		{"apikey", "apikey"},
		{"APIKEY", "apikey"},
		{"bearer", "apikey"}, // 別名
		{"BEARER", "apikey"},
	}
	for _, tc := range cases {
		cmd := &cli.McpCmd{AuthMode: tc.in}
		if got := cli.ResolvedAuthModeForTest(cmd); got != tc.want {
			t.Errorf("resolvedAuthMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// 未知の値は Validate で弾かれるが、万一 Run まで到達した場合は
// fail-closed で apikey 扱いにする（none にフォールバックして無認証で公開しない）。
func TestMcpCmd_ResolvedAuthMode_UnknownIsFailClosed(t *testing.T) {
	cmd := &cli.McpCmd{AuthMode: "magic"}
	if got := cli.ResolvedAuthModeForTest(cmd); got != "apikey" {
		t.Errorf("resolvedAuthMode(\"magic\") = %q, want \"apikey\" (fail-closed)", got)
	}
}

// --- Validate テスト群 ---

func TestMcpCmd_Validate_APIKeyMode(t *testing.T) {
	valid := strings.Repeat("x", 32)

	cases := []struct {
		name    string
		cmd     *cli.McpCmd
		wantErr bool
	}{
		{"apikey+api-key", &cli.McpCmd{AuthMode: "apikey", ApiKey: valid}, false},
		{"apikey+bearer-token alias", &cli.McpCmd{AuthMode: "apikey", BearerToken: valid}, false},
		{"bearer alias+api-key", &cli.McpCmd{AuthMode: "bearer", ApiKey: valid}, false},
		{"bearer alias+bearer-token", &cli.McpCmd{AuthMode: "bearer", BearerToken: valid}, false},
		{"apikey without key", &cli.McpCmd{AuthMode: "apikey"}, true},
		{"bearer without key", &cli.McpCmd{AuthMode: "bearer"}, true},
		{"apikey too short", &cli.McpCmd{AuthMode: "apikey", ApiKey: strings.Repeat("x", 31)}, true},
		{"bearer too short", &cli.McpCmd{AuthMode: "bearer", BearerToken: strings.Repeat("x", 31)}, true},
		{"none", &cli.McpCmd{AuthMode: "none"}, false},
		{"default", &cli.McpCmd{}, false},
		{"unknown", &cli.McpCmd{AuthMode: "magic"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --auth-mode=oidc は S19 の fail-fast を維持する。
func TestMcpCmd_Validate_AuthModeOIDC_StillFailsFast(t *testing.T) {
	err := (&cli.McpCmd{AuthMode: "oidc"}).Validate()
	if err == nil {
		t.Fatal("expected error for --auth-mode=oidc")
	}
	if !strings.Contains(err.Error(), "AgentCore Gateway") {
		t.Errorf("error should mention AgentCore Gateway delegation, got: %v", err)
	}
}

// フラグ経由（Kong パース）でも apikey モードが成立することを確認する。
// --auth-api-key はグローバルの --api-key（Backlog API キー）とは別物。
func TestMcpCmd_KongParse_APIKeyFlags(t *testing.T) {
	key := strings.Repeat("k", 32)

	cases := [][]string{
		{"mcp", "--auth-mode", "apikey", "--auth-api-key", key},
		{"mcp", "--auth-mode", "bearer", "--bearer-token", key},
	}

	for _, args := range cases {
		var root cli.CLI
		p, err := kong.New(&root,
			kong.Name("logvalet"),
			kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
			kong.Exit(func(int) {}),
		)
		if err != nil {
			t.Fatalf("kong.New() error: %v", err)
		}
		if _, err := p.Parse(args); err != nil {
			t.Fatalf("Parse(%v) error: %v", args, err)
		}
		if got := cli.ResolvedAuthModeForTest(&root.Mcp); got != "apikey" {
			t.Errorf("Parse(%v) → resolvedAuthMode = %q, want \"apikey\"", args, got)
		}
		if got := cli.APIKeyValueForTest(&root.Mcp); got != key {
			t.Errorf("Parse(%v) → apiKeyValue = %q, want %q", args, got, key)
		}
	}
}

// 有効鍵の解決: --auth-api-key が優先され、未指定なら --bearer-token（別名）を使う。
func TestMcpCmd_APIKeyValue(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cli.McpCmd
		want string
	}{
		{"api-key only", &cli.McpCmd{ApiKey: "aaa"}, "aaa"},
		{"bearer-token only", &cli.McpCmd{BearerToken: "bbb"}, "bbb"},
		{"api-key wins", &cli.McpCmd{ApiKey: "aaa", BearerToken: "bbb"}, "aaa"},
		{"neither", &cli.McpCmd{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cli.APIKeyValueForTest(tc.cmd); got != tc.want {
				t.Errorf("apiKeyValue() = %q, want %q", got, tc.want)
			}
		})
	}
}
