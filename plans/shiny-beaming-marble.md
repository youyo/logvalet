# 新ロードマップ計画: logvalet — 完全動作実装ロードマップ

## Context

M01〜M12 の基盤実装は完了済みだが、以下の問題が残存している：
1. **全31コマンドが stub 状態**（全 `Run()` が `ErrNotImplemented` を返す）
2. **リポジトリ名タイポ**: `logvaret` → `logvalet`（バイナリ名・モジュールパスは `logvalet` が正）
3. **`cmd/lv/` ディレクトリ名が不適切**: `lv` は completion エイリアス専用、ディレクトリは `cmd/logvalet/` にすべき
4. **auth login が未統合**: OAuth 実装コード（`credentials/oauth.go`）は完成済みだが `Run()` がスタブ
5. **credential/config → HTTPClient 統合レイヤーが未実装**: 全コマンドで共通が必要

## ユーザー確認事項

| 項目 | 決定 |
|------|------|
| バイナリ名 | `logvalet`（リポジトリ名 `logvaret` は後日 GitHub で rename） |
| auth login フラグ | profile が config に存在すれば `--profile` のみ OK。profile がなければ `--base-url` 必須。なければ exit code 2 |
| 実装範囲 | 全31コマンド完全実装 |
| `lv` エイリアス | completion `--short` フラグでのみ生成。`cmd/lv/` → `cmd/logvalet/` にリネーム |

---

## 作成するファイル

実装時に以下を作成・更新する：

### 1. `plans/logvalet-roadmap.md` の更新
- M01〜M12 を完了（`[x]`）にマーク
- M13〜M20 を新規追加
- Current Focus を M13 に更新

### 2. マイルストーン詳細計画ファイル（新規作成）
- `plans/logvalet-m13-rename.md`
- `plans/logvalet-m14-auth-login.md`
- `plans/logvalet-m15-cli-integration.md`
- `plans/logvalet-m16-issue-impl.md`
- `plans/logvalet-m17-project-meta-impl.md`
- `plans/logvalet-m18-activity-user-impl.md`
- `plans/logvalet-m19-document-team-space-impl.md`
- `plans/logvalet-m20-release.md`

---

## 新マイルストーン仕様

### M13: リポジトリ名・ディレクトリ名の整合性修正

**目標**: バイナリ名 `logvalet`、エイリアス `lv` の仕様をコードベースに反映する

**対象ファイル**:
- `cmd/lv/` → `cmd/logvalet/`（ディレクトリリネーム）
- `CLAUDE.md` — ビルドコマンドを `go build -o logvalet ./cmd/logvalet/` に更新
- `.goreleaser.yaml` — binary name / cmd ディレクトリ参照を確認・更新
- `plans/logvalet-roadmap.md` — 対象リポジトリパス更新

**実装ステップ**:
1. `cmd/lv/` を `cmd/logvalet/` にリネーム
2. `go build -o logvalet ./cmd/logvalet/` でビルド確認
3. `.goreleaser.yaml` の `main: ./cmd/logvalet/` に更新
4. `CLAUDE.md` のビルドコマンド更新
5. `go test ./...` グリーン確認

**TDD**: なし（リネームのみ）

---

### M14: auth login 完全実装

**目標**: `logvalet auth login` が実際に Backlog OAuth2 フローを完了して tokens.json に保存できる

**フロー設計**:
```
logvalet auth login [--profile <name>] [--base-url <url>]

1. --profile が指定されている場合:
   a. config.toml からプロファイルを読み込む
   b. プロファイルが存在する → base_url を自動取得
   c. プロファイルが存在しない → --base-url が必須（なければ exit 2）

2. base_url から Backlog OAuth2 フローを開始:
   a. Client ID / Client Secret をプロンプト入力（stdin）
   b. localhost callback server を起動（credentials/oauth.go の StartCallbackServer 使用）
   c. ブラウザで認可 URL を開く（os.Open または os.Stderr にプリント）
   d. callback で code を受け取り ExchangeCode でトークン取得
   e. tokens.json に保存

3. 成功時に JSON レスポンスを stdout に出力:
   {
     "schema_version": "1",
     "result": "ok",
     "profile": "work",
     "space": "example",
     "base_url": "https://example.backlog.com",
     "auth_type": "oauth",
     "saved": true
   }
```

**対象ファイル**:
- `internal/cli/auth.go` — AuthLoginCmd.Run() を実装
- `internal/credentials/oauth.go` — 既存実装を接続（StartCallbackServer, ExchangeCode）
- `internal/config/config.go` — ConfigLoader でプロファイル解決
- `internal/credentials/credentials.go` — CredentialStore.Save() 使用

**TDD**:
- `auth login --profile <existing>` → OAuth フロー起動
- `auth login --profile <nonexistent>` & no `--base-url` → exit 2
- `auth login --base-url <url>` → プロファイルなしでフロー起動
- モック: OAuth server を mock、token exchange は stub

---

### M15: CLI-API 統合レイヤー（全コマンド共通基盤）

**目標**: 全コマンドで認証情報を解決して `backlog.Client` を初期化できる共通パターンを確立

