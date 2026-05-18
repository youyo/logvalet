---
title: logvalet MCP stdio トランスポート対応
project: logvalet
created: 2026-05-18
status: Draft
complexity: M
---

# logvalet MCP stdio トランスポート対応

## Context

現状の `logvalet mcp` サブコマンド (`internal/cli/mcp.go`) は Streamable HTTP トランスポートのみを提供しており、Claude Desktop など stdio 経由で MCP サーバーを起動するローカルクライアントから直接利用できない。`mark3labs/mcp-go` は `NewStdioServer` API で stdio 対応が可能なため、新サブコマンド `logvalet mcp-stdio` を追加して、HTTP モードを変更せずローカル統合を可能にする。

ローカル CLI 起動のユースケースは単一ユーザー・単一プロセスのため、idproxy 経由の OIDC 認証や Backlog OAuth フローは不要。`buildRunContext` 経由で既存の API キー / OAuth トークン (`tokens.json`) を使って `backlog.Client` を初期化する流れに従う。

## スコープ

### 実装範囲
- `logvalet mcp-stdio` サブコマンドの新規追加
- `internal/cli/mcp_stdio.go` (新規) — `McpStdioCmd` 構造体と `Run()` 実装
- `internal/cli/root.go` への 1 行追加 (`McpStdio McpStdioCmd \`cmd:""\`...`)
- `internal/cli/mcp_stdio_test.go` (新規) — Validate / Help テスト
- `README.md` / `docs/` の使い方サンプル追記（Claude Desktop 設定例含む）

### スコープ外
- HTTP MCP サーバー (`mcp.go`) の挙動変更
- OAuth / idproxy / per-user 認証の stdio モード対応（不要）
- ログレベル制御フラグ
- ToolRegistry や個別ツールの変更

## 設計判断

### ファイル分離 vs 既存 `McpCmd` への統合

**選択: 新ファイル `mcp_stdio.go` + 新コマンド `mcp-stdio`**

理由:
- `McpCmd` は OAuth / idproxy / TokenStore など HTTP 固有フラグを 25+ 個持つ。stdio モードで無関係なフラグが Help に出るのは UX を損なう
- `--transport=stdio` フラグ案だと「stdio で `--auth` 指定したら無視されるのか / エラーになるのか」の暗黙挙動が増える。サブコマンド分離なら型レベルで防げる
- 既存テスト・既存ユーザーの挙動を 1 ミリも変えない原則と合致

### stdout/stderr 規約の遵守 (Critical)

stdio MCP は **stdout を JSON-RPC 専用パイプ**として使う。違反すると Claude Desktop 等が起動メッセージを JSON としてパースしようとして即死する。

**stdout 汚染リスクの事前棚卸し結果 (確定済み):**

`fmt.Print*` だけでなく `os.Stdout` への直接書き込みも含めて棚卸し:

```bash
grep -rnE 'os\.Stdout|fmt\.Print(ln|f)?\(' internal/ cmd/ | grep -v _test.go
```

- **CLI ハンドラ群** (`internal/cli/*.go`): 多数ヒット（`rc.Renderer.Render(os.Stdout, ...)`, `fmt.Fprintln(os.Stdout, ...)` 等）。
  → これらは全て **CLI コマンド経由でのみ呼ばれる**。`McpStdioCmd.Run()` → `mcpinternal.NewServer` → `StdioServer.Listen` 経路からは到達不能。
- **MCP ツール経路** (`internal/mcp/`, `internal/backlog/`, `internal/digest/`, `internal/domain/`, `internal/render/`, `internal/auth/`, `internal/credentials/`, `internal/config/`): **0 件**（grep でヒットなし）。
- `cmd/logvalet/main.go:169` の `fmt.Println` は `--completion-bash` 専用ルートで、`mcp-stdio` サブコマンド経由では実行されない。

**結論: 既存ツール経路に stdout 汚染リスクなし**。

