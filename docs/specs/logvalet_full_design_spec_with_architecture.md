
# logvalet — Full Detailed Design Specification

## 1. Overview

**logvalet** is an LLM-first CLI for Backlog.  
It is designed to produce **digest-oriented structured output** for LLM workflows, rather than acting as a thin wrapper over raw Backlog APIs.

### Product identity

- Product name: `logvalet`
- Primary command: `logvalet`
- Short alias: `lv`
- Language: Go
- CLI framework: Kong
- Distribution: GitHub Releases + Homebrew tap via GoReleaser
- Agent integration: Claude Code / Codex skill via `skills/SKILL.md`

### Core principles

- **LLM-first**
- **JSON by default**
- **digest-first**
- **safe-by-default**
- **explicit write operations**
- **predictable schemas**
- **minimal hidden state**

---

## 2. Scope

### MVP supported resources

- `auth`
- `completion`
- `issue`
- `project`
- `activity`
- `user`
- `document`
- `meta`
- `team`
- `space`

### Explicitly excluded from MVP

- `wiki`
- `group`
- `document delete`
- `document update`
- `document replace`

These exclusions are intentional to reduce destructive risk and keep the initial implementation focused.

---

## 3. CLI Command Tree

```text
logvalet
├── auth
│   ├── login
│   ├── logout
│   ├── whoami
│   └── list
├── completion
│   ├── bash
│   ├── zsh
│   └── fish
├── issue
│   ├── get
│   ├── list
│   ├── digest
│   ├── create
│   ├── update
│   └── comment
│       ├── list
│       ├── add
│       └── update
├── project
│   ├── get
│   ├── list
│   └── digest
├── activity
│   ├── list
│   └── digest
├── user
│   ├── list
│   ├── get
│   ├── activity
│   └── digest
├── document
│   ├── get
│   ├── list
│   ├── tree
│   ├── digest
│   └── create
├── meta
│   ├── status
│   ├── category
│   ├── version
│   └── custom-field
├── team
│   ├── list
│   ├── project
│   └── digest
└── space
    ├── info
    ├── disk-usage
    └── digest
```

---

## 4. Global Flags

### Global flags

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

### Environment variables

All global flags must be overridable by environment variables.

```text
LOGVALET_PROFILE
LOGVALET_FORMAT
LOGVALET_PRETTY
LOGVALET_CONFIG
LOGVALET_API_KEY
LOGVALET_ACCESS_TOKEN
LOGVALET_BASE_URL
LOGVALET_SPACE
LOGVALET_VERBOSE
LOGVALET_NO_COLOR
```

Digest/list/write-specific flags may also use env vars where reasonable:

```text
LOGVALET_COMMENTS
LOGVALET_NO_ACTIVITY
LOGVALET_LIMIT
LOGVALET_OFFSET
LOGVALET_CONTENT
LOGVALET_CONTENT_FILE
LOGVALET_DRY_RUN
```

### Precedence

#### General config resolution

```text
CLI flags
> environment variables
> config.toml
> built-in defaults
```

#### Credential resolution

```text
--api-key / --access-token
> environment variables
> tokens.json
```

### Boolean env parsing

Treat the following as true:

```text
1
true
yes
on
```

Treat the following as false:

```text
0
false
no
off
```

---

## 5. Configuration and Authentication

### Configuration files

Non-secret config:

```text
~/.config/logvalet/config.toml
```

Secret credentials:

```text
~/.config/logvalet/tokens.json
```

No project-local config file is used.  
`./logvalet.toml` is intentionally not supported.

### Rationale

- avoids accidental VCS commits
- reduces config precedence complexity
- aligns better with automation and LLM usage
- keeps credentials out of the current working directory

### config.toml schema

```toml
version = 1
default_profile = "work"
default_format = "json"

[profiles.work]
space = "example-space"
base_url = "https://example-space.backlog.com"
auth_ref = "example-space"

[profiles.dev]
space = "example-dev"
base_url = "https://example-dev.backlog.com"
auth_ref = "example-dev"
```

#### Fields

- `version`: config schema version
- `default_profile`: default profile name
- `default_format`: default output format
- `profiles.<name>.space`: Backlog space name
- `profiles.<name>.base_url`: Backlog base URL
- `profiles.<name>.auth_ref`: key in `tokens.json`

### tokens.json schema

```json
{
  "version": 1,
  "auth": {
    "example-space": {
      "auth_type": "oauth",
      "access_token": "ACCESS_TOKEN",
      "refresh_token": "REFRESH_TOKEN",
      "token_expiry": "2026-03-13T15:04:05Z"
    },
    "example-dev": {
      "auth_type": "api_key",
      "api_key": "API_KEY"
    }
  }
}
```

