package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/version"
)

// removedCallerAuthNotice は呼び出し元認証系の削除済みフラグに対する fail-fast エラーの定型文。
const removedCallerAuthNotice = "--auth-mode は v0.40 で廃止されました。" +
	"HTTP モードは常に Backlog 資格情報の Bearer passthrough で動作します。" +
	"呼び出し元の認証・認可は Cloudflare MCP Server Portals 等の前段で行ってください。"

// McpCmd は `logvalet mcp` サブコマンド。
// Streamable HTTP MCP サーバーを起動する。
type McpCmd struct {
	Port int    `help:"listen port" default:"8080"`
	Host string `help:"listen host" default:"127.0.0.1"`

	// 注記: HTTP モードは常に「呼び出し元の認証なし + Backlog 資格情報は
	// Authorization: Bearer passthrough」の単一構成。呼び出し元の認証・認可は
	// Cloudflare MCP Server Portals 等の前段で行う。--token-store 系フラグは
	// フィールドごと削除済みで、指定すると Kong の unknown flag エラーになる。

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

	// multi-space 撤去 (v0.40) で廃止された space store 系。同じく fail-fast 専用。
	RemovedSpaceStoreType     string `name:"space-store-type" hidden:"" env:"LOGVALET_SPACE_STORE_TYPE"`
	RemovedSpaceStorePath     string `name:"space-store-path" hidden:"" env:"LOGVALET_SPACE_STORE_PATH"`
	RemovedSpaceStoreDDBTable string `name:"space-store-dynamodb-table" hidden:"" env:"LOGVALET_SPACE_STORE_DYNAMODB_TABLE"`
	RemovedSpaceStoreDDBRegion string `name:"space-store-dynamodb-region" hidden:"" env:"LOGVALET_SPACE_STORE_DYNAMODB_REGION"`

	// v0.40 で廃止: 呼び出し元認証（auth-mode / apikey）と OAuth ハンドラ系。同じく fail-fast 専用。
	RemovedAuthMode            string `name:"auth-mode" hidden:"" env:"LOGVALET_MCP_AUTH_MODE"`
	RemovedApiKey              string `name:"auth-api-key" hidden:"" env:"LOGVALET_MCP_API_KEY"`
	RemovedBearerToken         string `name:"bearer-token" hidden:"" env:"LOGVALET_MCP_BEARER_TOKEN"`
	RemovedBacklogClientID     string `name:"backlog-client-id" hidden:"" env:"LOGVALET_MCP_BACKLOG_CLIENT_ID"`
	RemovedBacklogClientSecret string `name:"backlog-client-secret" hidden:"" env:"LOGVALET_MCP_BACKLOG_CLIENT_SECRET"`
	RemovedBacklogRedirectURL  string `name:"backlog-redirect-url" hidden:"" env:"LOGVALET_MCP_BACKLOG_REDIRECT_URL"`
	RemovedOAuthStateSecret    string `name:"oauth-state-secret" hidden:"" env:"LOGVALET_MCP_OAUTH_STATE_SECRET"`
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
			return fmt.Errorf("%s は削除されました: %s", r.flag, removedCallerAuthNotice)
		}
	}

	removedSpaceStore := []struct {
		env string
		set bool
	}{
		{"LOGVALET_SPACE_STORE_TYPE", c.RemovedSpaceStoreType != ""},
		{"LOGVALET_SPACE_STORE_PATH", c.RemovedSpaceStorePath != ""},
		{"LOGVALET_SPACE_STORE_DYNAMODB_TABLE", c.RemovedSpaceStoreDDBTable != ""},
		{"LOGVALET_SPACE_STORE_DYNAMODB_REGION", c.RemovedSpaceStoreDDBRegion != ""},
	}
	for _, r := range removedSpaceStore {
		if r.set {
			return fmt.Errorf("%s は削除されました: %s", r.env, removedMultiSpaceNotice)
		}
	}

	removedCallerAuth := []struct {
		flag string
		set  bool
	}{
		{"--auth-mode", c.RemovedAuthMode != ""},
		{"--auth-api-key", c.RemovedApiKey != ""},
		{"--bearer-token", c.RemovedBearerToken != ""},
		{"--backlog-client-id", c.RemovedBacklogClientID != ""},
		{"--backlog-client-secret", c.RemovedBacklogClientSecret != ""},
		{"--backlog-redirect-url", c.RemovedBacklogRedirectURL != ""},
		{"--oauth-state-secret", c.RemovedOAuthStateSecret != ""},
	}
	for _, r := range removedCallerAuth {
		if r.set {
			return fmt.Errorf("%s は削除されました: %s", r.flag, removedCallerAuthNotice)
		}
	}
	return nil
}

// Validate は McpCmd のフィールドを検証する。
func (c *McpCmd) Validate() error {
	if err := c.validateRemovedFlags(); err != nil {
		return err
	}

	return nil
}

// buildHTTPHandler は HTTP モードのハンドラートポロジーを構築する。
//
// HTTP モードは単一構成のみ: logvalet は呼び出し元を認証せず、Backlog 資格情報は
// per-request の `Authorization: Bearer <token>` を Backlog へそのまま転送する
// (passthrough)。呼び出し元の認証・認可・tool 許可リストは Cloudflare MCP Server
// Portals 等の前段に委ねる (docs/specs/remote-mcp-request-contract.md)。
//
// stdio モード (McpStdioCmd) はこの経路を通らず、従来どおり単一 client
// (server 側 credential) を使う。
func (c *McpCmd) buildHTTPHandler(ver string, cfg mcpinternal.ServerConfig) http.Handler {
	factory := auth.NewPassthroughClientFactory(cfg.BaseURL)
	// 公式 Go SDK の StreamableHTTPHandler を Stateless=true で使う。エンドポイントパスは
	// 下の mcpMux.Handle("/mcp", h) が決めるため、ハンドラー側にパス設定は不要。
	h := mcpinternal.NewOfficialStreamableHTTPHandlerWithFactory(factory, ver, cfg)

	mcpMux := http.NewServeMux()
	mcpMux.Handle("/mcp", h)

	// /healthz は認証対象外。それ以外は Bearer 必須の passthrough。
	topMux := http.NewServeMux()
	topMux.HandleFunc("/healthz", healthHandler)
	topMux.Handle("/", mcpinternal.PassthroughAuthMiddleware(mcpMux))
	return topMux
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

	handler := c.buildHTTPHandler(ver, cfg)

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	fmt.Fprintf(os.Stderr, "logvalet MCP server listening on %s/mcp\n", addr)

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
