# M27: WorkloadCalculator ロジック — 詳細実装計画

## 概要

WorkloadCalculator は Backlog プロジェクト内のユーザーごとの課題負荷（ワークロード）を計算する分析機能。
`internal/analysis/workload.go` に実装し、CLI（M28）・MCP（M28）の共通ロジック層を提供する。

## 目標

- ユーザーごとにアサインされた課題の負荷を数値・カテゴリで可視化
- `analysis/` パッケージの既存パターン（stale.go, blocker.go）に完全準拠
- TDD（Red → Green → Refactor）で実装

---

## 出力 JSON 構造

```json
{
  "schema_version": "1",
  "resource": "user_workload",
  "generated_at": "2026-04-01T12:00:00Z",
  "profile": "default",
  "space": "heptagon",
  "base_url": "https://heptagon.backlog.com",
  "warnings": [],
  "analysis": {
    "project_key": "PROJ",
    "total_issues": 50,
    "unassigned_count": 5,
    "stale_threshold_days": 7,
    "members": [
      {
        "user_id": 123,
        "name": "User A",
        "total": 10,
        "by_status": {"未対応": 3, "処理中": 5, "完了": 2},
        "by_priority": {"高": 2, "中": 5, "低": 3},
        "overdue": 1,
        "stale": 2,
        "load_level": "high"
      }
    ],
    "llm_hints": {
      "primary_entities": ["project:PROJ"],
      "open_questions": ["..."],
      "suggested_next_actions": []
    }
  }
}
```

## 負荷レベル判定基準（load_level）

| レベル | 条件 |
|--------|------|
| `overloaded` | total >= 20 |
| `high` | total >= 10 |
| `medium` | total >= 5 |
| `low` | total < 5 |

閾値は `WorkloadConfig` で上書き可能。

---

## 実装ファイル

### 新規作成
- `internal/analysis/workload.go` — WorkloadCalculator 本体
- `internal/analysis/workload_test.go` — TDD テスト

### 変更なし（参照のみ）
- `internal/analysis/analysis.go` — BaseAnalysisBuilder, AnalysisEnvelope
- `internal/analysis/stale.go` — fetchProjectIssues パターン参照
- `internal/analysis/blocker.go` — 最新パターン参照

---

## 型定義

```go
// WorkloadConfig はワークロード計算の設定。
type WorkloadConfig struct {
    StaleDays        int      // stale 判定の閾値（日数）。0以下の場合 DefaultStaleDays を使用。
    ExcludeStatus    []string // 除外ステータス名（例: "完了", "対応済み"）
    // 負荷レベル閾値（0以下の場合デフォルト値を使用）
    OverloadedThreshold int   // デフォルト: 20
    HighThreshold       int   // デフォルト: 10
    MediumThreshold     int   // デフォルト: 5
}

// WorkloadResult はワークロード計算の結果。
type WorkloadResult struct {
    ProjectKey      string              `json:"project_key"`
    TotalIssues     int                 `json:"total_issues"`
    UnassignedCount int                 `json:"unassigned_count"`
    StaleDays       int                 `json:"stale_threshold_days"`
    Members         []MemberWorkload    `json:"members"`
    LLMHints        digest.DigestLLMHints `json:"llm_hints"`
}

// MemberWorkload は個別メンバーのワークロード情報。
type MemberWorkload struct {
    UserID     int            `json:"user_id"`
    Name       string         `json:"name"`
    Total      int            `json:"total"`
    ByStatus   map[string]int `json:"by_status"`
    ByPriority map[string]int `json:"by_priority"`
    Overdue    int            `json:"overdue"`
    Stale      int            `json:"stale"`
    LoadLevel  string         `json:"load_level"` // "low" | "medium" | "high" | "overloaded"
}

// WorkloadCalculator はユーザーごとの課題負荷を計算する。
type WorkloadCalculator struct {
    BaseAnalysisBuilder
}
```

---

## メソッド設計

```go
// NewWorkloadCalculator は WorkloadCalculator を生成する。
func NewWorkloadCalculator(client backlog.Client, profile, space, baseURL string, opts ...Option) *WorkloadCalculator

// Calculate は指定プロジェクトのワークロードを計算する。
// projectKey が空の場合は全プロジェクトを対象とする（将来拡張）。
func (c *WorkloadCalculator) Calculate(ctx context.Context, projectKey string, config WorkloadConfig) (*AnalysisEnvelope, error)
```

---

## 実装ロジック

### Calculate の処理フロー

