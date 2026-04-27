package mcp_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== logvalet_wiki_list =====

func TestWikiList_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedKey string
	mock.ListWikisFunc = func(ctx context.Context, projectKey string, opt backlog.ListWikisOptions) ([]domain.WikiPage, error) {
		capturedKey = projectKey
		return []domain.WikiPage{{ID: 1, Name: "Top"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_list", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedKey != "PROJ" {
		t.Errorf("projectKey = %q, want PROJ", capturedKey)
	}
	if mock.GetCallCount("ListWikis") != 1 {
		t.Errorf("ListWikis called %d times, want 1", mock.GetCallCount("ListWikis"))
	}
}

func TestWikiList_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_list", map[string]any{})
	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

func TestWikiList_WithKeyword(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListWikisOptions
	mock.ListWikisFunc = func(ctx context.Context, projectKey string, opt backlog.ListWikisOptions) ([]domain.WikiPage, error) {
		capturedOpt = opt
		return []domain.WikiPage{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_list", map[string]any{
		"project_key": "PROJ",
		"keyword":     "hello",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Keyword != "hello" {
		t.Errorf("Keyword = %q, want hello", capturedOpt.Keyword)
	}
}

// ===== logvalet_wiki_get =====

func TestWikiGet_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedID int64
	mock.GetWikiFunc = func(ctx context.Context, wikiID int64) (*domain.WikiPage, error) {
		capturedID = wikiID
		return &domain.WikiPage{ID: wikiID, Name: "Top"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_get", map[string]any{"wiki_id": float64(42)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedID != 42 {
		t.Errorf("wikiID = %d, want 42", capturedID)
	}
}

func TestWikiGet_MissingID(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_get", map[string]any{})
	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== logvalet_wiki_count =====

func TestWikiCount_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.CountWikisFunc = func(ctx context.Context, projectKey string) (int, error) {
		return 5, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_count", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("CountWikis") != 1 {
		t.Errorf("CountWikis called %d times, want 1", mock.GetCallCount("CountWikis"))
	}
}

func TestWikiCount_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_count", map[string]any{})
	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== logvalet_wiki_tags =====

func TestWikiTags_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListWikiTagsFunc = func(ctx context.Context, projectKey string) ([]domain.WikiTag, error) {
		return []domain.WikiTag{{ID: 1, Name: "go"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_tags", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

// ===== logvalet_wiki_history =====

func TestWikiHistory_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedID int64
	mock.GetWikiHistoryFunc = func(ctx context.Context, wikiID int64, opt backlog.ListWikiHistoryOptions) ([]domain.WikiHistory, error) {
		capturedID = wikiID
		return []domain.WikiHistory{{PageID: wikiID, Version: 1}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_history", map[string]any{"wiki_id": float64(10)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedID != 10 {
		t.Errorf("wikiID = %d, want 10", capturedID)
	}
}

// ===== logvalet_wiki_stars =====

func TestWikiStars_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetWikiStarsFunc = func(ctx context.Context, wikiID int64) ([]domain.WikiStar, error) {
		return []domain.WikiStar{{ID: 1, Title: "Top"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_stars", map[string]any{"wiki_id": float64(10)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

// ===== logvalet_wiki_attachment_list =====

func TestWikiAttachmentList_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListWikiAttachmentsFunc = func(ctx context.Context, wikiID int64) ([]domain.Attachment, error) {
		return []domain.Attachment{{ID: 1, Name: "file.txt"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_attachment_list", map[string]any{"wiki_id": float64(10)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

// ===== logvalet_wiki_sharedfile_list =====

func TestWikiSharedFileList_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListWikiSharedFilesFunc = func(ctx context.Context, wikiID int64) ([]domain.SharedFile, error) {
		return []domain.SharedFile{{ID: 1, Name: "report.pdf"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_wiki_sharedfile_list", map[string]any{"wiki_id": float64(10)})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}
