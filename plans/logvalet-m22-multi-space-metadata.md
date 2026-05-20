# M22: multi-space metadata fix（AnalysisEnvelope.Space / BaseURL 修正）

> spec: `docs/specs/logvalet_multi_space_spec.md`
> roadmap: `plans/logvalet-multi-space-roadmap.md`
> 前提: M22 primary（`cli/mcp.go:252` で `SpaceClientFactory` を `auth.NewSpaceAwareClientFactory` に置換済み）

---

## 1. 背景と目的

### 現象
`logvalet_my_tasks spaces=["megumilog"]` を MCP 経由で呼び出すと:
- API ルーティング自体は megumilog に向く（M22 primary 修正済み）
- しかしレスポンス内側の `AnalysisEnvelope` メタデータが間違っている:
  - `result.space == "heptagon"`（期待: `"megumilog"`）
  - `result.base_url == "https://heptagon.backlog.com/"`（期待: `"https://megumilog.backlog.jp"`）

### 原因
`internal/mcp/tools_*.go` の各ビルダーが起動時設定値 `cfg.Space` / `cfg.BaseURL` を直接渡している:

```go
builder := analysis.NewMyTasksBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)
//                                              ^^^^^^^^^   ^^^^^^^^^^^
//                                              起動時の heptagon 固定値
```

`cfg.Space` / `cfg.BaseURL` は Lambda 起動時に環境変数（heptagon）から設定された `ServerConfig` 値であり、multi-space 呼び出し時に変わらない。
ビルダーは `BaseAnalysisBuilder.space` / `.baseURL` をそのまま `AnalysisEnvelope.Space` / `.BaseURL` に詰めるため、メタデータが不整合になる（`internal/analysis/analysis.go:69-83`）。

### 目的
multi-space モード（`spaces=["megumilog"]` や `all_spaces=true`）で呼ばれたツールが、**実行対象 SpaceRegistration の Alias / BaseURL** を `AnalysisEnvelope` メタデータに反映するようにする。

### 非目的（スコープ外）
- ビルダー側 API の変更（`NewMyTasksBuilder` 等のシグネチャは触らない）
- CLI 側の修正（CLI は `rc.Config.Space` / `rc.Config.BaseURL` 経由でビルダーを呼ぶが、multi-space fan-out は CLI 側で M22 primary までで既に space ごとに client 切替済み。本 M22 は MCP 側固有の問題）
- DigestEnvelope の他フィールドの再検討

---

## 2. 修正方針

context-aware パターン（既存 `internal/auth/context.go` と同形）で `SpaceRegistration` を context に伝搬し、各ビルダー call site で context から取り出して使う。

### 設計上の選択肢と決定

| 選択肢 | 評価 |
|--------|------|
| A: context key で SpaceRegistration を渡す（採用） | 既存 `auth.ContextWithUserID` と同じパターン。ビルダー API 不変。call site は 15 箇所だが mechanical 置換。 |
| B: 各ビルダーの signature を変更し SpaceRegistration を引数化 | 影響範囲が広い（analysis/digest パッケージ全体）。タスクスコープを越える。 |
| C: ServerConfig に mutable な「現在のスペース」を持たせる | 並行リクエストで race condition。却下。 |

**採用: A（context key パターン）**

### キー設計

```go
// internal/mcp/tools.go
type spaceRegCtxKey struct{}

// ContextWithSpace は context に SpaceRegistration を設定して返す。
func contextWithSpace(ctx context.Context, reg space.SpaceRegistration) context.Context {
    return context.WithValue(ctx, spaceRegCtxKey{}, reg)
}

// spaceInfoFromContext は context から (alias, baseURL) を取得する。
// 未設定の場合は fallback 値を返す。
func spaceInfoFromContext(ctx context.Context, fallbackSpace, fallbackBaseURL string) (string, string) {
    reg, ok := ctx.Value(spaceRegCtxKey{}).(space.SpaceRegistration)
    if !ok {
        return fallbackSpace, fallbackBaseURL
    }
    return reg.Alias, reg.BaseURL
}
```

### context 伝搬の 2 つの注入ポイント（重要）

`RegisterWithSpaces` には 2 つのパスがあり、**両方で** context を注入する必要がある:

1. **単一スペースパス**（`callWithSpaceClient`）— preference fallback / 単一 spaces 指定時
2. **fan-out パス**（`ExecuteAcrossSpaces` 内のクロージャ）— spaces=[...] 複数 or all_spaces=true 時

