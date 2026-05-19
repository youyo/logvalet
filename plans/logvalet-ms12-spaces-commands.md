# MS12: CLI lv spaces 管理コマンド

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS08, MS10, MS11

## 目的

`lv spaces` 管理コマンド群（list/add/connect/remove/use/verify）を実装する。
既存の `lv space info/disk-usage/digest` とは名前空間を分離（SpacesCmd と SpaceCmd は別物）。
rename は MVP 除外（C4 対応）。

## 完了条件

- [ ] `internal/cli/spaces_registry.go` — SpacesCmd と全サブコマンド
- [ ] `lv spaces list` — 登録済みスペース一覧表示
- [ ] `lv spaces add` — API key 登録
- [ ] `lv spaces connect` — OAuth 登録 URL 取得（browser フロー）
- [ ] `lv spaces remove <alias>` — 削除（TransactWriteItems で PREF も更新: H3 対応）
- [ ] `lv spaces use <alias>` — default space 設定
- [ ] `lv spaces verify [--spaces / --all-spaces]` — 接続確認（token_missing 検出: C2 対応）
- [ ] `go test ./internal/cli/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### spaces_registry_test.go

#### lv spaces list

```
T1: TestSpacesListCmd_NoSpaces
    - SpaceStore が空
    - 出力: {"spaces": []}
    
T2: TestSpacesListCmd_WithSpaces
    - SpaceStore に {foo, oauth, ok}, {bar, api_key, unknown}
    - 出力 JSON に alias, auth_type, status, default フィールドが含まれる
    - default: true のスペースが1件
```

#### lv spaces remove

```
T3: TestSpacesRemoveCmd_Success
    - Upsert({u1/foo})、Default = "foo"
    - remove foo
    - Get("u1", "foo") → nil
    - GetPreference("u1").DefaultSpaceAlias → "" または別 alias（H3: 自動更新）

T4: TestSpacesRemoveCmd_DefaultSpaceUpdated_WhenRemainingSpaces
    - Upsert({u1/foo}), Upsert({u1/bar})、Default = "foo"
    - remove foo → Default が "bar" に自動更新される（H3 対応）

T5: TestSpacesRemoveCmd_DefaultSpaceCleared_WhenNoRemaining
    - Upsert({u1/foo})、Default = "foo"
    - remove foo → DefaultSpaceAlias が "" にクリアされる
    - 出力に "No spaces registered" メッセージ

T6: TestSpacesRemoveCmd_NotExist
    - remove nonexistent → error（ErrSpaceNotFound）
```

#### lv spaces use

```
T7: TestSpacesUseCmd_Success
    - Upsert({u1/foo})
    - use foo → DefaultSpaceAlias = "foo"

T8: TestSpacesUseCmd_NotRegistered
    - use nonexistent → error（ErrSpaceNotFound）
```

#### lv spaces verify

```
T9: TestSpacesVerifyCmd_Connected
    - SpaceStore に {u1/foo, oauth}、TokenStore に token
    - verify: result.status = "ok"

T10: TestSpacesVerifyCmd_TokenMissing
    - SpaceStore に {u1/foo, oauth}、TokenStore に token なし
    - verify: result.error_code = "token_missing"
    - 出力に "run 'lv spaces connect --alias foo' to reconnect" メッセージ（C2 対応）

T11: TestSpacesVerifyCmd_Unauthorized
    - Backlog API が 401 を返す
    - result.error_code = "unauthorized"
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/cli/spaces_registry.go` | SpacesCmd, SpacesListCmd, SpacesAddCmd, SpacesConnectCmd, SpacesRemoveCmd, SpacesUseCmd, SpacesVerifyCmd |
| `internal/cli/spaces_registry_test.go` | T1-T11 |

### 更新

| ファイル | 内容 |
|---------|------|
| `cmd/logvalet/main.go` または root.go | SpacesCmd を Kong に追加 |

---

## 3. SpacesCmd 構造

```go
package cli

// SpacesCmd は spaces registry 管理コマンド群のルート。
// 既存 SpaceCmd（lv space info/disk-usage/digest）とは別物。
type SpacesCmd struct {
    List    SpacesListCmd    `cmd:"" help:"list registered spaces"`
    Add     SpacesAddCmd     `cmd:"" help:"register a space with API key"`
    Connect SpacesConnectCmd `cmd:"" help:"register a space via OAuth (returns authorization URL)"`
    Remove  SpacesRemoveCmd  `cmd:"" name:"remove" arg:"" help:"remove a registered space"`
    Use     SpacesUseCmd     `cmd:"" name:"use" arg:"" help:"set default space"`
    Verify  SpacesVerifyCmd  `cmd:"" help:"verify space connections"`
    // Rename は MVP 除外（C4: DynamoDB non-atomic rename リスク）
}
```

---

## 4. lv spaces remove の実装詳細（H3 対応）

```go
func (c *SpacesRemoveCmd) Run(g *GlobalFlags) error {
    // 1. alias が登録済みか確認
    // 2. 削除後の default space を計算
    //    - 残りの enabled space のうち最初のもの
    //    - 残りがなければ ""
    // 3. DynamoDB: TransactWriteItems で Delete SPACE + Update PREF を同時実行
    //    SQLite: BEGIN TRANSACTION で同時実行
    // 4. 結果メッセージ:
    //    残りあり: "Removed 'foo'. Default space changed to 'bar'."
    //    残りなし: "Removed 'foo'. No spaces registered. Run 'lv spaces add' or 'lv spaces connect'."
}
```

---

## 5. lv spaces verify の token_missing 検出（C2 対応）

```go
func verifySpace(ctx context.Context, reg space.SpaceRegistration, tm auth.TokenManager) VerifyResult {
    // まず token 存在を確認
    token, err := tm.GetValidToken(ctx, userID, "backlog", reg.Tenant)
    if err != nil {
        if errors.Is(err, auth.ErrProviderNotConnected) {
            return VerifyResult{
                Alias:     reg.Alias,
                OK:        false,
                ErrorCode: "token_missing", // "not_connected" より具体的
                Message:   fmt.Sprintf("run 'lv spaces connect --alias %s' to reconnect", reg.Alias),
            }
        }
        // その他のエラー
    }
    // Backlog API で GetSpace を呼んで接続確認
}
```

---

## 6. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/cli/spaces_registry_test.go` を作成（T1-T11）
2. `go test ./internal/cli/...` → コンパイルエラー

### Step 2: Green

1. `internal/cli/spaces_registry.go` を実装
2. `go test ./internal/cli/...` → 全テストパス

### Step 3: Refactor

- JSON 出力形式の統一（既存 Renderer パターンを使う）

---

## 7. 検証コマンド

```bash
go test ./internal/cli/... -v -run TestSpaces
go build ./...
go vet ./...
```

---

## 8. 次のマイルストーン

MS09 + MS12 完了後 → MS13（CLI read-only 横断対応）が着手可能。
