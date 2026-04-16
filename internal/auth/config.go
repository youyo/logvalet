package auth

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// 環境変数キー名定数
const (
	EnvTokenStore      = "LOGVALET_TOKEN_STORE"
	EnvSQLitePath      = "LOGVALET_TOKEN_STORE_SQLITE_PATH"
	EnvDynamoDBTable   = "LOGVALET_TOKEN_STORE_DYNAMODB_TABLE"
	EnvDynamoDBRegion  = "LOGVALET_TOKEN_STORE_DYNAMODB_REGION"
	EnvBacklogClientID     = "LOGVALET_BACKLOG_CLIENT_ID"
	EnvBacklogClientSecret = "LOGVALET_BACKLOG_CLIENT_SECRET"
	EnvBacklogRedirectURL  = "LOGVALET_BACKLOG_REDIRECT_URL"
	EnvOAuthStateSecret    = "LOGVALET_OAUTH_STATE_SECRET"
)

// デフォルト値
const (
	DefaultTokenStoreType = StoreTypeMemory
	DefaultSQLitePath     = "./logvalet.db"
)

// OAuthStateSecret の最小デコード長（バイト数）。HMAC-SHA256 向けに 16 bytes を要求する。
const minStateSecretBytes = 16

// StoreType は TokenStore の種別を表す。
type StoreType string

const (
	// StoreTypeMemory はインメモリ TokenStore。開発・テスト・PoC 向け。
	StoreTypeMemory StoreType = "memory"
	// StoreTypeSQLite は SQLite TokenStore。ローカル CLI・自己ホスト向け。
	StoreTypeSQLite StoreType = "sqlite"
	// StoreTypeDynamoDB は DynamoDB TokenStore。Lambda 本命。
	StoreTypeDynamoDB StoreType = "dynamodb"
)

// ErrInvalidStoreType は不正な TokenStore 種別が指定された場合に返される。
var ErrInvalidStoreType = errors.New("auth: invalid token store type")

// OAuthEnvConfig は OAuth 関連の環境変数設定を保持する。
// LoadOAuthEnvConfig() で構築し、Validate() で必須項目チェックを行う。
type OAuthEnvConfig struct {
	// TokenStore 設定
	TokenStoreType StoreType // LOGVALET_TOKEN_STORE (デフォルト: memory)
	SQLitePath     string    // LOGVALET_TOKEN_STORE_SQLITE_PATH (デフォルト: ./logvalet.db)
	DynamoDBTable  string    // LOGVALET_TOKEN_STORE_DYNAMODB_TABLE
	DynamoDBRegion string    // LOGVALET_TOKEN_STORE_DYNAMODB_REGION

	// Backlog OAuth 設定
	BacklogClientID     string // LOGVALET_BACKLOG_CLIENT_ID
	BacklogClientSecret string // LOGVALET_BACKLOG_CLIENT_SECRET
	BacklogRedirectURL  string // LOGVALET_BACKLOG_REDIRECT_URL

	// OAuth State 設定
	OAuthStateSecret string // LOGVALET_OAUTH_STATE_SECRET (hex エンコード)
}

// parseStoreType は文字列を StoreType に変換する。
// 空文字列の場合はデフォルト (memory) を返す。
// 不正値の場合は ErrInvalidStoreType を返す。
func parseStoreType(s string) (StoreType, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	switch normalized {
	case "", "memory":
		return StoreTypeMemory, nil
	case "sqlite":
		return StoreTypeSQLite, nil
	case "dynamodb":
		return StoreTypeDynamoDB, nil
	default:
		return "", fmt.Errorf("%w: %q (valid: memory, sqlite, dynamodb)", ErrInvalidStoreType, s)
	}
}

// LoadOAuthEnvConfig は環境変数から OAuthEnvConfig を構築する。
// getenv は依存注入用の関数で、通常は os.Getenv を渡す。
// Load 後に Validate() を呼び出して必須項目チェックを行うこと。
func LoadOAuthEnvConfig(getenv func(string) string) (*OAuthEnvConfig, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	storeType, err := parseStoreType(getenv(EnvTokenStore))
	if err != nil {
		return nil, err
	}

	sqlitePath := getenv(EnvSQLitePath)
	if sqlitePath == "" {
		sqlitePath = DefaultSQLitePath
	}

	return &OAuthEnvConfig{
		TokenStoreType:      storeType,
		SQLitePath:          sqlitePath,
		DynamoDBTable:       getenv(EnvDynamoDBTable),
		DynamoDBRegion:      getenv(EnvDynamoDBRegion),
		BacklogClientID:     getenv(EnvBacklogClientID),
		BacklogClientSecret: getenv(EnvBacklogClientSecret),
		BacklogRedirectURL:  getenv(EnvBacklogRedirectURL),
		OAuthStateSecret:    getenv(EnvOAuthStateSecret),
	}, nil
}

// OAuthEnabled は OAuth 機能が有効かどうかを返す。
// BacklogClientID が設定されている場合に true を返す。
func (c *OAuthEnvConfig) OAuthEnabled() bool {
	return c.BacklogClientID != ""
}

// Validate は OAuthEnvConfig の必須項目チェックを行う。
// OAuth が有効な場合（ClientID 設定済み）は、ClientSecret, RedirectURL, StateSecret を必須とする。
// DynamoDB 選択時は Table, Region を必須とする。
// 複数のエラーがある場合は errors.Join で集約して返す。
func (c *OAuthEnvConfig) Validate() error {
	var errs []error

	// OAuth 有効時の必須チェック
	if c.OAuthEnabled() {
		if c.BacklogClientSecret == "" {
			errs = append(errs, fmt.Errorf("auth: %s is required when OAuth is enabled", EnvBacklogClientSecret))
		}
		if c.BacklogRedirectURL == "" {
			errs = append(errs, fmt.Errorf("auth: %s is required when OAuth is enabled", EnvBacklogRedirectURL))
		}
		if c.OAuthStateSecret == "" {
			errs = append(errs, fmt.Errorf("auth: %s is required when OAuth is enabled", EnvOAuthStateSecret))
		} else {
			// hex 形式チェック
			decoded, err := hex.DecodeString(c.OAuthStateSecret)
			if err != nil {
				errs = append(errs, fmt.Errorf("auth: %s must be a valid hex-encoded string: %w", EnvOAuthStateSecret, err))
			} else if len(decoded) < minStateSecretBytes {
				errs = append(errs, fmt.Errorf("auth: %s must be at least 16 bytes (32 hex chars), got %d bytes", EnvOAuthStateSecret, len(decoded)))
			}
		}
	}

	// DynamoDB 選択時の必須チェック
	if c.TokenStoreType == StoreTypeDynamoDB {
		if c.DynamoDBTable == "" {
			errs = append(errs, fmt.Errorf("auth: %s is required when token store is dynamodb", EnvDynamoDBTable))
		}
		if c.DynamoDBRegion == "" {
			errs = append(errs, fmt.Errorf("auth: %s is required when token store is dynamodb", EnvDynamoDBRegion))
		}
	}

	return errors.Join(errs...)
}