#### 注入箇所 1: `callWithSpaceClient`
```go
// internal/mcp/tools.go:252 付近
func (r *ToolRegistry) callWithSpaceClient(ctx context.Context, fn ToolFunc, args map[string]any, reg space.SpaceRegistration) (*gomcp.CallToolResult, error) {
    client, err := r.spaceFactory(ctx, reg)
    if err != nil { /* 既存 */ }
    ctx = contextWithSpace(ctx, reg)  // ★ 追加
    result, err := fn(ctx, client, args)
    // ...
}
```

#### 注入箇所 2: fan-out クロージャ（`RegisterWithSpaces` 内）
```go
// internal/mcp/tools.go:154-158 付近
results := space.ExecuteAcrossSpaces[any](ctx, executor, targets,
    func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (any, error) {
        ctx = contextWithSpace(ctx, reg)  // ★ 追加
        return fn(ctx, client, args)
    },
)
```

> **advisor 指摘事項**: 注入箇所 1 だけだと `spaces=["foo","bar"]` の典型的 multi-space ケースで依然メタデータが間違う。両方の注入が load-bearing。

`RegisterWithSpacesWrite` は単一スペース呼び出しのみで、最終的に `callWithSpaceClient` を通るため注入箇所 1 のカバレッジで十分。

### ビルダー call site の置換パターン

#### Before
```go
builder := analysis.NewMyTasksBuilder(client, cfg.Profile, cfg.Space, cfg.BaseURL)
return builder.Build(ctx, opts)
```

#### After
```go
spaceAlias, spaceBaseURL := spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)
builder := analysis.NewMyTasksBuilder(client, cfg.Profile, spaceAlias, spaceBaseURL)
return builder.Build(ctx, opts)
```

fallback 動作:
- multi-space context あり → SpaceRegistration の Alias / BaseURL を使う
- multi-space context なし（=従来の単一スペース運用） → `cfg.Space` / `cfg.BaseURL` をそのまま使う（後方互換）

---

## 3. 影響範囲（全 15 call site）

### 修正対象ファイル

| ファイル | 行 | ビルダー | 修正後の Builder 引数 |
|---------|----|--------|---------------------|
| `internal/mcp/tools.go` | (新規追加) | — | `spaceRegCtxKey{}` + `contextWithSpace` + `spaceInfoFromContext` |
| `internal/mcp/tools.go` | ~252 | — | `callWithSpaceClient` 内で context 注入 |
| `internal/mcp/tools.go` | ~154 | — | fan-out クロージャ内で context 注入 |
| `internal/mcp/tools_analysis.go` | 43 | `analysis.NewIssueContextBuilder` | (client, cfg.Profile, **spaceAlias**, **spaceBaseURL**) |
| `internal/mcp/tools_analysis.go` | 83 | `analysis.NewBlockerDetector` | 同上 |
| `internal/mcp/tools_analysis.go` | 117 | `analysis.NewStaleIssueDetector` | 同上 |
| `internal/mcp/tools_analysis.go` | 174 | `analysis.NewProjectHealthBuilder` | 同上 |
| `internal/mcp/tools_analysis.go` | 192 | `analysis.NewTriageMaterialsBuilder` | 同上 |
| `internal/mcp/tools_analysis.go` | 230 | `analysis.NewPeriodicDigestBuilder` (weekly) | 同上 |
| `internal/mcp/tools_analysis.go` | 268 | `analysis.NewPeriodicDigestBuilder` (daily) | 同上 |
| `internal/mcp/tools_analysis.go` | 334 | `analysis.NewActivityStatsBuilder` | 同上 |
| `internal/mcp/tools_analysis.go` | 398 | `analysis.NewCommentTimelineBuilder` | 同上 |
| `internal/mcp/tools_analysis.go` | 430 | `analysis.NewWorkloadCalculator` | 同上 |
| `internal/mcp/tools_analysis.go` | 452 | `analysis.NewMyTasksBuilder` | 同上 |
| `internal/mcp/tools_analysis.go` | 525 | `digest.NewUnifiedDigestBuilder` | 同上 |
| `internal/mcp/tools_space.go` | 26 | `digest.NewDefaultSpaceDigestBuilder` | 同上 |
| `internal/mcp/tools_activity.go` | 119 | `digest.NewDefaultActivityDigestBuilder` | 同上 |
| `internal/mcp/tools_document.go` | 110 | `digest.NewDefaultDocumentDigestBuilder` | 同上 |

