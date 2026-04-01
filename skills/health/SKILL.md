---
name: logvalet:health
description: >
  Check the health of a Backlog project: stale issues, blockers, user workload imbalance,
  and an overall health score (0-100) with level (healthy/warning/critical).
  Use this skill whenever someone asks about the state of a project, team performance,
  or whether a project is on track.
  TRIGGER when: user says "プロジェクトの状態", "project health", "プロジェクト健全性",
  "プロジェクト大丈夫", "このプロジェクトどう", "停滞してる課題ある？",
  "ブロッカーは？", "プロジェクトの健全性", "health check",
  "チームの負荷状況", "ワークロード", "workload", "プロジェクト診断",
  "project status", "is the project on track", "プロジェクトのスコア",
  "開発の進み具合", "課題の滞留状況", "プロジェクト概況".
  DO NOT TRIGGER when: user wants a detailed risk assessment with recommendations
  (use risk) or wants activity trend analysis (use intelligence).
  Complements: Follow up with /logvalet:risk for actionable recommendations.
---

# logvalet-health

Backlog プロジェクトの健全性（停滞課題・ブロッカー・ユーザー負荷）を確認する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-health` when you need to:

- identify stale (long-unupdated) issues that may be blocking progress
- detect high-priority unassigned or overdue items
- understand team member workload distribution
- prepare for a project review, sprint planning, or standup

---

## Workflow

### Step 1: Identify project

If the user provides a project key, use it directly.

If not provided, list available projects:

```bash
lv project list -f md
```

Then ask the user to select a project.

### Step 2: Determine scope

Ask in a single question (if not already specified):

- `--days`: 停滞とみなす日数（デフォルト: 7）
- `--include-comments`: コメントを含む分析を行うか（デフォルト: false）
- `--exclude-status`: 除外するステータス（例: "完了,却下"）

If the user wants a quick overview without customization, skip and use defaults.

### Step 3: Fetch health data

Run all three commands **in parallel**:

```bash
lv project health PROJECT_KEY --days DAYS [--include-comments] [--exclude-status "STATUS1,STATUS2"] -f json
```

Or fetch the individual components in parallel for more control:

```bash
lv issue stale -k PROJECT_KEY --days DAYS [--exclude-status "STATUS1,STATUS2"] -f md
```

```bash
lv project blockers PROJECT_KEY --days DAYS [--include-comments] [--exclude-status "STATUS1,STATUS2"] -f md
```

```bash
lv user workload PROJECT_KEY --days DAYS [--exclude-status "STATUS1,STATUS2"] -f md
```

### Step 4: Format output

Combine results into a structured health report:

```
## プロジェクト健全性レポート — PROJECT_KEY

> 分析期間: DAYS日 / 生成日時: YYYY-MM-DD

---

### 停滞課題 (N件)

<stale issues table or "なし">

---

### ブロッカー

#### 未アサイン高優先度課題 (N件)
<blocker issues table or "なし">

#### 期限超過課題 (N件)
<overdue issues table or "なし">

---

### ユーザー負荷

<user workload table>

---

**サマリー:**
- 停滞課題: N件
- ブロッカー候補: N件
- 最多担当者: UserName (N件)
- 要対応アクション: <action items if any>
```

### Step 5: No writes

This is a read-only skill. No issue updates are performed.

If the user wants to act on findings (update an issue, reassign, etc.), switch to the `logvalet` skill.

---

## Notes

- `project health` is the integrated command — use it when you want a single call
- Use individual commands (`issue stale`, `project blockers`, `user workload`) when you need to customize flags independently
- `--exclude-status "完了,却下"` is recommended to filter out resolved items
- `--include-comments` adds comment analysis for deeper blocker detection but is slower
- If any section returns no items, show "なし" in that section

---

## Anti-patterns

- Do not run health checks on write-protected or archived projects without confirming with the user
- Do not auto-update issues based on health findings — always ask before writing
- Do not skip the project identification step — never guess the project key
