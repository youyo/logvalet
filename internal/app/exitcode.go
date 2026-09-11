// Package app はアプリケーション共通の定数・型を提供する。
package app

// Exit codes は logvalet CLI の終了コード定義。
// spec §8 に準拠。
const (
	ExitSuccess             = 0
	ExitGenericError        = 1
	ExitArgumentError       = 2
	ExitAuthenticationError = 3
	ExitPermissionError     = 4
	ExitNotFoundError       = 5
	ExitAPIError            = 6
	ExitDigestError         = 7
	ExitConfigError         = 10
)
