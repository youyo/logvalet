package mcp_test

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestIssueAttachmentUpload_Base64_Normal は file_name + file_content_base64 で正常アップロードできることを確認する。
func TestIssueAttachmentUpload_Base64_Normal(t *testing.T) {
	const wantContent = "hello base64 world"
	encoded := base64.StdEncoding.EncodeToString([]byte(wantContent))

	mock := backlog.NewMockClient()
	var uploadedFilename string
	var uploadedBody string
	mock.UploadAttachmentFunc = func(ctx context.Context, filename string, content io.Reader) (*domain.UploadedAttachment, error) {
		uploadedFilename = filename
		b, _ := io.ReadAll(content)
		uploadedBody = string(b)
		return &domain.UploadedAttachment{ID: 99, Name: filename, Size: int64(len(b))}, nil
	}
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{IssueKey: issueKey}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":           "PROJ-1",
		"file_name":           "asset9.png",
		"file_content_base64": encoded,
	})
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if uploadedFilename != "asset9.png" {
		t.Errorf("uploaded filename = %q, want asset9.png", uploadedFilename)
	}
	if uploadedBody != wantContent {
		t.Errorf("uploaded body = %q, want %q", uploadedBody, wantContent)
	}
	if len(capturedReq.AttachmentIDs) != 1 || capturedReq.AttachmentIDs[0] != 99 {
		t.Errorf("AttachmentIDs = %v, want [99]", capturedReq.AttachmentIDs)
	}
}

// TestIssueAttachmentUpload_Base64_MimeTypeAccepted は mime_type を指定してもエラーにならないことを確認する（advisory）。
func TestIssueAttachmentUpload_Base64_MimeTypeAccepted(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	mock := backlog.NewMockClient()
	mock.UploadAttachmentFunc = func(ctx context.Context, filename string, content io.Reader) (*domain.UploadedAttachment, error) {
		return &domain.UploadedAttachment{ID: 1, Name: filename}, nil
	}
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		return &domain.Issue{IssueKey: issueKey}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":           "PROJ-1",
		"file_name":           "asset.png",
		"file_content_base64": encoded,
		"mime_type":           "image/png",
	})
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

// TestIssueAttachmentUpload_Base64_InvalidEncoding は不正な Base64 でエラーになることを確認する。
func TestIssueAttachmentUpload_Base64_InvalidEncoding(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":           "PROJ-1",
		"file_name":           "a.bin",
		"file_content_base64": "!!!not-base64!!!",
	})
	if !result.IsError {
		t.Fatal("expected tool error for invalid base64 but got none")
	}
}

// TestIssueAttachmentUpload_Base64_MissingFileName は file_content_base64 ありで file_name なしでエラーになることを確認する。
func TestIssueAttachmentUpload_Base64_MissingFileName(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("x"))
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":           "PROJ-1",
		"file_content_base64": encoded,
	})
	if !result.IsError {
		t.Fatal("expected tool error for missing file_name but got none")
	}
}

// TestIssueAttachmentUpload_Base64_OversizeRejected はデコード後 4MB 超でエラーになることを確認する。
func TestIssueAttachmentUpload_Base64_OversizeRejected(t *testing.T) {
	// 4MB + 1 byte
	big := strings.Repeat("a", 4*1024*1024+1)
	encoded := base64.StdEncoding.EncodeToString([]byte(big))
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":           "PROJ-1",
		"file_name":           "big.bin",
		"file_content_base64": encoded,
	})
	if !result.IsError {
		t.Fatal("expected tool error for oversize content but got none")
	}
	if mock.GetCallCount("UploadAttachment") != 0 {
		t.Errorf("UploadAttachment should not be called on size rejection, got %d", mock.GetCallCount("UploadAttachment"))
	}
}

// TestIssueAttachmentUpload_BothModesSpecified は file_paths と file_content_base64 を両方指定するとエラーになることを確認する。
func TestIssueAttachmentUpload_BothModesSpecified(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(f1, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("y"))

	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_issue_attachment_upload", map[string]any{
		"issue_key":           "PROJ-1",
		"file_paths":          f1,
		"file_name":           "a.txt",
		"file_content_base64": encoded,
	})
	if !result.IsError {
		t.Fatal("expected tool error when both modes specified but got none")
	}
}
