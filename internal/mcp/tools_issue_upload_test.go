package mcp_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// TestIssueAttachmentUpload_Normal はファイルをアップロードして課題に添付することを確認する。
func TestIssueAttachmentUpload_Normal(t *testing.T) {
	// 一時ファイルを作成
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "upload.txt")
	if err := os.WriteFile(f1, []byte("test content"), 0600); err != nil {
		t.Fatal(err)
	}

	mock := backlog.NewMockClient()
	var uploadedFilename string
	mock.UploadAttachmentFunc = func(ctx context.Context, filename string, content io.Reader) (*domain.UploadedAttachment, error) {
		uploadedFilename = filename
		return &domain.UploadedAttachment{ID: 42, Name: filename, Size: 12}, nil
	}
	var capturedIssueKey string
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedIssueKey = issueKey
		capturedReq = req
		return &domain.Issue{IssueKey: issueKey}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":  "PROJ-1",
		"file_paths": f1,
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if uploadedFilename != "upload.txt" {
		t.Errorf("uploaded filename = %q, want upload.txt", uploadedFilename)
	}
	if capturedIssueKey != "PROJ-1" {
		t.Errorf("issue key = %q, want PROJ-1", capturedIssueKey)
	}
	if len(capturedReq.AttachmentIDs) != 1 || capturedReq.AttachmentIDs[0] != 42 {
		t.Errorf("AttachmentIDs = %v, want [42]", capturedReq.AttachmentIDs)
	}
	if mock.GetCallCount("UploadAttachment") != 1 {
		t.Errorf("UploadAttachment called %d times, want 1", mock.GetCallCount("UploadAttachment"))
	}
	if mock.GetCallCount("UpdateIssue") != 1 {
		t.Errorf("UpdateIssue called %d times, want 1", mock.GetCallCount("UpdateIssue"))
	}
}

// TestIssueAttachmentUpload_MissingIssueKey は issue_key 未指定でエラーになることを確認する。
func TestIssueAttachmentUpload_MissingIssueKey(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"file_paths": "/tmp/test.txt",
	})
	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestIssueAttachmentUpload_MissingFilePaths は file_paths 未指定でエラーになることを確認する。
func TestIssueAttachmentUpload_MissingFilePaths(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key": "PROJ-1",
	})
	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// TestIssueAttachmentUpload_FileNotFound は存在しないファイルパスでエラーになることを確認する。
func TestIssueAttachmentUpload_FileNotFound(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":  "PROJ-1",
		"file_paths": "/nonexistent/path/file.txt",
	})
	if !result.IsError {
		t.Fatal("expected tool error for nonexistent file but got none")
	}
}
