// Package config のテスト。
package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/youyo/logvalet/internal/config"
)

// ---- ParseBoolEnv ----

func TestParseBoolEnv_True(t *testing.T) {
	t.Parallel()
	trueValues := []string{"1", "true", "yes", "on", "TRUE", "True", "YES", "ON"}
	for _, v := range trueValues {
		got, err := config.ParseBoolEnv(v)
		if err != nil {
			t.Errorf("ParseBoolEnv(%q) returned unexpected error: %v", v, err)
			continue
		}
		if !got {
			t.Errorf("ParseBoolEnv(%q) = false, want true", v)
		}
	}
}

func TestParseBoolEnv_False(t *testing.T) {
	t.Parallel()
	falseValues := []string{"0", "false", "no", "off", "FALSE", "False", "NO", "OFF"}
	for _, v := range falseValues {
		got, err := config.ParseBoolEnv(v)
		if err != nil {
			t.Errorf("ParseBoolEnv(%q) returned unexpected error: %v", v, err)
			continue
		}
		if got {
			t.Errorf("ParseBoolEnv(%q) = true, want false", v)
		}
	}
}

func TestParseBoolEnv_Invalid(t *testing.T) {
	t.Parallel()
	invalidValues := []string{"invalid", "", "2", "maybe", "t", "f"}
	for _, v := range invalidValues {
		_, err := config.ParseBoolEnv(v)
		if err == nil {
			t.Errorf("ParseBoolEnv(%q) expected error, got nil", v)
		}
	}
}

// ---- DefaultConfigPath ----

func TestDefaultConfigPath(t *testing.T) {
	t.Parallel()
	path := config.DefaultConfigPath()
	if path == "" {
		t.Error("DefaultConfigPath() returned empty string")
	}
	// パスに logvalet/config.toml が含まれる
	if filepath.Base(path) != "config.toml" {
		t.Errorf("DefaultConfigPath() base = %q, want config.toml", filepath.Base(path))
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "logvalet" {
		t.Errorf("DefaultConfigPath() dir = %q, want .../logvalet", dir)
	}
}

func TestDefaultConfigPath_XDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG not tested on Windows")
	}
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	path := config.DefaultConfigPath()
	expected := filepath.Join(tmpDir, "logvalet", "config.toml")
	if path != expected {
		t.Errorf("DefaultConfigPath() with XDG = %q, want %q", path, expected)
	}
}

// ---- ResolveConfigPath ----

func TestResolveConfigPath_Override(t *testing.T) {
	t.Parallel()
	got := config.ResolveConfigPath("/custom/path.toml", func(key string) string { return "" })
	if got != "/custom/path.toml" {
		t.Errorf("ResolveConfigPath with override = %q, want /custom/path.toml", got)
	}
}

func TestResolveConfigPath_Env(t *testing.T) {
	t.Parallel()
	got := config.ResolveConfigPath("", func(key string) string {
		if key == "LOGVALET_CONFIG" {
			return "/env/path.toml"
		}
		return ""
	})
	if got != "/env/path.toml" {
		t.Errorf("ResolveConfigPath with env = %q, want /env/path.toml", got)
	}
}

func TestResolveConfigPath_Default(t *testing.T) {
	t.Parallel()
	got := config.ResolveConfigPath("", func(key string) string { return "" })
	if got == "" {
		t.Error("ResolveConfigPath default returned empty string")
	}
	if filepath.Base(got) != "config.toml" {
		t.Errorf("ResolveConfigPath default = %q, base should be config.toml", got)
	}
}

// ---- Load ----

func TestLoad_Valid(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "valid.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", path, err)
	}
	if cfg.Version != 1 {
		t.Errorf("cfg.Version = %d, want 1", cfg.Version)
	}
	if cfg.DefaultProfile != "work" {
		t.Errorf("cfg.DefaultProfile = %q, want work", cfg.DefaultProfile)
	}
	if cfg.DefaultFormat != "json" {
		t.Errorf("cfg.DefaultFormat = %q, want json", cfg.DefaultFormat)
	}
	if len(cfg.Profiles) != 2 {
		t.Errorf("len(cfg.Profiles) = %d, want 2", len(cfg.Profiles))
	}
	work, ok := cfg.Profiles["work"]
	if !ok {
		t.Fatal("cfg.Profiles[work] not found")
	}
	if work.Space != "example-space" {
		t.Errorf("work.Space = %q, want example-space", work.Space)
	}
	if work.BaseURL != "https://example-space.backlog.com" {
		t.Errorf("work.BaseURL = %q, want https://example-space.backlog.com", work.BaseURL)
	}
	if work.AuthRef != "example-space" {
		t.Errorf("work.AuthRef = %q, want example-space", work.AuthRef)
	}
}

