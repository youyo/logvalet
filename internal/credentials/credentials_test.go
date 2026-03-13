// Package credentials_test は credentials パッケージのテスト。
package credentials_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/credentials"
)

// ---- DefaultTokensPath ----

func TestDefaultTokensPath_WithXDGConfigHome(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	got := credentials.DefaultTokensPath(func(key string) string {
		if key == "XDG_CONFIG_HOME" {
			return tmpDir
		}
		return ""
	})
	want := filepath.Join(tmpDir, "logvalet", "tokens.json")
	if got != want {
		t.Errorf("DefaultTokensPath() = %q, want %q", got, want)
	}
}

func TestDefaultTokensPath_WithHomeDir(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	got := credentials.DefaultTokensPath(func(key string) string {
		if key == "HOME" {
			return home
		}
		return ""
	})
	want := filepath.Join(home, ".config", "logvalet", "tokens.json")
	if got != want {
		t.Errorf("DefaultTokensPath() = %q, want %q", got, want)
	}
}

// ---- Store: Load ----

func TestStore_Load_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "nonexistent.json"))
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if tokens == nil {
		t.Fatal("Load() returned nil TokensFile, want zero value")
	}
	if tokens.Version != 0 {
		t.Errorf("tokens.Version = %d, want 0", tokens.Version)
	}
	if tokens.Auth != nil && len(tokens.Auth) != 0 {
		t.Errorf("tokens.Auth = %v, want empty/nil", tokens.Auth)
	}
}

func TestStore_Load_Valid(t *testing.T) {
	t.Parallel()
	store := credentials.NewStore("testdata/tokens_valid.json")
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if tokens.Version != 1 {
		t.Errorf("tokens.Version = %d, want 1", tokens.Version)
	}
	if len(tokens.Auth) != 2 {
		t.Errorf("len(tokens.Auth) = %d, want 2", len(tokens.Auth))
	}
	entry, ok := tokens.Auth["example-space"]
	if !ok {
		t.Fatal("tokens.Auth[\"example-space\"] not found")
	}
	if entry.AuthType != "oauth" {
		t.Errorf("entry.AuthType = %q, want %q", entry.AuthType, "oauth")
	}
	if entry.AccessToken != "ACCESS_TOKEN_VALID" {
		t.Errorf("entry.AccessToken = %q, want %q", entry.AccessToken, "ACCESS_TOKEN_VALID")
	}
	devEntry, ok := tokens.Auth["example-dev"]
	if !ok {
		t.Fatal("tokens.Auth[\"example-dev\"] not found")
	}
	if devEntry.AuthType != "api_key" {
		t.Errorf("devEntry.AuthType = %q, want %q", devEntry.AuthType, "api_key")
	}
	if devEntry.APIKey != "API_KEY_VALID" {
		t.Errorf("devEntry.APIKey = %q, want %q", devEntry.APIKey, "API_KEY_VALID")
	}
}

func TestStore_Load_Invalid(t *testing.T) {
	t.Parallel()
	store := credentials.NewStore("testdata/tokens_invalid.json")
	_, err := store.Load()
	if err == nil {
		t.Fatal("Load() returned nil error for invalid JSON, want error")
	}
}

// ---- Store: Save ----

func TestStore_Save_CreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "tokens.json")
	store := credentials.NewStore(path)
	tokens := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"test-space": {
				AuthType:    "api_key",
				APIKey:      "TEST_KEY",
			},
		},
	}
	if err := store.Save(tokens); err != nil {
		t.Fatalf("Save() returned unexpected error: %v", err)
	}
	// ファイルが存在することを確認
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Save() did not create file: %v", err)
	}
	// パーミッションが 0600 であることを確認
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() returned error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permission = %o, want %o", perm, 0600)
	}
	// 内容が正しいことを確認
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after Save() returned error: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("loaded.Version = %d, want 1", loaded.Version)
	}
	entry, ok := loaded.Auth["test-space"]
	if !ok {
		t.Fatal("loaded.Auth[\"test-space\"] not found after Save")
	}
	if entry.APIKey != "TEST_KEY" {
		t.Errorf("entry.APIKey = %q, want %q", entry.APIKey, "TEST_KEY")
	}
}

