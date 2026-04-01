package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// newTestParser は CLI テスト用の Kong パーサーを生成する。
func newTestParser(t *testing.T) *kong.Kong {
	t.Helper()
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	return p
}

// T1: "issue context PROJ-123" のパースとデフォルト値
func TestIssueContextCmd_KongParse(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "context", "PROJ-123"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Context
	if cmd.IssueIDOrKey != "PROJ-123" {
		t.Errorf("IssueIDOrKey: 期待 %q, 実際 %q", "PROJ-123", cmd.IssueIDOrKey)
	}
	if cmd.Comments != 10 {
		t.Errorf("Comments デフォルト: 期待 %d, 実際 %d", 10, cmd.Comments)
	}
	if cmd.Compact {
		t.Error("Compact デフォルト: 期待 false, 実際 true")
	}
}

// T2: フラグ付きパース
func TestIssueContextCmd_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "context", "PROJ-123", "--comments", "20", "--compact"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Context
	if cmd.IssueIDOrKey != "PROJ-123" {
		t.Errorf("IssueIDOrKey: 期待 %q, 実際 %q", "PROJ-123", cmd.IssueIDOrKey)
	}
	if cmd.Comments != 20 {
		t.Errorf("Comments: 期待 %d, 実際 %d", 20, cmd.Comments)
	}
	if !cmd.Compact {
		t.Error("Compact: 期待 true, 実際 false")
	}
}

// T3: 必須引数なしでエラー
func TestIssueContextCmd_KongParse_MissingArg(t *testing.T) {
	p := newTestParser(t)
	_, err := p.Parse([]string{"issue", "context"})
	if err == nil {
		t.Error("必須引数なしでエラーが返されなかった")
	}
}

// T4: --help が panic せず処理できる
func TestIssueContextCmd_KongParse_Help(t *testing.T) {
	p := newTestParser(t)
	// --help は Exit を呼ぶためエラーが返される場合がある
	// panic しないことを確認
	_, _ = p.Parse([]string{"issue", "context", "--help"})
}
