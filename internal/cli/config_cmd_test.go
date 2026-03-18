package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
	"github.com/youyo/logvalet/internal/config"
	"github.com/youyo/logvalet/internal/credentials"
)

// mockPrompter はテスト用の Prompter 実装。
type mockPrompter struct {
	responses []string // Prompt の応答キュー
	confirms  []bool   // Confirm の応答キュー
	idx       int
	cidx      int
}

func (m *mockPrompter) Prompt(label, defaultValue string) (string, error) {
	if m.idx >= len(m.responses) {
		if defaultValue != "" {
			return defaultValue, nil
		}
		return "", nil
	}
	resp := m.responses[m.idx]
	m.idx++
	if resp == "" && defaultValue != "" {
		return defaultValue, nil
	}
	return resp, nil
}

func (m *mockPrompter) Confirm(label string, defaultYes bool) (bool, error) {
	if m.cidx >= len(m.confirms) {
		return defaultYes, nil
	}
	resp := m.confirms[m.cidx]
	m.cidx++
	return resp, nil
}

func TestConfigInit_AllFlags_NewProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "work", "example-space", "https://example-space.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	// stdout に JSON レスポンスが出力される
	var resp map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout: %s", err, stdout.String())
	}
	if resp["result"] != "ok" {
		t.Errorf("result = %v, want ok", resp["result"])
	}
	if resp["profile"] != "work" {
		t.Errorf("profile = %v, want work", resp["profile"])
	}
	if resp["space"] != "example-space" {
		t.Errorf("space = %v, want example-space", resp["space"])
	}
	if resp["created"] != true {
		t.Errorf("created = %v, want true", resp["created"])
	}

	// config.toml が作成された
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	p, ok := loaded.Profiles["work"]
	if !ok {
		t.Fatal("profile 'work' not found in config.toml")
	}
	if p.Space != "example-space" {
		t.Errorf("p.Space = %q, want example-space", p.Space)
	}
	if p.AuthRef != "work" {
		t.Errorf("p.AuthRef = %q, want work (same as profile name)", p.AuthRef)
	}
}

func TestConfigInit_AllFlags_ExistingProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// 既存の config.toml を作成
	initial := &config.Config{
		Version:        1,
		DefaultProfile: "work",
		DefaultFormat:  "json",
		Profiles: map[string]config.ProfileConfig{
			"work": {Space: "old-space", BaseURL: "https://old-space.backlog.com", AuthRef: "work"},
		},
	}
	w := config.NewWriter()
	if err := w.Write(configPath, initial); err != nil {
		t.Fatalf("initial write error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     w,
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{}, // 非対話
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	// 非対話モード: 確認なしに上書き
	err := cmd.RunWithDeps(deps, "work", "new-space", "https://new-space.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	// 上書きされた
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	p := loaded.Profiles["work"]
	if p.Space != "new-space" {
		t.Errorf("p.Space = %q, want new-space (overwritten)", p.Space)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// 既存プロファイル上書きなので created = false
	if resp["created"] != false {
		t.Errorf("created = %v, want false for overwrite", resp["created"])
	}
}

func TestConfigInit_Interactive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	prompter := &mockPrompter{
		responses: []string{"myprofile", "my-space", ""}, // profile, space, base_url (empty = use default)
	}

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   prompter,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "", "", "", "") // 全て空 = 対話モード
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	p, ok := loaded.Profiles["myprofile"]
	if !ok {
		t.Fatal("profile 'myprofile' not found")
	}
	if p.Space != "my-space" {
		t.Errorf("p.Space = %q, want my-space", p.Space)
	}
	// base_url は space から自動生成
	if p.BaseURL != "https://my-space.backlog.com" {
		t.Errorf("p.BaseURL = %q, want https://my-space.backlog.com", p.BaseURL)
	}
}

func TestConfigInit_Interactive_ExistingProfile_OverwriteYes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// 既存ファイル
	initial := &config.Config{
		Version:        1,
		DefaultProfile: "work",
		Profiles: map[string]config.ProfileConfig{
			"work": {Space: "old", BaseURL: "https://old.backlog.com", AuthRef: "work"},
		},
	}
	w := config.NewWriter()
	if err := w.Write(configPath, initial); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	var stdout, stderr bytes.Buffer
	prompter := &mockPrompter{
		responses: []string{"work", "new-space", ""},
		confirms:  []bool{true}, // 上書き確認 Yes
	}

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     w,
		Loader:     config.NewDefaultLoader(),
		Prompter:   prompter,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "", "", "", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	loaded, _ := config.Load(configPath)
	if loaded.Profiles["work"].Space != "new-space" {
		t.Errorf("Space = %q, want new-space (overwritten)", loaded.Profiles["work"].Space)
	}
}

