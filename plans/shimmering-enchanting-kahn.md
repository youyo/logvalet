# Plan: default_profile バグ修正 + config init に API キー統合

## Context

2つの問題を修正する:
1. `config.toml` の `default_profile` が auth コマンドで無視される → `--profile is required` エラー
2. `config init` と `auth login` が別ステップで UX が悪い → `config init` に API キー入力を統合し、1コマンドでセットアップ完了

## Unit 1: default_profile バグ修正

### 原因
auth コマンド（login/logout/whoami）で `g.Profile == ""` をハードコードチェック。
`config.Resolve()` (`internal/config/config.go:165`) は `default_profile` を正しく解決するが、auth コマンドはその結果を使わない。

### 修正

**`internal/cli/auth.go`**:

1. 共通ヘルパー `resolveProfile(g *GlobalFlags) (string, error)` を追加:
```go
func resolveProfile(g *GlobalFlags) (string, error) {
    if g.Profile != "" {
        return g.Profile, nil
    }
    // config.toml の default_profile を参照
    configPath := config.ResolveConfigPath(g.Config, os.Getenv)
    cfg, err := config.Load(configPath)
    if err != nil {
        return "", fmt.Errorf("設定ファイルの読み込みに失敗: %w", err)
    }
    flags := config.OverrideFlags{Profile: g.Profile}
    resolved, err := config.Resolve(cfg, flags, os.Getenv)
    if err != nil {
        return "", fmt.Errorf("設定の解決に失敗: %w", err)
    }
    if resolved.Profile == "" {
        return "", fmt.Errorf("--profile が必要です。config.toml の default_profile を設定するか --profile を指定してください")
    }
    return resolved.Profile, nil
}
```

2. 3箇所の `if g.Profile == ""` チェックを `resolveProfile` に置換:
   - `RunWithLoginRequestCapture` (L122-124): `profile, err := resolveProfile(g)` → 以降 `profile` を使用（`g.Profile` の代わり）
   - `RunWithStore` (L194-196): 同上
   - `RunWithStoreCapture` (L261-263): 同上、`authRef` 引数が空の場合も `profile` を使用

3. `resolveAuthBaseURL` (L84) も `g.Profile` → 解決済み profile を使うべきだが、既に `config.Resolve` を内部で呼んでいるので変更不要（空 profile でも config.Resolve が default_profile を解決する）

**`internal/cli/auth_test.go`**:

4. 既存テスト修正:
   - `TestAuthWhoamiCmd_Run_NoProfile` (L216): `g.Config` に temp config パスを指定し、default_profile なしの config で「--profile 必要」エラーを期待
   - `TestAuthLoginCmd_NoProfile` (L300): 同上
   - `TestAuthLogoutCmd_Run_NoProfile` (L70): 同上

5. 新規テスト追加:
   - `TestAuthWhoamiCmd_Run_DefaultProfile`: temp config.toml に `default_profile = "work"` + temp tokens.json にエントリ → `g.Profile=""` で成功
   - `TestAuthLoginCmd_DefaultProfile`: 同様パターン
   - `TestAuthLogoutCmd_DefaultProfile`: 同様パターン

### テストの config.toml 作成パターン
```go
// テストヘルパー
dir := t.TempDir()
configPath := filepath.Join(dir, "config.toml")
w := config.NewWriter()
w.Write(configPath, &config.Config{
    Version: 1,
    DefaultProfile: "work",
    Profiles: map[string]config.ProfileConfig{
        "work": {Space: "example", BaseURL: "https://example.backlog.com", AuthRef: "work"},
    },
})
g := &cli.GlobalFlags{Config: configPath}
```

## Unit 2: config init に API キー入力を統合

### 修正

**`internal/cli/config_cmd.go`**:

1. `ConfigInitCmd` にフラグ追加:
   - `InitAPIKey string` (`--init-api-key` / help: "API キーを設定する")

2. `ConfigInitDeps` にフィールド追加:
   - `CredStore credentials.Store` — tokens.json ストア

3. `ConfigureCmd` にも `InitAPIKey` フィールド追加（委譲先に渡す）

4. `Run` メソッド修正: `deps.CredStore` に `credentials.NewStore(credentials.DefaultTokensPath(os.Getenv))` を設定

