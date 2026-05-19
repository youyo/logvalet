# MS16: E2E・セキュリティテスト + ドキュメント

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS15

## 目的

user isolation・replay attack 防止・ログ秘密漏洩なしを自動テストで証明し、
ドキュメントを整備する。spec §23 の完了条件12項目を全て満たす。

## 完了条件

- [ ] user A/B 分離テスト pass
- [ ] state 改ざん → 400 テスト pass
- [ ] callback userID mismatch → 401 テスト pass
- [ ] nonce replay → 400 テスト pass
- [ ] ログにトークン含まれないテスト pass
- [ ] httptest multi-server fan-out テスト pass
- [ ] README に multi-space セクション追加
- [ ] remote MCP 運用ガイド（DynamoDB テーブル作成順序）追加
- [ ] `go test ./...` パス（全テスト）
- [ ] spec §23 完了条件 12項目を全て満たす

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### security_test.go

```
T1: TestSecurity_UserAB_Isolation
    - userA: spaces [foo, bar], tokenA
    - userB: spaces [baz], tokenB
    - ctxA の all_spaces → foo, bar のみ（baz なし）
    - ctxB の all_spaces → baz のみ（foo, bar なし）

T2: TestSecurity_StateTampering_Rejected
    - 正規 state を生成し、JWT payload を改ざん
    - ValidateState → ErrStateInvalid

T3: TestSecurity_CallbackUserMismatch_Rejected
    - state.uid = "u1"、ctx.userID = "u2"
    - MultiSpaceOAuthHandler.HandleCallback → 401 user_mismatch

T4: TestSecurity_NonceReplay_Rejected
    - 同じ callback を2回送る
    - 1回目: 200 成功
    - 2回目: 400 nonce_already_used

T5: TestSecurity_NoTokenInLogs
    - slog の出力を capture するハンドラーを設定
    - ExecuteAcrossSpaces を実行（エラーケース含む）
    - ログに "access_token", "refresh_token", "Bearer ", "api_key" が含まれないこと
```

### e2e_multi_space_test.go

```
T6: TestE2E_FanOut_MultipleBacklogServers
    - httptest で foo と bar の2サーバーを立てる
    - foo: Bearer token-foo を期待
    - bar: Bearer token-bar を期待
    - issue list --all-spaces → 両スペースの結果

T7: TestE2E_FanOut_PartialFailure
    - foo: 成功、bar: 401
    - 結果: [{space:"foo",ok:true,...},{space:"bar",ok:false,error_code:"unauthorized"}]
    - CLI exit code = ExitCodePartialFailure (8)

T8: TestE2E_OAuth_Callback_SpaceAutoRegistered
    - MultiSpaceOAuthHandler の正常 callback
    - SpaceRegistry に {u1/foo} が登録される
    - DefaultSpaceAlias = "foo"（最初の登録）
```

---

## 2. ドキュメント

### README 更新

```markdown
## Multi-Space Support

logvalet supports managing multiple Backlog spaces:

### Register spaces
\```bash
# OAuth
lv spaces connect --base-url https://foo.backlog.com --alias foo

# API key
lv spaces add --alias bar --base-url https://bar.backlog.com --auth-type api_key --auth-profile bar

# List
lv spaces list
\```

### Cross-space operations
\```bash
lv issue list --spaces foo,bar
lv issue list --all-spaces
lv project list --spaces foo,bar
\```

### Set default space
\```bash
lv spaces use foo
\```
```

### remote MCP 運用ガイド（docs/ops/remote-mcp.md）

```markdown
## remote MCP 運用ガイド

### DynamoDB テーブル作成順序

1. logvalet-spaces テーブルを作成
2. logvalet-auth テーブルを確認（既存）
3. 新コードをデプロイ
4. ユーザーに `lv spaces connect` を案内

### 環境変数

\```
LOGVALET_MCP_MODE=remote
LOGVALET_SPACE_STORE_TYPE=dynamodb
LOGVALET_SPACE_DYNAMODB_TABLE=logvalet-spaces
LOGVALET_AUTH_DYNAMODB_TABLE=logvalet-auth
\```
```

---

## 3. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/e2e/security_test.go` を作成（T1-T5）
2. `internal/e2e/e2e_multi_space_test.go` を作成（T6-T8）
3. `go test ./internal/e2e/...` → テストが fail

### Step 2: Green

1. 失敗したテストを修正
2. `go test ./...` → 全テストパス

### Step 3: Refactor + ドキュメント

1. README に multi-space セクション追加
2. `docs/ops/remote-mcp.md` を新規作成
3. spec §23 の完了条件チェックリストを確認

---

## 4. spec §23 完了条件チェックリスト

```
[T6] 1. ローカル CLI で複数 API key space を登録できる
[T6] 2. CLI で --spaces foo,bar により横断 read-only 操作ができる
[T6] 3. CLI で --all-spaces により登録済み全スペースを対象にできる
[T8] 4. OAuth callback 成功時に SpaceRegistry が自動更新される
[T1] 5. remote MCP でユーザーごとに space registry が分離される
[T1] 6. MCP all_spaces が現在ユーザーの登録済みスペースだけを対象にする
[T8] 7. default space がユーザーごとに保存され、spaces 未指定時に使われる
     8. API key で credential が無いスペースは not_configured になる（MS08）
[T7] 9. 認証失敗は space 単位の partial failure として返る
     10. write 操作で multi-space 指定は安全に拒否される（MS14）
[T5] 11. token/API key がログに出ない
     12. unit/integration/security tests が通る
```

---

## 5. 検証コマンド

```bash
go test ./internal/e2e/... -v -run TestSecurity
go test ./internal/e2e/... -v -run TestE2E_MultiSpace
go test ./...
go vet ./...
```
