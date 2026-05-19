package mcp

import (
	"context"
	"fmt"
	"net/url"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/space"
)

// RegisterSpaceRegistryTools は space 管理系 MCP tools を ToolRegistry に登録する。
// tools_space.go の既存スペース情報取得 tools（space_info, space_digest 等）とは別に、
// multi-space 構成時にのみ登録する管理操作 tools（list/use/verify/connect_url/disconnect）。
//
// NewServer / NewServerWithFactory には含めず、multi-space 対応サーバー構築時に
// 呼び出し側が明示的に呼ぶ設計（tool count が plain server と一致しなくなることを防止）。
func RegisterSpaceRegistryTools(reg *ToolRegistry, store space.Store, resolver *space.Resolver, authBaseURL string) {
	// logvalet_space_list — 登録済みスペース一覧
	reg.Register(gomcp.NewTool("logvalet_space_list",
		gomcp.WithDescription("List all Backlog spaces registered for the current user"),
		readOnlyAnnotation("スペース一覧取得"),
	), func(ctx context.Context, _ backlog.Client, args map[string]any) (any, error) {
		return spaceList(ctx, store)
	})

	// logvalet_space_use — default space 設定
	reg.Register(gomcp.NewTool("logvalet_space_use",
		gomcp.WithDescription("Set the default Backlog space for the current user"),
		writeAnnotation("デフォルトスペース設定", true),
		gomcp.WithString("alias",
			gomcp.Required(),
			gomcp.Description("Space alias to set as default"),
		),
	), func(ctx context.Context, _ backlog.Client, args map[string]any) (any, error) {
		return spaceUse(ctx, store, args)
	})

	// logvalet_space_verify — スペース接続確認
	reg.Register(gomcp.NewTool("logvalet_space_verify",
		gomcp.WithDescription("Verify connection status of registered Backlog spaces"),
		readOnlyAnnotation("スペース接続確認"),
		gomcp.WithString("alias", gomcp.Description("Target space alias (omit to use default)")),
		gomcp.WithBoolean("all_spaces", gomcp.Description("Check all registered spaces")),
	), func(ctx context.Context, _ backlog.Client, args map[string]any) (any, error) {
		return spaceVerify(ctx, store, resolver, args)
	})

	// logvalet_space_connect_url — OAuth 認可 URL 生成
	reg.Register(gomcp.NewTool("logvalet_space_connect_url",
		gomcp.WithDescription("Generate an OAuth authorization URL to connect a new Backlog space"),
		readOnlyAnnotation("スペース接続 URL 生成"),
		gomcp.WithString("base_url",
			gomcp.Required(),
			gomcp.Description("Backlog space base URL (e.g. https://myspace.backlog.com)"),
		),
		gomcp.WithString("alias", gomcp.Description("Space alias (derived from base_url if omitted)")),
	), func(ctx context.Context, _ backlog.Client, args map[string]any) (any, error) {
		return spaceConnectURL(ctx, args, authBaseURL)
	})

	// logvalet_space_disconnect — スペース削除
	reg.Register(gomcp.NewTool("logvalet_space_disconnect",
		gomcp.WithDescription("Remove a registered Backlog space for the current user"),
		destructiveAnnotation("スペース削除"),
		gomcp.WithString("alias",
			gomcp.Required(),
			gomcp.Description("Space alias to disconnect"),
		),
	), func(ctx context.Context, _ backlog.Client, args map[string]any) (any, error) {
		return spaceDisconnect(ctx, store, args)
	})
}

// spaceList は現在ユーザーの登録済みスペース一覧を返す。
func spaceList(ctx context.Context, store space.Store) (any, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("userID not found in context: authentication required")
	}

	spaces, err := store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}

	type spaceItem struct {
		Alias          string    `json:"alias"`
		Tenant         string    `json:"tenant,omitempty"`
		BaseURL        string    `json:"base_url"`
		Status         string    `json:"status"`
		AuthType       string    `json:"auth_type"`
		LastVerifiedAt time.Time `json:"last_verified_at,omitempty"`
		CreatedAt      time.Time `json:"created_at"`
	}

	items := make([]spaceItem, 0, len(spaces))
	for _, s := range spaces {
		items = append(items, spaceItem{
			Alias:          s.Alias,
			Tenant:         s.Tenant,
			BaseURL:        s.BaseURL,
			Status:         string(s.Status),
			AuthType:       string(s.AuthType),
			LastVerifiedAt: s.LastVerifiedAt,
			CreatedAt:      s.CreatedAt,
		})
	}

	pref, err := store.GetPreference(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get preference: %w", err)
	}

	defaultAlias := ""
	if pref != nil {
		defaultAlias = pref.DefaultSpaceAlias
	}

	return map[string]any{
		"spaces":        items,
		"default_alias": defaultAlias,
	}, nil
}

