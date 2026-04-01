package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// T1: "activity stats" のパースとデフォルト値
func TestActivityStats_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"activity", "stats"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Activity.Stats
	if cmd.Scope != "space" {
		t.Errorf("Scope デフォルト: 期待 space, 実際 %q", cmd.Scope)
	}
	if cmd.TopN != 5 {
		t.Errorf("TopN デフォルト: 期待 5, 実際 %d", cmd.TopN)
	}
	if cmd.ProjectKey != "" {
		t.Errorf("ProjectKey デフォルト: 期待 \"\", 実際 %q", cmd.ProjectKey)
	}
	if cmd.UserID != "" {
		t.Errorf("UserID デフォルト: 期待 \"\", 実際 %q", cmd.UserID)
	}
	if cmd.Since != "" {
		t.Errorf("Since デフォルト: 期待 \"\", 実際 %q", cmd.Since)
	}
	if cmd.Until != "" {
		t.Errorf("Until デフォルト: 期待 \"\", 実際 %q", cmd.Until)
	}
}

// T2: --scope project -k PROJ のパース確認
func TestActivityStats_KongParse_WithProjectScope(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"activity", "stats", "--scope", "project", "-k", "PROJ"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Activity.Stats
	if cmd.Scope != "project" {
		t.Errorf("Scope: 期待 project, 実際 %q", cmd.Scope)
	}
	if cmd.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey: 期待 PROJ, 実際 %q", cmd.ProjectKey)
	}
}

// T3: --scope user --user-id uid のパース確認
func TestActivityStats_KongParse_WithUserScope(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"activity", "stats", "--scope", "user", "--user-id", "alice"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Activity.Stats
	if cmd.Scope != "user" {
		t.Errorf("Scope: 期待 user, 実際 %q", cmd.Scope)
	}
	if cmd.UserID != "alice" {
		t.Errorf("UserID: 期待 alice, 実際 %q", cmd.UserID)
	}
}

// T4: 全フラグ指定パース確認
func TestActivityStats_KongParse_WithAllFlags(t *testing.T) {
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
		"activity", "stats",
		"--scope", "project",
		"-k", "MYPROJ",
		"--since", "2026-03-01T00:00:00Z",
		"--until", "2026-03-31T23:59:59Z",
		"--top-n", "10",
	})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Activity.Stats
	if cmd.Scope != "project" {
		t.Errorf("Scope: 期待 project, 実際 %q", cmd.Scope)
	}
	if cmd.ProjectKey != "MYPROJ" {
		t.Errorf("ProjectKey: 期待 MYPROJ, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Since != "2026-03-01T00:00:00Z" {
		t.Errorf("Since: 期待 2026-03-01T00:00:00Z, 実際 %q", cmd.Since)
	}
	if cmd.Until != "2026-03-31T23:59:59Z" {
		t.Errorf("Until: 期待 2026-03-31T23:59:59Z, 実際 %q", cmd.Until)
	}
	if cmd.TopN != 10 {
		t.Errorf("TopN: 期待 10, 実際 %d", cmd.TopN)
	}
}

// T5: 不正な scope 値でエラー
func TestActivityStats_KongParse_InvalidScope(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"activity", "stats", "--scope", "invalid"})
	if err == nil {
		t.Error("不正な scope 値でエラーが返されなかった")
	}
}
