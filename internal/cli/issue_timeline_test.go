package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: "issue timeline PROJ-123" のパースとデフォルト値
func TestIssueTimelineCmd_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "timeline", "PROJ-123"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Timeline
	if cmd.IssueKey != "PROJ-123" {
		t.Errorf("IssueKey: 期待 %q, 実際 %q", "PROJ-123", cmd.IssueKey)
	}
	if cmd.MaxComments != 0 {
		t.Errorf("MaxComments デフォルト: 期待 0, 実際 %d", cmd.MaxComments)
	}
	if !cmd.IncludeUpdates {
		t.Error("IncludeUpdates デフォルト: 期待 true, 実際 false")
	}
	if cmd.MaxActivityPages != 5 {
		t.Errorf("MaxActivityPages デフォルト: 期待 5, 実際 %d", cmd.MaxActivityPages)
	}
	if cmd.Since != "" {
		t.Errorf("Since デフォルト: 期待 \"\", 実際 %q", cmd.Since)
	}
	if cmd.Until != "" {
		t.Errorf("Until デフォルト: 期待 \"\", 実際 %q", cmd.Until)
	}
}

// T2: フラグ付きパース
func TestIssueTimelineCmd_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{
		"issue", "timeline", "PROJ-456",
		"--max-comments", "20",
		"--no-include-updates",
		"--max-activity-pages", "3",
		"--since", "2026-01-01",
		"--until", "2026-03-31",
	})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Timeline
	if cmd.IssueKey != "PROJ-456" {
		t.Errorf("IssueKey: 期待 %q, 実際 %q", "PROJ-456", cmd.IssueKey)
	}
	if cmd.MaxComments != 20 {
		t.Errorf("MaxComments: 期待 20, 実際 %d", cmd.MaxComments)
	}
	if cmd.IncludeUpdates {
		t.Error("IncludeUpdates: 期待 false (--no-include-updates), 実際 true")
	}
	if cmd.MaxActivityPages != 3 {
		t.Errorf("MaxActivityPages: 期待 3, 実際 %d", cmd.MaxActivityPages)
	}
	if cmd.Since != "2026-01-01" {
		t.Errorf("Since: 期待 %q, 実際 %q", "2026-01-01", cmd.Since)
	}
	if cmd.Until != "2026-03-31" {
		t.Errorf("Until: 期待 %q, 実際 %q", "2026-03-31", cmd.Until)
	}
}

// T3: 必須引数なしでエラー
func TestIssueTimelineCmd_KongParse_MissingArg(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "timeline"})
	if err == nil {
		t.Error("必須引数なしでエラーが返されなかった")
	}
}

// T4: --help が panic せず処理できる
func TestIssueTimelineCmd_KongParse_Help(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	// --help は Exit を呼ぶためエラーが返される場合がある
	// panic しないことを確認
	_, _ = p.Parse([]string{"issue", "timeline", "--help"})
}
