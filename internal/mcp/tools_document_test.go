package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
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

// ===== logvalet_document_search =====

// TestDocumentSearch_Normal は keyword 指定で SearchDocuments が呼ばれることを確認する。
func TestDocumentSearch_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.SearchDocumentsOptions
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		capturedOpt = opt
		return []domain.Document{
			{ID: "doc-1", Title: "OAuth ドキュメント"},
		}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_search", map[string]any{
		"keyword": "OAuth",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Keyword != "OAuth" {
		t.Errorf("Keyword: 期待 %q, 実際 %q", "OAuth", capturedOpt.Keyword)
	}
	if mock.GetCallCount("SearchDocuments") != 1 {
		t.Errorf("expected SearchDocuments called 1 time, got %d", mock.GetCallCount("SearchDocuments"))
	}
}

// TestDocumentSearch_MissingKeyword は keyword 未指定で IsError=true になることを確認する。
func TestDocumentSearch_MissingKeyword(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_search", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestDocumentSearch_WithProjectKeys は project_keys 指定で GetProject + SearchDocuments が呼ばれることを確認する。
func TestDocumentSearch_WithProjectKeys(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		if projectKey != "PROJ" {
			t.Errorf("projectKey = %q, want %q", projectKey, "PROJ")
		}
		return &domain.Project{ID: 100}, nil
	}
	var capturedOpt backlog.SearchDocumentsOptions
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		capturedOpt = opt
		return []domain.Document{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_search", map[string]any{
		"keyword":      "auth",
		"project_keys": "PROJ",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(capturedOpt.ProjectIDs) != 1 || capturedOpt.ProjectIDs[0] != 100 {
		t.Errorf("ProjectIDs: 期待 [100], 実際 %v", capturedOpt.ProjectIDs)
	}
}

// TestDocumentSearch_CountClamped は count > 100 の場合に 100 にクランプされることを確認する。
func TestDocumentSearch_CountClamped(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.SearchDocumentsOptions
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		capturedOpt = opt
		return []domain.Document{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_search", map[string]any{
		"keyword": "test",
		"count":   float64(200),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Count != 100 {
		t.Errorf("Count クランプ: 期待 100, 実際 %d", capturedOpt.Count)
	}
}

// TestDocumentSearch_PossiblyMore_Count50 は count=50 で50件返却時に
// possibly_more=true かつ next_offset=50 になることを確認する（M7 バグ修正検証）。
func TestDocumentSearch_PossiblyMore_Count50(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.SearchDocumentsFunc = func(ctx context.Context, opt backlog.SearchDocumentsOptions) ([]domain.Document, error) {
		docs := make([]domain.Document, 50)
		for i := range docs {
			docs[i] = domain.Document{ID: "doc", Title: "t", ProjectID: 0}
		}
		return docs, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_document_search", map[string]any{
		"keyword": "test",
		"count":   float64(50),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	// DigestEnvelope としてデコードする
	var envelope domain.DigestEnvelope
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}

	// Digest フィールド（interface{}→map[string]any）を DocumentSearchDigest としてデコード
	digestBytes, err := json.Marshal(envelope.Digest)
	if err != nil {
		t.Fatalf("failed to marshal digest: %v", err)
	}
	var d digest.DocumentSearchDigest
	if err := json.Unmarshal(digestBytes, &d); err != nil {
		t.Fatalf("failed to unmarshal DocumentSearchDigest: %v", err)
	}

	if !d.PossiblyMore {
		t.Error("PossiblyMore = false, want true (count=50, 50件返却)")
	}
	if d.NextOffset != 50 {
		t.Errorf("NextOffset = %d, want 50", d.NextOffset)
	}
}
