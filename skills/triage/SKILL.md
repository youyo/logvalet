---
name: logvalet:triage
description: >
  Triage a Backlog issue: the LLM analyzes triage materials (issue attributes, history,
  project statistics, similar issues) and suggests priority, assignee, and category assignments.
  Use this skill whenever someone needs help deciding how to prioritize or assign an issue.
  TRIGGER when: user says "トリアージ", "triage", "優先度を決めて", "アサイン提案",
  "課題を振り分けて", "誰に担当させる", "priority 提案", "担当者を決めて",
  "課題のカテゴリ分類", "issue triage", "優先度の見直し", "アサインの提案",
  "この課題の優先度は", "誰にアサインすべき", "振り分けて",
  "prioritize this issue", "who should handle this", "assign this task",
  "課題の優先順位", "タスクの振り分け", "課題を整理して".
  DO NOT TRIGGER when: user wants full issue context without recommendations (use context)
  or wants to create a new issue (use issue-create).
  Workflow: Automatically fetches triage-materials before analysis.
---

# logvalet-triage

`lv issue triage-materials` を材料に、LLM が priority / assignee / category を提案する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-triage` when you need to:

- decide or review the priority of a Backlog issue
- suggest an appropriate assignee based on workload and past involvement
- categorize an issue into the right category
- quickly resolve ambiguous or unassigned issues at inbox triage time

---

## Workflow

### Step 1: Identify issue key

If the user provides a Backlog issue key (e.g., `PROJ-123`, a URL like `*.backlog.com/view/PROJ-123`), extract and use it directly.

If no key is provided, ask for it.

### Step 2: Fetch triage materials

```bash
lv issue triage-materials ISSUE_KEY -f json
```

This returns a structured set of triage signals including:

- issue attributes (priority, type, assignee, due date, status)
- comment history and update history
- similar issues with assignee distribution (`similar_issues.assignee_distribution`)
- per-assignee open issue counts (`project_stats.by_assignee`)
- blocker signals

### Step 3: Analyze materials and produce proposals

Based on the triage materials, reason about each of the following:

**Priority:**
- Is the current priority appropriate?
- Consider: due date proximity, blocker signals, issue type, description urgency
- Propose a change only when there is clear evidence (e.g., overdue, high-impact keywords, blocked by this issue)

**Assignee:**
- Who should handle this issue?
- Consider:
  - `project_stats.by_assignee`: current open issue counts per user (prefer users with fewer open issues)
  - `similar_issues.assignee_distribution`: who has handled similar issues in the past (domain familiarity)
  - Watch signals: if the user has a watching list available (`lv watching list --user-id me -f json`), check whether any team member is already watching this issue — watching indicates interest or domain knowledge, which can be a positive signal for assignee selection
- Propose the best candidate with reasoning

**Category:**
- Does the issue belong to a specific category?
- Consider: description content, error logs, issue type, keywords

### Step 4: Present proposals

Present the triage proposal in a structured format:

```
## トリアージ提案 — ISSUE_KEY

### 現状
- ステータス: X | 優先度: Y | 担当者: Z

### 提案
| 項目 | 現在 | 提案 | 根拠 |
|------|------|------|------|
| 優先度 | 中 | 高 | 期限超過・高影響度 |
| 担当者 | 未設定 | UserName | 関連スキルあり、負荷余裕あり |
| カテゴリ | — | バグ | 再現手順・エラーログあり |

### 関連課題
- PROJ-100: 類似バグ（解決済み）
- PROJ-200: 同一コンポーネント（進行中）

### ウォッチ情報
- この課題をウォッチしているユーザーが特定できる場合、参考情報として表示する
- ウォッチしている ＝ 関心や知識がある可能性 → 担当候補の補助シグナル

---
適用しますか？ [優先度/担当者/カテゴリ/全て/キャンセル]
```

### Step 5: Apply confirmed changes

After the user confirms which items to apply, run `lv issue update` for each approved change:

```bash
lv issue update ISSUE_KEY --priority "高" --dry-run
lv issue update ISSUE_KEY --priority "高"
```

```bash
lv issue update ISSUE_KEY --assignee USER_ID --dry-run
lv issue update ISSUE_KEY --assignee USER_ID
```

```bash
lv issue update ISSUE_KEY --category "バグ" --dry-run
lv issue update ISSUE_KEY --category "バグ"
```

Always run `--dry-run` first before applying each change.

---

## Notes

- `triage-materials` is the authoritative source — do not fetch separate `issue get` / `user workload` calls; the materials contain everything needed
- `project_stats.by_assignee` provides current open issue count per user — use it to avoid overloading busy members
- `similar_issues.assignee_distribution` shows who has handled similar issues — use it for domain familiarity signals
- If triage materials return an empty `similar_issues` list, rely on `project_stats.by_assignee` alone for assignee proposals
- Do not apply changes without explicit user confirmation
- ウォッチ情報は担当者提案の補助シグナルとして使用する。ウォッチしている人は課題への関心や知識がある可能性が高い
- Backlog API の制約で「特定課題をウォッチしているユーザー一覧」を直接取得できない場合がある。その場合はウォッチ情報をスキップし、既存のシグナル（project_stats.by_assignee、similar_issues）のみで提案する
- Watch CLI（M17）が未実装の場合、ウォッチ関連の分析はスキップする

---

## Anti-patterns

- Do not apply any updates without asking the user first
- Do not guess the issue key — always confirm
- Do not run `user workload` separately; `triage-materials` already includes workload signals via `project_stats.by_assignee`
- Do not propose an assignee based solely on the current workload — consider domain familiarity from `similar_issues.assignee_distribution`
