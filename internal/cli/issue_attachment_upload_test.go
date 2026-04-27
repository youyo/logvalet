package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// TestIssueAttachmentUploadCmd_DryRun は --dry-run フラグで API を呼ばずにプレビュー出力することを確認する。
func TestIssueAttachmentUploadCmd_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(f, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := &IssueAttachmentUploadCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		Files:        []string{f},
	}
	g := &GlobalFlags{}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := cmd.Run(g)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if runErr != nil {
		t.Fatalf("Run() returned error: %v", runErr)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	if out["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", out["dry_run"])
	}
	if out["operation"] != "upload_issue_attachment" {
		t.Errorf("operation = %v, want upload_issue_attachment", out["operation"])
	}
}

// TestIssueAttachmentUploadCmd_Mock は mock Client でアップロード→添付の流れを確認する。
func TestIssueAttachmentUploadCmd_Mock(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(f, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	mock := backlog.NewMockClient()
	var uploadedFilename string
	mock.UploadAttachmentFunc = func(ctx context.Context, filename string, content io.Reader) (*domain.UploadedAttachment, error) {
		uploadedFilename = filename
		return &domain.UploadedAttachment{ID: 999, Name: filename, Size: 7}, nil
	}
	var capturedReq backlog.UpdateIssueRequest
	mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
		capturedReq = req
		return &domain.Issue{IssueKey: issueKey}, nil
	}

	cmd := &IssueAttachmentUploadCmd{
		IssueIDOrKey: "PROJ-1",
		Files:        []string{f},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := runIssueAttachmentUploadWithClient(mock, cmd)

	w.Close()
	os.Stdout = origStdout
	io.Copy(io.Discard, r) //nolint:errcheck

	if runErr != nil {
		t.Fatalf("runIssueAttachmentUploadWithClient() error = %v", runErr)
	}
	if uploadedFilename != "test.txt" {
		t.Errorf("uploaded filename = %q, want test.txt", uploadedFilename)
	}
	if len(capturedReq.AttachmentIDs) != 1 || capturedReq.AttachmentIDs[0] != 999 {
		t.Errorf("AttachmentIDs = %v, want [999]", capturedReq.AttachmentIDs)
	}
}
