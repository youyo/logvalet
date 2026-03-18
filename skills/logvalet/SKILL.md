---
name: logvalet
description: Use logvalet (lv) to read, summarize, and safely update Backlog with LLM-friendly JSON digests.
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
- inspect a specific user’s activity over a period, including comments
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
--limit <n>
--offset <n>
```

Write-oriented flags:

```text
--content <string>
--content-file <path>
--dry-run
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
logvalet issue digest PROJ-123
logvalet project digest PROJ
logvalet activity digest --project PROJ --since 30d
logvalet document digest 019b0240-4a9a-7c90-xxxx
logvalet user digest 12345 --since 30d
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
```

### Issue digest

```bash
logvalet issue digest PROJ-123
```

Recommended defaults:

- include recent comments
- include project metadata
- include recent activity unless `--no-activity` is set

Useful variants:

```bash
logvalet issue digest PROJ-123 --comments 10
logvalet issue digest PROJ-123 --no-activity
logvalet issue digest PROJ-123 -f md
```

Use this when you need:

- issue summary with context
- comment-aware reasoning
- metadata such as statuses, categories, versions, and custom fields

### Create an issue

```bash
logvalet issue create \
  --project PROJ \
  --summary "Fix login bug" \
  --issue-type "Bug"
```

With description file:

```bash
logvalet issue create \
  --project PROJ \
  --summary "Fix login bug" \
  --issue-type "Bug" \
  --description-file ./description.md
```

Review request payload first:

```bash
logvalet issue create \
  --project PROJ \
  --summary "Fix login bug" \
  --issue-type "Bug" \
  --dry-run
```

### Update an issue

```bash
logvalet issue update PROJ-123 --status 3 --assignee 12345
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

```bash
logvalet issue comment add PROJ-123 --content "I confirmed this issue."
```

From file:

```bash
logvalet issue comment add PROJ-123 --content-file ./comment.md
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

### Project digest

```bash
logvalet project digest PROJ
```

Use when you need:

- project metadata
- associated teams
- recent activity
- a compact LLM-oriented project context

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
```

### User digest

```bash
logvalet user digest 12345 --since 30d
```

This is especially useful for monthly summaries of a specific user's work.

Important behavior:

- include comment-related activity
- group by project when possible
- summarize activity types and related issue keys

Recommended usage for monthly reporting:

```bash
logvalet user digest 12345 --since 30d -f json
logvalet user digest 12345 --since 30d -f md
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

```bash
logvalet document create \
  --project PROJ \
  --title "Runbook" \
  --content-file ./runbook.md
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
```

### Teams for a project

```bash
logvalet team project PROJ
```

### Team digest

```bash
logvalet team digest 1
```

Use when you need team-to-project context or want to summarize ownership.

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

### 1. Prefer digest over get for reasoning

Good:

```bash
logvalet issue digest PROJ-123
```

Less useful for reasoning alone:

```bash
logvalet issue get PROJ-123
```

### 2. Use JSON unless a human needs to read it directly

Good:

```bash
logvalet user digest 12345 --since 30d -f json
```

Use Markdown for sharing:

```bash
logvalet user digest 12345 --since 30d -f md
```

### 3. Resolve metadata before mutating issues

Typical flow:

```bash
logvalet meta status PROJ
logvalet meta category PROJ
logvalet meta version PROJ
logvalet issue create --project PROJ ...
```

### 4. Use `user digest` for reporting workflows

For monthly activity review:

```bash
logvalet user digest 12345 --since 30d
```

### 5. Use `--dry-run` for write commands prepared by an agent

```bash
logvalet issue update PROJ-123 --status 3 --dry-run
logvalet issue comment add PROJ-123 --content-file ./comment.md --dry-run
logvalet document create --project PROJ --title "Runbook" --content-file ./runbook.md --dry-run
```

---

## Anti-patterns

Avoid these:

- using `document` commands for destructive operations
- treating `get` output as if it were equivalent to `digest`
- relying on stderr for structured data
- omitting `--profile` in environments with multiple spaces when ambiguity exists
- sending both `--api-key` and `--access-token` together

---

## Minimal command set to remember

If you only remember a few commands, remember these:

```bash
logvalet issue digest PROJ-123
logvalet issue list --project-key PROJ
logvalet issue create --project PROJ --summary "..." --issue-type "Bug"
logvalet issue comment add PROJ-123 --content "..."
logvalet project digest PROJ
logvalet activity digest --project PROJ --since 30d
logvalet user digest 12345 --since 30d
logvalet document digest 019b0240-4a9a-7c90-xxxx
logvalet document create --project PROJ --title "..." --content-file ./doc.md
```

