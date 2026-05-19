# MS13: CLI read-only コマンドの横断対応

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS09, MS12

## 目的

`lv issue list` 等の read-only コマンドで `--spaces` / `--all-spaces` が機能するようにする。
`--spaces/--all-spaces` 未指定時は既存 `buildRunContext` を完全保持（後方互換）。

## 完了条件

- [ ] `internal/cli/runner.go` — buildRunContext を拡張（multi-space モード検出）
- [ ] MVP 対象コマンドへの fan-out 処理追加
- [ ] `lv issue list --spaces foo,bar` が配列形式で返す
- [ ] `lv issue list --all-spaces` が全スペース対象で返す
- [ ] partial failure で一部失敗しても他スペースの結果は返る
- [ ] `go test ./internal/cli/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### runner_ext_test.go または既存テストへの追加

```
T1: TestBuildRunContext_NoSpaces_UsesExistingPath
    - --spaces / --all-spaces なし
    - buildRunContext が既存パス（credentials.Resolve → backlog.NewHTTPClient）を使う
    - SpaceStore は参照しない

T2: TestBuildMultiSpaceContext_SingleSpace
    - --spaces foo
    - SpaceStore に {u1/foo} 登録済み
    - []SpaceRegistration{foo} が解決される

T3: TestBuildMultiSpaceContext_MultipleSpaces
    - --spaces foo,bar
    - SpaceStore に {u1/foo}, {u1/bar}
    - 順序が ["foo","bar"] で保持される
```

### cli_fanout_test.go（各コマンドのテスト）

```
T4: TestIssueListCmd_Spaces_TwoSpaces
    - httptest で foo と bar の2サーバーを立てる
    - foo サーバー: Bearer token-foo を期待、issues[] を返す
    - bar サーバー: Bearer token-bar を期待、issues[] を返す
    - `lv issue list --spaces foo,bar`
    - 出力: [{space:"foo", ok:true, result:{issues:[...]}}, {space:"bar", ok:true, ...}]

T5: TestIssueListCmd_Spaces_PartialFailure
    - foo: 成功、bar: 401 Unauthorized
    - 出力: [{space:"foo", ok:true, ...}, {space:"bar", ok:false, error_code:"unauthorized"}]
    - exit code = ExitCodePartialFailure (8)

T6: TestIssueListCmd_AllSpaces_OnlyCurrentUser
    - userA: foo, bar 登録
    - userB: baz 登録
    - ctxA の --all-spaces → foo, bar のみ（baz は含まれない）

T7: TestIssueListCmd_NoSpaces_ExistingBehavior
    - --spaces なし → 既存の単一スペース動作（後方互換）
    - 出力形式は既存 JSON（array envelope ではない）
```

---

## 2. 対応コマンド（MVP）

```
lv issue list / get / context / stale
lv project list / get / health / blockers
lv space info / disk-usage / digest（既存コマンド、--spaces で横断可能に）
lv digest daily / weekly / unified
lv activity list / stats / digest
```

---

## 3. runner.go の拡張方針

```go
// buildRunContext の拡張（multi-space モード検出）
func buildRunContext(g *GlobalFlags) (*RunContext, error) {
    scope, err := buildSpaceScope(g) // MS11 で実装
    if err != nil {
        return nil, err
    }
    
    if scope != nil {
        // multi-space モード: SpaceResolver + Executor を使う
        return buildMultiSpaceRunContext(g, scope)
    }
    
    // 既存モード: 従来の buildRunContext ロジック（変更なし）
    // ...
}
```

---

## 4. 出力形式の切り替え

```text
--spaces / --all-spaces 未指定:
  既存出力形式（変更なし）

--spaces / --all-spaces 指定:
  []Result[T] の space result envelope 形式:
  [{"space":"foo","ok":true,"result":{...}},{"space":"bar","ok":false,"error_code":"..."}]
```

---

## 5. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/cli/cli_fanout_test.go` を作成（T1-T7）
2. httptest で複数 Backlog サーバーをセットアップ
3. `go test ./internal/cli/...` → コンパイルエラー

### Step 2: Green

1. `internal/cli/runner.go` を拡張
2. MVP 対象コマンドに fan-out 処理を追加
3. `go test ./internal/cli/...` → 全テストパス

### Step 3: Refactor

- fan-out ラッパーのパターン化（共通関数に抽出）
- exit code の統一（partial failure → ExitCodePartialFailure=8）

---

## 6. 検証コマンド

```bash
go test ./internal/cli/... -v -run TestIssueListCmd_Spaces
go test ./internal/cli/... -v -run TestBuildRunContext
go test -race ./internal/cli/...
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS13 + MS14 完了後 → MS15（backward compatibility テスト）が着手可能。