合計: tools.go の context 基盤 1 セット + Builder call site 15 箇所。

### 非影響範囲（変更しない）

- `internal/analysis/*.go`（ビルダー実装本体）
- `internal/digest/*.go`（同上）
- `internal/cli/*.go`（CLI 側は別経路で SpaceRegistration を扱うが本タスクの範囲外）
- `internal/space/*.go`
- `internal/auth/*.go`
- write 系ツール（`tools_issue.go`, `tools_document.go` の write 部分等）— ビルダーの `Space`/`BaseURL` をレスポンスに含めない

---

## 4. TDD 設計（Red → Green → Refactor）

### フェーズ 1: 基盤ヘルパーの TDD

#### T1-R: `spaceInfoFromContext` の fallback テスト（Red）

ファイル: `internal/mcp/tools_test.go` に追加

```go
func TestSpaceInfoFromContext_NoSpaceInContext_UsesFallback(t *testing.T) {
    ctx := context.Background()
    alias, baseURL := spaceInfoFromContext(ctx, "fallback-space", "https://fallback.example.com")
    if alias != "fallback-space" || baseURL != "https://fallback.example.com" {
        t.Errorf("expected fallback values, got alias=%q baseURL=%q", alias, baseURL)
    }
}

func TestSpaceInfoFromContext_WithSpaceInContext_ReturnsRegistrationValues(t *testing.T) {
    reg := space.SpaceRegistration{Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp"}
    ctx := contextWithSpace(context.Background(), reg)
    alias, baseURL := spaceInfoFromContext(ctx, "ignored", "https://ignored")
    if alias != "megumilog" || baseURL != "https://megumilog.backlog.jp" {
        t.Errorf("expected reg values, got alias=%q baseURL=%q", alias, baseURL)
    }
}
```

> パッケージは `mcp` 内部テスト（`package mcp`）。`internal/mcp/tools_test.go` が `package mcp_test` の場合は同パッケージの新規ファイル `internal/mcp/context_space_test.go` を作成し `package mcp` で書く。

#### T1-R 追加: Alias / BaseURL 空時の fallback テスト（R4）
T1-c も同ファイルに追加（§10 R4 のコード参照）。

#### T1-G: 実装（Green）
`internal/mcp/tools.go` に `spaceRegCtxKey`, `contextWithSpace`, `spaceInfoFromContext` を追加。
`spaceInfoFromContext` には Alias / BaseURL 空チェックを含める（R4）。

#### T1-Refactor
- 型は unexported（`spaceRegCtxKey{}`）で外部衝突を防ぐ
- godoc コメントを追加（既存 `auth.UserIDFromContext` と同等のトーン）

---

### フェーズ 2: 単一スペースパスの context 伝搬テスト

#### T2-R: `callWithSpaceClient` 経由で context にスペースが注入されることを確認

ファイル: 新規 `internal/mcp/tools_m22_test.go`（`package mcp_test`）

```go
// T2-1: single space (preference fallback) でビルダーが reg.Alias を使う
func TestRegisterWithSpaces_SingleSpace_BuilderSeesRegistrationMetadata(t *testing.T) {
    store := space.NewMemoryStore()
    ctx := context.Background()
    _ = store.Upsert(ctx, &space.SpaceRegistration{
        UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp",
        Status: space.SpaceStatusOK,
    })

    spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
        return backlog.NewMockClient(), nil
    }

    s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
    resolver := space.NewResolver(store)
    reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

    var seenAlias, seenBaseURL string
    tool := gomcp.NewTool("t", gomcp.WithDescription("test"))
    reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
        // ビルダー呼び出しと同等の取得をハンドラから実施
        a, b := mcpinternal.SpaceInfoFromContextForTest(ctx, "fallback", "https://fallback")
        seenAlias, seenBaseURL = a, b
        return map[string]any{"ok": true}, nil
    })

    userCtx := auth.ContextWithUserID(ctx, "u1")
    _ = callToolWithCtx(t, s, userCtx, "t", map[string]any{})
    if seenAlias != "megumilog" || seenBaseURL != "https://megumilog.backlog.jp" {
        t.Errorf("expected megumilog reg in context, got alias=%q baseURL=%q", seenAlias, seenBaseURL)
    }
}
```

