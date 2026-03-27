---
name: backlog:issue-create
description: >
  Create a Backlog issue interactively, gathering missing information via questions before submission.
  TRIGGER when: user says "課題作成", "issue作成", "チケット作成", "タスク登録",
  "backlogに課題を作って", "バックログに登録", "新しいissue", "create issue",
  "file a ticket", "make a task", "backlog.com に課題追加", "backlog 課題作成",
  "バックログ 課題作成", "backlog 登録", "バックログ 登録",
  "課題を作りたい", "issueを立てたい", "チケットを切って", "タスクを作って".
---

# backlog:issue-create

Backlog 課題をインタラクティブに作成する。不足情報は質問で補完し、dry-run でプレビューしてから実行する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## Workflow

### Step 1: Identify project

If `project-key` is provided by the user, use it directly.

If not provided, list available projects:

```bash
lv project list -f md
```

Then ask the user to select a project using AskUserQuestion.

### Step 2: Gather required fields

**Required:**
- `--project-key` — from Step 1
- `--summary` — the issue title

If `summary` is not provided, ask for it.

### Step 3: Gather optional fields

Ask about optional fields **in a single question** (not one-by-one). Present them as a checklist. The user can skip any or all.

Optional fields:
- `--description` — issue body text
- `--assignee` — user ID or name (use `lv user list -f md` if needed)
- `--priority` — name or ID (default: "normal"). Values: 高/中/低 or high/normal/low
- `--due-date` — YYYY-MM-DD format
- `--start-date` — YYYY-MM-DD format
- `--issue-type` — name or ID (use `lv meta status PROJECT_KEY` if user needs to see options)
- `--category` — name or ID, multiple allowed (use `lv meta category PROJECT_KEY` if needed)
- `--milestone` — name or ID, multiple allowed (use `lv meta version PROJECT_KEY` if needed)
- `--parent-issue-id` — parent issue ID for sub-tasks

### Step 4: Resolve metadata (if needed)

If the user needs to see available options for issue-type, category, milestone, or assignee:

```bash
lv meta category PROJECT_KEY -f md
lv meta version PROJECT_KEY -f md
lv user list -f md
```

Run these only when the user explicitly asks or is unsure about available values.

### Step 5: Dry-run preview

**ALWAYS** run with `--dry-run` first:

```bash
lv issue create --project-key PROJ --summary "..." \
  [--description "..."] [--assignee USER_ID] [--priority normal] \
  [--due-date YYYY-MM-DD] [--issue-type "タスク"] \
  [--category "カテゴリ名"] [--milestone "マイルストーン名"] \
  --dry-run
```

Show the dry-run output to the user and ask for confirmation.

### Step 6: Execute

After user confirms, run the same command **without** `--dry-run`:

```bash
lv issue create --project-key PROJ --summary "..." \
  [--description "..."] [--assignee USER_ID] [--priority normal] \
  [--due-date YYYY-MM-DD] [--issue-type "タスク"] \
  [--category "カテゴリ名"] [--milestone "マイルストーン名"]
```

### Step 7: Report result

Display:
- Created issue key (e.g., `PROJ-456`)
- Issue URL (construct from base_url in config)
- Summary of what was created

---

## Important rules

- **Always dry-run first** — never create without preview
- **Name resolution is automatic** — you can pass names (e.g., `--priority "高"`) instead of IDs; logvalet resolves them
- **--description vs --description-file** — mutually exclusive. Use `--description` for short text, `--description-file` for long content (write to a temp file first)
- **Multiple values** — `--category` and `--milestone` accept multiple `--category A --category B` flags
- **Assignee** — accepts numeric user ID or user name string

## Anti-patterns

- Do NOT create an issue without showing the dry-run output first
- Do NOT ask optional fields one-by-one — batch them in a single question
- Do NOT guess project-key — always confirm with the user or list projects
