# 認証まわりのコマンド体系を「configure + MCP」に整理する

## Context

logvalet の認証まわりは歴史的経緯で複数のエントリポイントが重複している。

1. **`auth login / logout / whoami / list`** — API Key を `tokens.json` に保存する CLI。実態は `configure` で既に APIキー保存まで統合済みのため機能重複 (`internal/cli/config_cmd.go:239-258`)。
2. **`configure`** と **`config init`** — `ConfigureCmd` が `ConfigInitCmd` への単純エイリアスになっており (`internal/cli/config_cmd.go:20-39`)、両方がトップレベルから呼べる。
3. **Backlog OAuth / OIDC** — MCP サーバーでしか使えないが (`plans/backlog-oauth-roadmap.md` / `README.md` L541)、`LOGVALET_BACKLOG_*` などの環境変数は名前空間上 MCP 専用である事が読み取りにくい。

方針：
- **API Key 設定は `configure` 一本**（AWS CLI 流）
- **OAuth / OIDC は `logvalet mcp` の引数と `LOGVALET_MCP_*` 環境変数に閉じる**
- **認証確認は `logvalet user me` 新設**（`auth whoami` の代替）
- **認証削除は `tokens.json` の手動編集**（専用コマンドは追加しない）

v0.10.0 リリース済み。`auth` 系削除と env リネームは明確な breaking change だが、メジャーバージョンに達していないため受け入れ可能。

## 変更方針

### Phase 1 — `auth` コマンド群の廃止

| 操作 | 対象 |
|------|------|
| 削除 | `internal/cli/auth.go`（全435行） |
| 削除 | `internal/cli/auth_test.go`（全565行） |
| 編集 | `internal/cli/root.go` L8 — `Auth AuthCmd` 行を削除 |
| 編集 | `internal/cli/runner.go` L70 — エラー文言 `(run logvalet auth login)` → `(run logvalet configure)` |
| 編集 | `internal/cli/config_cmd.go` L282 — `next step: logvalet auth login ...` → `next step: logvalet user me to verify` |
| 編集 | `cmd/logvalet/main_test.go` L139 / L155 — `"auth"` / `"auth login"` をハードコードしたテストケースを新コマンド（例：`configure` / `user`）に差し替え |

### Phase 2 — `configure` に一本化、`config` コマンドグループを削除

現状の `ConfigureCmd` は薄いラッパーなので、`ConfigInitCmd.RunWithDeps` のロジックを `ConfigureCmd` へ移し、`ConfigCmd` / `ConfigInitCmd` を削除する。

| 操作 | 対象 |
|------|------|
| 編集 | `internal/cli/root.go` — `Config ConfigCmd` と `Configure ConfigureCmd` の行を再整理。残すのは `Configure ConfigureCmd` のみ |
| 編集 | `internal/cli/config_cmd.go` — `ConfigCmd` / `ConfigInitCmd` 削除。`ConfigureCmd` 自身が `Run` / `RunWithDeps` を持つ形に書き換え |
| 編集 | `internal/cli/config_cmd_test.go` — テストを `ConfigureCmd` ベースへ移行 |

**注意**：`config_cmd.go:28` の既存 flag は `InitProfile` / `InitSpace` / `InitBaseURL` / `InitAPIKey` だが、AWS CLI 流に揃えるなら `--profile-name` / `--space` / `--base-url` / `--api-key` の方が直感的。ただし `--profile` は GlobalFlags で既に使われているため衝突回避のためフラグ名は現状維持（`--init-profile` 等）。インタラクティブ既定挙動は維持。

### Phase 3 — `logvalet user me` を新設

既存 `--assignee me` リゾルバ (`internal/cli/resolve.go:44-49`) が使う `client.GetMyself()` を同じ要領で呼ぶ。

| 操作 | 対象 |
|------|------|
| 追加 | `internal/cli/user.go` — `UserMeCmd` 構造体と `Run` 実装 |
| 追加 | `internal/cli/user.go` の `UserCmd` に `Me UserMeCmd \`cmd:"" help:"display current authenticated user"\`` 登録 |
| 追加 | `internal/cli/user_me_test.go`（既存 `mockBacklogClient` パターン流用） |

