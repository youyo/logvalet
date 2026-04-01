# M32: Phase 1 E2E テスト + リリース

## 概要

Phase 1（M20-M31）の全分析機能について E2E テストを作成し、Phase 1 完了条件を検証する。

## 目標

1. 参照系 E2E テストの作成（`//go:build e2e` ビルドタグで分離）
2. go test ./... パス確認（unit テスト）
3. go vet ./... パス確認
4. Phase 1 完了条件チェックリストの更新

## スコープ

### E2E テスト対象コマンド

| コマンド | テストファイル | 分析器 |
|---------|-------------|--------|
| `issue context PROJ-1` | `internal/e2e/issue_context_e2e_test.go` | IssueContextBuilder |
| `issue stale -k PROJECT` | `internal/e2e/issue_stale_e2e_test.go` | StaleIssueDetector |
| `project blockers PROJECT` | `internal/e2e/project_blockers_e2e_test.go` | BlockerDetector |
| `user workload PROJECT` | `internal/e2e/user_workload_e2e_test.go` | WorkloadCalculator |
| `project health PROJECT` | `internal/e2e/project_health_e2e_test.go` | ProjectHealthBuilder |

### 環境変数

```
LOGVALET_E2E_API_KEY     — Backlog APIキー（必須）
LOGVALET_E2E_SPACE       — Backlogスペース名（例: heptagon）
LOGVALET_E2E_PROJECT_KEY — テスト用プロジェクトキー（例: TEST）
LOGVALET_E2E_ISSUE_KEY   — テスト用課題キー（例: TEST-1）
```

### E2E テストの方針

- `//go:build e2e` ビルドタグで分離（通常の `go test ./...` では実行されない）
- 環境変数未設定時は `t.Skip()` でスキップ
- 読み取り専用操作のみ
- AnalysisEnvelope の基本構造を確認（schema_version, resource, generated_at 等）
- analysis フィールドが nil でないことを確認
- エラーが返らないことを確認

## 実装ステップ

### Step 1: E2E テストディレクトリ作成

`internal/e2e/` ディレクトリを作成し、各コマンドの E2E テストファイルを配置。

### Step 2: E2E テスト共通ヘルパー作成

`internal/e2e/helpers_test.go`:
- 環境変数読み込みヘルパー
- Backlog クライアント生成ヘルパー
- スキップ条件チェックヘルパー

### Step 3: 各 E2E テスト実装

各テストは以下の構造:
1. 環境変数チェック（未設定なら `t.Skip()`）
2. Backlog クライアント生成
3. 分析器生成と実行
4. AnalysisEnvelope 基本検証
5. analysis フィールド存在確認

### Step 4: ロードマップ更新

`plans/logvalet-roadmap-v3.md` の Phase 1 完了条件チェックボックスを更新。

## Phase 1 完了条件チェックリスト

- [x] `issue context` が1コマンドで課題の判断材料を返せる（M20-M22 完了）
- [x] `issue stale` が停滞課題を検出できる（デフォルト7日閾値）（M23-M24 完了）
- [x] `project blockers` がプロジェクトの進行阻害要因を抽出できる（M25-M26 完了）
- [x] `user workload` が担当者の負荷状況を可視化できる（M27-M28 完了）
- [x] `project health` が統合ビューを返せる（M29-M30 完了）
- [x] 全機能が CLI + MCP 両方で利用可能（各 M の MCP 実装済み）
- [x] JSON スキーマが安定（AnalysisEnvelope 統一）
- [ ] 全テストがパス（unit + E2E）← M32 で E2E テスト追加
- [x] README.md / README.ja.md に新コマンドを記載（M31 完了）
- [x] `logvalet` スキルに新コマンドを追加（M31 完了）
- [x] `logvalet-health` / `logvalet-context` 新スキル作成（M31 完了）
- [x] 既存スキル（logvalet-my-week, logvalet-my-next, logvalet-report）更新（M31 完了）
- [x] 全コマンドの help テキストが正確

## 既知の事項

- `internal/backlog` と `internal/credentials` のテストは httptest のポートバインドエラーで失敗する（サンドボックス環境の制限）。
  - これは M32 のスコープ外。実際の CI/CD 環境では正常動作する。
- E2E テストは `//go:build e2e` タグで分離するため、通常の `go test ./...` には影響しない。
