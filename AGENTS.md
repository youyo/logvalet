# AGENTS.md

## プロジェクト概要

`logvalet` は Backlog 向けの LLM-first CLI / MCP サーバーです。Backlog REST API の薄いラッパーではなく、Claude Code や Codex などのエージェントが扱いやすい、安定したコンパクトな JSON digest を返すことを主目的にしています。

- モジュール: `github.com/youyo/logvalet`
- バイナリ: `logvalet`
- 推奨エイリアス: `lv`
- 設定ファイル: `~/.config/logvalet/config.toml`
- トークンストア: `~/.config/logvalet/tokens.json`

実エントリポイントは `cmd/logvalet/main.go`、Kong の root command 定義は `internal/cli/root.go`。CLI 実行経路は `internal/cli/runner.go` で config → credentials → Backlog client → renderer を組み立てる。Backlog API は `internal/backlog.Client` interface 越しに扱う。分析ロジックは `internal/analysis/`、MCP tool 登録は `internal/mcp/`。

## 技術スタック

- Go 1.26.1
- CLI: Kong
- MCP: `github.com/mark3labs/mcp-go`
- OAuth / 認証: `internal/auth`, `internal/credentials`, `internal/transport/http`
- マルチスペース保存: memory / SQLite / DynamoDB
- リリース: GoReleaser, GitHub Actions, Homebrew tap

## 主要ディレクトリ

- `cmd/logvalet`: CLI エントリポイント。
- `internal/cli`: Kong のサブコマンド定義と実行処理。
- `internal/backlog`: Backlog API クライアント、リクエストオプション、mock client。
- `internal/domain`: Backlog API レスポンスに対応するドメイン型。
- `internal/digest`: issue / project / document / activity などの agent-friendly digest 生成。
- `internal/analysis`: stale issue、blocker、health、workload、timeline、triage などの決定論的分析。
- `internal/mcp`: MCP server、tool registry、各 `logvalet_*` tool 登録。
- `internal/space`: 複数 Backlog space の登録、解決、fan-out 実行。
- `internal/auth`, `internal/credentials`: API key / OAuth credential 解決と token 管理。
- `internal/render`: JSON / YAML / Markdown / gantt renderer。stdout は機械可読結果用。
- `docs/specs`: 仕様・設計メモ。実装とズレている可能性があるため、コードを最終確認する。
- `plans`: マイルストーン別の実装計画。
- `skills`: Claude Code / agent 向け skill 定義。
- `examples/lambroll`: AgentCore / Lambda deployment 例。

## 開発コマンド

Go は `go.mod` / `mise.toml` ともに `1.26.1`。

- Build: `mise run build`（実体: `go build -o logvalet ./cmd/logvalet/`）
- Unit test: `mise run test`（実体: `go test ./...`）
- Vet: `mise run vet`（実体: `go vet ./...`）
- Full local check: `mise run lint`（`mise run vet` → `mise run test`）
- Focused test: `go test ./internal/<pkg>/... -run TestName`
- MCP integration: `mise run test:integration`（`go test -tags=integration ./internal/cli/... -run TestIntegration -v`）
- Live Backlog E2E: `go test -tags e2e ./internal/e2e/` with `LOGVALET_E2E_API_KEY`, `LOGVALET_E2E_SPACE`, `LOGVALET_E2E_PROJECT_KEY`; `LOGVALET_E2E_ISSUE_KEY` は任意。
- 通常の Go PR CI は見当たらない。完了前にローカルで `mise run lint` を必ず実行する。

代表的な利用例:

```bash
logvalet configure --init-profile work --init-space myspace --init-api-key YOUR_API_KEY
logvalet user me
logvalet issue get PROJ-123
logvalet issue list -k PROJ --status not-closed
logvalet digest --issue PROJ-123
logvalet mcp
logvalet mcp-stdio
```

## CLI の主な機能面

- Issue: get/list/create/update/comment/attachment/context/stale/triage-materials/timeline。
- Project: get/list/blockers/health。
- Digest: issue/project/user/team/space、daily、weekly、unified。
- User/Team/Activity: user lookup、activity list/stats、team list。
- Document: get/list/tree/digest/create/search。
- Wiki: list/get/count/tags/history/stars/attachments/shared files。
- Shared file: list/get/download。
- Watchings: list/count/get/add/update/delete/mark-as-read。
- Space/Spaces: single-space config と multi-space registry / fan-out。
- MCP: CLI 機能の多くを `logvalet_*` tools として公開。

## MCP 実装メモ

MCP tool は `internal/mcp/server.go` から以下を登録します。

- `RegisterIssueTools`
- `RegisterProjectTools`
- `RegisterUserTools`
- `RegisterActivityTools`
- `RegisterDocumentTools`
- `RegisterTeamTools`
- `RegisterSpaceTools`
- `RegisterMetaTools`
- `RegisterSharedFileTools`
- `RegisterStarTools`
- `RegisterWatchingTools`
- `RegisterWikiTools`
- `RegisterAnalysisTools`
- `RegisterSpaceRegistryTools`

Tool の read/write/destructive 分類は `internal/mcp/tool_categories.go` を確認します。登録済み tool と分類の対応はテストで検証されています。

