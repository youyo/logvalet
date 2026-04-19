---
title: logvalet v0.15.0 — idproxy v0.3.0 取り込みと RefreshTokenTTL パススルー
project: logvalet
parent_plan: /Users/youyo/src/github.com/heptagon-inc/logvalet-mcp/plans/sunny-noodling-sutton-logvalet.md
depends_on: idproxy v0.3.0 (released)
author: devflow:planning-agent
created: 2026-04-20
status: Draft / Ready for Implementation
complexity: L
target_repo: /Users/youyo/src/github.com/youyo/logvalet
---

# logvalet v0.15.0 — idproxy v0.3.0 取り込みと RefreshTokenTTL パススルー

## Context

idproxy v0.3.0 がリリースされ、OAuth 2.1 の `refresh_token` grant と `RefreshTokenTTL` 設定が追加された。logvalet は `mcp` サブコマンドで idproxy を埋め込み使用しており、依存バージョンを v0.2.2 → v0.3.0 に更新し、運用側から TTL を上書きできるフラグ/環境変数を追加する必要がある。

**親プランとの差分（重要）:**
親プラン `sunny-noodling-sutton-logvalet.md` は「`idproxy.Config.OAuth.RefreshTokenTTL` に代入」と記載しているが、実際の idproxy v0.3.0 ソースでは `RefreshTokenTTL` は **`Config` 直下**（`OAuthConfig` 配下ではない）に定義されている。本プランでは正しい代入先を採用する。

- 正: `idproxy.Config{ RefreshTokenTTL: c.RefreshTokenTTL, ... }`
- 誤: `idproxy.Config{ OAuth: &idproxy.OAuthConfig{ RefreshTokenTTL: ... } }`

### 前提

- 現状 `go.mod`: `github.com/youyo/idproxy v0.2.2`
- `BuildAuthConfig`: `internal/cli/mcp_auth.go:61-113` — 現在 `RefreshTokenTTL` の設定は未実装
- `McpCmd`: `internal/cli/mcp.go:24-55` — 認証フラグは `group:"auth"` + `env:"LOGVALET_MCP_*"` で統一
- `BuildAuthConfig` 呼び出し箇所は `internal/cli/mcp.go:210` のみ
- idproxy v0.3.0 の挙動: `Config.RefreshTokenTTL == 0` のとき `Validate()` が `DefaultConfig.RefreshTokenTTL`（30日）で補完

### Backlog OAuth refresh との関係（別経路である理由）

logvalet には **2 つの独立した OAuth refresh 経路** が存在する。本プランが触るのは (A) のみで、(B) には影響しない。

| # | 経路 | 目的 | 対象トークン | 担当コード | TTL 決定 |
|---|------|------|------------|-----------|---------|
| A | **idproxy refresh_token grant** | MCP クライアント ↔ logvalet MCP サーバー間の認証トークン再発行 | idproxy が発行する JWT（`SigningKey` で署名） | `BuildAuthConfig()` → idproxy v0.3.0 | `Config.RefreshTokenTTL`（本プランで追加） |
| B | **Backlog OAuth refresh** | logvalet ↔ Backlog API 間のアクセストークン再取得 | Backlog 発行 access_token / refresh_token | `internal/auth/manager.go:85-127` + `internal/auth/provider/backlog.go:107-200` | Backlog 側で決定（logvalet からは制御不能） |

**(B) の動作は今回変更なしで正常**である根拠:
1. `provider/backlog.go:110-127` で `grant_type=refresh_token` を Backlog token endpoint に直接 POST（idproxy 非経由）
2. `manager.go:85` の `NeedsRefresh(5*time.Minute)` で期限 5 分前に自動リフレッシュ
3. 既存テストで網羅: `backlog_test.go:216-298`（TestBacklogProvider_RefreshToken / _Error）、`manager_test.go:115-267`（TestGetValidToken_AutoRefresh / _RefreshFailed）
4. (A) と (B) は HTTP レイヤーも識別子も独立: (A) は idproxy 内部 `OAuthConfig.SigningKey` で署名した JWT、(B) は Backlog が発行する access_token を `TokenStore` に保存

