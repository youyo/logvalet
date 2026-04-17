package auth

import (
	"strings"
	"testing"
)

// mockGetenv は環境変数のモックを作成するヘルパー。
func mockGetenv(envs map[string]string) func(string) string {
	return func(key string) string {
		return envs[key]
	}
}

// fullOAuthEnvs は OAuth が有効な全環境変数セットを返すヘルパー。
func fullOAuthEnvs() map[string]string {
	return map[string]string{
		"LOGVALET_MCP_TOKEN_STORE":                "memory",
		"LOGVALET_MCP_TOKEN_STORE_SQLITE_PATH":    "/tmp/test.db",
		"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE":  "test-table",
		"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION": "ap-northeast-1",
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":           "test-client-id",
		"LOGVALET_MCP_BACKLOG_CLIENT_SECRET":       "test-client-secret",
		"LOGVALET_MCP_BACKLOG_REDIRECT_URL":        "https://example.com/callback",
		"LOGVALET_MCP_OAUTH_STATE_SECRET":          "0123456789abcdef0123456789abcdef", // 32 hex chars = 16 bytes
	}
}

func TestLoadOAuthEnvConfig_Defaults(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TokenStoreType != StoreTypeMemory {
		t.Errorf("TokenStoreType = %q, want %q", cfg.TokenStoreType, StoreTypeMemory)
	}
	if cfg.SQLitePath != "./logvalet.db" {
		t.Errorf("SQLitePath = %q, want %q", cfg.SQLitePath, "./logvalet.db")
	}
	if cfg.DynamoDBTable != "" {
		t.Errorf("DynamoDBTable = %q, want empty", cfg.DynamoDBTable)
	}
	if cfg.DynamoDBRegion != "" {
		t.Errorf("DynamoDBRegion = %q, want empty", cfg.DynamoDBRegion)
	}
	if cfg.BacklogClientID != "" {
		t.Errorf("BacklogClientID = %q, want empty", cfg.BacklogClientID)
	}
	if cfg.BacklogClientSecret != "" {
		t.Errorf("BacklogClientSecret = %q, want empty", cfg.BacklogClientSecret)
	}
	if cfg.BacklogRedirectURL != "" {
		t.Errorf("BacklogRedirectURL = %q, want empty", cfg.BacklogRedirectURL)
	}
	if cfg.OAuthStateSecret != "" {
		t.Errorf("OAuthStateSecret = %q, want empty", cfg.OAuthStateSecret)
	}
}

func TestLoadOAuthEnvConfig_AllSet(t *testing.T) {
	envs := fullOAuthEnvs()
	cfg, err := LoadOAuthEnvConfig(mockGetenv(envs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TokenStoreType != StoreTypeMemory {
		t.Errorf("TokenStoreType = %q, want %q", cfg.TokenStoreType, StoreTypeMemory)
	}
	if cfg.SQLitePath != "/tmp/test.db" {
		t.Errorf("SQLitePath = %q, want %q", cfg.SQLitePath, "/tmp/test.db")
	}
	if cfg.DynamoDBTable != "test-table" {
		t.Errorf("DynamoDBTable = %q, want %q", cfg.DynamoDBTable, "test-table")
	}
	if cfg.DynamoDBRegion != "ap-northeast-1" {
		t.Errorf("DynamoDBRegion = %q, want %q", cfg.DynamoDBRegion, "ap-northeast-1")
	}
	if cfg.BacklogClientID != "test-client-id" {
		t.Errorf("BacklogClientID = %q, want %q", cfg.BacklogClientID, "test-client-id")
	}
	if cfg.BacklogClientSecret != "test-client-secret" {
		t.Errorf("BacklogClientSecret = %q, want %q", cfg.BacklogClientSecret, "test-client-secret")
	}
	if cfg.BacklogRedirectURL != "https://example.com/callback" {
		t.Errorf("BacklogRedirectURL = %q, want %q", cfg.BacklogRedirectURL, "https://example.com/callback")
	}
	if cfg.OAuthStateSecret != "0123456789abcdef0123456789abcdef" {
		t.Errorf("OAuthStateSecret = %q, want %q", cfg.OAuthStateSecret, "0123456789abcdef0123456789abcdef")
	}
}

func TestLoadOAuthEnvConfig_StoreTypeMemory(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "memory",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TokenStoreType != StoreTypeMemory {
		t.Errorf("TokenStoreType = %q, want %q", cfg.TokenStoreType, StoreTypeMemory)
	}
}

func TestLoadOAuthEnvConfig_StoreTypeSQLite(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "sqlite",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TokenStoreType != StoreTypeSQLite {
		t.Errorf("TokenStoreType = %q, want %q", cfg.TokenStoreType, StoreTypeSQLite)
	}
}

func TestLoadOAuthEnvConfig_StoreTypeDynamoDB(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "dynamodb",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TokenStoreType != StoreTypeDynamoDB {
		t.Errorf("TokenStoreType = %q, want %q", cfg.TokenStoreType, StoreTypeDynamoDB)
	}
}

func TestLoadOAuthEnvConfig_StoreTypeCaseInsensitive(t *testing.T) {
	cases := []struct {
		input string
		want  StoreType
	}{
		{"MEMORY", StoreTypeMemory},
		{"Memory", StoreTypeMemory},
		{"SQLITE", StoreTypeSQLite},
		{"Sqlite", StoreTypeSQLite},
		{"DYNAMODB", StoreTypeDynamoDB},
		{"DynamoDB", StoreTypeDynamoDB},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
				"LOGVALET_MCP_TOKEN_STORE": tc.input,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.TokenStoreType != tc.want {
				t.Errorf("TokenStoreType = %q, want %q", cfg.TokenStoreType, tc.want)
			}
		})
	}
}