### OAuth flow

- Authentication method: **OAuth localhost callback**
- Browser-based flow
- CLI starts a temporary localhost callback server
- Access token and refresh token are stored in `tokens.json`

### Why localhost callback

- straightforward implementation
- best UX for a local CLI
- consistent with standard authorization-code flows
- avoids inventing a pseudo-device flow

### API key support

API key support is also allowed via:

- `--api-key`
- `LOGVALET_API_KEY`
- `tokens.json`

### Auth commands

#### `lv auth login`

Purpose: login using OAuth and save credentials.

Required:

- `--profile`

Output example:

```json
{
  "schema_version": "1",
  "result": "ok",
  "profile": "work",
  "space": "example-space",
  "base_url": "https://example-space.backlog.com",
  "auth_type": "oauth",
  "saved": true
}
```

#### `lv auth logout`

Required:

- `--profile`

Output example:

```json
{
  "schema_version": "1",
  "result": "ok",
  "profile": "work",
  "removed": true
}
```

#### `lv auth whoami`

Output example:

```json
{
  "schema_version": "1",
  "profile": "work",
  "space": "example-space",
  "auth_type": "oauth",
  "user": {
    "id": 12345,
    "name": "Naoto Ishizawa"
  }
}
```

#### `lv auth list`

Output example:

```json
{
  "schema_version": "1",
  "profiles": [
    {
      "profile": "work",
      "space": "example-space",
      "base_url": "https://example-space.backlog.com",
      "auth_type": "oauth",
      "authenticated": true
    }
  ]
}
```

---

## 6. Completion

Completion is generated dynamically and loaded via `eval` or `source`.

Static completion files are intentionally not supported.

### Zsh

Add to `.zshrc`:

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

### Bash

Add to `.bashrc`:

```bash
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion bash --short)"
fi
```

### Fish

Add to `config.fish`:

```fish
if type -q logvalet
    logvalet completion fish --short | source
end
```

### Completion subcommands

```text
lv completion bash
lv completion zsh
lv completion fish
```

### `--short`

Enables completion for both:

- `logvalet`
- `lv`

### Rationale

Dynamic completion avoids stale cached scripts after CLI upgrades.

---

## 7. Output Rules

### stdout

- contains the main command result
- must remain machine-readable
- default format is `json`

### stderr

- warnings
- verbose logs
- auth refresh notices
- diagnostics

stdout must never be polluted by verbose text.

---

## 8. Exit Codes

```text
0  success
1  generic error
2  argument / validation error
3  authentication error
4  permission error
5  resource not found
6  API error
7  digest generation failed
10 configuration error
```

---

## 9. Common JSON Conventions

### Naming

- JSON keys use `snake_case`
- Go struct fields use `CamelCase` with explicit JSON tags

### Missing values

- `null` for absent scalar values
- `[]` for empty lists
- `""` for empty text content fields where appropriate

### Error envelope

For complete failure:

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

### Warning envelope

For partial success:

```json
[
  {
    "code": "project_custom_fields_fetch_failed",
    "message": "Failed to fetch custom fields.",
    "component": "project.custom_fields",
    "retryable": true
  }
]
```

---

## 10. Digest Philosophy

Digest commands are the primary value of logvalet.

### Goals

- aggregate multiple Backlog API resources
- normalize them into a stable LLM-oriented schema
- preserve enough fidelity for automation
- reduce token waste by simplifying repeated entities

### General digest structure

```text
schema_version
resource
generated_at
profile
space
base_url
warnings
digest
```

### Digest summary layers

For activity-oriented digests, structure should emphasize:

```text
activities -> raw event stream
comments   -> extracted high-signal comment content
projects   -> grouped project-level summary
summary    -> deterministic aggregate facts
llm_hints  -> low-cost structured hints
```

Comments are considered especially important and should be preserved and surfaced prominently.

---

## 11. User Schemas

### Simplified user reference

Used inside digests to reduce token usage.

```json
{
  "id": 12345,
  "name": "Naoto Ishizawa"
}
```

### Full user schema

Used by `user list` and `user get`.

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

### Rule

All digest-internal user fields use the simplified form:

- `assignee`
- `reporter`
- `author`
- `actor`
- `created_user`
- `updated_user`

---

## 12. Activity Model

Activities represent a normalized event stream.

Example:

```json
{
  "id": 1001,
  "type": "issue_commented",
  "created": "2026-03-10T10:00:00Z",
  "actor": {
    "id": 12345,
    "name": "Naoto Ishizawa"
  },
  "issue": {
    "id": 555,
    "key": "PROJ-123",
    "summary": "Login UI bug"
  },
  "comment": {
    "id": 888,
    "content": "Safari reproduces this."
  }
}
```

