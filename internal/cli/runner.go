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

// buildRunContext は GlobalFlags から RunContext を構築する。
// 1. config.toml をロード
// 2. credential resolver でトークン解決
// 3. backlog.NewHTTPClient でクライアント生成
// 4. render.NewRenderer でレンダラー生成
func buildRunContext(g *GlobalFlags) (*RunContext, error) {
	// 1. config.toml をロード（ファイル不在はゼロ値で継続）
	configPath := config.DefaultConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	// 2. 設定値を解決（CLI flags > env > config.toml > デフォルト）
	flags := config.OverrideFlags{
		Profile: g.Profile,
		Format:  g.Format,
	}
	resolved, err := config.Resolve(cfg, flags, os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("設定の解決に失敗しました: %w", err)
	}

	// 3. 認証情報を解決（CLI flags > env > tokens.json）
	tokensPath := credentials.DefaultTokensPath(os.Getenv)
	store := credentials.NewStore(tokensPath)
	resolver := credentials.NewResolver(store)
	cred, err := resolver.Resolve(resolved.AuthRef, credentials.CredentialFlags{}, os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("認証情報の解決に失敗しました (logvalet auth login を実行してください): %w", err)
	}

	// 4. BaseURL を決定
	baseURL := resolved.BaseURL
	if baseURL == "" && resolved.Space != "" {
		baseURL = fmt.Sprintf("https://%s.backlog.com", resolved.Space)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("Backlog スペースの URL が設定されていません (--profile または LOGVALET_BASE_URL を設定してください)")
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
	renderer, err := render.NewRenderer(format, resolved.Pretty)
	if err != nil {
		return nil, fmt.Errorf("レンダラーの生成に失敗しました: %w", err)
	}

	return &RunContext{
		Client:   client,
		Config:   resolved,
		Renderer: renderer,
	}, nil
}