1. `config` デフォルト値解決（StaleDays, 閾値群）
2. `GetProject(projectKey)` でプロジェクト取得（エラー時は warning として返す）
3. `ListIssues({ProjectIDs: [project.ID]})` で全課題取得
4. ExcludeStatus を set 化
5. 課題を走査してメンバー別集計
   - `assignee == nil` → `UnassignedCount++`
   - assignee ごとに `MemberWorkload` を集計（map[int]*MemberWorkload）
   - `ByStatus[statusName]++`
   - `ByPriority[priorityName]++`
   - IsOverdue 判定: `dueDate != nil && dueDate.Before(now)`
   - IsStale 判定: `updated != nil && now.Sub(*updated).Hours()/24 >= staleDays`
6. MemberWorkload スライス化 → UserID 昇順ソート
7. 各メンバーの `LoadLevel` を計算
8. `WorkloadResult` 組み立て
9. `newEnvelope("user_workload", result, warnings)` を返す

### loadLevel 計算

```go
func calcLoadLevel(total, overloaded, high, medium int) string {
    switch {
    case total >= overloaded:
        return "overloaded"
    case total >= high:
        return "high"
    case total >= medium:
        return "medium"
    default:
        return "low"
    }
}
```

---

## TDD 実装ステップ

### Step 1: Red — 失敗するテストを書く

`workload_test.go` に以下のテストケースを記述（実装前）:

1. `TestWorkloadCalculator_Calculate_Basic` — 基本ケース（3メンバー、異なる負荷）
2. `TestWorkloadCalculator_Calculate_Unassigned` — 担当者なし課題のカウント
3. `TestWorkloadCalculator_Calculate_ExcludeStatus` — 完了ステータス除外
4. `TestWorkloadCalculator_Calculate_OverdueAndStale` — 期限超過・停滞の検出
5. `TestWorkloadCalculator_Calculate_LoadLevel` — 負荷レベル判定（各閾値境界値）
6. `TestWorkloadCalculator_Calculate_ProjectFetchError` — プロジェクト取得失敗時の warning
7. `TestWorkloadCalculator_Calculate_IssuesFetchError` — 課題取得失敗時の warning
8. `TestWorkloadCalculator_Calculate_EmptyProject` — 課題0件

### Step 2: Green — 最小実装

`workload.go` を実装してテストを全て通す。

### Step 3: Refactor

- 重複ロジックを helper 関数に抽出
- コメント整備
- `go vet ./...` パス確認

---

## テストの MockClient 設計

```go
// テスト内で backlog.MockClient を使用（既存パターン）
mock := &backlog.MockClient{
    GetProjectFunc: func(ctx context.Context, key string) (*domain.Project, error) {
        return &domain.Project{ID: 1, ProjectKey: key}, nil
    },
    ListIssuesFunc: func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
        return testIssues, nil
    },
}
```

---

## 定数定義

```go
const (
    DefaultWorkloadStaleDays        = 7   // stale 判定デフォルト閾値（stale.go と同値）
    DefaultOverloadedThreshold      = 20
    DefaultHighThreshold            = 10
    DefaultMediumThreshold          = 5
)
```

---

## 完了条件

- [ ] `internal/analysis/workload.go` が実装済み
- [ ] `internal/analysis/workload_test.go` が全テスト green
- [ ] `go test ./internal/analysis/...` がパス
- [ ] `go vet ./...` がパス
- [ ] `resource: "user_workload"` の AnalysisEnvelope が正しく返される
- [ ] ExcludeStatus が機能する
- [ ] LoadLevel が正しく計算される
- [ ] Overdue / Stale カウントが正しい
- [ ] 未アサイン課題が UnassignedCount に反映される

---

## リスクと対策

| リスク | 対策 |
|--------|------|
| N+1 API 呼び出し | `ListIssues` の `ProjectIDs` フィルタで一括取得（課題ごとの API 呼び出しなし） |
| 停滞判定の重複 | StaleIssueDetector と同じ判定ロジックを使うが、独立実装（依存関係なし） |
| 負荷閾値の主観性 | `WorkloadConfig` で全閾値を上書き可能にする |
| メンバーソート順 | UserID 昇順で deterministic にする |

---

## 実装スケジュール

1. `workload_test.go` を書く（Red）
2. `workload.go` を実装（Green）
3. リファクタリング（Refactor）
4. `go test ./...` & `go vet ./...` で確認
5. git commit

---

## 次マイルストーン（M28）への引き渡し情報

- `WorkloadCalculator.Calculate(ctx, projectKey, config)` が `*AnalysisEnvelope` を返す
- CLI コマンド: `logvalet user workload [PROJECT_KEY]`
- MCP ツール名: `logvalet_user_workload`（`internal/mcp/tools_analysis.go` に追加）
- MCP パラメータはカンマ区切り文字列方式（`stringArg + strings.Split` パターン）
