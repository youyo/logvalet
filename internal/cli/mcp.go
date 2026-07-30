package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
	"github.com/youyo/logvalet/internal/version"
)

// removedAuthNotice は削除済み認証フラグに対する fail-fast エラーの定型文。
const removedAuthNotice = "MCP アクセスの認証は AgentCore Gateway に委譲されました。" +
	"logvalet 側の外部 IdP 連携は廃止されています。"

// McpCmd は `logvalet mcp` サブコマンド。
// Streamable HTTP MCP サーバーを起動する。
type McpCmd struct {
	Port int    `help:"listen port" default:"8080"`
	Host string `help:"listen host" default:"127.0.0.1"`

	// 認証フラグ（Gateway 手前の共有シークレット）
	AuthMode    string `name:"auth-mode" help:"auth mode: bearer|none" group:"auth" env:"LOGVALET_MCP_AUTH_MODE"`
	BearerToken string `name:"bearer-token" help:"static bearer token for mode=bearer (min 32 chars)" group:"auth" env:"LOGVALET_MCP_BEARER_TOKEN"`

	// Backlog OAuth フラグ
	BacklogClientID     string `name:"backlog-client-id" help:"Backlog OAuth client ID" group:"auth" env:"LOGVALET_MCP_BACKLOG_CLIENT_ID"`
	BacklogClientSecret string `name:"backlog-client-secret" help:"Backlog OAuth client secret" group:"auth" env:"LOGVALET_MCP_BACKLOG_CLIENT_SECRET"`
	BacklogRedirectURL  string `name:"backlog-redirect-url" help:"Backlog OAuth redirect URL" group:"auth" env:"LOGVALET_MCP_BACKLOG_REDIRECT_URL"`
	OAuthStateSecret    string `name:"oauth-state-secret" help:"HMAC-SHA256 signing key for OAuth state (hex-encoded, 32+ bytes)" group:"auth" env:"LOGVALET_MCP_OAUTH_STATE_SECRET"`

	// TokenStore フラグ
	TokenStore               string `name:"token-store" help:"token store type (memory/sqlite/dynamodb)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE"`
	TokenStoreSQLitePath     string `name:"token-store-sqlite-path" help:"SQLite DB file path (sqlite store only)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE_SQLITE_PATH"`
	TokenStoreDynamoDBTable  string `name:"token-store-dynamodb-table" help:"DynamoDB table name (dynamodb store only)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE"`
	TokenStoreDynamoDBRegion string `name:"token-store-dynamodb-region" help:"AWS region for DynamoDB table (dynamodb store only)" group:"store" env:"LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION"`

	// 削除済みフラグ。値が渡された場合に移行先を案内して fail-fast するためだけに
	// 定義を残している（ヘルプ非表示・機能なし）。
	RemovedAuth             bool   `name:"auth" hidden:"" env:"LOGVALET_MCP_AUTH"`
	RemovedExternalURL      string `name:"external-url" hidden:"" env:"LOGVALET_MCP_EXTERNAL_URL"`
	RemovedOIDCIssuer       string `name:"oidc-issuer" hidden:"" env:"LOGVALET_MCP_OIDC_ISSUER"`
	RemovedOIDCClientID     string `name:"oidc-client-id" hidden:"" env:"LOGVALET_MCP_OIDC_CLIENT_ID"`
	RemovedOIDCClientSecret string `name:"oidc-client-secret" hidden:"" env:"LOGVALET_MCP_OIDC_CLIENT_SECRET"`
	RemovedCookieSecret     string `name:"cookie-secret" hidden:"" env:"LOGVALET_MCP_COOKIE_SECRET"`
	RemovedAllowedDomains   string `name:"allowed-domains" hidden:"" env:"LOGVALET_MCP_ALLOWED_DOMAINS"`
	RemovedAllowedEmails    string `name:"allowed-emails" hidden:"" env:"LOGVALET_MCP_ALLOWED_EMAILS"`
	RemovedSigningKey       string `name:"signing-key" hidden:"" env:"LOGVALET_MCP_SIGNING_KEY"`
	RemovedRefreshTokenTTL  string `name:"refresh-token-ttl" hidden:"" env:"LOGVALET_MCP_REFRESH_TOKEN_TTL"`
}

