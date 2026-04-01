# M21: IssueContext CLI コマンド

Plan: plans/logvalet-m21-issue-context-cli.md
Roadmap: plans/logvalet-roadmap-v3.md
依存: M20（analysis 基盤 + IssueContextBuilder）

## 目的

M20 で実装した `IssueContextBuilder` を CLI コマンドとして公開する。
`logvalet issue context PROJ-123` で issue の総合コンテキストを取得可能にする。

## 完了条件

- [ ] `internal/cli/issue.go` に `Context IssueContextCmd` フィールドを追加
- [ ] `internal/cli/issue_context.go` を新規作成（IssueContextCmd + Run()）
- [ ] `internal/cli/issue_context_test.go` を新規作成（CLI 統合テスト）
- [ ] `logvalet issue context PROJ-123` が正常動作
- [ ] `--comments` フラグで MaxComments を制御可能
- [ ] `--compact` フラグで compact モードを切り替え可能
- [ ] `-f md` / `-f json` / `-f yaml` の出力フォーマット切り替え
- [ ] `go test ./internal/cli/...` パス
- [ ] `go test ./...` パス
- [ ] `go vet ./...` パス

---

## 1. コマンド定義

### IssueContextCmd

```go
// IssueContextCmd は issue context コマンド。
type IssueContextCmd struct {
    IssueIDOrKey string `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
    Comments     int    `help:"max recent comments to include" default:"10"`
    Compact      bool   `help:"omit description and comment bodies"`
}
```

### IssueCmd への追加

```go
type IssueCmd struct {
    Get        IssueGetCmd        `cmd:"" help:"get issue"`
    List       IssueListCmd       `cmd:"" help:"list issues"`
    Create     IssueCreateCmd     `cmd:"" help:"create issue"`
    Update     IssueUpdateCmd     `cmd:"" help:"update issue"`
    Comment    IssueCommentCmd    `cmd:"" help:"manage comments"`
    Attachment IssueAttachmentCmd `cmd:"" help:"manage attachments"`
    Context    IssueContextCmd    `cmd:"" help:"get issue context for AI analysis"`  // 追加
}
```

### Run() メソッドのパターン

既存の `IssueGetCmd.Run()` パターンに準拠:

```go
func (c *IssueContextCmd) Run(g *GlobalFlags) error {
    ctx := context.Background()
    rc, err := buildRunContext(g)
    if err != nil {
        return err
    }

    builder := analysis.NewIssueContextBuilder(
        rc.Client,
        rc.Config.Profile,
        rc.Config.Space,
        rc.Config.BaseURL,
    )

    envelope, err := builder.Build(ctx, c.IssueIDOrKey, analysis.IssueContextOptions{
        MaxComments: c.Comments,
        Compact:     c.Compact,
    })
    if err != nil {
        return err
    }

    return rc.Renderer.Render(os.Stdout, envelope)
}
```

---

## 2. TDD テストケース一覧（Red フェーズで先に書く）

### issue_context_test.go

CLI 層のテストは以下の観点に絞る:
- Kong タグの正しさ（Parse テスト）
- IssueContextCmd の構造体フィールドのデフォルト値
- `issue context` サブコマンドが Kong で認識されること

注意: `IssueContextBuilder` のロジックテスト（正常系/異常系/compact/maxComments/部分失敗）は
M20 の `internal/analysis/context_test.go` で十分にカバー済み。
CLI 層は薄いラッパーのため、重複テストは避ける。

```
T1: TestIssueContextCmd_KongParse
    Kong で "issue context PROJ-123" をパースし、
    IssueIDOrKey="PROJ-123", Comments=10(default), Compact=false(default) を確認。

T2: TestIssueContextCmd_KongParse_WithFlags
    Kong で "issue context PROJ-123 --comments 20 --compact" をパースし、
    IssueIDOrKey="PROJ-123", Comments=20, Compact=true を確認。

