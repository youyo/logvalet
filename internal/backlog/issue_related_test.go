package backlog_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// TestHTTPClientListRelatedIssues は ListRelatedIssues のテスト。
func TestHTTPClientListRelatedIssues(t *testing.T) {
	t.Run("returns related issues list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/v2/issues/PROJ-1/relatedIssues" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			related := []map[string]interface{}{
				{"id": 10, "issueKey": "PROJ-2", "summary": "related issue", "type": "RELATES"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(related)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.ListRelatedIssues(context.Background(), "PROJ-1")
		if err != nil {
			t.Fatalf("ListRelatedIssues() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].IssueKey != "PROJ-2" || got[0].Type != "RELATES" {
			t.Errorf("unexpected result: %+v", got[0])
		}
	})

	t.Run("returns empty slice without error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.ListRelatedIssues(context.Background(), "PROJ-1")
		if err != nil {
			t.Fatalf("ListRelatedIssues() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("issueKey is url.PathEscape'd", func(t *testing.T) {
		var gotRequestURI string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRequestURI = r.RequestURI
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListRelatedIssues(context.Background(), "PROJ/1 A")
		if err != nil {
			t.Fatalf("ListRelatedIssues() error = %v", err)
		}
		// RequestURI はエンコード済みのままなので、"/" や空白が単一パスセグメント
		// として url.PathEscape でエンコードされたことを確認できる。
		wantSegment := url.PathEscape("PROJ/1 A")
		if !strings.Contains(gotRequestURI, "/api/v2/issues/"+wantSegment+"/relatedIssues") {
			t.Errorf("RequestURI = %q, want to contain escaped segment %q", gotRequestURI, wantSegment)
		}
	})

	t.Run("404 maps to ErrNotFound", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{{"message": "not found", "code": 404}},
			})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListRelatedIssues(context.Background(), "PROJ-1")
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("403 maps to ErrForbidden", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{{"message": "forbidden", "code": 403}},
			})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.ListRelatedIssues(context.Background(), "PROJ-1")
		if !errors.Is(err, backlog.ErrForbidden) {
			t.Errorf("error = %v, want ErrForbidden", err)
		}
	})
}

// TestHTTPClientAddRelatedIssue は AddRelatedIssue のテスト。
func TestHTTPClientAddRelatedIssue(t *testing.T) {
	t.Run("posts targetIssueId in body", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotForm url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			_ = r.ParseForm()
			gotForm = r.PostForm
			related := map[string]interface{}{"id": 20, "issueKey": "PROJ-3", "summary": "target", "type": "RELATES"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(related)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.AddRelatedIssue(context.Background(), "PROJ-1", backlog.AddRelatedIssueRequest{
			TargetIssueID: 20,
		})
		if err != nil {
			t.Fatalf("AddRelatedIssue() error = %v", err)
		}
		if gotMethod != http.MethodPost {
			t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
		}
		if gotPath != "/api/v2/issues/PROJ-1/relatedIssues" {
			t.Errorf("path = %q, want %q", gotPath, "/api/v2/issues/PROJ-1/relatedIssues")
		}
		if gotForm.Get("targetIssueId") != "20" {
			t.Errorf("targetIssueId = %q, want %q", gotForm.Get("targetIssueId"), "20")
		}
		if got.IssueKey != "PROJ-3" {
			t.Errorf("IssueKey = %q, want %q", got.IssueKey, "PROJ-3")
		}
	})
}

