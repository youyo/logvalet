package mcp

import (
	"context"
	"time"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
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

// newOfficialMCPServer は公式 Go SDK の *officialmcp.Server を logvalet の
// Implementation 情報で生成する。NewServer / NewServerWithFactory の共通処理。
func newOfficialMCPServer(ver string) *officialmcp.Server {
	return officialmcp.NewServer(&officialmcp.Implementation{Name: "logvalet", Version: ver}, nil)
}

// buildRegistry は cfg に応じた ToolRegistry を backend 上に構築し、全ツールを登録する。
//
// client と factory はどちらか一方を渡す (factory != nil が優先)。cfg.SpaceResolver が
// 設定されている場合は multi-space 対応の ToolRegistry を使い、factory が nil のときは
// client を返すだけの固定 factory でラップする。
//
// NewServer / NewServerWithFactory / NewOfficialStreamableHTTPHandler 系および
// テスト用の fake backend 経路がすべてこの関数を通ることで、どの経路でも
// 同一のツールセットが登録されることを保証する。
func buildRegistry(
	backend ServerBackend,
	client backlog.Client,
	factory func(ctx context.Context) (backlog.Client, error),
	cfg ServerConfig,
) *ToolRegistry {
	var reg *ToolRegistry
	switch {
	case cfg.SpaceResolver != nil:
		f := factory
		if f == nil {
			f = func(context.Context) (backlog.Client, error) { return client, nil }
		}
		reg = NewToolRegistryWithMultiSpace(backend, f, cfg.AuthorizationURL, cfg.SpaceResolver, cfg.SpaceClientFactory)
	case factory != nil:
		reg = NewToolRegistryWithFactory(backend, factory, cfg.AuthorizationURL)
	default:
		reg = NewToolRegistryWithBackend(backend, client, cfg.AuthorizationURL)
	}
	reg.disableFilePaths = cfg.DisableFilePaths
	registerAllTools(reg, cfg)
	return reg
}

// NewServer は logvalet MCP サーバーを単一 client で初期化して返す。
// すべての tool を登録済みの公式 Go SDK サーバー (*officialmcp.Server) を返す。
// 既存パス（CLI profile / API key 認証）で使用する。
func NewServer(client backlog.Client, ver string, cfg ServerConfig) *officialmcp.Server {
	s := newOfficialMCPServer(ver)
	buildRegistry(NewOfficialBackend(s), client, nil, cfg)
	return s
}

// NewServerWithFactory は per-user ClientFactory を使って logvalet MCP サーバーを初期化して返す。
// MCP ツール呼び出し時にリクエストの context.Context からユーザーを特定し、
// そのユーザーの Backlog OAuth トークンで backlog.Client を生成する。
// OAuth モード（--auth かつ LOGVALET_BACKLOG_CLIENT_ID 設定時）で使用する。
//
// factory には `auth.NewClientFactory(...)` で生成した ClientFactory を渡す。
// mcp → auth の import cycle を避けるため、引数型は匿名関数型で表現する。
func NewServerWithFactory(factory func(ctx context.Context) (backlog.Client, error), ver string, cfg ServerConfig) *officialmcp.Server {
	s := newOfficialMCPServer(ver)
	buildRegistry(NewOfficialBackend(s), nil, factory, cfg)
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
