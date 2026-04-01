package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: "user workload PROJ" のパースとデフォルト値
func TestUserWorkload_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"user", "workload", "PROJ"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.User.Workload
	if cmd.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey: 期待 PROJ, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Days != 7 {
		t.Errorf("Days デフォルト: 期待 7, 実際 %d", cmd.Days)
	}
	if cmd.ExcludeStatus != "" {
		t.Errorf("ExcludeStatus デフォルト: 期待 \"\", 実際 %q", cmd.ExcludeStatus)
	}
}

// T2: フラグ付きパース
func TestUserWorkload_KongParse_WithFlags(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"user", "workload", "PROJ", "--days", "14", "--exclude-status", "完了,対応済み"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.User.Workload
	if cmd.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey: 期待 PROJ, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Days != 14 {
		t.Errorf("Days: 期待 14, 実際 %d", cmd.Days)
	}
	if cmd.ExcludeStatus != "完了,対応済み" {
		t.Errorf("ExcludeStatus: 期待 %q, 実際 %q", "完了,対応済み", cmd.ExcludeStatus)
	}
}

// T3: PROJECT_KEY なしでエラー
func TestUserWorkload_KongParse_MissingProjectKey(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"user", "workload"})
	if err == nil {
		t.Error("PROJECT_KEY 引数なしでエラーが返されなかった")
	}
}
