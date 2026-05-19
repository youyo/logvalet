# MS14: MCP spaces/all_spaces 共通引数 + space 管理 tools

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS09, MS10（RC3 修正: MS10 も必要）

## 目的

MCP の read-only ツール（MVP 17件）に spaces/all_spaces 引数を追加し、
space 管理 tools（5件）を新規実装する。
`connect_url` tool は MultiSpaceOAuthHandler（MS10）に依存するため MS10 必須（RC3 修正）。

## 完了条件

- [ ] `internal/mcp/tools_space_registry.go` — 新規 space 管理 tools
- [ ] MVP 17ツールに spaces/all_spaces 引数追加
- [ ] `withSpaces` ラッパー関数実装（RH4 対応: 共通ミドルウェア）
- [ ] `logvalet_space_list` でユーザーの登録済みスペースが返る
- [ ] `logvalet_issue_list` に `spaces: ["foo", "bar"]` を渡すと横断結果が返る
- [ ] 他ユーザーのスペース情報が漏洩しない（user isolation テスト）
- [ ] `go test ./internal/mcp/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### tools_space_registry_test.go

```
T1: TestLogvaletSpaceList_ReturnsUserSpaces
    - userID = "u1"、SpaceStore に {u1/foo, u1/bar}
    - logvalet_space_list → {"spaces":[{alias:"foo",...},{alias:"bar",...}]}
    - userB の spaces は含まれない

T2: TestLogvaletSpaceUse_ChangesDefault
    - SpaceStore に {u1/foo}
    - logvalet_space_use {alias:"foo"} → DefaultSpaceAlias = "foo"

T3: TestLogvaletSpaceVerify_AllSpaces
    - {u1/foo, connected}, {u1/bar, token_missing}
    - logvalet_space_verify {all_spaces:true}
    - foo: ok=true, bar: error_code="token_missing"

T4: TestLogvaletSpaceConnectUrl_ReturnsAuthURL
    - logvalet_space_connect_url {base_url:"https://foo.backlog.com", alias:"foo"}
    - 返り値に authorization_url が含まれる
    - URL は MultiSpaceOAuthHandler が生成する /oauth/backlog/authorize へのリンク

T5: TestLogvaletSpaceDisconnect_RemovesSpace
    - SpaceStore に {u1/foo}
    - logvalet_space_disconnect {alias:"foo"} → space 削除
```

### tools_fanout_test.go

```
T6: TestLogvaletIssueList_WithSpaces_FanOut
    - spaces=["foo","bar"]
    - httptest で2スペース分のサーバー
    - 結果が [{space:"foo",...},{space:"bar",...}] の配列

T7: TestLogvaletIssueList_AllSpaces_UserIsolation
    - userA: foo, bar → all_spaces で foo+bar のみ
    - userB: baz → all_spaces で baz のみ
    - userA の結果に baz が含まれないこと

T8: TestLogvaletIssueList_NoSpaces_DefaultBehavior
    - spaces/all_spaces 未指定 → 既存の default space 動作
    - 出力形式は single result（配列 envelope でない）

T9: TestMCPTool_SpacesAndAllSpaces_Conflict
    - spaces=["foo"], all_spaces=true → error envelope を返す
```

---

## 2. 新規 MCP tools

```
logvalet_space_list        — 登録済みスペース一覧（現在ユーザー）
logvalet_space_use         — default space 設定
logvalet_space_verify      — 接続確認
logvalet_space_connect_url — OAuth 認可 URL 取得（MultiSpaceOAuthHandler 使用: RC3）
logvalet_space_disconnect  — スペース削除
```

---

## 3. withSpaces ミドルウェア（RH4 対応）

MVP 17ツール × 2引数のスキーマ変更と fan-out 処理を共通化する。
17箇所に個別実装すると実装ミスの温床になるため、ラッパー関数を用意する。

```go
// withSpaces は spaces/all_spaces 引数を解決し、
// 単一スペースなら fn(ctx, singleClient, args) を呼び、
// 複数スペースなら ExecuteAcrossSpaces で fan-out する。
func withSpaces[T any](
    reg *ToolRegistry,
    fn func(ctx context.Context, client backlog.Client, args map[string]any) (T, error),
) func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
    return func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
        spaces, allSpaces := extractSpaceArgs(args)
        
        if spaces == nil && !allSpaces {
            // 既存動作: single client で実行
            return fn(ctx, client, args)
        }
        
        // multi-space: Resolver → ExecuteAcrossSpaces
        // ...
    }
}
```

---

## 4. MVP 対象 17ツール一覧

```
logvalet_space_info, logvalet_space_disk_usage, logvalet_space_digest
logvalet_project_list, logvalet_project_get, logvalet_project_health, logvalet_project_blockers
logvalet_issue_list, logvalet_issue_get, logvalet_issue_context, logvalet_issue_stale
logvalet_digest_daily, logvalet_digest_weekly, logvalet_digest_unified
logvalet_activity_list, logvalet_activity_stats, logvalet_activity_digest
```

---

## 5. 引数スキーマ追加

各ツールに以下を追加（optional: 未指定時は既存動作を保持）:

```json
{
  "spaces": {
    "type": "array",
    "items": {"type": "string"},
    "description": "Target Backlog space aliases. Omit to use the default space.",
    "optional": true
  },
  "all_spaces": {
    "type": "boolean",
    "description": "Run against all spaces registered for the current user.",
    "optional": true
  }
}
```

`spaces: []`（空配列）は「指定なし」と同等（H4 対応: LLM が明示的に渡しても current/default 扱い）。

---

## 6. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/mcp/tools_space_registry_test.go` を作成（T1-T5）
2. `internal/mcp/tools_fanout_test.go` を作成（T6-T9）
3. `go test ./internal/mcp/...` → コンパイルエラー

### Step 2: Green

1. `internal/mcp/tools_space_registry.go` を実装（5 tools）
2. `withSpaces` ミドルウェアを `internal/mcp/tools.go` に追加
3. MVP 17ツールを `withSpaces` でラップ
4. `go test ./internal/mcp/...` → 全テストパス

### Step 3: Refactor

- `withSpaces` の型推論を確認（Go generics の制約に注意）
- `tool_categories.go` に新規 tools の annotation を追加

---

## 7. 検証コマンド

```bash
go test ./internal/mcp/... -v -run TestLogvaletSpace
go test ./internal/mcp/... -v -run TestLogvaletIssueList_WithSpaces
go test -race ./internal/mcp/...
go build ./...
go vet ./...
```

---

## 8. 次のマイルストーン

MS13 + MS14 完了後 → MS15（backward compatibility テスト）が着手可能。
