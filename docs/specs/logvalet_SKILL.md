---
name: logvalet
summary: Use logvalet (lv) to read, summarize, and safely update Backlog with LLM-friendly JSON digests.
---

# logvalet

`logvalet` is an LLM-first CLI for Backlog.

It is **not** a thin API wrapper. Its main job is to turn Backlog data into **stable, compact, machine-readable digest JSON** that works well for Claude Code, Codex, and other coding agents.

Full command:

```bash
logvalet
```

Short command:

```bash
lv
```

Default output format:

```bash
json
```

Supported output formats:

```bash
json
md
text
yaml
```

Use `digest` commands whenever you need context for reasoning. Use `get` or `list` when you need raw-ish structured data.

---

## When to use this skill

Use `logvalet` when you need to:

- understand a Backlog issue with comments and project metadata included
- summarize project activity for a time window
- inspect documents stored in Backlog
- retrieve project metadata such as statuses, categories, versions, and custom fields
- inspect a specific user's activity over a period, including comments
- create or update issues
- add or update issue comments
- create Backlog documents safely

Prefer `logvalet` over ad hoc Backlog API calls when:

- you want **LLM-friendly JSON**
- you want **digest-oriented summaries**
- you want **stable schemas**
- you want **explicit, non-destructive operations**

---

## Safety model

`logvalet` is intentionally conservative.

Supported write operations:

- issue create
- issue update
- issue comment add
- issue comment update
- document create

Explicitly unsupported:

- document update
- document delete
- document replace
- wiki operations
- group operations

For write operations, prefer `--dry-run` first when reviewing payloads.

---

## Auth and configuration

Configuration file:

```text
~/.config/logvalet/config.toml
```

Token store:

```text
~/.config/logvalet/tokens.json
```

Resolution priority:

```text
CLI flags > environment variables > config.toml
```

Secrets are not stored in project-local files.

Primary auth mode:

- API key (via `config init --init-api-key` or `auth login`)

Secondary auth mode:

- OAuth (access token via `--access-token` flag or env var)

Useful environment variables:

```text
LOGVALET_PROFILE
LOGVALET_FORMAT
LOGVALET_CONFIG
LOGVALET_API_KEY
LOGVALET_ACCESS_TOKEN
LOGVALET_BASE_URL
LOGVALET_SPACE
LOGVALET_PRETTY
LOGVALET_NO_COLOR
LOGVALET_VERBOSE
LOGVALET_COMMENTS
LOGVALET_NO_ACTIVITY
LOGVALET_LIMIT
LOGVALET_OFFSET
LOGVALET_CONTENT
LOGVALET_CONTENT_FILE
LOGVALET_DRY_RUN
```

---

## Global flags

Common flags available across commands:

```text
--profile, -p <name>
--format, -f <json|md|text|yaml>
--pretty
--config, -c <path>
--api-key <key>
--access-token <token>
--base-url <url>
--space, -s <space>
--verbose, -v
--no-color
```

Digest-oriented flags:

```text
--comments <n>
--no-activity
```

List-oriented flags:

```text
--count <n>
--offset <n>
```

Write-oriented flags:

```text
--content <string>
--content-file <path>
--dry-run
--parent-issue-id <id>
--notified-user-id <id> (repeatable)
--emoji <emoji>
--add-last
--comment <string>
```

Rules:

- `--content` and `--content-file` are mutually exclusive.
- `--api-key` and `--access-token` should not be provided together.
- Use `--profile` explicitly in multi-space environments.

---

## Output conventions

### Default

Default output is JSON.

This is intentional.

`logvalet` is designed for agent use, automation, and piping into other tools.

### stdout

- primary result only
- machine-readable output

### stderr

- verbose logs
- warnings and operational details
- token refresh details

Do not rely on stderr for primary data.

### Error contract

On complete failure, commands should return non-zero exit codes and emit an error JSON shape like:

```json
{
  "schema_version": "1",
  "error": {
    "code": "issue_not_found",
    "message": "Issue PROJ-999 was not found.",
    "retryable": false
  }
}
```

On partial success, commands should still return a valid result and populate `warnings`.

---

## Digest philosophy

A `digest` command returns **context**, not just records.

Typical digest output includes:

