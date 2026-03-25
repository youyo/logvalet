package backlog_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

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

	t.Run("assigneeId[] single value", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			AssigneeIDs: []int{42},
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if ids := gotQuery["assigneeId[]"]; len(ids) != 1 || ids[0] != "42" {
			t.Errorf("assigneeId[] = %v, want [42]", ids)
		}
	})

	t.Run("assigneeId[] multiple values", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			AssigneeIDs: []int{1, 2},
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if ids := gotQuery["assigneeId[]"]; len(ids) != 2 {
			t.Errorf("assigneeId[] = %v, want 2 items", ids)
		}
	})

	t.Run("statusId[] multiple values", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			StatusIDs: []int{1, 2, 3},
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		ids := gotQuery["statusId[]"]
		if len(ids) != 3 {
			t.Fatalf("statusId[] = %v, want 3 items", ids)
		}
		for i, want := range []string{"1", "2", "3"} {
			if ids[i] != want {
				t.Errorf("statusId[][%d] = %q, want %q", i, ids[i], want)
			}
		}
	})

	t.Run("dueDateSince and dueDateUntil", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		since := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)
		until := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)
		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			DueDateSince: &since,
			DueDateUntil: &until,
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if gotQuery.Get("dueDateSince") != "2026-03-23" {
			t.Errorf("dueDateSince = %q, want %q", gotQuery.Get("dueDateSince"), "2026-03-23")
		}
		if gotQuery.Get("dueDateUntil") != "2026-03-23" {
			t.Errorf("dueDateUntil = %q, want %q", gotQuery.Get("dueDateUntil"), "2026-03-23")
		}
	})

	t.Run("startDateSince and startDateUntil", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		until := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			StartDateSince: &since,
			StartDateUntil: &until,
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if gotQuery.Get("startDateSince") != "2026-03-01" {
			t.Errorf("startDateSince = %q, want %q", gotQuery.Get("startDateSince"), "2026-03-01")
		}
		if gotQuery.Get("startDateUntil") != "2026-03-31" {
			t.Errorf("startDateUntil = %q, want %q", gotQuery.Get("startDateUntil"), "2026-03-31")
		}
	})

	t.Run("nil AssigneeIDs and StatusIDs are excluded", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
			AssigneeIDs: nil,
			StatusIDs:   nil,
		})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if _, ok := gotQuery["assigneeId[]"]; ok {
			t.Error("assigneeId[] should not be present when AssigneeIDs is nil")
		}
		if _, ok := gotQuery["statusId[]"]; ok {
			t.Error("statusId[] should not be present when StatusIDs is nil")
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

func TestHTTPClientListProjectIssueTypes(t *testing.T) {
	t.Run("calls correct endpoint and returns IDName list", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			issueTypes := []map[string]interface{}{
				{"id": 1, "name": "課題"},
				{"id": 2, "name": "バグ"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(issueTypes)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		result, err := client.ListProjectIssueTypes(context.Background(), "PROJ")
		if err != nil {
			t.Fatalf("ListProjectIssueTypes() error = %v", err)
		}
		if gotPath != "/api/v2/projects/PROJ/issueTypes" {
			t.Errorf("path = %q, want %q", gotPath, "/api/v2/projects/PROJ/issueTypes")
		}
		if len(result) != 2 {
			t.Fatalf("len(result) = %d, want 2", len(result))
		}
		if result[0].ID != 1 || result[0].Name != "課題" {
			t.Errorf("result[0] = %+v, want {ID:1, Name:課題}", result[0])
		}
	})
}

func TestHTTPClientListPriorities(t *testing.T) {
	t.Run("calls correct endpoint and returns IDName list", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			priorities := []map[string]interface{}{
				{"id": 2, "name": "高"},
				{"id": 3, "name": "中"},
				{"id": 4, "name": "低"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(priorities)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		result, err := client.ListPriorities(context.Background())
		if err != nil {
			t.Fatalf("ListPriorities() error = %v", err)
		}
		if gotPath != "/api/v2/priorities" {
			t.Errorf("path = %q, want %q", gotPath, "/api/v2/priorities")
		}
		if len(result) != 3 {
			t.Fatalf("len(result) = %d, want 3", len(result))
		}
	})
}

func TestHTTPClientCreateIssue_sendsProjectId(t *testing.T) {
	var gotProjectID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("projectId")
		issue := map[string]interface{}{"id": 1, "issueKey": "PROJ-1", "summary": "test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issue)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.CreateIssue(context.Background(), backlog.CreateIssueRequest{
		ProjectID:   42,
		Summary:     "test",
		IssueTypeID: 1,
		PriorityID:  3,
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if gotProjectID != "42" {
		t.Errorf("projectId = %q, want %q (should be numeric ID, not key)", gotProjectID, "42")
	}
}

func TestHTTPClientCreateIssue_allParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		issue := map[string]interface{}{"id": 1, "issueKey": "PROJ-1", "summary": "test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issue)
	}))
	defer srv.Close()

	dueDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	startDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	client := newOAuthClient(t, srv.URL)
	_, err := client.CreateIssue(context.Background(), backlog.CreateIssueRequest{
		ProjectID:       42,
		Summary:         "test",
		IssueTypeID:     1,
		PriorityID:      3,
		AssigneeID:      100,
		CategoryIDs:     []int{10, 11},
		VersionIDs:      []int{20},
		MilestoneIDs:    []int{30},
		DueDate:         &dueDate,
		StartDate:       &startDate,
		ParentIssueID:   5,
		NotifiedUserIDs: []int{50, 51},
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	checks := map[string]string{
		"projectId":     "42",
		"issueTypeId":   "1",
		"priorityId":    "3",
		"assigneeId":    "100",
		"parentIssueId": "5",
		"dueDate":       "2026-04-01",
		"startDate":     "2026-03-01",
	}
	for key, want := range checks {
		if gotQuery.Get(key) != want {
			t.Errorf("query[%q] = %q, want %q", key, gotQuery.Get(key), want)
		}
	}
	if catIDs := gotQuery["categoryId[]"]; len(catIDs) != 2 {
		t.Errorf("categoryId[] = %v, want 2 items", catIDs)
	}
	if notIDs := gotQuery["notifiedUserId[]"]; len(notIDs) != 2 {
		t.Errorf("notifiedUserId[] = %v, want 2 items", notIDs)
	}
}

func TestHTTPClientCreateIssue_optionalSkip(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		issue := map[string]interface{}{"id": 1, "issueKey": "PROJ-1", "summary": "test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issue)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.CreateIssue(context.Background(), backlog.CreateIssueRequest{
		ProjectID:   42,
		Summary:     "test",
		IssueTypeID: 1,
		PriorityID:  3,
		AssigneeID:  0, // 未指定
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if _, ok := gotQuery["assigneeId"]; ok {
		t.Error("assigneeId should not be present when AssigneeID=0")
	}
}

func TestHTTPClientUpdateIssue_allParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		issue := map[string]interface{}{"id": 1, "issueKey": "PROJ-1", "summary": "test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issue)
	}))
	defer srv.Close()

	statusID := 2
	priorityID := 3
	assigneeID := 100
	issueTypeID := 1
	comment := "更新コメント"
	dueDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	client := newOAuthClient(t, srv.URL)
	_, err := client.UpdateIssue(context.Background(), "PROJ-1", backlog.UpdateIssueRequest{
		StatusID:        &statusID,
		PriorityID:      &priorityID,
		AssigneeID:      &assigneeID,
		IssueTypeID:     &issueTypeID,
		CategoryIDs:     []int{10},
		VersionIDs:      []int{20},
		MilestoneIDs:    []int{30},
		NotifiedUserIDs: []int{50},
		DueDate:         &dueDate,
		Comment:         &comment,
	})
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
	checks := map[string]string{
		"statusId":    "2",
		"priorityId":  "3",
		"assigneeId":  "100",
		"issueTypeId": "1",
		"dueDate":     "2026-04-01",
		"comment":     "更新コメント",
	}
	for key, want := range checks {
		if gotQuery.Get(key) != want {
			t.Errorf("query[%q] = %q, want %q", key, gotQuery.Get(key), want)
		}
	}
}

func TestHTTPClientUpdateIssue_nilSkip(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		issue := map[string]interface{}{"id": 1, "issueKey": "PROJ-1", "summary": "test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issue)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.UpdateIssue(context.Background(), "PROJ-1", backlog.UpdateIssueRequest{})
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
	for _, key := range []string{"statusId", "priorityId", "assigneeId", "issueTypeId", "comment"} {
		if _, ok := gotQuery[key]; ok {
			t.Errorf("query[%q] should not be present for nil fields", key)
		}
	}
}

func TestHTTPClientAddIssueComment_withNotify(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		comment := map[string]interface{}{"id": 1, "content": "test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(comment)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.AddIssueComment(context.Background(), "PROJ-1", backlog.AddCommentRequest{
		Content:         "修正完了",
		NotifiedUserIDs: []int{1, 2},
	})
	if err != nil {
		t.Fatalf("AddIssueComment() error = %v", err)
	}
	if gotQuery.Get("content") != "修正完了" {
		t.Errorf("content = %q, want %q", gotQuery.Get("content"), "修正完了")
	}
	notIDs := gotQuery["notifiedUserId[]"]
	if len(notIDs) != 2 {
		t.Errorf("notifiedUserId[] = %v, want 2 items", notIDs)
	}
}

func TestHTTPClientCreateDocument_allParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		doc := map[string]interface{}{"id": "doc-1", "title": "Test"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	parentID := "parent-uuid"
	client := newOAuthClient(t, srv.URL)
	_, err := client.CreateDocument(context.Background(), backlog.CreateDocumentRequest{
		ProjectID: 42,
		Title:     "テストドキュメント",
		Content:   "本文",
		ParentID:  &parentID,
		Emoji:     "📝",
		AddLast:   true,
	})
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	checks := map[string]string{
		"projectId": "42",
		"title":     "テストドキュメント",
		"content":   "本文",
		"parentId":  "parent-uuid",
		"emoji":     "📝",
		"addLast":   "true",
	}
	for key, want := range checks {
		if gotQuery.Get(key) != want {
			t.Errorf("query[%q] = %q, want %q", key, gotQuery.Get(key), want)
		}
	}
}

// TestHTTPClientListIssues_sort_order は Sort/Order フィールドがクエリパラメータとして送信されることを確認。
func TestHTTPClientListIssues_sort_order(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
		Sort:  "dueDate",
		Order: "asc",
	})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if gotQuery.Get("sort") != "dueDate" {
		t.Errorf("sort = %q, want %q", gotQuery.Get("sort"), "dueDate")
	}
	if gotQuery.Get("order") != "asc" {
		t.Errorf("order = %q, want %q", gotQuery.Get("order"), "asc")
	}
}

