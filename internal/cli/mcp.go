package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	// OIDC 認証フラグ（idproxy）
	Auth             bool   `help:"enable idproxy authentication" group:"auth" env:"LOGVALET_MCP_AUTH"`
	ExternalURL      string `help:"external URL for OAuth callbacks" group:"auth" env:"LOGVALET_MCP_EXTERNAL_URL"`
	OIDCIssuer       string `help:"OIDC issuer URL" group:"auth" env:"LOGVALET_MCP_OIDC_ISSUER"`
	OIDCClientID     string `help:"OIDC client ID" group:"auth" env:"LOGVALET_MCP_OIDC_CLIENT_ID"`
	OIDCClientSecret string `help:"OIDC client secret" group:"auth" env:"LOGVALET_MCP_OIDC_CLIENT_SECRET"`
	CookieSecret     string `help:"cookie encryption key (hex-encoded, 64+ chars = 32+ bytes)" group:"auth" env:"LOGVALET_MCP_COOKIE_SECRET"`
	AllowedDomains   string `help:"comma-separated allowed email domains" group:"auth" env:"LOGVALET_MCP_ALLOWED_DOMAINS"`
	AllowedEmails    string `help:"comma-separated allowed email addresses" group:"auth" env:"LOGVALET_MCP_ALLOWED_EMAILS"`

	// Backlog OAuth フラグ（OIDC と同じ group + env タグ様式）
	BacklogClientID     string `name:"backlog-client-id" help:"Backlog OAuth client ID" group:"auth" env:"LOGVALET_MCP_BACKLOG_CLIENT_ID"`
	BacklogClientSecret string `name:"backlog-client-secret" help:"Backlog OAuth client secret" group:"auth" env:"LOGVALET_MCP_BACKLOG_CLIENT_SECRET"`
	BacklogRedirectURL  string `name:"backlog-redirect-url" help:"Backlog OAuth redirect URL" group:"auth" env:"LOGVALET_MCP_BACKLOG_REDIRECT_URL"`
	OAuthStateSecret    string `name:"oauth-state-secret" help:"HMAC-SHA256 signing key for OAuth state (hex-encoded, 32+ bytes)" group:"auth" env:"LOGVALET_MCP_OAUTH_STATE_SECRET"`

	// TokenStore フラグ
	TokenStore              string `name:"token-store" help:"token store type (memory/sqlite/dynamodb)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE"`
	TokenStoreSQLitePath    string `name:"token-store-sqlite-path" help:"SQLite DB file path (sqlite store only)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE_SQLITE_PATH"`
	TokenStoreDynamoDBTable  string `name:"token-store-dynamodb-table" help:"DynamoDB table name (dynamodb store only)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE"`
	TokenStoreDynamoDBRegion string `name:"token-store-dynamodb-region" help:"AWS region for DynamoDB table (dynamodb store only)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION"`

	// idproxy Store / JWT 署名鍵フラグ (Lambda マルチインスタンス対応)
	SigningKey                 string `name:"signing-key" help:"ECDSA P-256 signing key (PEM)" group:"auth" env:"LOGVALET_MCP_SIGNING_KEY"`
	IDProxyStore               string `name:"idproxy-store" help:"idproxy store type (memory|dynamodb)" group:"store" env:"LOGVALET_MCP_IDPROXY_STORE"`
	IDProxyStoreDynamoDBTable  string `name:"idproxy-store-dynamodb-table" help:"DynamoDB table name for idproxy store" group:"store" env:"LOGVALET_MCP_IDPROXY_STORE_DYNAMODB_TABLE"`
	IDProxyStoreDynamoDBRegion string `name:"idproxy-store-dynamodb-region" help:"AWS region for idproxy DynamoDB store" group:"store" env:"LOGVALET_MCP_IDPROXY_STORE_DYNAMODB_REGION"`
}

