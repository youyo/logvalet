// Package cli_test — auth コマンドのテスト。
package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/cli"
	"github.com/youyo/logvalet/internal/credentials"
)

// ---- auth logout ----

func TestAuthLogoutCmd_Run_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")

	// 事前にトークンを作成
	tokens := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"work":    {AuthType: "oauth", AccessToken: "AT1"},
			"staging": {AuthType: "api_key", APIKey: "AK1"},
		},
	}
	store := credentials.NewStore(tokensPath)
	if err := store.Save(tokens); err != nil {
		t.Fatalf("setup: Save() error: %v", err)
	}

	// auth logout --profile work を実行
	cmd := &cli.AuthLogoutCmd{}
	g := &cli.GlobalFlags{Profile: "work"}
	err := cmd.RunWithStore(g, store)
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// tokens.json から "work" が削除されていることを確認
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after logout returned error: %v", err)
	}
	if _, ok := loaded.Auth["work"]; ok {
		t.Error("logout did not remove 'work' entry from tokens.json")
	}
	if _, ok := loaded.Auth["staging"]; !ok {
		t.Error("logout removed 'staging' entry unexpectedly")
	}
}

func TestAuthLogoutCmd_Run_ProfileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")
	store := credentials.NewStore(tokensPath)

	cmd := &cli.AuthLogoutCmd{}
	g := &cli.GlobalFlags{Profile: "nonexistent"}
	err := cmd.RunWithStore(g, store)
	if err == nil {
		t.Fatal("Run() returned nil error for nonexistent profile, want error")
	}
}

func TestAuthLogoutCmd_Run_NoProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))

	cmd := &cli.AuthLogoutCmd{}
	g := &cli.GlobalFlags{Profile: ""}
	err := cmd.RunWithStore(g, store)
	if err == nil {
		t.Fatal("Run() returned nil error when profile not specified, want error")
	}
}

// ---- auth list ----

func TestAuthListCmd_Run_WithEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")

	tokens := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"work": {AuthType: "oauth", AccessToken: "AT1"},
			"dev":  {AuthType: "api_key", APIKey: "AK1"},
		},
	}
	store := credentials.NewStore(tokensPath)
	if err := store.Save(tokens); err != nil {
		t.Fatalf("setup: Save() error: %v", err)
	}

	cmd := &cli.AuthListCmd{}
	g := &cli.GlobalFlags{}

	output, err := cmd.RunWithStoreCapture(g, store)
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// JSON をパースして検証
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Profiles      []struct {
			Profile       string `json:"profile"`
			AuthType      string `json:"auth_type"`
			Authenticated bool   `json:"authenticated"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput: %s", err, output)
	}
	if result.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", result.SchemaVersion, "1")
	}
	if len(result.Profiles) != 2 {
		t.Errorf("len(profiles) = %d, want 2", len(result.Profiles))
	}
	// 全エントリが authenticated: true であることを確認
	for _, p := range result.Profiles {
		if !p.Authenticated {
			t.Errorf("profile %q: authenticated = false, want true", p.Profile)
		}
	}
}

func TestAuthListCmd_Run_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))

	cmd := &cli.AuthListCmd{}
	g := &cli.GlobalFlags{}
	output, err := cmd.RunWithStoreCapture(g, store)
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	var result struct {
		SchemaVersion string        `json:"schema_version"`
		Profiles      []interface{} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput: %s", err, output)
	}
	if len(result.Profiles) != 0 {
		t.Errorf("len(profiles) = %d, want 0", len(result.Profiles))
	}
}

// ---- auth whoami ----

func TestAuthWhoamiCmd_Run_OAuth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	tokens := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"work": {
				AuthType:     "oauth",
				AccessToken:  "AT_WORK",
				RefreshToken: "RT_WORK",
				TokenExpiry:  future,
			},
		},
	}
	store := credentials.NewStore(tokensPath)
	if err := store.Save(tokens); err != nil {
		t.Fatalf("setup: Save() error: %v", err)
	}

	cmd := &cli.AuthWhoamiCmd{}
	g := &cli.GlobalFlags{Profile: "work"}
	output, err := cmd.RunWithStoreCapture(g, store, "work")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	var result struct {
		SchemaVersion string      `json:"schema_version"`
		Profile       string      `json:"profile"`
		AuthType      string      `json:"auth_type"`
		ExpiresAt     *string     `json:"expires_at"`
		Expired       bool        `json:"expired"`
		User          interface{} `json:"user"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput: %s", err, output)
	}
	if result.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", result.SchemaVersion, "1")
	}
	if result.AuthType != "oauth" {
		t.Errorf("auth_type = %q, want %q", result.AuthType, "oauth")
	}
	if result.Expired {
		t.Error("expired = true for future token, want false")
	}
	if result.User != nil {
		t.Errorf("user = %v, want null (M03 has no API call)", result.User)
	}
}

