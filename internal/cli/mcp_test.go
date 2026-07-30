package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

// removedFlagCases は削除済み認証フラグの fail-fast を検証するテーブル。
// いずれも「AgentCore Gateway に委譲された」旨のエラーになる必要がある。
func TestMcpCmd_Validate_RemovedFlags_FailFast(t *testing.T) {
	cases := []struct {
		name     string
		cmd      *cli.McpCmd
		wantFlag string
	}{
		{"auth", &cli.McpCmd{RemovedAuth: true}, "--auth"},
		{"external-url", &cli.McpCmd{RemovedExternalURL: "https://example.com"}, "--external-url"},
		{"oidc-issuer", &cli.McpCmd{RemovedOIDCIssuer: "https://accounts.google.com"}, "--oidc-issuer"},
		{"oidc-client-id", &cli.McpCmd{RemovedOIDCClientID: "cid"}, "--oidc-client-id"},
		{"oidc-client-secret", &cli.McpCmd{RemovedOIDCClientSecret: "sec"}, "--oidc-client-secret"},
		{"cookie-secret", &cli.McpCmd{RemovedCookieSecret: strings.Repeat("ab", 32)}, "--cookie-secret"},
		{"allowed-domains", &cli.McpCmd{RemovedAllowedDomains: "example.com"}, "--allowed-domains"},
		{"allowed-emails", &cli.McpCmd{RemovedAllowedEmails: "a@example.com"}, "--allowed-emails"},
		{"signing-key", &cli.McpCmd{RemovedSigningKey: "pem"}, "--signing-key"},
		{"refresh-token-ttl", &cli.McpCmd{RemovedRefreshTokenTTL: "720h"}, "--refresh-token-ttl"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.Validate()
			if err == nil {
				t.Fatalf("expected error for removed flag %s", tc.wantFlag)
			}
			if !strings.Contains(err.Error(), tc.wantFlag) {
				t.Errorf("error should mention %s, got: %v", tc.wantFlag, err)
			}
			if !strings.Contains(err.Error(), "AgentCore Gateway") {
				t.Errorf("error should mention AgentCore Gateway delegation, got: %v", err)
			}
		})
	}
}

func TestMcpCmd_Validate_AuthModeOIDC_FailFast(t *testing.T) {
	cmd := &cli.McpCmd{AuthMode: "oidc"}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for --auth-mode=oidc")
	}
	if !strings.Contains(err.Error(), "AgentCore Gateway") {
		t.Errorf("error should mention AgentCore Gateway delegation, got: %v", err)
	}
}

func TestMcpCmd_Validate_Default_OK(t *testing.T) {
	cmd := &cli.McpCmd{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Backlog OAuth フラグは OIDC 認証を前提としなくなったため、
// 単独で設定しても Validate はエラーにならない。
func TestMcpCmd_Validate_BacklogClientIDAlone_OK(t *testing.T) {
	cmd := &cli.McpCmd{BacklogClientID: "some-client-id"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
