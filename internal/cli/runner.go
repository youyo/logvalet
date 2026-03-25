package cli

import (
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/config"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/render"
)

// RunContext は全コマンドで共有する実行コンテキスト。
type RunContext struct {
	Client   backlog.Client
	Config   *config.ResolvedConfig
	Renderer render.Renderer
}

// boolPtr は bool 値のポインタを返すヘルパー。
// GlobalFlags の bool フィールドを config.OverrideFlags の *bool に変換するために使用する。
func boolPtr(b bool) *bool {
	return &b
}

// buildRunContext は GlobalFlags から RunContext を構築する。
// 1. config パスを解決
// 2. config.toml をロード
// 3. 設定値を優先順位に従い解決
// 4. 認証情報を解決
// 5. Backlog クライアントを生成
// 6. レンダラーを生成
func buildRunContext(g *GlobalFlags) (*RunContext, error) {
	// 1. config パスを解決（--config > LOGVALET_CONFIG > デフォルト）
	configPath := config.ResolveConfigPath(g.Config, os.Getenv)
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// 2. 設定値を解決（CLI flags > env > config.toml > デフォルト）
	flags := config.OverrideFlags{
		Profile:    g.Profile,
		Format:     g.Format,
		Pretty:     boolPtr(g.Pretty),
		Space:      g.Space,
		BaseURL:    g.BaseURL,
		Verbose:    boolPtr(g.Verbose),
		NoColor:    boolPtr(g.NoColor),
		ConfigPath: g.Config,
	}
	resolved, err := config.Resolve(cfg, flags, os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config: %w", err)
	}

	// 3. 認証情報を解決（CLI flags > env > tokens.json）
	tokensPath := credentials.DefaultTokensPath(os.Getenv)
	store := credentials.NewStore(tokensPath)
	resolver := credentials.NewResolver(store)
	credFlags := credentials.CredentialFlags{
		APIKey:      g.APIKey,
		AccessToken: g.AccessToken,
	}
	cred, err := resolver.Resolve(resolved.AuthRef, credFlags, os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve credentials (run logvalet auth login): %w", err)
	}

	// 4. BaseURL を決定
	baseURL := resolved.BaseURL
	if baseURL == "" && resolved.Space != "" {
		baseURL = fmt.Sprintf("https://%s.backlog.com", resolved.Space)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("Backlog space URL not set (configure --profile or LOGVALET_BASE_URL)")
	}

	// 5. Backlog クライアントを生成
	client := backlog.NewHTTPClient(backlog.ClientConfig{
		BaseURL:    baseURL,
		Credential: cred,
	})

	// 6. レンダラーを生成
	format := resolved.Format
	if format == "" {
		format = "json"
	}
	renderer, err := render.NewRenderer(format, resolved.Pretty, resolved.Space)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	return &RunContext{
		Client:   client,
		Config:   resolved,
		Renderer: renderer,
	}, nil
}

// buildRenderer は GlobalFlags から Renderer のみを構築する。
// 認証不要なコマンド（auth logout, auth list 等）で使用する。
func buildRenderer(g *GlobalFlags) (render.Renderer, error) {
	format := g.Format
	if format == "" {
		format = "json"
	}
	return render.NewRenderer(format, g.Pretty, g.Space)
}
