package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
)

// passthroughContextKey は context に Backlog credential passthrough トークンを
// 格納するためのキー型。unexported にすることで context.go の contextKey (userID 用)
// や他パッケージのキーと衝突しない。
//
// docs/specs/gateway-request-contract.md §4 参照: HTTP(Gateway) モードでは、
// AgentCore Gateway が logvalet へのリクエストの Authorization ヘッダーに
// per-user の Backlog OAuth access token を注入する。logvalet はこの値を
// 検証・デコード・キャッシュせず、そのまま Backlog API 呼び出しへ転送する
// (passthrough)。
type passthroughContextKey struct{}

// ErrPassthroughTokenMissing は Backlog credential passthrough トークンが
// context に存在しない場合に返されるセンチネルエラー。
// HTTP(Gateway) モードで Authorization: Bearer ヘッダーが欠落しているリクエストに対する
// fail-fast 判定に使う (契約 §4.3)。
var ErrPassthroughTokenMissing = errors.New("auth: backlog credential passthrough token missing")

// ContextWithPassthroughToken は ctx に Backlog credential passthrough トークンを
// 設定して返す。トークンはリクエストスコープの context にのみ保持され、
// logvalet プロセス内のグローバル状態や tokenstore 等の永続ストアには一切保存されない。
func ContextWithPassthroughToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, passthroughContextKey{}, token)
}

// PassthroughTokenFromContext は ctx から Backlog credential passthrough トークンを
// 取得する。キーが存在しない場合、または空文字列の場合は ("", false) を返す。
func PassthroughTokenFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(passthroughContextKey{}).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// NewPassthroughClientFactory は context の passthrough トークンのみを使う
// ClientFactory を返す。トークンが context に無い場合は ErrPassthroughTokenMissing を
// 返す (fail-fast。契約 §4.3: Backlog API 呼び出しを試みずに即座にエラーを返す)。
//
// per-user token 解決 (TokenManager/tokenstore) には一切依存しない。HTTP(Gateway)
// モードでの単独利用を想定している。
func NewPassthroughClientFactory(baseURL string) ClientFactory {
	return func(ctx context.Context) (backlog.Client, error) {
		token, ok := PassthroughTokenFromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("passthrough client factory: %w", ErrPassthroughTokenMissing)
		}

		cred := &credentials.ResolvedCredential{
			AuthType:    credentials.AuthTypeOAuth,
			AccessToken: token,
			Source:      "gateway_bearer_passthrough",
		}

		return backlog.NewHTTPClient(backlog.ClientConfig{
			BaseURL:    baseURL,
			Credential: cred,
		}), nil
	}
}

// NewPassthroughAwareClientFactory は既存の per-user token 解決方式 (fallback) と
// Backlog credential passthrough を、リクエストごとに排他的に選択する ClientFactory を
// 返す。
//
// 選択規則: context に passthrough トークンが存在すればそれを優先して使い、
// tokenstore 等の per-user 解決には一切触れない。存在しなければ fallback
// (例: NewClientFactory が返す ClientFactory) にそのまま委譲する。
// fallback が nil の場合、passthrough トークンが無いリクエストは
// ErrPassthroughTokenMissing を返す (HTTP(Gateway) モード専用構成)。
func NewPassthroughAwareClientFactory(baseURL string, fallback ClientFactory) ClientFactory {
	passthroughFactory := NewPassthroughClientFactory(baseURL)
	return func(ctx context.Context) (backlog.Client, error) {
		if _, ok := PassthroughTokenFromContext(ctx); ok {
			return passthroughFactory(ctx)
		}
		if fallback == nil {
			return nil, fmt.Errorf("passthrough-aware client factory: %w", ErrPassthroughTokenMissing)
		}
		return fallback(ctx)
	}
}
