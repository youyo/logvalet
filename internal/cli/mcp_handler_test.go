package cli_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	cli.HealthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", w.Body.String(), `{"status":"ok"}`)
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	cli.HealthHandler(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
