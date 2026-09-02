package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

func newIssueTestParser(t *testing.T, root *cli.CLI) *kong.Kong {
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

func TestIssueCreateCmd_KongParseEngagement(t *testing.T) {
	var root cli.CLI
	p := newIssueTestParser(t, &root)

	_, err := p.Parse([]string{
		"issue", "create", "--project-key", "PROJ", "--summary", "S",
		"--engagement", "顧客A基盤更改",
	})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}
	if got := root.Issue.Create.Engagement; got != "顧客A基盤更改" {
		t.Errorf("Engagement: 期待 %q, 実際 %q", "顧客A基盤更改", got)
	}
}

func TestIssueUpdateCmd_KongParseEngagement(t *testing.T) {
	var root cli.CLI
	p := newIssueTestParser(t, &root)

	_, err := p.Parse([]string{
		"issue", "update", "PROJ-1", "--engagement", "顧客A基盤更改",
	})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}
	if root.Issue.Update.Engagement == nil || *root.Issue.Update.Engagement != "顧客A基盤更改" {
		t.Errorf("Engagement: 期待 %q, 実際 %v", "顧客A基盤更改", root.Issue.Update.Engagement)
	}
}

func TestIssueEngagement_KongParseDefault(t *testing.T) {
	var root cli.CLI
	p := newIssueTestParser(t, &root)

	_, err := p.Parse([]string{"issue", "create", "--project-key", "PROJ", "--summary", "S"})
	if err != nil {
		t.Fatalf("create のパースエラー: %v", err)
	}
	if root.Issue.Create.Engagement != "" {
		t.Errorf("create Engagement デフォルト: 期待空文字, 実際 %q", root.Issue.Create.Engagement)
	}

	root = cli.CLI{}
	p = newIssueTestParser(t, &root)
	_, err = p.Parse([]string{"issue", "update", "PROJ-1", "--summary", "S"})
	if err != nil {
		t.Fatalf("update のパースエラー: %v", err)
	}
	if root.Issue.Update.Engagement != nil {
		t.Errorf("update Engagement デフォルト: 期待 nil, 実際 %v", root.Issue.Update.Engagement)
	}
}
