// Package auth は OAuth トークン管理のための型とエラーを提供する。
package auth

import "errors"

// センチネルエラー — errors.Is() でラップされたエラーの判定に使用する。
var (
	// ErrUnauthenticated はユーザーが認証されていない場合に返される。
	ErrUnauthenticated = errors.New("auth: user not authenticated")

	// ErrProviderNotConnected はユーザーに対して指定プロバイダーが未接続の場合に返される。
	ErrProviderNotConnected = errors.New("auth: provider not connected for this user")

	// ErrTokenExpired はトークンが期限切れでリフレッシュも失敗した場合に返される。
	ErrTokenExpired = errors.New("auth: token expired and refresh failed")

	// ErrTokenRefreshFailed はトークンのリフレッシュに失敗した場合に返される。
	ErrTokenRefreshFailed = errors.New("auth: token refresh failed")

	// ErrProviderUserMismatch はプロバイダーのユーザー情報が一致しない場合に返される。
	ErrProviderUserMismatch = errors.New("auth: provider user does not match")

	// ErrInvalidTenant はテナントが無効または未指定の場合に返される。
	ErrInvalidTenant = errors.New("auth: invalid or missing tenant")
)
