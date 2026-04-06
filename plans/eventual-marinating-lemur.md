# プロファイル固有フィールドの優先順位修正

## Context

環境変数 `LOGVALET_BASE_URL` が `.zshrc` で設定されていると、`-p dc` でプロファイルを切り替えても
`base_url` が環境変数の値で上書きされ、常に heptagon.backlog.com にリクエストが飛ぶ。

**原因**: Kong の `env:` タグが `g.BaseURL` にセット → `OverrideFlags.BaseURL` に渡される → CLI フラグと同等の最優先扱いになる。

## 修正方針

プロファイル固有フィールド（`base_url`, `space`）の優先順位を変更する:

```
変更前: CLI flag > env > profile config > default
変更後: CLI flag > profile config > env > default
```

グローバルフィールド（`format`, `pretty`, `verbose` 等）は現状維持。

## 変更ファイル

### 1. `internal/cli/global_flags.go`

`BaseURL` と `Space` から Kong の `env:` タグを削除。
Kong が env 値を CLI フラグと区別できないため、env 処理は `config.Resolve()` に一本化する。

```go
// 変更前
BaseURL string `help:"specify Backlog base URL directly" env:"LOGVALET_BASE_URL"`
Space   string `short:"s" help:"specify Backlog space name directly" env:"LOGVALET_SPACE"`

// 変更後
BaseURL string `help:"specify Backlog base URL directly (env: LOGVALET_BASE_URL)"`
Space   string `short:"s" help:"specify Backlog space name directly (env: LOGVALET_SPACE)"`
```

help テキストに env 変数名を記載し、`--help` での発見可能性を維持。

### 2. `internal/config/config.go`

#### ResolvedConfig に Warnings フィールド追加

```go
type ResolvedConfig struct {
    // ... 既存フィールド
    Warnings []string  // 環境変数がプロファイルで上書きされた場合等
}
```

#### Resolve() の Space/BaseURL 優先順位変更

```go
// 変更前: flags > env > profile
resolved.Space = resolveString(flags.Space, getenv("LOGVALET_SPACE"), profileCfg.Space, "")
resolved.BaseURL = resolveString(flags.BaseURL, getenv("LOGVALET_BASE_URL"), profileCfg.BaseURL, "")

// 変更後: flags > profile > env（+ warning 生成）
resolved.Space = resolveString(flags.Space, profileCfg.Space, getenv("LOGVALET_SPACE"), "")
resolved.BaseURL = resolveString(flags.BaseURL, profileCfg.BaseURL, getenv("LOGVALET_BASE_URL"), "")

// Warning: env が設定されているがプロファイルに上書きされた場合
if envBaseURL != "" && profileCfg.BaseURL != "" && envBaseURL != profileCfg.BaseURL && flags.BaseURL == "" {
    warnings = append(warnings, fmt.Sprintf(
        "LOGVALET_BASE_URL=%q is set but overridden by profile %q (base_url=%q)",
        envBaseURL, profile, profileCfg.BaseURL))
}
// Space も同様
```

### 3. `internal/cli/runner.go`

buildRunContext で warnings を stderr に出力:

```go
resolved, err := config.Resolve(cfg, flags, os.Getenv)
// ...
for _, w := range resolved.Warnings {
    fmt.Fprintf(os.Stderr, "warning: %s\n", w)
}
```

### 4. テスト更新

#### `internal/config/config_test.go`

- `TestResolve_ProfileOverridesEnvForBaseURL` — profile の base_url が env より優先されることを検証
- `TestResolve_ProfileOverridesEnvForSpace` — 同上 Space
- `TestResolve_EnvFallbackWhenProfileEmpty` — profile に base_url がない場合は env が使われることを検証
- `TestResolve_WarningWhenEnvOverriddenByProfile` — warning が生成されることを検証
- `TestResolve_NoWarningWhenEnvMatchesProfile` — env と profile が同じ値なら warning なし
- 既存テスト `TestResolve_EnvBaseURL` / `TestResolve_EnvSpace` の期待値調整

## 変更しないもの

- `Format`（`default:"json"` の問題）— 別のスコープ
- `Profile` の `env:` タグ — プロファイル名自体は env で指定できて正しい
- `APIKey` / `AccessToken` の `env:` タグ — 認証は credentials.Resolve() で処理、問題なし
- `Pretty` / `Verbose` / `NoColor` — グローバル設定、現状で動作している

## 検証手順

```bash
# 1. LOGVALET_BASE_URL が設定された状態で各プロファイルをテスト
export LOGVALET_BASE_URL=https://heptagon.backlog.com/
go run ./cmd/logvalet/ -p dc project list
# → megumilog.backlog.jp のプロジェクトが返る
# → stderr に warning が出る

# 2. --base-url 明示指定が最優先であることを確認
go run ./cmd/logvalet/ -p dc --base-url https://override.example.com project list
# → override.example.com にリクエスト

# 3. profile に base_url がない場合、env にフォールバック
# (config.toml に base_url なしのプロファイルを一時的に作成して確認)

# 4. ユニットテスト
go test ./internal/config/...

# 5. 全テスト
go test ./...
```
