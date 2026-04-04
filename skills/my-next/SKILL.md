---
name: logvalet:my-next
description: >
  Show near-term Backlog issues assigned to me AND issues I'm watching:
  next few business days across all projects, including overdue items —
  helps answer "what should I work on next?" and "what should I keep an eye on?"
  Watched issues are included because they represent work you care about even
  without being the assignee — blocked dependencies, pending reviews, cross-team items.
  TRIGGER when: user says "直近のタスク", "upcoming tasks", "次にやること", "次やること",
  "明日以降のタスク", "backlogの直近", "coming up", "what's next",
  "明日何やる", "次の予定", "今日と明日のタスク", "直近の課題",
  "next tasks", "upcoming issues", "what should I do next",
  "明日の予定", "次のアクション", "今日やること", "today's tasks",
  "直近やるべきこと", "tomorrow's tasks", "近日中のタスク",
  "ウォッチしてるやつどうなった", "watched issues status".
  DO NOT TRIGGER when: user wants a full week overview (use my-week)
  or wants a project-wide task list (use logvalet CLI directly).
---

# logvalet-my-next

直近数日（今日から4営業日先まで）の Backlog 課題 + ウォッチしている課題の一覧を表示する。週を跨ぐ。期限切れ含む。プロジェクト横断。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## Difference from logvalet-my-week

| | logvalet-my-week | logvalet-my-next |
|---|---|---|
| 範囲 | 今週（月〜日） | 今日〜4営業日先 |
| 金曜日の場合 | 金〜日のみ | 金〜翌週木 |
| 週跨ぎ | しない | する |

---

## Workflow

### Step 1: Calculate date range

Compute the end date as **today + 4 business days** using the day-of-week offset table:

| Day of week (date +%u) | Calendar days to add | Example (from Thu 3/27) |
|---|---|---|
| 1 (Mon) | +4 | Fri |
| 2 (Tue) | +6 | Mon next week |
| 3 (Wed) | +6 | Tue next week |
| 4 (Thu) | +6 | Wed next week |
| 5 (Fri) | +6 | Thu next week |
| 6 (Sat) | +5 | Thu next week |
| 7 (Sun) | +4 | Thu next week |

Run this to compute the dates:

```bash
DOW=$(/bin/date +%u 2>/dev/null || date +%u)
case $DOW in
  1) OFFSET=4 ;; 2|3|4|5) OFFSET=6 ;; 6) OFFSET=5 ;; 7) OFFSET=4 ;;
esac
# macOS native /bin/date uses -v, GNU date uses -d
END_DATE=$(/bin/date -v+${OFFSET}d +%Y-%m-%d 2>/dev/null || date -d "+${OFFSET} days" +%Y-%m-%d)
TODAY=$(/bin/date +%Y-%m-%d 2>/dev/null || date +%Y-%m-%d)
echo "TODAY=${TODAY} END_DATE=${END_DATE}"
```

### Step 2: Fetch data

Run these three commands **in parallel**:

```bash
lv issue list --assignee me --status not-closed --due-date overdue --sort dueDate --order asc -f md
```

```bash
lv issue list --assignee me --status not-closed --due-date ${TODAY}:${END_DATE} --sort dueDate --order asc -f md
```

```bash
lv watching list --user-id me -f md
```

### Step 3: Format output

Combine results into three sections. Deduplicate by issue key.

**Output format:**

```
## ⚠ 期限切れ (N件)

<overdue issues in md format>

## 📅 直近 〜 END_DATE (N件)

<upcoming issues in md format>

## 👁 ウォッチ中 (N件)

<watched issues — 自分担当ではないが注視している課題>

---
担当（期限切れ）: X件 / 担当（直近）: Y件 / ウォッチ: W件 / 合計: Z件
```

**Exclusion and enrichment logic for watched issues:**
- Exclude watched issues that are also assigned to you (already shown in assigned sections)
- Show only watched issues with `status: not-closed`
- Mark overdue watched issues with ⚠ prefix
- Add "停滞中" signal for watched issues with last update > 7 days ago
- Display assignee name for each watched issue

### Step 4: No user interaction needed

This is a display-only skill. No questions, no writes.

---

## Notes

- `--assignee me` resolves to the authenticated user automatically
- `--status not-closed` includes Open (1), In Progress (2), Resolved (3)
- `--due-date overdue` returns items with due date before today
- `--due-date ${TODAY}:${END_DATE}` returns items within the computed date range
- If any command returns no items, show "なし" in that section
- Output is cross-project — no `--project-key` filter is used
- `lv watching list --user-id me` returns all watched issues regardless of status. The skill filters to `not-closed` only
- Watched issues represent work you care about even without being the assignee — blocked dependencies, pending reviews, cross-team items
- **If Watch CLI (M17) is not yet implemented**, skip the ウォッチ中 section and display only assigned issues

---

## Optional: Workload context (Phase 1)

When the user wants to understand team workload before picking the next task, use:

```bash
# Show workload distribution for a project
lv user workload PROJECT_KEY -f md
```

Use this when:
- The user is choosing what to work on next and wants to know if colleagues are overloaded
- The user asks "誰かに頼める？" or "チームのキャパは？"
- You want to recommend delegation targets based on workload balance

**When to include automatically:** If the user's upcoming tasks include items blocked by others, run `lv user workload` for the relevant project and append a "チーム負荷サマリー" section below the upcoming task list.

---

## Optional: Triage materials (Phase 2)

When the user wants to assess or reprioritize a specific upcoming issue before working on it:

```bash
# Get structured triage materials for an issue
lv issue triage-materials ISSUE_KEY -f json
```

Use this when:
- The user asks "この課題どうすればいい？" or "優先度合ってる？"
- An upcoming issue looks ambiguous (no assignee, unclear priority, long stale)

The `issue triage-materials` output includes issue attributes, comment history, and similar-issue statistics. Hand the JSON to the `logvalet-triage` skill for LLM-assisted priority and assignee suggestions.

---

## Optional: Issue decision history (Phase 3)

When the user wants to understand the history of an upcoming issue before working on it:

```bash
# Get chronological timeline for an issue
lv issue timeline ISSUE_KEY -f json
```

Use this when:
- The user asks "この課題の経緯は？" or "なぜこの課題が作られた？"
- An upcoming issue has a long or complex history worth reviewing before starting

Hand the JSON to the `logvalet-decisions` skill for LLM-assisted decision log extraction and summary.
