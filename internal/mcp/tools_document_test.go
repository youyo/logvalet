package mcp_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)


// ===== C1/C3: logvalet_document_list =====

// TestDocumentList_ProjectKey_Normal は project_key="PROJ" で GetProject + ListDocuments が呼ばれることを確認する。
func TestDocumentList_ProjectKey_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		if projectKey != "PROJ" {
			t.Errorf("projectKey = %q, want %q", projectKey, "PROJ")
		}
		return &domain.Project{ID: 100}, nil
	}
	var capturedProjectID int
	mock.ListDocumentsFunc = func(ctx context.Context, projectID int, opt backlog.ListDocumentsOptions) ([]domain.Document, error) {
		capturedProjectID = projectID
		return []domain.Document{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_list", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedProjectID != 100 {
		t.Errorf("expected projectID=100, got %d", capturedProjectID)
	}
}

// TestDocumentList_MissingProjectKey は project_key 未指定で IsError=true になることを確認する。
func TestDocumentList_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_list", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestDocumentList_Count_Normal は count=5 で ListDocumentsOptions.Limit が 5 になることを確認する。
func TestDocumentList_Count_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100}, nil
	}
	var capturedOpt backlog.ListDocumentsOptions
	mock.ListDocumentsFunc = func(ctx context.Context, projectID int, opt backlog.ListDocumentsOptions) ([]domain.Document, error) {
		capturedOpt = opt
		return []domain.Document{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_list", map[string]any{
		"project_key": "PROJ",
		"count":       float64(5),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Limit != 5 {
		t.Errorf("expected Limit=5, got %d", capturedOpt.Limit)
	}
}

// ===== B5: logvalet_document_tree =====

// TestDocumentTree_Normal は project_key 指定で GetDocumentTree が呼ばれることを確認する。
func TestDocumentTree_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedProjectKey string
	mock.GetDocumentTreeFunc = func(ctx context.Context, projectKey string) (*domain.DocumentTree, error) {
		capturedProjectKey = projectKey
		return &domain.DocumentTree{ProjectID: 100}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_tree", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedProjectKey != "PROJ" {
		t.Errorf("projectKey = %q, want %q", capturedProjectKey, "PROJ")
	}
	if mock.GetCallCount("GetDocumentTree") != 1 {
		t.Errorf("expected GetDocumentTree called 1 time, got %d", mock.GetCallCount("GetDocumentTree"))
	}
}

// TestDocumentTree_MissingProjectKey は project_key 未指定で IsError=true になることを確認する。
func TestDocumentTree_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_tree", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== B6: logvalet_document_digest =====

// TestDocumentDigest_Normal は document_id 指定で Build が呼ばれることを確認する。
func TestDocumentDigest_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	// document_digest は digest.DefaultDocumentDigestBuilder を使うが、
	// テストでは GetDocument を使った mock を通じて動作確認する。
	mock.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
		return &domain.Document{
			ID:        "doc-123",
			ProjectID: 100,
			Title:     "テストドキュメント",
		}, nil
	}
	mock.ListDocumentAttachmentsFunc = func(ctx context.Context, documentID string) ([]domain.Attachment, error) {
		return []domain.Attachment{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_digest", map[string]any{"document_id": "doc-123"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetDocument") != 1 {
		t.Errorf("expected GetDocument called 1 time, got %d", mock.GetCallCount("GetDocument"))
	}
}

// TestDocumentDigest_MissingDocumentID は document_id 未指定で IsError=true になることを確認する。
func TestDocumentDigest_MissingDocumentID(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_digest", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}