func TestLoadOAuthEnvConfig_InvalidStoreType(t *testing.T) {
	_, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "redis",
	}))
	if err == nil {
		t.Fatal("expected error for invalid store type, got nil")
	}
	if !strings.Contains(err.Error(), "redis") {
		t.Errorf("error should contain invalid value 'redis': %v", err)
	}
}

func TestValidate_OAuthEnabled_AllRequired(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(fullOAuthEnvs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidate_OAuthEnabled_MissingClientID(t *testing.T) {
	// ClientID 未設定 = OAuth 無効 → Validate は成功
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "memory",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("OAuth disabled should not fail validation: %v", err)
	}
}

func TestValidate_OAuthEnabled_MissingClientSecret(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":  "test-id",
		"LOGVALET_MCP_OAUTH_STATE_SECRET": "0123456789abcdef0123456789abcdef",
		"LOGVALET_MCP_BACKLOG_REDIRECT_URL": "https://example.com/callback",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing client secret")
	}
	if !strings.Contains(err.Error(), EnvBacklogClientSecret) {
		t.Errorf("error should mention %s: %v", EnvBacklogClientSecret, err)
	}
}

func TestValidate_OAuthEnabled_MissingRedirectURL(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":     "test-id",
		"LOGVALET_MCP_BACKLOG_CLIENT_SECRET": "test-secret",
		"LOGVALET_MCP_OAUTH_STATE_SECRET":    "0123456789abcdef0123456789abcdef",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing redirect URL")
	}
	if !strings.Contains(err.Error(), EnvBacklogRedirectURL) {
		t.Errorf("error should mention %s: %v", EnvBacklogRedirectURL, err)
	}
}

func TestValidate_OAuthEnabled_MissingStateSecret(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":     "test-id",
		"LOGVALET_MCP_BACKLOG_CLIENT_SECRET": "test-secret",
		"LOGVALET_MCP_BACKLOG_REDIRECT_URL":  "https://example.com/callback",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing state secret")
	}
	if !strings.Contains(err.Error(), EnvOAuthStateSecret) {
		t.Errorf("error should mention %s: %v", EnvOAuthStateSecret, err)
	}
}

func TestValidate_SQLite_MissingSQLitePath(t *testing.T) {
	// SQLite 選択時に Path 未設定でもデフォルト値があるので成功
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "sqlite",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("sqlite with default path should not fail: %v", err)
	}
	if cfg.SQLitePath != "./logvalet.db" {
		t.Errorf("SQLitePath = %q, want %q", cfg.SQLitePath, "./logvalet.db")
	}
}

func TestValidate_DynamoDB_MissingTable(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE":                "dynamodb",
		"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION": "ap-northeast-1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing dynamodb table")
	}
	if !strings.Contains(err.Error(), EnvDynamoDBTable) {
		t.Errorf("error should mention %s: %v", EnvDynamoDBTable, err)
	}
}

