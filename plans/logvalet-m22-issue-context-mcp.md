# M22: IssueContext MCP ツール

Plan: plans/logvalet-m22-issue-context-mcp.md
Roadmap: plans/logvalet-roadmap-v3.md

## 目的

`logvalet_issue_context` MCP tool を追加し、MCP 経由で IssueContextBuilder を利用可能にする。
CLI (M21) と同じ `analysis.IssueContextBuilder` を使用するため、ロジックの二重実装は不要。

## 完了条件

- [ ] `internal/mcp/tools_analysis.go` — RegisterAnalysisTools + logvalet_issue_context handler
- [ ] `internal/mcp/tools_analysis_test.go` — テスト全パス
- [ ] `internal/mcp/tools.go` — boolArg ヘルパー追加
- [ ] `internal/mcp/server.go` — NewServer シグネチャ拡張 + RegisterAnalysisTools 呼び出し
- [ ] `internal/cli/mcp.go` — NewServer 呼び出しを更新
- [ ] `go test ./...` パス
- [ ] `go vet ./...` パス

---

## 1. 設計決定

### 1.1 NewServer へ config 情報を渡す

現在の `NewServer(client, ver)` は profile/space/baseURL を持たない。
IssueContextBuilder は `NewIssueContextBuilder(client, profile, space, baseURL)` を必要とする。

**方針**: `NewServer` に `ServerConfig` 構造体を追加する。

```go
// ServerConfig は MCP サーバーの設定。
type ServerConfig struct {
    Profile string
    Space   string
    BaseURL string
}

func NewServer(client backlog.Client, ver string, cfg ServerConfig) *mcpserver.MCPServer
```

これにより RegisterAnalysisTools は `(reg *ToolRegistry, cfg ServerConfig)` を受け取る。
他の Register 関数は変更不要（config 不要のため）。

### 1.2 boolArg ヘルパー

`tools.go` に `boolArg` を追加。JSON の bool は Go の `bool` として渡される。

```go
func boolArg(args map[string]any, key string) (bool, bool) {
    v, ok := args[key]
    if !ok {
        return false, false
    }
    b, ok := v.(bool)
    return b, ok
}
```

### 1.3 MCP tool 定義

```
tool名: logvalet_issue_context
パラメータ:
  - issue_key: required string — Issue key (e.g. PROJ-123)
  - comments: optional number — Max recent comments (default 10)
  - compact: optional boolean — Omit description and comment bodies (default false)
```

handler は:
1. issue_key を取得・バリデーション
2. comments, compact をオプション取得
3. `analysis.NewIssueContextBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)` で builder 生成
4. `builder.Build(ctx, issueKey, opts)` を呼び出し
5. AnalysisEnvelope を返却（ToolRegistry.Register が JSON 化）

---

## 2. TDD テストケース一覧

### tools_analysis_test.go

```
T1: TestRegisterAnalysisTools_ToolRegistered
    - MockMCPServer + ToolRegistry で RegisterAnalysisTools を呼ぶ
    - logvalet_issue_context が登録されたことを確認

T2: TestIssueContextHandler_Success
    - MockClient に GetIssue, ListIssueComments, ListProjectStatuses をセット
    - handler を直接呼び出し（args: {issue_key: "PROJ-123"}）
    - 返り値が *AnalysisEnvelope であること
    - envelope.Resource == "issue_context"

T3: TestIssueContextHandler_MissingIssueKey
    - args: {} で handler 呼び出し
    - エラーが返ること

T4: TestIssueContextHandler_WithComments
    - args: {issue_key: "PROJ-123", comments: 5}
    - MaxComments=5 で Build が呼ばれること（結果の recent_comments 件数で検証）

T5: TestIssueContextHandler_WithCompact
    - args: {issue_key: "PROJ-123", compact: true}
    - Compact=true で Build が呼ばれること（description が空で検証）
```

### tools_test.go（boolArg 追加分）

```
T6: TestBoolArg_True
T7: TestBoolArg_False
T8: TestBoolArg_Missing
T9: TestBoolArg_WrongType
```

---

## 3. ファイル変更一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/mcp/tools_analysis.go` | RegisterAnalysisTools, logvalet_issue_context handler |
| `internal/mcp/tools_analysis_test.go` | T1-T5 |

### 変更

| ファイル | 変更内容 |
|---------|---------|
| `internal/mcp/tools.go` | boolArg ヘルパー追加 |
| `internal/mcp/server.go` | ServerConfig 型追加、NewServer シグネチャ変更、RegisterAnalysisTools 呼び出し |
| `internal/cli/mcp.go` | NewServer 呼び出しに ServerConfig を渡す |

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red — boolArg テスト

1. `internal/mcp/tools_test.go` に T6-T9 を追加
2. `go test ./internal/mcp/...` → コンパイルエラー（boolArg 未定義）

### Step 2: Green — boolArg 実装

1. `internal/mcp/tools.go` に boolArg を追加
2. `go test ./internal/mcp/...` → T6-T9 パス

### Step 3: Red — tools_analysis テスト

1. `internal/mcp/tools_analysis_test.go` を作成（T1-T5）
2. `go test ./internal/mcp/...` → コンパイルエラー

### Step 4: Green — tools_analysis + server 変更

1. `internal/mcp/server.go` に ServerConfig 追加、NewServer シグネチャ変更
2. `internal/mcp/tools_analysis.go` に RegisterAnalysisTools 実装
3. `internal/cli/mcp.go` の NewServer 呼び出し更新
4. `go test ./internal/mcp/...` → T1-T5 パス

### Step 5: 全テスト確認

1. `go test ./...` → 全パス
2. `go vet ./...` → クリーン

### Step 6: Refactor

1. コードの整理
2. 全テスト再確認

---

## 5. リスク評価

| リスク | 影響 | 対策 |
|--------|------|------|
| NewServer シグネチャ変更の既存テスト影響 | 中 | server_test.go が存在すれば呼び出しを更新 |
| boolArg の型アサーション漏れ | 低 | テストで bool, 非bool, 不在を網羅 |
| MCP テストで MockClient のセットアップ複雑 | 中 | 既存 tools_issue_test.go のパターンを踏襲 |

---

## 6. 検証コマンド

```bash
go test ./internal/mcp/... -v
go test ./...
go vet ./...
```

---

## 7. 次のマイルストーン

M22 完了後 → M23（StaleIssueDetector ロジック）へ進む。
M23 では `internal/analysis/stale.go` に StaleIssueDetector を実装する。

### M23 へのハンドオフ情報

- `RegisterAnalysisTools` に新しい analysis tool を追加するパターンが確立された
- `ServerConfig` で profile/space/baseURL を MCP handler に渡せる
- `boolArg` ヘルパーが利用可能