対策:
- 起動メッセージ・エラーログは `os.Stderr` のみに出す
- `mcpserver.NewStdioServer(s, mcpserver.WithErrorLogger(log.New(os.Stderr, "", log.LstdFlags)))` を明示指定
- 実装後の手動スモークテスト（後述）で stdout が JSON 行のみであることを検証
- 今後の tools_*.go 改修時に `fmt.Print*` を入れないルールを README に明記

## 変更対象ファイル

| ファイル | 変更種別 | 概要 |
|---------|---------|------|
| `internal/cli/mcp_stdio.go` | 新規 | `McpStdioCmd` + `Run()` (~50 行) |
| `internal/cli/mcp_stdio_test.go` | 新規 | Validate / 構造テスト (~30 行) |
| `internal/cli/root.go` | 編集 | フィールド 1 行追加 |
| `README.md` | 編集 | 使い方追加 |

`internal/mcp/server.go` の `NewServer` / `NewServerWithFactory` は変更不要（そのまま再利用）。

## 再利用する既存資産

- `buildRunContext(g)` — `/Users/youyo/src/github.com/youyo/logvalet/internal/cli/runner.go:26` — `RunContext{Client, Config, Renderer}` を返す
- `mcpinternal.NewServer(client, ver, cfg)` — `/Users/youyo/src/github.com/youyo/logvalet/internal/mcp/server.go:22` — 全ツール登録済み `*MCPServer` を返す
- `version.NewInfo().Version` — `/Users/youyo/src/github.com/youyo/logvalet/internal/version/` — GoReleaser ldflags 経由
- `mcpinternal.ServerConfig` — Profile / Space / BaseURL のみ設定（AuthorizationURL は空のまま）

## 実装方針 (TDD)

### Red: テスト先行

`internal/cli/mcp_stdio_test.go` で以下のテストを書く:

| テストID | 内容 | 期待動作 | 必須/推奨 |
|---------|------|---------|---------|
| T1 | `&McpStdioCmd{}` の Validate() | nil（無フラグ前提のため常に成功） | 必須 |
| T2 | コマンドが CLI 構造体に登録されている | `kong.New(&cli.CLI{})` が成功し、`mcp-stdio` コマンドがパースできる | 必須 |
| T3 | **stdout が JSON 行のみであることを統合テストで保証** | `mcpinternal.NewServer` をモッククライアントで構築し、`StdioServer.Listen` を pipe 経由で起動。`initialize` リクエストを書き込み、stdout の全行が `json.Valid([]byte(line))` を満たすことをアサート | **必須**（最重要回帰防止） |
| T4 | `Listen` 異常終了時にエラーを伝播する | `stdio.Listen` をラッパー関数で差し替え、強制エラーを返した場合に `Run()` が同エラーを return すること | 必須 |
| T5 | `ctx` キャンセル / `io.EOF` 時に nil を返す | ctx をキャンセル、または stdin パイプを閉じた場合に `Run()` が nil を返すこと | 必須 |

**T3 が最重要**: stdout 汚染（プラン最大リスク）の自動回帰防止。CI で常時実行することで、将来 `tools_*.go` 等に誤って `fmt.Print*` が混入しても検出できる。

**T4/T5 用の依存注入設計**: `Run()` の終了判定ロジックを直接テストするため、`stdio.Listen` 呼び出しを 1 段ラップする:

```go
// internal/cli/mcp_stdio.go
// listenFunc はテスト時に差し替え可能にするための関数値。
var listenFunc = func(s *mcpserver.StdioServer, ctx context.Context, stdin io.Reader, stdout io.Writer) error {
    return s.Listen(ctx, stdin, stdout)
}
```

テストで `listenFunc` を差し替えて `Listen` の戻り値を制御し、エラー伝播・正常終了ハンドリングを検証する。

### Green: 最小実装

`mark3labs/mcp-go` **v0.52.0** (go.mod で固定済み) の確定 API:

