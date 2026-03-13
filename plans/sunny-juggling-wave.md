---
title: auth login を API キー認証のみに修正
project: logvalet
author: planning-agent
created: 2026-03-13
status: Ready for Review
---

# auth login バグ修正: OAuth2 を削除して API キー認証に統一

## Context

`lv auth login` が実行時に client_id と client_secret をインタラクティブに要求する実装になっている。
Backlog の OAuth2 は client_id/secret の事前登録が必要で CLi への埋め込みが複雑なため、
当面は API キー認証のみをサポートする方針に変更する。

**現在の誤った動作:**
```
$ lv auth login --profile work
Client ID: (ユーザー入力待ち) ← ここが問題
Client Secret: (ユーザー入力待ち)
```

**修正後の正しい動作:**
```
$ lv auth login --profile work --api-key XXXXX
{"schema_version":"1","result":"ok","profile":"work","auth_type":"api_key","saved":true}

# --api-key 省略時は stdin プロンプト
$ lv auth login --profile work
API Key: (入力)
```

## スコープ

### 変更対象（1ファイルのみ）
- `internal/cli/auth.go` — `AuthLoginCmd` 構造体と `Run()` メソッドのみ

### 変更不要
- `internal/credentials/credentials.go` — `AuthTypeAPIKey`, `AuthEntry.APIKey` 既存対応済み
- `internal/credentials/oauth.go` — 将来の OAuth 復活に備えて残す（CLI から呼ばない）
- `internal/cli/auth_test.go` — `TestAuthLoginCmd_SaveAPIKey` が既にAPIキーをカバー済み

## 実装手順

### Step 1: `AuthLoginCmd` に `--api-key` フラグを追加

**変更前（`:35`）:**
```go
type AuthLoginCmd struct{}
```

**変更後:**
```go
type AuthLoginCmd struct {
    APIKey string `name:"api-key" help:"Backlog API キー" env:"LOGVALET_API_KEY"`
}
```

### Step 2: `Run()` を API キーフローに書き換え

**変更前（`:39-110`）:** OAuth フロー全体（client_id stdin + StartCallbackServer + ExchangeCode）

**変更後:**
```go
func (c *AuthLoginCmd) Run(g *GlobalFlags) error {
    // API キーを解決（フラグ > stdin プロンプト）
    apiKey := c.APIKey
    if apiKey == "" {
        fmt.Fprint(os.Stderr, "API Key: ")
        if _, err := fmt.Fscanln(os.Stdin, &apiKey); err != nil {
            return fmt.Errorf("API Key の読み取りに失敗しました: %w", err)
        }
    }
    if apiKey == "" {
        return fmt.Errorf("API Key が指定されていません")
    }

    // BaseURL / Space を設定から解決
    baseURL, space, err := resolveAuthBaseURL(g)
    if err != nil {
        return err
    }

    req := AuthLoginRequest{
        AuthType: credentials.AuthTypeAPIKey,
        APIKey:   apiKey,
        Space:    space,
        BaseURL:  baseURL,
    }

    store := credentials.NewStore(credentials.DefaultTokensPath(os.Getenv))
    output, err := c.RunWithLoginRequestCapture(g, store, req)
    if err != nil {
        return err
    }
    fmt.Println(output)
    return nil
}
```

### Step 3: 不要な import を削除

`Run()` から以下が不要になる:
- `"context"`
- `credentials.StartCallbackServer`
- `credentials.BuildAuthorizeURL`
- `credentials.ExchangeCode`
- `credentials.GenerateState`
- `credentials.TokenExpiry`
- `credentials.AuthTypeOAuth`（Run() 内では使わない。RunWithLoginRequestCapture の AuthTypeAPIKey に変更）

## テスト設計

### 既存テスト（変更なし、カバー済み）

| テスト | 内容 |
|--------|------|
| `TestAuthLoginCmd_SaveAPIKey` | APIキーが tokens.json に保存される ✅ |
| `TestAuthLoginCmd_NoProfile` | --profile 未指定でエラー ✅ |

### 追加テスト不要な理由
- `Run()` は stdin/config 依存で単体テストが困難
- `RunWithLoginRequestCapture()` 経由の既存テストで機能をカバー済み
- `--api-key` フラグの解決は Kong が保証

## 影響チェックリスト

- [ ] `credentials.AuthTypeAPIKey` 定数 — credentials.go:21 に存在確認済み
- [ ] `AuthEntry.APIKey` フィールド — credentials.go:36 に存在確認済み
- [ ] `RunWithLoginRequestCapture` — api_key AuthType を受け入れ済み（:159-205）
- [ ] テスト全通過 — `go test ./...` で確認

## 検証手順

```bash
# 1. ビルド
go build -o /tmp/lv ./cmd/logvalet/

# 2. テスト（既存テストが全通過すること）
go test ./internal/cli/... -v -run TestAuthLogin

# 3. 手動確認（--profile は config.toml にスペース設定が必要）
/tmp/lv auth login --profile work --api-key TEST_KEY
# 期待: {"schema_version":"1","result":"ok","profile":"work","auth_type":"api_key","saved":true}
```

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| OAuth テスト（TestAuthWhoamiCmd_Run_OAuth）が tokens.json の oauth エントリに依存 | 低 | whoami は auth_type を表示するだけで変更不要 |
| resolveAuthBaseURL が config.toml 未設定時にエラー | 低 | エラーメッセージは既存のまま（設定ドキュメントで案内） |