> `SpaceInfoFromContextForTest` は test-only export（`internal/mcp/export_test.go` で `var SpaceInfoFromContextForTest = spaceInfoFromContext` を追加）。

#### T2-G: 実装
`callWithSpaceClient` 内で `ctx = contextWithSpace(ctx, reg)` を追加し fn を呼ぶ。

---

### フェーズ 3: fan-out パスの context 伝搬テスト（load-bearing）

#### T3-R: 複数スペース指定時にビルダーが各スペースのメタデータを使う

```go
// T3-1: spaces=["megumilog","heptagon"] 各 Result 内で異なる metadata
func TestRegisterWithSpaces_FanOut_EachClosureSeesItsOwnRegistration(t *testing.T) {
    store := space.NewMemoryStore()
    ctx := context.Background()
    _ = store.Upsert(ctx, &space.SpaceRegistration{
        UserID: "u1", Alias: "heptagon", BaseURL: "https://heptagon.backlog.com",
        Status: space.SpaceStatusOK,
    })
    _ = store.Upsert(ctx, &space.SpaceRegistration{
        UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp",
        Status: space.SpaceStatusOK,
    })

    spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
        return backlog.NewMockClient(), nil
    }
    s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
    resolver := space.NewResolver(store)
    reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

    tool := gomcp.NewTool("fanout_tool", gomcp.WithDescription("fanout"))
    reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
        alias, baseURL := mcpinternal.SpaceInfoFromContextForTest(ctx, "fallback", "https://fallback")
        return map[string]any{"alias": alias, "base_url": baseURL}, nil
    })

    userCtx := auth.ContextWithUserID(ctx, "u1")
    result := callToolWithCtx(t, s, userCtx, "fanout_tool", map[string]any{
        "spaces": []any{"megumilog", "heptagon"},
    })
    if result.IsError {
        t.Fatalf("unexpected error: %v", result.Content)
    }

    // []Result[any] が返る; 各 Value.alias / base_url を検査
    raw := decodeTextJSONArray(t, result)
    if len(raw) != 2 {
        t.Fatalf("expected 2 results, got %d", len(raw))
    }
    // 入力順保持: megumilog, heptagon
    assertResultMetadata(t, raw[0], "megumilog", "https://megumilog.backlog.jp")
    assertResultMetadata(t, raw[1], "heptagon", "https://heptagon.backlog.com")
}
```

> このテストが本タスクで最も load-bearing。元のバグ（fan-out で全 result の metadata が heptagon になる）を直接検出する。

#### T3-R 追加: all_spaces=true パス（R3）
T3-2 として `all_spaces=true` でも同じ伝搬が機能することを検証:

```go
func TestRegisterWithSpaces_AllSpaces_EachClosureSeesItsOwnRegistration(t *testing.T) {
    // setup: 2 spaces registered for u1
    // call: all_spaces=true (spaces 引数なし)
    // assert: 各 result[i].result.space / .base_url が登録 reg と一致
}
```

#### T3-G: 実装
`RegisterWithSpaces` 内 fan-out クロージャに `ctx = contextWithSpace(ctx, reg)` を追加。
（spaces=[...] と all_spaces=true は同じ fan-out 経路を通るため修正は 1 箇所）

#### T3-Refactor
ヘルパー `decodeTextJSONArray`, `assertResultInnerMetadata`, `assertResultOuterMetadata` を
`internal/mcp/external_helpers_test.go`（新規, `package mcp_test`）に配置し共通化（R2）。
既存 `internal/mcp/helpers_test.go`（`package mcp`）には追加しない。

---

### フェーズ 4: end-to-end メタデータテスト（実 builder 経由）

#### T4: `MyTasksBuilder` を実際に呼んで AnalysisEnvelope.Space を検証

このテストは「ハンドラ内で `spaceInfoFromContext` を呼ぶ→ビルダーに渡す→envelope 出力」までを通して検証する。
モックの `backlog.Client` を使い、最小限の `MyTasksOptions` で `Build` を実行。

```go
func TestMyTasksHandler_FanOut_EnvelopeMetadataReflectsSpace(t *testing.T) {
    // setup 2 spaces, mock client, register全ツール
    cfg := mcpinternal.ServerConfig{
        Profile: "default", Space: "heptagon-fallback", BaseURL: "https://heptagon.example",
        SpaceResolver: resolver, SpaceClientFactory: spaceFactory,
    }
    // ... RegisterAnalysisTools(reg, cfg) で実 builder 経由ツールを登録 ...

    // spaces=["megumilog"] で呼び出し
    // result.Value.space == "megumilog"
    // result.Value.base_url == "https://megumilog.backlog.jp"
}
```