```go
// /Users/youyo/pkg/mod/github.com/mark3labs/mcp-go@v0.52.0/server/stdio.go
func NewStdioServer(server *MCPServer) *StdioServer                                // L395 (引数1つのみ)
func (s *StdioServer) SetErrorLogger(logger *log.Logger)                           // L410
func (s *StdioServer) Listen(ctx context.Context, stdin io.Reader, stdout io.Writer) error  // L521
func ServeStdio(server *MCPServer, opts ...StdioOption) error                      // L857 (高レベルAPI)
func WithErrorLogger(logger *log.Logger) StdioOption                               // L52 (ServeStdio 用)
```

**重要**: `NewStdioServer` は可変長 Option を受け付けない。Option は `ServeStdio` 専用。明示的に context 制御するなら `NewStdioServer(s)` + `SetErrorLogger(...)` の組み合わせを使う。

選択: SIGINT/SIGTERM ハンドリングのため `NewStdioServer` + `SetErrorLogger` + `Listen` の組み合わせ。

正常終了条件（`Listen` の戻り値判定）:
- `ctx.Err() != nil` (Canceled / DeadlineExceeded) → SIGINT/SIGTERM またはクライアント切断による正常終了
- `errors.Is(err, io.EOF)` → stdin EOF による正常終了（Claude Desktop 等が stdin を閉じた場合）
- それ以外のエラー → 異常終了として exit code 1 を返す

```go
// internal/cli/mcp_stdio.go (擬似コード)
package cli

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    mcpserver "github.com/mark3labs/mcp-go/server"
    mcpinternal "github.com/youyo/logvalet/internal/mcp"
    "github.com/youyo/logvalet/internal/version"
)

type McpStdioCmd struct{}

func (c *McpStdioCmd) Validate() error { return nil }

func (c *McpStdioCmd) Run(g *GlobalFlags) error {
    rc, err := buildRunContext(g)
    if err != nil {
        return err
    }
    ver := version.NewInfo().Version
    cfg := mcpinternal.ServerConfig{
        Profile: rc.Config.Profile,
        Space:   rc.Config.Space,
        BaseURL: rc.Config.BaseURL,
    }
    s := mcpinternal.NewServer(rc.Client, ver, cfg)

    stdio := mcpserver.NewStdioServer(s)
    stdio.SetErrorLogger(log.New(os.Stderr, "", log.LstdFlags))

    fmt.Fprintln(os.Stderr, "logvalet MCP server (stdio) ready")

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    err = stdio.Listen(ctx, os.Stdin, os.Stdout)
    if err == nil || ctx.Err() != nil || errors.Is(err, io.EOF) {
        return nil
    }
    return err
}
```

注:
- `NewStdioServer` は可変長 Option 非対応のため、`SetErrorLogger` メソッドで明示設定（デフォルトでも stderr 出力だが、契約として明示）
- `ctx.Err()` チェックで SIGINT/SIGTERM の正常終了を吸収
- `io.EOF` チェックで Claude Desktop 等が stdin を閉じた場合の正常終了を吸収

### root.go への追加 (1行)

```go
// internal/cli/root.go
type CLI struct {
    // ... 既存フィールド
    Mcp      McpCmd      `cmd:"" help:"start MCP server (Streamable HTTP)"`
    McpStdio McpStdioCmd `cmd:"" help:"start MCP server (stdio transport)"`
    // ...
}
```

### Refactor

- 起動ログメッセージのフォーマットを `mcp.go` (`listening on ...`) と一貫させる
- 共通定数があれば抽出（現時点では不要）

## アーキテクチャ整合性

