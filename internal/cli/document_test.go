package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentCreateCmd_run_dry_run(t *testing.T) {
	cmd := &DocumentCreateCmd{
		WriteFlags: WriteFlags{DryRun: true},
		ProjectKey: "PROJ",
		Title:      "テストドキュメント",
		Content:    "本文テキスト",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestDocumentCreateCmd_run_content_conflict(t *testing.T) {
	cmd := &DocumentCreateCmd{
		WriteFlags:  WriteFlags{DryRun: true},
		ProjectKey:  "PROJ",
		Title:       "テスト",
		Content:     "some content",
		ContentFile: "somefile.txt",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when both --content and --content-file are specified")
	}
}

func TestDocumentCreateCmd_run_content_required(t *testing.T) {
	cmd := &DocumentCreateCmd{
		WriteFlags: WriteFlags{DryRun: true},
		ProjectKey: "PROJ",
		Title:      "テスト",
		// Content も ContentFile も未指定
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when neither --content nor --content-file is specified")
	}
}

func TestDocumentCreateCmd_run_content_file(t *testing.T) {
	tmpDir := t.TempDir()
	contentPath := filepath.Join(tmpDir, "content.txt")
	if err := os.WriteFile(contentPath, []byte("ファイルの本文"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := &DocumentCreateCmd{
		WriteFlags:  WriteFlags{DryRun: true},
		ProjectKey:  "PROJ",
		Title:       "テスト",
		ContentFile: contentPath,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestDocumentCreateCmd_run_not_dry_run(t *testing.T) {
	cmd := &DocumentCreateCmd{
		WriteFlags: WriteFlags{DryRun: false},
		ProjectKey: "PROJ",
		Title:      "テスト",
		Content:    "本文",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	// dry-run でない場合は ErrNotImplemented が返る
	if err == nil {
		t.Error("Run() should return error (ErrNotImplemented) when not dry-run")
	}
}