// TestHTTPClientDeleteRelatedIssue は DeleteRelatedIssue のテスト。
func TestHTTPClientDeleteRelatedIssue(t *testing.T) {
	t.Run("deletes related issue and returns info", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete || r.URL.Path != "/api/v2/issues/PROJ-1/relatedIssues/30" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			related := map[string]interface{}{"id": 30, "issueKey": "PROJ-4", "summary": "deleted", "type": "RELATES"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(related)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if err != nil {
			t.Fatalf("DeleteRelatedIssue() error = %v", err)
		}
		if got == nil || got.IssueKey != "PROJ-4" {
			t.Errorf("unexpected result: %+v", got)
		}
	})

	t.Run("204 No Content does not error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if err != nil {
			t.Fatalf("DeleteRelatedIssue() error = %v", err)
		}
		if got == nil {
			t.Fatal("DeleteRelatedIssue() returned nil result, want non-nil zero value")
		}
	})

	t.Run("200 with empty body does not error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		got, err := client.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if err != nil {
			t.Fatalf("DeleteRelatedIssue() error = %v", err)
		}
		if got == nil {
			t.Fatal("DeleteRelatedIssue() returned nil result, want non-nil zero value")
		}
	})

	t.Run("relatedIssueId path is built with issueKey escaped", func(t *testing.T) {
		var gotRequestURI string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRequestURI = r.RequestURI
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.DeleteRelatedIssue(context.Background(), "PROJ/1 A", 30)
		if err != nil {
			t.Fatalf("DeleteRelatedIssue() error = %v", err)
		}
		wantSegment := url.PathEscape("PROJ/1 A")
		if !strings.Contains(gotRequestURI, "/api/v2/issues/"+wantSegment+"/relatedIssues/30") {
			t.Errorf("RequestURI = %q, want to contain escaped segment %q", gotRequestURI, wantSegment)
		}
	})

	t.Run("404 maps to ErrNotFound", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{{"message": "not found", "code": 404}},
			})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("403 maps to ErrForbidden", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{{"message": "forbidden", "code": 403}},
			})
		}))
		defer srv.Close()

		client := newOAuthClient(t, srv.URL)
		_, err := client.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if !errors.Is(err, backlog.ErrForbidden) {
			t.Errorf("error = %v, want ErrForbidden", err)
		}
	})
}

// ---- MockClient ----

func TestMockClientListRelatedIssues(t *testing.T) {
	t.Run("returns value from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
			return []domain.RelatedIssue{{Type: "RELATES"}}, nil
		}
		got, err := mock.ListRelatedIssues(context.Background(), "PROJ-1")
		if err != nil {
			t.Fatalf("ListRelatedIssues() error = %v", err)
		}
		if len(got) != 1 || got[0].Type != "RELATES" {
			t.Errorf("unexpected result: %+v", got)
		}
		if mock.GetCallCount("ListRelatedIssues") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("ListRelatedIssues"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.ListRelatedIssues(context.Background(), "PROJ-1")
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientAddRelatedIssue(t *testing.T) {
	t.Run("calls func with request", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var gotIssueKey string
		var gotReq backlog.AddRelatedIssueRequest
		mock.AddRelatedIssueFunc = func(ctx context.Context, issueKey string, req backlog.AddRelatedIssueRequest) (*domain.RelatedIssue, error) {
			gotIssueKey = issueKey
			gotReq = req
			return &domain.RelatedIssue{Type: "RELATES"}, nil
		}
		got, err := mock.AddRelatedIssue(context.Background(), "PROJ-1", backlog.AddRelatedIssueRequest{TargetIssueID: 42})
		if err != nil {
			t.Fatalf("AddRelatedIssue() error = %v", err)
		}
		if gotIssueKey != "PROJ-1" || gotReq.TargetIssueID != 42 {
			t.Errorf("unexpected call args: issueKey=%q req=%+v", gotIssueKey, gotReq)
		}
		if got.Type != "RELATES" {
			t.Errorf("unexpected result: %+v", got)
		}
		if mock.GetCallCount("AddRelatedIssue") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("AddRelatedIssue"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.AddRelatedIssue(context.Background(), "PROJ-1", backlog.AddRelatedIssueRequest{TargetIssueID: 42})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientDeleteRelatedIssue(t *testing.T) {
	t.Run("calls func with args", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var gotIssueKey string
		var gotID int64
		mock.DeleteRelatedIssueFunc = func(ctx context.Context, issueKey string, relatedIssueID int64) (*domain.RelatedIssue, error) {
			gotIssueKey = issueKey
			gotID = relatedIssueID
			return &domain.RelatedIssue{}, nil
		}
		_, err := mock.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if err != nil {
			t.Fatalf("DeleteRelatedIssue() error = %v", err)
		}
		if gotIssueKey != "PROJ-1" || gotID != 30 {
			t.Errorf("unexpected call args: issueKey=%q id=%d", gotIssueKey, gotID)
		}
		if mock.GetCallCount("DeleteRelatedIssue") != 1 {
			t.Errorf("call count = %d, want 1", mock.GetCallCount("DeleteRelatedIssue"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.DeleteRelatedIssue(context.Background(), "PROJ-1", 30)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}