func TestValidate_DynamoDB_MissingRegion(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE":               "dynamodb",
		"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE": "test-table",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing dynamodb region")
	}
	if !strings.Contains(err.Error(), EnvDynamoDBRegion) {
		t.Errorf("error should mention %s: %v", EnvDynamoDBRegion, err)
	}
}

func TestValidate_DynamoDB_AllSet(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE":                "dynamodb",
		"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE":  "test-table",
		"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION": "ap-northeast-1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("dynamodb with all set should not fail: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	// OAuth 有効 + DynamoDB だが、必須項目が複数欠けている
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE":       "dynamodb",
		"LOGVALET_MCP_BACKLOG_CLIENT_ID": "test-id",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected multiple validation errors")
	}
	errStr := err.Error()
	// OAuth 必須項目: client_secret, redirect_url, state_secret
	// DynamoDB 必須項目: table, region
	expectedMentions := []string{
		EnvBacklogClientSecret,
		EnvBacklogRedirectURL,
		EnvOAuthStateSecret,
		EnvDynamoDBTable,
		EnvDynamoDBRegion,
	}
	for _, mention := range expectedMentions {
		if !strings.Contains(errStr, mention) {
			t.Errorf("error should mention %s: %v", mention, err)
		}
	}
}

func TestOAuthEnabled_True(t *testing.T) {
	cfg := &OAuthEnvConfig{
		BacklogClientID: "test-id",
	}
	if !cfg.OAuthEnabled() {
		t.Error("OAuthEnabled() = false, want true when ClientID is set")
	}
}

func TestOAuthEnabled_False(t *testing.T) {
	cfg := &OAuthEnvConfig{}
	if cfg.OAuthEnabled() {
		t.Error("OAuthEnabled() = true, want false when ClientID is empty")
	}
}

func TestLoadOAuthEnvConfig_SQLiteDefaultPath(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_TOKEN_STORE": "sqlite",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SQLitePath != "./logvalet.db" {
		t.Errorf("SQLitePath = %q, want %q", cfg.SQLitePath, "./logvalet.db")
	}
}

func TestStoreType_String(t *testing.T) {
	cases := []struct {
		st   StoreType
		want string
	}{
		{StoreTypeMemory, "memory"},
		{StoreTypeSQLite, "sqlite"},
		{StoreTypeDynamoDB, "dynamodb"},
	}
	for _, tc := range cases {
		if got := string(tc.st); got != tc.want {
			t.Errorf("StoreType(%q) string = %q, want %q", tc.st, got, tc.want)
		}
	}
}

func TestValidate_OAuthEnabled_InvalidHexStateSecret(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":     "test-id",
		"LOGVALET_MCP_BACKLOG_CLIENT_SECRET": "test-secret",
		"LOGVALET_MCP_BACKLOG_REDIRECT_URL":  "https://example.com/callback",
		"LOGVALET_MCP_OAUTH_STATE_SECRET":    "not-valid-hex-string-!@#$%^&*()",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid hex state secret")
	}
	if !strings.Contains(err.Error(), "hex") {
		t.Errorf("error should mention hex: %v", err)
	}
}

func TestValidate_OAuthEnabled_ValidHexStateSecret(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":     "test-id",
		"LOGVALET_MCP_BACKLOG_CLIENT_SECRET": "test-secret",
		"LOGVALET_MCP_BACKLOG_REDIRECT_URL":  "https://example.com/callback",
		"LOGVALET_MCP_OAUTH_STATE_SECRET":    "0123456789abcdef0123456789abcdef",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid hex state secret should pass: %v", err)
	}
}

func TestValidate_OAuthEnabled_TooShortStateSecret(t *testing.T) {
	cfg, err := LoadOAuthEnvConfig(mockGetenv(map[string]string{
		"LOGVALET_MCP_BACKLOG_CLIENT_ID":     "test-id",
		"LOGVALET_MCP_BACKLOG_CLIENT_SECRET": "test-secret",
		"LOGVALET_MCP_BACKLOG_REDIRECT_URL":  "https://example.com/callback",
		"LOGVALET_MCP_OAUTH_STATE_SECRET":    "0123456789abcdef", // 16 hex chars = 8 bytes, too short
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for too short state secret")
	}
	if !strings.Contains(err.Error(), "16 bytes") {
		t.Errorf("error should mention minimum 16 bytes: %v", err)
	}
}
