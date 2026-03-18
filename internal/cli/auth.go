package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/config"
	"github.com/youyo/logvalet/internal/credentials"
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
	output, err := c.RunWithLoginRequestCapture(g, store, req)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
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
// 出力 JSON 文字列を返す。
func (c *AuthLoginCmd) RunWithLoginRequestCapture(g *GlobalFlags, store credentials.Store, req AuthLoginRequest) (string, error) {
	if g.Profile == "" {
		return "", fmt.Errorf("auth login: --profile is required")
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
	tokens.Auth[g.Profile] = entry

	if err := store.Save(tokens); err != nil {
		return "", fmt.Errorf("auth login: failed to save tokens: %w", err)
	}

	// レスポンス JSON 生成
	resp := authLoginResponse{
		SchemaVersion: "1",
		Result:        "ok",
		Profile:       g.Profile,
		Space:         req.Space,
		BaseURL:       req.BaseURL,
		AuthType:      req.AuthType,
		Saved:         true,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("auth login: failed to marshal response: %w", err)
	}
	return string(data), nil
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
	return c.RunWithStore(g, store)
}

// RunWithStore は tokens.json から指定プロファイルのエントリを削除する（テスト用DI）。
func (c *AuthLogoutCmd) RunWithStore(g *GlobalFlags, store credentials.Store) error {
	if g.Profile == "" {
		return fmt.Errorf("auth logout: --profile is required")
	}

	tokens, err := store.Load()
	if err != nil {
		return fmt.Errorf("auth logout: failed to load tokens: %w", err)
	}

	if tokens.Auth == nil || len(tokens.Auth) == 0 {
		return fmt.Errorf("auth logout: profile %q not found in tokens.json", g.Profile)
	}

	if _, ok := tokens.Auth[g.Profile]; !ok {
		return fmt.Errorf("auth logout: profile %q not found in tokens.json", g.Profile)
	}

	delete(tokens.Auth, g.Profile)

	if err := store.Save(tokens); err != nil {
		return fmt.Errorf("auth logout: failed to save tokens: %w", err)
	}

	resp := authLogoutResponse{
		SchemaVersion: "1",
		Result:        "ok",
		Profile:       g.Profile,
		Removed:       true,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("auth logout: failed to marshal response: %w", err)
	}
	// stdout に出力（spec §7 stdout は機械可読な結果のみ）
	fmt.Println(string(data))
	return nil
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
	authRef := g.Profile // M03: auth_ref = profile 名で簡略化
	output, err := c.RunWithStoreCapture(g, store, authRef)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

// RunWithStoreCapture は tokens.json から認証情報を取得して JSON 文字列を返す（テスト用DI）。
// M03: Backlog API 呼び出しなし。tokens.json の情報のみ表示。
func (c *AuthWhoamiCmd) RunWithStoreCapture(g *GlobalFlags, store credentials.Store, authRef string) (string, error) {
	if g.Profile == "" {
		return "", fmt.Errorf("auth whoami: --profile is required")
	}

	tokens, err := store.Load()
	if err != nil {
		return "", fmt.Errorf("auth whoami: failed to load tokens: %w", err)
	}

	if tokens.Auth == nil {
		return "", fmt.Errorf("auth whoami: no credentials found for profile %q", g.Profile)
	}

	entry, ok := tokens.Auth[authRef]
	if !ok {
		return "", fmt.Errorf("auth whoami: no credentials found for profile %q (auth_ref %q)", g.Profile, authRef)
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
		Profile:       g.Profile,
		AuthType:      entry.AuthType,
		ExpiresAt:     expiresAt,
		Expired:       expired,
		User:          nil, // M04 以降で Backlog API 呼び出しで取得
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("auth whoami: failed to marshal response: %w", err)
	}
	return string(data), nil
}

type authWhoamiResponse struct {
	SchemaVersion string      `json:"schema_version"`
	Profile       string      `json:"profile"`
	Space         string      `json:"space,omitempty"`
	AuthType      string      `json:"auth_type"`
	ExpiresAt     *string     `json:"expires_at,omitempty"`
	Expired       bool        `json:"expired"`
	User          interface{} `json:"user"`
}

// ---- auth list ----

// AuthListCmd は auth list コマンド。
type AuthListCmd struct{}

// Run は auth list のメインエントリポイント（Kong から呼び出し）。
func (c *AuthListCmd) Run(g *GlobalFlags) error {
	store := credentials.NewStore(credentials.DefaultTokensPath(func(key string) string {
		return ""
	}))
	output, err := c.RunWithStoreCapture(g, store)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

// RunWithStoreCapture は tokens.json の全エントリ一覧を JSON 文字列で返す（テスト用DI）。
func (c *AuthListCmd) RunWithStoreCapture(g *GlobalFlags, store credentials.Store) (string, error) {
	tokens, err := store.Load()
	if err != nil {
		return "", fmt.Errorf("auth list: failed to load tokens: %w", err)
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
	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("auth list: failed to marshal response: %w", err)
	}
	return string(data), nil
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

// sortProfileEntries はプロファイルエントリをプロファイル名でソートする（挿入ソート）。
func sortProfileEntries(entries []authListProfileEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Profile < entries[j-1].Profile; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

