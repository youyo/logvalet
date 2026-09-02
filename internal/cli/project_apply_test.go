package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

func TestProjectApply_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{"project", "apply", "--file", "path"}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Apply
	wantPath, err := filepath.Abs("path")
	if err != nil {
		t.Fatalf("filepath.Abs() エラー: %v", err)
	}
	if cmd.File != wantPath {
		t.Errorf("File: 期待 %q, 実際 %q", wantPath, cmd.File)
	}
	if cmd.DryRun {
		t.Error("DryRun デフォルト: 期待 false, 実際 true")
	}
	if cmd.Create {
		t.Error("Create デフォルト: 期待 false, 実際 true")
	}
	if cmd.Project != "" {
		t.Errorf("Project デフォルト: 期待 \"\", 実際 %q", cmd.Project)
	}
}

func TestProjectApply_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{
		"project", "apply", "--file", "path", "--dry-run", "--create", "--project", "OTHER",
	}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Apply
	if !cmd.DryRun {
		t.Error("DryRun: 期待 true, 実際 false")
	}
	if !cmd.Create {
		t.Error("Create: 期待 true, 実際 false")
	}
	if cmd.Project != "OTHER" {
		t.Errorf("Project: 期待 %q, 実際 %q", "OTHER", cmd.Project)
	}
}

func TestProjectApply_KongParse_MissingFile(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{"project", "apply"}); err == nil {
		t.Error("--file なしでエラーが返されなかった")
	}
}

func TestProjectApply_KongParse_GlobalFormatAndFile(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{
		"-f", "yaml", "project", "apply", "--file", "path",
	}); err != nil {
		t.Fatalf("グローバル --format と --file の併用でパースエラー: %v", err)
	}

	if root.Format != "yaml" {
		t.Errorf("Format: 期待 %q, 実際 %q", "yaml", root.Format)
	}
}
