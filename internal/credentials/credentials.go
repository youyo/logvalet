// Package credentials は logvalet の認証情報管理を提供する。
//
// tokens.json のスキーマ定義、ストア（読み書き）、認証情報リゾルバーを実装する。
// ファイルパス: ~/.config/logvalet/tokens.json (または XDG_CONFIG_HOME)
// spec §5 準拠。
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AuthTypeOAuth は OAuth 認証タイプの定数。
const AuthTypeOAuth = "oauth"

// AuthTypeAPIKey は API Key 認証タイプの定数。
const AuthTypeAPIKey = "api_key"

// TokensFile は tokens.json 全体構造。
// spec §5 tokens.json schema 準拠。
type TokensFile struct {
	Version int                  `json:"version"`
	Auth    map[string]AuthEntry `json:"auth"`
}

// AuthEntry は1プロファイル分の認証エントリ。
type AuthEntry struct {
	AuthType     string `json:"auth_type"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenExpiry  string `json:"token_expiry,omitempty"` // ISO 8601 (RFC3339)
	APIKey       string `json:"api_key,omitempty"`
}

// IsExpired は oauth エントリの token_expiry が現在時刻より前かどうかを返す。
// auth_type が oauth 以外の場合は常に false を返す。
// token_expiry が空文字の場合も false を返す（期限なし扱い）。
func (e AuthEntry) IsExpired() bool {
	if e.AuthType != AuthTypeOAuth {
		return false
	}
	if e.TokenExpiry == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, e.TokenExpiry)
	if err != nil {
		// パースエラーは期限切れ扱いにしない（安全側に倒す）
		return false
	}
	return time.Now().After(expiry)
}

// Store は tokens.json を読み書きするインターフェース。
type Store interface {
	Load() (*TokensFile, error)
	Save(tokens *TokensFile) error
	Path() string
}

// fileStore は Store の標準実装。
type fileStore struct {
	path string
}

// NewStore は指定パスに対する Store を返す。
func NewStore(path string) Store {
	return &fileStore{path: path}
}

// Path は tokens.json のファイルパスを返す。
func (s *fileStore) Path() string {
	return s.path
}

// Load は tokens.json をロードして TokensFile を返す。
// ファイルが存在しない場合はゼロ値の TokensFile を返す（エラーなし）。
// JSON パースエラーの場合はエラーを返す。
func (s *fileStore) Load() (*TokensFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// ファイル不在は正常ケース（初回起動等）
			return &TokensFile{}, nil
		}
		return nil, fmt.Errorf("credentials: failed to read %s: %w", s.path, err)
	}
	var tokens TokensFile
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("credentials: failed to parse %s: %w", s.path, err)
	}
	return &tokens, nil
}

// Save は TokensFile を tokens.json に保存する。
// tempfile + rename パターンでアトミック書き込みを保証する。
// ディレクトリが存在しない場合は作成する。
// ファイルのパーミッションは 0600（owner read/write のみ）。
func (s *fileStore) Save(tokens *TokensFile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("credentials: failed to create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("credentials: failed to marshal tokens: %w", err)
	}

	// tempfile + rename でアトミック書き込み
	tmpPath := fmt.Sprintf("%s.tmp.%d", s.path, os.Getpid())
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("credentials: failed to write temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		// rename 失敗時は一時ファイルを削除
		_ = os.Remove(tmpPath)
		return fmt.Errorf("credentials: failed to rename %s to %s: %w", tmpPath, s.path, err)
	}

	return nil
}

// DefaultTokensPath はデフォルトの tokens.json パスを返す。
// getenv は環境変数取得関数（テスト用DI）。
// XDG_CONFIG_HOME が設定されていれば $XDG_CONFIG_HOME/logvalet/tokens.json、
// そうでなければ $HOME/.config/logvalet/tokens.json を返す。
func DefaultTokensPath(getenv func(string) string) string {
	xdgConfigHome := getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "logvalet", "tokens.json")
	}
	// HOME 環境変数またはデフォルトのホームディレクトリを使用
	home := getenv("HOME")
	if home == "" {
		// HOME が設定されていない場合は os.UserHomeDir にフォールバック
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return filepath.Join(".config", "logvalet", "tokens.json")
		}
	}
	return filepath.Join(home, ".config", "logvalet", "tokens.json")
}

// CredentialFlags は CLI フラグからの認証情報上書き値。
type CredentialFlags struct {
	APIKey      string
	AccessToken string
}

// ResolvedCredential は優先順位解決後の認証情報。
type ResolvedCredential struct {
	AuthType    string // "oauth" | "api_key"
	AccessToken string
	RefreshToken string
	TokenExpiry  string
	APIKey      string
	Source      string // "flag" | "env" | "tokens_json"
}

// Resolver は認証情報を優先順位に従い解決するインターフェース。
// 優先順位（高い順）: CLI flags > 環境変数 > tokens.json
type Resolver interface {
	Resolve(authRef string, flags CredentialFlags, getenv func(string) string) (*ResolvedCredential, error)
}

// credResolver は Resolver の標準実装。
type credResolver struct {
	store Store
}

// NewResolver は Store を使用する Resolver を返す。
func NewResolver(store Store) Resolver {
	return &credResolver{store: store}
}

// Resolve は優先順位に従い認証情報を解決する。
//
// 優先順位（高い順）:
//  1. flags.APIKey（--api-key フラグ）
//  2. flags.AccessToken（--access-token フラグ）
//  3. tokens.json の authRef エントリ（プロファイル固有）
//  4. getenv("LOGVALET_API_KEY")
//  5. getenv("LOGVALET_ACCESS_TOKEN")
//
// いずれも存在しない場合はエラーを返す。
func (r *credResolver) Resolve(authRef string, flags CredentialFlags, getenv func(string) string) (*ResolvedCredential, error) {
	// 1. --api-key フラグ
	if flags.APIKey != "" {
		return &ResolvedCredential{
			AuthType: AuthTypeAPIKey,
			APIKey:   flags.APIKey,
			Source:   "flag",
		}, nil
	}

	// 2. --access-token フラグ
	if flags.AccessToken != "" {
		return &ResolvedCredential{
			AuthType:    AuthTypeOAuth,
			AccessToken: flags.AccessToken,
			Source:      "flag",
		}, nil
	}

	// 3. tokens.json（プロファイル固有の認証情報）
	if authRef != "" {
		tokens, err := r.store.Load()
		if err != nil {
			return nil, fmt.Errorf("credentials: failed to load tokens: %w", err)
		}
		if entry, ok := tokens.Auth[authRef]; ok {
			return &ResolvedCredential{
				AuthType:     entry.AuthType,
				AccessToken:  entry.AccessToken,
				RefreshToken: entry.RefreshToken,
				TokenExpiry:  entry.TokenExpiry,
				APIKey:       entry.APIKey,
				Source:       "tokens_json",
			}, nil
		}
	}

	// 4. 環境変数 LOGVALET_API_KEY
	if apiKey := getenv("LOGVALET_API_KEY"); apiKey != "" {
		return &ResolvedCredential{
			AuthType: AuthTypeAPIKey,
			APIKey:   apiKey,
			Source:   "env",
		}, nil
	}

	// 5. 環境変数 LOGVALET_ACCESS_TOKEN
	if accessToken := getenv("LOGVALET_ACCESS_TOKEN"); accessToken != "" {
		return &ResolvedCredential{
			AuthType:    AuthTypeOAuth,
			AccessToken: accessToken,
			Source:      "env",
		}, nil
	}

	// いずれも見つからない
	if authRef == "" {
		return nil, fmt.Errorf("credentials: no credentials found (no flags, no env vars, no auth_ref configured)")
	}
	return nil, fmt.Errorf("credentials: no credentials found for auth_ref %q in tokens.json", authRef)
}
