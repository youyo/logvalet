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
}

// --- McpCmd.Validate() Bearer関連テスト群 ---

func TestMcpCmd_Validate_BearerMode_ValidToken(t *testing.T) {
	// auth-mode=bearer + token 32文字 → pass
	cmd := &cli.McpCmd{
		Auth:        false,
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
		Auth:        false,
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
		Auth:        false,
		AuthMode:    "bearer",
		BearerToken: strings.Repeat("x", 31),
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error when bearer token is shorter than 32 chars")
	}
}

func TestMcpCmd_Validate_BearerMode_And_Auth_Exclusive(t *testing.T) {
	// --auth=true と --auth-mode=bearer は排他
	cmd := &cli.McpCmd{
		Auth:        true,
		AuthMode:    "bearer",
		BearerToken: strings.Repeat("x", 32),
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error: --auth and --auth-mode=bearer are mutually exclusive")
	}
}

func TestMcpCmd_Validate_NoneMode_And_Auth_Contradiction(t *testing.T) {
	// --auth=true と --auth-mode=none は矛盾
	cmd := &cli.McpCmd{
		Auth:     true,
		AuthMode: "none",
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error: --auth=true and --auth-mode=none are contradictory")
	}
}

func TestMcpCmd_Validate_OIDCMode_Auth_True(t *testing.T) {
	// Auth=true, AuthMode="oidc" → OIDCバリデーションが走る（既存動作維持）
	t.Setenv("LOGVALET_SPACE_STORE_TYPE", "dynamodb")
	cmd := &cli.McpCmd{
		Auth:     true,
		AuthMode: "oidc",
		// OIDC フィールドなし → エラーになるが bearer-related エラーではない
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error when oidc mode without required fields")
	}
	// bearer 関連エラーでないことを確認
	if strings.Contains(err.Error(), "bearer") {
		t.Fatalf("should not be bearer error, got: %v", err)
	}
}

func TestMcpCmd_Validate_InvalidAuthMode(t *testing.T) {
	// 未知の auth-mode → error
	cmd := &cli.McpCmd{
		Auth:     false,
		AuthMode: "magic",
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for unknown auth-mode")
	}
}

func TestMcpCmd_Validate_AuthFalse_NoAuthMode_Pass(t *testing.T) {
	// Auth=false, AuthMode="" → no-auth、バリデーション通過（既存テストとの整合）
	cmd := &cli.McpCmd{Auth: false}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
