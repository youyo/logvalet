package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/config"
	"github.com/youyo/logvalet/internal/credentials"
)

// ConfigCmd は config サブコマンド群。
type ConfigCmd struct {
	Init ConfigInitCmd `cmd:"" help:"interactively initialize configuration"`
}

// ConfigureCmd は config init のトップレベルエイリアス。
// logvalet configure = logvalet config init として動作する。
type ConfigureCmd struct {
	InitProfile string `help:"profile name" name:"init-profile"`
	InitSpace   string `help:"Backlog space name" name:"init-space"`
	InitBaseURL string `help:"Backlog base URL" name:"init-base-url"`
	InitAPIKey  string `help:"set API key" name:"init-api-key"`
}

// Run は configure のメインエントリポイント（Kong から呼び出し）。
// ConfigInitCmd に委譲する。
func (c *ConfigureCmd) Run(g *GlobalFlags) error {
	cmd := &ConfigInitCmd{
		InitProfile: c.InitProfile,
		InitSpace:   c.InitSpace,
		InitBaseURL: c.InitBaseURL,
		InitAPIKey:  c.InitAPIKey,
	}
	return cmd.Run(g)
}

// ConfigInitCmd は config init コマンド。
// 対話プロンプトで profile 名、space 名、base_url を入力し config.toml を生成する。
type ConfigInitCmd struct {
	InitProfile string `help:"profile name" name:"init-profile"`
	InitSpace   string `help:"Backlog space name" name:"init-space"`
	InitBaseURL string `help:"Backlog base URL" name:"init-base-url"`
	InitAPIKey  string `help:"set API key" name:"init-api-key"`
}

// Prompter は対話入力を抽象化するインターフェース。
type Prompter interface {
	// Prompt はラベルを表示してユーザー入力を取得する。
	// 空入力の場合は defaultValue を返す。
	Prompt(label string, defaultValue string) (string, error)
	// Confirm は確認プロンプトを表示して bool を返す。
	Confirm(label string, defaultYes bool) (bool, error)
}

// stdinPrompter は os.Stdin / os.Stderr を使った Prompter 実装。
type stdinPrompter struct {
	reader *bufio.Reader
	writer io.Writer
}

// newStdinPrompter は標準入出力を使った Prompter を返す。
func newStdinPrompter() Prompter {
	return &stdinPrompter{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stderr,
	}
}

func (p *stdinPrompter) Prompt(label string, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(p.writer, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(p.writer, "%s: ", label)
	}

	line, err := p.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func (p *stdinPrompter) Confirm(label string, defaultYes bool) (bool, error) {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(p.writer, "%s %s: ", label, suffix)

	line, err := p.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes, nil
	}
	return line == "y" || line == "yes", nil
}

// ConfigInitDeps は ConfigInitCmd のテスト用依存注入。
type ConfigInitDeps struct {
	ConfigPath string
	Writer     config.Writer
	Loader     config.Loader
	Prompter   Prompter
	Stdout     io.Writer
	Stderr     io.Writer
	CredStore  credentials.Store // tokens.json ストア（nil の場合スキップ）
}

// Run は config init のメインエントリポイント（Kong から呼び出し）。
func (c *ConfigInitCmd) Run(g *GlobalFlags) error {
	configPath := config.ResolveConfigPath(g.Config, os.Getenv)
	deps := ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   newStdinPrompter(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		CredStore:  credentials.NewStore(credentials.DefaultTokensPath(os.Getenv)),
	}
	return c.RunWithDeps(deps, c.InitProfile, c.InitSpace, c.InitBaseURL, c.InitAPIKey)
}

