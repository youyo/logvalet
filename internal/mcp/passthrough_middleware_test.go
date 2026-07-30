package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/mcp"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{name: "valid bearer", header: "Bearer abc123", wantToken: "abc123", wantOK: true},
		{name: "empty header", header: "", wantToken: "", wantOK: false},
		{name: "missing scheme", header: "abc123", wantToken: "", wantOK: false},
		{name: "wrong scheme", header: "Basic abc123", wantToken: "", wantOK: false},
		{name: "empty token after prefix", header: "Bearer ", wantToken: "", wantOK: false},
		{name: "whitespace only token", header: "Bearer    ", wantToken: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotOK := mcp.ExtractBearerToken(tt.header)
			if gotOK != tt.wantOK || gotToken != tt.wantToken {
				t.Errorf("ExtractBearerToken(%q) = (%q, %v), want (%q, %v)",
					tt.header, gotToken, gotOK, tt.wantToken, tt.wantOK)
			}
		})
	}
}

// TestPassthroughAuthMiddleware_MissingHeader は、Authorization ヘッダー無しの
// リクエストに対して、next を呼ばずに 401 + spec §9 エラーエンベロープを
// 返すことを検証する (done_criteria #3)。
func TestPassthroughAuthMiddleware_MissingHeader(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := mcp.PassthroughAuthMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("next should not be called when Authorization header is missing")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		Error         struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response body as JSON envelope: %v (body=%s)", err, rec.Body.String())
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Error.Code == "" {
		t.Error("error.code should not be empty")
	}
	if envelope.Error.Message == "" {
		t.Error("error.message should not be empty")
	}
	if envelope.Error.Retryable {
		t.Error("error.retryable should be false for a missing-header auth failure")
	}
}

// TestPassthroughAuthMiddleware_MalformedHeader は、Bearer スキームでない
// Authorization ヘッダーも同様に拒否されることを検証する。
func TestPassthroughAuthMiddleware_MalformedHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called for a malformed Authorization header")
	})

	handler := mcp.PassthroughAuthMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestPassthroughAuthMiddleware_ValidHeader は、有効な Authorization: Bearer
// ヘッダーがリクエストスコープの context へ運ばれ、next ハンドラーから
// auth.PassthroughTokenFromContext で取得できることを検証する。
func TestPassthroughAuthMiddleware_ValidHeader(t *testing.T) {
	var gotToken string
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken, gotOK = auth.PassthroughTokenFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mcp.PassthroughAuthMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer request-scoped-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !gotOK {
		t.Fatal("expected passthrough token to be present in next handler's context")
	}
	if gotToken != "request-scoped-token" {
		t.Errorf("token = %q, want %q", gotToken, "request-scoped-token")
	}
}

// TestPassthroughAuthMiddleware_RequestIsolation は、並行する2つのリクエストが
// 異なるトークンを持ち、互いに context を共有・汚染しないことを検証する
// (done_criteria #2: リクエスト単位の分離)。
func TestPassthroughAuthMiddleware_RequestIsolation(t *testing.T) {
	seen := make(chan string, 2)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := auth.PassthroughTokenFromContext(r.Context())
		seen <- token
		w.WriteHeader(http.StatusOK)
	})
	handler := mcp.PassthroughAuthMiddleware(next)

	reqA := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	reqA.Header.Set("Authorization", "Bearer token-A")
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)

	reqB := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	reqB.Header.Set("Authorization", "Bearer token-B")
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	got := map[string]bool{<-seen: true, <-seen: true}
	if !got["token-A"] || !got["token-B"] {
		t.Fatalf("expected both token-A and token-B to be observed independently, got %v", got)
	}
}
