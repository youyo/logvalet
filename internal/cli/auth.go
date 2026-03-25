package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/config"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/domain"
)

// AuthCmd は auth コマンド群のルート。
type AuthCmd struct {
	Login  AuthLoginCmd  `cmd:"" help:"Backlog にログインする"`
	Logout AuthLogoutCmd `cmd:"" help:"Backlog からログアウトする"`
	Whoami AuthWhoamiCmd `cmd:"" help:"現在のログインユーザーを表示する"`
	List   AuthListCmd   `cmd:"" help:"保存されている認証情報を一覧表示する"`
}

// ---- auth login ----

// AuthLoginRequest は auth login の内部リクエスト（テスト用DI）。
type AuthLoginRequest struct {
	AuthType     string
	APIKey       string
	AccessToken  string
	RefreshToken string
	TokenExpiry  string
	Space        string
	BaseURL      string
}

// AuthLoginCmd は auth login コマンド。
// API キーは GlobalFlags.APIKey (--api-key / LOGVALET_API_KEY) から取得する。
type AuthLoginCmd struct{}

// Run は auth login のメインエントリポイント。
// API キーを受け取り tokens.json に保存する。
func (c *AuthLoginCmd) Run(g *GlobalFlags) error {
	// API キーを解決（GlobalFlags --api-key > stdin プロンプト）
	apiKey := g.APIKey
	if apiKey == "" {
		fmt.Fprint(os.Stderr, "API Key: ")
		if _, err := fmt.Fscanln(os.Stdin, &apiKey); err != nil {
			return fmt.Errorf("API Key の読み取りに失敗しました: %w", err)
		}
	}
	if apiKey == "" {
		return fmt.Errorf("API Key が指定されていません")
	}

	// BaseURL を解決（プロファイルまたは直接指定）
	baseURL, space, err := resolveAuthBaseURL(g)
	if err != nil {
		return err
	}

	req := AuthLoginRequest{
		AuthType: credentials.AuthTypeAPIKey,
		APIKey:   apiKey,
		Space:    space,
		BaseURL:  baseURL,
	}

	store := credentials.NewStore(credentials.DefaultTokensPath(os.Getenv))
	resp, err := c.RunWithLoginRequestCapture(g, store, req)
	if err != nil {
		return err
	}
	renderer, rerr := buildRenderer(g)
	if rerr != nil {
		return rerr
	}
	return renderer.Render(os.Stdout, resp)
}

// resolveAuthBaseURL は auth login の BaseURL と space 名を解決する。
// プロファイルが指定されている場合はプロファイルから取得。
// --profile が存在しない場合、または BaseURL が設定されていない場合はエラー。
func resolveAuthBaseURL(g *GlobalFlags) (baseURL, space string, err error) {
	configPath := config.DefaultConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", "", fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
	}

	flags := config.OverrideFlags{Profile: g.Profile}
	resolved, err := config.Resolve(cfg, flags, os.Getenv)
	if err != nil {
		return "", "", fmt.Errorf("設定の解決に失敗しました: %w", err)
	}

	baseURL = resolved.BaseURL
	space = resolved.Space

	if baseURL == "" && space != "" {
		baseURL = fmt.Sprintf("https://%s.backlog.com", space)
	}

	if baseURL == "" {
		return "", "", fmt.Errorf("Backlog スペースの URL が設定されていません。--profile でプロファイルを指定するか、LOGVALET_BASE_URL 環境変数を設定してください (exit 2)")
	}

	if space == "" {
		// BaseURL からスペース名を抽出（簡略: backlog.com のサブドメイン）
		// https://example.backlog.com → example
		u := baseURL
		if len(u) > 8 && u[:8] == "https://" {
			u = u[8:]
		}
		if idx := len(u) - len(".backlog.com"); idx > 0 && u[idx:] == ".backlog.com" {
			space = u[:idx]
		} else {
			space = u
		}
	}

	return baseURL, space, nil
}

