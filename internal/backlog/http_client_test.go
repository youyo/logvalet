package backlog_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/domain"
)

// newOAuthClient は OAuth 認証の HTTPClient をテスト用 httptest サーバーへ向けて生成する。
func newOAuthClient(t *testing.T, baseURL string) *backlog.HTTPClient {
	t.Helper()
	cred := &credentials.ResolvedCredential{
		AuthType:    "oauth",
		AccessToken: "test-token",
		Source:      "flag",
	}
	return backlog.NewHTTPClient(backlog.ClientConfig{
		BaseURL:    baseURL,
		Credential: cred,
	})
}

// newAPIKeyClient は API key 認証の HTTPClient をテスト用 httptest サーバーへ向けて生成する。
func newAPIKeyClient(t *testing.T, baseURL string) *backlog.HTTPClient {
	t.Helper()
	cred := &credentials.ResolvedCredential{
		AuthType: "api_key",
		APIKey:   "my-api-key",
		Source:   "env",
	}
	return backlog.NewHTTPClient(backlog.ClientConfig{
		BaseURL:    baseURL,
		Credential: cred,
	})
}

func TestHTTPClientGetMyself(t *testing.T) {
	t.Run("OAuth: sets Authorization: Bearer header", func(t *testing.T) {
		var gotAuthHeader string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuthHeader = r.Header.Get("Authorization")
			user := map[string]interface{}{"id": 1, "userId": "user1", "name": "Test User"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		user, err := client.GetMyself(context.Background())
		if err != nil {
			t.Fatalf("GetMyself() error = %v", err)
		}
		if gotAuthHeader != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", gotAuthHeader, "Bearer test-token")
		}
		if user == nil {
			t.Fatal("GetMyself() returned nil user")
		}
	})

	t.Run("API key: adds apiKey query parameter", func(t *testing.T) {
		var gotAPIKey string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAPIKey = r.URL.Query().Get("apiKey")
			user := map[string]interface{}{"id": 2, "userId": "user2", "name": "API Key User"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		}))
		defer srv.Close()

		client := newAPIKeyClient(t, srv.URL)
		user, err := client.GetMyself(context.Background())
		if err != nil {
			t.Fatalf("GetMyself() error = %v", err)
		}
		if gotAPIKey != "my-api-key" {
			t.Errorf("apiKey query param = %q, want %q", gotAPIKey, "my-api-key")
		}
		if user == nil {
			t.Fatal("GetMyself() returned nil user")
		}
	})

	t.Run("calls correct endpoint", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			user := map[string]interface{}{"id": 1, "userId": "u", "name": "N"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, _ = client.GetMyself(context.Background())
		if gotPath != "/api/v2/users/myself" {
			t.Errorf("path = %q, want %q", gotPath, "/api/v2/users/myself")
		}
	})
}

func TestHTTPClientErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"404 -> ErrNotFound", http.StatusNotFound, backlog.ErrNotFound},
		{"401 -> ErrUnauthorized", http.StatusUnauthorized, backlog.ErrUnauthorized},
		{"403 -> ErrForbidden", http.StatusForbidden, backlog.ErrForbidden},
		{"429 -> ErrRateLimited", http.StatusTooManyRequests, backlog.ErrRateLimited},
		{"500 -> ErrAPI", http.StatusInternalServerError, backlog.ErrAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				body := map[string]interface{}{
					"errors": []map[string]interface{}{
						{"message": "error message", "code": tt.statusCode},
					},
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer srv.Close()

			client := newOAuthClient(t, srv.URL)
			_, err := client.GetMyself(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetMyself() error = %v, want errors.Is(err, %v) = true", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPClientGetIssue(t *testing.T) {
	t.Run("returns issue by key", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/issues/PROJ-123" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			issue := map[string]interface{}{
				"id":       555,
				"issueKey": "PROJ-123",
				"summary":  "Test issue",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(issue)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		issue, err := client.GetIssue(context.Background(), "PROJ-123")
		if err != nil {
			t.Fatalf("GetIssue() error = %v", err)
		}
		if issue.IssueKey != "PROJ-123" {
			t.Errorf("IssueKey = %q, want %q", issue.IssueKey, "PROJ-123")
		}
	})
}

func TestHTTPClientListIssues(t *testing.T) {
	t.Run("sets query params with projectId", func(t *testing.T) {
		var gotProjectIDs []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotProjectIDs = r.URL.Query()["projectId[]"]
			issues := []map[string]interface{}{
				{"id": 1, "issueKey": "PROJ-1", "summary": "Issue 1"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(issues)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			ProjectIDs: []int{42, 99},
			Limit:      10,
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if len(gotProjectIDs) != 2 || gotProjectIDs[0] != "42" || gotProjectIDs[1] != "99" {
			t.Errorf("projectId[] query = %v, want [42 99]", gotProjectIDs)
		}
	})
}

func TestHTTPClientContextCancellation(t *testing.T) {
	t.Run("cancelled context returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// レスポンスを返さずにブロック（テスト内でキャンセルするので問題なし）
			<-r.Context().Done()
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 即座にキャンセル

		client := newOAuthClient(t, srv.URL)
		_, err := client.GetMyself(ctx)
		if err == nil {
			t.Error("GetMyself() should return error for cancelled context")
		}
	})
}

func TestHTTPClientGetSpace(t *testing.T) {
	t.Run("returns space info", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/space" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			space := map[string]interface{}{
				"spaceKey": "example",
				"name":     "Example Space",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(space)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		space, err := client.GetSpace(context.Background())
		if err != nil {
			t.Fatalf("GetSpace() error = %v", err)
		}
		if space.SpaceKey != "example" {
			t.Errorf("SpaceKey = %q, want %q", space.SpaceKey, "example")
		}
	})
}

func TestHTTPClientGetProject(t *testing.T) {
	t.Run("returns project by key", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			project := map[string]interface{}{
				"id":         1,
				"projectKey": "PROJ",
				"name":       "My Project",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(project)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		proj, err := client.GetProject(context.Background(), "PROJ")
		if err != nil {
			t.Fatalf("GetProject() error = %v", err)
		}
		if proj == nil {
			t.Fatal("GetProject() returned nil")
		}
	})
}

func TestHTTPClientImplementsClient(t *testing.T) {
	// HTTPClient が Client interface を実装していることを確認
	var _ backlog.Client = (*backlog.HTTPClient)(nil)
}

// TestHTTPClientGetMyselfParsesUserFields は GetMyself のレスポンスが正しく domain.User にマップされるかテスト。
func TestHTTPClientGetMyselfParsesUserFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := domain.User{
			ID:     42,
			UserID: "testuser",
			Name:   "Test User",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	got, err := client.GetMyself(context.Background())
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}
	if got.UserID != "testuser" {
		t.Errorf("UserID = %q, want %q", got.UserID, "testuser")
	}
}
