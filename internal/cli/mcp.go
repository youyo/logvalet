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

	// 認証フラグ（Gateway → logvalet の service-to-service 共有シークレット）。
	// apikey は Gateway を認証する共有鍵であってエンドユーザーを認証しない。
	AuthMode string `name:"auth-mode" help:"auth mode: apikey|none" group:"auth" env:"LOGVALET_MCP_AUTH_MODE"`
	// フラグ名がグローバルの --api-key（Backlog API キー）と衝突するため --auth-api-key とする。
	ApiKey string `name:"auth-api-key" help:"static api key for mode=apikey, sent as X-Logvalet-Api-Key (min 32 chars)" group:"auth" env:"LOGVALET_MCP_API_KEY"`
	// BearerToken は --auth-api-key の後方互換エイリアス。値は同じ apikey として扱われ、
	// 受理ヘッダーは X-Logvalet-Api-Key（Authorization ではない）。
	BearerToken string `name:"bearer-token" help:"deprecated alias for --auth-api-key" group:"auth" env:"LOGVALET_MCP_BEARER_TOKEN"`

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

// 実効認証モード。logvalet 側の認証は none|apikey の 2 値のみで、
// エンドユーザー認証（OIDC/JWT 検証）は AgentCore Gateway に委譲されている。
const (
	authModeNone   = "none"
	authModeAPIKey = "apikey"
)

// resolvedAuthMode は実効認証モードを返す。未指定は "none"。
// "bearer" は "apikey" の後方互換エイリアス。未知の値は Validate で弾かれるが、
// 万一 Run まで到達した場合に無認証で公開しないよう apikey へ倒す（fail-closed）。
func (c *McpCmd) resolvedAuthMode() string {
	switch strings.ToLower(strings.TrimSpace(c.AuthMode)) {
	case "", authModeNone:
		return authModeNone
	default:
		return authModeAPIKey
	}
}

// apiKeyValue は実効 apikey を返す。--auth-api-key を優先し、未指定なら
// 後方互換エイリアスの --bearer-token を使う。
func (c *McpCmd) apiKeyValue() string {
	if c.ApiKey != "" {
		return c.ApiKey
	}
	return c.BearerToken
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

	switch strings.ToLower(strings.TrimSpace(c.AuthMode)) {
	case "", authModeNone:
		return nil
	case authModeAPIKey, "bearer": // bearer は apikey の後方互換エイリアス
		key := c.apiKeyValue()
		if key == "" {
			return fmt.Errorf("--auth-api-key is required when --auth-mode=apikey (fail-closed: missing key would expose unauthenticated MCP)")
		}
		if len(key) < 32 {
			return fmt.Errorf("--auth-api-key: must be at least 32 characters, got %d", len(key))
		}
		return nil
	case "oidc":
		return fmt.Errorf("--auth-mode=oidc は削除されました: %s", removedAuthNotice)
	default:
		return fmt.Errorf("--auth-mode: invalid value %q; must be apikey or none", c.AuthMode)
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
	if c.resolvedAuthMode() == authModeAPIKey {
		// /healthz は apikey 検証の対象外（契約 §1.5）。
		topMux := http.NewServeMux()
		topMux.HandleFunc("/healthz", healthHandler)
		topMux.Handle("/", apiKeyAuthMiddleware(c.apiKeyValue())(mux))
		handler = topMux
		fmt.Fprintf(os.Stderr, "logvalet MCP server (apikey auth) listening on %s/mcp\n", addr)
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
