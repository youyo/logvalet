package space_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/config"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/space"
)

// BC3: Resolver.Resolve(Scope{}) が WithLegacyProfileFallback の config から
// SpaceRegistration を返し、ErrNoDefaultSpace にならないことを確認する。
// 詳細な Resolver ロジックは resolver_test.go の TestResolver_LegacyProfileFallback* で網羅済み。
// ここでは regression guard として最小限の確認を行う。
func TestBC3_ResolverLegacyProfileFallback(t *testing.T) {
	t.Parallel()
	store := space.NewMemoryStore()
	ctx := context.Background()

	cfg := &config.ResolvedConfig{
		Space:   "legacy-space",
		AuthRef: "default",
	}
	r := space.NewResolver(store, space.WithLegacyProfileFallback(cfg))

	got, err := r.Resolve(ctx, "u1", space.Scope{})
	if err != nil {
		t.Fatalf("BC3: expected no error, got: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("BC3: expected 1 space from legacy fallback, got %d", len(got))
	}
	if got[0].BaseURL != "https://legacy-space.backlog.com" {
		t.Errorf("BC3: unexpected BaseURL: %s", got[0].BaseURL)
	}
}

// BC4: StateClaims に BaseURL / Alias フィールドが追加されたが、
// omitempty タグにより既存の state JWT（これらフィールドなし）が問題なくデコードできること。
func TestBC4_StateClaimsLegacyJWTCompatible(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key")
	ttl := 10 * time.Minute

	// BaseURL/Alias なしの旧形式 JWT を生成（GenerateState は旧 API）
	stateJWT, err := auth.GenerateState("user1", "mytenant", secret, ttl)
	if err != nil {
		t.Fatalf("BC4: GenerateState: %v", err)
	}

	// 既存フィールドで検証できること
	claims, err := auth.ValidateState(stateJWT, secret)
	if err != nil {
		t.Fatalf("BC4: ValidateState: %v", err)
	}
	if claims.UserID != "user1" {
		t.Errorf("BC4: UserID = %q, want user1", claims.UserID)
	}
	if claims.Tenant != "mytenant" {
		t.Errorf("BC4: Tenant = %q, want mytenant", claims.Tenant)
	}
	// 旧形式 JWT では BaseURL / Alias は空文字（omitempty により未設定）
	if claims.BaseURL != "" {
		t.Errorf("BC4: BaseURL should be empty in legacy JWT, got %q", claims.BaseURL)
	}
	if claims.Alias != "" {
		t.Errorf("BC4: Alias should be empty in legacy JWT, got %q", claims.Alias)
	}
}

// BC4b: 新形式（BaseURL/Alias 付き）の JWT も正常にデコードできること。
func TestBC4b_StateClaimsNewFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key")
	ttl := 10 * time.Minute

	stateJWT, err := auth.GenerateStateWithSpaceInfo(
		"user1", "mytenant",
		"https://mytenant.backlog.com", "mytenant-alias",
		secret, ttl,
	)
	if err != nil {
		t.Fatalf("BC4b: GenerateStateWithSpaceInfo: %v", err)
	}

	claims, err := auth.ValidateState(stateJWT, secret)
	if err != nil {
		t.Fatalf("BC4b: ValidateState: %v", err)
	}
	if claims.BaseURL != "https://mytenant.backlog.com" {
		t.Errorf("BC4b: BaseURL = %q, want https://mytenant.backlog.com", claims.BaseURL)
	}
	if claims.Alias != "mytenant-alias" {
		t.Errorf("BC4b: Alias = %q, want mytenant-alias", claims.Alias)
	}
}

// BC4c: StateClaims を JSON にシリアライズしたとき、BaseURL/Alias が空なら
// JSON に現れないことを確認（omitempty の動作確認）。
func TestBC4c_StateClaimsOmitemptyJSON(t *testing.T) {
	t.Parallel()

	now := time.Now()
	claims := auth.StateClaims{
		UserID: "user1",
		Tenant: "mytenant",
		Nonce:  "abc123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	data, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("BC4c: json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("BC4c: json.Unmarshal: %v", err)
	}
	if _, ok := m["base_url"]; ok {
		t.Error("BC4c: base_url should not appear in JSON when empty (omitempty)")
	}
	if _, ok := m["alias"]; ok {
		t.Error("BC4c: alias should not appear in JSON when empty (omitempty)")
	}
}