func TestLoad_Empty(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "empty.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", path, err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil Config for empty file")
	}
	if cfg.Version != 0 {
		t.Errorf("cfg.Version = %d, want 0 for empty", cfg.Version)
	}
}

func TestLoad_NotFound(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load(nonexistent) should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load(nonexistent) returned nil Config, want empty Config")
	}
	if cfg.Version != 0 {
		t.Errorf("cfg.Version = %d, want 0 for not-found", cfg.Version)
	}
}

func TestLoad_Invalid(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "invalid.toml")
	_, err := config.Load(path)
	if err == nil {
		t.Error("Load(invalid.toml) expected error, got nil")
	}
}

// ---- Resolve ----

func makeNoopGetenv() func(string) string {
	return func(key string) string { return "" }
}

func TestResolve_BuiltInDefaults(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	resolved, err := config.Resolve(cfg, config.OverrideFlags{}, makeNoopGetenv())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if resolved.Format != "json" {
		t.Errorf("resolved.Format = %q, want json (default)", resolved.Format)
	}
}

func TestResolve_ConfigDefault(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		DefaultProfile: "work",
		DefaultFormat:  "yaml",
		Profiles: map[string]config.ProfileConfig{
			"work": {
				Space:   "my-space",
				BaseURL: "https://my-space.backlog.com",
				AuthRef: "my-space",
			},
		},
	}
	resolved, err := config.Resolve(cfg, config.OverrideFlags{}, makeNoopGetenv())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if resolved.Profile != "work" {
		t.Errorf("resolved.Profile = %q, want work", resolved.Profile)
	}
	if resolved.Format != "yaml" {
		t.Errorf("resolved.Format = %q, want yaml", resolved.Format)
	}
	if resolved.Space != "my-space" {
		t.Errorf("resolved.Space = %q, want my-space", resolved.Space)
	}
	if resolved.BaseURL != "https://my-space.backlog.com" {
		t.Errorf("resolved.BaseURL = %q, want https://my-space.backlog.com", resolved.BaseURL)
	}
	if resolved.AuthRef != "my-space" {
		t.Errorf("resolved.AuthRef = %q, want my-space", resolved.AuthRef)
	}
}

func TestResolve_EnvOverride(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		DefaultProfile: "work",
		DefaultFormat:  "json",
		Profiles: map[string]config.ProfileConfig{
			"work": {Space: "work-space", BaseURL: "https://work.backlog.com", AuthRef: "work"},
			"dev":  {Space: "dev-space", BaseURL: "https://dev.backlog.com", AuthRef: "dev"},
		},
	}
	getenv := func(key string) string {
		m := map[string]string{
			"LOGVALET_PROFILE": "dev",
			"LOGVALET_FORMAT":  "markdown",
			"LOGVALET_PRETTY":  "1",
			"LOGVALET_VERBOSE": "yes",
		}
		return m[key]
	}
	resolved, err := config.Resolve(cfg, config.OverrideFlags{}, getenv)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if resolved.Profile != "dev" {
		t.Errorf("resolved.Profile = %q, want dev (from env)", resolved.Profile)
	}
	if resolved.Format != "markdown" {
		t.Errorf("resolved.Format = %q, want markdown (from env)", resolved.Format)
	}
	if !resolved.Pretty {
		t.Error("resolved.Pretty = false, want true (from env 1)")
	}
	if !resolved.Verbose {
		t.Error("resolved.Verbose = false, want true (from env yes)")
	}
	if resolved.Space != "dev-space" {
		t.Errorf("resolved.Space = %q, want dev-space", resolved.Space)
	}
}