**ユーザー体験での流れ（変更後も同じ）**:
```
MCP Client ──(1)── idproxy (JWT, TTL: RefreshTokenTTL) ──(2)── logvalet handler
                                                                    │
                                                                  (3) GetValidToken
                                                                    │
                                                              TokenManager (5min margin)
                                                                    │
                                                                  (4) RefreshToken if due
                                                                    │
                                                          Backlog token endpoint (grant_type=refresh_token)
```
- (1)(2) の TTL は本プランの `RefreshTokenTTL`（デフォルト 30 日）
- (3)(4) は Backlog の TTL に従う（logvalet からは制御しない）
- 両者は独立に動作し、片方のトークン切れは他方に伝搬しない

## ゴール

idproxy v0.3.0 の `RefreshTokenTTL` を logvalet CLI から上書き可能にし、v0.15.0 としてリリース。

## スコープ

### 含む
- `go.mod` / `go.sum` の idproxy を v0.3.0 に更新
- `McpCmd` に `RefreshTokenTTL` フィールドを追加（フラグ `--refresh-token-ttl`、env `LOGVALET_MCP_REFRESH_TOKEN_TTL`、group `auth`）
- `BuildAuthConfig()` で `idproxy.Config.RefreshTokenTTL` にパススルー
- `internal/cli/mcp_auth_test.go` に RefreshTokenTTL 伝搬のテスト追加
- `CHANGELOG.md` v0.15.0 エントリ
- `README.md` / `README.ja.md` の Mode 4/6 環境変数例に `LOGVALET_MCP_REFRESH_TOKEN_TTL` を追記

### 含まない
- idproxy 本体の実装（リリース済）
- Backlog OAuth 関連コード（`internal/auth/manager.go`）の変更
- logvalet-mcp Lambda デプロイ（親プランの Phase 3）
- `time.ParseDuration` 未対応の `d` / `w` 単位への独自拡張（YAGNI; `720h` 等の標準単位で十分）

---

## TDD テスト設計

テスト対象: `internal/cli/mcp_auth_test.go`

| ID | 内容 | 種別 |
|----|------|------|
| T1 | `McpCmd.RefreshTokenTTL = 72h` を `BuildAuthConfig` に渡すと `idproxy.Config.RefreshTokenTTL == 72h` | unit |
| T2 | `McpCmd.RefreshTokenTTL = 0`（未設定）のとき `idproxy.Config.RefreshTokenTTL == 0`（idproxy Validate に委ねる） | unit |
| T3 | 既存 `BuildAuthConfig` 9 テストの回帰がない（go test ./internal/cli/... 全 pass） | regression |

### Red → Green → Refactor

1. **Red**: T1/T2 を FAIL で `mcp_auth_test.go` に追加（`authCfg.RefreshTokenTTL` フィールドが未定義で build fail → idproxy v0.3.0 依存更新が先）
2. **Green**:
   - Step 1: `go get github.com/youyo/idproxy@v0.3.0 && go mod tidy`
   - Step 2: `McpCmd` に `RefreshTokenTTL time.Duration` フィールド追加
   - Step 3: `BuildAuthConfig` に `RefreshTokenTTL: c.RefreshTokenTTL` を追加
3. **Refactor**: 無し（YAGNI。単純代入のため抽象化不要）

---

## 実装手順

### Step 1: 依存更新
```bash
cd /Users/youyo/src/github.com/youyo/logvalet
go get github.com/youyo/idproxy@v0.3.0
go mod tidy
go build ./...
go test ./...
```
- 確認: `go.mod` に `github.com/youyo/idproxy v0.3.0` があること
- 既存テストの破綻なし

### Step 2: テスト追加（Red）
ファイル: `internal/cli/mcp_auth_test.go`

