package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// D1: "digest daily -k HEP" のデフォルトパース
func TestDigestDailyCmd_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "daily", "-k", "HEP"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Digest.Daily
	if cmd.ProjectKey != "HEP" {
		t.Errorf("ProjectKey: 期待 HEP, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Since != "" {
		t.Errorf("Since デフォルト: 期待 \"\", 実際 %q", cmd.Since)
	}
}

// D2: "digest daily -k HEP --since 2026-03-31" のパース
func TestDigestDailyCmd_KongParse_WithSince(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "daily", "-k", "HEP", "--since", "2026-03-31"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Digest.Daily
	if cmd.ProjectKey != "HEP" {
		t.Errorf("ProjectKey: 期待 HEP, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Since != "2026-03-31" {
		t.Errorf("Since: 期待 2026-03-31, 実際 %q", cmd.Since)
	}
}

// D3: -k なしでエラー
func TestDigestDailyCmd_KongParse_MissingProjectKey(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "daily"})
	if err == nil {
		t.Error("-k なしでエラーが返されなかった")
	}
}

// U1: "digest unified --since 2026-03-01" が正常にパースされること
func TestDigestUnified_KongParse(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "unified", "--since", "2026-03-01"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Digest.Unified
	if cmd.Since != "2026-03-01" {
		t.Errorf("Since: 期待 2026-03-01, 実際 %q", cmd.Since)
	}
}
