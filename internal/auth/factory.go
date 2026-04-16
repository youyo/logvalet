package auth

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
)

// ClientFactory は context からユーザーを特定し、そのユーザーの
// Backlog OAuth トークンを使った backlog.Client を生成する関数型。
type ClientFactory func(ctx context.Context) (backlog.Client, error)

// NewClientFactory は TokenManager を使って per-user の backlog.Client を
// 生成する ClientFactory を返す。
//
// フロー:
//  1. ctx から UserIDFromContext で userID を取得（なければ ErrUnauthenticated）
//  2. tm.GetValidToken で有効なトークンを取得
//  3. backlog.NewHTTPClient で Bearer トークン付きクライアントを生成
func NewClientFactory(tm TokenManager, provider, tenant, baseURL string) ClientFactory {
	return func(ctx context.Context) (backlog.Client, error) {
		userID, ok := UserIDFromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("client factory: userID not found in context: %w", ErrUnauthenticated)
		}

		rec, err := tm.GetValidToken(ctx, userID, provider, tenant)
		if err != nil {
			return nil, fmt.Errorf("client factory: get token for user %q: %w", userID, err)
		}

		cred := &credentials.ResolvedCredential{
			AuthType:    credentials.AuthTypeOAuth,
			AccessToken: rec.AccessToken,
			Source:      "oauth_token_manager",
		}

		client := backlog.NewHTTPClient(backlog.ClientConfig{
			BaseURL:    baseURL,
			Credential: cred,
		})

		return client, nil
	}
}
