package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// CLI-IA-1: issue attachment delete --dry-run (API 呼び出しなし + dry-run 出力確認)
func TestIssueAttachmentDeleteCmd_dry_run(t *testing.T) {
	cmd := &IssueAttachmentDeleteCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		AttachmentID: 99,
	}
	g := &GlobalFlags{}

	// stdout をキャプチャ
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

	// JSON に dry_run:true が含まれる
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, buf.String())
	}
	if out["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", out["dry_run"])
	}
	if out["operation"] != "delete_issue_attachment" {
		t.Errorf("operation = %v, want delete_issue_attachment", out["operation"])
	}
}

// IssueAttachmentDeleteCmd: DryRun=false は buildRunContext を呼んでエラーになる
func TestIssueAttachmentDeleteCmd_not_dry_run(t *testing.T) {
	cmd := &IssueAttachmentDeleteCmd{
		WriteFlags:   WriteFlags{DryRun: false},
		IssueIDOrKey: "PROJ-1",
		AttachmentID: 99,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when not dry-run (config not available)")
	}
}

// IssueAttachmentDownloadCmd: download ヘルパーが正しく動作することを確認（mock 経由）
func TestIssueAttachmentDownloadCmd_helper(t *testing.T) {
	content := "attachment content"
	body := io.NopCloser(strings.NewReader(content))

	tmpDir := t.TempDir()
	destPath := tmpDir + "/att.txt"
	got, err := downloadToFile(body, "att.txt", destPath)
	if err != nil {
		t.Fatalf("downloadToFile: %v", err)
	}
	if got != destPath {
		t.Errorf("returned path = %q, want %q", got, destPath)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}