T3: TestIssueContextCmd_KongParse_MissingArg
    Kong で "issue context" (引数なし) をパースし、
    エラーが返されることを確認。

T4: TestIssueContextCmd_KongParse_Help
    Kong で "issue context --help" が panic せずに処理できることを確認。
```

---

## 3. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/cli/issue_context.go` | IssueContextCmd 構造体 + Run() メソッド |
| `internal/cli/issue_context_test.go` | T1-T7 テスト |

### 変更

| ファイル | 変更内容 |
|---------|---------|
| `internal/cli/issue.go` | IssueCmd に `Context IssueContextCmd` フィールド追加 |

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red — テストを先に書く

1. `internal/cli/issue_context_test.go` を作成（T1-T7）
2. `internal/cli/issue_context.go` の型定義のみ（空の Run() メソッド）
3. `go test ./internal/cli/...` → テスト失敗（Red）

### Step 2: Green — 最小限の実装

1. `internal/cli/issue_context.go` の Run() を実装
   - `buildRunContext(g)` で Client, Config, Renderer を取得
   - `analysis.NewIssueContextBuilder()` で builder を生成
   - `builder.Build()` で AnalysisEnvelope を取得
   - `rc.Renderer.Render()` で出力
2. `internal/cli/issue.go` の IssueCmd に `Context` フィールドを追加
3. `go test ./internal/cli/...` → 全テストパス（Green）

### Step 3: Refactor

1. コードの整理（不要なコメント削除）
2. `go test ./internal/cli/...` → 全テストパス
3. `go test ./...` → 全テストパス
4. `go vet ./...` → クリーン

---

## 5. テスト戦略

### テスト可能性の設計

`IssueContextCmd.Run()` は `buildRunContext()` に依存しており、
これは config.toml / tokens.json などの実ファイルに依存する。
そのため、CLI 統合テストでは以下のアプローチを取る:

1. **IssueContextBuilder のテストは M20 で完了済み** — analysis パッケージ内で十分にテスト済み
2. **CLI 層のテストは Kong タグの検証 + ヘルパー関数テスト** に絞る
3. **buildRunContext 依存の統合テストは E2E（M32）で実施**

### MockClient の活用

既存テスト（`issue_list_test.go`）のパターンに準拠:
- `backlog.NewMockClient()` で MockClient を生成
- 各 Func フィールドをセットしてテスト

---

## 6. リスク評価

| リスク | 影響度 | 対策 |
|--------|--------|------|
| `buildRunContext` のモック困難 | 中 | ロジックを analysis パッケージに委譲し、CLI 層は薄く保つ。テストは analysis パッケージで十分にカバー済み |
| Kong タグの構文エラー | 低 | `go build` で検出可能。root_test.go で Kong Parse テストがあれば検出 |
| Renderer が AnalysisEnvelope を処理できない | 低 | Renderer は `any` 型を受け取るため問題なし（JSON/YAML は Marshal で対応） |
| `extractProjectKey` の挙動差異 | 低 | analysis パッケージと cli パッケージで同名関数が存在するが、各パッケージ内で閉じている |

---

## 7. 使用例

```bash
# 基本使用
logvalet issue context PROJ-123

# コメント数を制限
logvalet issue context PROJ-123 --comments 20

# compact モード（description, comment bodies を省略）
logvalet issue context PROJ-123 --compact

# Markdown 出力
logvalet issue context PROJ-123 -f md

# YAML 出力
logvalet issue context PROJ-123 -f yaml

# JSON + pretty print
logvalet issue context PROJ-123 --pretty
```

---

## 8. 次のマイルストーン

M21 完了後 → M22（IssueContext MCP ツール）へ進む。
M22 では `internal/mcp/tools_analysis.go` に `logvalet_issue_context` MCP tool を追加する。
CLI と同じ `analysis.IssueContextBuilder` を使用するため、ロジックの二重実装は不要。
