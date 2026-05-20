package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/youyo/logvalet/internal/space"
)

const cliUserID = "local"

// SpacesCmd は lv spaces 管理コマンド群のルート。
// 既存 SpaceCmd（lv space info/disk-usage/digest）とは別物。
type SpacesCmd struct {
	List    SpacesListCmd    `cmd:"" help:"list registered spaces"`
	Add     SpacesAddCmd     `cmd:"" help:"register a space with API key"`
	Connect SpacesConnectCmd `cmd:"" help:"register a space via OAuth (returns authorization URL)"`
	Remove  SpacesRemoveCmd  `cmd:"" name:"remove" help:"remove a registered space"`
	Use     SpacesUseCmd     `cmd:"" name:"use" help:"set default space"`
	Verify  SpacesVerifyCmd  `cmd:"" help:"verify space connections"`
}

// VerifyResult は verify コマンドの1スペース結果。
type VerifyResult struct {
	Alias     string `json:"alias"`
	OK        bool   `json:"ok"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// buildSpaceStore は環境変数から SpaceStore を構築する。
// LOGVALET_SPACE_STORE_TYPE: sqlite（デフォルト）または dynamodb
// LOGVALET_SPACE_STORE_PATH: SQLite DBパス（デフォルト: ~/.logvalet/spaces.db）
// DynamoDB モードでは LOGVALET_SPACE_STORE_DYNAMODB_TABLE/REGION が未設定の場合、
// LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE/REGION にフォールバックする（既存テーブルへの相乗り）。
func buildSpaceStore() (space.Store, error) {
	storeType := os.Getenv("LOGVALET_SPACE_STORE_TYPE")
	if storeType == "" {
		storeType = "sqlite"
	}
	switch space.StoreType(storeType) {
	case space.StoreTypeSQLite:
		dbPath := os.Getenv("LOGVALET_SPACE_STORE_PATH")
		if dbPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("spaces: get home dir: %w", err)
			}
			dbPath = homeDir + "/.logvalet/spaces.db"
		}
		if err := os.MkdirAll(getDir(dbPath), 0o700); err != nil {
			return nil, fmt.Errorf("spaces: mkdir: %w", err)
		}
		return space.NewSQLiteStore(dbPath)
	case space.StoreTypeDynamoDB:
		table := os.Getenv("LOGVALET_SPACE_STORE_DYNAMODB_TABLE")
		if table == "" {
			table = os.Getenv("LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE")
		}
		region := os.Getenv("LOGVALET_SPACE_STORE_DYNAMODB_REGION")
		if region == "" {
			region = os.Getenv("LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION")
		}
		return space.NewDynamoDBStore(table, region)
	default:
		return nil, fmt.Errorf("spaces: unknown store type %q", storeType)
	}
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// ---------- SpacesListCmd ----------

// SpacesListCmd は lv spaces list コマンド。
type SpacesListCmd struct{}

func (c *SpacesListCmd) Run(g *GlobalFlags) error {
	s, err := buildSpaceStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return c.RunWithStore(os.Stdout, s, cliUserID)
}

// RunWithStore はテスト可能なエントリポイント。
func (c *SpacesListCmd) RunWithStore(w io.Writer, s space.Store, userID string) error {
	ctx := context.Background()
	regs, err := s.List(ctx, userID)
	if err != nil {
		return err
	}
	pref, _ := s.GetPreference(ctx, userID)
	defaultAlias := ""
	if pref != nil {
		defaultAlias = pref.DefaultSpaceAlias
	}

	type spaceItem struct {
		Alias    string `json:"alias"`
		Tenant   string `json:"tenant"`
		BaseURL  string `json:"base_url"`
		AuthType string `json:"auth_type"`
		Status   string `json:"status"`
		Default  bool   `json:"default"`
	}

	items := make([]spaceItem, 0, len(regs))
	for _, r := range regs {
		items = append(items, spaceItem{
			Alias:    r.Alias,
			Tenant:   r.Tenant,
			BaseURL:  r.BaseURL,
			AuthType: string(r.AuthType),
			Status:   string(r.Status),
			Default:  r.Alias == defaultAlias,
		})
	}
	// alias でソートして安定出力
	sort.Slice(items, func(i, j int) bool { return items[i].Alias < items[j].Alias })

	return json.NewEncoder(w).Encode(map[string]interface{}{"spaces": items})
}

// ---------- SpacesAddCmd ----------

// SpacesAddCmd は lv spaces add コマンド（API key 登録）。
type SpacesAddCmd struct {
	Alias      string `arg:"" help:"alias name for the space"`
	SpaceURL   string `name:"space-url" required:"" help:"Backlog space base URL (e.g. https://foo.backlog.com)"`
	APIProfile string `name:"api-key-profile" default:"" help:"credentials profile name for API key auth"`
}

func (c *SpacesAddCmd) Run(g *GlobalFlags) error {
	s, err := buildSpaceStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return c.RunWithStore(os.Stdout, s, cliUserID)
}

// RunWithStore はテスト可能なエントリポイント。
func (c *SpacesAddCmd) RunWithStore(w io.Writer, s space.Store, userID string) error {
	ctx := context.Background()

	normalizedURL, err := space.NormalizeBaseURL(c.SpaceURL)
	if err != nil {
		return fmt.Errorf("spaces add: invalid base URL %q: %w", c.SpaceURL, err)
	}

	alias := c.Alias
	if alias == "" {
		derived, deriveErr := space.DeriveAliasFromBaseURL(normalizedURL)
		if deriveErr != nil || derived == "" {
			return fmt.Errorf("spaces add: --alias is required when base_url has no backlog.com subdomain")
		}
		alias = derived
	}
	if err := space.ValidateAlias(alias); err != nil {
		return fmt.Errorf("spaces add: invalid alias %q: %w", alias, err)
	}

	tenant, _ := space.DeriveInitialTenant(normalizedURL)
	if tenant == "" {
		tenant = alias
	}

	reg := &space.SpaceRegistration{
		UserID:      userID,
		Alias:       alias,
		Tenant:      tenant,
		BaseURL:     normalizedURL,
		AuthType:    space.AuthTypeAPIKey,
		AuthProfile: c.APIProfile,
		Provider:    "backlog",
		Status:      space.SpaceStatusUnknown,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.Upsert(ctx, reg); err != nil {
		return fmt.Errorf("spaces add: upsert: %w", err)
	}

	// default が未設定なら自動設定
	updateDefaultIfEmpty(ctx, s, userID, alias)

	return json.NewEncoder(w).Encode(map[string]interface{}{
		"result": "ok",
		"alias":  alias,
		"status": string(space.SpaceStatusUnknown),
	})
}

// ---------- SpacesConnectCmd ----------

// SpacesConnectCmd は lv spaces connect コマンド（OAuth URL 出力）。
type SpacesConnectCmd struct {
	Alias    string `name:"alias" default:"" help:"alias name for the space"`
	SpaceURL string `name:"space-url" required:"" help:"Backlog space base URL (e.g. https://foo.backlog.com)"`
}

func (c *SpacesConnectCmd) Run(g *GlobalFlags) error {
	return c.RunWithWriter(os.Stdout)
}

// RunWithWriter はテスト可能なエントリポイント。
// CLI モードでは MCP サーバーの外部 URL が不明なため、ユーザーに手順を案内する。
func (c *SpacesConnectCmd) RunWithWriter(w io.Writer) error {
	normalizedURL, err := space.NormalizeBaseURL(c.SpaceURL)
	if err != nil {
		return fmt.Errorf("spaces connect: invalid base URL %q: %w", c.SpaceURL, err)
	}

	alias := c.Alias
	if alias == "" {
		derived, _ := space.DeriveAliasFromBaseURL(normalizedURL)
		alias = derived
	}

	return json.NewEncoder(w).Encode(map[string]interface{}{
		"action":   "visit_oauth_authorize",
		"alias":    alias,
		"base_url": normalizedURL,
		"note":     "Start the MCP server and visit /oauth/backlog/authorize?base_url=" + normalizedURL + "&alias=" + alias,
	})
}

// ---------- SpacesRemoveCmd ----------

// SpacesRemoveCmd は lv spaces remove コマンド。
type SpacesRemoveCmd struct {
	Alias string `arg:"" help:"alias of the space to remove"`
}

func (c *SpacesRemoveCmd) Run(g *GlobalFlags) error {
	s, err := buildSpaceStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return c.RunWithStore(os.Stdout, s, cliUserID)
}

// RunWithStore はテスト可能なエントリポイント。
func (c *SpacesRemoveCmd) RunWithStore(w io.Writer, s space.Store, userID string) error {
	ctx := context.Background()

	// alias が存在するか確認
	existing, err := s.Get(ctx, userID, c.Alias)
	if err != nil {
		return fmt.Errorf("spaces remove: get: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("spaces remove: %w: %s", space.ErrSpaceNotFound, c.Alias)
	}

	// 削除前に残りスペース一覧を取得
	all, err := s.List(ctx, userID)
	if err != nil {
		return fmt.Errorf("spaces remove: list: %w", err)
	}

	// 削除実行
	if err := s.Delete(ctx, userID, c.Alias); err != nil {
		return fmt.Errorf("spaces remove: delete: %w", err)
	}

	// 残りの enabled スペースを計算
	remaining := make([]space.SpaceRegistration, 0, len(all))
	for _, r := range all {
		if r.Alias != c.Alias && !r.Disabled && r.Status != space.SpaceStatusDisabled {
			remaining = append(remaining, r)
		}
	}

	// default スペースを更新
	newDefault := ""
	if len(remaining) > 0 {
		// CreatedAt 昇順で先頭を選択
		sort.Slice(remaining, func(i, j int) bool {
			return remaining[i].CreatedAt.Before(remaining[j].CreatedAt)
		})
		newDefault = remaining[0].Alias
	}

	// UserPreference を更新
	pref, _ := s.GetPreference(ctx, userID)
	if pref == nil {
		pref = &space.UserPreference{UserID: userID}
	}
	pref.DefaultSpaceAlias = newDefault
	if err := s.PutPreference(ctx, pref); err != nil {
		return fmt.Errorf("spaces remove: update preference: %w", err)
	}

	var msg string
	if len(remaining) > 0 {
		msg = fmt.Sprintf("Removed '%s'. Default space changed to '%s'.", c.Alias, newDefault)
	} else {
		msg = fmt.Sprintf("Removed '%s'. No spaces registered. Run 'lv spaces add' or 'lv spaces connect'.", c.Alias)
	}

	return json.NewEncoder(w).Encode(map[string]interface{}{
		"result":        "ok",
		"removed":       c.Alias,
		"default_space": newDefault,
		"message":       msg,
	})
}

// ---------- SpacesUseCmd ----------

// SpacesUseCmd は lv spaces use コマンド。
type SpacesUseCmd struct {
	Alias string `arg:"" help:"alias to set as default"`
}

func (c *SpacesUseCmd) Run(g *GlobalFlags) error {
	s, err := buildSpaceStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return c.RunWithStore(os.Stdout, s, cliUserID)
}

// RunWithStore はテスト可能なエントリポイント。
func (c *SpacesUseCmd) RunWithStore(w io.Writer, s space.Store, userID string) error {
	ctx := context.Background()

	existing, err := s.Get(ctx, userID, c.Alias)
	if err != nil {
		return fmt.Errorf("spaces use: get: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("spaces use: %w: %s", space.ErrSpaceNotFound, c.Alias)
	}

	pref, _ := s.GetPreference(ctx, userID)
	if pref == nil {
		pref = &space.UserPreference{UserID: userID}
	}
	pref.DefaultSpaceAlias = c.Alias
	if err := s.PutPreference(ctx, pref); err != nil {
		return fmt.Errorf("spaces use: update preference: %w", err)
	}

	return json.NewEncoder(w).Encode(map[string]interface{}{
		"result":        "ok",
		"default_space": c.Alias,
	})
}

// ---------- SpacesVerifyCmd ----------

// SpacesVerifyCmd は lv spaces verify コマンド。
type SpacesVerifyCmd struct {
	Only      string `name:"only" default:"" help:"comma-separated space aliases to verify"`
	VerifyAll bool   `name:"verify-all" help:"verify all registered spaces"`
}

// VerifyFn は1スペースの接続確認を行う関数型。テストで差し替え可能。
type VerifyFn func(ctx context.Context, reg space.SpaceRegistration) VerifyResult

func (c *SpacesVerifyCmd) Run(g *GlobalFlags) error {
	s, err := buildSpaceStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return c.RunWithStore(os.Stdout, s, cliUserID, defaultVerifyFn)
}

// RunWithStore はテスト可能なエントリポイント。
func (c *SpacesVerifyCmd) RunWithStore(w io.Writer, s space.Store, userID string, fn VerifyFn) error {
	ctx := context.Background()

	regs, err := s.List(ctx, userID)
	if err != nil {
		return fmt.Errorf("spaces verify: list: %w", err)
	}

	// alias フィルタ
	if c.Only != "" {
		aliases, parseErr := ParseSpacesFlag(c.Only)
		if parseErr != nil {
			return parseErr
		}
		filtered := make([]space.SpaceRegistration, 0, len(aliases))
		for _, alias := range aliases {
			for _, r := range regs {
				if r.Alias == alias {
					filtered = append(filtered, r)
					break
				}
			}
		}
		regs = filtered
	}

	results := make([]VerifyResult, 0, len(regs))
	for _, reg := range regs {
		result := fn(ctx, reg)
		results = append(results, result)

		// store に status を反映
		updated := reg
		if result.OK {
			updated.Status = space.SpaceStatusOK
		} else {
			switch result.ErrorCode {
			case "token_missing", "not_connected":
				updated.Status = space.SpaceStatusNotConnected
			case "unauthorized":
				updated.Status = space.SpaceStatusUnauthorized
			}
		}
		updated.LastVerifiedAt = time.Now()
		_ = s.Upsert(ctx, &updated)
	}

	type verifyOutput struct {
		Alias     string `json:"alias"`
		OK        bool   `json:"ok"`
		Status    string `json:"status"`
		ErrorCode string `json:"error_code,omitempty"`
		Message   string `json:"message,omitempty"`
	}
	out := make([]verifyOutput, 0, len(results))
	for _, r := range results {
		out = append(out, verifyOutput{
			Alias:     r.Alias,
			OK:        r.OK,
			Status:    r.Status,
			ErrorCode: r.ErrorCode,
			Message:   r.Message,
		})
	}

	return json.NewEncoder(w).Encode(map[string]interface{}{"results": out})
}

// defaultVerifyFn は本番用の verify 実装。
// CLI モードでは APIKey スペースのみ検証可能。OAuth スペースは token_missing を返す。
func defaultVerifyFn(ctx context.Context, reg space.SpaceRegistration) VerifyResult {
	if reg.AuthType == space.AuthTypeAPIKey {
		// API key スペースは token store 不要 → 登録済み＝接続済みとみなす
		return VerifyResult{Alias: reg.Alias, OK: true, Status: "ok"}
	}
	// OAuth スペースは CLI 環境では token 確認不可
	return VerifyResult{
		Alias:     reg.Alias,
		OK:        false,
		Status:    "error",
		ErrorCode: "token_missing",
		Message:   fmt.Sprintf("run 'lv spaces connect --alias %s' to reconnect", reg.Alias),
	}
}

// ---------- ヘルパー ----------

// updateDefaultIfEmpty は DefaultSpaceAlias が空の場合のみ alias を設定する。
func updateDefaultIfEmpty(ctx context.Context, s space.Store, userID, alias string) {
	pref, err := s.GetPreference(ctx, userID)
	if err != nil {
		return
	}
	if pref != nil && pref.DefaultSpaceAlias != "" {
		return
	}
	newPref := &space.UserPreference{UserID: userID, DefaultSpaceAlias: alias}
	if pref != nil {
		newPref.CreatedAt = pref.CreatedAt
	}
	_ = s.PutPreference(ctx, newPref)
}