func TestStore_Save_AtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	store := credentials.NewStore(path)

	// 最初のバージョンを保存
	tokens1 := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"space1": {AuthType: "api_key", APIKey: "KEY1"},
		},
	}
	if err := store.Save(tokens1); err != nil {
		t.Fatalf("Save() first returned error: %v", err)
	}

	// 2回目の保存（上書き）
	tokens2 := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"space2": {AuthType: "oauth", AccessToken: "TOKEN2"},
		},
	}
	if err := store.Save(tokens2); err != nil {
		t.Fatalf("Save() second returned error: %v", err)
	}

	// 2回目の内容だけが残っていることを確認
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after second Save() returned error: %v", err)
	}
	if _, ok := loaded.Auth["space1"]; ok {
		t.Error("space1 should not exist after second Save")
	}
	if _, ok := loaded.Auth["space2"]; !ok {
		t.Error("space2 should exist after second Save")
	}
}

// ---- Resolver ----

func TestResolver_Resolve_FlagAPIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{
		APIKey: "FLAG_API_KEY",
	}
	cred, err := resolver.Resolve("some-ref", flags, func(string) string { return "" })
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.APIKey != "FLAG_API_KEY" {
		t.Errorf("cred.APIKey = %q, want %q", cred.APIKey, "FLAG_API_KEY")
	}
	if cred.Source != "flag" {
		t.Errorf("cred.Source = %q, want %q", cred.Source, "flag")
	}
	if cred.AuthType != "api_key" {
		t.Errorf("cred.AuthType = %q, want %q", cred.AuthType, "api_key")
	}
}

func TestResolver_Resolve_FlagAccessToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{
		AccessToken: "FLAG_ACCESS_TOKEN",
	}
	cred, err := resolver.Resolve("some-ref", flags, func(string) string { return "" })
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.AccessToken != "FLAG_ACCESS_TOKEN" {
		t.Errorf("cred.AccessToken = %q, want %q", cred.AccessToken, "FLAG_ACCESS_TOKEN")
	}
	if cred.Source != "flag" {
		t.Errorf("cred.Source = %q, want %q", cred.Source, "flag")
	}
	if cred.AuthType != "oauth" {
		t.Errorf("cred.AuthType = %q, want %q", cred.AuthType, "oauth")
	}
}

func TestResolver_Resolve_FlagAPIKeyTakesPrecedenceOverAccessToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{
		APIKey:      "FLAG_API_KEY",
		AccessToken: "FLAG_ACCESS_TOKEN",
	}
	cred, err := resolver.Resolve("some-ref", flags, func(string) string { return "" })
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.APIKey != "FLAG_API_KEY" {
		t.Errorf("cred.APIKey = %q, want %q", cred.APIKey, "FLAG_API_KEY")
	}
	if cred.AuthType != "api_key" {
		t.Errorf("cred.AuthType = %q, want %q", cred.AuthType, "api_key")
	}
}

func TestResolver_Resolve_EnvAPIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{}
	getenv := func(key string) string {
		if key == "LOGVALET_API_KEY" {
			return "ENV_API_KEY"
		}
		return ""
	}
	cred, err := resolver.Resolve("some-ref", flags, getenv)
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.APIKey != "ENV_API_KEY" {
		t.Errorf("cred.APIKey = %q, want %q", cred.APIKey, "ENV_API_KEY")
	}
	if cred.Source != "env" {
		t.Errorf("cred.Source = %q, want %q", cred.Source, "env")
	}
}

func TestResolver_Resolve_EnvAccessToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{}
	getenv := func(key string) string {
		if key == "LOGVALET_ACCESS_TOKEN" {
			return "ENV_ACCESS_TOKEN"
		}
		return ""
	}
	cred, err := resolver.Resolve("some-ref", flags, getenv)
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.AccessToken != "ENV_ACCESS_TOKEN" {
		t.Errorf("cred.AccessToken = %q, want %q", cred.AccessToken, "ENV_ACCESS_TOKEN")
	}
	if cred.Source != "env" {
		t.Errorf("cred.Source = %q, want %q", cred.Source, "env")
	}
}