既存テーブルテストと同様のスタイルで以下 2 ケースを追加:
```go
{
    name: "RefreshTokenTTL 72h is propagated",
    input: &McpCmd{ /* 既存有効値 */ RefreshTokenTTL: 72 * time.Hour },
    check: func(t *testing.T, cfg idproxy.Config) {
        if cfg.RefreshTokenTTL != 72*time.Hour { t.Errorf(...) }
    },
},
{
    name: "RefreshTokenTTL zero is passed as zero",
    input: &McpCmd{ /* 既存有効値 */ },
    check: func(t *testing.T, cfg idproxy.Config) {
        if cfg.RefreshTokenTTL != 0 { t.Errorf(...) }
    },
},
```
- この時点ではフィールド未定義で build fail する（Red 成立）

### Step 3: McpCmd フラグ追加（Green）
ファイル: `internal/cli/mcp.go:51` 付近（既存 `SigningKey` の直後、auth group 末尾）

```go
// idproxy OAuth TTL
RefreshTokenTTL time.Duration `name:"refresh-token-ttl" help:"MCP OAuth refresh token TTL (e.g. 720h for 30d). 0 = idproxy default (30 days)" group:"auth" env:"LOGVALET_MCP_REFRESH_TOKEN_TTL"`
```
- `time` パッケージは既存 import で使用中か要確認。未使用なら import 追加
- Kong は `time.Duration` 型を `time.ParseDuration` でパースする（`d` 単位は非サポート、`720h` 等を使用）

### Step 4: BuildAuthConfig パススルー（Green）
ファイル: `internal/cli/mcp_auth.go:96-112`

`idproxy.Config{...}` リテラルに以下を追加:
```go
return idproxy.Config{
    Providers:       []idproxy.OIDCProvider{...},
    AllowedDomains:  allowedDomains,
    AllowedEmails:   allowedEmails,
    ExternalURL:     c.ExternalURL,
    CookieSecret:    cookieSecret,
    Store:           idpStore,
    RefreshTokenTTL: c.RefreshTokenTTL,  // ← 追加
    OAuth: &idproxy.OAuthConfig{
        SigningKey: signingKey,
    },
}, nil
```
- ゼロ値でも代入（idproxy `Validate()` が 30 日で補完）
- `go test ./internal/cli/...` で T1/T2/T3 Green を確認

### Step 5: ドキュメント更新
- `CHANGELOG.md` `## [Unreleased]` → `### Added` に追記:
  ```
  - feat(mcp): idproxy v0.3.0 取り込みと `--refresh-token-ttl` / `LOGVALET_MCP_REFRESH_TOKEN_TTL` を追加
    - OAuth 2.1 refresh_token grant が有効化
    - 未指定時は idproxy デフォルトの 30 日が適用される
  ```
- `README.md` / `README.ja.md` の Mode 4/6 環境変数例（README.ja.md:590-650）に行を追加:
  ```
  export LOGVALET_MCP_REFRESH_TOKEN_TTL=720h  # 30 days (default)
  ```

### Step 6: リリース
`/release` スキル経由で v0.15.0 として push → tag → CI 監視。
手動で `git tag` しない（プロジェクト標準ワークフロー準拠）。
完了条件:
- v0.15.0 タグ作成
- Homebrew tap 更新（GoReleaser 自動）

---

## 変更対象ファイル

| パス | 変更種別 |
|------|---------|
| `go.mod` | 依存バージョン更新 |
| `go.sum` | ハッシュ更新 |
| `internal/cli/mcp.go` | `RefreshTokenTTL` フィールド追加、必要なら `time` import |
| `internal/cli/mcp_auth.go` | `BuildAuthConfig` で RefreshTokenTTL 代入 |
| `internal/cli/mcp_auth_test.go` | T1/T2 追加 |
| `CHANGELOG.md` | v0.15.0 エントリ |
| `README.md` | 環境変数例追記 |
| `README.ja.md` | 環境変数例追記 |

---

## リスク評価