// RunWithLoginRequestCapture は認証情報を受け取って tokens.json に保存する。
// テストおよび内部実装から呼び出す。
// レスポンス struct を返す（レンダリングは呼び出し元が行う）。
func (c *AuthLoginCmd) RunWithLoginRequestCapture(g *GlobalFlags, store credentials.Store, req AuthLoginRequest) (any, error) {
	profile, err := resolveProfile(g)
	if err != nil {
		return "", fmt.Errorf("auth login: %w", err)
	}

	// tokens.json をロードして更新
	tokens, err := store.Load()
	if err != nil {
		return "", fmt.Errorf("auth login: failed to load tokens: %w", err)
	}
	if tokens.Auth == nil {
		tokens.Auth = make(map[string]credentials.AuthEntry)
	}
	if tokens.Version == 0 {
		tokens.Version = 1
	}

	// エントリを追加/上書き
	entry := credentials.AuthEntry{
		AuthType:     req.AuthType,
		APIKey:       req.APIKey,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		TokenExpiry:  req.TokenExpiry,
	}
	tokens.Auth[profile] = entry

	if err := store.Save(tokens); err != nil {
		return "", fmt.Errorf("auth login: failed to save tokens: %w", err)
	}

	// レスポンス struct を返す
	resp := authLoginResponse{
		SchemaVersion: "1",
		Result:        "ok",
		Profile:       profile,
		Space:         req.Space,
		BaseURL:       req.BaseURL,
		AuthType:      req.AuthType,
		Saved:         true,
	}
	return resp, nil
}