- the primary resource
- nearby metadata required for interpretation
- comments when important
- activity when relevant
- a deterministic summary section
- LLM hints

Use `digest` first for reasoning tasks.

Examples:

```bash
logvalet digest --issue PROJ-123 --since 30d
logvalet digest --project PROJ --since 30d
logvalet activity digest --project PROJ --since 30d
logvalet document digest 019b0240-4a9a-7c90-xxxx
logvalet digest --user 12345 --since 30d
```

---

## User representation

Inside digests, users should be represented in compact form to save tokens:

```json
{
  "id": 12345,
  "name": "Naoto Ishizawa"
}
```

Use the full user shape only in user-focused commands such as `user list` and `user get`:

```json
{
  "id": 12345,
  "user_id": "naoto",
  "name": "Naoto Ishizawa",
  "nulab_account": {
    "nulab_id": "xxxxx"
  }
}
```

---

## Command guide

## auth

### Login with API key

```bash
logvalet auth login --profile work
```

Use this to authenticate and save tokens into `~/.config/logvalet/tokens.json`.

### Show active identity

```bash
logvalet auth whoami --profile work
```

Backlog API から認証ユーザー情報を取得して表示する。API にアクセスできない場合は認証情報のみ表示。

### List configured profiles and auth state

```bash
logvalet auth list
```

### Remove stored credentials for a profile

```bash
logvalet auth logout --profile work
```

---

## config

### Initialize configuration interactively

```bash
logvalet config init
```

or use the top-level alias:

```bash
logvalet configure
```

This creates `~/.config/logvalet/config.toml` with profile, space, and base URL.

### One-command setup with API key

```bash
logvalet configure --init-profile work --init-space myspace --init-api-key YOUR_KEY
```

This creates `config.toml` and saves the API key to `tokens.json` in a single step. No separate `auth login` is needed.

Non-interactive flags:

```text
--init-profile <name>
--init-space <space>
--init-base-url <url>
--init-api-key <key>
```

---

## completion

Completion is generated dynamically and should be loaded with `eval`.

### zsh

Put this in `.zshrc`:

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

`--short` enables completion for both `logvalet` and `lv`.

---

## digest

The unified digest command combines issue, activity, and user data into a single time-scoped summary.

### Basic usage

```bash
logvalet digest --project PROJ --since 30d
```

### By user

```bash
logvalet digest --user me --since this-week
logvalet digest --user 12345 --since 30d
```

### By team

```bash
logvalet digest --team "TeamName" --since this-month
```

### By issue

```bash
logvalet digest --issue PROJ-123 --since 30d
```

### Combined filters

```bash
logvalet digest --project PROJ --user me --since this-week
```

Flags:

```text
--project, -k <key> (repeatable)
--user <id-or-name> (repeatable)
--team <id-or-name> (repeatable)
--issue <key> (repeatable)
--since <period> (required: today, this-week, this-month, YYYY-MM-DD)
--until <period> (optional: today, this-week, this-month, YYYY-MM-DD)
```

Use this for:

- project timeline summaries
- user activity reviews
- team workload overviews
- issue-focused context gathering

---

## issue

### Get one issue

```bash
logvalet issue get PROJ-123
```

Use when you want structured issue details without the extra context-building behavior of `digest`.

### List issues

```bash
logvalet issue list --project-key PROJ --limit 20
```

Common filters:

```bash
logvalet issue list --project-key PROJ --assignee me
logvalet issue list --project-key PROJ --status 3
logvalet issue list --assignee me --status not-closed
logvalet issue list --project-key PROJ --due-date today
logvalet issue list --project-key PROJ --sort dueDate --order asc
```

`--assignee` accepts: `me`, numeric user ID, user name, or team name.

`--status` accepts:

| Value | Description | `-k` required? |
|-------|-------------|----------------|
| `not-closed` | 完了以外すべて（未対応+処理中+処理済み） | No — cross-project OK |
| `open` | 未対応のみ（プロジェクトのステータス一覧から完了以外を取得） | Yes |
| Name string | `"処理中"` etc. — プロジェクトのステータス名で照合 | Yes |
| Numeric ID | `3` etc. — ステータスIDを直接指定 | No |
| Comma-separated | `"1,2,3"` — 複数ステータスIDを一括指定 | No (if all numeric) |