// spaceUse は current user の default space alias を更新する。
func spaceUse(ctx context.Context, store space.Store, args map[string]any) (any, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("userID not found in context: authentication required")
	}

	alias, ok := stringArg(args, "alias")
	if !ok || alias == "" {
		return nil, fmt.Errorf("alias is required")
	}

	pref := &space.UserPreference{
		UserID:            userID,
		DefaultSpaceAlias: alias,
		UpdatedAt:         time.Now(),
	}
	if err := store.PutPreference(ctx, pref); err != nil {
		return nil, fmt.Errorf("put preference: %w", err)
	}

	return map[string]any{
		"ok":            true,
		"default_alias": alias,
	}, nil
}

// spaceVerify はスペースの接続状態を確認して返す。
// all_spaces=true の場合は全スペース、alias 指定の場合はそのスペースのみ確認する。
// 実際のネットワーク疎通確認は行わず、Store に記録されたステータスを返す。
func spaceVerify(ctx context.Context, store space.Store, resolver *space.Resolver, args map[string]any) (any, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("userID not found in context: authentication required")
	}

	allSpaces, _ := boolArg(args, "all_spaces")
	alias, hasAlias := stringArg(args, "alias")

	var targets []space.SpaceRegistration
	var err error

	switch {
	case allSpaces:
		targets, err = store.List(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("list spaces: %w", err)
		}
	case hasAlias && alias != "":
		reg, getErr := store.Get(ctx, userID, alias)
		if getErr != nil {
			return nil, fmt.Errorf("get space: %w", getErr)
		}
		if reg == nil {
			return nil, fmt.Errorf("%w: %s", space.ErrSpaceNotFound, alias)
		}
		targets = []space.SpaceRegistration{*reg}
	default:
		scope := space.Scope{}
		targets, err = resolver.Resolve(ctx, userID, scope)
		if err != nil {
			return nil, fmt.Errorf("resolve default space: %w", err)
		}
	}

	type verifyResult struct {
		Alias     string `json:"alias"`
		BaseURL   string `json:"base_url"`
		Status    string `json:"status"`
		OK        bool   `json:"ok"`
		ErrorCode string `json:"error_code,omitempty"`
	}

	results := make([]verifyResult, 0, len(targets))
	for _, s := range targets {
		ok := s.Status == space.SpaceStatusOK
		errCode := ""
		if !ok {
			errCode = string(s.Status)
		}
		results = append(results, verifyResult{
			Alias:     s.Alias,
			BaseURL:   s.BaseURL,
			Status:    string(s.Status),
			OK:        ok,
			ErrorCode: errCode,
		})
	}

	return map[string]any{
		"results": results,
	}, nil
}

// spaceConnectURL は OAuth 認可 URL を構築して返す。
// ブラウザでこの URL を開くことでスペースの OAuth 登録が完了する。
func spaceConnectURL(ctx context.Context, args map[string]any, authBaseURL string) (any, error) {
	if authBaseURL == "" {
		return nil, fmt.Errorf("OAuth authorization endpoint not configured for this server")
	}

	rawBaseURL, ok := stringArg(args, "base_url")
	if !ok || rawBaseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}

	alias, _ := stringArg(args, "alias")

	q := url.Values{}
	q.Set("base_url", rawBaseURL)
	if alias != "" {
		q.Set("alias", alias)
	}

	authURL := authBaseURL + "/oauth/backlog/authorize?" + q.Encode()

	return map[string]any{
		"authorization_url": authURL,
		"message":           "Open the authorization_url in your browser to connect the Backlog space",
	}, nil
}

// spaceDisconnect は指定 alias のスペースを削除する。
func spaceDisconnect(ctx context.Context, store space.Store, args map[string]any) (any, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("userID not found in context: authentication required")
	}

	alias, ok := stringArg(args, "alias")
	if !ok || alias == "" {
		return nil, fmt.Errorf("alias is required")
	}

	if err := store.Delete(ctx, userID, alias); err != nil {
		return nil, fmt.Errorf("delete space: %w", err)
	}

	return map[string]any{
		"ok":    true,
		"alias": alias,
	}, nil
}
