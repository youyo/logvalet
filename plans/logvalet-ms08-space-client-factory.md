# MS08: SpaceAwareClientFactory

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS01（RC1 修正: MS04 は不要）

## 目的

`SpaceRegistration → backlog.Client` を生成するファクトリを実装する。
OAuth / APIKey 両認証方式を内部で分岐する。
既存の `auth.ClientFactory` は変更しない（後方互換）。

## 完了条件

- [ ] `internal/auth/space_factory.go` — SpaceAwareClientFactory
- [ ] `internal/auth/space_factory_test.go` — 全テストケース pass
- [ ] `go test ./internal/auth/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### space_factory_test.go

```
T1: TestSpaceAwareClientFactory_OAuth_Success
    - reg.AuthType = "oauth", reg.Tenant = "foo", reg.BaseURL = "https://foo.backlog.com"
    - ctx に userID "u1" を設定
    - TokenStore に {u1, "backlog", "foo"} のトークンを設定
    - factory(ctx, reg) → backlog.Client 生成成功
    - 生成した Client が Authorization: Bearer <token> でリクエストを送る（httptest）

T2: TestSpaceAwareClientFactory_OAuth_NoUserID
    - ctx に userID が設定されていない
    - factory(ctx, reg{AuthType: "oauth"}) → ErrUnauthenticated

T3: TestSpaceAwareClientFactory_OAuth_NoToken
    - TokenStore に token が存在しない
    - factory(ctx, reg{AuthType: "oauth"}) → ErrProviderNotConnected

T4: TestSpaceAwareClientFactory_OAuth_UserIsolation
    - tokenStoreA: {userA, "backlog", "foo"} → token-A
    - tokenStoreB: {userB, "backlog", "foo"} → token-B（別ユーザー・同テナント）
    - ctxA（userA）で factory → token-A を使う
    - ctxB（userB）で factory → token-B を使う
    - 2つの Client が別の Bearer token を使うことを httptest で確認

T5: TestSpaceAwareClientFactory_APIKey_Success
    - reg.AuthType = "api_key", reg.AuthProfile = "foo-profile"
    - tokens.json に foo-profile の api_key を設定
    - factory(ctx, reg) → backlog.Client 生成成功
    - 生成した Client が apiKey クエリパラメータでリクエストを送る（httptest）

T6: TestSpaceAwareClientFactory_APIKey_NoCredential
    - reg.AuthType = "api_key", reg.AuthProfile = "missing-profile"
    - factory(ctx, reg) → error（not_configured 相当）

T7: TestSpaceAwareClientFactory_SameTenant_DifferentAlias_SameToken
    - reg1 = {Alias:"prod-ro", Tenant:"myorg"}, reg2 = {Alias:"prod-rw", Tenant:"myorg"}
    - 両方とも同じ token を使う（同一 tenant → TokenManager.GetValidToken で dedup）
    - H5: singleflight は TokenManager 側で保証されているため factory 側では意識不要
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/auth/space_factory.go` | SpaceAwareClientFactory |
| `internal/auth/space_factory_test.go` | T1-T7 |

---

## 3. 実装

### space_factory.go

```go
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
//   ctx → UserIDFromContext → tm.GetValidToken(userID, "backlog", reg.Tenant)
//   → backlog.NewHTTPClient(Bearer)
//
// APIKey パス:
//   credResolver.Resolve(reg.AuthProfile) → backlog.NewHTTPClient(APIKey)
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
        return "" // env var は space factory では使わない
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
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/auth/space_factory_test.go` を作成（T1-T7）
2. `go test ./internal/auth/...` → コンパイルエラー

### Step 2: Green

1. `internal/auth/space_factory.go` を実装
2. `go test ./internal/auth/...` → 全テストパス

### Step 3: Refactor

- T4（user isolation）テストで httptest の token を明示的に確認
- `buildOAuthClient` / `buildAPIKeyClient` のヘルパー分離は既に実施

---

## 5. 実装の要点

### RC1: MS04（Resolver）に依存しない

SpaceAwareClientFactory は `SpaceRegistration` を引数として受け取る。
どの space を対象にするかの解決は Resolver（MS04）の責務。
factory は「与えられた registration から client を作る」だけ。

### H5: singleflight dedup

```text
同一 tenant を持つ2つの alias（例: prod-ro, prod-rw）から並列に factory を呼ぶ場合:
  → tm.GetValidToken(ctx, userID, "backlog", "myorg") が2回呼ばれる
  → auth.TokenManager の singleflight.Group が key="userID:backlog:myorg" で dedup
  → 1回のリフレッシュで2つの factory 呼び出しが共有される（正しい動作）
factory 側では特別な処理は不要。
```

---

## 6. 検証コマンド

```bash
go test ./internal/auth/... -v -run TestSpaceAwareClientFactory
go test -race ./internal/auth/...
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS08 完了後:
- MS09（ExecuteAcrossSpaces）が着手可能
- MS06 + MS06a + MS08 完了後 → MS10（MultiSpaceOAuthHandler）が着手可能
