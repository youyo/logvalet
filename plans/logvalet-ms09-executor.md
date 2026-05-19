# MS09: ExecuteAcrossSpaces fan-out Executor

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS08

## 目的

複数スペースへの並列実行・partial failure 収集・入力順序保持を実装する。
`ExecuteAcrossSpaces[T any]` は Go generics の制約（メソッドに型パラメータ不可）から
package-level 関数として実装する。

## 完了条件

- [ ] `internal/space/executor.go` — Executor, Result[T], ExecuteAcrossSpaces[T]
- [ ] `internal/space/executor_test.go` — 全テストケース pass
- [ ] `go test -race ./internal/space/...` パス（race detector 有効）

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### executor_test.go

```
T1: TestExecuteAcrossSpaces_AllSuccess
    - spaces = [{foo}, {bar}]、どちらも fn が成功
    - 返り値の len == 2
    - results[0].SpaceAlias == "foo", results[0].OK == true
    - results[1].SpaceAlias == "bar", results[1].OK == true

T2: TestExecuteAcrossSpaces_PartialFailure
    - spaces = [{foo}, {bar}]
    - foo は成功、bar は fn がエラーを返す
    - results[0].OK == true
    - results[1].OK == false, results[1].ErrorCode != ""
    - （全体は fail しない）

T3: TestExecuteAcrossSpaces_InputOrderPreserved
    - spaces = [{a}, {b}, {c}, {d}, {e}]（5件）
    - fn は各スペースに短い sleep でランダム遅延を入れる
    - results の SpaceAlias が ["a","b","c","d","e"] の順を保持する

T4: TestExecuteAcrossSpaces_MaxConcurrency_Zero_Fallback
    - Executor{MaxConcurrency: 0} → デフォルト 4 にフォールバック
    - deadlock しないことを確認（10件のスペースを問題なく処理）

T5: TestExecuteAcrossSpaces_MaxConcurrency_Negative_Fallback
    - Executor{MaxConcurrency: -1} → デフォルト 4 にフォールバック

T6: TestExecuteAcrossSpaces_ContextCancellation
    - spaces = [{a}, {b}, ..., {z}]（26件）
    - cancel() を呼ぶ
    - fn は context 経由でキャンセルを受け取る
    - goroutine leak しないこと（goleak または手動確認）

T7: TestExecuteAcrossSpaces_EmptySpaces
    - spaces = []
    - 返り値は空スライス（nil でなく []Result[T]{}）

T8: TestExecuteAcrossSpaces_FactoryError_Mapped_To_Result
    - factory が not_configured エラーを返す
    - results[*].OK == false
    - results[*].ErrorCode == "not_configured"

T9: TestResult_FailureCase_ResultIsNil
    - OK=false の Result の Result フィールドは nil
    （*T なので失敗時は nil になることを確認）
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/executor.go` | Executor, Result[T], ExecuteAcrossSpaces[T] |
| `internal/space/executor_test.go` | T1-T9 |

---

## 3. 実装

### executor.go

```go
package space

import (
    "context"
    "sync"

    "golang.org/x/sync/semaphore"
    "github.com/youyo/logvalet/internal/auth"
    "github.com/youyo/logvalet/internal/backlog"
)

const defaultMaxConcurrency = 4

// Result は1スペースの実行結果。
// *T を使うことで失敗時（OK=false）に null/omitted になる（GPT-5.4 指摘対応）。
type Result[T any] struct {
    SpaceAlias string `json:"space"`
    Tenant     string `json:"tenant,omitempty"`
    BaseURL    string `json:"base_url,omitempty"`
    OK         bool   `json:"ok"`
    Result     *T     `json:"result,omitempty"` // *T: 失敗時 nil
    Error      string `json:"error,omitempty"`
    ErrorCode  string `json:"error_code,omitempty"`
}

// Executor は fan-out 実行設定を保持する。型パラメータは持たない。
type Executor struct {
    Factory        auth.SpaceAwareClientFactory
    MaxConcurrency int // <= 0 なら defaultMaxConcurrency (4) にフォールバック（H1 対応）
}

// resolveMaxConcurrency は MaxConcurrency を正規化する（H1: deadlock 防止）。
func (e *Executor) resolveMaxConcurrency() int {
    if e.MaxConcurrency <= 0 {
        return defaultMaxConcurrency
    }
    return e.MaxConcurrency
}

// ExecuteAcrossSpaces は spaces 一覧に対して fn を並列実行し、
// 入力順を保持した []Result[T] を返す。
// 1スペースの失敗は Result.OK=false に記録し、全体は続行する（partial failure）。
func ExecuteAcrossSpaces[T any](
    ctx context.Context,
    executor *Executor,
    spaces []SpaceRegistration,
    fn func(context.Context, SpaceRegistration, backlog.Client) (T, error),
) []Result[T] {
    results := make([]Result[T], len(spaces))
    maxConc := executor.resolveMaxConcurrency()
    sem := semaphore.NewWeighted(int64(maxConc))
    var wg sync.WaitGroup

    for i, reg := range spaces {
        i, reg := i, reg // capture
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := sem.Acquire(ctx, 1); err != nil {
                results[i] = Result[T]{
                    SpaceAlias: reg.Alias,
                    Tenant:     reg.Tenant,
                    OK:         false,
                    ErrorCode:  "context_cancelled",
                    Error:      err.Error(),
                }
                return
            }
            defer sem.Release(1)

            result := executeOne(ctx, executor.Factory, reg, fn)
            results[i] = result
        }()
    }

    wg.Wait()
    return results
}

func executeOne[T any](
    ctx context.Context,
    factory auth.SpaceAwareClientFactory,
    reg SpaceRegistration,
    fn func(context.Context, SpaceRegistration, backlog.Client) (T, error),
) Result[T] {
    base := Result[T]{
        SpaceAlias: reg.Alias,
        Tenant:     reg.Tenant,
        BaseURL:    reg.BaseURL,
    }

    client, err := factory(ctx, reg)
    if err != nil {
        base.OK = false
        base.Error = err.Error()
        base.ErrorCode = mapErrorCode(err)
        return base
    }

    val, err := fn(ctx, reg, client)
    if err != nil {
        base.OK = false
        base.Error = err.Error()
        base.ErrorCode = mapErrorCode(err)
        return base
    }

    base.OK = true
    base.Result = &val
    return base
}
```

---

## 4. エラーコードマッピング

```go
func mapErrorCode(err error) string {
    switch {
    case errors.Is(err, auth.ErrProviderNotConnected):
        return "not_connected"
    case errors.Is(err, auth.ErrUnauthenticated):
        return "not_configured"
    case errors.Is(err, backlog.ErrUnauthorized):
        return "unauthorized"
    case errors.Is(err, backlog.ErrForbidden):
        return "forbidden"
    case errors.Is(err, backlog.ErrRateLimit):
        return "rate_limited"
    default:
        return "internal_error"
    }
}
```

---

## 5. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/executor_test.go` を作成（T1-T9）
2. `go test ./internal/space/...` → コンパイルエラー

### Step 2: Green

1. `internal/space/executor.go` を実装
2. `go test ./internal/space/...` → 全テストパス

### Step 3: Refactor

- `go test -race ./internal/space/...` で T3、T6 の race detector を通す
- `executeOne` ヘルパーの型推論を確認

---

## 6. 検証コマンド

```bash
go test ./internal/space/... -v -run TestExecute -race
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS09 完了後:
- MS14（MCP spaces/all_spaces 共通引数）が着手可能（MS10 と並行）
- MS09 + MS12 完了後 → MS13（CLI read-only 横断対応）