### Important behavior

- activity should include comment-related events
- digest should extract comments as a first-class list
- user and space activity digests should support grouping by project

---

## 13. Digest Schemas

### 13.1 Issue Digest

Command:

```text
lv issue digest <issue_key>
```

Top-level shape:

```json
{
  "schema_version": "1",
  "resource": "issue",
  "generated_at": "2026-03-13T12:34:56Z",
  "profile": "work",
  "space": "example-space",
  "base_url": "https://example-space.backlog.com",
  "warnings": [],
  "digest": {
    "issue": {},
    "project": {},
    "meta": {},
    "comments": [],
    "activity": [],
    "summary": {},
    "llm_hints": {}
  }
}
```

#### `digest.issue`

Includes:

- `id`
- `key`
- `summary`
- `description`
- `status`
- `priority`
- `issue_type`
- `assignee`
- `reporter`
- `categories`
- `versions`
- `milestones`
- `custom_fields`
- `created`
- `updated`
- `due_date`
- `start_date`

#### `digest.project`

```json
{
  "id": 1000,
  "key": "PROJ",
  "name": "Example Project"
}
```

#### `digest.meta`

Contains:

- `statuses`
- `categories`
- `versions`
- `custom_fields`

#### `digest.comments`

Recent comments, default `5`.

#### `digest.activity`

Recent project or issue-related activity if enabled.

#### `digest.summary`

Deterministic summary only.

Example:

```json
{
  "headline": "Processing bug issue assigned to Naoto Ishizawa.",
  "comment_count_included": 5,
  "activity_count_included": 3,
  "has_description": true,
  "has_assignee": true,
  "status_name": "処理中",
  "priority_name": "高"
}
```

#### `digest.llm_hints`

```json
{
  "primary_entities": ["PROJ-123", "frontend", "v1.2.0"],
  "open_questions": [],
  "suggested_next_actions": []
}
```

### 13.2 Project Digest

Command:

```text
lv project digest <project_key>
```

Contains:

- `project`
- `meta`
- `teams`
- `recent_activity`
- `summary`
- `llm_hints`

### 13.3 Activity Digest

Command:

