---
name: backlog:my-week
description: >
  Show this week's Backlog issues assigned to me, including overdue items, across all projects.
  TRIGGER when: user says "今週のタスク", "my week", "今週やること", "今週の予定",
  "backlogの今週分", "バックログで今週", "weekly tasks", "this week's issues",
  "今週やるべきこと", "backlog.com の今週のタスク", "今週何やる", "今週の課題",
  "backlog 今週", "バックログ 今週", "今週 backlog".
---

# backlog:my-week

今週自分がやるべき Backlog 課題の一覧を表示する。期限切れも含む。プロジェクト横断。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## Workflow

### Step 1: Fetch data

Run these two commands **in parallel** (use two Bash tool calls in a single message):

```bash
lv issue list --assignee me --status not-closed --due-date overdue --sort dueDate --order asc -f md
```

```bash
lv issue list --assignee me --status not-closed --due-date this-week --sort dueDate --order asc -f md
```

### Step 2: Format output

Combine results into two sections. Deduplicate by issue key (if an item appears in both, show it only in Overdue).

**Output format:**

```
## ⚠ 期限切れ (N件)

<overdue issues in md format>

## 📅 今週 (N件)

<this week issues in md format>

---
期限切れ: X件 / 今週: Y件 / 合計: Z件
```

### Step 3: No user interaction needed

This is a display-only skill. No questions, no writes.

---

## Notes

- `--assignee me` resolves to the authenticated user automatically
- `--status not-closed` includes Open (1), In Progress (2), Resolved (3)
- `--due-date overdue` returns items with due date before today
- `--due-date this-week` returns items due Monday–Sunday of the current week
- If either command returns no items, show "なし" in that section
- Output is cross-project — no `--project-key` filter is used
