package cli

import (
	"errors"

	"github.com/youyo/logvalet/internal/auth"
)

// needsBacklogAuthorization は GetValidToken のエラーが Backlog 認可を要求するものかを判定する。
// ホワイトリスト方式で判定し、予期しないエラー（ErrUnauthenticated 等）は対象外とする。
//
// この関数は EnsureBacklogConnected (mcp_auto_redirect.go) と
// BacklogAuthorizeGate (backlog_authorize_gate.go) の両方から参照される。
func needsBacklogAuthorization(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, auth.ErrProviderNotConnected) ||
		errors.Is(err, auth.ErrTokenRefreshFailed) ||
		errors.Is(err, auth.ErrTokenExpired)
}