```text
lv activity digest`
```

Contains:

- `scope`
- `activities`
- `comments`
- `projects`
- `summary`
- `llm_hints`

Comments must be extracted from the activity stream.  
Project grouping must be included where possible.

### 13.4 User Digest

Command:

```text
lv user digest <user_id> --since 30d
```

Recommended shape:

```json
{
  "schema_version": "1",
  "resource": "user",
  "generated_at": "2026-03-13T12:34:56Z",
  "profile": "work",
  "space": "example-space",
  "base_url": "https://example-space.backlog.com",
  "warnings": [],
  "digest": {
    "user": {
      "id": 12345,
      "name": "Naoto Ishizawa"
    },
    "scope": {
      "since": "2026-02-10T00:00:00Z",
      "until": "2026-03-10T00:00:00Z"
    },
    "activities": [],
    "comments": [],
    "projects": {
      "PROJ": {
        "activity_count": 30,
        "comment_count": 12
      }
    },
    "summary": {
      "headline": "User activity digest for Naoto Ishizawa",
      "total_activity": 42,
      "comment_count": 18,
      "types": {
        "issue_created": 3,
        "issue_updated": 21,
        "issue_commented": 18
      },
      "related_issue_keys": ["PROJ-123", "PROJ-140"],
      "related_project_keys": ["PROJ", "OPS"]
    },
    "llm_hints": {
      "primary_entities": ["PROJ-123", "PROJ-140", "PROJ", "OPS"],
      "open_questions": [],
      "suggested_next_actions": []
    }
  }
}
```

This command is explicitly intended to support monthly summaries of a specific user's activity.

### 13.5 Document Digest

Contains:

- `document`
- `project`
- `attachments`
- `summary`
- `llm_hints`

### 13.6 Team Digest

Contains:

- `team`
- `projects`
- `summary`
- `llm_hints`

### 13.7 Space Digest

Contains:

- `space`
- `disk_usage`
- `summary`
- `llm_hints`

---

## 14. Command-by-Command I/O Contracts

### 14.1 `lv issue get <issue_key>`

Purpose: fetch structured issue details.

Output: structured issue JSON, not a digest.

### 14.2 `lv issue list`

Flags:

- `--project <key>`
- `--assignee <id|me>`
- `--status <id>`
- `--limit`
- `--offset`

Output:

- `items`
- `count`
- `limit`
- `offset`

### 14.3 `lv issue digest <issue_key>`

Flags:

- `--comments <n>` default `5`
- `--no-activity`

Returns issue digest schema.

### 14.4 `lv issue create`

Required:

- `--project <key>`
- `--summary <text>`
- `--issue-type <name|id>`

Optional:

- `--description`
- `--description-file`
- `--priority`
- `--assignee`
- `--category`
- `--version`
- `--milestone`
- `--due-date`
- `--start-date`
- `--custom-field`
- `--dry-run`

Rules:

- `--description` and `--description-file` cannot be combined

### 14.5 `lv issue update <issue_key>`

At least one update field is required.

If none are provided, return exit code `2`.

### 14.6 `lv issue comment list <issue_key>`

Returns paginated comment list.

### 14.7 `lv issue comment add <issue_key>`

Required:

- `--content` or `--content-file`

Optional:

- `--dry-run`

### 14.8 `lv issue comment update <issue_key> <comment_id>`

Required:

- `--content` or `--content-file`

### 14.9 `lv project get <project_key>`

Returns structured project JSON.

### 14.10 `lv project list`

Returns project list JSON.

### 14.11 `lv project digest <project_key>`

Returns project digest.

### 14.12 `lv activity list`

Flags:

- `--project <key>`
- `--limit`
- `--offset`
- `--since <duration|timestamp>`

### 14.13 `lv activity digest`

Flags:

- `--project <key>`
- `--limit`
- `--since <duration|timestamp>`

Should include comments extracted from activities and project grouping.

### 14.14 `lv user list`

Returns full user list.

This is the only place where the full user schema is broadly returned.

### 14.15 `lv user get <user_id>`

Returns full user details.

### 14.16 `lv user activity <user_id>`

Flags:

- `--since <duration|timestamp>`
- `--limit <n>`
- `--project <key>`
- `--type <activity_type>` (optional extension)

Purpose: retrieve user-scoped activity stream.

### 14.17 `lv user digest <user_id>`

Flags:

- `--since <duration|timestamp>`
- `--limit <n>`
- `--project <key>`

Purpose: summarize a user's recent activity, including comments and project grouping.

### 14.18 `lv document get <document_id>`

Returns structured document JSON.

### 14.19 `lv document list`

Required:

- `--project <key>`

### 14.20 `lv document tree`

Required:

- `--project <key>`

Returns hierarchical document tree.

### 14.21 `lv document digest <document_id>`

Returns document digest.

### 14.22 `lv document create`

Required:

- `--project <key>`
- `--title <text>`
- `--content` or `--content-file`

Optional:

- `--parent-id`
- `--dry-run`

### 14.23 `lv meta status <project_key>`

Returns project statuses.

### 14.24 `lv meta category <project_key>`

Returns project categories.

### 14.25 `lv meta version <project_key>`

Returns project versions / milestones.

### 14.26 `lv meta custom-field <project_key>`

Returns project custom field definitions.

### 14.27 `lv team list`

Returns space-wide team list.

### 14.28 `lv team project <project_key>`

Returns teams attached to a project.

### 14.29 `lv team digest <team_id|name>`

Returns team digest.

### 14.30 `lv space info`

Returns space metadata.

### 14.31 `lv space disk-usage`

Returns disk usage metadata.

### 14.32 `lv space digest`

Returns space digest.

---

## 15. Backlog API Mapping

Primary endpoints expected:

```text
/api/v2/users
/api/v2/users/:id
/api/v2/users/:id/activities

/api/v2/issues
/api/v2/issues/:idOrKey
/api/v2/issues/:idOrKey/comments

/api/v2/projects
/api/v2/projects/:idOrKey

/api/v2/space
/api/v2/space/diskUsage
/api/v2/space/activities

/api/v2/documents
/api/v2/documents/:id
/api/v2/documents/:id/attachments
/api/v2/documents/:projectId/tree

/api/v2/projects/:projectId/statuses
/api/v2/projects/:projectId/categories
/api/v2/projects/:projectId/versions
/api/v2/projects/:projectId/customFields

