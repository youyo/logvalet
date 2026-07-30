package cli_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

// --- bearerAuthMiddleware テスト群 ---

func TestBearerAuthMiddleware_ValidToken(t *testing.T) {
	token := strings.Repeat("a", 32)
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestBearerAuthMiddleware_MissingHeader(t *testing.T) {
	token := strings.Repeat("a", 32)
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	// Authorization ヘッダーなし
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestBearerAuthMiddleware_WrongScheme(t *testing.T) {
	token := strings.Repeat("a", 32)
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Basic "+token)
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestBearerAuthMiddleware_WrongToken(t *testing.T) {
	token := strings.Repeat("a", 32)
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestBearerAuthMiddleware_CaseInsensitiveScheme(t *testing.T) {
	token := strings.Repeat("a", 32)
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 小文字 "bearer"
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "bearer "+token)
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("[lowercase] status = %d, want 200", rr.Code)
	}

	// 大文字 "BEARER"
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req2.Header.Set("Authorization", "BEARER "+token)
	rr2 := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("[uppercase] status = %d, want 200", rr2.Code)
	}
}

func TestBearerAuthMiddleware_ResponseContentType(t *testing.T) {
	token := strings.Repeat("a", 32)
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	// ヘッダーなしで401を返させる
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	// RFC 6750準拠: 認証失敗時はWWW-Authenticateヘッダーを返す
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	if wwwAuth == "" {
		t.Errorf("WWW-Authenticate header missing, want Bearer realm=...")
	}
	if !strings.Contains(wwwAuth, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want to contain 'Bearer'", wwwAuth)
	}
}

func TestBearerAuthMiddleware_SchemeInsensitiveTokenSensitive(t *testing.T) {
	// スキームはcase-insensitive、トークン自体はcase-sensitiveであることを明示的に検証する。
	// 例: "BEARER Token123..." → スキームOK、トークン"Token123..."はcase-sensitiveに比較。
	token := "Token123" + strings.Repeat("x", 24) // 32文字、大文字小文字混在
	middleware := cli.BearerAuthMiddlewareForTest(token)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 大文字スキーム + 正しいトークン → 200
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "BEARER "+token)
	rr := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("[BEARER+correct] status = %d, want 200", rr.Code)
	}

	// 大文字スキーム + トークンを小文字に変換 → 401（トークンはcase-sensitive）
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req2.Header.Set("Authorization", "BEARER "+strings.ToLower(token))
	rr2 := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Errorf("[BEARER+lowercase-token] status = %d, want 401 (token is case-sensitive)", rr2.Code)
	}
}

// --- McpCmd.Validate() Bearer関連テスト群 ---

func TestMcpCmd_Validate_BearerMode_ValidToken(t *testing.T) {
	// auth-mode=bearer + token 32文字 → pass
	cmd := &cli.McpCmd{
		AuthMode:    "bearer",
		BearerToken: strings.Repeat("x", 32),
	}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMcpCmd_Validate_BearerMode_EmptyToken(t *testing.T) {
	// auth-mode=bearer + token="" → error (fail-closed)
	cmd := &cli.McpCmd{
		AuthMode:    "bearer",
		BearerToken: "",
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error when bearer token is empty")
	}
}

func TestMcpCmd_Validate_BearerMode_TooShortToken(t *testing.T) {
	// auth-mode=bearer + token 31文字 → error (min 32文字)
	cmd := &cli.McpCmd{
		AuthMode:    "bearer",
		BearerToken: strings.Repeat("x", 31),
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error when bearer token is shorter than 32 chars")
	}
}

func TestMcpCmd_Validate_InvalidAuthMode(t *testing.T) {
	// 未知の auth-mode → error
	cmd := &cli.McpCmd{AuthMode: "magic"}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for unknown auth-mode")
	}
}

func TestMcpCmd_Validate_NoneMode_Pass(t *testing.T) {
	// auth-mode=none → 認証なしで通過
	cmd := &cli.McpCmd{AuthMode: "none"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
