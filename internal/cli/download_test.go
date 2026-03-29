package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI-DL-1: 正常書き込み
func TestDownloadToFile_normal(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello download"
	body := io.NopCloser(strings.NewReader(content))

	destPath := filepath.Join(tmpDir, "out.txt")
	got, err := downloadToFile(body, "original.txt", destPath)
	if err != nil {
		t.Fatalf("downloadToFile returned error: %v", err)
	}
	if got != destPath {
		t.Errorf("returned path = %q, want %q", got, destPath)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

// CLI-DL-2: パストラバーサル防止
// filename に "../evil.txt" が含まれていても filepath.Base で安全なファイル名に変換される。
func TestDownloadToFile_path_traversal_prevention(t *testing.T) {
	tmpDir := t.TempDir()
	content := "safe"
	body := io.NopCloser(strings.NewReader(content))

	// output を tmpDir 内に指定し、filename のトラバーサルが無効化されることを確認
	destPath := filepath.Join(tmpDir, "safe.txt")
	got, err := downloadToFile(body, "../../../etc/passwd", destPath)
	if err != nil {
		t.Fatalf("downloadToFile returned error: %v", err)
	}
	// 指定した output PATH に書き込まれるはずなので got == destPath
	if got != destPath {
		t.Errorf("returned path = %q, want %q", got, destPath)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

// output 未指定時: safeName（filename の Base）が戻り値になる
func TestDownloadToFile_no_output(t *testing.T) {
	// カレントディレクトリに書き込むため tmpDir に chdir する
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	content := "no output path"
	body := io.NopCloser(strings.NewReader(content))
	got, err := downloadToFile(body, "report.csv", "")
	if err != nil {
		t.Fatalf("downloadToFile returned error: %v", err)
	}
	if got != "report.csv" {
		t.Errorf("returned path = %q, want %q", got, "report.csv")
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "report.csv"))
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

// output の親ディレクトリが存在しない場合は MkdirAll で作成される
func TestDownloadToFile_creates_parent_dir(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "sub", "dir", "file.txt")
	body := io.NopCloser(strings.NewReader("content"))

	got, err := downloadToFile(body, "file.txt", destPath)
	if err != nil {
		t.Fatalf("downloadToFile returned error: %v", err)
	}
	if got != destPath {
		t.Errorf("returned path = %q, want %q", got, destPath)
	}
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("destination file does not exist")
	}
}