これが**完了条件 2 番目**「`logvalet_my_tasks spaces=["megumilog"]` で `result.space == "megumilog"`」を自動化したもの。

#### T4-2: `ProjectHealthBuilder` の sub-builder 連鎖を pin-down する e2e テスト

ファイル: `internal/mcp/tools_analysis_m22_test.go` または `tools_health_m22_test.go`（新規, `package mcp_test`）

`ProjectHealthBuilder`（`internal/analysis/health.go:79-81`）は内部で `StaleIssueDetector` / `BlockerDetector` / `WorkloadCalculator` を再生成する sub-builder 連鎖を持つ。
A 案の context-aware パターンで call site `tools_analysis.go:174` が修正されると、親 builder の `b.space` / `b.baseURL` に multi-space の値が入り、sub-detector にも正しく伝搬する。
この連鎖が壊れないことを pin-down する:

```go
func TestProjectHealthHandler_MultiSpace_EnvelopeAndSubBuildersMetadata(t *testing.T) {
    // setup: megumilog reg を登録
    // call: spaces=["megumilog"] で logvalet_project_health
    // assert:
    //   - 親 envelope の result.space == "megumilog"
    //   - 親 envelope の result.base_url == "https://megumilog.backlog.jp"
    //   - （可能なら）内部の stale/blocker/workload セクションでも同じ space metadata が反映されること
}
```

> **目的**: A 案で `ProjectHealthBuilder` の sub-builder 連鎖が正しく動くことを e2e で固定し、将来 sub-detector の初期化順序や引数受け渡しが変更された際の回帰を検出する。

---

### フェーズ 5: 後方互換テスト

#### T5: 旧パス（resolver nil / spaces 未指定 / context にスペースなし）で fallback が効くこと

```go
func TestRegister_NoMultiSpace_FallsBackToCfgSpace(t *testing.T) {
    s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
    reg := mcpinternal.NewToolRegistry(s, backlog.NewMockClient(), "")  // resolver なし
    tool := gomcp.NewTool("legacy", gomcp.WithDescription("legacy"))
    reg.RegisterWithSpaces(tool, func(ctx context.Context, _ backlog.Client, _ map[string]any) (any, error) {
        alias, baseURL := mcpinternal.SpaceInfoFromContextForTest(ctx, "cfg-space", "https://cfg.example")
        return map[string]any{"alias": alias, "base_url": baseURL}, nil
    })
    result := callToolWithCtx(t, s, context.Background(), "legacy", map[string]any{})
    out := decodeTextJSON(t, result)
    if out["alias"] != "cfg-space" || out["base_url"] != "https://cfg.example" {
        t.Errorf("expected fallback to cfg values, got %v", out)
    }
}
```

---

## 5. 実装手順（mechanical step-by-step）

1. **新規テストファイル作成**
   - `internal/mcp/context_space_test.go`: `spaceRegCtxKey` の単体テスト（T1）
   - `internal/mcp/tools_m22_test.go`: T2/T3/T5（必要なら）
   - `internal/mcp/export_test.go`: test-only export 追加
     ```go
     package mcp
     // export_test.go: test 用に unexported ヘルパーをパッケージ外テストへ公開する
     var SpaceInfoFromContextForTest = spaceInfoFromContext
     var ContextWithSpaceForTest = contextWithSpace
     ```

2. **T1: ヘルパー実装** (`internal/mcp/tools.go`)
   - `spaceRegCtxKey struct{}`
   - `contextWithSpace(ctx, reg) context.Context`
   - `spaceInfoFromContext(ctx, fb1, fb2) (string, string)`
   - `go test ./internal/mcp/ -run TestSpaceInfoFromContext` → Green

3. **T2/T3: context 注入実装** (`internal/mcp/tools.go`)
   - `RegisterWithSpaces` の fan-out クロージャ内に `ctx = contextWithSpace(ctx, reg)` を追加
   - `callWithSpaceClient` 内に `ctx = contextWithSpace(ctx, reg)` を追加
   - `go test ./internal/mcp/ -run TestRegisterWithSpaces_SingleSpace_BuilderSees` → Green
   - `go test ./internal/mcp/ -run TestRegisterWithSpaces_FanOut_EachClosure` → Green

