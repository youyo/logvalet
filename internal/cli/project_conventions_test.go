package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

func newProjectConventionsParser(t *testing.T, root *cli.CLI) *kong.Kong {
	t.Helper()

	p, err := kong.New(root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	return p
}

func TestProjectConventionsInit_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{"project", "conventions", "init"}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Conventions.Init
	if cmd.Out != "" {
		t.Errorf("Out デフォルト: 期待 \"\", 実際 %q", cmd.Out)
	}
	if cmd.FromProject != "" {
		t.Errorf("FromProject デフォルト: 期待 \"\", 実際 %q", cmd.FromProject)
	}
}

func TestProjectConventionsInit_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{
		"project", "conventions", "init", "--out", "conventions.yaml", "--from-project", "PROJ",
	}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Conventions.Init
	wantPath, err := filepath.Abs("conventions.yaml")
	if err != nil {
		t.Fatalf("filepath.Abs() エラー: %v", err)
	}
	if cmd.Out != wantPath {
		t.Errorf("Out: 期待 %q, 実際 %q", wantPath, cmd.Out)
	}
	if cmd.FromProject != "PROJ" {
		t.Errorf("FromProject: 期待 %q, 実際 %q", "PROJ", cmd.FromProject)
	}
}

func TestProjectConventionsValidate_KongParse_WithStrict(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{
		"project", "conventions", "validate", "--file", "conventions.yaml", "--strict",
	}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Conventions.Validate
	wantPath, err := filepath.Abs("conventions.yaml")
	if err != nil {
		t.Fatalf("filepath.Abs() エラー: %v", err)
	}
	if cmd.File != wantPath {
		t.Errorf("File: 期待 %q, 実際 %q", wantPath, cmd.File)
	}
	if !cmd.Strict {
		t.Error("Strict: 期待 true, 実際 false")
	}
}

func TestProjectConventionsValidate_KongParse_MissingFile(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{"project", "conventions", "validate"}); err == nil {
		t.Error("--file なしでエラーが返されなかった")
	}
}

func TestProjectConventionsValidate_KongParse_GlobalFormatAndFile(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{
		"-f", "yaml", "project", "conventions", "validate", "--file", "conventions.yaml",
	}); err != nil {
		t.Fatalf("グローバル --format と --file の併用でパースエラー: %v", err)
	}

	if root.Format != "yaml" {
		t.Errorf("Format: 期待 %q, 実際 %q", "yaml", root.Format)
	}
	wantPath, err := filepath.Abs("conventions.yaml")
	if err != nil {
		t.Fatalf("filepath.Abs() エラー: %v", err)
	}
	if root.Project.Conventions.Validate.File != wantPath {
		t.Errorf("File: 期待 %q, 実際 %q", wantPath, root.Project.Conventions.Validate.File)
	}
}

func TestProjectConventionsShow_KongParse_ProjectRequired(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{"project", "conventions", "show", "--project", "PROJ"}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Conventions.Show
	if cmd.Project != "PROJ" {
		t.Errorf("Project: 期待 %q, 実際 %q", "PROJ", cmd.Project)
	}
	if cmd.File != "" {
		t.Errorf("File デフォルト: 期待 \"\", 実際 %q", cmd.File)
	}
}

func TestProjectConventionsShow_KongParse_WithFile(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{
		"project", "conventions", "show", "--project", "PROJ", "--file", "conventions.yaml",
	}); err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	wantPath, err := filepath.Abs("conventions.yaml")
	if err != nil {
		t.Fatalf("filepath.Abs() エラー: %v", err)
	}
	cmd := root.Project.Conventions.Show
	if cmd.Project != "PROJ" || cmd.File != wantPath {
		t.Errorf("Show = (%q, %q), want (%q, %q)", cmd.Project, cmd.File, "PROJ", wantPath)
	}
}

func TestProjectConventionsShow_KongParse_MissingProject(t *testing.T) {
	var root cli.CLI
	p := newProjectConventionsParser(t, &root)

	if _, err := p.Parse([]string{"project", "conventions", "show"}); err == nil {
		t.Error("--project なしでエラーが返されなかった")
	}
}
