package cli_test

import (
	"os"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// ---- M13: 新フラグのパーステスト ----

func TestGlobalFlags_NewFlags_CLIParse(t *testing.T) {
	tests := []struct {
		name string
		args []string
		check func(t *testing.T, g cli.GlobalFlags)
	}{
		{
			name: "--api-key が正しくパースされる",
			args: []string{"--api-key", "my-api-key"},
			check: func(t *testing.T, g cli.GlobalFlags) {
				if g.APIKey != "my-api-key" {
					t.Errorf("APIKey = %q, want %q", g.APIKey, "my-api-key")
				}
			},
		},
		{
			name: "--access-token が正しくパースされる",
			args: []string{"--access-token", "my-token"},
			check: func(t *testing.T, g cli.GlobalFlags) {
				if g.AccessToken != "my-token" {
					t.Errorf("AccessToken = %q, want %q", g.AccessToken, "my-token")
				}
			},
		},
		{
			name: "--base-url が正しくパースされる",
			args: []string{"--base-url", "https://example.backlog.com"},
			check: func(t *testing.T, g cli.GlobalFlags) {
				if g.BaseURL != "https://example.backlog.com" {
					t.Errorf("BaseURL = %q, want %q", g.BaseURL, "https://example.backlog.com")
				}
			},
		},
		{
			name: "--space (-s) が正しくパースされる",
			args: []string{"-s", "my-space"},
			check: func(t *testing.T, g cli.GlobalFlags) {
				if g.Space != "my-space" {
					t.Errorf("Space = %q, want %q", g.Space, "my-space")
				}
			},
		},
		{
			name: "--config (-c) が正しくパースされる",
			args: []string{"-c", "/tmp/custom-config.toml"},
			check: func(t *testing.T, g cli.GlobalFlags) {
				if g.Config != "/tmp/custom-config.toml" {
					t.Errorf("Config = %q, want %q", g.Config, "/tmp/custom-config.toml")
				}
			},
		},
		{
			name: "--no-color が正しくパースされる",
			args: []string{"--no-color"},
			check: func(t *testing.T, g cli.GlobalFlags) {
				if !g.NoColor {
					t.Error("NoColor = false, want true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 排他バリデーション対策: 認証系 env をクリア
			t.Setenv("LOGVALET_API_KEY", "")
			t.Setenv("LOGVALET_ACCESS_TOKEN", "")

			var flags cli.GlobalFlags
			p, err := kong.New(&flags)
			if err != nil {
				t.Fatalf("kong.New() エラー: %v", err)
			}
			if _, err := p.Parse(tt.args); err != nil {
				t.Fatalf("Parse(%v) エラー: %v", tt.args, err)
			}
			tt.check(t, flags)
		})
	}
}

func TestGlobalFlags_NewFlags_EnvOverride(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		envVal string
		check  func(t *testing.T, g cli.GlobalFlags)
	}{
		// LOGVALET_API_KEY, LOGVALET_ACCESS_TOKEN, LOGVALET_BASE_URL, LOGVALET_SPACE は
		// Kong env タグから削除。プロファイル固有設定（tokens.json / config.toml）より
		// 低優先にするため、それぞれ credentials.Resolve() / config.Resolve() で処理する。
		{
			name:   "LOGVALET_CONFIG で Config を設定できる",
			envKey: "LOGVALET_CONFIG",
			envVal: "/env/config.toml",
			check: func(t *testing.T, g cli.GlobalFlags) {
				if g.Config != "/env/config.toml" {
					t.Errorf("Config = %q, want %q", g.Config, "/env/config.toml")
				}
			},
		},
		{
			name:   "LOGVALET_NO_COLOR で NoColor を設定できる",
			envKey: "LOGVALET_NO_COLOR",
			envVal: "true",
			check: func(t *testing.T, g cli.GlobalFlags) {
				if !g.NoColor {
					t.Error("NoColor = false, want true via env")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 排他バリデーション対策: 他の認証系 env をクリア
			t.Setenv("LOGVALET_API_KEY", "")
			t.Setenv("LOGVALET_ACCESS_TOKEN", "")
			t.Setenv(tt.envKey, tt.envVal)

			var flags cli.GlobalFlags
			p, err := kong.New(&flags)
			if err != nil {
				t.Fatalf("kong.New() エラー: %v", err)
			}
			if _, err := p.Parse([]string{}); err != nil {
				t.Fatalf("Parse() エラー: %v", err)
			}
			tt.check(t, flags)
		})
	}
}

func TestGlobalFlags_Validate_MutualExclusion(t *testing.T) {
	t.Run("--api-key と --access-token の同時指定はエラー", func(t *testing.T) {
		var flags cli.GlobalFlags
		p, err := kong.New(&flags)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		_, err = p.Parse([]string{"--api-key", "key", "--access-token", "token"})
		// Kong が Validate() を呼び出すか、または手動呼び出しでエラーになるべき
		if err == nil {
			// Kong が自動で Validate を呼ばない場合は手動テスト
			if validateErr := flags.Validate(); validateErr == nil {
				t.Error("APIKey と AccessToken の同時指定でエラーが返るべき")
			}
		}
	})

	t.Run("--api-key のみはエラーなし", func(t *testing.T) {
		flags := cli.GlobalFlags{APIKey: "key"}
		if err := flags.Validate(); err != nil {
			t.Errorf("APIKey のみで Validate() エラー: %v", err)
		}
	})

	t.Run("--access-token のみはエラーなし", func(t *testing.T) {
		flags := cli.GlobalFlags{AccessToken: "token"}
		if err := flags.Validate(); err != nil {
			t.Errorf("AccessToken のみで Validate() エラー: %v", err)
		}
	})

	t.Run("両方未指定はエラーなし", func(t *testing.T) {
		flags := cli.GlobalFlags{}
		if err := flags.Validate(); err != nil {
			t.Errorf("両方未指定で Validate() エラー: %v", err)
		}
	})
}

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
