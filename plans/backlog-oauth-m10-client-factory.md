# M10: Per-User ClientFactory

## 概要

リクエスト context からユーザーを特定し、そのユーザーの Backlog OAuth トークンを使って
backlog.Client を生成する ClientFactory を実装する。

## 対象ファイル

| ファイル | 役割 |
|---------|------|
| `internal/auth/context.go` | UserIDFromContext / ContextWithUserID |
| `internal/auth/context_test.go` | context ヘルパーのテスト |
| `internal/auth/factory.go` | ClientFactory 型, NewClientFactory |
| `internal/auth/factory_test.go` | factory のテスト（httptest で Bearer 検証） |

## 依存関係

- M01: TokenRecord, ErrUnauthenticated, ErrProviderNotConnected (完了)
- M06: TokenManager.GetValidToken (完了)
- backlog.NewHTTPClient, ClientConfig, credentials.ResolvedCredential, credentials.AuthTypeOAuth

## TDD テストケース

### context_test.go

1. `ContextWithUserID` → `UserIDFromContext` で正しいユーザーID取得
2. 空 context → `UserIDFromContext` が `"", false`
3. 空文字列のユーザーID → `UserIDFromContext` が `"", false`

### factory_test.go

1. userID 未設定 context → `ErrUnauthenticated`
2. TokenManager が `ErrProviderNotConnected` → そのまま伝搬
3. Happy path: httptest サーバーで `Authorization: Bearer <token>` ヘッダ検証
4. ユーザー隔離: 異なるユーザーで異なるトークンが使用されること

## 実装

### context.go

```go
type contextKey struct{}

func ContextWithUserID(ctx context.Context, userID string) context.Context
func UserIDFromContext(ctx context.Context) (string, bool)
```

### factory.go

```go
type ClientFactory func(ctx context.Context) (backlog.Client, error)

func NewClientFactory(tm TokenManager, provider, tenant, baseURL string) ClientFactory
```

フロー:
1. ctx → UserIDFromContext → userID 取得（なければ ErrUnauthenticated）
2. tm.GetValidToken(ctx, userID, provider, tenant) → TokenRecord
3. backlog.NewHTTPClient(ClientConfig{BaseURL, Credential{OAuth, AccessToken}})
4. Client を返却

## ステータス

- [x] context_test.go Red
- [x] context.go Green
- [x] factory_test.go Red
- [x] factory.go Green
- [x] go test ./internal/auth/... PASS
- [x] go vet ./internal/auth/... PASS
