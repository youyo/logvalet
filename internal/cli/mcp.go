package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	idproxy "github.com/youyo/idproxy"
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
	s := mcpinternal.NewServer(rc.Client, ver, cfg)
	h := mcpserver.NewStreamableHTTPServer(s, mcpserver.WithEndpointPath("/mcp"))

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	mcpMux := http.NewServeMux()
	mcpMux.Handle("/mcp", h)

	var handler http.Handler

	if c.Auth {
		authCfg, err := BuildAuthConfig(c)
		if err != nil {
			return err
		}

		auth, err := idproxy.New(context.Background(), authCfg)
		if err != nil {
			return err
		}

		topMux := http.NewServeMux()
		topMux.HandleFunc("/healthz", healthHandler)
		topMux.Handle("/", auth.Wrap(mcpMux))
		handler = topMux

		fmt.Fprintf(os.Stderr, "logvalet MCP server (auth enabled) listening on %s/mcp\n", addr)
	} else {
		mcpMux.HandleFunc("/healthz", healthHandler)
		handler = mcpMux

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
