---
name: digest-periodic
description: >
  Generate a weekly or daily digest of Backlog project activity: completed issues,
  newly started work, blocked items, and active issue counts — deterministic, no LLM needed.
  TRIGGER when: user says "週次ダイジェスト", "日次ダイジェスト", "今週の進捗",
  "weekly digest", "daily digest", "今週のまとめ", "進捗まとめ",
  "先週の振り返り", "日報ネタ", "週報ネタ", "今日の進捗",
  "プロジェクトの今週の動き", "weekly summary", "daily summary",
  "weekly progress", "今週完了したもの", "今日のアクティビティ",
  "定期レポート", "periodic digest", "進捗サマリー".
  DO NOT TRIGGER when: user wants a full activity report with user/team breakdown
  (use report) or wants anomaly detection (use intelligence).
  Complements: Combine with /logvalet:report for a comprehensive periodic review.
---

# logvalet-digest-periodic

`lv digest weekly` / `lv digest daily` を材料に、LLM が人間向けのサマリーとハイライトを生成する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-digest-periodic` when you need to:

- generate a weekly summary for a sprint review or weekly standup
- create a daily progress report for a project
- surface highlights, risks, and next actions from a project's recent activity
- prepare a concise human-readable digest from raw Backlog activity data

---

## Workflow

### Step 1: Identify project and period

If the user provides a project key and period (weekly/daily), use them directly.

If not provided, ask:

- project key (or list with `lv project list -f md`)
- period: `weekly` or `daily`
- for `daily`: specific date (default: today)

### Step 2: Fetch digest

For weekly digest:

```bash
lv digest weekly -k PROJECT_KEY -f json
```

For daily digest:

```bash
lv digest daily -k PROJECT_KEY -f json
lv digest daily -k PROJECT_KEY --since YYYY-MM-DD --until YYYY-MM-DD -f json
```

**Note:** Use `--since` / `--until` flags (not `--date`) when specifying a custom date range.

### Step 3: Analyze digest output

The digest JSON includes structured fields to guide LLM reasoning:

- `summary`: pre-computed counts and metrics (completed, new, in-progress, overdue, stale)
- `completed_issues`: list of issues closed in the period
- `new_issues`: list of issues created in the period
- `status_changes`: issue status transitions
- `active_comments`: comment activity
- `llm_hints`: LLM-facing guidance on what to highlight, what risks to surface, and what actions to suggest

**Always read and apply `llm_hints`** — they encode deterministic signals from the CLI (e.g., "N issues are overdue", "M issues are stale") that should directly inform the narrative. The `llm_hints` field removes guesswork and ensures the summary accurately reflects project state.

### Step 4: Generate human-readable summary

Using the digest data and `llm_hints`, produce a Markdown summary:

```
## 週次ダイジェスト — PROJECT_KEY (YYYY/MM/DD - YYYY/MM/DD)

### ハイライト
- 完了: N件（PROJ-XXX, PROJ-YYY）
- 新規開始: N件
- 注目: PROJ-ZZZ が期限に近づいています

### リスク・懸念
- N件が停滞中（7日以上更新なし）
- 期限超過: N件

### 次のアクション
- PROJ-ZZZ のステータス確認
- UserName の過負荷解消のためのタスク再配分検討

---
詳細を確認しますか？ 特定の課題に絞りますか？
```

For daily digest, adjust the header to:

```
## 日次ダイジェスト — PROJECT_KEY (YYYY/MM/DD)
```

### Step 5: Offer drill-down

After presenting the summary, offer to:

- show details for a specific issue
- filter by assignee or status
- generate a report for a different period

---

## Notes

- `llm_hints` in the digest output is the primary signal for risk and action items — always surface content flagged there
- `summary` fields provide pre-computed counts — use them directly rather than re-counting from raw arrays
- For multi-project digests, run `lv digest weekly -k PROJ1 -k PROJ2 -f json` with multiple `-k` flags
- Completed issues list should include issue keys for easy reference
- If the period contains no activity, report "活動なし" rather than an empty section

---

## Anti-patterns

- Do not use `--date` flag — use `--since` / `--until` for custom date ranges
- Do not ignore `llm_hints` — they contain pre-analyzed signals that improve summary accuracy
- Do not re-compute counts from raw arrays when `summary` already provides them
- Do not skip the project identification step — never guess the project key
