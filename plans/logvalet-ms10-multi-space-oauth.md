# MS10: StateClaims 拡張 + MultiSpaceOAuthHandler + Nonce (C2/C3 対応)

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS06, MS06a, MS08

## 目的

multi-space OAuth 登録フローを実装する。
- C2: TokenStore.Put → SpaceRegistry.Upsert の書き込み順序固定
- C3: OAuth state nonce の consume-once（replay attack 防止）を MVP から必須化

既存の `OAuthHandler` は変更しない（後方互換保持）。

## 完了条件

- [ ] `internal/auth/state.go` に BaseURL/Alias フィールド追加（後方互換）
- [ ] `internal/transport/http/multi_space_oauth_handler.go` — MultiSpaceOAuthHandler
- [ ] 全テストケース pass
- [ ] state 改ざん → 401
- [ ] userID mismatch → 401
- [ ] nonce replay → 400
- [ ] 正常 callback → SpaceRegistry に登録、TokenStore に token 保存

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### state_ext_test.go（既存 state_test.go への追加）

```
T1: TestGenerateStateWithSpaceInfo_ContainsBaseURLAndAlias
    - GenerateStateWithSpaceInfo("u1", "foo", "https://foo.backlog.com", "foo", secret, ttl)
    - ValidateState で ParseState → BaseURL, Alias フィールドが含まれる

T2: TestStateClaims_BackwardCompat_ExistingFields
    - 既存 GenerateState(userID, tenant, secret, ttl) で生成した JWT を ValidateState で検証
    - BaseURL, Alias は空文字でエラーにならない（omitempty）
```

### multi_space_oauth_handler_test.go

```
T3: TestMultiSpaceOAuthHandler_HandleAuthorize_Success
    - GET /oauth/backlog/authorize?base_url=https://foo.backlog.com&alias=foo
    - 302 → Backlog OAuth URL へリダイレクト
    - state JWT に BaseURL, Alias が含まれる
    - nonce が NonceStore に保存される

T4: TestMultiSpaceOAuthHandler_HandleCallback_Success
    - 正常 callback（code + state）
    - nonce 消費 → TokenStore.Put → SpaceRegistry.Upsert の順で実行
    - SpaceRegistry に {u1/foo} が登録される
    - レスポンス 200 JSON

T5: TestMultiSpaceOAuthHandler_HandleCallback_StateTampering
    - JWT の署名を改ざん
    - 400 state_invalid

T6: TestMultiSpaceOAuthHandler_HandleCallback_UserMismatch
    - state の uid="u1"、ctx の userID="u2"
    - 401 user_mismatch

T7: TestMultiSpaceOAuthHandler_HandleCallback_NonceReplay
    - 同じ callback を2回送る
    - 1回目: 200 成功
    - 2回目: 400 nonce_already_used（C3 対応）

T8: TestMultiSpaceOAuthHandler_HandleCallback_TokenStoreFailure
    - TokenStore.Put がエラーを返す
    - 500 internal_error
    - SpaceRegistry.Upsert は呼ばれない（書き込み順序保証: C2）

T9: TestMultiSpaceOAuthHandler_HandleCallback_SpaceUpsertFailure
    - TokenStore.Put は成功
    - SpaceRegistry.Upsert がエラーを返す
    - 500 internal_error（但し token は保存済み → lv spaces connect 再実行で回復可能）

T10: TestMultiSpaceOAuthHandler_HandleCallback_DefaultSpaceSetIfEmpty
    - UserPreference が未設定の場合
    - callback 後に DefaultSpaceAlias = "foo" が設定される
    - 既に DefaultSpaceAlias が設定済みの場合は変更しない（条件付き write）
```

---

## 2. ファイル一覧

### 更新

| ファイル | 内容 |
|---------|------|
| `internal/auth/state.go` | StateClaims に BaseURL/Alias フィールド追加 |

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/transport/http/multi_space_oauth_handler.go` | MultiSpaceOAuthHandler |
| `internal/transport/http/multi_space_oauth_handler_test.go` | T3-T10 |

---

## 3. StateClaims 拡張

```go
// state.go への追加（後方互換: omitempty）
type StateClaims struct {
    UserID   string `json:"uid"`
    Tenant   string `json:"tenant"`
    Nonce    string `json:"nonce"`
    Continue string `json:"continue,omitempty"`
    // multi-space 登録フロー用追加フィールド（既存フローでは空でよい）
    BaseURL  string `json:"base_url,omitempty"`
    Alias    string `json:"alias,omitempty"`
    jwt.RegisteredClaims
}

// GenerateStateWithSpaceInfo は multi-space 登録用 state JWT を生成する。
// alias は authorize 前に確定している必要がある（callback 側での alias 生成禁止: C2）。
func GenerateStateWithSpaceInfo(
    userID, tenant, baseURL, alias string,
    secret []byte,
    ttl time.Duration,
) (string, error)
```

---

## 4. Callback 処理順序（C2 対応）

```
MultiSpaceOAuthHandler.HandleCallback:
  1. state JWT 検証（署名・期限）
  2. userID 一致検証（ctx vs state.uid）
  3. nonce 消費（NonceStore.Consume → 失敗なら 400 nonce_already_used）
  4. code exchange → token 取得
  5. TokenStore.Put（先に保存: 失敗なら 500、SpaceRegistry.Upsert は呼ばない）
  6. GetCurrentUser / GetSpace で検証
  7. SpaceRegistry.Upsert（べき等なので失敗しても再実行可能）
  8. UserPreference 条件付き更新（DefaultSpaceAlias == "" なら設定）
  9. 200 JSON レスポンス
```

---

## 5. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/auth/state.go` への拡張テストを追加（T1-T2）
2. `internal/transport/http/multi_space_oauth_handler_test.go` を作成（T3-T10）
3. `go test ./...` → コンパイルエラー

### Step 2: Green

1. `internal/auth/state.go` を更新（GenerateStateWithSpaceInfo 追加）
2. `internal/transport/http/multi_space_oauth_handler.go` を実装
3. `go test ./...` → 全テストパス

### Step 3: Refactor

- 既存 OAuthHandler との共通ロジック（writeJSONError 等）を抽出
- callback 処理順序のコメントを明確化

---

## 6. 検証コマンド

```bash
go test ./internal/auth/... -v -run TestGenerateStateWithSpaceInfo
go test ./internal/transport/... -v -run TestMultiSpaceOAuthHandler
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS10 完了後:
- MS09 + MS10 完了後 → MS14（MCP tools）が着手可能
- MS08 + MS10 + MS11 完了後 → MS12（lv spaces コマンド）が着手可能
