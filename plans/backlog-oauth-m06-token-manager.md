# M06: TokenManager 実装

## 目標

TokenStore（保存）と OAuthProvider（リフレッシュ）を組み合わせて、有効なトークンを返す TokenManager を実装する。

## 設計決定

| # | 決定 | 理由 |
|---|------|------|
| 1 | `TokenRefresher` interface を auth パッケージに定義 | auth → auth/provider の循環依存を回避。Go の構造的部分型で BacklogOAuthProvider は自動的に満たす |
| 2 | リフレッシュ後に identity fields をコピー | M05 ハンドオフ: RefreshToken は UserID/ProviderUserID を設定しない。manager が既存レコードからコピーする |
| 3 | UpdatedAt は manager が設定 | MemoryStore は受け取ったレコードをそのまま保存するため、manager 側で time.Now() を設定する |

## 対象ファイル

- `internal/auth/manager.go` — TokenRefresher interface + TokenManager interface + tokenManager 実装
- `internal/auth/manager_test.go` — TDD テスト

## 型定義

```go
// TokenRefresher はトークンリフレッシュ機能のみを抽象化する。
// auth/provider.OAuthProvider の部分集合で、循環依存を回避する。
type TokenRefresher interface {
    Name() string
    RefreshToken(ctx context.Context, refreshToken string) (*TokenRecord, error)
}

type TokenManager interface {
    GetValidToken(ctx context.Context, userID, provider, tenant string) (*TokenRecord, error)
    SaveToken(ctx context.Context, record *TokenRecord) error
    RevokeToken(ctx context.Context, userID, provider, tenant string) error
}

type Option func(*tokenManager)

func WithRefreshMargin(d time.Duration) Option

type tokenManager struct {
    store         TokenStore
    providers     map[string]TokenRefresher
    refreshMargin time.Duration  // デフォルト: 5min
}

func NewTokenManager(store TokenStore, providers map[string]TokenRefresher, opts ...Option) TokenManager
```

## GetValidToken フロー

1. `store.Get(ctx, userID, provider, tenant)`
2. nil → return `ErrProviderNotConnected`
3. `record.NeedsRefresh(margin)` → false → return record
4. true → `providers[provider].RefreshToken(ctx, record.RefreshToken)`
5. 失敗 → return `fmt.Errorf("...: %w", ErrTokenRefreshFailed)`
6. 成功 → identity fields コピー (UserID, ProviderUserID, Provider, Tenant, CreatedAt) + UpdatedAt = time.Now()
7. `store.Put(ctx, refreshed)`
8. return refreshed

## TDD テストケース

### Red Phase

1. **有効トークン取得**: store にトークンあり + 期限内 → そのまま返す
2. **自動リフレッシュ**: store にトークンあり + NeedsRefresh=true → provider.RefreshToken 呼び出し + store 更新 + 新トークン返却
3. **リフレッシュ後 identity fields 保持**: RefreshToken が UserID 等を設定しない → manager がコピー
4. **レコード未存在**: store.Get が nil → `ErrProviderNotConnected`
5. **リフレッシュ失敗**: provider.RefreshToken がエラー → `ErrTokenRefreshFailed`
6. **プロバイダー未登録**: providers map に該当 provider なし → `ErrProviderNotConnected`
7. **デフォルトマージン 5min**: WithRefreshMargin 未指定時のデフォルト動作
8. **カスタムマージン**: `WithRefreshMargin(10*time.Minute)` で変更可能
9. **SaveToken**: store.Put に委譲
10. **RevokeToken**: store.Delete に委譲

### Observability

- リフレッシュ成功: `slog.Info("token refreshed", "provider", ..., "user_id", ..., "access_token", maskToken(...))`
- リフレッシュ失敗: `slog.Warn("token refresh failed", "provider", ..., "user_id", ..., "error", ...)`
- トークンは maskToken で必ずマスク

## リスク

| リスク | 対策 |
|--------|------|
| 循環依存 auth → auth/provider | TokenRefresher interface で回避 |
| RefreshToken が identity fields を返さない | コピーロジック + テスト |
| トークンがログに漏洩 | maskToken 使用 + テスト |
