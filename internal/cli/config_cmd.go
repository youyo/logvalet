package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/youyo/logvalet/internal/config"
)

// ConfigCmd は config サブコマンド群。
type ConfigCmd struct {
	Init ConfigInitCmd `cmd:"" help:"対話型で設定を初期化する"`
}

// ConfigureCmd は config init のトップレベルエイリアス。
// logvalet configure = logvalet config init として動作する。
type ConfigureCmd struct {
	InitProfile string `help:"プロファイル名" name:"init-profile"`
	InitSpace   string `help:"Backlog スペース名" name:"init-space"`
	InitBaseURL string `help:"Backlog ベース URL" name:"init-base-url"`
}

// Run は configure のメインエントリポイント（Kong から呼び出し）。
// ConfigInitCmd に委譲する。
func (c *ConfigureCmd) Run(g *GlobalFlags) error {
	cmd := &ConfigInitCmd{
		InitProfile: c.InitProfile,
		InitSpace:   c.InitSpace,
		InitBaseURL: c.InitBaseURL,
	}
	return cmd.Run(g)
}

// ConfigInitCmd は config init コマンド。
// 対話プロンプトで profile 名、space 名、base_url を入力し config.toml を生成する。
type ConfigInitCmd struct {
	InitProfile string `help:"プロファイル名" name:"init-profile"`
	InitSpace   string `help:"Backlog スペース名" name:"init-space"`
	InitBaseURL string `help:"Backlog ベース URL" name:"init-base-url"`
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
		return "", fmt.Errorf("入力の読み取りに失敗しました: %w", err)
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
		return false, fmt.Errorf("入力の読み取りに失敗しました: %w", err)
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
	}
	return c.RunWithDeps(deps, c.InitProfile, c.InitSpace, c.InitBaseURL)
}

// RunWithDeps はテスト用の依存注入付き実行。
// profileName, space, baseURL が全て非空の場合は非対話モード。
// それ以外は対話プロンプトで入力を取得する。
func (c *ConfigInitCmd) RunWithDeps(deps ConfigInitDeps, profileName, space, baseURL string) error {
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
			return fmt.Errorf("プロファイル名は必須です")
		}

		if space == "" {
			space, err = deps.Prompter.Prompt("Space name", "")
			if err != nil {
				return err
			}
		}
		if space == "" {
			return fmt.Errorf("スペース名は必須です")
		}

		defaultBaseURL := fmt.Sprintf("https://%s.backlog.com", space)
		if baseURL == "" {
			baseURL, err = deps.Prompter.Prompt("Base URL", defaultBaseURL)
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
		return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	// 既存プロファイルの上書き確認（対話モードのみ）
	created := true
	if cfg.Profiles != nil {
		if _, exists := cfg.Profiles[profileName]; exists {
			created = false
			if interactive {
				ok, err := deps.Prompter.Confirm(
					fmt.Sprintf("プロファイル %q は既に存在します。上書きしますか？", profileName),
					false,
				)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("プロファイル %q の上書きがキャンセルされました", profileName)
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
		return fmt.Errorf("設定ファイルの書き出しに失敗しました: %w", err)
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
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("レスポンスの生成に失敗しました: %w", err)
	}
	fmt.Fprintln(deps.Stdout, string(data))

	// stderr に次のステップを案内
	fmt.Fprintf(deps.Stderr, "設定を保存しました: %s\n", deps.ConfigPath)
	fmt.Fprintf(deps.Stderr, "次のステップ: logvalet auth login --profile %s\n", profileName)

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
}
