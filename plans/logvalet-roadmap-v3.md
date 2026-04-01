# logvalet Roadmap v3: AI-Native Intelligence Layer

> Phase 1〜3 で logvalet を Backlog 向け AI ネイティブ基盤に進化させるロードマップ。

## 概要

logvalet v0.5.0（M01-M18）は基本 CRUD + Digest + MCP が完備済み。
本ロードマップでは、AI が Backlog を高精度に扱うための **分析・ワークフロー・インテリジェンス** 機能を段階的に追加する。

**原則:**
- deterministic な分析を優先（LLM 依存は後回し）
- JSON を正本、Markdown はレンダリング層
- CLI と MCP の二重実装を避ける（`internal/analysis/` で共通化）
- 1コマンド1意思決定
- TDD 必須（Red → Green → Refactor）

---

## 現状サマリ (v0.5.0)

### 完了済み機能
- issue / project / activity / user / document / team / space の CRUD
- 7種の Digest（Issue, Project, Activity, Document, User, Team, Space, Unified）
- MCP サーバー（23+ tools、Streamable HTTP）
- 出力フォーマット: JSON / YAML / Markdown / Gantt
- DigestEnvelope + LLMHints + Warning Envelope
- 共有ファイル / 課題添付 / スター (M17)

### 既知のバグ（Hotfix）