// resolvedAuthMode は実効認証モードを返す。未指定は "none"。
func (c *McpCmd) resolvedAuthMode() string {
	return strings.ToLower(strings.TrimSpace(c.AuthMode))
}

// validateRemovedFlags は削除済みフラグが指定されていないかを検査する。
func (c *McpCmd) validateRemovedFlags() error {
	removed := []struct {
		flag string
		set  bool
	}{
		{"--auth", c.RemovedAuth},
		{"--external-url", c.RemovedExternalURL != ""},
		{"--oidc-issuer", c.RemovedOIDCIssuer != ""},
		{"--oidc-client-id", c.RemovedOIDCClientID != ""},
		{"--oidc-client-secret", c.RemovedOIDCClientSecret != ""},
		{"--cookie-secret", c.RemovedCookieSecret != ""},
		{"--allowed-domains", c.RemovedAllowedDomains != ""},
		{"--allowed-emails", c.RemovedAllowedEmails != ""},
		{"--signing-key", c.RemovedSigningKey != ""},
		{"--refresh-token-ttl", c.RemovedRefreshTokenTTL != ""},
	}
	for _, r := range removed {
		if r.set {
			return fmt.Errorf("%s は削除されました: %s", r.flag, removedAuthNotice)
		}
	}
	return nil
}

// Validate は McpCmd のフィールドを検証する。
func (c *McpCmd) Validate() error {
	if err := c.validateRemovedFlags(); err != nil {
		return err
	}

	switch c.resolvedAuthMode() {
	case "", "none":
		return nil
	case "bearer":
		if c.BearerToken == "" {
			return fmt.Errorf("--bearer-token is required when --auth-mode=bearer (fail-closed: missing token would expose unauthenticated MCP)")
		}
		if len(c.BearerToken) < 32 {
			return fmt.Errorf("--bearer-token: must be at least 32 characters, got %d", len(c.BearerToken))
		}
		return nil
	case "oidc":
		return fmt.Errorf("--auth-mode=oidc は削除されました: %s", removedAuthNotice)
	default:
		return fmt.Errorf("--auth-mode: invalid value %q; must be bearer or none", c.AuthMode)
	}
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

	// SpaceStore / Resolver / ClientFactory を設定（space 管理ツール有効化）。
	// 失敗しても MCP サーバー起動は継続し、space 管理ツールのみ無効になる。
	if spaceStore, storeErr := buildSpaceStore(); storeErr != nil {
		slog.Warn("space store init failed, space management tools disabled", "error", storeErr)
	} else {
		cfg.SpaceStore = spaceStore
		cfg.SpaceResolver = space.NewResolver(spaceStore)
		if cliFactory, factoryErr := buildCLIClientFactory(); factoryErr != nil {
			slog.Warn("space client factory init failed", "error", factoryErr)
		} else {
			cfg.SpaceClientFactory = cliFactory
		}
	}

	// 公式 Go SDK の StreamableHTTPHandler を Stateless=true で使う。エンドポイントパスは
	// 下の mux.Handle("/mcp", h) が決めるため、ハンドラー側にパス設定は不要。
	h := mcpinternal.NewOfficialStreamableHTTPHandler(rc.Client, ver, cfg)

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	mux := http.NewServeMux()
	mux.Handle("/mcp", h)

	var handler http.Handler
	if c.resolvedAuthMode() == "bearer" {
		topMux := http.NewServeMux()
		topMux.HandleFunc("/healthz", healthHandler)
		topMux.Handle("/", bearerAuthMiddleware(c.BearerToken)(mux))
		handler = topMux
		fmt.Fprintf(os.Stderr, "logvalet MCP server (bearer auth) listening on %s/mcp\n", addr)
	} else {
		mux.HandleFunc("/healthz", healthHandler)
		handler = mux
		fmt.Fprintf(os.Stderr, "logvalet MCP server listening on %s/mcp\n", addr)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

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
