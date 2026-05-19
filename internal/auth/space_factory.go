package auth

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/space"
)

// SpaceAwareClientFactory は (ctx, SpaceRegistration) → backlog.Client を返す関数型。
// 認証方式（OAuth / APIKey）を内部で分岐する。
// 既存の ClientFactory とは異なり、tenant/baseURL が動的（RC1 対応）。
type SpaceAwareClientFactory func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error)

// NewSpaceAwareClientFactory は認証方式を内部で分岐する SpaceAwareClientFactory を返す。
//
// OAuth パス:
//
//	ctx → UserIDFromContext → tm.GetValidToken(userID, "backlog", reg.Tenant)
//	→ backlog.NewHTTPClient(Bearer)
//
// APIKey パス:
//
//	credResolver.Resolve(reg.AuthProfile) → backlog.NewHTTPClient(APIKey)
//
// H5: 同一 tenant の refresh は TokenManager の singleflight が自動的に dedup する。
// factory 側では特別な処理は不要。
func NewSpaceAwareClientFactory(
	tm TokenManager,
	credResolver credentials.Resolver,
) SpaceAwareClientFactory {
	return func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		switch reg.AuthType {
		case space.AuthTypeOAuth:
			return buildOAuthClient(ctx, tm, reg)
		case space.AuthTypeAPIKey:
			return buildAPIKeyClient(credResolver, reg)
		default:
			return nil, fmt.Errorf("space factory: unknown auth type %q for space %q", reg.AuthType, reg.Alias)
		}
	}
}

func buildOAuthClient(ctx context.Context, tm TokenManager, reg space.SpaceRegistration) (backlog.Client, error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("space factory: userID not in context: %w", ErrUnauthenticated)
	}

	rec, err := tm.GetValidToken(ctx, userID, "backlog", reg.Tenant)
	if err != nil {
		return nil, fmt.Errorf("space factory: get token for space %q (user %q): %w", reg.Alias, userID, err)
	}

	cred := &credentials.ResolvedCredential{
		AuthType:    credentials.AuthTypeOAuth,
		AccessToken: rec.AccessToken,
		Source:      "oauth_token_manager",
	}

	return backlog.NewHTTPClient(backlog.ClientConfig{
		BaseURL:    reg.BaseURL,
		Credential: cred,
	}), nil
}

func buildAPIKeyClient(credResolver credentials.Resolver, reg space.SpaceRegistration) (backlog.Client, error) {
	cred, err := credResolver.Resolve(reg.AuthProfile, credentials.CredentialFlags{}, func(s string) string {
		return ""
	})
	if err != nil {
		return nil, fmt.Errorf("space factory: resolve credential for space %q (profile %q): %w",
			reg.Alias, reg.AuthProfile, err)
	}

	return backlog.NewHTTPClient(backlog.ClientConfig{
		BaseURL:    reg.BaseURL,
		Credential: cred,
	}), nil
}