/api/v2/teams
/api/v2/projects/:projectId/teams
```

Pagination should be abstracted by the CLI where needed.

---

## 16. Go Directory Structure

This section defines the recommended implementation layout.

```text
logvalet/
├── cmd/
│   └── lv/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go
│   ├── backlog/
│   │   ├── client.go
│   │   ├── auth.go
│   │   ├── issues.go
│   │   ├── projects.go
│   │   ├── activities.go
│   │   ├── users.go
│   │   ├── documents.go
│   │   ├── meta.go
│   │   ├── teams.go
│   │   └── space.go
│   ├── cli/
│   │   ├── root.go
│   │   ├── global_flags.go
│   │   ├── auth.go
│   │   ├── completion.go
│   │   ├── issue.go
│   │   ├── project.go
│   │   ├── activity.go
│   │   ├── user.go
│   │   ├── document.go
│   │   ├── meta.go
│   │   ├── team.go
│   │   └── space.go
│   ├── config/
│   │   ├── config.go
│   │   ├── loader.go
│   │   └── resolver.go
│   ├── credentials/
│   │   ├── store.go
│   │   ├── oauth.go
│   │   └── resolver.go
│   ├── digest/
│   │   ├── issue.go
│   │   ├── project.go
│   │   ├── activity.go
│   │   ├── user.go
│   │   ├── document.go
│   │   ├── team.go
│   │   ├── space.go
│   │   └── common.go
│   ├── domain/
│   │   ├── issue.go
│   │   ├── project.go
│   │   ├── activity.go
│   │   ├── user.go
│   │   ├── document.go
│   │   ├── team.go
│   │   ├── space.go
│   │   ├── warning.go
│   │   └── error.go
│   ├── render/
│   │   ├── json.go
│   │   ├── yaml.go
│   │   ├── markdown.go
│   │   ├── text.go
│   │   └── render.go
│   ├── version/
│   │   └── version.go
│   └── util/
│       ├── time.go
│       ├── files.go
│       └── validate.go
├── skills/
│   └── SKILL.md
├── .github/
│   └── workflows/
│       └── release.yml
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── README.md
└── README.ja.md
```

### Package responsibilities

#### `cmd/lv`
Program entrypoint only.

#### `internal/app`
Application wiring, dependency composition, command bootstrapping helpers.

#### `internal/backlog`
Raw Backlog API client and endpoint-specific request methods.

#### `internal/cli`
Kong command structs, argument parsing, command execution adapters.

#### `internal/config`
Config file schema, loading, profile resolution.

#### `internal/credentials`
Token store access, OAuth flow, credential resolution.

#### `internal/digest`
Digest builders and aggregation logic.

#### `internal/domain`
Stable internal models and shared types.

#### `internal/render`
Format-specific renderers.

#### `internal/version`
Version metadata injected by GoReleaser.

#### `internal/util`
Small shared helpers.

---

## 17. Kong CLI Struct Skeleton

The following is the recommended command struct layout.

### 17.1 Global flags

```go
type GlobalFlags struct {
    Profile     string `help:"Profile name." short:"p" env:"LOGVALET_PROFILE"`
    Format      string `help:"Output format." enum:"json,md,text,yaml" default:"json" short:"f" env:"LOGVALET_FORMAT"`
    Pretty      bool   `help:"Pretty-print output." env:"LOGVALET_PRETTY"`
    Config      string `help:"Path to config file." short:"c" env:"LOGVALET_CONFIG"`
    APIKey      string `help:"Override API key." env:"LOGVALET_API_KEY"`
    AccessToken string `help:"Override access token." env:"LOGVALET_ACCESS_TOKEN"`
    BaseURL     string `help:"Override Backlog base URL." env:"LOGVALET_BASE_URL"`
    Space       string `help:"Override Backlog space." short:"s" env:"LOGVALET_SPACE"`
    Verbose     bool   `help:"Verbose logging." short:"v" env:"LOGVALET_VERBOSE"`
    NoColor     bool   `help:"Disable colored output." env:"LOGVALET_NO_COLOR"`
}
```

### 17.2 Shared option groups

```go
type DigestFlags struct {
    Comments   int  `help:"Number of recent comments to include." default:"5" env:"LOGVALET_COMMENTS"`
    NoActivity bool `help:"Do not include recent activity." env:"LOGVALET_NO_ACTIVITY"`
}

type ListFlags struct {
    Limit  int `help:"Maximum number of items to return." default:"20" env:"LOGVALET_LIMIT"`
    Offset int `help:"Offset for pagination." default:"0" env:"LOGVALET_OFFSET"`
}

