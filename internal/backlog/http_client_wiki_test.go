package backlog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ---- ListWikis ----

func TestHTTPClientListWikis_Normal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis" {
			t.Errorf("path = %q, want /api/v2/wikis", r.URL.Path)
		}
		if got := r.URL.Query().Get("projectIdOrKey"); got != "TEST" {
			t.Errorf("projectIdOrKey = %q, want TEST", got)
		}
		pages := []map[string]interface{}{
			{"id": 1, "projectId": 100, "name": "Top", "content": "hello", "tag": []interface{}{}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pages)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	pages, err := client.ListWikis(context.Background(), "TEST", backlog.ListWikisOptions{})
	if err != nil {
		t.Fatalf("ListWikis() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("len(pages) = %d, want 1", len(pages))
	}
	if pages[0].Name != "Top" {
		t.Errorf("Name = %q, want Top", pages[0].Name)
	}
	_ = now
}

func TestHTTPClientListWikis_Keyword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("keyword"); got != "hello" {
			t.Errorf("keyword = %q, want hello", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.ListWikis(context.Background(), "TEST", backlog.ListWikisOptions{Keyword: "hello"})
	if err != nil {
		t.Fatalf("ListWikis() error = %v", err)
	}
}

// ---- CountWikis ----

func TestHTTPClientCountWikis_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/count" {
			t.Errorf("path = %q, want /api/v2/wikis/count", r.URL.Path)
		}
		if got := r.URL.Query().Get("projectIdOrKey"); got != "TEST" {
			t.Errorf("projectIdOrKey = %q, want TEST", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"count": 42})
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	count, err := client.CountWikis(context.Background(), "TEST")
	if err != nil {
		t.Fatalf("CountWikis() error = %v", err)
	}
	if count != 42 {
		t.Errorf("count = %d, want 42", count)
	}
}

// ---- ListWikiTags ----

func TestHTTPClientListWikiTags_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/tags" {
			t.Errorf("path = %q, want /api/v2/wikis/tags", r.URL.Path)
		}
		if got := r.URL.Query().Get("projectIdOrKey"); got != "TEST" {
			t.Errorf("projectIdOrKey = %q, want TEST", got)
		}
		tags := []map[string]interface{}{
			{"id": 1, "name": "golang"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tags)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	tags, err := client.ListWikiTags(context.Background(), "TEST")
	if err != nil {
		t.Fatalf("ListWikiTags() error = %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("len(tags) = %d, want 1", len(tags))
	}
	if tags[0].Name != "golang" {
		t.Errorf("Name = %q, want golang", tags[0].Name)
	}
}

// ---- GetWiki ----

func TestHTTPClientGetWiki_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/10" {
			t.Errorf("path = %q, want /api/v2/wikis/10", r.URL.Path)
		}
		page := map[string]interface{}{
			"id": 10, "projectId": 100, "name": "MyWiki", "content": "body", "tag": []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	page, err := client.GetWiki(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetWiki() error = %v", err)
	}
	if page == nil {
		t.Fatal("GetWiki() returned nil")
	}
	if page.Name != "MyWiki" {
		t.Errorf("Name = %q, want MyWiki", page.Name)
	}
}

// ---- GetWikiHistory ----

func TestHTTPClientGetWikiHistory_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/10/history" {
			t.Errorf("path = %q, want /api/v2/wikis/10/history", r.URL.Path)
		}
		hist := []map[string]interface{}{
			{"pageId": 10, "version": 1, "name": "MyWiki", "content": "v1"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hist)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	hist, err := client.GetWikiHistory(context.Background(), 10, backlog.ListWikiHistoryOptions{})
	if err != nil {
		t.Fatalf("GetWikiHistory() error = %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("len(hist) = %d, want 1", len(hist))
	}
	if hist[0].Name != "MyWiki" {
		t.Errorf("Name = %q, want MyWiki", hist[0].Name)
	}
}

func TestHTTPClientGetWikiHistory_Options(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("minId"); got != "5" {
			t.Errorf("minId = %q, want 5", got)
		}
		if got := q.Get("maxId"); got != "20" {
			t.Errorf("maxId = %q, want 20", got)
		}
		if got := q.Get("count"); got != "10" {
			t.Errorf("count = %q, want 10", got)
		}
		if got := q.Get("order"); got != "asc" {
			t.Errorf("order = %q, want asc", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	_, err := client.GetWikiHistory(context.Background(), 10, backlog.ListWikiHistoryOptions{
		MinID: 5, MaxID: 20, Count: 10, Order: "asc",
	})
	if err != nil {
		t.Fatalf("GetWikiHistory() error = %v", err)
	}
}

// ---- GetWikiStars ----

func TestHTTPClientGetWikiStars_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/10/stars" {
			t.Errorf("path = %q, want /api/v2/wikis/10/stars", r.URL.Path)
		}
		stars := []map[string]interface{}{
			{"id": 1, "url": "https://example.backlog.com/wiki/TEST/Top", "title": "Top", "comment": "nice"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stars)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	stars, err := client.GetWikiStars(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetWikiStars() error = %v", err)
	}
	if len(stars) != 1 {
		t.Fatalf("len(stars) = %d, want 1", len(stars))
	}
	if stars[0].Title != "Top" {
		t.Errorf("Title = %q, want Top", stars[0].Title)
	}
}

// ---- ListWikiAttachments ----

func TestHTTPClientListWikiAttachments_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/10/attachments" {
			t.Errorf("path = %q, want /api/v2/wikis/10/attachments", r.URL.Path)
		}
		att := []domain.Attachment{{ID: 1, Name: "file.txt", Size: 100}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(att)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	att, err := client.ListWikiAttachments(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListWikiAttachments() error = %v", err)
	}
	if len(att) != 1 {
		t.Fatalf("len(att) = %d, want 1", len(att))
	}
	if att[0].Name != "file.txt" {
		t.Errorf("Name = %q, want file.txt", att[0].Name)
	}
}

// ---- ListWikiSharedFiles ----

func TestHTTPClientListWikiSharedFiles_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/wikis/10/sharedFiles" {
			t.Errorf("path = %q, want /api/v2/wikis/10/sharedFiles", r.URL.Path)
		}
		files := []domain.SharedFile{{ID: 1, Name: "report.pdf", Type: "file"}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	client := newOAuthClient(t, srv.URL)
	files, err := client.ListWikiSharedFiles(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListWikiSharedFiles() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if files[0].Name != "report.pdf" {
		t.Errorf("Name = %q, want report.pdf", files[0].Name)
	}
}