// RunWithDeps はテスト用の依存注入付き実行。
// profileName, space, baseURL が全て非空の場合は非対話モード。
// それ以外は対話プロンプトで入力を取得する。
func (c *ConfigInitCmd) RunWithDeps(deps ConfigInitDeps, profileName, space, baseURL, apiKey string) error {
	interactive := profileName == "" || space == ""

	// 対話モード: プロンプトで入力を取得
	if interactive {
		var err error
		if profileName == "" {
			profileName, err = deps.Prompter.Prompt("Profile name", "default")
			if err != nil {
				return err
			}
		}
		if profileName == "" {
			return fmt.Errorf("profile name is required")
		}

		if space == "" {
			space, err = deps.Prompter.Prompt("Space name", "")
			if err != nil {
				return err
			}
		}
		if space == "" {
			return fmt.Errorf("space name is required")
		}

		defaultBaseURL := fmt.Sprintf("https://%s.backlog.com", space)
		if baseURL == "" {
			baseURL, err = deps.Prompter.Prompt("Base URL", defaultBaseURL)
			if err != nil {
				return err
			}
		}

		// API Key 入力（対話モード）
		if apiKey == "" {
			apiKey, err = deps.Prompter.Prompt("API Key (leave empty to skip)", "")
			if err != nil {
				return err
			}
		}
	}

	// base_url が空の場合は space から自動生成
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://%s.backlog.com", space)
	}

	// 既存 config.toml をロード
	cfg, err := deps.Loader.Load(deps.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// 既存プロファイルの上書き確認（対話モードのみ）
	created := true
	if cfg.Profiles != nil {
		if _, exists := cfg.Profiles[profileName]; exists {
			created = false
			if interactive {
				ok, err := deps.Prompter.Confirm(
					fmt.Sprintf("profile %q already exists. overwrite?", profileName),
					false,
				)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("overwrite of profile %q cancelled", profileName)
				}
			}
		}
	}

	// Config をマージ
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.ProfileConfig)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = profileName
	}
	if cfg.DefaultFormat == "" {
		cfg.DefaultFormat = "json"
	}

	cfg.Profiles[profileName] = config.ProfileConfig{
		Space:   space,
		BaseURL: baseURL,
		AuthRef: profileName, // auth_ref はプロファイル名と同じ
	}

	// config.toml を書き出し
	if err := deps.Writer.Write(deps.ConfigPath, cfg); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// API Key が入力された場合、tokens.json に保存
	authSaved := false
	if apiKey != "" && deps.CredStore != nil {
		tokens, err := deps.CredStore.Load()
		if err != nil {
			return fmt.Errorf("failed to load credentials: %w", err)
		}
		if tokens.Auth == nil {
			tokens.Auth = make(map[string]credentials.AuthEntry)
		}
		if tokens.Version == 0 {
			tokens.Version = 1
		}
		tokens.Auth[profileName] = credentials.AuthEntry{
			AuthType: credentials.AuthTypeAPIKey,
			APIKey:   apiKey,
		}
		if err := deps.CredStore.Save(tokens); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		authSaved = true
	}

	// JSON レスポンスを stdout に出力
	resp := configInitResponse{
		SchemaVersion: "1",
		Result:        "ok",
		Profile:       profileName,
		Space:         space,
		BaseURL:       baseURL,
		ConfigPath:    deps.ConfigPath,
		Created:       created,
		AuthSaved:     authSaved,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}
	fmt.Fprintln(deps.Stdout, string(data))

	// stderr に案内
	fmt.Fprintf(deps.Stderr, "configuration saved: %s\n", deps.ConfigPath)
	if authSaved {
		fmt.Fprintf(deps.Stderr, "setup complete! run logvalet project list to verify\n")
	} else {
		fmt.Fprintf(deps.Stderr, "next step: logvalet auth login --profile %s\n", profileName)
	}

	return nil
}

type configInitResponse struct {
	SchemaVersion string `json:"schema_version"`
	Result        string `json:"result"`
	Profile       string `json:"profile"`
	Space         string `json:"space"`
	BaseURL       string `json:"base_url"`
	ConfigPath    string `json:"config_path"`
	Created       bool   `json:"created"`
	AuthSaved     bool   `json:"auth_saved"`
}
