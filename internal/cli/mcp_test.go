package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

func TestMcpCmd_Validate_AuthRequiresFields(t *testing.T) {
	cmd := &cli.McpCmd{Auth: true}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error when Auth=true with empty required fields")
	}
}

func TestMcpCmd_Validate_NoAuthSkipsValidation(t *testing.T) {
	cmd := &cli.McpCmd{Auth: false}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMcpCmd_Validate_CookieSecretTooShort(t *testing.T) {
	cmd := &cli.McpCmd{
		Auth:         true,
		ExternalURL:  "https://example.com",
		OIDCIssuer:   "https://accounts.google.com",
		OIDCClientID: "client-id",
		CookieSecret: strings.Repeat("ab", 16), // 32 hex chars = 16 bytes
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for cookie secret shorter than 32 bytes")
	}
}

func TestMcpCmd_Validate_ValidAuth(t *testing.T) {
	cmd := &cli.McpCmd{
		Auth:         true,
		ExternalURL:  "https://example.com",
		OIDCIssuer:   "https://accounts.google.com",
		OIDCClientID: "client-id",
		CookieSecret: strings.Repeat("ab", 32), // 64 hex chars = 32 bytes
	}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMcpCmd_Validate_InvalidHexCookieSecret(t *testing.T) {
	cmd := &cli.McpCmd{
		Auth:         true,
		ExternalURL:  "https://example.com",
		OIDCIssuer:   "https://accounts.google.com",
		OIDCClientID: "client-id",
		CookieSecret: "ZZZZ",
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

// BacklogClientID フィールドベースのバリデーションテスト（新設計）
func TestMcpCmd_Validate_BacklogClientIDWithoutAuth_Fails(t *testing.T) {
	cmd := &cli.McpCmd{
		Auth:            false,
		BacklogClientID: "some-client-id",
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error when BacklogClientID is set but --auth is disabled")
	}
	if !strings.Contains(err.Error(), "--backlog-client-id") {
		t.Fatalf("error message should mention --backlog-client-id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--auth") {
		t.Fatalf("error message should mention --auth, got: %v", err)
	}
}

func TestMcpCmd_Validate_BacklogClientIDWithAuth_OK(t *testing.T) {
	// --auth 有効 + BacklogClientID 設定 → Validate 自体は通る（OIDC 必須チェックのみ失敗）
	cmd := &cli.McpCmd{
		Auth:            true,
		BacklogClientID: "some-client-id",
	}
	err := cmd.Validate()
	// OIDC 必須フィールドが無いのでエラーになるが、BacklogClientID の fast-fail ではないこと
	if err != nil && strings.Contains(err.Error(), "--backlog-client-id") {
		t.Fatalf("should not get backlog-client-id error when --auth is enabled, got: %v", err)
	}
}

func TestMcpCmd_Validate_NoBacklogClientID_NoAuthRequired(t *testing.T) {
	cmd := &cli.McpCmd{Auth: false}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error when no BacklogClientID and no auth: %v", err)
	}
}

func TestMcpCmd_Validate_IDProxyStoreDynamoDB_RequiresTable(t *testing.T) {
	cmd := &cli.McpCmd{
		IDProxyStore: "dynamodb",
		SigningKey:   "dummy-pem",
	}
	err := cmd.Validate()
	if err == nil || !strings.Contains(err.Error(), "idproxy-store-dynamodb-table") {
		t.Fatalf("expected table-required error, got: %v", err)
	}
}

func TestMcpCmd_Validate_IDProxyStoreDynamoDB_RequiresSigningKey(t *testing.T) {
	cmd := &cli.McpCmd{
		IDProxyStore:              "dynamodb",
		IDProxyStoreDynamoDBTable: "tbl",
	}
	err := cmd.Validate()
	if err == nil || !strings.Contains(err.Error(), "signing-key") {
		t.Fatalf("expected signing-key-required error, got: %v", err)
	}
}

func TestMcpCmd_Validate_IDProxyStore_InvalidValue(t *testing.T) {
	cmd := &cli.McpCmd{IDProxyStore: "redis"}
	err := cmd.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid --idproxy-store") {
		t.Fatalf("expected invalid-store error, got: %v", err)
	}
}

func TestMcpCmd_Validate_IDProxyStoreMemory_OK(t *testing.T) {
	cmd := &cli.McpCmd{IDProxyStore: "memory"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMcpCmd_Validate_IDProxyStoreEmpty_OK(t *testing.T) {
	cmd := &cli.McpCmd{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