5. `RunWithDeps` の対話フロー修正（base_url 入力の後に追加）:
```go
// API Key 入力（対話モード）
apiKey := deps の InitAPIKey 相当
if interactive && apiKey == "" {
    apiKey, err = deps.Prompter.Prompt("API Key (空欄でスキップ)", "")
    // エラーハンドリング
}

// API Key が入力された場合、tokens.json に保存
if apiKey != "" && deps.CredStore != nil {
    tokens, err := deps.CredStore.Load()
    // err ハンドリング
    if tokens.Auth == nil {
        tokens.Auth = make(map[string]credentials.AuthEntry)
    }
    if tokens.Version == 0 {
        tokens.Version = 1
    }
    tokens.Auth[profileName] = credentials.AuthEntry{
        AuthType: credentials.AuthTypeAPIKey,
        APIKey:   apiKey,
    }
    deps.CredStore.Save(tokens)
}
```
※ tokens.json 保存ロジックは `auth login` の `RunWithLoginRequestCapture` (auth.go:126-150) と同じパターンを再利用

6. JSON レスポンスに `AuthSaved bool` フィールド追加:
```go
type configInitResponse struct {
    // 既存フィールド...
    AuthSaved bool `json:"auth_saved"`
}
```

7. stderr 案内を条件分岐:
   - API キー保存済み → `"セットアップ完了！ logvalet project list で動作確認できます"`
   - API キー未入力 → `"次のステップ: logvalet auth login --profile {name}"` （現行維持）

**`internal/cli/config_cmd_test.go`**:

8. 新規テスト追加:
   - `TestConfigInit_WithAPIKey`: 非対話モードで `--init-api-key` 指定 → config.toml + tokens.json に保存
   - `TestConfigInit_WithAPIKey_Interactive`: 対話モードで API キー入力 → 両方に保存
   - `TestConfigInit_WithoutAPIKey`: API キー空 → tokens.json に触らない（`CredStore` は nil でも OK）
   - `TestConfigInit_StderrGuidance` (既存): API キー未入力時は `auth login` 案内が出ることを確認（既存テスト維持）
   - `TestConfigInit_StderrComplete`: API キー入力時は完了メッセージが出ることを確認

## 修正ファイルまとめ

| ファイル | 変更内容 |
|---------|---------|
| `internal/cli/auth.go` | `resolveProfile` ヘルパー追加、3箇所のチェック修正 |
| `internal/cli/auth_test.go` | default_profile テスト修正・追加（temp config.toml 使用） |
| `internal/cli/config_cmd.go` | API キー入力ステップ追加、ConfigInitDeps に CredStore、レスポンスに auth_saved |
| `internal/cli/config_cmd_test.go` | API キー統合テスト追加 |

## 再利用する既存コード

| 関数/型 | ファイル | 用途 |
|--------|---------|------|
| `config.ResolveConfigPath(g.Config, os.Getenv)` | `internal/config/config.go` | config パス解決 |
| `config.Load(path)` | `internal/config/config.go` | config.toml ロード |
| `config.Resolve(cfg, flags, getenv)` | `internal/config/config.go:161` | default_profile 解決 |
| `config.NewWriter()` / `config.NewDefaultLoader()` | `internal/config/writer.go` | テスト用 config 作成 |
| `credentials.NewStore(path)` | `internal/credentials/credentials.go:70` | tokens.json ストア |
| `credentials.AuthTypeAPIKey` | `internal/credentials/credentials.go:21` | 定数 |
| `credentials.TokensFile` / `credentials.AuthEntry` | `internal/credentials/credentials.go:25-37` | tokens.json 構造体 |

## 検証方法

```bash
# ユニットテスト
go test ./internal/cli/ -v -run "TestAuth|TestConfigInit"
go test ./...
go vet ./...

# 手動確認（default_profile）
./logvalet auth whoami | jq .          # --profile なしで成功
./logvalet auth whoami --profile heptagon | jq .  # 明示指定でも成功

# 手動確認（config init + API キー統合）
# 新しい一時プロファイルで試す:
./logvalet config init --init-profile test --init-space test-space --init-api-key YOUR_KEY
cat ~/.config/logvalet/tokens.json | jq .  # test エントリが追加されている
./logvalet auth whoami --profile test | jq .  # 認証情報が見える
```