type WriteFlags struct {
    Content     string `help:"Inline content." env:"LOGVALET_CONTENT"`
    ContentFile string `help:"Read content from file." type:"path" env:"LOGVALET_CONTENT_FILE"`
    DryRun      bool   `help:"Print request payload without executing." env:"LOGVALET_DRY_RUN"`
}
```

### 17.3 Root command skeleton

```go
type CLI struct {
    GlobalFlags

    Auth       AuthCmd       `cmd:"" help:"Authentication commands."`
    Completion CompletionCmd `cmd:"" help:"Shell completion."`
    Issue      IssueCmd      `cmd:"" help:"Issue commands."`
    Project    ProjectCmd    `cmd:"" help:"Project commands."`
    Activity   ActivityCmd   `cmd:"" help:"Activity commands."`
    User       UserCmd       `cmd:"" help:"User commands."`
    Document   DocumentCmd   `cmd:"" help:"Document commands."`
    Meta       MetaCmd       `cmd:"" help:"Project metadata commands."`
    Team       TeamCmd       `cmd:"" help:"Team commands."`
    Space      SpaceCmd      `cmd:"" help:"Space commands."`
}
```

### 17.4 Example issue command skeleton

```go
type IssueCmd struct {
    Get     IssueGetCmd     `cmd:"" help:"Get issue details."`
    List    IssueListCmd    `cmd:"" help:"List issues."`
    Digest  IssueDigestCmd  `cmd:"" help:"Generate issue digest."`
    Create  IssueCreateCmd  `cmd:"" help:"Create an issue."`
    Update  IssueUpdateCmd  `cmd:"" help:"Update an issue."`
    Comment IssueCommentCmd `cmd:"" help:"Issue comment commands."`
}

type IssueGetCmd struct {
    IssueKey string `arg:"" name:"issue_key" help:"Issue key."`
}

type IssueListCmd struct {
    ListFlags
    Project  string `help:"Project key."`
    Assignee string `help:"Assignee ID or 'me'."`
    Status   string `help:"Status ID."`
}

type IssueDigestCmd struct {
    DigestFlags
    IssueKey string `arg:"" name:"issue_key" help:"Issue key."`
}

type IssueCreateCmd struct {
    WriteFlags
    Project     string   `help:"Project key." required:""`
    Summary     string   `help:"Issue summary." required:""`
    IssueType   string   `help:"Issue type ID or name." required:""`
    Description string   `help:"Inline description."`
    Priority    string   `help:"Priority ID or name."`
    Assignee    string   `help:"Assignee ID."`
    Category    []string `help:"Category ID."`
    Version     []string `help:"Version ID."`
    Milestone   []string `help:"Milestone ID."`
    DueDate     string   `help:"Due date (YYYY-MM-DD)."`
    StartDate   string   `help:"Start date (YYYY-MM-DD)."`
    CustomField []string `help:"Custom field pair, e.g. key=value."`
}

type IssueUpdateCmd struct {
    WriteFlags
    IssueKey     string   `arg:"" name:"issue_key" help:"Issue key."`
    Summary      string   `help:"Issue summary."`
    Description  string   `help:"Inline description."`
    Status       string   `help:"Status ID or name."`
    Priority     string   `help:"Priority ID or name."`
    Assignee     string   `help:"Assignee ID."`
    Category     []string `help:"Category ID."`
    Version      []string `help:"Version ID."`
    Milestone    []string `help:"Milestone ID."`
    DueDate      string   `help:"Due date (YYYY-MM-DD)."`
    StartDate    string   `help:"Start date (YYYY-MM-DD)."`
    CustomField  []string `help:"Custom field pair, e.g. key=value."`
}
```

### Validation rules

- `--content` and `--content-file` are mutually exclusive
- `--description` and `--description-file` are mutually exclusive
- `--api-key` and `--access-token` must not both be provided
- `issue update` must fail with exit code `2` if no update fields are given

---

## 18. Backlog API Client Interface

The client layer should expose stable, testable interfaces.

### 18.1 Primary interface

```go
type Client interface {
    // Auth / user identity
    GetMyself(ctx context.Context) (*User, error)
    ListUsers(ctx context.Context) ([]User, error)
    GetUser(ctx context.Context, userID string) (*User, error)
    ListUserActivities(ctx context.Context, userID string, opt ListUserActivitiesOptions) ([]Activity, error)

    // Issues
    GetIssue(ctx context.Context, issueKey string) (*Issue, error)
    ListIssues(ctx context.Context, opt ListIssuesOptions) ([]Issue, error)
    CreateIssue(ctx context.Context, req CreateIssueRequest) (*Issue, error)
    UpdateIssue(ctx context.Context, issueKey string, req UpdateIssueRequest) (*Issue, error)

    // Issue comments
    ListIssueComments(ctx context.Context, issueKey string, opt ListCommentsOptions) ([]Comment, error)
    AddIssueComment(ctx context.Context, issueKey string, req AddCommentRequest) (*Comment, error)
    UpdateIssueComment(ctx context.Context, issueKey string, commentID int64, req UpdateCommentRequest) (*Comment, error)

    // Projects
    GetProject(ctx context.Context, projectKey string) (*Project, error)
    ListProjects(ctx context.Context) ([]Project, error)
    ListProjectActivities(ctx context.Context, projectKey string, opt ListActivitiesOptions) ([]Activity, error)

    // Space activities
    ListSpaceActivities(ctx context.Context, opt ListActivitiesOptions) ([]Activity, error)

    // Documents
    GetDocument(ctx context.Context, documentID int64) (*Document, error)
    ListDocuments(ctx context.Context, projectKey string, opt ListDocumentsOptions) ([]Document, error)
    GetDocumentTree(ctx context.Context, projectKey string) ([]DocumentNode, error)
    CreateDocument(ctx context.Context, req CreateDocumentRequest) (*Document, error)
    ListDocumentAttachments(ctx context.Context, documentID int64) ([]Attachment, error)

    // Project meta
    ListProjectStatuses(ctx context.Context, projectKey string) ([]Status, error)
    ListProjectCategories(ctx context.Context, projectKey string) ([]Category, error)
    ListProjectVersions(ctx context.Context, projectKey string) ([]Version, error)
    ListProjectCustomFields(ctx context.Context, projectKey string) ([]CustomFieldDefinition, error)

    // Teams
    ListTeams(ctx context.Context) ([]Team, error)
    ListProjectTeams(ctx context.Context, projectKey string) ([]Team, error)

    // Space
    GetSpace(ctx context.Context) (*Space, error)
    GetSpaceDiskUsage(ctx context.Context) (*DiskUsage, error)
}
```

### 18.2 Request option types

```go
type ListIssuesOptions struct {
    ProjectKey string
    Assignee   string
    Status     string
    Limit      int
    Offset     int
}

