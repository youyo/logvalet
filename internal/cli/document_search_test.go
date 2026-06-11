package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: keyword のみ → デフォルト値確認
func TestDocumentSearchCmd_Parse_KeywordOnly(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"document", "search", "OAuth"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Document.Search
	if cmd.Keyword != "OAuth" {
		t.Errorf("Keyword: 期待 %q, 実際 %q", "OAuth", cmd.Keyword)
	}
	if cmd.Count != 100 {
		t.Errorf("Count デフォルト: 期待 100, 実際 %d", cmd.Count)
	}
	if cmd.Detail != "snippet" {
		t.Errorf("Detail デフォルト: 期待 %q, 実際 %q", "snippet", cmd.Detail)
	}
	if len(cmd.ProjectKeys) != 0 {
		t.Errorf("ProjectKeys デフォルト: 期待 [], 実際 %v", cmd.ProjectKeys)
	}
}

// T2: フラグ付き → -p PROJ --detail meta --count 50
func TestDocumentSearchCmd_Parse_WithFlags(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"document", "search", "login", "--project-keys", "PROJ", "--detail", "meta", "--count", "50"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Document.Search
	if cmd.Keyword != "login" {
		t.Errorf("Keyword: 期待 %q, 実際 %q", "login", cmd.Keyword)
	}
	if len(cmd.ProjectKeys) != 1 || cmd.ProjectKeys[0] != "PROJ" {
		t.Errorf("ProjectKeys: 期待 [PROJ], 実際 %v", cmd.ProjectKeys)
	}
	if cmd.Detail != "meta" {
		t.Errorf("Detail: 期待 %q, 実際 %q", "meta", cmd.Detail)
	}
	if cmd.Count != 50 {
		t.Errorf("Count: 期待 50, 実際 %d", cmd.Count)
	}
}

// T3: 複数プロジェクト → -p A -p B
func TestDocumentSearchCmd_Parse_MultipleProjects(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"document", "search", "auth", "--project-keys", "A", "--project-keys", "B"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Document.Search
	if len(cmd.ProjectKeys) != 2 {
		t.Fatalf("ProjectKeys 長さ: 期待 2, 実際 %d", len(cmd.ProjectKeys))
	}
	if cmd.ProjectKeys[0] != "A" || cmd.ProjectKeys[1] != "B" {
		t.Errorf("ProjectKeys: 期待 [A B], 実際 %v", cmd.ProjectKeys)
	}
}

// T4: keyword なし → エラー
func TestDocumentSearchCmd_Parse_MissingKeyword(t *testing.T) {
	var root cli.CLI
	errBuf := bytes.NewBuffer(nil)
	exitCalled := false
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), errBuf),
		kong.Exit(func(int) { exitCalled = true }),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"document", "search"})
	if err == nil && !exitCalled {
		t.Error("keyword 未指定でエラーが返らなかった")
	}
}

// T5: --count 指定
func TestDocumentSearchCmd_Parse_Count(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"document", "search", "test", "--count", "50"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Document.Search
	if cmd.Count != 50 {
		t.Errorf("Count: 期待 50, 実際 %d", cmd.Count)
	}
}
