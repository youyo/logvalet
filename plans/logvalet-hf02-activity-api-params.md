---
title: HF02 - Activity API パラメータ不具合修正
project: logvalet
author: planning-agent
created: 2026-04-01
status: Draft
complexity: M
type: hotfix
---

# HF02: Activity API パラメータ不具合 + 完了課題フィルタ修正

## 概要

実コマンド実行で検出された 2 種類の不具合を一括修正する。

### A. Activity API パラメータ不具合
Backlog API の activities 系エンドポイントがサポートしないパラメータ (`offset`, `since`, `until`) を
誤って送信しているため `unknownParameter` エラーが発生している。

- 不具合 A-1: `CommentTimelineBuilder.fetchActivities()` が `offset` ベースのページネーションを使用
- 不具合 A-2: `ActivityStatsBuilder.Build()` が `Since`/`Until` を API パラメータとして送信

### B. 完了課題フィルタ不具合
`issue stale` / `project blockers` / `project health` / `user workload` が
ステータス「完了」の課題をデフォルトで除外していないため、完了済み課題がブロッカー・停滞課題として検出される。

- 不具合 B-1: `ExcludeStatus` のデフォルトが空のため、`--exclude-status "完了"` を明示しないと完了課題が含まれる
- 影響: `issue stale` で全 13 件が完了課題、`project blockers` で全 15 件が完了課題、`project health` の score=0 は完了課題混入のため

## Backlog API 仕様（正しいパラメータ）

以下の3エンドポイントはすべて同じパラメータセットを持つ:
- `GET /api/v2/space/activities`
- `GET /api/v2/projects/:projectIdOrKey/activities`
- `GET /api/v2/users/:userId/activities`

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `activityTypeId[]` | number[] | アクティビティタイプでフィルタ |
| `minId` | number | 取得する最小 ID（下限） |
| `maxId` | number | 取得する最大 ID（上限） |
| `count` | number | 取得件数（最大100、デフォルト20） |
| `order` | string | `"asc"` or `"desc"` |

**サポートされないパラメータ: `offset`, `since`, `until`**

期間フィルタはクライアント側（ローカル）で実施する必要がある。
ページネーションは `maxId`（1ページ目は省略、2ページ目以降は前ページ末尾の `id - 1`）で実現する。

## 調査結果

### 現在の問題箇所

#### `internal/backlog/options.go`

```go
// ListActivitiesOptions — Since/Until/Offset が定義されており API に送られる
type ListActivitiesOptions struct {
    ProjectKey string
    Since      *time.Time  // 不正: API 非サポート
    Until      *time.Time  // 不正: API 非サポート
    Limit      int
    Offset     int         // 不正: API 非サポート
}

// ListUserActivitiesOptions — Since/Until/Offset が定義されており API に送られる
type ListUserActivitiesOptions struct {
    Since   *time.Time  // 不正: API 非サポート
    Until   *time.Time  // 不正: API 非サポート
    Limit   int
    Offset  int         // 不正: API 非サポート
    Project string
    Types   []string
}
```

#### `internal/backlog/http_client.go`

3つのメソッド全てで `offset`/`since`/`until` をクエリパラメータに変換している:
- `ListProjectActivities()` (行 504-527)
- `ListSpaceActivities()` (行 531-553)
- `ListUserActivities()` (行 217-244)

#### `internal/analysis/timeline.go`

`fetchActivities()` (行 201-230) が `offset` ベースのページネーションを使用:
```go
for page := 0; page < opt.MaxActivityPages; page++ {
    batch, err := b.client.ListProjectActivities(ctx, projectKey, backlog.ListActivitiesOptions{
        Limit:  DefaultActivityPageLimit,
        Offset: offset,  // 不正: API 非サポート
    })
    offset += len(batch)
}
```

#### `internal/analysis/activity_stats.go`

`Build()` (行 117-166) が `Since`/`Until` を API に渡している:
```go
acts, err := b.client.ListProjectActivities(ctx, opt.ScopeKey, backlog.ListActivitiesOptions{
    Since: &since,  // 不正: API 非サポート
    Until: &until,  // 不正: API 非サポート
})
```

#### `internal/cli/activity.go`