**Important:** `not-closed` is the most useful default for agents. It requires no `-k` flag and works across all projects, making it ideal for `--assignee me` queries.

### Create an issue

Minimum (issue type and priority use project defaults):

```bash
logvalet issue create \
  --project-key PROJ \
  --summary "Fix login bug"
```

With issue type and priority by name:

```bash
logvalet issue create \
  --project-key PROJ \
  --summary "Fix login bug" \
  --issue-type "バグ" \
  --priority "高"
```

Full options:

```bash
logvalet issue create \
  --project-key PROJ \
  --summary "Fix login bug" \
  --issue-type "バグ" \
  --priority "高" \
  --assignee 12345 \
  --category "UI" \
  --versions "v1.0" \
  --milestone "Sprint 3" \
  --start-date 2026-03-01 \
  --due-date 2026-04-01 \
  --parent-issue-id 999 \
  --notified-user-id 111 \
  --notified-user-id 222
```

With description file:

```bash
logvalet issue create \
  --project-key PROJ \
  --summary "Fix login bug" \
  --description-file ./description.md
```

Review request payload first:

```bash
logvalet issue create \
  --project-key PROJ \
  --summary "Fix login bug" \
  --dry-run
```

### Update an issue

By name:

```bash
logvalet issue update PROJ-123 --status "処理中" --priority "高"
```

Change assignee and issue type:

```bash
logvalet issue update PROJ-123 --assignee 12345 --issue-type "バグ"
```

Update status with inline comment:

```bash
logvalet issue update PROJ-123 --status "完了" --comment "対応完了しました"
```

Update start date:

```bash
logvalet issue update PROJ-123 --start-date 2026-03-01
```

Notify specific users:

```bash
logvalet issue update PROJ-123 --status "処理中" --notified-user-id 111
```

With description file:

```bash
logvalet issue update PROJ-123 --description-file ./description.md
```

Use `--dry-run` before update when a coding agent is preparing the change.

---

## issue comment

### List comments

```bash
logvalet issue comment list PROJ-123
```

### Add a comment

Simple comment:

```bash
logvalet issue comment add PROJ-123 --content "I confirmed this issue."
```

From file:

```bash
logvalet issue comment add PROJ-123 --content-file ./comment.md
```

With notification:

```bash
logvalet issue comment add PROJ-123 --content "確認しました" --notified-user-id 111
```

Dry run:

```bash
logvalet issue comment add PROJ-123 --content-file ./comment.md --dry-run
```

### Update a comment

```bash
logvalet issue comment update PROJ-123 999 --content "Updated note"
```

---

## project

### Get one project

```bash
logvalet project get PROJ
```

### List projects

```bash
logvalet project list
```

---

## activity

### List activity

```bash
logvalet activity list --project PROJ --limit 50
```

### Activity digest

```bash
logvalet activity digest --project PROJ --since 30d
```

Important design note:

- comment-related events matter a lot and should be included
- digest output should separate raw activities, extracted comments, and summary
- project grouping is useful when consuming broader activity streams

Use this for:

- project timeline summaries
- recent team changes
- broad operational context

---

## user

### List users

```bash
logvalet user list
```

Use this when you need the mapping between Backlog user IDs and names.

### Get one user

```bash
logvalet user get 12345
```

### User activity

```bash
logvalet user activity 12345 --since 30d
logvalet user activity 12345 --since 30d --project PROJ
logvalet user activity 12345 --since 30d --type issue_created
```

---

## document

### Get one document

```bash
logvalet document get 019b0240-4a9a-7c90-xxxx
```

### List documents in a project

```bash
logvalet document list --project PROJ
```

### Get document tree

```bash
logvalet document tree --project PROJ
```

Returns the document tree structure with `activeTree` and `trashTree` nodes. Each node has `id` (UUID), `name`, `emoji`, and `children`.

### Document digest

```bash
logvalet document digest 019b0240-4a9a-7c90-xxxx
```

### Create a document

Basic:

```bash
logvalet document create \
  --project-key PROJ \
  --title "Runbook" \
  --content-file ./runbook.md
```

With emoji:

```bash
logvalet document create \
  --project-key PROJ \
  --title "Runbook" \
  --content-file ./runbook.md \
  --emoji "📖"
```

Append to end of document list:

