# MS04: Space Resolver (5段階 fallback)

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS02, MS03

## 目的

`--spaces` / `--all-spaces` スコープから `[]SpaceRegistration` への解決ロジックを実装する。
5段階 fallback により後方互換を保ちながら multi-space 操作を実現する。

## 完了条件

- [ ] `internal/space/resolver.go` — Resolver, WithLegacyProfileFallback オプション（RM2 対応）
- [ ] `internal/space/resolver_test.go` — 全テストケース pass
- [ ] `go test ./internal/space/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### resolver_test.go

#### AllSpaces 解決

```
T1: TestResolver_AllSpaces_ReturnsOnlyEnabled
    - Store に {u1/foo, enabled}, {u1/bar, enabled}, {u1/baz, disabled} を登録
    - Resolve("u1", Scope{AllSpaces: true}) → [{foo}, {bar}]（disabled の baz は除外）
    - 結果の len == 2

T2: TestResolver_AllSpaces_Empty
    - Store に enabled space が0件
    - Resolve("u1", Scope{AllSpaces: true}) → ErrNoSpacesRegistered
```

#### Aliases 解決

```
T3: TestResolver_Aliases_SingleAlias
    - Store に {u1/foo}
    - Resolve("u1", Scope{Aliases: ["foo"]}) → [{foo}]

T4: TestResolver_Aliases_MultipleAliases_OrderPreserved
    - Store に {u1/foo}, {u1/bar}
    - Resolve("u1", Scope{Aliases: ["bar", "foo"]}) → [{bar}, {foo}]（入力順を保持）

T5: TestResolver_Aliases_UnknownAlias
    - Resolve("u1", Scope{Aliases: ["unknown"]}) → ErrSpaceNotFound

T6: TestResolver_Aliases_PartiallyUnknown
    - Store に {u1/foo}
    - Resolve("u1", Scope{Aliases: ["foo", "missing"]}) → ErrSpaceNotFound（1件でも未登録ならエラー）
```

#### 排他チェック

```
T7: TestResolver_BothSpacesAndAllSpaces_Error
    - Resolve("u1", Scope{Aliases: ["foo"], AllSpaces: true}) → ErrInvalidSpaceScope
```

#### Default space 解決（fallback 3-4）

```
T8: TestResolver_DefaultSpace_FromPreference
    - PutPreference({u1, DefaultSpaceAlias: "foo"})
    - Upsert({u1/foo}), Upsert({u1/bar})
    - Resolve("u1", Scope{}) → [{foo}]（preference の default を使う）

T9: TestResolver_DefaultSpace_FallbackToSingleSpace
    - Preference なし、Store に {u1/foo} のみ
    - Resolve("u1", Scope{}) → [{foo}]（1件だけなのでそれを使う）

T10: TestResolver_DefaultSpace_MultipleSpacesNoDefault
    - Preference なし、Store に {u1/foo}, {u1/bar}
    - Resolve("u1", Scope{}) → ErrNoDefaultSpace
```

#### Legacy profile fallback（fallback 5）

```
T11: TestResolver_LegacyProfileFallback
    - Store は空
    - WithLegacyProfileFallback オプションに有効な legacy config を渡す
    - Resolve("u1", Scope{}) → legacy config から生成した SpaceRegistration 1件

T12: TestResolver_LegacyProfileFallback_NoConfig
    - Store は空、WithLegacyProfileFallback なし
    - Resolve("u1", Scope{}) → ErrNoDefaultSpace

T13: TestResolver_LegacyProfileFallback_StoreHasSpaces_FallbackSkipped
    - Store に {u1/foo} がある
    - WithLegacyProfileFallback も設定済み
    - Resolve("u1", Scope{}) → Store の {foo} を使う（fallback は使わない）
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/resolver.go` | Resolver, ResolverOption, WithLegacyProfileFallback |
| `internal/space/resolver_test.go` | T1-T13 |

---

## 3. 実装

### resolver.go

```go
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

type Resolver struct {
    store     Store
    legacyCfg *config.ResolvedConfig // nil なら fallback 5 は無効
}

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
    // 排他チェック
    if scope.AllSpaces && len(scope.Aliases) > 0 {
        return nil, ErrInvalidSpaceScope
    }

    // fallback 1: AllSpaces
    if scope.AllSpaces { ... }

    // fallback 2: Aliases
    if len(scope.Aliases) > 0 { ... }

    // fallback 3-4: preference / single space
    spaces, err := r.store.List(ctx, userID)
    // ...

    // fallback 5: legacy profile
    if r.legacyCfg != nil { ... }

    return nil, ErrNoDefaultSpace
}

// resolveFromLegacyProfile は legacy config から SpaceRegistration を生成する（BC 対応）。
func resolveFromLegacyProfile(cfg *config.ResolvedConfig) (*SpaceRegistration, error) {
    baseURL := cfg.BaseURL
    if baseURL == "" && cfg.Space != "" {
        baseURL = fmt.Sprintf("https://%s.backlog.com", cfg.Space)
    }
    if baseURL == "" {
        return nil, ErrNoDefaultSpace
    }
    tenant := cfg.Space
    return &SpaceRegistration{
        UserID:      "local",
        Alias:       tenant,
        Tenant:      tenant,
        BaseURL:     baseURL,
        AuthType:    AuthTypeAPIKey,
        AuthProfile: cfg.AuthRef,
        Status:      SpaceStatusUnknown,
    }, nil
}
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/resolver_test.go` を作成（T1-T13）
2. `go test ./internal/space/...` → コンパイルエラー（Resolver 未定義）

### Step 2: Green

1. `internal/space/resolver.go` を実装
2. `go test ./internal/space/...` → 全テストパス

### Step 3: Refactor

- `resolveFromLegacyProfile` 関数をヘルパーとして抽出
- remote MCP では nil を渡す旨を doc comment に明記

---

## 5. 実装の要点

### disabled space の除外（RH3）

```go
// fallback 1: AllSpaces
if scope.AllSpaces {
    all, err := r.store.List(ctx, userID)
    // disabled を除外
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
```

### WithLegacyProfileFallback の remote MCP 分離（RM2）

```go
// CLI の buildRunContext 拡張（将来）:
resolver := space.NewResolver(store,
    space.WithLegacyProfileFallback(resolvedCfg), // CLI のみ
)

// remote MCP:
resolver := space.NewResolver(store) // fallback 5 無効
```

---

## 6. 検証コマンド

```bash
go test ./internal/space/... -v -run TestResolver
go test -race ./internal/space/...
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS04 完了後 → MS11（CLI --spaces/--all-spaces フラグ）が着手可能。
