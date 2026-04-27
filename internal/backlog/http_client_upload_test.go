package backlog_test

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientUploadAttachment_Normal(t *testing.T) {
	const wantFilename = "test.txt"
	const wantContent = "hello world"

	var gotFilename string
	var gotContent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/space/attachment" {
			t.Errorf("path = %q, want /api/v2/space/attachment", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		// Content-Type が multipart/form-data であること
		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("Content-Type = %q, want multipart/form-data", ct)
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		gotFilename = part.FileName()
		body, _ := io.ReadAll(part)
		gotContent = string(body)

		resp := map[string]interface{}{"id": 100, "name": wantFilename, "size": 11}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	att, err := client.UploadAttachment(context.Background(), wantFilename, strings.NewReader(wantContent))
	if err != nil {
		t.Fatalf("UploadAttachment() error = %v", err)
	}
	if att == nil {
		t.Fatal("UploadAttachment() returned nil")
	}
	if att.ID != 100 {
		t.Errorf("ID = %d, want 100", att.ID)
	}
	if att.Name != wantFilename {
		t.Errorf("Name = %q, want %q", att.Name, wantFilename)
	}
	if gotFilename != wantFilename {
		t.Errorf("multipart filename = %q, want %q", gotFilename, wantFilename)
	}
	if gotContent != wantContent {
		t.Errorf("multipart content = %q, want %q", gotContent, wantContent)
	}
}

func TestHTTPClientUploadAttachment_OAuthBearer(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		resp := map[string]interface{}{"id": 1, "name": "f.txt", "size": 0}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.UploadAttachment(context.Background(), "f.txt", strings.NewReader(""))
	if err != nil {
		t.Fatalf("UploadAttachment() error = %v", err)
	}
	if gotAuthHeader != "Bearer test-token" {
		t.Errorf("Authorization = %q, want 'Bearer test-token'", gotAuthHeader)
	}
}

func TestHTTPClientUploadAttachment_APIKey(t *testing.T) {
	var gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.URL.Query().Get("apiKey")
		resp := map[string]interface{}{"id": 1, "name": "f.txt", "size": 0}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newAPIKeyClient(t, srv.URL)
	_, err := client.UploadAttachment(context.Background(), "f.txt", strings.NewReader(""))
	if err != nil {
		t.Fatalf("UploadAttachment() error = %v", err)
	}
	if gotAPIKey != "my-api-key" {
		t.Errorf("apiKey = %q, want my-api-key", gotAPIKey)
	}
}
