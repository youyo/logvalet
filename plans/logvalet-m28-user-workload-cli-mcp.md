# M28: User Workload CLI + MCP 実装計画

## 目標

M27 で実装済みの `WorkloadCalculator.Calculate()` を CLI コマンドと MCP ツールから利用できるようにする。

## 前提条件

- M27 完了: `internal/analysis/workload.go` に `WorkloadCalculator` 実装済み
- `WorkloadCalculator.Calculate(ctx, projectKey, config WorkloadConfig)` が利用可能
- `WorkloadConfig`: `StaleDays`, `ExcludeStatus`, `OverloadedThreshold`, `HighThreshold`, `MediumThreshold`

---

## コマンド仕様

```
logvalet user workload PROJECT_KEY [--stale-days 7] [--exclude-status "完了,対応済み"]
```

### フラグ

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|----------|------|
| `PROJECT_KEY` | `string` | required | プロジェクトキー（位置引数） |
| `--stale-days` | `int` | `7` | 停滞判定の閾値（日数） |
| `--exclude-status` | `string` | `""` | 除外ステータス（カンマ区切り） |

---

## MCP ツール仕様

**ツール名:** `logvalet_user_workload`

| パラメータ | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `project_key` | `string` | required | プロジェクトキー |
| `stale_days` | `number` | optional | 停滞判定閾値（日数） |
| `exclude_status` | `string` | optional | 除外ステータス（カンマ区切り） |

---

## TDD 設計（Red → Green → Refactor）

### Phase A: CLI テスト（Red）

ファイル: `internal/cli/user_workload_test.go`

1. **T1**: `user workload PROJ` のパースとデフォルト値確認
2. **T2**: フラグ付きパース（`--stale-days 14 --exclude-status "完了,対応済み"`）
3. **T3**: `PROJECT_KEY` なしでエラー

### Phase B: MCP テスト（Red）

ファイル: `internal/mcp/tools_test.go`

- `expectedCount` を 27 → 28 に更新
- `TestUserWorkload_MCPTool_ReturnsJSON`: `logvalet_user_workload` が JSON を返すことを確認

### Phase C: 実装（Green）

1. `internal/cli/user_workload.go` - `UserWorkloadCmd` 実装
2. `internal/cli/user.go` - `UserCmd` に `Workload UserWorkloadCmd` フィールド追加
3. `internal/mcp/tools_analysis.go` - `logvalet_user_workload` ツール登録

---

## 実装ステップ

### Step 1: CLI テスト作成（Red）

`internal/cli/user_workload_test.go` を作成:

```go
package cli_test

import (
    "bytes"
    "testing"

    "github.com/alecthomas/kong"
    "github.com/youyo/logvalet/internal/cli"
)

// T1: "user workload PROJ" のパースとデフォルト値
func TestUserWorkload_KongParse_Default(t *testing.T) { ... }

// T2: フラグ付きパース
func TestUserWorkload_KongParse_WithFlags(t *testing.T) { ... }

// T3: PROJECT_KEY なしでエラー
func TestUserWorkload_KongParse_MissingProjectKey(t *testing.T) { ... }
```

### Step 2: MCP テスト更新（Red）

`internal/mcp/tools_test.go`:
- `expectedCount` を `27` → `28` に変更
- `TestUserWorkload_MCPTool_ReturnsJSON` テストを追加

### Step 3: CLI 実装（Green）

`internal/cli/user_workload.go`:

