package cli_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

// 削除済み認証フラグの fail-fast を検証するテーブル。
// いずれも Portals 前段への移行を案内するエラーになる必要がある。
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
			if !strings.Contains(err.Error(), "Portals") {
				t.Errorf("error should mention Portals, got: %v", err)
			}
		})
	}
}

func TestMcpCmd_Validate_Default_OK(t *testing.T) {
	cmd := &cli.McpCmd{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// multi-space 撤去 (v0.40): space store 系の設定は廃止され、指定すると
// Validate() が移行先を案内して fail-fast する。Kong が env から値を読むため、
// ここでは対応する Removed* フィールドを直接埋めて検証する。
func TestMcpCmd_Validate_RemovedSpaceStoreSettings(t *testing.T) {
	cases := []struct {
		name string
		cmd  cli.McpCmd
		env  string
	}{
		{"type", cli.McpCmd{RemovedSpaceStoreType: "sqlite"}, "LOGVALET_SPACE_STORE_TYPE"},
		{"path", cli.McpCmd{RemovedSpaceStorePath: "/tmp/spaces.db"}, "LOGVALET_SPACE_STORE_PATH"},
		{"ddb_table", cli.McpCmd{RemovedSpaceStoreDDBTable: "tbl"}, "LOGVALET_SPACE_STORE_DYNAMODB_TABLE"},
		{"ddb_region", cli.McpCmd{RemovedSpaceStoreDDBRegion: "ap-northeast-1"}, "LOGVALET_SPACE_STORE_DYNAMODB_REGION"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.Validate()
			if err == nil {
				t.Fatalf("%s 指定時はエラーになるべき", tc.env)
			}
			if !strings.Contains(err.Error(), tc.env) {
				t.Errorf("エラーに %s を含むべき: %v", tc.env, err)
			}
			if !strings.Contains(err.Error(), "Portals") {
				t.Errorf("エラーに移行先 (Portals) の案内を含むべき: %v", err)
			}
		})
	}
}

// 未設定なら Validate() は通る（multi-space 撤去後の既定経路）。
func TestMcpCmd_Validate_NoSpaceStoreSettings_OK(t *testing.T) {
	cmd := &cli.McpCmd{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("space store 系未指定ならエラーにならないべき: %v", err)
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
