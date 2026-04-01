# logvalet

**logvalet** is an LLM-first CLI for [Backlog](https://backlog.com/).

It is not a thin API wrapper. Its primary purpose is to turn Backlog data into **stable, compact, machine-readable digest JSON** that works well for Claude Code, Codex, and other coding agents.

## Installation

### Homebrew

```bash
brew install youyo/tap/logvalet
```

### go install

```bash
go install github.com/youyo/logvalet/cmd/logvalet@latest
```

The installed binary is named `logvalet`. You can alias it to `lv` in your shell:

```bash
alias lv=logvalet
```

## Quick Start

### 1. Setup

```bash
logvalet config init --init-profile work --init-space myspace --init-api-key YOUR_API_KEY
```

Or interactively:

```bash
logvalet config init
```

### 2. Verify

```bash
logvalet auth whoami
```

### 3. Get a digest

```bash
# Single issue
logvalet digest --issue PROJ-123

# Project + user for this month
logvalet digest --project PROJ --user me --since this-month

# Team for this week
logvalet digest --team 173843 --since this-week
```

## Configuration

Run `logvalet config init` to create the configuration interactively, or use flags for non-interactive setup:

```bash
logvalet config init --init-profile work --init-space myspace --init-api-key YOUR_API_KEY
```

Configuration file:

```text
~/.config/logvalet/config.toml
```

Token store:

```text
~/.config/logvalet/tokens.json
```

## Shell Completion

Add this to `.zshrc`:

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

This enables completion for both `logvalet` and `lv`.

## Commands

| Command | Description |
|---------|-------------|
| `auth login` | Authenticate with API key |
| `auth logout` | Remove stored credentials |
| `auth whoami` | Show current identity |
| `auth list` | List configured profiles |
| `completion bash/zsh/fish` | Generate shell completion scripts |
| `digest` | Generate digest for issues, projects, users, teams, or space |
| `issue get <KEY>` | Get a single issue |
| `issue list` | List issues with filters |
| `issue create` | Create a new issue |
| `issue update <KEY>` | Update an existing issue |
| `issue comment list <KEY>` | List issue comments |
| `issue comment add <KEY>` | Add a comment to an issue |
| `issue comment update <KEY> <ID>` | Update a comment |
| `issue attachment list <KEY>` | List issue attachments |
| `issue attachment get <KEY> <ID>` | Get attachment info |
| `issue attachment download <KEY> <ID>` | Download an attachment |
| `issue attachment delete <KEY> <ID>` | Delete an attachment |
| `issue context <KEY>` | Get full context for a single issue (details, comments, signals) |
| `issue stale` | Detect stale issues in a project |
| `project get <KEY>` | Get a single project |
| `project list` | List all projects |
| `project blockers <KEY>` | Detect project blockers (stale, unassigned, overdue) |
| `project health <KEY>` | Integrated project health view |
| `user workload <KEY>` | Analyze user workload distribution |
| `activity list` | List activity events |
| `user list` | List space users |
| `user get <ID>` | Get a single user |
| `user activity <ID>` | Get user activity |
| `document get <ID>` | Get a single document |
| `document list` | List documents in a project |
| `document tree` | Get document tree |
| `document create` | Create a new document |
| `meta status <KEY>` | List project statuses |
| `meta category <KEY>` | List project categories |
| `meta version <KEY>` | List project versions |
| `meta custom-field <KEY>` | List project custom fields |
| `team list` | List all teams |
| `team project <KEY>` | List teams for a project |
| `space info` | Show space information |
| `space disk-usage` | Show disk usage |
| `shared-file list` | List shared files in a project |
| `shared-file get <FILE-ID>` | Get shared file info |
| `shared-file download <FILE-ID>` | Download a shared file |
| `star add` | Add a star (issue, comment, wiki, PR, etc.) |
| `mcp` | Start MCP server (Streamable HTTP) |
| `config init` | Interactive configuration setup |
| `configure` | Alias for config init |
| `version` | Show version information |

## AI Analysis Commands

Phase 1 added a set of AI-oriented analysis commands for project insight and decision support:

| Command | Description |
|---------|-------------|
| `issue context <KEY>` | Fetch full context for a single issue: details, comments, and analysis signals |
| `issue stale -k <PROJECT>` | Detect issues that haven't been updated for N days |
| `project blockers <PROJECT>` | Detect blockers: stale high-priority, unassigned, or overdue issues |
| `user workload <PROJECT>` | Analyze per-user open issue counts and overdue distribution |
| `project health <PROJECT>` | Integrated health view combining stale detection, blockers, and workload |

### Examples

```bash
# Get full context for an issue
logvalet issue context PROJ-123

# Detect issues stale for 7+ days
logvalet issue stale -k PROJ --days 7

# Detect project blockers including comments
logvalet project blockers PROJ --days 14 --include-comments

# Analyze user workload, excluding closed statuses
logvalet user workload PROJ --exclude-status "完了,却下"

# Full project health report
logvalet project health PROJ --days 7
```

---

## Global Flags

```text
--profile, -p <name>     Profile to use
--format, -f <format>    Output format: json (default), yaml, md, gantt
--pretty                 Pretty-print JSON output
--config, -c <path>      Config file path
--api-key <key>          Backlog API key
--access-token <token>   OAuth access token
--base-url <url>         Backlog base URL
--space, -s <space>      Space key
--verbose, -v            Verbose output
--no-color               Disable color output
```

## Issue Filtering

Filter issues by assignee, status, and due date:

```bash
# List my open issues
logvalet issue list --assignee me --status open -k PROJECT_KEY

# List issues assigned to a specific user
logvalet issue list --assignee "Taro Tanaka" -k PROJECT_KEY

# List issues assigned to team members (by team name or partial team name)
logvalet issue list --assignee "ヘプタゴン" --status not-closed --due-date this-week

# List overdue issues
logvalet issue list --assignee me --due-date overdue -k PROJECT_KEY

# List issues due today
logvalet issue list --assignee me --due-date today -k PROJECT_KEY

# Filter by specific status names
logvalet issue list --status "未対応,処理中" -k PROJECT_KEY

# Filter by status ID
logvalet issue list --status 1

# List all non-closed issues (no project key required)
logvalet issue list --status not-closed

# List issues due this month
logvalet issue list --due-date this-month

# List issues due this week, sorted by due date (ascending)
logvalet issue list --due-date this-week --sort dueDate --order asc

# List issues in a specific date range
logvalet issue list --due-date 2026-03-01:2026-03-31

# List issues due on or after a specific date
logvalet issue list --due-date 2026-03-20:

# List issues due on or before a specific date
logvalet issue list --due-date :2026-03-31

# Combine filters: my non-closed issues, sorted by due date
logvalet issue list --assignee me --status not-closed --sort dueDate --order asc

# Filter by start date (issues starting this month)
logvalet issue list --start-date this-month

# Filter by start date range
logvalet issue list --start-date 2026-03-01:2026-03-31

# Combine --start-date and --due-date (AND condition)
logvalet issue list --start-date this-month --due-date this-month
```

| Flag | Values | Description |
|------|--------|-------------|
| `--assignee` | `me`, user ID, user name, or team name | Filter by assignee. Specify a team name (partial match supported) to filter by all team members. |
| `--status` | `open`, `not-closed`, status name(s), or status ID | Filter by status. `open` excludes completed. `not-closed` excludes completed (no project key required). Names/`open` require `-k` |
| `--due-date` | `today`, `overdue`, `this-week`, `this-month`, `YYYY-MM-DD`, or `YYYY-MM-DD:YYYY-MM-DD` | Filter by due date. Date ranges support open-ended queries (`:YYYY-MM-DD` or `YYYY-MM-DD:`) |
| `--start-date` | `today`, `this-week`, `this-month`, `YYYY-MM-DD`, or `YYYY-MM-DD:YYYY-MM-DD` | Filter by start date. Date ranges support open-ended queries. Can be combined with `--due-date` (AND). |
| `--sort` | `dueDate`, `created`, `updated`, `priority`, `status`, `assignee` | Sort results by field |
| `--order` | `asc`, `desc` | Sort direction. Default: `desc` |

Note: When using `--due-date` or `--start-date`, results are automatically paginated to retrieve all matching issues (up to 10,000 total).

## Digest Command

The `digest` command generates stable, structured summaries of Backlog data for a time window. It supports filtering by project, user, team, or issue, and returns a compact machine-readable format optimized for LLM agents.

### Examples

```bash
# Single issue with context
logvalet digest --issue PROJ-123

# Project + user activity for this month
logvalet digest --project HEP_ISSUES --user "Naoto Ishizawa" --since this-month

# Multiple projects and users (AND condition)
logvalet digest --project HEP_ISSUES --project TAISEI --user "Ishizawa" --user "Sugo" --since this-month

# Team digest for this week
logvalet digest --team 173843 --since this-week

# Space-wide digest for this month
logvalet digest --since this-month

# Custom date range
logvalet digest --project PROJ --user me --since 2026-03-01 --until 2026-03-31
```

### Flags

| Flag | Values | Description |
|------|--------|-------------|
| `--issue` | Issue key (e.g., `PROJ-123`) | Single issue digest. Can be specified multiple times. |
| `--project` | Project key (e.g., `HEP_ISSUES`) | Filter by project. Can be specified multiple times. |
| `--user` | `me`, user ID, or user name | Filter by user activity. Can be specified multiple times. |
| `--team` | Team ID | Filter by team members. Can be specified multiple times. |
| `--since` | `today`, `this-week`, `this-month`, or `YYYY-MM-DD` | Period start (required). Issues are filtered by `updatedSince`. |
| `--until` | `today`, `this-week`, `this-month`, or `YYYY-MM-DD` | Period end (optional). Issues are filtered by `updatedUntil`. |
| `--start-date` | `today`, `this-week`, `this-month`, or `YYYY-MM-DD` | Filter by issue start date (schedule). Independent of `--since`/`--until`. |
| `--due-date` | `today`, `this-week`, `this-month`, or `YYYY-MM-DD` | Filter by issue due date (schedule). Independent of `--since`/`--until`. |

### Notes

- When no filters are specified, digest returns a space-wide summary for the time window.
- Multiple `--project`, `--user`, `--team`, or `--issue` flags combine with AND logic.
- `--since`/`--until` filter issues by update date (`updatedSince`/`updatedUntil`), not creation date.
- `--start-date`/`--due-date` filter issues by schedule dates and are independent of the update-date window.
- The digest output includes summary statistics, key issues, and activity patterns.

### Digest with schedule date filters

```bash
# Issues with start date this month
logvalet digest --project PROJ --since this-month --start-date this-month

# Issues due this week
logvalet digest --project PROJ --since this-month --due-date this-week

# Schedule-only filter (no update-date window required)
logvalet digest --project PROJ --start-date 2026-03-01 --due-date 2026-03-31
```

## Output

Default output is JSON. Use `--format` to change the format:

| Format | Description |
|--------|-------------|
| `json` | Machine-readable JSON (default) |
| `yaml` | YAML output |
| `md` | Rich Markdown — arrays render as tables, single objects render as key/value lists |
| `gantt` | Issue-specific Gantt table with date columns, elapsed/remaining display, and Backlog URLs |

```bash
# Markdown table output (general purpose)
lv issue list --due-date this-month --format md

# YAML output
lv issue get PROJ-123 --format yaml
```

### Gantt format

Use `--format gantt` with `issue list` to generate a date-annotated Gantt table. Each row shows the issue key, summary, start/due dates, elapsed and remaining days, and a direct Backlog URL. Issues without both start date and due date are skipped (a warning is printed to stderr).

```bash
# Gantt table of issues due this month
logvalet issue list --due-date this-month --format gantt

# Gantt table filtered by project
logvalet issue list -k PROJ --start-date this-month --format gantt
```

## Attachments

Manage issue attachments:

```bash
# List attachments for an issue
logvalet issue attachment list PROJ-123

# Get attachment info
logvalet issue attachment get PROJ-123 12345

# Download an attachment
logvalet issue attachment download PROJ-123 12345 --output ./file.pdf

# Delete an attachment (with dry-run for safety)
logvalet issue attachment delete PROJ-123 12345 --dry-run
logvalet issue attachment delete PROJ-123 12345
```

## Shared Files

Manage shared files in a project:

```bash
# List shared files in a project
logvalet shared-file list --project PROJ

# List files in a specific directory
logvalet shared-file list --project PROJ --path "/docs/technical"

# Get shared file info
logvalet shared-file get --project PROJ abc123def

# Download a shared file
logvalet shared-file download --project PROJ abc123def --output ./file.pdf
```

## Stars

Add stars to issues, comments, wikis, and pull requests:

```bash
# Star an issue
logvalet star add --issue-id 12345

# Star a comment
logvalet star add --comment-id 67890

# Star a wiki page
logvalet star add --wiki-id wiki123

# Star a pull request
logvalet star add --pr-id pr456

# Star a pull request comment
logvalet star add --pr-comment-id prcomment789
```

## MCP Server

logvalet can run as a Model Context Protocol (MCP) server, exposing all its operations as tools for Claude Desktop and Claude Code:

```bash
# Start MCP server (default: 127.0.0.1:8080)
logvalet mcp

# Specify custom host and port
logvalet mcp --host 0.0.0.0 --port 9000
```

The MCP server provides 29+ tools including:
- `logvalet_issue_get`, `logvalet_issue_list`, `logvalet_issue_create`
- `logvalet_project_get`, `logvalet_project_list`
- `logvalet_digest`
- `logvalet_shared_file_list`, `logvalet_shared_file_download`
- `logvalet_star_add`
- `logvalet_issue_context` — Issue context analysis
- `logvalet_issue_stale` — Stale issue detection
- `logvalet_project_blockers` — Project blocker detection
- `logvalet_user_workload` — User workload analysis
- `logvalet_project_health` — Integrated project health view
- And many more...

Configure the MCP server in your Claude Desktop config or Claude Code settings to use logvalet as a tool.

## Safety

Write operations support `--dry-run` to preview the request payload before executing:

```bash
lv issue create --project PROJ --summary "Fix bug" --dry-run
lv issue create --project PROJ --summary "Fix bug" --issue-type "バグ" --priority "高" --dry-run
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
lv issue attachment delete PROJ-123 12345 --dry-run
```

## Skills

logvalet includes agent skills that teach AI coding agents how to use logvalet commands effectively.

### Install (all supported agents)

```bash
npx skills add youyo/logvalet
```

### Install (Claude Code only)

```bash
npx skills add youyo/logvalet -a claude-code
```

### Available Skills

| Skill | Description |
|-------|-------------|
| `logvalet` | Core operations (issue, project, digest, user, team, and AI analysis commands) |
| `logvalet-report` | Report generation and analysis (with project health integration) |
| `logvalet-my-week` | Weekly summary and task management (with stale/overdue signals) |
| `logvalet-my-next` | Next-up task and priority management (with workload context) |
| `logvalet-issue-create` | Issue creation workflow with templates |
| `logvalet-health` | Project health check: stale issues, blockers, and user workload |
| `logvalet-context` | Full issue context: details, comments, and analysis signals |

After installation, your coding agent will automatically know how to use logvalet commands for Backlog operations.

## License

MIT