type authLoginResponse struct {
	SchemaVersion string `json:"schema_version"`
	Result        string `json:"result"`
	Profile       string `json:"profile"`
	Space         string `json:"space,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	AuthType      string `json:"auth_type"`
	Saved         bool   `json:"saved"`
}

// ---- auth logout ----

// AuthLogoutCmd は auth logout コマンド。
type AuthLogoutCmd struct{}

// Run は auth logout のメインエントリポイント（Kong から呼び出し）。
func (c *AuthLogoutCmd) Run(g *GlobalFlags) error {
	store := credentials.NewStore(credentials.DefaultTokensPath(func(key string) string {
		return ""
	}))
	resp, err := c.RunWithStore(g, store)
	if err != nil {
		return err
	}
	renderer, rerr := buildRenderer(g)
	if rerr != nil {
		return rerr
	}
	return renderer.Render(os.Stdout, resp)
}

// RunWithStore は tokens.json から指定プロファイルのエントリを削除する（テスト用DI）。
// レスポンス struct を返す（レンダリングは呼び出し元が行う）。
func (c *AuthLogoutCmd) RunWithStore(g *GlobalFlags, store credentials.Store) (any, error) {
	profile, err := resolveProfile(g)
	if err != nil {
		return nil, fmt.Errorf("auth logout: %w", err)
	}

	tokens, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("auth logout: failed to load tokens: %w", err)
	}

	if tokens.Auth == nil || len(tokens.Auth) == 0 {
		return nil, fmt.Errorf("auth logout: profile %q not found in tokens.json", profile)
	}

	if _, ok := tokens.Auth[profile]; !ok {
		return nil, fmt.Errorf("auth logout: profile %q not found in tokens.json", profile)
	}

	delete(tokens.Auth, profile)

	if err := store.Save(tokens); err != nil {
		return nil, fmt.Errorf("auth logout: failed to save tokens: %w", err)
	}

	resp := authLogoutResponse{
		SchemaVersion: "1",
		Result:        "ok",
		Profile:       profile,
		Removed:       true,
	}
	return resp, nil
}

type authLogoutResponse struct {
	SchemaVersion string `json:"schema_version"`
	Result        string `json:"result"`
	Profile       string `json:"profile"`
	Removed       bool   `json:"removed"`
}

// ---- auth whoami ----

// AuthWhoamiCmd は auth whoami コマンド。
type AuthWhoamiCmd struct{}

// Run は auth whoami のメインエントリポイント（Kong から呼び出し）。
func (c *AuthWhoamiCmd) Run(g *GlobalFlags) error {
	store := credentials.NewStore(credentials.DefaultTokensPath(func(key string) string {
		return ""
	}))
	authRef := g.Profile

	// API クライアントを構築して user 情報を取得
	var user *domain.User
	rc, err := buildRunContext(g)
	if err == nil {
		ctx := context.Background()
		u, apiErr := rc.Client.GetMyself(ctx)
		if apiErr == nil {
			user = u
		}
		// API 呼び出し失敗時は user = nil でフォールバック（オフライン対応）
	}

	resp, err := c.RunWithStoreCapture(g, store, authRef, user)
	if err != nil {
		return err
	}
	renderer, rerr := buildRenderer(g)
	if rerr != nil {
		return rerr
	}
	return renderer.Render(os.Stdout, resp)
}

// RunWithStoreCapture は tokens.json から認証情報を取得してレスポンス struct を返す（テスト用DI）。
// user が非 nil の場合はレスポンスに含める（API 呼び出し成功時）。
func (c *AuthWhoamiCmd) RunWithStoreCapture(g *GlobalFlags, store credentials.Store, authRef string, user *domain.User) (any, error) {
	profile, err := resolveProfile(g)
	if err != nil {
		return nil, fmt.Errorf("auth whoami: %w", err)
	}

	tokens, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("auth whoami: failed to load tokens: %w", err)
	}

	if tokens.Auth == nil {
		return nil, fmt.Errorf("auth whoami: no credentials found for profile %q", profile)
	}

	if authRef == "" {
		authRef = profile
	}

	entry, ok := tokens.Auth[authRef]
	if !ok {
		return nil, fmt.Errorf("auth whoami: no credentials found for profile %q (auth_ref %q)", profile, authRef)
	}

	// token_expiry から有効期限チェック
	var expiresAt *string
	var expired bool
	if entry.TokenExpiry != "" {
		ea := entry.TokenExpiry
		expiresAt = &ea
		expired = entry.IsExpired()
	}

	resp := authWhoamiResponse{
		SchemaVersion: "1",
		Profile:       profile,
		AuthType:      entry.AuthType,
		ExpiresAt:     expiresAt,
		Expired:       expired,
		User:          user,
	}
	return resp, nil
}

type authWhoamiResponse struct {
	SchemaVersion string       `json:"schema_version"`
	Profile       string       `json:"profile"`
	Space         string       `json:"space,omitempty"`
	AuthType      string       `json:"auth_type"`
	ExpiresAt     *string      `json:"expires_at,omitempty"`
	Expired       bool         `json:"expired"`
	User          *domain.User `json:"user"`
}

// ---- auth list ----

// AuthListCmd は auth list コマンド。
type AuthListCmd struct{}

// Run は auth list のメインエントリポイント（Kong から呼び出し）。
func (c *AuthListCmd) Run(g *GlobalFlags) error {
	store := credentials.NewStore(credentials.DefaultTokensPath(func(key string) string {
		return ""
	}))
	resp, err := c.RunWithStoreCapture(g, store)
	if err != nil {
		return err
	}
	renderer, rerr := buildRenderer(g)
	if rerr != nil {
		return rerr
	}
	return renderer.Render(os.Stdout, resp)
}

// RunWithStoreCapture は tokens.json の全エントリ一覧をレスポンス struct で返す（テスト用DI）。
// レンダリングは呼び出し元が行う。
func (c *AuthListCmd) RunWithStoreCapture(g *GlobalFlags, store credentials.Store) (any, error) {
	tokens, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("auth list: failed to load tokens: %w", err)
	}

	profiles := make([]authListProfileEntry, 0, len(tokens.Auth))
	for profileName, entry := range tokens.Auth {
		pe := authListProfileEntry{
			Profile:       profileName,
			AuthType:      entry.AuthType,
			Authenticated: true,
		}
		if entry.AuthType == credentials.AuthTypeOAuth && entry.TokenExpiry != "" {
			pe.ExpiresAt = entry.TokenExpiry
			pe.Expired = entry.IsExpired()
		}
		profiles = append(profiles, pe)
	}

	// 安定したソート（プロファイル名順）
	sortProfileEntries(profiles)

	resp := authListResponse{
		SchemaVersion: "1",
		Profiles:      profiles,
	}
	return resp, nil
}

// authListProfileEntry は auth list レスポンスの各プロファイルエントリ。
type authListProfileEntry struct {
	Profile       string `json:"profile"`
	Space         string `json:"space,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	AuthType      string `json:"auth_type"`
	Authenticated bool   `json:"authenticated"`
	Expired       bool   `json:"expired,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
}

type authListResponse struct {
	SchemaVersion string                 `json:"schema_version"`
	Profiles      []authListProfileEntry `json:"profiles"`
}

// resolveProfile は GlobalFlags と config.toml の default_profile からプロファイル名を解決する。
// --profile フラグが指定されている場合はそれを優先し、
// 未指定の場合は config.toml の default_profile を使用する。
func resolveProfile(g *GlobalFlags) (string, error) {
	if g.Profile != "" {
		return g.Profile, nil
	}
	configPath := config.ResolveConfigPath(g.Config, os.Getenv)
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("設定ファイルの読み込みに失敗: %w", err)
	}
	flags := config.OverrideFlags{Profile: g.Profile}
	resolved, err := config.Resolve(cfg, flags, os.Getenv)
	if err != nil {
		return "", fmt.Errorf("設定の解決に失敗: %w", err)
	}
	if resolved.Profile == "" {
		return "", fmt.Errorf("--profile が必要です。config.toml の default_profile を設定するか --profile を指定してください")
	}
	return resolved.Profile, nil
}

// sortProfileEntries はプロファイルエントリをプロファイル名でソートする（挿入ソート）。
func sortProfileEntries(entries []authListProfileEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Profile < entries[j-1].Profile; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