`Run()` (行 35-51) が `Offset` と `Since` を API に渡している:
```go
opt := backlog.ListActivitiesOptions{
    Limit:  c.Count,
    Offset: c.Offset,  // 不正: API 非サポート
}
opt.Since = &t  // 不正: API 非サポート
```

#### `internal/mcp/tools_activity.go`

`logvalet_activity_list` ツール (行 22-24) が `offset` を API に渡している:
```go
if offset, ok := intArg(args, "offset"); ok {
    opt.Offset = offset  // 不正: API 非サポート
}
```

### 正常動作している箇所（参照実装）

`internal/digest/unified.go` + `internal/digest/activity_filter.go` は既に正しい実装をしている:
- `maxId` ベースのページネーション (`FetchActivitiesWithDateFilter`)
- `since`/`until` によるクライアント側フィルタリング
- fetcher 関数のシグネチャ: `func(ctx context.Context, count int, maxID int) ([]interface{}, error)`

この実装を analysis パッケージでも活用できる。

## 修正方針

### 修正対象ファイル一覧

| ファイル | 修正内容 |
|---------|---------|
| `internal/backlog/options.go` | `ListActivitiesOptions` / `ListUserActivitiesOptions` から `Offset`/`Since`/`Until` を削除し `MinId`/`MaxId`/`Order` を追加 |
| `internal/backlog/options_test.go` | `ListActivitiesOptions` / `ListUserActivitiesOptions` のテストを新フィールドに合わせて更新 |
| `internal/backlog/http_client.go` | 3メソッドのクエリパラメータ変換から `offset`/`since`/`until` を削除し `minId`/`maxId`/`order` を追加 |
| `internal/analysis/timeline.go` | `fetchActivities()` のページネーションを `maxId` ベースに変更 |
| `internal/analysis/activity_stats.go` | `Build()` の API 呼び出しから `Since`/`Until` を削除し、クライアント側フィルタリングに切り替え |
| `internal/cli/activity.go` | `Offset` / `Since` の直接渡しを削除。`Since` は表示用メタ情報のみ |
| `internal/mcp/tools_activity.go` | `offset` パラメータの `Offset` セットを削除 |

### 1. `ListActivitiesOptions` の修正

```go
// Before
type ListActivitiesOptions struct {
    ProjectKey string
    Since      *time.Time
    Until      *time.Time
    Limit      int
    Offset     int
}

// After
type ListActivitiesOptions struct {
    ActivityTypeIDs []int      // activityTypeId[] フィルタ
    MinId           int        // minId: この ID 以上の活動を取得（0 = 制限なし）
    MaxId           int        // maxId: この ID 以下の活動を取得（0 = 制限なし）
    Count           int        // count: 取得件数（最大100）
    Order           string     // "asc" or "desc"（空文字 = API デフォルト desc）
}
```

`ProjectKey` フィールドは使用箇所がないため削除する。

### 2. `ListUserActivitiesOptions` の修正

```go
// Before
type ListUserActivitiesOptions struct {
    Since   *time.Time
    Until   *time.Time
    Limit   int
    Offset  int
    Project string
    Types   []string
}

// After
type ListUserActivitiesOptions struct {
    ActivityTypeIDs []int      // activityTypeId[] フィルタ（Types フィールドを置き換え）
    MinId           int        // minId
    MaxId           int        // maxId
    Count           int        // count: 取得件数（最大100）
    Order           string     // "asc" or "desc"
}
```

`Types []string` は `ActivityTypeIDs []int` に統合する（型も Backlog API の実際の型 int に合わせる）。
`Project` フィールドは analysis 内ロジックのフィールドであり API パラメータではないため削除する。

### 3. `http_client.go` クエリパラメータ変換の修正

3メソッド共通の変換ロジックを修正:

```go
// ListProjectActivities, ListSpaceActivities 共通（ListActivitiesOptions）
q := url.Values{}
for _, id := range opt.ActivityTypeIDs {
    q.Add("activityTypeId[]", strconv.Itoa(id))
}
if opt.MinId > 0 {
    q.Set("minId", strconv.Itoa(opt.MinId))
}
if opt.MaxId > 0 {
    q.Set("maxId", strconv.Itoa(opt.MaxId))
}
if opt.Count > 0 {
    q.Set("count", strconv.Itoa(opt.Count))
}
if opt.Order != "" {
    q.Set("order", opt.Order)
}
```