type ListCommentsOptions struct {
    Limit  int
    Offset int
}

type ListActivitiesOptions struct {
    ProjectKey string
    Since      *time.Time
    Until      *time.Time
    Limit      int
    Offset     int
}

type ListUserActivitiesOptions struct {
    Since   *time.Time
    Until   *time.Time
    Limit   int
    Offset  int
    Project string
    Types   []string
}

type ListDocumentsOptions struct {
    Limit  int
    Offset int
}
```

### 18.3 Write request types

```go
type CreateIssueRequest struct {
    ProjectKey   string
    Summary      string
    IssueType    string
    Description  string
    Priority     string
    Assignee     string
    Categories   []string
    Versions     []string
    Milestones   []string
    DueDate      *time.Time
    StartDate    *time.Time
    CustomFields map[string]string
}

type UpdateIssueRequest struct {
    Summary      *string
    Description  *string
    Status       *string
    Priority     *string
    Assignee     *string
    Categories   []string
    Versions     []string
    Milestones   []string
    DueDate      *time.Time
    StartDate    *time.Time
    CustomFields map[string]string
}

type AddCommentRequest struct {
    Content string
}

type UpdateCommentRequest struct {
    Content string
}

type CreateDocumentRequest struct {
    ProjectKey string
    Title      string
    Content    string
    ParentID   *int64
}
```

### 18.4 Error handling expectations

The client should normalize API errors into typed errors such as:

- `ErrNotFound`
- `ErrUnauthorized`
- `ErrForbidden`
- `ErrRateLimited`
- `ErrValidation`
- `ErrAPI`

This allows CLI exit code mapping to remain consistent.

---

## 19. Digest Builder Interfaces

Digest generation should remain independent from the raw API client.

```go
type IssueDigestBuilder interface {
    Build(ctx context.Context, issueKey string, opt IssueDigestOptions) (*IssueDigest, error)
}

type ProjectDigestBuilder interface {
    Build(ctx context.Context, projectKey string, opt ProjectDigestOptions) (*ProjectDigest, error)
}

type ActivityDigestBuilder interface {
    Build(ctx context.Context, opt ActivityDigestOptions) (*ActivityDigest, error)
}

type UserDigestBuilder interface {
    Build(ctx context.Context, userID string, opt UserDigestOptions) (*UserDigest, error)
}

type DocumentDigestBuilder interface {
    Build(ctx context.Context, documentID int64, opt DocumentDigestOptions) (*DocumentDigest, error)
}
```

### Example options

```go
type IssueDigestOptions struct {
    MaxComments        int
    IncludeActivity    bool
}

type ActivityDigestOptions struct {
    Project string
    Since   *time.Time
    Until   *time.Time
    Limit   int
}