// Validate は McpCmd のフィールドを検証する。
//
// チェック順序:
//  1. BacklogClientID が設定されているが --auth が無効な場合は fast-fail する。
//     OAuth は per-user であり OIDC 認証（idproxy）が必須のため。
//  2. --auth 有効時は OIDC 必須フィールドをチェックする。
func (c *McpCmd) Validate() error {
	// idproxy Store 検証 (--auth の有無に関わらず適用)
	switch strings.ToLower(c.IDProxyStore) {
	case "", "memory":
		// OK
	case "dynamodb":
		if c.IDProxyStoreDynamoDBTable == "" {
			return fmt.Errorf("--idproxy-store-dynamodb-table is required when --idproxy-store=dynamodb")
		}
		if c.SigningKey == "" {
			return fmt.Errorf("--signing-key is required when --idproxy-store=dynamodb " +
				"(random signing key cannot be shared across Lambda containers)")
		}
	default:
		return fmt.Errorf("invalid --idproxy-store: %q (must be memory or dynamodb)", c.IDProxyStore)
	}

	// Backlog OAuth fast-fail: --auth なしに --backlog-client-id を設定しても動かない
	if c.BacklogClientID != "" && !c.Auth {
		return fmt.Errorf(
			"--backlog-client-id is set but --auth is disabled. " +
				"OAuth requires client authentication (OIDC). " +
				"Either enable --auth or unset --backlog-client-id. " +
				"See README \"Supported Modes\".",
		)
	}

	if !c.Auth {
		return nil
	}

	// OIDC 必須フィールドチェック
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

// buildOAuthEnvConfig は McpCmd のフィールドから OAuthEnvConfig を組み立てる。
// Kong が env タグを見て McpCmd フィールドに env 値を自動注入するため、
// flag/env 両対応は McpCmd フィールドを転記するだけで実現できる。
func (c *McpCmd) buildOAuthEnvConfig() (*auth.OAuthEnvConfig, error) {
	storeType, err := auth.ParseStoreType(c.TokenStore)
	if err != nil {
		return nil, fmt.Errorf("--token-store: %w", err)
	}

	sqlitePath := c.TokenStoreSQLitePath
	if sqlitePath == "" {
		sqlitePath = auth.DefaultSQLitePath
	}

	return &auth.OAuthEnvConfig{
		TokenStoreType:      storeType,
		SQLitePath:          sqlitePath,
		DynamoDBTable:       c.TokenStoreDynamoDBTable,
		DynamoDBRegion:      c.TokenStoreDynamoDBRegion,
		BacklogClientID:     c.BacklogClientID,
		BacklogClientSecret: c.BacklogClientSecret,
		BacklogRedirectURL:  c.BacklogRedirectURL,
		OAuthStateSecret:    c.OAuthStateSecret,
	}, nil
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

	// OAuth モード判定（--auth かつ BacklogClientID 設定時のみ有効）。
	// 既存 CLI / 既存 MCP パスは一切変更しない。
	var oauthDeps *OAuthDeps
	if c.Auth {
		oauthCfg, err := c.buildOAuthEnvConfig()
		if err != nil {
			return fmt.Errorf("load oauth config: %w", err)
		}
		if oauthCfg.OAuthEnabled() {
			if err := oauthCfg.Validate(); err != nil {
				return fmt.Errorf("validate oauth config: %w", err)
			}
			deps, err := BuildOAuthDeps(oauthCfg, rc.Config.Space, rc.Config.BaseURL, c.ExternalURL, slog.Default())
			if err != nil {
				return err
			}
			oauthDeps = deps
			defer func() { _ = oauthDeps.Close() }()
		}
	}

	// OAuth 有効時は AuthorizationURL を ServerConfig に設定
	if oauthDeps != nil {
		cfg.AuthorizationURL = oauthDeps.AuthorizeURL
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
		defer func() {
			if authCfg.Store != nil {
				_ = authCfg.Store.Close()
			}
		}()

		authMW, err := idproxy.New(context.Background(), authCfg)
		if err != nil {
			return err
		}

		topMux := http.NewServeMux()
		topMux.HandleFunc("/healthz", healthHandler)
		// idproxy.Wrap の内側に userID bridge を挟む（順序: auth.Wrap → bridge → innerMux）。
		// bridge を外側に置くと idproxy が context に注入する前に動き、userID が取れない。
		bridge := newUserIDBridge()
		var finalInner http.Handler = innerMux
		if oauthDeps != nil {
			finalInner = EnsureBacklogConnected(
				oauthDeps.TokenManager,
				oauthDeps.Provider.Name(),
				rc.Config.Space,
				oauthDeps.AuthorizeURL,
			)(innerMux)
		}
		topMux.Handle("/", authMW.Wrap(bridge(finalInner)))
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
