# M44: Phase 3 SKILL 作成

## 概要

Phase 3 の新規コマンド（`issue timeline`, `activity stats`）を活用した Intelligence 系スキルを作成する。
logvalet = 材料提供、SKILL = LLM 判断の設計原則を維持する。

## 対象スキル

### 新規スキル

| スキル | 材料源 | LLM 判断内容 |
|--------|--------|-------------|
| `logvalet-decisions` | `issue timeline` + `project timeline` | コメント・履歴から意思決定を抽出・要約 |
| `logvalet-intelligence` | `activity stats` + `project health` | 偏り・異常の解釈、リスク評価 |
| `logvalet-risk` | `project health` + `project blockers` + `issue stale` | 統合リスク評価・推奨アクション生成 |

### 既存スキル更新

| スキル | 変更内容 |
|--------|---------|
| `logvalet` | `issue timeline`, `activity stats` コマンドを追加 |

## 設計方針

- logvalet は deterministic な材料提供に徹する
- SKILL プロンプトで LLM 判断を行う
- Phase 2 スキル（logvalet-triage, logvalet-health 等）のパターンに従う
- トリガーワードは日本語・英語の両方を含める

## スキル詳細設計

### logvalet-decisions

**目的:** 課題・プロジェクトの時系列データからチームの意思決定プロセスを可視化

**材料取得コマンド:**
```bash
lv issue timeline ISSUE_KEY -f json
```

**LLM が行う判断:**
- どのコメント・更新が意思決定を示しているか
- 意思決定の文脈・背景・理由の抽出
- 決定に関与したステークホルダー
- 決定のタイムライン（いつ、誰が、何を決めたか）

**トリガー:** "意思決定", "decision log", "決定履歴", "なぜこうなったか", "経緯", "決定の背景", "decision history"

### logvalet-intelligence

**目的:** アクティビティ統計からプロジェクトの異常・偏り・リスクを検出・解釈

**材料取得コマンド:**
```bash
lv activity stats --scope project -k PROJECT_KEY -f json
lv project health PROJECT_KEY -f json
```

**LLM が行う判断:**
- アクティビティパターンの異常検知（急増・急減）
- 特定ユーザーへの偏りの解釈
- リスク評価（停滞・過負荷・孤立メンバー等）

**トリガー:** "アクティビティ分析", "activity intelligence", "異常検知", "偏り", "アクティビティの偏り", "activity anomaly", "activity pattern"

### logvalet-risk

**目的:** プロジェクト全体の健全性・ブロッカー・停滞課題を統合してリスク評価と推奨アクションを生成

**材料取得コマンド:**
```bash
lv project health PROJECT_KEY -f json
lv project blockers PROJECT_KEY -f json
lv issue stale -k PROJECT_KEY -f json
```

**LLM が行う判断:**
- 統合リスクスコアリング
- リスク優先順位付け
- 具体的な推奨アクション生成
- リスクの依存関係・連鎖の分析

**トリガー:** "リスク評価", "risk summary", "プロジェクトリスク", "リスク分析", "risk assessment", "project risk"

## 実装ファイル

```
skills/logvalet-decisions/SKILL.md  (新規)
skills/logvalet-intelligence/SKILL.md  (新規)
skills/logvalet-risk/SKILL.md  (新規)
skills/logvalet/SKILL.md  (更新: issue timeline, activity stats 追加)
```

## チェックリスト

- [ ] `skills/logvalet-decisions/SKILL.md` 作成
- [ ] `skills/logvalet-intelligence/SKILL.md` 作成
- [ ] `skills/logvalet-risk/SKILL.md` 作成
- [ ] `skills/logvalet/SKILL.md` 更新（Phase 3 コマンド追加）
- [ ] コミット: `feat(skills): Phase 3 SKILL を作成 (M44)`
