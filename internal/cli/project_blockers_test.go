package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: "project blockers PROJ" のパースとデフォルト値
func TestProjectBlockers_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"project", "blockers", "PROJ"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Blockers
	if cmd.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey: 期待 PROJ, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Days != 14 {
		t.Errorf("Days デフォルト: 期待 14, 実際 %d", cmd.Days)
	}
	if cmd.IncludeComments != false {
		t.Errorf("IncludeComments デフォルト: 期待 false, 実際 %v", cmd.IncludeComments)
	}
	if cmd.ExcludeStatus != "" {
		t.Errorf("ExcludeStatus デフォルト: 期待 \"\", 実際 %q", cmd.ExcludeStatus)
	}
}

// T2: フラグ付きパース
func TestProjectBlockers_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"project", "blockers", "PROJ", "--days", "7", "--include-comments", "--exclude-status", "完了,対応済み"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Project.Blockers
	if cmd.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey: 期待 PROJ, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Days != 7 {
		t.Errorf("Days: 期待 7, 実際 %d", cmd.Days)
	}
	if cmd.IncludeComments != true {
		t.Errorf("IncludeComments: 期待 true, 実際 %v", cmd.IncludeComments)
	}
	if cmd.ExcludeStatus != "完了,対応済み" {
		t.Errorf("ExcludeStatus: 期待 %q, 実際 %q", "完了,対応済み", cmd.ExcludeStatus)
	}
}

// T3: PROJECT 引数なしでエラー
func TestProjectBlockers_KongParse_MissingProjectKey(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"project", "blockers"})
	if err == nil {
		t.Error("PROJECT 引数なしでエラーが返されなかった")
	}
}
