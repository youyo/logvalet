// Package config はlogvaletの設定管理を提供する。
//
// 設定値の優先順位: CLI flags > 環境変数 > config.toml > 組み込みデフォルト
// 設定ファイル: ~/.config/logvalet/config.toml (または XDG_CONFIG_HOME)
// spec §4, §5 準拠。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config はconfig.tomlの全体構造。
// spec §5 config.toml schema 準拠。
type Config struct {
	Version        int                      `toml:"version"`
	DefaultProfile string                   `toml:"default_profile"`
	DefaultFormat  string                   `toml:"default_format"`
	Profiles       map[string]ProfileConfig `toml:"profiles"`
}

// ProfileConfig は単一プロファイルの設定。
type ProfileConfig struct {
	Space   string `toml:"space"`
	BaseURL string `toml:"base_url"`
	AuthRef string `toml:"auth_ref"`
	TeamID  int    `toml:"team_id"`
}

// ResolvedConfig は優先順位解決後の最終設定値。
// CLI flags > env > config.toml > デフォルト の順で決定される。
type ResolvedConfig struct {
	Profile    string
	Format     string
	Pretty     bool
	Space      string
	BaseURL    string
	AuthRef    string
	TeamID     int
	Verbose    bool
	NoColor    bool
	ConfigPath string
}

// OverrideFlags はCLI flagsからの上書き値。
// 空文字列は「未指定」を意味する。
// boolポインタは nil が「未指定」、非nilが「明示的指定」を意味する。
type OverrideFlags struct {
	Profile    string
	Format     string
	Pretty     *bool
	Space      string
	BaseURL    string
	Verbose    *bool
	NoColor    *bool
	ConfigPath string
}

// Loader はconfig.tomlをロードし、優先順位解決を行う。
type Loader interface {
	Load(path string) (*Config, error)
	Resolve(cfg *Config, flags OverrideFlags, getenv func(string) string) (*ResolvedConfig, error)
}

// defaultLoader は Loader の標準実装。
type defaultLoader struct{}

// NewDefaultLoader は標準的な Loader を返す。
func NewDefaultLoader() Loader {
	return &defaultLoader{}
}

// Load はconfig.tomlをパースして Config を返す。
// ファイルが存在しない場合はゼロ値の Config を返す（エラーなし）。
// TOML パースエラーの場合はエラーを返す。
func (l *defaultLoader) Load(path string) (*Config, error) {
	return Load(path)
}

// Resolve は優先順位に従い ResolvedConfig を生成する。
// 優先順位: OverrideFlags > getenv > Config > 組み込みデフォルト
func (l *defaultLoader) Resolve(cfg *Config, flags OverrideFlags, getenv func(string) string) (*ResolvedConfig, error) {
	return Resolve(cfg, flags, getenv)
}

// ParseBoolEnv は環境変数の文字列をboolに変換する。
// spec §4 Boolean env parsing 準拠。
//
// true: "1", "true", "yes", "on"（大文字小文字不問）
// false: "0", "false", "no", "off"（大文字小文字不問）
// その他: エラー
func ParseBoolEnv(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean env value: %q (expected 1/true/yes/on or 0/false/no/off)", s)
	}
}

// DefaultConfigPath はデフォルトのconfig.tomlパスを返す。
// XDG_CONFIG_HOME が設定されていれば $XDG_CONFIG_HOME/logvalet/config.toml、
// そうでなければ ~/.config/logvalet/config.toml を返す。
func DefaultConfigPath() string {
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "logvalet", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// UserHomeDir が失敗する場合は固定パス
		return filepath.Join(".config", "logvalet", "config.toml")
	}
	return filepath.Join(home, ".config", "logvalet", "config.toml")
}

// ResolveConfigPath は設定ファイルパスを優先順位に従い決定する。
//
// 優先順位:
//  1. override が非空なら使用（CLI --config フラグ）
//  2. getenv("LOGVALET_CONFIG") が非空なら使用
//  3. DefaultConfigPath() を使用
func ResolveConfigPath(override string, getenv func(string) string) string {
	if override != "" {
		return override
	}
	if envPath := getenv("LOGVALET_CONFIG"); envPath != "" {
		return envPath
	}
	return DefaultConfigPath()
}