// TestHTTPClientListIssues_sort_empty は Sort/Order が空のとき sort/order クエリが含まれないことを確認。
func TestHTTPClientListIssues_sort_empty(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
		Sort:  "",
		Order: "",
	})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if _, ok := gotQuery["sort"]; ok {
		t.Error("sort should not be present when Sort is empty")
	}
	if _, ok := gotQuery["order"]; ok {
		t.Error("order should not be present when Order is empty")
	}
}

// TestHTTPClientGetTeam は GetTeam の正常系テスト。
func TestHTTPClientGetTeam(t *testing.T) {
	t.Run("returns TeamWithMembers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/teams/173843" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			team := map[string]interface{}{
				"id":           173843,
				"name":         "Test Team",
				"displayOrder": 1,
				"members": []map[string]interface{}{
					{"id": 10, "userId": "user10", "name": "User Ten"},
					{"id": 20, "userId": "user20", "name": "User Twenty"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(team)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.GetTeam(context.Background(), 173843)
		if err != nil {
			t.Fatalf("GetTeam() error = %v", err)
		}
		if got.ID != 173843 {
			t.Errorf("ID = %d, want 173843", got.ID)
		}
		if got.Name != "Test Team" {
			t.Errorf("Name = %q, want %q", got.Name, "Test Team")
		}
		if len(got.Members) != 2 {
			t.Fatalf("len(Members) = %d, want 2", len(got.Members))
		}
		if got.Members[0].ID != 10 || got.Members[0].Name != "User Ten" {
			t.Errorf("Members[0] = %+v, want {ID:10, Name:User Ten}", got.Members[0])
		}
		if got.Members[1].ID != 20 || got.Members[1].Name != "User Twenty" {
			t.Errorf("Members[1] = %+v, want {ID:20, Name:User Twenty}", got.Members[1])
		}
	})
}

// TestHTTPClientGetTeam_notFound は GetTeam の 404 → ErrNotFound テスト。
func TestHTTPClientGetTeam_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		body := map[string]interface{}{
			"errors": []map[string]interface{}{
				{"message": "No team found.", "code": 404},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.GetTeam(context.Background(), 999)
	if !errors.Is(err, backlog.ErrNotFound) {
		t.Errorf("GetTeam() error = %v, want ErrNotFound", err)
	}
}

// TestListIssues_updatedSinceUntil は updatedSince/Until がクエリに含まれることを確認。
func TestListIssues_updatedSinceUntil(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer srv.Close()

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	client := newOAuthClient(t, srv.URL)
	_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
		UpdatedSince: &since,
		UpdatedUntil: &until,
	})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if gotQuery.Get("updatedSince") != "2026-03-01" {
		t.Errorf("updatedSince = %q, want %q", gotQuery.Get("updatedSince"), "2026-03-01")
	}
	if gotQuery.Get("updatedUntil") != "2026-03-31" {
		t.Errorf("updatedUntil = %q, want %q", gotQuery.Get("updatedUntil"), "2026-03-31")
	}
}

// TestListIssues_updatedEmpty は UpdatedSince/Until が nil のとき updatedSince/Until が含まれないことを確認。
func TestListIssues_updatedEmpty(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.ListIssues(context.Background(), backlog.ListIssuesOptions{
		UpdatedSince: nil,
		UpdatedUntil: nil,
	})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if _, ok := gotQuery["updatedSince"]; ok {
		t.Error("updatedSince should not be present when UpdatedSince is nil")
	}
	if _, ok := gotQuery["updatedUntil"]; ok {
		t.Error("updatedUntil should not be present when UpdatedUntil is nil")
	}
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
