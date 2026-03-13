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

Explicitly unsupported in MVP:

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

- OAuth with localhost callback

Secondary auth mode:

- API key override

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
lv issue digest PROJ-123
lv project digest PROJ
lv activity digest --project PROJ --since 30d
lv document digest 12345
lv user digest 12345 --since 30d
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

### Login with OAuth

```bash
lv auth login --profile work
```

Use this to authenticate and save tokens into `~/.config/logvalet/tokens.json`.

### Show active identity

```bash
lv auth whoami --profile work
```

### List configured profiles and auth state

```bash
lv auth list
```

### Remove stored credentials for a profile

```bash
lv auth logout --profile work
```

---

## completion

Completion is generated dynamically and should be loaded with `eval` or `source`.

### zsh

Put this in `.zshrc`:

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

### bash

Put this in `.bashrc`:

```bash
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion bash --short)"
fi
```

### fish

Put this in `config.fish`:

```fish
if type -q logvalet
    logvalet completion fish --short | source
end
```

`--short` enables completion for both `logvalet` and `lv`.

---

## issue

### Get one issue

```bash
lv issue get PROJ-123
```

Use when you want structured issue details without the extra context-building behavior of `digest`.

### List issues

```bash
lv issue list --project PROJ --limit 20
```

Common filters:

```bash
lv issue list --project PROJ --assignee me
lv issue list --project PROJ --status 3
```

### Issue digest

```bash
lv issue digest PROJ-123
```

Recommended defaults:

- include recent comments
- include project metadata
- include recent activity unless `--no-activity` is set

Useful variants:

```bash
lv issue digest PROJ-123 --comments 10
lv issue digest PROJ-123 --no-activity
lv issue digest PROJ-123 -f md
```

Use this when you need:

- issue summary with context
- comment-aware reasoning
- metadata such as statuses, categories, versions, and custom fields

### Create an issue

```bash
lv issue create \
  --project PROJ \
  --summary "Fix login bug" \
  --issue-type "Bug"
```

With description file:

```bash
lv issue create \
  --project PROJ \
  --summary "Fix login bug" \
  --issue-type "Bug" \
  --description-file ./description.md
```

Review request payload first:

```bash
lv issue create \
  --project PROJ \
  --summary "Fix login bug" \
  --issue-type "Bug" \
  --dry-run
```

### Update an issue

```bash
lv issue update PROJ-123 --status 3 --assignee 12345
```

With description file:

```bash
lv issue update PROJ-123 --description-file ./description.md
```

Use `--dry-run` before update when a coding agent is preparing the change.

---

## issue comment

### List comments

```bash
lv issue comment list PROJ-123
```

### Add a comment

```bash
lv issue comment add PROJ-123 --content "I confirmed this issue."
```

From file:

```bash
lv issue comment add PROJ-123 --content-file ./comment.md
```

Dry run:

```bash
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
```

### Update a comment

```bash
lv issue comment update PROJ-123 999 --content "Updated note"
```

---

## project

### Get one project

```bash
lv project get PROJ
```

### List projects

```bash
lv project list
```

### Project digest

```bash
lv project digest PROJ
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
lv activity list --project PROJ --limit 50
```

### Activity digest

```bash
lv activity digest --project PROJ --since 30d
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
lv user list
```

Use this when you need the mapping between Backlog user IDs and names.

### Get one user

```bash
lv user get 12345
```

### User activity

```bash
lv user activity 12345 --since 30d
```

### User digest

```bash
lv user digest 12345 --since 30d
```

This is especially useful for monthly summaries of a specific user's work.

Important behavior:

- include comment-related activity
- group by project when possible
- summarize activity types and related issue keys

Recommended usage for monthly reporting:

```bash
lv user digest 12345 --since 30d -f json
lv user digest 12345 --since 30d -f md
```

---

## document

### Get one document

```bash
lv document get 12345
```

### List documents in a project

```bash
lv document list --project PROJ
```

### Get document tree

```bash
lv document tree --project PROJ
```

### Document digest

```bash
lv document digest 12345
```

### Create a document

```bash
lv document create \
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

---

## meta

Use project metadata commands when an agent needs the project dictionary.

### Statuses

```bash
lv meta status PROJ
```

### Categories

```bash
lv meta category PROJ
```

### Versions / milestones

```bash
lv meta version PROJ
```

### Custom fields

```bash
lv meta custom-field PROJ
```

Use these commands to resolve names, IDs, and valid metadata choices before creating or updating issues.

---

## team

### List teams

```bash
lv team list
```

### Teams for a project

```bash
lv team project PROJ
```

### Team digest

```bash
lv team digest 1
```

Use when you need team-to-project context or want to summarize ownership.

---

## space

### Show space info

```bash
lv space info
```

### Show disk usage

```bash
lv space disk-usage
```

### Space digest

```bash
lv space digest
```

Use for admin-oriented overview and space-level context.

---

## Recommended patterns for coding agents

### 1. Prefer digest over get for reasoning

Good:

```bash
lv issue digest PROJ-123
```

Less useful for reasoning alone:

```bash
lv issue get PROJ-123
```

### 2. Use JSON unless a human needs to read it directly

Good:

```bash
lv user digest 12345 --since 30d -f json
```

Use Markdown for sharing:

```bash
lv user digest 12345 --since 30d -f md
```

### 3. Resolve metadata before mutating issues

Typical flow:

```bash
lv meta status PROJ
lv meta category PROJ
lv meta version PROJ
lv issue create --project PROJ ...
```

### 4. Use `user digest` for reporting workflows

For monthly activity review:

```bash
lv user digest 12345 --since 30d
```

### 5. Use `--dry-run` for write commands prepared by an agent

```bash
lv issue update PROJ-123 --status 3 --dry-run
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
lv document create --project PROJ --title "Runbook" --content-file ./runbook.md --dry-run
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
lv issue digest PROJ-123
lv issue list --project PROJ
lv issue create --project PROJ --summary "..." --issue-type "Bug"
lv issue comment add PROJ-123 --content "..."
lv project digest PROJ
lv activity digest --project PROJ --since 30d
lv user digest 12345 --since 30d
lv document digest 12345
lv document create --project PROJ --title "..." --content-file ./doc.md
```

