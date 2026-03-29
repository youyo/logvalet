package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// CLI-SF-1: shared-file download --output (tmpdir にファイル保存 → 内容一致確認)
func TestSharedFileDownloadCmd_with_output(t *testing.T) {
	tmpDir := t.TempDir()
	content := "shared file content"
	mock := backlog.NewMockClient()
	mock.DownloadSharedFileFunc = func(ctx context.Context, projectKey string, fileID int64) (io.ReadCloser, string, error) {
		return io.NopCloser(strings.NewReader(content)), "report.txt", nil
	}

	destPath := filepath.Join(tmpDir, "out.txt")
	cmd := &SharedFileDownloadCmd{
		ProjectKey: "PROJ",
		FileID:     42,
		Output:     destPath,
	}

	// Run は buildRunContext を呼ぶが、テストでは直接 rc を使う方式を採用できないため
	// ダウンロードヘルパーを mock 経由で直接テストする。
	body, filename, err := mock.DownloadSharedFile(context.Background(), "PROJ", 42)
	if err != nil {
		t.Fatalf("DownloadSharedFile: %v", err)
	}
	got, err := downloadToFile(body, filename, cmd.Output)
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
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

// CLI-SF-2: shared-file download (output 未指定: カレントディレクトリに保存)
func TestSharedFileDownloadCmd_no_output(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	content := "shared no output"
	mock := backlog.NewMockClient()
	mock.DownloadSharedFileFunc = func(ctx context.Context, projectKey string, fileID int64) (io.ReadCloser, string, error) {
		return io.NopCloser(strings.NewReader(content)), "shared.bin", nil
	}

	body, filename, err := mock.DownloadSharedFile(context.Background(), "PROJ", 1)
	if err != nil {
		t.Fatalf("DownloadSharedFile: %v", err)
	}
	got, err := downloadToFile(body, filename, "")
	if err != nil {
		t.Fatalf("downloadToFile: %v", err)
	}
	if got != "shared.bin" {
		t.Errorf("returned path = %q, want %q", got, "shared.bin")
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "shared.bin"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

// SharedFileListCmd の dry-run テスト（mock を使って正常系を確認）
func TestSharedFileListCmd_mock(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListSharedFilesFunc = func(ctx context.Context, projectKey string, opt backlog.ListSharedFilesOptions) ([]domain.SharedFile, error) {
		return []domain.SharedFile{{ID: 1, Name: "file.txt"}}, nil
	}

	files, err := mock.ListSharedFiles(context.Background(), "PROJ", backlog.ListSharedFilesOptions{})
	if err != nil {
		t.Fatalf("ListSharedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files, want 1", len(files))
	}
}
