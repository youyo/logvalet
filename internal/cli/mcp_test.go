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