4. **call site 15 箇所の置換**（mechanical）
   各 call site を以下の形に置換:
   ```go
   spaceAlias, spaceBaseURL := spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)
   builder := <NewBuilder>(client, cfg.Profile, spaceAlias, spaceBaseURL)
   ```
   - `internal/mcp/tools_analysis.go`: 12 箇所
   - `internal/mcp/tools_space.go`: 1 箇所
   - `internal/mcp/tools_activity.go`: 1 箇所
   - `internal/mcp/tools_document.go`: 1 箇所

5. **T4: end-to-end envelope テスト追加**
   - `internal/mcp/tools_m22_test.go` に追加
   - 全 builder 系のうち代表的に `MyTasksBuilder` を実 client モック経由で実行
   - `go test ./internal/mcp/` 全 Green

6. **T5: 後方互換テスト追加・確認**

7. **全テスト実行・vet**
   - `go test -race ./...`（fan-out 並行レース検出のため `-race` フラグを付与）
   - `go vet ./...`

8. **手動 sanity check（オプション）**
   - ローカル MCP 起動 + 実 Backlog 接続が可能なら `logvalet_my_tasks spaces=["megumilog"]` の応答で `result.space` を確認
   - 不可なら省略（テストでカバー済み）

---

## 6. 回帰防止テスト一覧（最終）

| ID | テスト | 検出する回帰 |
|----|------|------------|
| T1-a | `spaceInfoFromContext` fallback | context 未設定時に panic / nil 返さない |
| T1-b | `spaceInfoFromContext` 値取得 | reg.Alias / reg.BaseURL が正しく取れる |
| T1-c | Alias/BaseURL 空 → fallback | 壊れた reg で空文字 metadata 返す回帰 (R4) |
| T2-1 | 単一スペースパスで context 注入 | `callWithSpaceClient` の伝搬漏れ |
| T3-1 | fan-out (spaces=[...]) 各クロージャで独立 context | 全 result が同じ inner metadata になる回帰 |
| T3-2 | fan-out (all_spaces=true) 各クロージャで独立 context | all_spaces 経路の伝搬漏れ (R3) |
| T4-1 | 実 builder 経由 inner envelope メタデータ | call site の置換漏れ / fallback 残存 |
| T5-1 | resolver=nil でも cfg.Space で動く | 後方互換性の破壊 |

**重要**: T3-1 / T3-2 / T4-1 では **outer wrapper（`result[i].space`）と inner envelope（`result[i].result.space`）の両方** を検証する (R1)。
本 M22 のバグは inner envelope 側だが、outer 側が後の修正で壊れる回帰も同時に防ぐ。

---

## 7. リスクと緩和

| リスク | 緩和策 |
|--------|--------|
| call site 15 箇所の置換漏れ | T4 e2e テストが実 builder 経由で envelope を検証し検出。さらに `grep -n "cfg.Space" internal/mcp/` を実装完了後に再実行し残存ゼロを確認。 |
| fan-out クロージャ内の context 伝搬を忘れる | T3-1 が直接ターゲット。advisor が指摘した最大のリスク。 |
| ビルダー内部で `b.space`/`b.baseURL` を非エンベロープ用途にも使っている可能性 | analysis/digest パッケージは render（envelope 出力）専用に保持しており、API 呼び出しに使っていないことを確認済み（`grep -rn "b.space\|b.baseURL" internal/analysis/ internal/digest/`）。回帰なし。 |
| context 値の型衝突 | unexported struct key（`spaceRegCtxKey{}`）で防止。既存 `auth.contextKey{}` と同パターン。 |
| Lambda 環境での race condition | context は immutable; 各リクエストごとに `context.WithValue` で派生 → 並行安全。 |
| CLI 側で同問題があるのでは | CLI は CLI 側で `rc.Config.Space` / `rc.Config.BaseURL` を使うが、CLI の multi-space fan-out は別経路（ロードマップ MS13 範囲、現状未実装）。本タスクの対象外と明示。 |

---

## 8. 完了条件

