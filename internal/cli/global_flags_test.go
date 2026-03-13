package cli_test

import (
	"os"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

func TestGlobalFlags_Defaults(t *testing.T) {
	t.Run("Format デフォルト値は json", func(t *testing.T) {
		var flags cli.GlobalFlags
		p, err := kong.New(&flags)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		if _, err := p.Parse([]string{}); err != nil {
			t.Fatalf("Parse() エラー: %v", err)
		}
		if flags.Format != "json" {
			t.Errorf("Format = %q, want %q", flags.Format, "json")
		}
	})

	t.Run("Pretty デフォルト値は false", func(t *testing.T) {
		var flags cli.GlobalFlags
		p, err := kong.New(&flags)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		if _, err := p.Parse([]string{}); err != nil {
			t.Fatalf("Parse() エラー: %v", err)
		}
		if flags.Pretty {
			t.Error("Pretty のデフォルト値は false であるべき")
		}
	})
}

func TestGlobalFlags_EnvOverride(t *testing.T) {
	t.Run("LOGVALET_FORMAT 環境変数で Format を設定できる", func(t *testing.T) {
		t.Setenv("LOGVALET_FORMAT", "yaml")
		defer os.Unsetenv("LOGVALET_FORMAT")

		var flags cli.GlobalFlags
		p, err := kong.New(&flags)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		if _, err := p.Parse([]string{}); err != nil {
			t.Fatalf("Parse() エラー: %v", err)
		}
		if flags.Format != "yaml" {
			t.Errorf("Format = %q, want %q via env LOGVALET_FORMAT", flags.Format, "yaml")
		}
	})

	t.Run("LOGVALET_PROFILE 環境変数で Profile を設定できる", func(t *testing.T) {
		t.Setenv("LOGVALET_PROFILE", "myspace")

		var flags cli.GlobalFlags
		p, err := kong.New(&flags)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		if _, err := p.Parse([]string{}); err != nil {
			t.Fatalf("Parse() エラー: %v", err)
		}
		if flags.Profile != "myspace" {
			t.Errorf("Profile = %q, want %q via env LOGVALET_PROFILE", flags.Profile, "myspace")
		}
	})
}
