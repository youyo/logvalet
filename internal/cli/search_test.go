package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

func TestSearchCmd_ParseKeywordOnly(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() error = %v", err)
	}
	_, err = p.Parse([]string{"search", "OAuth"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if root.Search.Keyword != "OAuth" {
		t.Errorf("Keyword = %q, want OAuth", root.Search.Keyword)
	}
	if root.Search.Count != 20 {
		t.Errorf("Count = %d, want 20", root.Search.Count)
	}
	if root.Search.Detail != "snippet" {
		t.Errorf("Detail = %q, want snippet", root.Search.Detail)
	}
}

func TestSearchCmd_ParseWithProjectAndCount(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() error = %v", err)
	}
	_, err = p.Parse([]string{"search", "login", "--project", "PROJ", "--project", "OPS", "--count", "50", "--detail", "meta"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(root.Search.ProjectKeys) != 2 || root.Search.ProjectKeys[0] != "PROJ" || root.Search.ProjectKeys[1] != "OPS" {
		t.Errorf("ProjectKeys = %v, want [PROJ OPS]", root.Search.ProjectKeys)
	}
	if root.Search.Count != 50 {
		t.Errorf("Count = %d, want 50", root.Search.Count)
	}
	if root.Search.Detail != "meta" {
		t.Errorf("Detail = %q, want meta", root.Search.Detail)
	}
}