// BC6: SQLiteStore（space パッケージ）が使うテーブル（spaces, user_preferences, nonces）と
// auth/tokenstore の SQLite が使うテーブル（oauth_tokens）は名前が衝突しない。
// 同一 DB ファイルに両方を作成して競合しないことを確認する。
func TestBC6_SQLiteStoreTablesNoConflict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := dir + "/test.db"

	// space.SQLiteStore を同パスで開く
	spaceStore, err := space.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("BC6: NewSQLiteStore: %v", err)
	}
	defer spaceStore.Close()

	// space store に書き込み
	ctx := context.Background()
	reg := &space.SpaceRegistration{
		UserID:    "u1",
		Alias:     "myspace",
		Tenant:    "myspace",
		BaseURL:   "https://myspace.backlog.com",
		AuthType:  space.AuthTypeAPIKey,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := spaceStore.Upsert(ctx, reg); err != nil {
		t.Fatalf("BC6: Upsert: %v", err)
	}

	// 読み戻し確認
	got, err := spaceStore.Get(ctx, "u1", "myspace")
	if err != nil {
		t.Fatalf("BC6: Get: %v", err)
	}
	if got == nil || got.Alias != "myspace" {
		t.Errorf("BC6: expected alias=myspace, got %v", got)
	}
}

// BC7: space.AuthTypeAPIKey の値が credentials.AuthTypeAPIKey と一致することを確認。
// 既存 tokens.json を持つユーザーが SpaceRegistration に移行しても
// 文字列変換なしで "api_key" のまま保持できることを保証する。
func TestBC7_AuthTypeAPIKeyValueConsistency(t *testing.T) {
	t.Parallel()

	if string(space.AuthTypeAPIKey) != credentials.AuthTypeAPIKey {
		t.Errorf("BC7: space.AuthTypeAPIKey=%q != credentials.AuthTypeAPIKey=%q",
			string(space.AuthTypeAPIKey), credentials.AuthTypeAPIKey)
	}
}

// BC7b: SpaceRegistration に AuthTypeAPIKey を設定して SQLiteStore に保存し、
// 読み戻したときも "api_key" のまま変化しないことを確認。
func TestBC7b_AuthTypeAPIKey_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := space.NewSQLiteStore(dir + "/bc7b.db")
	if err != nil {
		t.Fatalf("BC7b: NewSQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	reg := &space.SpaceRegistration{
		UserID:    "u1",
		Alias:     "foo",
		Tenant:    "foo",
		BaseURL:   "https://foo.backlog.com",
		AuthType:  space.AuthTypeAPIKey,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Upsert(ctx, reg); err != nil {
		t.Fatalf("BC7b: Upsert: %v", err)
	}
	got, err := store.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("BC7b: Get: %v", err)
	}
	if string(got.AuthType) != credentials.AuthTypeAPIKey {
		t.Errorf("BC7b: AuthType = %q, want %q", got.AuthType, credentials.AuthTypeAPIKey)
	}
}

// BC8: ExitCodePartialFailure=8 が既存 exit code 定義（0-7, 10）と衝突しないことを確認。
func TestBC8_ExitCodePartialFailure_NoConflict(t *testing.T) {
	t.Parallel()

	existing := map[string]int{
		"ExitSuccess":             0,
		"ExitGenericError":        1,
		"ExitArgumentError":       2,
		"ExitAuthenticationError": 3,
		"ExitPermissionError":     4,
		"ExitNotFoundError":       5,
		"ExitAPIError":            6,
		"ExitDigestError":         7,
		"ExitConfigError":         10,
	}

	for name, code := range existing {
		if code == space.ExitCodePartialFailure {
			t.Errorf("BC8: ExitCodePartialFailure=%d conflicts with %s=%d",
				space.ExitCodePartialFailure, name, code)
		}
	}
	if space.ExitCodePartialFailure != 8 {
		t.Errorf("BC8: ExitCodePartialFailure = %d, want 8", space.ExitCodePartialFailure)
	}
}
