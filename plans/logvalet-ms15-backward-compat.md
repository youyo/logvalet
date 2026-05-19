# MS15: backward compatibility テスト

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS13, MS14

## 目的

既存ユーザーのフローが壊れていないことを全テストケースで確認する。
spec §30.1 の BC1-BC9 テストケースを自動テストとして実装する。

## 完了条件

- [ ] `internal/e2e/` または `internal/cli/` に BC1-BC9 の全テストが実装
- [ ] BC6（Resolver fallback 5）が MemoryStore + WithLegacyProfileFallback でテスト可能
- [ ] `go test ./...` パス（全テスト）

---

## 1. TDD テストケース一覧

### bc_test.go（backward compatibility テスト）

```
BC1: TestBC_NoSpaces_ExistingProfile_UsesProfileSpace
    - --spaces 未指定、config.toml に profile 設定あり
    - buildRunContext が既存パスを使う（SpaceStore 参照しない）
    - 出力形式は既存 JSON（配列 envelope でない）

BC2: TestBC_LiteralSpaceFlag_ExistingBehavior
    - --space foo（既存フラグ）
    - buildRunContext の従来動作（https://foo.backlog.com を構築）
    - alias ベースの SpaceResolver は使わない

BC3: TestBC_EnvVar_LOGVALET_SPACE
    - LOGVALET_SPACE=foo の環境変数
    - 従来通り動作する

BC4: TestBC_ConfigToml_Profile
    - config.toml の profile を使う
    - 従来通り動作する

BC5: TestBC_TokensJson_OAuthToken
    - tokens.json の OAuth token を使う
    - 従来通り動作する

BC6: TestBC_SpaceRegistryEmpty_ProfileFallback
    - SpaceStore は空
    - WithLegacyProfileFallback に有効な config.ResolvedConfig を渡す
    - Resolver.Resolve(Scope{}) → legacy profile から生成した SpaceRegistration を返す
    - ErrNoDefaultSpace にならない

BC7: TestBC_MCP_NoSpacesArg_DefaultBehavior
    - MCP ツールの spaces/all_spaces 引数未指定
    - 従来と同等の結果を返す（単一スペース結果）

BC8: TestBC_AuthTypeAPIKey_ValueConsistency
    - SpaceRegistration{AuthType: "api_key"}
    - credentials.go の AuthTypeAPIKey = "api_key" と値が一致
    - SpaceStore に保存 → 読み込み後も "api_key" のまま（文字列変換なし）

BC9: TestBC_DefaultProfile_SpaceRegistryEmpty_NoError
    - config.toml に default_profile あり、SpaceRegistry 空
    - CLI コマンド実行 → ErrNoDefaultSpace にならない（fallback 5 で profile 使用）
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/e2e/backward_compat_test.go` | BC1-BC9（または `internal/cli/bc_test.go`） |

---

## 3. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/e2e/backward_compat_test.go` を作成（BC1-BC9）
2. `go test ./internal/e2e/...` → 一部テストが fail する場合は後方互換性が壊れている

### Step 2: Green

1. 失敗したテストの根本原因を修正
2. `go test ./...` → 全テストパス

### Step 3: Refactor

- BC テストのヘルパー関数を整理

---

## 4. 検証コマンド

```bash
go test ./internal/e2e/... -v -run TestBC
go test ./...
go vet ./...
```

---

## 5. 次のマイルストーン

MS15 完了後 → MS16（E2E・セキュリティテスト + ドキュメント）が着手可能。