実装イメージ：
```go
type UserMeCmd struct{}

func (c *UserMeCmd) Run(g *GlobalFlags) error {
    rc, err := buildRunContext(g)
    if err != nil { return err }
    u, err := rc.Client.GetMyself(context.Background())
    if err != nil { return err }
    return rc.Renderer.Render(os.Stdout, u)
}
```

### Phase 4 — MCP の Backlog OAuth / TokenStore 設定を「CLI flag + env 両対応」に統一（breaking）

現状、OIDC 系 (`LOGVALET_MCP_OIDC_*`) は flag+env 両対応だが、Backlog OAuth 系と TokenStore 系は env-only。これを OIDC と揃えて全て両対応にする。あわせて env 名を `LOGVALET_MCP_*` 名前空間に寄せる。

#### 4-1. env リネーム

| 旧 | 新 |
|----|----|
| `LOGVALET_BACKLOG_CLIENT_ID` | `LOGVALET_MCP_BACKLOG_CLIENT_ID` |
| `LOGVALET_BACKLOG_CLIENT_SECRET` | `LOGVALET_MCP_BACKLOG_CLIENT_SECRET` |
| `LOGVALET_BACKLOG_REDIRECT_URL` | `LOGVALET_MCP_BACKLOG_REDIRECT_URL` |
| `LOGVALET_OAUTH_STATE_SECRET` | `LOGVALET_MCP_OAUTH_STATE_SECRET` |
| `LOGVALET_TOKEN_STORE` | `LOGVALET_MCP_TOKEN_STORE` |
| `LOGVALET_TOKEN_STORE_SQLITE_PATH` | `LOGVALET_MCP_TOKEN_STORE_SQLITE_PATH` |
| `LOGVALET_TOKEN_STORE_DYNAMODB_TABLE` | `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE` |
| `LOGVALET_TOKEN_STORE_DYNAMODB_REGION` | `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION` |

#### 4-2. `McpCmd` に CLI flag を追加

`internal/cli/mcp.go` の `McpCmd` 構造体に以下 8 flag を追加（既存の OIDC flag と同じ group="auth" / env タグの様式で）：

| フラグ | 対応 env | グループ |
|--------|---------|---------|
| `--backlog-client-id` | `LOGVALET_MCP_BACKLOG_CLIENT_ID` | `auth` |
| `--backlog-client-secret` | `LOGVALET_MCP_BACKLOG_CLIENT_SECRET` | `auth` |
| `--backlog-redirect-url` | `LOGVALET_MCP_BACKLOG_REDIRECT_URL` | `auth` |
| `--oauth-state-secret` | `LOGVALET_MCP_OAUTH_STATE_SECRET` | `auth` |
| `--token-store` | `LOGVALET_MCP_TOKEN_STORE` | `store` |
| `--token-store-sqlite-path` | `LOGVALET_MCP_TOKEN_STORE_SQLITE_PATH` | `store` |
| `--token-store-dynamodb-table` | `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE` | `store` |
| `--token-store-dynamodb-region` | `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION` | `store` |

#### 4-3. 実装変更点

- `internal/auth/config.go` の `OAuthEnvConfig` ロード関数を「env から読む」→「呼び出し元から値を受け取る」形にリファクタ、または「env を読んだ後に flag で上書きするマージ関数」を追加
- 現在 `BuildOAuthDeps` は `*auth.OAuthEnvConfig` を受け取る (`internal/cli/mcp_oauth.go:52`) ので、`McpCmd` → `OAuthEnvConfig` への変換関数を追加するのがシンプル
- `internal/cli/mcp.go:42` `ValidateEnv` は「env 読み」専用なので不要になる。`McpCmd` 自身の値（flag または env で埋まる）を見るように `Validate()` へ統合。`LOGVALET_BACKLOG_CLIENT_ID` 直接参照は削除