func TestConfigInit_Interactive_ExistingProfile_OverwriteNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	initial := &config.Config{
		Version:        1,
		DefaultProfile: "work",
		Profiles: map[string]config.ProfileConfig{
			"work": {Space: "old", BaseURL: "https://old.backlog.com", AuthRef: "work"},
		},
	}
	w := config.NewWriter()
	if err := w.Write(configPath, initial); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	var stdout, stderr bytes.Buffer
	prompter := &mockPrompter{
		responses: []string{"work", "new-space", ""},
		confirms:  []bool{false}, // 上書き確認 No
	}

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     w,
		Loader:     config.NewDefaultLoader(),
		Prompter:   prompter,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "", "", "", "")
	if err == nil {
		t.Fatal("RunWithDeps() should error when overwrite denied")
	}

	// 元のプロファイルが保持されている
	loaded, _ := config.Load(configPath)
	if loaded.Profiles["work"].Space != "old" {
		t.Errorf("Space = %q, want old (not overwritten)", loaded.Profiles["work"].Space)
	}
}

func TestConfigInit_DefaultBaseURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	// base_url 空 → space から自動生成
	err := cmd.RunWithDeps(deps, "test", "my-space", "", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	loaded, _ := config.Load(configPath)
	p := loaded.Profiles["test"]
	if p.BaseURL != "https://my-space.backlog.com" {
		t.Errorf("BaseURL = %q, want https://my-space.backlog.com", p.BaseURL)
	}
}

func TestConfigInit_AuthRef_AutoSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "myprof", "space1", "https://space1.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	loaded, _ := config.Load(configPath)
	if loaded.Profiles["myprof"].AuthRef != "myprof" {
		t.Errorf("AuthRef = %q, want myprof", loaded.Profiles["myprof"].AuthRef)
	}
}

func TestConfigInit_DefaultProfile_AutoSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "first", "s", "https://s.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	loaded, _ := config.Load(configPath)
	if loaded.DefaultProfile != "first" {
		t.Errorf("DefaultProfile = %q, want first", loaded.DefaultProfile)
	}

	// 2回目: default_profile は変更されない
	stdout.Reset()
	stderr.Reset()
	deps.Stdout = &stdout
	deps.Stderr = &stderr
	err = cmd.RunWithDeps(deps, "second", "s2", "https://s2.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() 2nd error: %v", err)
	}

	loaded2, _ := config.Load(configPath)
	if loaded2.DefaultProfile != "first" {
		t.Errorf("DefaultProfile = %q, want first (unchanged)", loaded2.DefaultProfile)
	}
}

