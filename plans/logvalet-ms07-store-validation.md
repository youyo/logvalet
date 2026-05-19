# MS07: SpaceStore 設定 validation (C1 対応)

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS05, MS06

## 目的

remote MCP + SQLite Store の組み合わせを起動時に拒否する validation を実装する。
SQLite の `user_id="local"` 固定による multi-tenant 漏洩リスク（C1）を防ぐ。

## 完了条件

- [ ] `internal/space/config.go` — StoreType 定数, ValidateSpaceStoreConfig 関数
- [ ] `internal/space/config_test.go` — 全テストケース pass
- [ ] `internal/cli/mcp.go` — 起動時に ValidateSpaceStoreConfig を呼び出す
- [ ] `lv mcp --auth --space-store sqlite` が起動時エラーになる
- [ ] `lv mcp --space-store sqlite`（非認証）は通る
- [ ] `go test ./internal/space/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### config_test.go

```
T1: TestValidateSpaceStoreConfig_Remote_Sqlite_Rejected
    - ValidateSpaceStoreConfig("sqlite", true) → error
    - エラーメッセージに "remote MCP requires dynamodb" が含まれる

T2: TestValidateSpaceStoreConfig_Remote_Memory_Rejected
    - ValidateSpaceStoreConfig("memory", true) → error
    （memory も remote では NG: 再起動で全データ消失）

T3: TestValidateSpaceStoreConfig_Remote_DynamoDB_OK
    - ValidateSpaceStoreConfig("dynamodb", true) → nil

T4: TestValidateSpaceStoreConfig_Local_Sqlite_OK
    - ValidateSpaceStoreConfig("sqlite", false) → nil

T5: TestValidateSpaceStoreConfig_Local_Memory_OK
    - ValidateSpaceStoreConfig("memory", false) → nil

T6: TestValidateSpaceStoreConfig_InvalidStoreType
    - ValidateSpaceStoreConfig("invalid", false) → error
    （不正な store type は local でもエラー）

T7: TestStoreTypeConstants
    - StoreTypeMemory == "memory"
    - StoreTypeSQLite == "sqlite"
    - StoreTypeDynamoDB == "dynamodb"
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/config.go` | StoreType 定数, ValidateSpaceStoreConfig |
| `internal/space/config_test.go` | T1-T7 |

### 更新

| ファイル | 内容 |
|---------|------|
| `internal/cli/mcp.go` | McpAuthCmd.Run で起動時 validation 呼び出し追加 |

---

## 3. 実装

### config.go

```go
package space

import "fmt"

type StoreType string

const (
    StoreTypeMemory   StoreType = "memory"
    StoreTypeSQLite   StoreType = "sqlite"
    StoreTypeDynamoDB StoreType = "dynamodb"
)

// ValidateSpaceStoreConfig は store type と MCP モードの組み合わせを検証する。
// remote MCP で SQLite または memory store を使うと user_id="local" 固定で
// multi-tenant 漏洩が発生するため、dynamodb のみ許可する（C1 対応）。
func ValidateSpaceStoreConfig(storeType string, isMCPRemote bool) error {
    switch StoreType(storeType) {
    case StoreTypeMemory, StoreTypeSQLite, StoreTypeDynamoDB:
        // 有効な store type
    default:
        return fmt.Errorf(
            "space: invalid store type %q; must be one of: memory, sqlite, dynamodb",
            storeType,
        )
    }

    if isMCPRemote && StoreType(storeType) != StoreTypeDynamoDB {
        return fmt.Errorf(
            "remote MCP requires dynamodb store type. "+
                "Set LOGVALET_SPACE_STORE_TYPE=dynamodb and "+
                "LOGVALET_SPACE_DYNAMODB_TABLE=<table-name>. "+
                "Got: %q",
            storeType,
        )
    }
    return nil
}
```

### mcp.go への追加（isMCPRemote の判定）

```go
// McpAuthCmd.Run（起動時 validation）
func (c *McpAuthCmd) Run(g *GlobalFlags) error {
    storeType := os.Getenv("LOGVALET_SPACE_STORE_TYPE")
    if storeType == "" {
        storeType = "sqlite" // デフォルト
    }
    isMCPRemote := c.Auth // --auth フラグが立っていれば remote MCP とみなす
    if err := space.ValidateSpaceStoreConfig(storeType, isMCPRemote); err != nil {
        return fmt.Errorf("startup validation failed: %w", err)
    }
    // ... 既存の起動処理
}
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/config_test.go` を作成（T1-T7）
2. `go test ./internal/space/...` → コンパイルエラー

### Step 2: Green

1. `internal/space/config.go` を実装
2. `go test ./internal/space/...` → 全テストパス
3. `internal/cli/mcp.go` に起動時 validation を追加

### Step 3: Refactor

- エラーメッセージの確認（ユーザーが次に何をすべきかが分かる内容）

---

## 5. 検証コマンド

```bash
go test ./internal/space/... -v -run TestValidate
go test ./internal/space/... -v -run TestStoreType
go build ./...
go vet ./...
```

---

## 6. 次のマイルストーン

MS07 完了後 → MS08（SpaceAwareClientFactory）と並行して MS09 以降へ進む。