func TestResolver_Resolve_TokensJSON_OAuth(t *testing.T) {
	t.Parallel()
	store := credentials.NewStore("testdata/tokens_valid.json")
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{}
	cred, err := resolver.Resolve("example-space", flags, func(string) string { return "" })
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.AccessToken != "ACCESS_TOKEN_VALID" {
		t.Errorf("cred.AccessToken = %q, want %q", cred.AccessToken, "ACCESS_TOKEN_VALID")
	}
	if cred.Source != "tokens_json" {
		t.Errorf("cred.Source = %q, want %q", cred.Source, "tokens_json")
	}
	if cred.AuthType != "oauth" {
		t.Errorf("cred.AuthType = %q, want %q", cred.AuthType, "oauth")
	}
}

func TestResolver_Resolve_TokensJSON_APIKey(t *testing.T) {
	t.Parallel()
	store := credentials.NewStore("testdata/tokens_valid.json")
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{}
	cred, err := resolver.Resolve("example-dev", flags, func(string) string { return "" })
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if cred.APIKey != "API_KEY_VALID" {
		t.Errorf("cred.APIKey = %q, want %q", cred.APIKey, "API_KEY_VALID")
	}
	if cred.Source != "tokens_json" {
		t.Errorf("cred.Source = %q, want %q", cred.Source, "tokens_json")
	}
	if cred.AuthType != "api_key" {
		t.Errorf("cred.AuthType = %q, want %q", cred.AuthType, "api_key")
	}
}

func TestResolver_Resolve_NoCredentials(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{}
	_, err := resolver.Resolve("nonexistent-ref", flags, func(string) string { return "" })
	if err == nil {
		t.Fatal("Resolve() returned nil error when no credentials found, want error")
	}
}

func TestResolver_Resolve_EmptyAuthRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := credentials.NewStore(filepath.Join(dir, "tokens.json"))
	resolver := credentials.NewResolver(store)
	flags := credentials.CredentialFlags{}
	_, err := resolver.Resolve("", flags, func(string) string { return "" })
	if err == nil {
		t.Fatal("Resolve() returned nil error when authRef is empty and no creds, want error")
	}
}

// ---- IsExpired ----

func TestAuthEntry_IsExpired_NotOAuth(t *testing.T) {
	t.Parallel()
	entry := credentials.AuthEntry{
		AuthType: "api_key",
		APIKey:   "KEY",
	}
	if entry.IsExpired() {
		t.Error("IsExpired() = true for api_key entry, want false")
	}
}

func TestAuthEntry_IsExpired_NoExpiry(t *testing.T) {
	t.Parallel()
	entry := credentials.AuthEntry{
		AuthType:    "oauth",
		AccessToken: "TOKEN",
	}
	if entry.IsExpired() {
		t.Error("IsExpired() = true for oauth entry with no expiry, want false")
	}
}

func TestAuthEntry_IsExpired_Expired(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	entry := credentials.AuthEntry{
		AuthType:    "oauth",
		AccessToken: "TOKEN",
		TokenExpiry: past,
	}
	if !entry.IsExpired() {
		t.Error("IsExpired() = false for past expiry, want true")
	}
}

func TestAuthEntry_IsExpired_NotExpired(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	entry := credentials.AuthEntry{
		AuthType:    "oauth",
		AccessToken: "TOKEN",
		TokenExpiry: future,
	}
	if entry.IsExpired() {
		t.Error("IsExpired() = true for future expiry, want false")
	}
}

// ---- TokensFile JSON round-trip ----

func TestTokensFile_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := &credentials.TokensFile{
		Version: 1,
		Auth: map[string]credentials.AuthEntry{
			"space1": {
				AuthType:     "oauth",
				AccessToken:  "AT",
				RefreshToken: "RT",
				TokenExpiry:  "2099-12-31T23:59:59Z",
			},
			"space2": {
				AuthType: "api_key",
				APIKey:   "AK",
			},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}
	var decoded credentials.TokensFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}
	if decoded.Version != original.Version {
		t.Errorf("decoded.Version = %d, want %d", decoded.Version, original.Version)
	}
	if len(decoded.Auth) != len(original.Auth) {
		t.Errorf("len(decoded.Auth) = %d, want %d", len(decoded.Auth), len(original.Auth))
	}
}