func TestAuthWhoamiCmd_Run_NoProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))

	cmd := &cli.AuthWhoamiCmd{}
	g := &cli.GlobalFlags{Profile: ""}
	_, err := cmd.RunWithStoreCapture(g, store, "")
	if err == nil {
		t.Fatal("Run() returned nil error when no profile specified, want error")
	}
}

func TestAuthWhoamiCmd_Run_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))

	cmd := &cli.AuthWhoamiCmd{}
	g := &cli.GlobalFlags{Profile: "nonexistent"}
	_, err := cmd.RunWithStoreCapture(g, store, "nonexistent-ref")
	if err == nil {
		t.Fatal("Run() returned nil error for nonexistent profile, want error")
	}
}

// ---- auth login (APIキー経由: OAuthなしで tokens.json に直接保存) ----

func TestAuthLoginCmd_SaveAPIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")
	store := credentials.NewStore(tokensPath)

	cmd := &cli.AuthLoginCmd{}
	g := &cli.GlobalFlags{Profile: "test-space"}

	loginReq := cli.AuthLoginRequest{
		AuthType: "api_key",
		APIKey:   "MY_API_KEY",
		Space:    "test-space",
		BaseURL:  "https://test-space.backlog.com",
	}

	output, err := cmd.RunWithLoginRequestCapture(g, store, loginReq)
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// JSON レスポンスの確認
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Result        string `json:"result"`
		Profile       string `json:"profile"`
		AuthType      string `json:"auth_type"`
		Saved         bool   `json:"saved"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput: %s", err, output)
	}
	if result.Result != "ok" {
		t.Errorf("result = %q, want %q", result.Result, "ok")
	}
	if !result.Saved {
		t.Error("saved = false, want true")
	}
	if result.AuthType != "api_key" {
		t.Errorf("auth_type = %q, want %q", result.AuthType, "api_key")
	}

	// tokens.json に保存されていることを確認
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after login returned error: %v", err)
	}
	entry, ok := loaded.Auth["test-space"]
	if !ok {
		t.Fatal("test-space entry not found in tokens.json after login")
	}
	if entry.APIKey != "MY_API_KEY" {
		t.Errorf("entry.APIKey = %q, want %q", entry.APIKey, "MY_API_KEY")
	}
}

func TestAuthLoginCmd_NoProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))

	cmd := &cli.AuthLoginCmd{}
	g := &cli.GlobalFlags{Profile: ""}
	loginReq := cli.AuthLoginRequest{
		AuthType: "api_key",
		APIKey:   "KEY",
	}
	_, err := cmd.RunWithLoginRequestCapture(g, store, loginReq)
	if err == nil {
		t.Fatal("Run() returned nil error when no profile specified, want error")
	}
}

// ---- output contains schema_version ----

func TestAuthOutput_HasSchemaVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")
	store := credentials.NewStore(tokensPath)

	// auth list で schema_version が含まれることを確認
	cmd := &cli.AuthListCmd{}
	g := &cli.GlobalFlags{}
	output, err := cmd.RunWithStoreCapture(g, store)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	data, err := os.ReadFile(tokensPath)
	// ファイルがなくても正常（空リスト）
	_ = data

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if _, ok := result["schema_version"]; !ok {
		t.Error("output does not contain 'schema_version' field")
	}
}