```go
package cli

import (
    "context"
    "os"
    "strings"

    "github.com/youyo/logvalet/internal/analysis"
)

// UserWorkloadCmd は user workload コマンド。
type UserWorkloadCmd struct {
    ProjectKey    string `arg:"" required:"" help:"project key"`
    StaleDays     int    `help:"days threshold for stale detection" default:"7"`
    ExcludeStatus string `help:"comma-separated status names to exclude (e.g. '完了,対応済み')"`
}

// Run は user workload コマンドを実行する。
func (c *UserWorkloadCmd) Run(g *GlobalFlags) error {
    ctx := context.Background()
    rc, err := buildRunContext(g)
    if err != nil {
        return err
    }

    var excludeStatus []string
    if c.ExcludeStatus != "" {
        excludeStatus = strings.Split(c.ExcludeStatus, ",")
    }

    cfg := analysis.WorkloadConfig{
        StaleDays:     c.StaleDays,
        ExcludeStatus: excludeStatus,
    }

    calculator := analysis.NewWorkloadCalculator(
        rc.Client,
        rc.Config.Profile,
        rc.Config.Space,
        rc.Config.BaseURL,
    )

    envelope, err := calculator.Calculate(ctx, c.ProjectKey, cfg)
    if err != nil {
        return err
    }

    return rc.Renderer.Render(os.Stdout, envelope)
}
```

### Step 4: UserCmd に Workload を追加（Green）

`internal/cli/user.go` の `UserCmd` に `Workload UserWorkloadCmd` を追加:

```go
type UserCmd struct {
    List     UserListCmd     `cmd:"" help:"list users"`
    Get      UserGetCmd      `cmd:"" help:"get user"`
    Activity UserActivityCmd `cmd:"" help:"get user activities"`
    Workload UserWorkloadCmd `cmd:"" help:"calculate user workload for a project"`
}
```

### Step 5: MCP ツール登録（Green）

`internal/mcp/tools_analysis.go` に `logvalet_user_workload` を追加:

```go
// logvalet_user_workload
r.Register(gomcp.NewTool("logvalet_user_workload",
    gomcp.WithDescription("Calculate user workload distribution for a project"),
    gomcp.WithString("project_key",
        gomcp.Required(),
        gomcp.Description("Project key (e.g. PROJ)"),
    ),
    gomcp.WithNumber("stale_days",
        gomcp.Description("Days threshold for stale detection (default 7)"),
    ),
    gomcp.WithString("exclude_status",
        gomcp.Description("Comma-separated status names to exclude (e.g. '完了,対応済み')"),
    ),
), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
    projectKey, ok := stringArg(args, "project_key")
    if !ok || projectKey == "" {
        return nil, fmt.Errorf("project_key is required")
    }

    workloadCfg := analysis.WorkloadConfig{}
    if staleDays, ok := intArg(args, "stale_days"); ok && staleDays > 0 {
        workloadCfg.StaleDays = staleDays
    }
    if excludeStatusStr, ok := stringArg(args, "exclude_status"); ok && excludeStatusStr != "" {
        workloadCfg.ExcludeStatus = strings.Split(excludeStatusStr, ",")
    }

    calculator := analysis.NewWorkloadCalculator(client, cfg.Profile, cfg.Space, cfg.BaseURL)
    return calculator.Calculate(ctx, projectKey, workloadCfg)
})
```

### Step 6: テスト実行・確認

```bash
go test ./...
go vet ./...
```

---

## ファイル変更一覧

| ファイル | 変更種別 | 内容 |
|---------|---------|------|
| `internal/cli/user_workload.go` | 新規作成 | `UserWorkloadCmd` 実装 |
| `internal/cli/user_workload_test.go` | 新規作成 | CLI パーステスト |
| `internal/cli/user.go` | 修正 | `UserCmd` に `Workload` フィールド追加 |
| `internal/mcp/tools_analysis.go` | 修正 | `logvalet_user_workload` ツール登録 |
| `internal/mcp/tools_test.go` | 修正 | `expectedCount` 27 → 28、新テスト追加 |

---

## 完了条件

- [ ] `logvalet user workload PROJ` が動作する
- [ ] `--stale-days` フラグが機能する
- [ ] `--exclude-status` フラグが機能する
- [ ] MCP ツール `logvalet_user_workload` が登録される
- [ ] `go test ./...` が全パス
- [ ] `go vet ./...` がクリーン
- [ ] tools_test.go の expectedCount が 28 になっている