func TestConfigInit_OutputJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "work", "example", "https://example.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	var resp struct {
		SchemaVersion string `json:"schema_version"`
		Result        string `json:"result"`
		Profile       string `json:"profile"`
		Space         string `json:"space"`
		BaseURL       string `json:"base_url"`
		ConfigPath    string `json:"config_path"`
		Created       bool   `json:"created"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse error: %v\nstdout: %s", err, stdout.String())
	}
	if resp.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want 1", resp.SchemaVersion)
	}
	if resp.Result != "ok" {
		t.Errorf("result = %q, want ok", resp.Result)
	}
	if resp.ConfigPath != configPath {
		t.Errorf("config_path = %q, want %q", resp.ConfigPath, configPath)
	}
}

func TestConfigInit_StderrGuidance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "work", "example", "https://example.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	stderrStr := stderr.String()
	if !bytes.Contains(stderr.Bytes(), []byte("auth login")) {
		t.Errorf("stderr should contain 'auth login' guidance, got: %s", stderrStr)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("--profile work")) {
		t.Errorf("stderr should contain '--profile work', got: %s", stderrStr)
	}
}

func TestConfigInit_WithAPIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokensPath := filepath.Join(dir, "tokens.json")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
		CredStore:  credentials.NewStore(tokensPath),
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "work", "example", "https://example.backlog.com", "MY_API_KEY")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	// config.toml が作成された
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := loaded.Profiles["work"]; !ok {
		t.Fatal("profile 'work' not found in config.toml")
	}

	// tokens.json に保存された
	store := credentials.NewStore(tokensPath)
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("tokens Load() error: %v", err)
	}
	entry, ok := tokens.Auth["work"]
	if !ok {
		t.Fatal("work entry not found in tokens.json")
	}
	if entry.AuthType != "api_key" {
		t.Errorf("AuthType = %q, want api_key", entry.AuthType)
	}
	if entry.APIKey != "MY_API_KEY" {
		t.Errorf("APIKey = %q, want MY_API_KEY", entry.APIKey)
	}

	// JSON レスポンスに auth_saved: true
	var resp map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp["auth_saved"] != true {
		t.Errorf("auth_saved = %v, want true", resp["auth_saved"])
	}
}

func TestConfigInit_WithAPIKey_Interactive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokensPath := filepath.Join(dir, "tokens.json")
	var stdout, stderr bytes.Buffer

	prompter := &mockPrompter{
		responses: []string{"myprofile", "my-space", "", "MY_INTERACTIVE_KEY"}, // profile, space, base_url, api_key
	}

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   prompter,
		Stdout:     &stdout,
		Stderr:     &stderr,
		CredStore:  credentials.NewStore(tokensPath),
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "", "", "", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	// tokens.json にエントリ存在
	store := credentials.NewStore(tokensPath)
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("tokens Load() error: %v", err)
	}
	entry, ok := tokens.Auth["myprofile"]
	if !ok {
		t.Fatal("myprofile entry not found in tokens.json")
	}
	if entry.APIKey != "MY_INTERACTIVE_KEY" {
		t.Errorf("APIKey = %q, want MY_INTERACTIVE_KEY", entry.APIKey)
	}
}

func TestConfigInit_WithoutAPIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokensPath := filepath.Join(dir, "tokens.json")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
		CredStore:  credentials.NewStore(tokensPath),
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "work", "example", "https://example.backlog.com", "")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	// tokens.json は空（エントリなし）
	store := credentials.NewStore(tokensPath)
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("tokens Load() error: %v", err)
	}
	if len(tokens.Auth) != 0 {
		t.Errorf("tokens.Auth should be empty, got %d entries", len(tokens.Auth))
	}

	// auth_saved: false
	var resp map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp["auth_saved"] != false {
		t.Errorf("auth_saved = %v, want false", resp["auth_saved"])
	}
}

func TestConfigInit_StderrComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokensPath := filepath.Join(dir, "tokens.json")
	var stdout, stderr bytes.Buffer

	deps := cli.ConfigInitDeps{
		ConfigPath: configPath,
		Writer:     config.NewWriter(),
		Loader:     config.NewDefaultLoader(),
		Prompter:   &mockPrompter{},
		Stdout:     &stdout,
		Stderr:     &stderr,
		CredStore:  credentials.NewStore(tokensPath),
	}

	cmd := cli.ConfigInitCmd{}
	err := cmd.RunWithDeps(deps, "work", "example", "https://example.backlog.com", "MY_KEY")
	if err != nil {
		t.Fatalf("RunWithDeps() error: %v", err)
	}

	stderrStr := stderr.String()
	if !bytes.Contains(stderr.Bytes(), []byte("セットアップ完了")) {
		t.Errorf("stderr should contain setup complete message, got: %s", stderrStr)
	}
	if bytes.Contains(stderr.Bytes(), []byte("auth login")) {
		t.Errorf("stderr should NOT contain 'auth login' when API key saved, got: %s", stderrStr)
	}
}