`ListUserActivities` も同様に `ListUserActivitiesOptions` の新フィールドに対応する。

### 4. `timeline.go` ページネーション修正（不具合1の修正）

`fetchActivities()` を `maxId` ベースのページネーションに変更する。
`digest/activity_filter.go` の `FetchActivitiesWithDateFilter` は `interface{}` スライスを扱うため
analysis パッケージでは独自の typed 実装を行う（`domain.Activity` スライスを直接扱う）。

```go
func (b *CommentTimelineBuilder) fetchActivities(ctx context.Context, projectKey string, opt CommentTimelineOptions) ([]domain.Activity, bool, error) {
    var result []domain.Activity
    truncated := false
    maxId := 0  // 初回は 0（制限なし）

    for page := 0; page < opt.MaxActivityPages; page++ {
        listOpt := backlog.ListActivitiesOptions{
            Count: DefaultActivityPageLimit,
        }
        if maxId > 0 {
            listOpt.MaxId = maxId
        }

        batch, err := b.client.ListProjectActivities(ctx, projectKey, listOpt)
        if err != nil {
            return nil, false, err
        }

        // クライアント側で since/until フィルタ
        for _, a := range batch {
            if a.Created != nil {
                if opt.Since != nil && a.Created.Before(*opt.Since) {
                    // since より古い → これ以上取る必要なし（ID 降順前提）
                    return result, false, nil
                }
                if opt.Until != nil && a.Created.After(*opt.Until) {
                    continue  // until より新しい → スキップ
                }
            }
            result = append(result, a)
        }

        // 取得件数が limit 未満 → これ以上ない
        if len(batch) < DefaultActivityPageLimit {
            break
        }

        // まだページが続くが上限に達した
        if page == opt.MaxActivityPages-1 {
            truncated = true
            break
        }

        // 次ページ: 最後の activity の ID - 1 を maxId に設定
        lastID := batch[len(batch)-1].ID
        maxId = int(lastID) - 1
        if maxId <= 0 {
            break
        }
    }

    return result, truncated, nil
}
```

### 5. `activity_stats.go` 期間フィルタ修正（不具合2の修正）

`Build()` の API 呼び出し部分から `Since`/`Until` を削除し、取得後にクライアント側でフィルタリングする。

```go
// project scope の例
acts, err := b.client.ListProjectActivities(ctx, opt.ScopeKey, backlog.ListActivitiesOptions{
    Count: 100,  // max 件数のみ指定（since/until は削除）
})
if err != nil {
    // ...
} else {
    // クライアント側で期間フィルタ
    activities = filterActivitiesByDateRange(acts, since, until)
}
```

ただし 1 回の API 呼び出し（最大 100 件）では since/until の範囲外のデータが含まれる可能性があるため、
ページネーション対応も同時に追加することが望ましい。

**ActivityStats のページネーション戦略:**

`digest/activity_filter.go` の `FetchActivitiesWithDateFilter` パターンを参照し、
`activityStatsFetch()` ヘルパー関数を `activity_stats.go` 内に追加する。

```go
// fetchActivitiesForStats は maxId ベースでページングしつつ since/until でローカルフィルタする。
func (b *ActivityStatsBuilder) fetchActivitiesForStats(
    ctx context.Context,
    fetcher func(ctx context.Context, maxId int) ([]domain.Activity, error),
    since, until time.Time,
) ([]domain.Activity, error) {
    var result []domain.Activity
    maxId := 0

    for {
        batch, err := fetcher(ctx, maxId)
        if err != nil {
            return nil, err
        }
        if len(batch) == 0 {
            break
        }

        reachedSince := false
        for _, a := range batch {
            if a.Created != nil {
                if a.Created.Before(since) {
                    reachedSince = true
                    break
                }
                if a.Created.After(until) {
                    continue
                }
            }
            result = append(result, a)
        }

        if reachedSince || len(batch) < 100 {
            break
        }

        lastID := batch[len(batch)-1].ID
        maxId = int(lastID) - 1
        if maxId <= 0 {
            break
        }
    }

    return result, nil
}
```

