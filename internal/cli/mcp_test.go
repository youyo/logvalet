package cli_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
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

// S23 決定F: HTTP モードでは LOGVALET_SPACE_STORE_TYPE の明示指定を必須とし、
// 未設定・memory 選択時は起動エラー（警告ではない）になる。このチェックは
// Run() の最初（config/認証情報の解決や listen より前）に行われるため、
// サーバーを起動せずにエラーを検証できる。
// 注意: sqlite/dynamodb を選択した「成功」系は Run() が実際に listen まで
// 到達しテストがハングしうるため、ここでは検証しない
// （sqlite/dynamodb 許容の検証は internal/space の RequireExplicitStoreType
// 単体テストで行う）。
func TestMcpCmd_Run_RequiresExplicitSpaceStoreType(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
	}{
		{"unset", ""},
		{"memory", "memory"},
		{"MEMORY_case_insensitive", "MEMORY"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOGVALET_SPACE_STORE_TYPE", tc.envVal)

			cmd := &cli.McpCmd{Port: 0}
			g := &cli.GlobalFlags{}
			err := cmd.Run(g)

			if err == nil {
				t.Fatal("expected space store type error, got nil")
			}
			if !strings.Contains(err.Error(), "LOGVALET_SPACE_STORE_TYPE") {
				t.Errorf("error should mention LOGVALET_SPACE_STORE_TYPE, got: %v", err)
			}
		})
	}
}

// S23 決定E: HTTP モードでは tokenstore を使用しない。
// McpCmd に TokenStore 関連フィールドが存在しないことをコンパイル時に保証する
// （フィールドが復活すれば reflect チェックでも検出できるようにしておく）。
func TestMcpCmd_NoTokenStoreFields(t *testing.T) {
	typ := reflect.TypeOf(cli.McpCmd{})
	forbidden := []string{
		"TokenStore",
		"TokenStoreSQLitePath",
		"TokenStoreDynamoDBTable",
		"TokenStoreDynamoDBRegion",
	}
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		for _, f := range forbidden {
			if name == f {
				t.Errorf("McpCmd must not have field %s (HTTP mode does not use tokenstore, 決定E)", f)
			}
		}
	}
}

// S23 決定F: --token-store 系フラグは HTTP モードから完全に削除されており、
// Kong の unknown flag エラーとして fail-fast する。
func TestMcpCmd_TokenStoreFlags_UnknownFlagError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"token-store=dynamodb", []string{"mcp", "--token-store=dynamodb"}},
		{"token-store=sqlite", []string{"mcp", "--token-store=sqlite"}},
		{"token-store=memory", []string{"mcp", "--token-store=memory"}},
		{"token-store-dynamodb-table", []string{"mcp", "--token-store-dynamodb-table=t"}},
		{"token-store-dynamodb-region", []string{"mcp", "--token-store-dynamodb-region=r"}},
		{"token-store-sqlite-path", []string{"mcp", "--token-store-sqlite-path=/tmp/x.db"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var root cli.CLI
			p, err := kong.New(&root,
				kong.Name("logvalet"),
				kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
				kong.Exit(func(int) {}),
			)
			if err != nil {
				t.Fatalf("kong.New() エラー: %v", err)
			}
			_, err = p.Parse(tc.args)
			if err == nil {
				t.Fatalf("expected unknown flag error for %v, got nil", tc.args)
			}
			if !strings.Contains(err.Error(), "unknown flag") {
				t.Errorf("error should mention unknown flag, got: %v", err)
			}
		})
	}
}
