package mcp

import (
	"context"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/space"
)

// ServerConfig は MCP サーバーの設定。
// analysis 系ツールが IssueContextBuilder に渡す情報を保持する。
type ServerConfig struct {
	Profile          string
	Space            string
	BaseURL          string
	AuthorizationURL string
	DisableFilePaths bool // stdio モードでローカルファイルシステムへのアクセスを防止する
	// multi-space 対応フィールド（nil 許容 — 未設定時は通常動作）
	SpaceStore         space.Store
	SpaceResolver      *space.Resolver
	SpaceClientFactory space.ClientFactory
	// bootstrap_token 関連（multi-space OAuth フロー用）
	MultiSpaceAuthorizeURL string
	BootstrapKey           []byte
	BootstrapTokenTTL      time.Duration
	NonceStore             space.NonceStore
}

// NewServer は logvalet MCP サーバーを単一 client で初期化して返す。
// すべての tool を登録済みの MCPServer を返す。
// 既存パス（CLI profile / API key 認証）で使用する。
func NewServer(client backlog.Client, ver string, cfg ServerConfig) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"logvalet",
		ver,
		mcpserver.WithToolCapabilities(true),
	)
	var reg *ToolRegistry
	if cfg.SpaceResolver != nil {
		reg = NewToolRegistryWithMultiSpace(s, func(ctx context.Context) (backlog.Client, error) {
			return client, nil
		}, cfg.AuthorizationURL, cfg.SpaceResolver, cfg.SpaceClientFactory)
	} else {
		reg = NewToolRegistry(s, client, cfg.AuthorizationURL)
	}
	reg.disableFilePaths = cfg.DisableFilePaths
	registerAllTools(reg, cfg)
	return s
}

// NewServerWithFactory は per-user ClientFactory を使って logvalet MCP サーバーを初期化して返す。
// MCP ツール呼び出し時にリクエストの context.Context からユーザーを特定し、
// そのユーザーの Backlog OAuth トークンで backlog.Client を生成する。
// OAuth モード（--auth かつ LOGVALET_BACKLOG_CLIENT_ID 設定時）で使用する。
//
// factory には `auth.NewClientFactory(...)` で生成した ClientFactory を渡す。
// mcp → auth の import cycle を避けるため、引数型は匿名関数型で表現する。
func NewServerWithFactory(factory func(ctx context.Context) (backlog.Client, error), ver string, cfg ServerConfig) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"logvalet",
		ver,
		mcpserver.WithToolCapabilities(true),
	)
	var reg *ToolRegistry
	if cfg.SpaceResolver != nil {
		reg = NewToolRegistryWithMultiSpace(s, factory, cfg.AuthorizationURL, cfg.SpaceResolver, cfg.SpaceClientFactory)
	} else {
		reg = NewToolRegistryWithFactory(s, factory, cfg.AuthorizationURL)
	}
	reg.disableFilePaths = cfg.DisableFilePaths
	registerAllTools(reg, cfg)
	return s
}

// registerAllTools は MCP サーバーに全ツールを登録する共通ヘルパー。
// NewServer / NewServerWithFactory から呼ばれ、両者で同一のツールセットを保証する。
// space 管理5ツールは SpaceStore の有無に関わらず常に登録する。
func registerAllTools(reg *ToolRegistry, cfg ServerConfig) {
	RegisterIssueTools(reg)
	RegisterSearchTools(reg, cfg)
	RegisterProjectTools(reg)
	RegisterUserTools(reg)
	RegisterActivityTools(reg, cfg)
	RegisterDocumentTools(reg, cfg)
	RegisterTeamTools(reg)
	RegisterSpaceTools(reg, cfg)
	RegisterMetaTools(reg)
	RegisterSharedFileTools(reg)
	RegisterStarTools(reg)
	RegisterWatchingTools(reg)
	RegisterWikiTools(reg)
	RegisterAnalysisTools(reg, cfg)
	RegisterSpaceRegistryTools(reg, cfg.SpaceStore, cfg.SpaceResolver, cfg.MultiSpaceAuthorizeURL, cfg.BootstrapKey, cfg.BootstrapTokenTTL, cfg.NonceStore)
}
