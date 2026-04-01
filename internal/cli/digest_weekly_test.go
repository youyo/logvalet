package cli_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// W1: "digest weekly -k HEP" のデフォルトパース
func TestDigestWeeklyCmd_KongParse_Default(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "weekly", "-k", "HEP"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Digest.Weekly
	if cmd.ProjectKey != "HEP" {
		t.Errorf("ProjectKey: 期待 HEP, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Since != "" {
		t.Errorf("Since デフォルト: 期待 \"\", 実際 %q", cmd.Since)
	}
	if cmd.Until != "" {
		t.Errorf("Until デフォルト: 期待 \"\", 実際 %q", cmd.Until)
	}
}

// W2: "digest weekly -k HEP --since 2026-03-25 --until 2026-03-31" のパース
func TestDigestWeeklyCmd_KongParse_WithSinceUntil(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "weekly", "-k", "HEP", "--since", "2026-03-25", "--until", "2026-03-31"})
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	cmd := root.Digest.Weekly
	if cmd.ProjectKey != "HEP" {
		t.Errorf("ProjectKey: 期待 HEP, 実際 %q", cmd.ProjectKey)
	}
	if cmd.Since != "2026-03-25" {
		t.Errorf("Since: 期待 2026-03-25, 実際 %q", cmd.Since)
	}
	if cmd.Until != "2026-03-31" {
		t.Errorf("Until: 期待 2026-03-31, 実際 %q", cmd.Until)
	}
}

// W3: -k なしでエラー
func TestDigestWeeklyCmd_KongParse_MissingProjectKey(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "weekly"})
	if err == nil {
		t.Error("-k なしでエラーが返されなかった")
	}
}

// U2: 旧構文 "digest --since 2026-03-01" はパースエラーになること（廃止確認）
func TestDigestCmd_OldSyntax_ParseError(t *testing.T) {
	var root cli.CLI
	p, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() エラー: %v", err)
	}
	_, err = p.Parse([]string{"digest", "--since", "2026-03-01"})
	if err == nil {
		t.Error("旧構文 'digest --since ...' はパースエラーになるべき（DigestCmd はグループコマンドになったため）")
	}
}
