package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	idproxy "github.com/youyo/idproxy"
	"github.com/youyo/logvalet/internal/auth"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/version"
)

// McpCmd は `logvalet mcp` サブコマンド。
// Streamable HTTP MCP サーバーを起動する。
type McpCmd struct {
	Port int    `help:"listen port" default:"8080"`
	Host string `help:"listen host" default:"127.0.0.1"`

	Auth             bool   `help:"enable idproxy authentication" group:"auth" env:"LOGVALET_MCP_AUTH"`
	ExternalURL      string `help:"external URL for OAuth callbacks" group:"auth" env:"LOGVALET_MCP_EXTERNAL_URL"`
	OIDCIssuer       string `help:"OIDC issuer URL" group:"auth" env:"LOGVALET_MCP_OIDC_ISSUER"`
	OIDCClientID     string `help:"OIDC client ID" group:"auth" env:"LOGVALET_MCP_OIDC_CLIENT_ID"`
	OIDCClientSecret string `help:"OIDC client secret" group:"auth" env:"LOGVALET_MCP_OIDC_CLIENT_SECRET"`
	CookieSecret     string `help:"cookie encryption key (hex-encoded, 64+ chars = 32+ bytes)" group:"auth" env:"LOGVALET_MCP_COOKIE_SECRET"`
	AllowedDomains   string `help:"comma-separated allowed email domains" group:"auth" env:"LOGVALET_MCP_ALLOWED_DOMAINS"`
	AllowedEmails    string `help:"comma-separated allowed email addresses" group:"auth" env:"LOGVALET_MCP_ALLOWED_EMAILS"`
}

// Validate は認証フラグが有効な場合に必須フィールドをチェックする。
func (c *McpCmd) Validate() error {
	if !c.Auth {
		return nil
	}
	if c.ExternalURL == "" {
		return fmt.Errorf("--external-url is required when --auth is enabled")
	}
	if c.OIDCIssuer == "" {
		return fmt.Errorf("--oidc-issuer is required when --auth is enabled")
	}
	if c.OIDCClientID == "" {
		return fmt.Errorf("--oidc-client-id is required when --auth is enabled")
	}
	if c.CookieSecret == "" {
		return fmt.Errorf("--cookie-secret is required when --auth is enabled")
	}
	secret, err := hex.DecodeString(c.CookieSecret)
	if err != nil {
		return fmt.Errorf("--cookie-secret: invalid hex: %w", err)
	}
	if len(secret) < 32 {
		return fmt.Errorf("--cookie-secret: must be at least 32 bytes (64 hex chars), got %d bytes", len(secret))
	}
	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// Run は MCP サーバーを起動する。
func (c *McpCmd) Run(g *GlobalFlags) error {
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	ver := version.NewInfo().Version
	cfg := mcpinternal.ServerConfig{
		Profile: rc.Config.Profile,
		Space:   rc.Config.Space,
		BaseURL: rc.Config.BaseURL,
	}

	// OAuth モード判定（--auth かつ LOGVALET_BACKLOG_CLIENT_ID 設定時のみ有効）。
	// 既存 CLI / 既存 MCP パスは一切変更しない。
	var oauthDeps *OAuthDeps
	if c.Auth {
		oauthCfg, err := auth.LoadOAuthEnvConfig(os.Getenv)
		if err != nil {
			return fmt.Errorf("load oauth config: %w", err)
		}
		if oauthCfg.OAuthEnabled() {
			if err := oauthCfg.Validate(); err != nil {
				return fmt.Errorf("validate oauth config: %w", err)
			}
			deps, err := BuildOAuthDeps(oauthCfg, rc.Config.Space, rc.Config.BaseURL, slog.Default())
			if err != nil {
				return err
			}
			oauthDeps = deps
			defer func() { _ = oauthDeps.Close() }()
		}
	}

	// MCP サーバー構築（OAuth 有無で分岐）
	var s *mcpserver.MCPServer
	if oauthDeps != nil {
		s = mcpinternal.NewServerWithFactory(oauthDeps.Factory, ver, cfg)
	} else {
		s = mcpinternal.NewServer(rc.Client, ver, cfg)
	}
	h := mcpserver.NewStreamableHTTPServer(s, mcpserver.WithEndpointPath("/mcp"))

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	// innerMux: MCP + OAuth ルートを同居させる（OAuth モード時のみ OAuth ルート登録）
	innerMux := http.NewServeMux()
	innerMux.Handle("/mcp", h)
	if oauthDeps != nil {
		InstallOAuthRoutes(innerMux, oauthDeps.Handler)
	}

	var handler http.Handler

	if c.Auth {
		authCfg, err := BuildAuthConfig(c)
		if err != nil {
			return err
		}

		authMW, err := idproxy.New(context.Background(), authCfg)
		if err != nil {
			return err
		}

		topMux := http.NewServeMux()
		topMux.HandleFunc("/healthz", healthHandler)
		// idproxy.Wrap の内側に userID bridge を挟む（順序: auth.Wrap → bridge → innerMux）。
		// bridge を外側に置くと idproxy が context に注入する前に動き、userID が取れない。
		bridge := newUserIDBridge()
		topMux.Handle("/", authMW.Wrap(bridge(innerMux)))
		handler = topMux

		if oauthDeps != nil {
			fmt.Fprintf(os.Stderr, "logvalet MCP server (auth + OAuth) listening on %s/mcp\n", addr)
			fmt.Fprintln(os.Stderr, "  OAuth routes: /oauth/backlog/{authorize,callback,status,disconnect}")
		} else {
			fmt.Fprintf(os.Stderr, "logvalet MCP server (auth enabled) listening on %s/mcp\n", addr)
		}
	} else {
		innerMux.HandleFunc("/healthz", healthHandler)
		handler = innerMux

		fmt.Fprintf(os.Stderr, "logvalet MCP server listening on %s/mcp\n", addr)
	}

	srv := &http.Server{Addr: addr, Handler: handler}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
		}
	}

	return nil
}