### 6. `cli/activity.go` の修正

`Offset` フィールドを削除し、`Since` は API には送らない。
`ActivityListCmd` の Kong 定義から `Offset` フラグも削除するか、廃止通知に変更する。

```go
// Before
opt := backlog.ListActivitiesOptions{
    Limit:  c.Count,
    Offset: c.Offset,
}
if c.Since != "" {
    t, parseErr := time.Parse(time.RFC3339, c.Since)
    if parseErr == nil {
        opt.Since = &t
    }
}

// After: API には count のみ渡す（since フィルタはクライアント側対応 or 廃止）
opt := backlog.ListActivitiesOptions{
    Count: c.Count,
}
// Since/Until はクライアント側フィルタに変更（または機能を削除してシンプルに保つ）
```

`Since` フィルタの扱いは以下のいずれかを採用:
- **A（シンプル）**: `--since` フラグを削除し、最新 N 件取得に絞る
- **B（機能保持）**: 取得後にクライアント側でフィルタリング（ページネーション込み）

推奨: **オプション A（シンプル）**。CLI の `activity list` は生データ取得コマンドであり、
期間フィルタが必要な場合は `activity stats` コマンドの方が適切。

### 7. `mcp/tools_activity.go` の修正

`offset` パラメータの登録と利用を削除する。`maxId` ベースのページネーションは内部実装のため MCP ツールの引数としては公開しない。

```go
// Before
r.Register(gomcp.NewTool("logvalet_activity_list",
    gomcp.WithDescription("List space activities"),
    gomcp.WithNumber("limit", gomcp.Description("Max number of activities (default 20, max 100)")),
    gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
    opt := backlog.ListActivitiesOptions{}
    if limit, ok := intArg(args, "limit"); ok && limit > 0 {
        opt.Limit = limit
    }
    if offset, ok := intArg(args, "offset"); ok {
        opt.Offset = offset  // 削除
    }
    return client.ListSpaceActivities(ctx, opt)
})

// After
r.Register(gomcp.NewTool("logvalet_activity_list",
    gomcp.WithDescription("List space activities"),
    gomcp.WithNumber("count", gomcp.Description("Max number of activities (default 20, max 100)")),
), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
    opt := backlog.ListActivitiesOptions{}
    if count, ok := intArg(args, "count"); ok && count > 0 {
        opt.Count = count
    }
    return client.ListSpaceActivities(ctx, opt)
})
```

## テスト修正方針

### `internal/backlog/options_test.go`

`TestListActivitiesOptions` / `TestListUserActivitiesOptions` を新フィールドに合わせて更新:
- `Since` / `Until` / `Offset` のテストケースを削除
- `MinId` / `MaxId` / `Count` / `Order` のテストケースを追加

### `internal/analysis/timeline_test.go`

`fetchActivities()` のページネーションテストは既に `callCount` ベースで実装されているため、
mock の呼び出し回数の検証はそのまま使えるが、`opt.Offset` を検証しているテストは修正が必要。
`opt.MaxId` に基づいたページネーション検証に変更する。

全テストケース (~20 件) は MockClient を使用しているため API パラメータの変更の影響は最小限。
`ListActivitiesOptions` の型変更に伴うコンパイルエラーを修正するのみ。

### `internal/analysis/activity_stats_test.go`

`Since`/`Until` フィルタのテスト (EC05, EC06 等) は、クライアント側フィルタリングに変更後も
同じ入出力で検証できる（フィルタ処理が API 側からローカル側に移動するだけ）。

mock の `ListProjectActivitiesFunc` / `ListSpaceActivitiesFunc` / `ListUserActivitiesFunc` に
`Since`/`Until` を含まない activities を返すよう修正不要（既にローカルフィルタの前提で書かれているため）。

### `internal/digest/` テスト

`digest/activity.go` と `digest/unified.go` は既に `maxId` ベースの実装を使っているため、
`ListActivitiesOptions` の型変更に伴うコンパイルエラーのみ修正。

## 影響範囲の確認

### `ListActivitiesOptions` を使用している箇所