- 命名: `McpCmd` → `McpStdioCmd`（PascalCase, CLI 上は `mcp-stdio`）。既存パターン (`SharedFileCmd`, `WatchingCmd` 等) と整合
- ファイル配置: `internal/cli/mcp_stdio.go` — `mcp.go`, `mcp_oauth.go`, `mcp_auth.go` と同じレイアウト
- エラーハンドリング: `buildRunContext` のエラーをそのまま return（既存パターン）
- Run シグネチャ: `(g *GlobalFlags) error` — 全コマンド共通

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| stdout に起動ログが混入し JSON-RPC が壊れる | **Critical** | 事前棚卸し済み（既存経路にリスクなし）。`WithErrorLogger(os.Stderr)` 明示。実装後の手動スモークテスト（後述）で stdout が JSON 行のみか検証 |
| `mark3labs/mcp-go` の Stdio API シグネチャが想定と異なる | ~~Medium~~ → 解消 | v0.52.0 の API を実装前に確定済み（プラン本文参照） |
| `buildRunContext` が認証情報を要求し、stdio モードで毎回失敗する | Medium | `mcp` (HTTP) サブコマンドと同じ流れなので、既存ユーザーが `configure` 済みなら動く。README に「事前に `logvalet configure` 実行」と明記 |
| Claude Desktop が SIGTERM ではなく stdin EOF で終了させる | Low | `Listen` が EOF で正常 return することを `mark3labs/mcp-go` のテストで確認済み。両方ハンドルされていれば問題なし |
| 既存 `mcp.go` の挙動を誤って変えてしまう | Low | `mcp.go` には一切手を入れず、新ファイルで完結させる方針を厳守 |
| 信頼できない MCP クライアントが接続し、トークンの権限で Backlog API を呼べてしまう | Medium | stdio は OS のプロセス境界が認可境界。README に「専用プロファイル / 最小権限トークンの使用」「信頼できる MCP クライアントのみ接続させる」旨を明記。HTTP モードと違いネットワーク露出はゼロなのでローカル外への攻撃面はない |

## 動作検証手順

1. ビルド: `go build -o logvalet ./cmd/logvalet/`
2. 単体テスト: `go test ./internal/cli/...`
3. 全体テスト: `go test ./...`
4. 型チェック: `go vet ./...`
5. 手動 stdio スモーク:
   ```bash
   # initialize リクエストを送って stdout に応答 JSON が返ることを確認
   printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' | ./logvalet mcp-stdio --profile <profile> | head -1
   ```
   stdout に純粋な JSON 行のみが出ること（起動ログが混ざらないこと）を確認。
6. Claude Desktop 統合確認（任意）:
   ```json
   {
     "mcpServers": {
       "logvalet": {
         "command": "/absolute/path/to/logvalet",
         "args": ["mcp-stdio", "--profile", "default"]
       }
     }
   }
   ```

## ドキュメント更新

- `README.md`:
  - 「MCP サーバーモード」セクションに stdio バリエーション追記（Claude Desktop 設定例含む）
  - **セキュリティ注意**を明記:
    > stdio モードでは MCP クライアントが選択されたプロファイルのトークンをそのまま使って Backlog API を呼び出します。次のいずれも守ってください:
    > - 専用プロファイルを作成し、必要最小限の権限を持つ Backlog API キーを設定する
    > - 信頼できる MCP クライアントのみ起動コマンドに登録する
    > - チーム共有マシンでは利用しない
  - **stdout 規約**を Contributor 向けに明記: 「`internal/mcp/` 配下のツールハンドラで `os.Stdout` / `fmt.Print*` を絶対に使わないこと（stdio モードで JSON-RPC が壊れる）」
- `docs/` 配下に該当ファイルがあれば更新（HTTP 専用記述があれば「stdio もサポート」と追記）
- CHANGELOG: 次回リリースエントリに `feat(mcp): stdio transport モード追加` を記載

## 推定実装時間

- 実装: 30 分（テスト含む）
- レビュー対応: 15 分
- 動作確認: 15 分
- ドキュメント: 10 分

合計: 約 70 分

## チェックリスト

- [x] 観点1: 実装実現可能性（変更ファイル明示、依存関係明示、ステップ具体的）
- [x] 観点2: TDDテスト設計（Validate / Kong パース確認）
- [x] 観点3: アーキテクチャ整合性（既存命名規則・ファイル配置と一致）
- [x] 観点4: リスク評価（stdout 混入 Critical を最重要に置く）
- [ ] 観点5: シーケンス図（薄い委譲層のため N/A — 1コマンド = 1関数呼び出し）
