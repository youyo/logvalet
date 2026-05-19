package space

import "time"

type AuthType string

const (
	// AuthTypeOAuth は OAuth 認証。
	AuthTypeOAuth AuthType = "oauth"
	// AuthTypeAPIKey は API key 認証。
	// 既存 internal/credentials/credentials.go の AuthTypeAPIKey = "api_key" と統一する。
	AuthTypeAPIKey AuthType = "api_key"
)

type SpaceStatus string

const (
	SpaceStatusUnknown      SpaceStatus = "unknown"
	SpaceStatusOK           SpaceStatus = "ok"
	SpaceStatusUnauthorized SpaceStatus = "unauthorized"
	SpaceStatusNotConnected SpaceStatus = "not_connected"
	// SpaceStatusDisabled は将来の自動無効化機能向け。
	// MVP では disabled を設定/解除する CLI コマンドは実装しない。
	// Resolver と verify は disabled space を除外する（RH3）。
	SpaceStatusDisabled SpaceStatus = "disabled"
)

type SpaceRegistration struct {
	UserID         string
	Alias          string
	Tenant         string
	BaseURL        string
	AuthType       AuthType
	AuthProfile    string // API key mode で使う profile 名
	Provider       string // 現在は常に "backlog"
	Status         SpaceStatus
	LastVerifiedAt time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Disabled       bool
}

type UserPreference struct {
	UserID            string
	DefaultSpaceAlias string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