| ファイル | 使用箇所 | 修正要否 |
|---------|---------|---------|
| `internal/backlog/http_client.go` | `ListProjectActivities()`, `ListSpaceActivities()` | 要 |
| `internal/backlog/mock_client.go` | シグネチャ（型のみ参照） | コンパイル通れば不要 |
| `internal/analysis/timeline.go` | `fetchActivities()` | 要（不具合1の本体） |
| `internal/analysis/activity_stats.go` | `Build()` 3箇所 | 要（不具合2の本体） |
| `internal/analysis/timeline_test.go` | mock の `ListProjectActivitiesFunc` | 型変更のみ対応 |
| `internal/analysis/activity_stats_test.go` | mock の各 `ListActivitiesFunc` | 型変更のみ対応 |
| `internal/cli/activity.go` | `Run()` | 要（`Offset`/`Since` 削除） |
| `internal/mcp/tools_activity.go` | `logvalet_activity_list` | 要（`Offset` 削除） |
| `internal/mcp/tools_analysis_test.go` | mock の `ListActivitiesFunc` | 型変更のみ対応 |
| `internal/digest/activity.go` | `Build()` 2箇所 | 要（`Since`/`Until`/`Limit` → `Count`） |
| `internal/digest/unified.go` | `fetchProjectActivities()`, `fetchSpaceActivities()` 内 fetcher | 要（`Limit` → `Count`） |
| `internal/digest/unified_test.go` | mock の `ListActivitiesFunc` | 型変更のみ対応 |
| `internal/digest/activity_test.go` | mock の `ListActivitiesFunc` | 型変更のみ対応 |
| `internal/digest/digest_cmd_test.go` | mock の `ListActivitiesFunc` | 型変更のみ対応 |

### `ListUserActivitiesOptions` を使用している箇所

| ファイル | 使用箇所 | 修正要否 |
|---------|---------|---------|
| `internal/backlog/http_client.go` | `ListUserActivities()` | 要 |
| `internal/analysis/activity_stats.go` | `Build()` の user scope | 要（`Since`/`Until` 削除） |
| `internal/analysis/activity_stats_test.go` | mock の `ListUserActivitiesFunc` | 型変更のみ対応 |
| `internal/cli/user.go` | `UserActivityCmd.Run()` | 要（`Since`/`Until` 削除） |
| `internal/digest/unified.go` | `fetchUserActivities()` 内 fetcher | 要（`Limit` → `Count`） |

## 実装順序

以下の順序で実装することで、コンパイルエラーが発生せず段階的に修正できる:

1. `internal/backlog/options.go` — 型定義の修正（全ての変更の起点）
2. `internal/backlog/http_client.go` — クエリパラメータ変換の修正
3. `internal/backlog/options_test.go` — オプションテストの更新
4. `internal/digest/activity.go` — `Since`/`Until`/`Limit` → `Count` 対応（コンパイルエラー修正）
5. `internal/digest/unified.go` — `Limit` → `Count` 対応（コンパイルエラー修正）
6. `internal/cli/activity.go` — `Offset`/`Since` の削除（コンパイルエラー修正）
7. `internal/cli/user.go` — `Since`/`Until` の削除（コンパイルエラー修正）
8. `internal/mcp/tools_activity.go` — `Offset` の削除（コンパイルエラー修正）
9. `internal/analysis/timeline.go` — `fetchActivities()` の `maxId` ベースへの変更（不具合1修正）
10. `internal/analysis/activity_stats.go` — クライアント側フィルタリングへの変更（不具合2修正）
11. 全テストのコンパイルエラー修正・更新
12. `go test ./...` で全テストパス確認

## B. 完了課題フィルタ不具合の修正

### 現状の問題

`StaleConfig.ExcludeStatus`, `BlockerConfig.ExcludeStatus`, `WorkloadConfig.ExcludeStatus` のデフォルトが空スライス。
CLI のフラグ `--exclude-status` もデフォルト空文字。結果、完了課題がフィルタされずに含まれる。

### 修正方針: デフォルトで「完了」を除外

Backlog のデフォルトステータスは「未対応」「処理中」「処理済み」「完了」の 4 つ。
プロジェクトによっては「対応済み」「クローズ」等のカスタムステータスもある。