| 操作 | 対象 |
|------|------|
| 編集 | `internal/cli/mcp.go` — `McpCmd` に 8 flag 追加、`Validate/ValidateEnv` 再設計 |
| 編集 | `internal/auth/config.go` — env ロードは据え置きで OK（flag 値優先のマージ関数を追加、または `McpCmd` 側で `OAuthEnvConfig` を組み立てる） |
| 編集 | `internal/auth/config_test.go` — 新 env 名でのテスト |
| 編集 | `internal/cli/mcp_oauth.go` — `BuildOAuthDeps` 呼び出し側で `McpCmd` から `OAuthEnvConfig` を組み立てる |
| 編集 | `internal/cli/mcp_test.go` 他 MCP 関連テスト |

### Phase 5 — ドキュメント更新

| 操作 | 対象 |
|------|------|
| 編集 | `README.md` L96-99（コマンド表）、L554 / L561（使用例）、L618 / L636-644 / L653-659 / L731-738 / L758-759（env テーブルと例）を `LOGVALET_MCP_*` に統一 |
| 編集 | `README.ja.md` 対応箇所（L98-101, L553, L560 ほか） |
| 編集 | `skills/logvalet/SKILL.md:76` — 初期設定ガイドを `logvalet configure` のみに |
| 編集 | `docs/agentcore-deployment.md` 内の env 名更新、および新 flag の紹介 |
| 据え置き | `docs/specs/` 配下（初期構想ドキュメントのため編集不要） |
| 据え置き | `plans/` 配下（歴史記録） |

### Phase 6 — 動作確認

1. `go vet ./...`
2. `go test ./...`
3. `go build -o logvalet ./cmd/logvalet/`
4. 手動スモーク：
   - `./logvalet configure --init-profile default --init-space <space> --init-api-key <key>` → `tokens.json` が書かれる
   - `./logvalet user me` → 自分のユーザー情報が返る
   - `./logvalet project list` → Backlog API が叩ける
   - MCP: 旧 env 名で起動失敗することを確認、新 env 名で起動成功することを確認
5. zsh completion スモーク（`eval "$(./logvalet completion zsh)"` 後）：
   - `./logvalet --completion-bash auth` → 空出力（`auth` ノードが消滅）
   - `./logvalet --completion-bash configure` → `--init-api-key` 等のフラグが列挙
   - `./logvalet --completion-bash user` → `list`, `get`, `activity`, `workload`, `me` が列挙
   - `./logvalet --completion-bash mcp` → 新規 `--backlog-client-id` 等が列挙

## 影響しないもの（確認済み）

- `auth` 系 CLI コマンドは `skills/` 内で直接実行されていない（`SKILL.md:76` のガイダンス文言のみ）
- 環境変数 `LOGVALET_API_KEY` / `LOGVALET_ACCESS_TOKEN` は据え置き（これは CLI / MCP 共通の Backlog API 認証であり、MCP 限定ではない）
- `LOGVALET_MCP_*` 既存 flag（OIDC 系）は影響なし
- `credentials.Resolver` の解決順序（`internal/credentials/credentials.go:185-245`）は変えない
- **zsh completion**：
  - `logvalet completion zsh` の出力（`internal/cli/completion.go:16-24`）は `_logvalet() { completions=($(logvalet --completion-bash ...)) }` の形で、**サブコマンド名を一切ハードコードしていない**（`name` 変数は `"logvalet"` 固定のみ）。ユーザーは `eval "$(logvalet completion zsh)"` で補完関数を登録する。**スクリプト出力そのものは変更不要**。
  - `--completion-bash` は Kong 標準ではなく **自前実装**（`cmd/logvalet/main.go:44` でインターセプト → `collectCompletions` L67 / `handleCompletionBash` L163）。実装は Kong AST の `node.Children` / `node.Flags` を動的に走査する汎用ロジックなので、コマンド追加・削除・flag 増減は**自動追従**し、実装コード修正も不要。
  - **唯一の例外**：`cmd/logvalet/main_test.go:139, 155` が `"auth"` / `"auth login"` をハードコードしたテストケースを持つため、Phase 1 で差し替え必要（上記表に既記）。

## ロールアウト単位

PR は 3 本に分割推奨：

1. **PR1**: Phase 1 + Phase 2（auth 削除・configure 一本化）＋ README 該当部分
2. **PR2**: Phase 3（`user me` 新設）＋ runner.go 誘導文修正
3. **PR3**: Phase 4（MCP flag 追加 + env リネーム、breaking）＋ README env テーブル / flag ドキュメント
