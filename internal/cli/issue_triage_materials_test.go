package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: "issue triage-materials PROJ-123" のパースと必須引数確認
func TestIssueTriageMaterialsCmd_KongParse(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "triage-materials", "PROJ-123"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.TriageMaterials
	if cmd.IssueKey != "PROJ-123" {
		t.Errorf("IssueKey: 期待 %q, 実際 %q", "PROJ-123", cmd.IssueKey)
	}
	if !cmd.IncludeRelatedIssues {
		t.Error("IncludeRelatedIssues デフォルト: 期待 true, 実際 false")
	}
}

// T3: --no-include-related-issues でフラグが false になる
func TestIssueTriageMaterialsCmd_KongParse_NoIncludeRelatedIssues(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "triage-materials", "PROJ-123", "--no-include-related-issues"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.TriageMaterials
	if cmd.IncludeRelatedIssues {
		t.Error("IncludeRelatedIssues: 期待 false (--no-include-related-issues), 実際 true")
	}
}

// T4: --include-related-issues で明示的に true になる
func TestIssueTriageMaterialsCmd_KongParse_IncludeRelatedIssuesExplicit(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "triage-materials", "PROJ-123", "--include-related-issues"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.TriageMaterials
	if !cmd.IncludeRelatedIssues {
		t.Error("IncludeRelatedIssues: 期待 true (--include-related-issues), 実際 false")
	}
}

// T5: 引数なしでエラー
func TestIssueTriageMaterialsCmd_KongParse_MissingArg(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "triage-materials"})
	if err == nil {
		t.Error("必須引数なしでエラーが返されなかった")
	}
}
