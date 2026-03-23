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
| `project get <KEY>` | Get a single project |
| `project list` | List all projects |
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
| `config init` | Interactive configuration setup |
| `configure` | Alias for config init |
| `version` | Show version information |

## Global Flags

```text
--profile, -p <name>     Profile to use
--format, -f <format>    Output format: json (default), md, text, yaml
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

# List issues assigned to team members
logvalet issue list --assignee team --status not-closed --due-date this-week

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
```

| Flag | Values | Description |
|------|--------|-------------|
| `--assignee` | `me`, `team`, user ID, or user name | Filter by assignee. `team` filters by configured team members (requires `team_id` in config). |
| `--status` | `open`, `not-closed`, status name(s), or status ID | Filter by status. `open` excludes completed. `not-closed` excludes completed (no project key required). Names/`open` require `-k` |
| `--due-date` | `today`, `overdue`, `this-week`, `this-month`, `YYYY-MM-DD`, or `YYYY-MM-DD:YYYY-MM-DD` | Filter by due date. Date ranges support open-ended queries (`:YYYY-MM-DD` or `YYYY-MM-DD:`) |
| `--sort` | `dueDate`, `created`, `updated`, `priority`, `status`, `assignee` | Sort results by field |
| `--order` | `asc`, `desc` | Sort direction. Default: `desc` |

Note: When using `--due-date`, results are automatically paginated to retrieve all matching issues (up to 10,000 total).

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

### Notes

- When no filters are specified, digest returns a space-wide summary for the time window.
- Multiple `--project`, `--user`, `--team`, or `--issue` flags combine with AND logic.
- Issues are filtered by update date (`updatedSince`/`updatedUntil`), not creation date.
- The digest output includes summary statistics, key issues, and activity patterns.

## Configuration: Team ID

To use `--assignee team` in `issue list`, configure your team ID in `config.toml`:

```toml
[profiles.work]
space = "heptagon"
base_url = "https://heptagon.backlog.com"
auth_ref = "heptagon"
team_id = 173843
```

Once configured, you can filter issues by team:

```bash
logvalet issue list --assignee team --status not-closed --due-date this-week
```

## Output

Default output is JSON. Use `--format` to change the format:

```bash
lv issue digest PROJ-123 --format md
lv issue digest PROJ-123 --format yaml
lv issue digest PROJ-123 --format text
```

## Safety

Write operations support `--dry-run` to preview the request payload before executing:

```bash
lv issue create --project PROJ --summary "Fix bug" --dry-run
lv issue create --project PROJ --summary "Fix bug" --issue-type "バグ" --priority "高" --dry-run
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
```

## Skill

logvalet includes an agent skill that teaches AI coding agents how to use logvalet commands effectively.

### Install (all supported agents)

```bash
npx skills add youyo/logvalet
```

### Install (Claude Code only)

```bash
npx skills add youyo/logvalet -a claude-code
```

After installation, your coding agent will automatically know how to use logvalet commands for Backlog operations.

## License

MIT