**方針**: 各 Builder の Config で `ExcludeStatus` が空の場合、デフォルトで `["完了"]` を適用する。
ユーザーが `--exclude-status ""` と空文字を明示した場合のみ全ステータスを含める。

### 修正対象ファイル

| ファイル | 修正内容 |
|---------|---------|
| `internal/analysis/stale.go` | `Detect()` 冒頭で `ExcludeStatus` が空なら `["完了"]` をデフォルト設定 |
| `internal/analysis/blocker.go` | `Detect()` 冒頭で `ExcludeStatus` が空なら `["完了"]` をデフォルト設定 |
| `internal/analysis/workload.go` | `Calculate()` 冒頭で `ExcludeStatus` が空なら `["完了"]` をデフォルト設定 |
| `internal/analysis/health.go` | 上記 3 Builder の Config に `ExcludeStatus` を連携（変更不要の可能性あり） |
| `internal/cli/issue_stale.go` | `--exclude-status` のヘルプテキストにデフォルト値を明記 |
| `internal/cli/project_blockers.go` | 同上 |
| `internal/cli/user_workload.go` | 同上 |
| `internal/cli/project_health.go` | 同上 |

### 共通定数の追加

```go
// internal/analysis/analysis.go に追加
var DefaultExcludeStatus = []string{"完了"}
```

### 各 Builder での適用パターン

```go
// stale.go の Detect() 冒頭
if len(config.ExcludeStatus) == 0 {
    config.ExcludeStatus = DefaultExcludeStatus
}
```

### テスト修正

既存テストの多くは `ExcludeStatus: []string{}` を明示的に渡しているため影響は限定的。
ただし、デフォルト動作のテストケースを追加する:

| テスト | 検証内容 |
|--------|---------|
| `TestStaleIssueDetector_DefaultExcludeStatus` | ExcludeStatus 未指定時に「完了」課題が除外される |
| `TestBlockerDetector_DefaultExcludeStatus` | 同上 |
| `TestWorkloadCalculator_DefaultExcludeStatus` | 同上 |

---

## 受け入れ条件

### A. Activity API パラメータ
- [ ] `logvalet issue timeline ESU2_S2-1` が `unknownParameter: offset` エラーなく動作する
- [ ] `logvalet activity stats --scope project -k ESU2_S2` が `unknownParameter: since` エラーなく動作する
- [ ] `ListActivitiesOptions` に `Offset`/`Since`/`Until` フィールドが存在しない
- [ ] `ListUserActivitiesOptions` に `Offset`/`Since`/`Until` フィールドが存在しない
- [ ] ページネーションが `maxId` ベースで動作する（timeline, activity_stats 両方）
- [ ] 期間フィルタがクライアント側で実施される（timeline, activity_stats 両方）

### B. 完了課題フィルタ
- [ ] `logvalet issue stale -k ESU2_S2` がステータス「完了」の課題を含まない
- [ ] `logvalet project blockers ESU2_S2` がステータス「完了」の課題を含まない
- [ ] `logvalet project health ESU2_S2` の score が完了課題を除外した値になる
- [ ] `logvalet user workload ESU2_S2` の total が完了課題を除外した値になる
- [ ] `--exclude-status "完了,処理済み"` で追加の除外が可能

### 共通
- [ ] `go test ./...` が全てパスする
- [ ] `go vet ./...` がエラーなし

## 備考

### `digest/` との設計の統一

`digest/unified.go` は既に正しい `FetchActivitiesWithDateFilter` + `maxId` パターンを実装している。
`analysis/` パッケージでも同様のパターンを採用することで設計の一貫性を保つ。

将来的には `digest.FetchActivitiesWithDateFilter` を `internal/backlog` または `internal/util` に
移動してパッケージ間で共有することを検討できるが、今回のスコープ外とする。

### `ActivityStatsBuilder` のページネーション深さ

`activity_stats.go` は期間が長い場合（例: 1ヶ月分）に多数のページが必要になる可能性がある。
デフォルトでは `FetchActivitiesWithDateFilter` 相当の 10,000 件上限を設ける。
`ActivityStatsOptions` に `MaxActivities int` フィールドを追加することで呼び出し側で制御可能にする。