`logvalet mcp` は Streamable HTTP、`logvalet mcp-stdio` は stdio transport。stdio では attachment upload の `file_paths` は無効。MCP OAuth で `--backlog-client-id` を使う場合、`--auth` なしは fast-fail。remote/multi-instance の token/space store は DynamoDB 前提。

## Backlog API クライアントの現状

中心インターフェースは `internal/backlog/client.go` の `Client` です。実 HTTP 実装は `internal/backlog/http_client.go`、テスト用 mock は `internal/backlog/mock_client.go` です。

実装済みの主な API:

- Users: myself/list/get/user activities。
- Issues: get/list/create/update/comments/attachments。
- Projects/meta: project get/list/activities/status/category/version/custom fields/issue types/priorities。
- Space: info/disk usage/activities。
- Documents: get/list/tree/search/create/attachments。
- Teams。
- Shared files。
- Wikis。
- Stars。
- Watchings。

## Backlog 検索 API の実装状況

検索として明確に実装済みなのは以下です。

- Document search: `Client.SearchDocuments`, CLI `logvalet document search`, MCP `logvalet_document_search`。
  - Backlog API は `GET /api/v2/documents` に `keyword`, `projectId[]`, `sort`, `order`, `offset`, `count` を付与。
  - Digest は `internal/digest/document_search.go` で snippet/meta/full を生成。
- Wiki keyword filter: `Client.ListWikis` の `ListWikisOptions.Keyword`, CLI `logvalet wiki list PROJECT --keyword`, MCP `logvalet_wiki_list` の `keyword`。
  - Backlog API は `GET /api/v2/wikis?projectIdOrKey=...&keyword=...`。

課題検索については注意が必要です。

- Backlog 公式 API の `GET /api/v2/issues` には `keyword` パラメータがあり、課題のキーワード検索自体は API として存在する。
- `Client.ListIssues` は `GET /api/v2/issues` を呼び、`ListIssuesOptions.Keyword` を `keyword` query parameter として送出する。
- CLI `issue list` と MCP `logvalet_issue_list` は project / assignee / status / keyword / due date / start date / updated date / sort / order / count / offset のフィルタに対応する。
- 専用の `SearchIssues`, `IssueSearch`, `logvalet_issue_search` は存在せず、課題単体の keyword 検索は既存 list API の拡張として扱う。
- `docs/specs/logvalet_multi_space_spec.md` には `logvalet_issue_search` の例がありますが、現状は仕様メモだけで実装済み tool ではありません。

スペース全体を単一エンドポイントで横断検索する Backlog API は、公式 API 一覧上は確認できていない。logvalet では `logvalet search <keyword>` / MCP `logvalet_search` として、issue `keyword`、document `keyword`、wiki `keyword` の個別 API を呼び分け、統合 digest を返す。

## 会話・作業スタイル

- ユーザー向けの会話は日本語。コミットメッセージも日本語。
- TDD 前提: 変更前に失敗する/不足を示すテストを書き、Green 後に refactor する。Backlog API は mock client / httptest ベースでテストする。
- 全コミットが Conventional Commits（`type(scope): 日本語`）。`plans/*.md` を含むコミットは追加で `Plan: plans/<file>.md` フッターを付ける。

## 出力・エラー契約

- stdout は機械可読な結果専用。デフォルト format は JSON。警告・ログ・診断は stderr。
- エラーも stdout に JSON envelope として出す。exit code 定義は `internal/app/exitcode.go` を正とし、`8` は partial failure。
- `internal/mcp/` の tool handler で `os.Stdout` / `fmt.Print*` を使わない。`mcp-stdio` では stdout が JSON-RPC パイプになる。
- 対応 renderer は `json`, `yaml`, `md`, `markdown`, `gantt`。`text` format はない。
- JSON キーは `snake_case`、Go struct field は `CamelCase` + 明示的 JSON tag。

## 設定・認証の落とし穴

- 設定ファイルは XDG 対応で `~/.config/logvalet/config.toml`、token store は `~/.config/logvalet/tokens.json`。
- 一般設定は概ね CLI flags > env > config > default だが、profile 固有の `space` / `base_url` は CLI flags > profile config > env > default。
- 認証情報の実装上の優先順位は CLI flags > `tokens.json` の `authRef` > env。env が profile token を上書きすると思い込まない。
- Write 系には `--dry-run` / `LOGVALET_DRY_RUN` がある。Backlog に書く変更ではまず dry-run を検討する。
- multi-space は read fan-out と write で扱いが違う。read は `--spaces` / `--all-spaces`、write は単一 space 指定が前提。

## リリース

- Release は `v*` tag push で `.github/workflows/release.yml` → GoReleaser。GoReleaser hook は `go mod tidy` と `go test ./...` を実行する。
- GitHub Actions は pinact 管理。workflow を編集したら SHA pinning を崩さない。

## ドキュメントの読み方

- 実行可能な source of truth は `mise.toml`, `go.mod`, `.goreleaser.yaml`, `.github/workflows/*`, 実コード。
- `README.ja.md` や `CLAUDE.md` には古い記述が混じることがある（例: `cmd/lv`, skill 配布方法, MCP tool 数, exit code 表）。迷ったら実コードと `README.md` を優先する。
- 仕様書や README よりも実装とテストを優先して確認する。特にロードマップ上の未実装項目が docs に残っていることがある。