type UserDigestOptions struct {
    Since   *time.Time
    Until   *time.Time
    Limit   int
    Project string
}
```

### Partial success behavior

Digest builders should prefer partial success:

- collect warnings where optional sub-fetches fail
- return a usable digest whenever the core subject was fetched successfully

---

## 20. Rendering Layer

Supported formats:

- `json`
- `md`
- `text`
- `yaml`

### Renderer interface

```go
type Renderer interface {
    Render(v any) ([]byte, error)
}
```

### Expectations

#### JSON
- default
- stable schema
- pretty-print optional

#### Markdown
- human-readable
- bullet-oriented
- fixed section ordering
- avoid wide tables unless clearly useful

#### Text
- compact terminal-oriented summary

#### YAML
- primarily for debugging and config-adjacent workflows

---

## 21. GoReleaser

Release tooling should follow the style used in `youyo/ccmix`.

### Goals

- build multi-platform binaries
- generate archives
- generate checksums
- create GitHub Releases
- publish Homebrew formula using GitHub App token

### `.goreleaser.yaml`

```yaml
project_name: logvalet

before:
  hooks:
    - go mod tidy

builds:
  - id: logvalet
    main: ./cmd/lv/main.go
    binary: logvalet
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X github.com/youyo/logvalet/internal/version.Version={{ .Version }}
      - -X github.com/youyo/logvalet/internal/version.Commit={{ .Commit }}
      - -X github.com/youyo/logvalet/internal/version.Date={{ .Date }}

archives:
  - id: default
    builds:
      - logvalet
    formats:
      - tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- title .Os }}_
      {{- if eq .Arch "amd64" }}x86_64
      {{- else if eq .Arch "arm64" }}arm64
      {{- else }}{{ .Arch }}{{ end }}

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  use: github

brews:
  - name: logvalet
    repository:
      owner: youyo
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/youyo/logvalet"
    description: "LLM-first Backlog CLI with digest-oriented output"
    license: "MIT"
    install: |
      bin.install "logvalet"
    test: |
      system "#{bin}/logvalet", "--help"

release:
  github:
    owner: youyo
    name: logvalet
```

---

## 22. GitHub Actions Release Pipeline

### `.github/workflows/release.yml`

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Generate GitHub App token
        id: app-token
        uses: tibdex/github-app-token@v2
        with:
          app_id: ${{ secrets.APP_ID }}
          private_key: ${{ secrets.APP_PRIVATE_KEY }}

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ github.token }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}
```

### Secrets

Required GitHub Actions secrets:

- `APP_ID`
- `APP_PRIVATE_KEY`

### Notes

- GitHub Release creation uses `GITHUB_TOKEN`
- Homebrew tap publishing uses GitHub App token

---

## 23. Version Metadata

Version information should be injected by GoReleaser and exposed in a version package.

### Example

```go
package version

var (
    Version = "dev"
    Commit  = "none"
    Date    = "unknown"
)
```

A future `lv version` command is recommended even if not included in the initial MVP.

---

## 24. Skills Integration

A single skill file is sufficient for agent usage.

### Repository path

```text
skills/SKILL.md
```

### Claude Code install target

```text
.claude/skills/logvalet/SKILL.md
```

### Purpose

- explain what logvalet is
- document key commands
- explain digest-first usage
- explain safety rules
- explain auth/profile/env usage

This file should be copyable as-is for Claude Code or similar agents.

---

## 25. Testing Strategy

### Unit tests

- config loader
- credential resolver
- flag validation
- digest builders
- renderers

### Integration tests

- mocked Backlog API client
- command execution end-to-end for major commands

### Golden tests

Recommended for:

- JSON digest output
- Markdown output
- Text output

### High-priority test targets

- `issue digest`
- `user digest --since 30d`
- `activity digest --project PROJ`
- `document create --dry-run`
- `issue comment add`

---

## 26. Recommended Implementation Order

### Phase 1
- config loading
- credential loading
- backlog client scaffold
- root CLI with Kong
- JSON rendering
- auth commands

### Phase 2
- issue get/list/digest
- project get/list/digest
- meta commands

### Phase 3
- issue create/update
- issue comment list/add/update
- document get/list/tree/digest/create

### Phase 4
- activity list/digest
- user list/get/activity/digest
- team and space commands

### Phase 5
- GoReleaser
- release workflow
- README
- skill file

---

## 27. Design Summary

logvalet should be understood as:

> **A digest-oriented, LLM-first interface layer for Backlog**

It is not merely a raw API wrapper.

Its main value is:

- predictable schemas
- digest-centric context generation
- comment-aware activity summarization
- safe write support for issues and comments
- low-friction automation through JSON and env overrides

This design is intended to be sufficiently concrete for a coding agent to begin implementation directly.
