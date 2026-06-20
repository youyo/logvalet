# AGENTS.md

## プロジェクト概要

`logvalet` は Backlog 向けの LLM-first CLI / MCP サーバーです。Backlog REST API の薄いラッパーではなく、Claude Code や Codex などのエージェントが扱いやすい、安定したコンパクトな JSON digest を返すことを主目的にしています。

- モジュール: `github.com/youyo/logvalet`
- バイナリ: `logvalet`
- 推奨エイリアス: `lv`
- 設定ファイル: `~/.config/logvalet/config.toml`
- トークンストア: `~/.config/logvalet/tokens.json`

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

## 実行コマンド

```bash
go build -o logvalet ./cmd/logvalet/
go test ./...
go vet ./...
```

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

つまり「Backlog の検索 API」が課題検索を指す場合、`issue list --keyword` / `logvalet_issue_list.keyword` として実装済みです。ドキュメント検索と Wiki keyword 検索も実装済みです。

スペース全体を単一エンドポイントで横断検索する Backlog API は、公式 API 一覧上は確認できていない。logvalet では `logvalet search <keyword>` / MCP `logvalet_search` として、issue `keyword`、document `keyword`、wiki `keyword` の個別 API を呼び分け、統合 digest を返す。

## 開発ルールと注意点

- 既存方針は TDD。Backlog API は mock client / httptest ベースでテストする。
- stdout は原則として機械可読な結果のみ。警告・診断は stderr。
- エラーは JSON envelope と exit code に正規化される。`internal/app` を確認する。
- JSON key は `snake_case`、Go field は `CamelCase` + 明示的 JSON tag。
- 仕様書や README よりも実装とテストを優先して確認する。特にロードマップ上の未実装項目が docs に残っていることがある。
- マルチスペース対応の変更では、単一スペース後方互換と fan-out (`internal/cli/fanout_helper.go`, `internal/space`) の両方を見る。
- MCP tool 追加時は登録、category annotation、tests の3点を揃える。