// Load は指定パスのconfig.tomlをロードして Config を返す。
// ファイルが存在しない場合はゼロ値の Config を返す（エラーなし）。
// パースエラーの場合はエラーを返す。
func Load(path string) (*Config, error) {
	cfg := &Config{}
	_, err := toml.DecodeFile(path, cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// ファイル不在は正常ケース（初回起動時等）
			return cfg, nil
		}
		return nil, fmt.Errorf("config: failed to parse %s: %w", path, err)
	}
	return cfg, nil
}

// Resolve は優先順位に従い ResolvedConfig を生成する。
//
// 優先順位（高い順）:
//  1. OverrideFlags（CLI フラグ経由）
//  2. getenv で取得した環境変数
//  3. cfg の設定値（config.toml）
//  4. 組み込みデフォルト値
func Resolve(cfg *Config, flags OverrideFlags, getenv func(string) string) (*ResolvedConfig, error) {
	resolved := &ResolvedConfig{}

	// --- Profile ---
	profile := resolveString(flags.Profile, getenv("LOGVALET_PROFILE"), cfg.DefaultProfile, "")
	resolved.Profile = profile

	// --- Format ---
	resolved.Format = resolveString(flags.Format, getenv("LOGVALET_FORMAT"), cfg.DefaultFormat, "json")

	// --- Pretty ---
	pretty, err := resolveBool(flags.Pretty, getenv("LOGVALET_PRETTY"), false)
	if err != nil {
		return nil, fmt.Errorf("config: LOGVALET_PRETTY: %w", err)
	}
	resolved.Pretty = pretty

	// --- Verbose ---
	verbose, err := resolveBool(flags.Verbose, getenv("LOGVALET_VERBOSE"), false)
	if err != nil {
		return nil, fmt.Errorf("config: LOGVALET_VERBOSE: %w", err)
	}
	resolved.Verbose = verbose

	// --- NoColor ---
	noColor, err := resolveBool(flags.NoColor, getenv("LOGVALET_NO_COLOR"), false)
	if err != nil {
		return nil, fmt.Errorf("config: LOGVALET_NO_COLOR: %w", err)
	}
	resolved.NoColor = noColor

	// --- Profile-specific settings ---
	// プロファイルが指定されている場合はプロファイル設定を取得
	var profileCfg ProfileConfig
	if profile != "" && cfg.Profiles != nil {
		profileCfg = cfg.Profiles[profile]
	}

	// --- Space ---
	// CLI flags.Space > env > profileCfg.Space
	resolved.Space = resolveString(flags.Space, getenv("LOGVALET_SPACE"), profileCfg.Space, "")

	// --- BaseURL ---
	// CLI flags.BaseURL > env > profileCfg.BaseURL
	resolved.BaseURL = resolveString(flags.BaseURL, getenv("LOGVALET_BASE_URL"), profileCfg.BaseURL, "")

	// --- AuthRef ---
	// profileCfg からのみ（CLI/env での上書きは M03 で対応）
	resolved.AuthRef = profileCfg.AuthRef

	// --- TeamID ---
	// profileCfg からのみ
	resolved.TeamID = profileCfg.TeamID

	return resolved, nil
}

// resolveString は優先順位付きで文字列を解決する。
// 空文字列は「未指定」として扱う。
func resolveString(override, envVal, cfgVal, defaultVal string) string {
	if override != "" {
		return override
	}
	if envVal != "" {
		return envVal
	}
	if cfgVal != "" {
		return cfgVal
	}
	return defaultVal
}

// resolveBool は優先順位付きでboolを解決する。
// flagVal が非nil なら CLI フラグ値を使用。
// envVal が非空なら ParseBoolEnv で変換。
// どちらも未指定なら defaultVal を使用。
func resolveBool(flagVal *bool, envVal string, defaultVal bool) (bool, error) {
	if flagVal != nil {
		return *flagVal, nil
	}
	if envVal != "" {
		return ParseBoolEnv(envVal)
	}
	return defaultVal, nil
}
