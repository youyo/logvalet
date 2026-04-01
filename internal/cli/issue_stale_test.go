package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: "issue stale -k PROJ" のパースとデフォルト値
func TestIssueStale_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "stale", "-k", "PROJ"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Stale
	if len(cmd.ProjectKey) != 1 || cmd.ProjectKey[0] != "PROJ" {
		t.Errorf("ProjectKey: 期待 [PROJ], 実際 %v", cmd.ProjectKey)
	}
	if cmd.Days != 7 {
		t.Errorf("Days デフォルト: 期待 7, 実際 %d", cmd.Days)
	}
	if cmd.ExcludeStatus != "" {
		t.Errorf("ExcludeStatus デフォルト: 期待 \"\", 実際 %q", cmd.ExcludeStatus)
	}
}

// T2: フラグ付きパース
func TestIssueStale_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "stale", "-k", "PROJ", "--days", "14", "--exclude-status", "完了,対応済み"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Stale
	if len(cmd.ProjectKey) != 1 || cmd.ProjectKey[0] != "PROJ" {
		t.Errorf("ProjectKey: 期待 [PROJ], 実際 %v", cmd.ProjectKey)
	}
	if cmd.Days != 14 {
		t.Errorf("Days: 期待 14, 実際 %d", cmd.Days)
	}
	if cmd.ExcludeStatus != "完了,対応済み" {
		t.Errorf("ExcludeStatus: 期待 %q, 実際 %q", "完了,対応済み", cmd.ExcludeStatus)
	}
}

// T3: 複数プロジェクトキー
func TestIssueStale_KongParse_MultipleProjectKeys(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "stale", "-k", "PROJ1", "-k", "PROJ2"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Issue.Stale
	if len(cmd.ProjectKey) != 2 {
		t.Errorf("ProjectKey len: 期待 2, 実際 %d", len(cmd.ProjectKey))
	}
	if cmd.ProjectKey[0] != "PROJ1" || cmd.ProjectKey[1] != "PROJ2" {
		t.Errorf("ProjectKey: 期待 [PROJ1, PROJ2], 実際 %v", cmd.ProjectKey)
	}
}

// T4: -k なしでエラー
func TestIssueStale_KongParse_MissingProjectKey(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"issue", "stale"})
	if err == nil {
		t.Error("-k なしでエラーが返されなかった")
	}
}
