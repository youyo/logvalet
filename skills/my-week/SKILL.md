---
name: logvalet:my-week
description: >
  Show this week's Backlog issues assigned to me AND issues I'm watching
  across all projects, including overdue items from previous weeks — the weekly planning view.
  Watched issues matter because they affect your work even when you're not the assignee:
  dependency blockers, reviews you're waiting on, cross-team items you need to track.
  TRIGGER when: user says "今週のタスク", "my week", "今週やること", "今週の予定",
  "backlogの今週分", "weekly tasks", "this week's issues",
  "今週やるべきこと", "今週何やる", "今週の課題",
  "今週のバックログ", "weekly plan", "week overview",
  "今週の計画", "月曜から金曜のタスク", "今週のスケジュール",
  "weekly planning", "what's on my plate this week", "今週の見通し",
  "ウォッチしてる課題", "watching issues", "気にしてる課題",
  "注視してる課題", "watched tasks this week".
  DO NOT TRIGGER when: user wants only the next 1-2 days (use my-next)
  or wants team-wide workload (use health).
---

# logvalet-my-week

今週自分がやるべき Backlog 課題 + ウォッチしている課題の一覧を表示する。期限切れも含む。プロジェクト横断。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## Workflow

### Step 1: Fetch data

Run these three commands **in parallel** (use three Bash tool calls in a single message):

```bash
lv issue list --assignee me --status not-closed --due-date overdue --sort dueDate --order asc -f md
```

```bash
lv issue list --assignee me --status not-closed --due-date this-week --sort dueDate --order asc -f md
```

```bash
lv watching list --user-id me -f md
```

### Step 2: Format output

Combine results into three sections. Deduplicate by issue key (if an item appears in both, show it only in Overdue).

**Exclusion logic:**
- Watched issues that are also assigned to you: exclude from the Watch section (already shown in Assigned sections)
- Watched issues with status closed: exclude from the Watch section (show only not-closed items)
- Watched issues with due date in the past: mark with ⚠ to signal they are overdue
- Watched issues without update for 7+ days: add "停滞中" signal to indicate stagnation
- Watched issue assignees: display assignee name in parentheses so the user knows who is responsible

**Output format:**

```
## ⚠ 期限切れ (N件)

<overdue issues in md format>

## 📅 今週 (N件)

<this week issues in md format>

## 👁 ウォッチ中 (N件)

<watched issues — 自分担当ではないが注視している課題>

---
担当（期限切れ）: X件 / 担当（今週）: Y件 / ウォッチ: W件 / 合計: Z件
```

### Step 3: No user interaction needed

This is a display-only skill. No questions, no writes.

---

## Notes

- `--assignee me` resolves to the authenticated user automatically
- `--status not-closed` includes Open (1), In Progress (2), Resolved (3)
- `--due-date overdue` returns items with due date before today
- `--due-date this-week` returns items due Monday–Sunday of the current week
- If any command returns no items, show "なし" in that section
- Output is cross-project — no `--project-key` filter is used
- `lv watching list --user-id me` returns all watched issues regardless of status. Filtering by not-closed status is performed by this skill
- Watched issues represent work that affects you even when you're not assigned: dependency blockers, reviews you're waiting on, cross-team items requiring tracking
- If the Watch CLI (M17) is not yet implemented, skip the Watch section and display only assigned issues

---

## Optional: Stale & overdue signals (Phase 1)

When the user wants deeper insight beyond the issue list, use stale detection to surface long-overdue items:

```bash
# Identify issues that haven't been updated in 7+ days across a specific project
lv issue stale -k PROJECT_KEY --days 7 -f md
```

Use this when:
- The user wants to know which overdue items are actually stuck (not just late)
- You want to flag items that may need escalation

If multiple projects are active, run `lv issue stale` per project in parallel.

**Signal integration rule:** If any stale issues appear in the current week's items, annotate them in the output with a warning indicator (e.g., `⚠ 停滞中`) to help the user prioritize review.

---

## Optional: Weekly digest (Phase 2)

When the user wants a higher-level summary of the week's activity rather than a personal task list, use:

```bash
# Weekly activity digest for a project
lv digest weekly -k PROJECT_KEY -f json
```

Use this when:
- The user asks "今週何が進んだ？" or "チームの週次サマリーが欲しい"
- You want to provide a project-level view alongside the personal task list

The `digest weekly` output includes completed, started, and blocked issues for the week. Hand the JSON to the `logvalet-digest-periodic` skill for an LLM-generated narrative summary.

---

## Optional: Activity intelligence (Phase 3)

When the user wants to understand team activity patterns or detect unusual workload distribution this week:

```bash
# Get activity statistics for a project
lv activity stats --scope project -k PROJECT_KEY --since 2026-04-01T00:00:00Z --until 2026-04-07T23:59:59Z -f json
```

Use this when:
- The user asks "今週誰が一番動いた？" or "チームの活動に偏りがある？"
- You want to surface contributors who may be overloaded or inactive

Hand the JSON to the `logvalet-intelligence` skill for LLM-assisted anomaly detection and risk interpretation.
