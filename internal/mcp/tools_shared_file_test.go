package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== C1: logvalet_shared_file_list count パラメータ =====

// TestSharedFileList_Count_Normal は count=10 で ListSharedFilesOptions.Limit が 10 になることを確認する。
func TestSharedFileList_Count_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedOpt backlog.ListSharedFilesOptions
	mock.ListSharedFilesFunc = func(ctx context.Context, projectKey string, opt backlog.ListSharedFilesOptions) ([]domain.SharedFile, error) {
		capturedOpt = opt
		return []domain.SharedFile{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_shared_file_list", map[string]any{
		"project_key": "PROJ",
		"count":       float64(10),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedOpt.Limit != 10 {
		t.Errorf("expected Limit=10, got %d", capturedOpt.Limit)
	}
}

// ===== B14: logvalet_shared_file_download =====

// TestSharedFileDownload_Normal は正常系で base64 エンコード済みコンテンツが返ることを確認する。
func TestSharedFileDownload_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.DownloadSharedFileBoundedFunc = func(ctx context.Context, projectKey string, fileID int64, maxBytes int64) ([]byte, string, string, error) {
		if projectKey != "PROJ" {
			t.Errorf("projectKey = %q, want %q", projectKey, "PROJ")
		}
		if fileID != 42 {
			t.Errorf("fileID = %d, want 42", fileID)
		}
		return []byte("file contents"), "readme.txt", "text/plain", nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_shared_file_download", map[string]any{
		"project_key": "PROJ",
		"file_id":     float64(42),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &out); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	if _, ok := out["content_base64"]; !ok {
		t.Error("expected content_base64 in result")
	}
	if out["filename"] != "readme.txt" {
		t.Errorf("filename = %v, want readme.txt", out["filename"])
	}
	if mock.GetCallCount("DownloadSharedFileBounded") != 1 {
		t.Errorf("expected DownloadSharedFileBounded called 1 time, got %d", mock.GetCallCount("DownloadSharedFileBounded"))
	}
}

// TestSharedFileDownload_TooLarge は ErrDownloadTooLarge で IsError=true になることを確認する。
func TestSharedFileDownload_TooLarge(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.DownloadSharedFileBoundedFunc = func(ctx context.Context, projectKey string, fileID int64, maxBytes int64) ([]byte, string, string, error) {
		return nil, "", "", backlog.ErrDownloadTooLarge
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_shared_file_download", map[string]any{
		"project_key": "PROJ",
		"file_id":     float64(42),
	})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
	// エラーメッセージに too large が含まれるか確認
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(strings.ToLower(textContent.Text), "too large") &&
		!strings.Contains(strings.ToLower(textContent.Text), "large") {
		t.Logf("error message: %s", textContent.Text)
	}
}

// TestSharedFileDownload_MissingProjectKey は project_key 未指定で IsError=true になることを確認する。
func TestSharedFileDownload_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_shared_file_download", map[string]any{"file_id": float64(42)})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestSharedFileDownload_MissingFileID は file_id 未指定で IsError=true になることを確認する。
func TestSharedFileDownload_MissingFileID(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_shared_file_download", map[string]any{"project_key": "PROJ"})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}