| M# | タイトル | 依存 | 概要 | Issue |
|----|---------|------|------|-------|
| **HF01** | Document.json フィールドの型不一致修正 | — | `document create` のレスポンスパースエラー。Backlog API が `json` フィールドをオブジェクトとして返すが、Go 構造体では `string` 型で定義されているため `json.Unmarshal` が失敗する。`json.RawMessage` に変更して修正。 | [#1](https://github.com/youyo/logvalet/issues/1) |

### 未実装（本ロードマップの対象）
- AI 分析機能（issue context, stale, blockers, workload）
- AI ワークフロー機能（draft comment, triage, periodic digest）
- Intelligence 機能（decision log, anomaly detection, risk summary）

---

## 目標アーキテクチャ

### 新パッケージ: `internal/analysis/`

`digest/`（what: 何があるか）とは別に `analysis/`（so what: だから何か）を新設。

```
internal/analysis/
  analysis.go         — 共通型（AnalysisEnvelope, BaseAnalysisBuilder）
  context.go          — IssueContextBuilder
  stale.go            — StaleIssueDetector
  blocker.go          — BlockerDetector
  workload.go         — WorkloadCalculator
  health.go           — ProjectHealthBuilder（統合）
  draft.go            — DraftCommentBuilder (Phase 2)
  triage.go           — TriageAnalyzer (Phase 2)
  periodic.go         — PeriodicDigestBuilder (Phase 2)
  decision.go         — DecisionLogExtractor (Phase 3)
  intelligence.go     — ActivityIntelligence (Phase 3)
  risk.go             — RiskSummaryBuilder (Phase 3)
```

### AnalysisEnvelope（DigestEnvelope と同構造）

```json
{
  "schema_version": "1",
  "resource": "issue_context",
  "generated_at": "2026-04-01T12:00:00Z",
  "profile": "default",
  "space": "heptagon",
  "base_url": "https://heptagon.backlog.com",
  "warnings": [],
  "analysis": { ... }
}
```

### コマンド配置

既存サブコマンドの下に配置（新トップレベルコマンドは作らない）:

| 機能 | コマンド |
|------|---------|
| Issue Context | `logvalet issue context PROJ-123` |
| Stale Issues | `logvalet issue stale -k PROJECT` |
| Project Blockers | `logvalet project blockers PROJECT` |
| User Workload | `logvalet user workload [USER]` |
| Project Health | `logvalet project health PROJECT` |
| Draft Comment | `logvalet issue draft-comment PROJ-123` |
| Issue Triage | `logvalet issue triage PROJ-123` |
| Weekly Digest | `logvalet digest weekly -k PROJECT` |
| Daily Digest | `logvalet digest daily -k PROJECT` |

---

## Phase 1: AI ネイティブ操作層 (M20-M32)

### フェーズ目的

AI が issue / project / user を扱う際、一発で判断材料を得られるようにする。
全て deterministic（LLM 不要）。

### マイルストーン一覧

| M# | タイトル | 依存 | 概要 |
|----|---------|------|------|
| **M20** | analysis 基盤 + IssueContext ロジック | — | 共通型、BaseAnalysisBuilder、IssueContextBuilder |
| **M21** | IssueContext CLI コマンド | M20 | `issue context` コマンド + フラグ + help |
| **M22** | IssueContext MCP ツール | M21 | `logvalet_issue_context` MCP tool |
| **M23** | StaleIssueDetector ロジック | M20 | 停滞検出アルゴリズム（デフォルト7日）+ 閾値設定 |
| **M24** | Stale Issues CLI + MCP | M23 | `issue stale` コマンド + MCP tool + help |
| **M25** | BlockerDetector ロジック | M20 | 進行阻害要因検出アルゴリズム |
| **M26** | Project Blockers CLI + MCP | M25 | `project blockers` コマンド + MCP tool + help |
| **M27** | WorkloadCalculator ロジック | M20 | 担当者負荷計算 |
| **M28** | User Workload CLI + MCP | M27 | `user workload` コマンド + MCP tool + help |
| **M29** | Enhanced Project Digest | M23,M25,M27 | 既存 project digest に stale/blocker/workload 統合 |
| **M30** | Project Health CLI + MCP | M29 | `project health` 統合ビュー |
| **M31** | Phase 1 スキル・ドキュメント整備 | M30 | README 更新、既存スキル更新、新スキル作成 |
| **M32** | Phase 1 E2E テスト + リリース | M31 | 参照系 E2E テスト、Phase 1 完了検証 |

### M31 スキル・ドキュメント詳細

**新規スキル（`skills/` ディレクトリ、plugin 配布用）:**

| スキル | トリガー |
|--------|---------|
| `logvalet-health` | "プロジェクトの状態", "project health", "ブロッカー", "停滞" |
| `logvalet-context` | "課題の詳細", "issue context", "コンテキスト" |

**既存スキル更新:**

| スキル | 変更内容 |
|--------|---------|
| `logvalet` | issue context, issue stale, project blockers, user workload, project health コマンド追加 |
| `logvalet-my-week` | stale/overdue signals を活用した追加情報表示 |
| `logvalet-my-next` | workload 情報の参照を追加 |
| `logvalet-report` | project health/blockers データの統合 |

**ドキュメント:** README.md, README.ja.md 更新

### Phase 1 完了条件

- [ ] `issue context` が1コマンドで課題の判断材料を返せる
- [ ] `issue stale` が停滞課題を検出できる（デフォルト7日閾値）
- [ ] `project blockers` がプロジェクトの進行阻害要因を抽出できる
- [ ] `user workload` が担当者の負荷状況を可視化できる
- [ ] `project health` が統合ビューを返せる
- [ ] 全機能が CLI + MCP 両方で利用可能
- [ ] JSON スキーマが安定（AnalysisEnvelope 統一）
- [ ] 全テストがパス（unit + E2E）
- [ ] README.md / README.ja.md に新コマンドを記載
- [ ] `logvalet` スキルに新コマンドを追加
- [ ] `logvalet-health` / `logvalet-context` 新スキル作成
- [ ] 既存スキル（logvalet-my-week, logvalet-my-next, logvalet-report）更新
- [ ] 全コマンドの help テキストが正確

---

## Phase 2: AI ワークフロー層 (M33-M41)

### フェーズ目的

Phase 1 で得られる判断材料をもとに、AI が実際の業務フローに入り込める操作を追加する。

### 設計方針: logvalet = 材料提供、SKILL = LLM 判断

**logvalet 本体は deterministic な材料提供に徹する。** LLM 判断が必要な機能は SKILL 側のプロンプトで吸収する。

| 機能 | logvalet（CLI/MCP） | SKILL（プロンプト） |
|------|---------------------|-------------------|
| DraftComment | 課題コンテキスト + コメント履歴を JSON で返す | 文脈に合ったコメントを LLM が下書き |
| IssueTriage | 課題属性・履歴・プロジェクト統計を JSON で返す | priority/assignee を LLM が提案 |
| Weekly/Daily Digest | 期間内の活動を集約して JSON で返す | — (deterministic で完結) |
| SpecToIssues | — (SKILL 完結) | spec.md を LLM が解析し課題に分解 |

**メリット:**
- logvalet に LLM 依存が入らない（テスタブル、deterministic 維持）
- SKILL のプロンプト改善だけで精度が上がる
- LLM のモデル進化の恩恵を自動的に受けられる

### マイルストーン一覧

| M# | タイトル | 依存 | 概要 |
|----|---------|------|------|
| **M33** | TriageMaterials ロジック | M20 | 課題属性・履歴・類似課題統計を構造化して返す材料収集器 |
| **M34** | TriageMaterials CLI + MCP | M33 | `issue triage-materials` コマンド + MCP tool |
| **M35** | Weekly/Daily Digest ロジック | M29 | 期間ベース集約 + completed/started/blocked (deterministic) |
| **M36** | Weekly/Daily Digest CLI + MCP | M35 | `digest weekly/daily` コマンド + MCP tool |
| **M37** | Phase 2 SKILL 作成 | M34,M36 | `logvalet-triage`, `logvalet-draft`, `logvalet-digest-periodic`, `logvalet-spec-to-issues` スキル作成 |
| **M38** | Phase 2 ドキュメント整備 | M37 | 既存スキル更新、README 更新 |
| **M39** | Phase 2 E2E テスト + リリース | M38 | Phase 2 完了検証 |

### M37 SKILL 詳細

**新規スキル（LLM 判断を含む）:**

| スキル | 材料源（logvalet） | LLM 判断内容 |
|--------|-------------------|-------------|
| `logvalet-triage` | `issue triage-materials` | priority/assignee/category の提案 |
| `logvalet-draft` | `issue context` + `issue get` | コメント下書き生成 |
| `logvalet-digest-periodic` | `digest weekly/daily` | サマリー・ハイライト抽出 |
| `logvalet-spec-to-issues` | — (SKILL 完結) | spec.md → 課題リスト分解 |

**既存スキル更新:** `logvalet`, `logvalet-issue-create`

### Phase 2 完了条件

- [ ] `issue triage-materials` が課題の判断材料を JSON で返せる
- [ ] `digest weekly/daily` が期間内活動を集約できる
- [ ] `logvalet-triage` SKILL が材料をもとに priority/assignee を提案できる
- [ ] `logvalet-draft` SKILL がコンテキストに基づくコメント下書きを生成できる
- [ ] `logvalet-digest-periodic` SKILL が定期 digest を生成できる
- [ ] `logvalet-spec-to-issues` SKILL が spec からの課題分解を支援できる
- [ ] 既存スキル更新
- [ ] README 更新

---

## Phase 3: Intelligence / 差別化層 (M40-M46)

### フェーズ目的

Backlog に標準では不足しがちな PM / intelligence 的価値を logvalet が補完する。
logvalet は材料（構造化データ）を提供し、SKILL が LLM で洞察を生成する。

### 設計方針

Phase 2 と同様、logvalet = 材料提供、SKILL = LLM 判断。

| 機能 | logvalet（CLI/MCP） | SKILL（プロンプト） |
|------|---------------------|-------------------|
| DecisionLog | コメント一覧 + 更新履歴を時系列 JSON で返す | LLM が意思決定に該当するものを抽出・要約 |
| ActivityIntelligence | アクティビティ統計・偏り・パターンを JSON で返す | LLM が異常・リスクを解釈 |
| RiskSummary | health + blocker + stale + workload を統合 JSON で返す | LLM がリスク評価・推奨アクションを生成 |

### マイルストーン一覧

| M# | タイトル | 依存 | 概要 |
|----|---------|------|------|
| **M40** | CommentTimeline ロジック | M20 | コメント + 更新履歴を時系列構造化（DecisionLog の材料） |
| **M41** | CommentTimeline CLI + MCP | M40 | `issue timeline` / `project timeline` コマンド + MCP tool |
| **M42** | ActivityStats ロジック | M20 | アクティビティ統計・偏り・パターンの数値集約 |
| **M43** | ActivityStats CLI + MCP | M42 | `activity stats` コマンド + MCP tool |
| **M44** | Phase 3 SKILL 作成 | M41,M43,M30 | `logvalet-decisions`, `logvalet-intelligence`, `logvalet-risk` スキル作成 |
| **M45** | Phase 3 ドキュメント最終整備 | M44 | 全スキル最終更新、README 完全更新 |
| **M46** | Phase 3 E2E テスト + 最終リリース | M45 | 全 Phase 完了検証 |

### M44 SKILL 詳細

**新規スキル（LLM 判断を含む）:**

| スキル | 材料源（logvalet） | LLM 判断内容 |
|--------|-------------------|-------------|
| `logvalet-decisions` | `issue timeline` + `project timeline` | コメント・履歴から意思決定を抽出・要約 |
| `logvalet-intelligence` | `activity stats` + `project health` | 偏り・異常の解釈、リスク評価 |
| `logvalet-risk` | `project health` + `project blockers` + `issue stale` | 統合リスク評価・推奨アクション生成 |

**最終更新:** 全スキル + README.md / README.ja.md

### Phase 3 完了条件

- [ ] `issue timeline` が時系列データを構造化して返せる
- [ ] `activity stats` がアクティビティ統計を返せる
- [ ] `logvalet-decisions` SKILL が意思決定ログを抽出できる
- [ ] `logvalet-intelligence` SKILL が異常・偏りを検出・解釈できる
- [ ] `logvalet-risk` SKILL が統合リスク評価を生成できる
- [ ] 全スキル最終更新
- [ ] README 完全更新

---

## MCP 反映方針

Phase 1-3 の分析機能を MCP tool として公開（材料提供のみ、LLM 判断は SKILL 側）:

| MCP Tool | Phase | 対応 M# | 役割 |
|----------|-------|--------|------|
| `logvalet_issue_context` | 1 | M22 | 課題の判断材料一括取得 |
| `logvalet_issue_stale` | 1 | M24 | 停滞課題検出 |
| `logvalet_project_blockers` | 1 | M26 | ブロッカー検出 |
| `logvalet_user_workload` | 1 | M28 | ユーザー負荷分析 |
| `logvalet_project_health` | 1 | M30 | プロジェクト健全性 |
| `logvalet_issue_triage_materials` | 2 | M34 | triage 用材料収集 |
| `logvalet_digest_weekly` | 2 | M36 | 週次活動集約 |
| `logvalet_digest_daily` | 2 | M36 | 日次活動集約 |
| `logvalet_issue_timeline` | 3 | M41 | コメント・履歴時系列 |
| `logvalet_activity_stats` | 3 | M43 | アクティビティ統計 |

`internal/mcp/tools_analysis.go` に集約。`RegisterAnalysisTools(reg)` を `server.go` に追加。

> **注意**: `logvalet_issue_triage`（判断）、`logvalet_project_risk`（評価）等の LLM 判断機能は MCP tool ではなく SKILL として提供する。

---

## テスト戦略

### Unit テスト（全マイルストーン）
- `backlog.MockClient` の Func フィールドパターン
- `now func() time.Time` による clock injection
- Table-driven tests
- JSON shape verification

### E2E テスト（参照系のみ、M32/M41/M49）
- 実 Backlog API に対する統合テスト
- `//go:build e2e` ビルドタグで分離
- 読み取り専用操作のみ

### Golden テスト
- analysis 出力の JSON snapshot を `testdata/` に保存

---

## リスクと対策

| リスク | 対策 |
|--------|------|
| N+1 API 呼び出し | ListIssues の assigneeId[] フィルタで一括取得 |
| stale 閾値の誤検出 | デフォルト7日 + `--days` で上書き可能 |
| blocker の false positive | signals 配列で根拠明示、severity フィルタ |
| analysis/ と digest/ の責務重複 | digest=要約(what)、analysis=洞察(so what) を明確分離 |
| LLM 判断の精度 | logvalet は材料提供に徹し、SKILL のプロンプトで吸収。モデル進化で自動改善 |
| SKILL と MCP tool の責務混同 | MCP tool = deterministic 材料、SKILL = LLM 判断と明確分離 |

---

## 実装順序の根拠

1. **M20 (IssueContext)** → 最も価値が高く、全後続機能の共通基盤
2. **Stale → Blocker → Workload** → 単純→複合の順で段階的に構築
3. **CLI → MCP** → CLI で動作検証してから MCP 反映
4. **各 Phase 末にスキル・ドキュメント整備** → 使える状態でリリース

> **注意**: `docs/specs/` は初期構想として保存。編集しない。スキルは全て `skills/` ディレクトリに配置（plugin 配布用）。