1. `go test ./...` 全 Green
2. `go vet ./...` 警告なし
3. `grep -n "cfg.Space\|cfg.BaseURL" internal/mcp/tools_*.go` の結果が `spaceInfoFromContext(ctx, cfg.Space, cfg.BaseURL)` の fallback 引数文脈のみで出現する状態。`cfg.SpaceResolver` / `cfg.SpaceClientFactory` / `cfg.SpaceStore` 等の false positive は許容（単語境界で識別）
4. 新規 T3-1 fan-out テストで `result[0].alias == "megumilog"`, `result[1].alias == "heptagon"` を確認
5. （理想）実 MCP 経由で `logvalet_my_tasks spaces=["megumilog"]` を呼び出すと `result.space == "megumilog"`, `result.base_url == "https://megumilog.backlog.jp"` が返る

---

## 9. 工数見積もり

- ヘルパー実装 + 単体テスト: 30 分
- `callWithSpaceClient` / fan-out クロージャの context 注入 + テスト: 30 分
- 15 call site の mechanical 置換: 30 分
- e2e + 後方互換テスト: 45 分
- `go test ./...` Green 化 + 既存テストの調整: 30 分
- 合計: 約 2.5 〜 3 時間

---

## 10. レビュー指摘事項の反映状況

### advisor からの指摘（反映済み）

1. **fan-out クロージャの context 注入忘れ** — §2 注入箇所 2 / §4 T3-1 で対応 (load-bearing)
2. **call site は 15 箇所（13 ではなく）** — §3 表で完全列挙
3. **fan-out e2e テストの追加** — §4 T3-1 / T4 として組み込み
4. **CLI 側も grep して影響範囲を確認** — §3 / §7 で「CLI 側は本タスクの対象外」と明示

### copilot CLI plan-review からの指摘（反映済み）

#### R1 [high / テスト]: outer wrapper と inner envelope の混同に注意
`RegisterWithSpaces` の `spaces=[...]` / `all_spaces=true` は `[]space.Result[T]` を返し、
各要素は **outer wrapper**（`Result.Space`, `Result.BaseURL` 等のメタ）と **inner Value**（`AnalysisEnvelope`）に分かれる。

→ T3-1 / T4 のアサーション仕様を以下に明確化:

```text
T3-1 / T4 の検証対象:
  各 result[i] について:
    - outer: result[i].space / result[i].base_url   ← Executor が ExecuteAcrossSpaces で詰める層
    - inner: result[i].result.space / result[i].result.base_url  ← AnalysisEnvelope.Space/BaseURL（本タスク対象）

本 M22 のバグは inner envelope の metadata。テストは inner も outer も両方検証する。
```

Executor.Result 構造体（`internal/space/executor.go`）と AnalysisEnvelope（`internal/analysis/analysis.go`）の JSON 形を実装前に再確認すること。

#### R2 [medium / 実現可能性]: テストパッケージ配置の整理
- 既存の multi-space 統合テストは `package mcp_test`
- `helpers_test.go` は `package mcp`

→ 配置方針を確定:

```text
1. 外部テスト（mcp_test）から使う JSON デコード helper:
   → 新規 internal/mcp/external_helpers_test.go (package mcp_test) に配置
   既存 helpers_test.go（package mcp）には追記しない

2. unexported ヘルパーの test export:
   → internal/mcp/export_test.go (package mcp) に var として再公開
     例: var SpaceInfoFromContextForTest = spaceInfoFromContext

3. 単体テスト（T1）は package mcp 側で書く
   → internal/mcp/context_space_test.go (package mcp)

4. 統合テスト（T2/T3/T4/T5）は package mcp_test 側
   → internal/mcp/tools_m22_test.go (package mcp_test)
```

#### R3 [medium / 完全性]: all_spaces=true パスの個別テスト追加
`spaces=[...]` と `all_spaces=true` は resolver の挙動が異なる（`Scope.Aliases` vs `Scope.AllSpaces`）。
fan-out 経路は同じだが、要件カバレッジとして個別テストを 1 本追加する:

```go
// T3-2: all_spaces=true でも inner envelope の metadata が各 space を反映
func TestRegisterWithSpaces_AllSpaces_EachClosureSeesItsOwnRegistration(t *testing.T) {
    // setup: 2 spaces registered for u1
    // call: all_spaces=true
    // assert: 各 result[i].result の inner alias/base_url が一致
}
```

#### R4 [low / リスク]: SpaceRegistration の Alias / BaseURL 空チェック
`spaceInfoFromContext` を「context に reg があれば即採用」にすると、
壊れた reg（Alias="" / BaseURL=""）が入ると空文字 metadata を返す。

