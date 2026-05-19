package space

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/config"
)

// ResolverOption は Resolver の構成オプション。
type ResolverOption func(*Resolver)

// WithLegacyProfileFallback は fallback 5 で既存 profile を使う設定を提供する（RM2 対応）。
// CLI では buildRunContext が提供する config.ResolvedConfig を渡す。
// remote MCP では nil を渡す（fallback 5 無効化）。
func WithLegacyProfileFallback(cfg *config.ResolvedConfig) ResolverOption {
	return func(r *Resolver) {
		r.legacyCfg = cfg
	}
}

// Resolver は Scope から SpaceRegistration 一覧を解決する。
type Resolver struct {
	store     Store
	legacyCfg *config.ResolvedConfig // nil なら fallback 5 は無効
}

// NewResolver は Resolver を生成する。
func NewResolver(store Store, opts ...ResolverOption) *Resolver {
	r := &Resolver{store: store}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve は Scope から対象 SpaceRegistration 一覧を解決する。
//
// 解決優先順位（spec §5.3）:
//  1. scope.AllSpaces == true → enabled spaces 全件
//  2. len(scope.Aliases) > 0 → 指定 alias を解決（1つでも未登録ならエラー）
//  3. UserPreference.DefaultSpaceAlias → default
//  4. enabled space が 1件だけ → それを使う
//  5. WithLegacyProfileFallback で渡した config から生成（CLI 専用）
//  6. ErrNoDefaultSpace
func (r *Resolver) Resolve(ctx context.Context, userID string, scope Scope) ([]SpaceRegistration, error) {
	if scope.AllSpaces && len(scope.Aliases) > 0 {
		return nil, ErrInvalidSpaceScope
	}

	// fallback 1: AllSpaces
	if scope.AllSpaces {
		all, err := r.store.List(ctx, userID)
		if err != nil {
			return nil, err
		}
		var enabled []SpaceRegistration
		for _, s := range all {
			if !s.Disabled && s.Status != SpaceStatusDisabled {
				enabled = append(enabled, s)
			}
		}
		if len(enabled) == 0 {
			return nil, ErrNoSpacesRegistered
		}
		return enabled, nil
	}

	// fallback 2: Aliases
	if len(scope.Aliases) > 0 {
		result := make([]SpaceRegistration, 0, len(scope.Aliases))
		for _, alias := range scope.Aliases {
			reg, err := r.store.Get(ctx, userID, alias)
			if err != nil {
				return nil, err
			}
			if reg == nil {
				return nil, fmt.Errorf("%w: %s", ErrSpaceNotFound, alias)
			}
			result = append(result, *reg)
		}
		return result, nil
	}

	// fallback 3-4: preference / single enabled space
	spaces, err := r.store.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	var enabled []SpaceRegistration
	for _, s := range spaces {
		if !s.Disabled && s.Status != SpaceStatusDisabled {
			enabled = append(enabled, s)
		}
	}

	// fallback 3: DefaultSpaceAlias from preference
	pref, err := r.store.GetPreference(ctx, userID)
	if err != nil {
		return nil, err
	}
	if pref != nil && pref.DefaultSpaceAlias != "" {
		for _, s := range spaces {
			if s.Alias == pref.DefaultSpaceAlias {
				return []SpaceRegistration{s}, nil
			}
		}
	}

	// fallback 4: single enabled space
	if len(enabled) == 1 {
		return []SpaceRegistration{enabled[0]}, nil
	}

	// fallback 5: legacy profile
	if r.legacyCfg != nil && len(spaces) == 0 {
		reg, err := resolveFromLegacyProfile(r.legacyCfg, userID)
		if err != nil {
			return nil, err
		}
		return []SpaceRegistration{*reg}, nil
	}

	return nil, ErrNoDefaultSpace
}

// resolveFromLegacyProfile は legacy config から SpaceRegistration を生成する（BC 対応）。
func resolveFromLegacyProfile(cfg *config.ResolvedConfig, userID string) (*SpaceRegistration, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" && cfg.Space != "" {
		baseURL = fmt.Sprintf("https://%s.backlog.com", cfg.Space)
	}
	if baseURL == "" {
		return nil, ErrNoDefaultSpace
	}
	tenant := cfg.Space
	return &SpaceRegistration{
		UserID:      userID,
		Alias:       tenant,
		Tenant:      tenant,
		BaseURL:     baseURL,
		AuthType:    AuthTypeAPIKey,
		AuthProfile: cfg.AuthRef,
		Status:      SpaceStatusUnknown,
	}, nil
}