**設計**:
```go
// internal/cli/runner.go（新規）
type RunContext struct {
    Client   backlog.Client
    Config   *config.Config
    Renderer render.Renderer
}

// 全コマンドで使う共通初期化関数
func buildRunContext(g *GlobalFlags) (*RunContext, error) {
    // 1. config.toml を読み込み
    // 2. credential resolver でトークン/API key を解決
    // 3. backlog.NewHTTPClient(...) でクライアント生成
    // 4. render.New(g.Format) でレンダラー生成
    return &RunContext{...}, nil
}
```

**対象ファイル**:
- `internal/cli/runner.go`（新規）
- `internal/backlog/http_client.go` — NewHTTPClient シグネチャ確認
- `internal/credentials/credentials.go` — Resolver インターフェース確認
- `internal/config/config.go` — Resolve() 使用

**TDD**:
- 正常ケース: 有効な credential → Client 初期化成功
- 認証なし: tokens.json 未存在 → exit 3（authentication error）
- 設定なし: config.toml 未存在 → デフォルト値でフォールバック

---

### M16: issue 系コマンド完全実装（8コマンド）

**対象コマンド**:
- `issue get <issue_key>` — IssueDigestBuilder を経由した構造化 JSON
- `issue list` — ページネーション対応
- `issue digest <issue_key>` — IssueDigestBuilder 使用
- `issue create` — API 呼び出し + dry-run
- `issue update <issue_key>` — API 呼び出し + dry-run
- `issue comment list <issue_key>`
- `issue comment add <issue_key>` — API 呼び出し + dry-run
- `issue comment update <issue_key> <comment_id>` — API 呼び出し + dry-run

**対象ファイル**:
- `internal/cli/issue.go` — 全 Run() を実装
- `internal/digest/issue.go` — IssueDigestBuilder（既存実装を接続）
- `internal/backlog/mock_client.go` — テスト用 mock 使用

**TDD**: MockClient で全ケースをテスト

---

### M17: project/meta 系コマンド完全実装（7コマンド）

**対象コマンド**:
- `project get <project_key>`
- `project list`
- `project digest <project_key>` — ProjectDigestBuilder 使用
- `meta status <project_key>`
- `meta category <project_key>`
- `meta version <project_key>`
- `meta custom-field <project_key>`

**対象ファイル**:
- `internal/cli/project.go`、`internal/cli/meta.go`
- `internal/digest/project.go`

---

### M18: activity/user 系コマンド完全実装（6コマンド）

**対象コマンド**:
- `activity list`
- `activity digest` — ActivityDigestBuilder 使用
- `user list`
- `user get <user_id>`
- `user activity <user_id>`
- `user digest <user_id>` — UserDigestBuilder 使用

**対象ファイル**:
- `internal/cli/activity.go`、`internal/cli/user.go`
- `internal/digest/activity.go`、`internal/digest/user.go`

---

### M19: document/team/space 系コマンド完全実装（11コマンド）

**対象コマンド**:
- `document get <id>`, `document list`, `document tree`, `document digest`, `document create`
- `team list`, `team project <project_key>`, `team digest <team_id>`
- `space info`, `space disk-usage`, `space digest`

**対象ファイル**:
- `internal/cli/document.go`、`internal/cli/team.go`、`internal/cli/space.go`
- `internal/digest/document.go`、`internal/digest/team.go`、`internal/digest/space.go`

---

### M20: v0.1.0 リリース

**チェックリスト**:
- [ ] `go test ./...` が全パス
- [ ] `go build -o logvalet ./cmd/logvalet/` が成功
- [ ] `logvalet completion zsh --short` で `lv` エイリアスが生成されること確認
- [ ] GitHub Secrets に `APP_ID` / `APP_PRIVATE_KEY` を設定
- [ ] `git tag v0.1.0 && git push origin v0.1.0` → GoReleaser 自動実行
- [ ] GitHub Releases にバイナリが公開されること確認

---

## 検証方法

```bash
# M13 後
go build -o logvalet ./cmd/logvalet/
./logvalet --version

# M14 後
./logvalet auth login --profile myprofile
./logvalet auth login --base-url https://example.backlog.com  # プロンプト入力

# M15-M19 後（実際の API 疎通確認）
./logvalet issue list --project PROJ
./logvalet issue digest PROJ-123
./logvalet project digest PROJ -f md

# 全テスト
go test ./...

# completion & lv エイリアス
eval "$(./logvalet completion zsh --short)"
lv --version
```

## spec 仕様との整合確認済み事項

| 仕様 | 現状 | アクション |
|------|------|-----------|
| バイナリ名 `logvalet` | `cmd/lv/` がある | M13 でリネーム |
| `lv` は completion エイリアスのみ | completion.go 実装済み ✅ | 変更不要 |
| `auth login` — localhost OAuth | oauth.go 実装済み ✅ | M14 で Run() に接続 |
| auth login に profile + base_url | Run() がスタブ | M14 で実装 |
| 全コマンド Run() 実装 | 全 stub | M16-M19 で実装 |
| exit code 仕様 | app.ExitXxx 定義済み ✅ | M15 で統合 |
| dry-run サポート | 一部実装済み ✅ | M16-M19 で全コマンドに |