→ 仕様明記 + ガード追加:

```go
func spaceInfoFromContext(ctx context.Context, fbSpace, fbBaseURL string) (string, string) {
    reg, ok := ctx.Value(spaceRegCtxKey{}).(space.SpaceRegistration)
    if !ok {
        return fbSpace, fbBaseURL
    }
    // 不変条件: 通常 store layer で非空が保証されるが、壊れたデータでの no-op を防ぐ
    if reg.Alias == "" || reg.BaseURL == "" {
        return fbSpace, fbBaseURL
    }
    return reg.Alias, reg.BaseURL
}
```

テスト追加:
```go
// T1-c: Alias/BaseURL のいずれかが空なら fallback を使う
func TestSpaceInfoFromContext_EmptyRegistration_UsesFallback(t *testing.T) {
    cases := []space.SpaceRegistration{
        {Alias: "", BaseURL: "https://x"},
        {Alias: "x", BaseURL: ""},
    }
    for _, r := range cases {
        ctx := contextWithSpace(context.Background(), r)
        a, b := spaceInfoFromContext(ctx, "fb-s", "https://fb")
        if a != "fb-s" || b != "https://fb" {
            t.Errorf("expected fallback for %+v", r)
        }
    }
}
```

§4 フェーズ 1 と §6 回帰防止テスト一覧に追記。

### devils-advocate / advocate 評価結果

将来の review で同じ議論を繰り返さないよう、各指摘の採否と判断根拠を記録する。

#### 却下された指摘

- **devils-advocate 指摘 #1（B' 案未検討: `newEnvelope` シグネチャ変更 / builder に ctx 保持）**
  → advocate 判定: **却下**
  - `newEnvelope` シグネチャ変更は内部全 call site の修正が必要で、A 案より破壊的
  - builder に `ctx` を保持するのは Go の慣習違反（`context.Context` を struct フィールドに保持しない原則）
  - ドメイン層（`internal/analysis` / `internal/digest`）が MCP transport の context key を知ることになり、レイヤリング違反
- **devils-advocate 指摘 #3（sub-builder 連鎖問題: `ProjectHealthBuilder` で stale/blocker/workload detector を再生成する経路で metadata が漏れる）**
  → advocate 判定: **却下（事実誤認）**
  - call site `tools_analysis.go:174` が修正されれば、親 `ProjectHealthBuilder` の `b.space` / `b.baseURL` に正しい multi-space 値が入る
  - `health.go:79-81` の sub-detector 再生成は親の `b.space` / `b.baseURL` を引数渡ししているため、伝搬経路は正常
  - ただし将来の回帰防止として T4-2 を追加（指摘 #5 として採用）
- **devils-advocate 指摘 #4（CLI 非対称性: MCP 側のみ対応で CLI 側に同問題が残る）**
  → advocate 判定: **却下**
  - CLI の fan-out は `runFanout` クロージャ内で `reg` が直接見えるため、CLI 側は適材適所の別パターンで対応可能
  - 本 M22 は MCP 側固有の問題（`AnalysisEnvelope` metadata 不整合）に閉じたスコープ
  - CLI 側の multi-space 対応はロードマップ MS13 範囲（M22 スコープ外と §3 / §7 で明示済み）

#### 採用された指摘

- **指摘 #5（`ProjectHealthBuilder` の sub-builder 連鎖 pin-down テスト）**
  → §4 フェーズ 4 に **T4-2** として追加（sub-detector 連鎖の e2e 検証）
- **指摘 #6（`-race` フラグ）**
  → §5 step 7 に **`go test -race ./...`** として反映（fan-out 並行レース検出）
- **指摘 #2（完了条件 #3 の文言精緻化）**
  → §8 完了条件 #3 を **`spaceInfoFromContext` の fallback 引数文脈のみ許容 / `cfg.SpaceResolver` 等の false positive は単語境界で識別** と精緻化

---

*Created: 2026-05-21 by architect agent*
*Reviewed by: advisor + copilot CLI (gpt-5.4 high effort) + devils-advocate / advocate (A 案採用判定)*
*前提リファレンス: cli/mcp.go:252（M22 primary 修正）, internal/auth/context.go（context key パターン参考）, internal/analysis/analysis.go:69-83（AnalysisEnvelope）, internal/space/executor.go（Result 型）*
