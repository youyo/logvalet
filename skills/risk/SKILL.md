---
name: logvalet:risk
description: >
  Generate an integrated risk assessment for a Backlog project: combining project health,
  blockers, stale issues, and workload data — the LLM produces risk ratings, root cause analysis,
  and prioritized recommended actions.
  TRIGGER when: user says "リスク評価", "risk summary", "プロジェクトリスク", "リスク分析",
  "リスクは何", "対策は", "risk assessment", "リスクを洗い出して",
  "プロジェクトの懸念事項", "危険な兆候", "リスクマネジメント",
  "what are the risks", "risk analysis", "プロジェクトの課題を特定",
  "改善すべき点", "問題点の洗い出し", "risk mitigation",
  "推奨アクション", "次にやるべき対策", "プロジェクトの危機管理".
  DO NOT TRIGGER when: user wants a quick health score only (use health)
  or wants activity pattern analysis without risk interpretation (use intelligence).
  Workflow: Automatically gathers health + blockers + stale data before analysis.
---

# logvalet-risk

`lv project health`, `lv project blockers`, `lv issue stale` を統合し、LLM がプロジェクトの統合リスク評価・推奨アクションを生成する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-risk` when you need to:

- assess the overall risk posture of a project at a glance
- identify the most critical risks and their interdependencies
- generate prioritized recommended actions for a project review or sprint planning
- create a risk report for stakeholders
- proactively surface escalation-worthy issues before they become blockers

---

## Workflow

### Step 1: Identify project

If the user provides a project key, use it directly.

If not provided, list available projects:

```bash
lv project list -f md
```

Then ask the user to select a project.

### Step 2: Determine analysis parameters

Ask in a single question (if not already specified):

- `--days`: 停滞とみなす基準日数（デフォルト: 7）
- `--exclude-status`: 除外するステータス（例: "完了,却下"）

If the user wants a quick risk overview, use defaults.

### Step 3: Fetch risk materials in parallel

Run all three commands **in parallel**:

```bash
lv project health PROJECT_KEY --days DAYS [--exclude-status "STATUS1,STATUS2"] -f json
```

```bash
lv project blockers PROJECT_KEY --days DAYS [--exclude-status "STATUS1,STATUS2"] -f json
```

```bash
lv issue stale -k PROJECT_KEY --days DAYS [--exclude-status "STATUS1,STATUS2"] -f json
```

The materials provide:

**project health:**
- integrated view: stale count, blocker count, user workload distribution
- overall health signals

**project blockers:**
- `unassigned_high_priority`: 未アサインの高優先度課題
- `overdue_issues`: 期限超過課題
- `stale_high_priority`: 停滞している高優先度課題
- `severity` (high/medium/low)

**issue stale:**
- 停滞日数・最終更新日
- stale issues list with `days_stale`

### Step 4: Integrate and assess risk

Combine the three data sources to evaluate risk across dimensions:

**Dimension 1: Schedule Risk**
- Overdue issues count and severity
- Stale high-priority issues
- Issues with due dates approaching (next 7 days)

**Dimension 2: Resource Risk**
- Unassigned high-priority issues
- Workload imbalance (one member carries too many issues)
- Members with zero activity (may be blocked or unavailable)

**Dimension 3: Quality / Progress Risk**
- Long-stale issues (possible abandonment)
- High comment-to-resolution ratio (discussion without decisions)
- Repeated status resets

**Dimension 4: Risk Chains**
- Are any risks interdependent? (e.g., key person has all unassigned issues AND highest workload)
- Identify cascading risks if they exist

**Risk scoring:**
- Count issues with `severity: high` as high-risk signals
- Count issues with `severity: medium` as medium-risk signals
- Derive overall project risk level:
  - **Critical**: 3+ high-severity signals across multiple dimensions
  - **High**: 2+ high-severity signals or 1 high + 3+ medium
  - **Medium**: 1 high-severity signal or 3+ medium
  - **Low**: 0-2 medium signals

### Step 5: Present integrated risk report

```
## 統合リスク評価レポート — PROJECT_KEY

> 分析期間: DAYS日 / 生成日時: YYYY-MM-DD
> **総合リスクレベル: Critical / High / Medium / Low**

---

### リスクサマリー

| リスク次元 | 件数 | レベル |
|-----------|------|--------|
| スケジュールリスク | N件 | High/Medium/Low |
| リソースリスク | N件 | High/Medium/Low |
| 進捗リスク | N件 | High/Medium/Low |
| リスク連鎖 | N件 | High/Medium/Low |

---

### 高リスク項目（要即時対応）

1. **[課題キー] <課題サマリー>**
   - リスク種別: <スケジュール / リソース / 進捗>
   - 詳細: <なぜリスクか>
   - 推奨アクション: <具体的な次のステップ>

2. ...

---

### 中リスク項目（今週対応推奨）

1. **[課題キー] <課題サマリー>**
   - リスク種別: <種別>
   - 詳細: <なぜリスクか>
   - 推奨アクション: <具体的な次のステップ>

2. ...

---

### リスク連鎖

<特定された連鎖リスクがあれば記載。なければ「リスク連鎖なし」>

---

### 推奨アクション（優先順位付き）

1. **[即時]** <最優先アクション> — 担当: <UserName or 未定>
2. **[今日中]** <次に重要なアクション>
3. **[今週中]** <中優先アクション>
4. ...（最大7件）

---

**判断の根拠:**
- 分析したブロッカー: N件（高: N、中: N、低: N）
- 停滞課題: N件（最長 N日停滞）
- 未アサイン高優先度課題: N件
```

### Step 6: No writes

This is a read-only skill. No issue updates are performed.

After the user reviews the report, they can:

- use `logvalet-triage` to address individual unassigned or mis-prioritized issues
- use the `logvalet` skill to run `issue update` for specific changes
- use `logvalet-health` for ongoing health monitoring

---

## Notes

- 3コマンドは**必ず並列実行**する（API 呼び出し最小化・速度向上）
- `--exclude-status "完了,却下"` の指定を推奨（解決済み課題を除外）
- `project health` は統合ビューのため、それだけでも基本的なリスク評価には十分
- `project blockers` と `issue stale` は詳細根拠として活用
- リスクレベル評価は mechanical ではなく contextual — コメント数・期限までの残日数・プロジェクト規模を考慮する

---

## Anti-patterns

- Do not auto-update any issues — risk assessment is read-only
- Do not present a risk report without recommending concrete actions
- Do not treat every stale issue as high risk — use severity signals from `project blockers`
- Do not run the three fetch commands sequentially — always run in parallel
- Do not skip the project identification step — never guess the project key