| リスク | 重大度 | 対策 |
|-------|:-:|------|
| idproxy v0.3.0 の破壊的変更で `BuildAuthConfig` が compile fail | 中 | Step 1 で `go build ./...` し、失敗時は idproxy リリースノートと v0.3.0 ソースで差分確認 |
| `RefreshTokenTTL` の代入先が親プラン記載と異なる | 中 | 本プラン上部で明記済 — 正: `Config` 直下、誤: `OAuthConfig` 配下 |
| Kong の `time.Duration` が `30d` 記法を受け付けない | 低 | ヘルプ文で「e.g. 720h」と明記。`d` 単位は YAGNI で非対応 |
| `time` import 漏れで build fail | 低 | Step 3 で既存 import を確認し、無ければ追加 |
| 既存 9 テストの回帰 | 低 | Step 1/4 後に `go test ./internal/cli/...` で全件 green 確認 |
| Backlog OAuth refresh が壊れる懸念 | 低 | (B) 経路は idproxy 非経由で完全独立。本プラン変更範囲 (`internal/cli/mcp.go` / `mcp_auth.go`) は (B) のコード (`internal/auth/...`) と静的に疎結合。回帰確認: `go test ./internal/auth/...` が green のまま |

---

## 検証

```bash
cd /Users/youyo/src/github.com/youyo/logvalet

# ビルド + 全テスト
go build ./...
go test ./...

# ピンポイント
go test ./internal/cli/... -run TestBuildAuthConfig -v

# Backlog OAuth refresh の回帰確認（別経路だが念のため）
go test ./internal/auth/... -v

# CLI help
go run ./cmd/logvalet mcp --help | grep refresh-token-ttl
```

期待:
- 全テスト green
- `--refresh-token-ttl` が help 出力に含まれる
- 環境変数 `LOGVALET_MCP_REFRESH_TOKEN_TTL` が help の env 列（または説明）に表示される

エンドツーエンド検証は親プラン `sunny-noodling-sutton-logvalet.md` 経由の logvalet-mcp Lambda デプロイで実施。

## ロールバック

```bash
# コミット単位で revert（v0.15.0 が tag 済みでも patch release で戻せる）
git revert <commit-sha>

# または go.mod を idproxy v0.2.2 に戻し、フラグ追加を revert
go get github.com/youyo/idproxy@v0.2.2 && go mod tidy
```

---

## チェックリスト（複雑度 L、対象項目のみ）

### 観点1: 実装実現可能性と完全性
- [x] 手順に抜け漏れなし（Step 1〜6 で依存更新→Red→Green→ドキュメント→リリース）
- [x] 各ステップが具体的（ファイル行数付き）
- [x] 依存関係明示（Step 1 → Step 2 → …）
- [x] 変更対象ファイルを表で列挙
- [x] 影響範囲特定（Backlog OAuth 無関係を明記）

### 観点2: TDD テスト設計
- [x] 正常系（T1: 72h 伝搬）、異常系/境界（T2: 0 値パススルー）、回帰（T3）
- [x] 入出力具体的（`72 * time.Hour` → `cfg.RefreshTokenTTL`）
- [x] Red → Green → Refactor 順序明示
- [x] モック不要（純粋関数のテスト）

### 観点3: アーキテクチャ整合性
- [x] 既存 `McpCmd` フラグ命名規則（`LOGVALET_MCP_*` + group `auth`）に従う
- [x] 既存 `BuildAuthConfig` のパターン踏襲（`idproxy.Config` リテラルに追加）

### 観点4: リスク評価
- [x] 親プラン記述ミス（`OAuthConfig.RefreshTokenTTL`）を主要リスクとして明記
- [x] Kong の duration パースの制約を記載
- [x] ロールバック手順あり

### 観点5: シーケンス図
- N/A（単純な値パススルーで処理フロー図は不要）

---

## Next Action

このプランを実装するには以下を実行してください:
- `/devflow:implement` — このプランに基づいて実装を開始
- `/devflow:cycle` — 自律ループで複数マイルストーンを連続実行

実装後は `/release` スキルで v0.15.0 をリリース。