```bash
logvalet document create \
  --project-key PROJ \
  --title "New Doc" \
  --content "Content goes here" \
  --add-last
```

Use `--dry-run` first when having an agent prepare content.

Document mutation is intentionally limited:

- create is supported
- update is not supported
- delete is not supported
- replace is not supported

Document data model notes:

- Document IDs are UUID strings (not integers)
- Document body is split into `plain` (plain text) and `json` (structured content) fields
- Documents include `tags`, `emoji`, `statusId`, and `updatedUser` metadata

---

## meta

Use project metadata commands when an agent needs the project dictionary.

### Statuses

```bash
logvalet meta status PROJ
```

### Categories

```bash
logvalet meta category PROJ
```

### Versions / milestones

```bash
logvalet meta version PROJ
```

### Custom fields

```bash
logvalet meta custom-field PROJ
```

Use these commands to resolve names, IDs, and valid metadata choices before creating or updating issues.

---

## team

### List teams

```bash
logvalet team list
logvalet team list --no-members
```

### Teams for a project

```bash
logvalet team project PROJ
```

---

## space

### Show space info

```bash
logvalet space info
```

### Show disk usage

```bash
logvalet space disk-usage
```

### Space digest

```bash
logvalet space digest
```

Use for admin-oriented overview and space-level context.

---

## version

### Show version information

```bash
logvalet version
```

or use the global flag:

```bash
logvalet --version
```

---

## Recommended patterns for coding agents

### 1. Get my open tasks (most common pattern)

```bash
logvalet issue list --assignee me --status not-closed --due-date this-week -f gantt
```

`not-closed` requires no `-k` flag and works across all projects, making it the best choice for assignee-based task lists. Combine with `--due-date` or `--start-date` to scope the time range. Use `-f gantt` for a visual timeline or `-f md` for a Markdown table.

### 2. Prefer digest over get for reasoning

Good:

```bash
logvalet digest --issue PROJ-123 --since 30d
```

Less useful for reasoning alone:

```bash
logvalet issue get PROJ-123
```

### 3. Use JSON unless a human needs to read it directly

Good:

```bash
logvalet digest --user 12345 --since 30d -f json
```

Use Markdown for sharing:

```bash
logvalet digest --user 12345 --since 30d -f md
```

### 4. Name-or-ID resolution is automatic

`issue create` and `issue update` accept both names and IDs for issue types, priorities, statuses, categories, versions, and milestones.

The CLI resolves names to IDs automatically. You no longer need to call `meta` commands before creating or updating issues.

Good:

```bash
logvalet issue create --project-key PROJ --summary "Fix bug" --issue-type "バグ" --priority "高"
```

Still useful for exploring available values:

```bash
logvalet meta status PROJ
logvalet meta category PROJ
```

### 5. Use `digest --user` for reporting workflows

For monthly activity review:

```bash
logvalet digest --user 12345 --since 30d
```

### 6. Use `--dry-run` for write commands prepared by an agent

```bash
logvalet issue update PROJ-123 --status 3 --dry-run
logvalet issue comment add PROJ-123 --content-file ./comment.md --dry-run
logvalet document create --project PROJ --title "Runbook" --content-file ./runbook.md --dry-run
```

---

## Anti-patterns

Avoid these:

- using `--status "未対応" --status "処理中"` without `-k` — use `--status not-closed` instead (no `-k` needed, cross-project)
- using `document` commands for destructive operations
- treating `get` output as if it were equivalent to `digest`
- relying on stderr for structured data
- omitting `--profile` in environments with multiple spaces when ambiguity exists
- sending both `--api-key` and `--access-token` together

---

## Minimal command set to remember

If you only remember a few commands, remember these:

```bash
logvalet issue list --assignee me --status not-closed -f gantt
logvalet digest --issue PROJ-123 --since 30d
logvalet issue list --project-key PROJ
logvalet issue create --project-key PROJ --summary "..."
logvalet issue comment add PROJ-123 --content "..."
logvalet digest --project PROJ --since 30d
logvalet activity digest --project PROJ --since 30d
logvalet digest --user 12345 --since 30d
logvalet document digest 019b0240-4a9a-7c90-xxxx
logvalet document create --project-key PROJ --title "..." --content-file ./doc.md
```
