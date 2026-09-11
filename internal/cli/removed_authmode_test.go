package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

// A: --auth-mode 系は v0.40 で廃止され、値によらず fail-fast する。
// none も含めて分岐を残さない（HTTP は常に Bearer passthrough）。
func TestMcpCmd_Validate_RemovedAuthModeSettings(t *testing.T) {
	tests := []struct {
		name string
		cmd  cli.McpCmd
		want string
	}{
		{"auth_mode_none", cli.McpCmd{RemovedAuthMode: "none"}, "--auth-mode"},
		{"auth_mode_apikey", cli.McpCmd{RemovedAuthMode: "apikey"}, "--auth-mode"},
		{"auth_mode_bearer", cli.McpCmd{RemovedAuthMode: "bearer"}, "--auth-mode"},
		{"auth_api_key", cli.McpCmd{RemovedApiKey: "x"}, "--auth-api-key"},
		{"bearer_token", cli.McpCmd{RemovedBearerToken: "x"}, "--bearer-token"},
		{"backlog_client_id", cli.McpCmd{RemovedBacklogClientID: "x"}, "--backlog-client-id"},
		{"backlog_client_secret", cli.McpCmd{RemovedBacklogClientSecret: "x"}, "--backlog-client-secret"},
		{"backlog_redirect_url", cli.McpCmd{RemovedBacklogRedirectURL: "x"}, "--backlog-redirect-url"},
		{"oauth_state_secret", cli.McpCmd{RemovedOAuthStateSecret: "x"}, "--oauth-state-secret"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.Validate()
			if err == nil {
				t.Fatalf("%s 指定時はエラーになるべき", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("エラーに %s を含むべき: %v", tc.want, err)
			}
			if !strings.Contains(err.Error(), "Portals") {
				t.Errorf("エラーに移行先 (Portals) の案内を含むべき: %v", err)
			}
		})
	}
}

// 何も指定しなければ Validate() は通る（唯一のモード = passthrough）。
func TestMcpCmd_Validate_DefaultPassthrough_OK(t *testing.T) {
	cmd := &cli.McpCmd{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("既定構成でエラーになるべきでない: %v", err)
	}
}