func TestResolve_CLIFlagsOverride(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		DefaultProfile: "work",
		DefaultFormat:  "json",
		Profiles: map[string]config.ProfileConfig{
			"work": {Space: "work-space", BaseURL: "https://work.backlog.com", AuthRef: "work"},
			"dev":  {Space: "dev-space", BaseURL: "https://dev.backlog.com", AuthRef: "dev"},
		},
	}
	prettyTrue := true
	flags := config.OverrideFlags{
		Profile: "dev",
		Format:  "text",
		Pretty:  &prettyTrue,
	}
	// Env overrides should be ignored when CLI flags are set
	getenv := func(key string) string {
		m := map[string]string{
			"LOGVALET_PROFILE": "work",
			"LOGVALET_FORMAT":  "yaml",
		}
		return m[key]
	}
	resolved, err := config.Resolve(cfg, flags, getenv)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	// CLI flags win
	if resolved.Profile != "dev" {
		t.Errorf("resolved.Profile = %q, want dev (CLI flag wins)", resolved.Profile)
	}
	if resolved.Format != "text" {
		t.Errorf("resolved.Format = %q, want text (CLI flag wins)", resolved.Format)
	}
	if !resolved.Pretty {
		t.Error("resolved.Pretty = false, want true (CLI flag wins)")
	}
}

func TestResolve_ProfileNotFound_UsesEmpty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		DefaultProfile: "nonexistent",
	}
	resolved, err := config.Resolve(cfg, config.OverrideFlags{}, makeNoopGetenv())
	// プロファイルが見つからない場合は空のプロファイル設定でエラーなし
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if resolved.Profile != "nonexistent" {
		t.Errorf("resolved.Profile = %q, want nonexistent", resolved.Profile)
	}
	if resolved.Space != "" {
		t.Errorf("resolved.Space = %q, want empty for missing profile", resolved.Space)
	}
}

func TestResolve_EnvBoolInvalid(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	getenv := func(key string) string {
		if key == "LOGVALET_PRETTY" {
			return "invalid-bool"
		}
		return ""
	}
	_, err := config.Resolve(cfg, config.OverrideFlags{}, getenv)
	if err == nil {
		t.Error("Resolve with invalid bool env expected error, got nil")
	}
}

func TestResolve_EnvBaseURL(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	getenv := func(key string) string {
		if key == "LOGVALET_BASE_URL" {
			return "https://custom.backlog.com"
		}
		return ""
	}
	resolved, err := config.Resolve(cfg, config.OverrideFlags{}, getenv)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if resolved.BaseURL != "https://custom.backlog.com" {
		t.Errorf("resolved.BaseURL = %q, want https://custom.backlog.com", resolved.BaseURL)
	}
}

func TestResolve_EnvSpace(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	getenv := func(key string) string {
		if key == "LOGVALET_SPACE" {
			return "env-space"
		}
		return ""
	}
	resolved, err := config.Resolve(cfg, config.OverrideFlags{}, getenv)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if resolved.Space != "env-space" {
		t.Errorf("resolved.Space = %q, want env-space", resolved.Space)
	}
}

// ---- ProfileConfig.TeamID ----

func TestProfileConfig_TeamID(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "valid_with_team_id.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", path, err)
	}
	work, ok := cfg.Profiles["work"]
	if !ok {
		t.Fatal("cfg.Profiles[work] not found")
	}
	if work.TeamID != 173843 {
		t.Errorf("work.TeamID = %d, want 173843", work.TeamID)
	}
}

func TestProfileConfig_TeamID_Unspecified(t *testing.T) {
	t.Parallel()
	// valid.toml には team_id が指定されていない
	path := filepath.Join("testdata", "valid.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", path, err)
	}
	work, ok := cfg.Profiles["work"]
	if !ok {
		t.Fatal("cfg.Profiles[work] not found")
	}
	// TeamID が未指定の場合はゼロ値（0）になる
	if work.TeamID != 0 {
		t.Errorf("work.TeamID = %d, want 0 (zero value)", work.TeamID)
	}
}

// ---- DefaultLoader ----

func TestDefaultLoader_LoadAndResolve(t *testing.T) {
	path := filepath.Join("testdata", "valid.toml")
	loader := config.NewDefaultLoader()
	cfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("loader.Load error: %v", err)
	}
	resolved, err := loader.Resolve(cfg, config.OverrideFlags{}, os.Getenv)
	if err != nil {
		t.Fatalf("loader.Resolve error: %v", err)
	}
	if resolved.Profile != "work" {
		t.Errorf("resolved.Profile = %q, want work", resolved.Profile)
	}
}
