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
logvalet configure --init-profile work --init-space myspace --init-api-key YOUR_API_KEY
```

Or interactively:

```bash
logvalet configure
```

### 2. Verify

```bash
logvalet user me
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

Run `logvalet configure` to create the configuration interactively, or use flags for non-interactive setup:

```bash
logvalet configure --init-profile work --init-space myspace --init-api-key YOUR_API_KEY
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
| `configure` | Interactive configuration setup |
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
| `watching list <USER-ID>` | List watchings for a user (`me` supported) |
| `watching count <USER-ID>` | Count watchings for a user |
| `watching get <WATCHING-ID>` | Get watching detail |
| `watching add <ISSUE-ID-OR-KEY>` | Add watching for an issue |
| `watching update <WATCHING-ID>` | Update watching note |
| `watching delete <WATCHING-ID>` | Delete watching |
| `watching mark-as-read <WATCHING-ID>` | Mark watching as read |
| `mcp` | Start MCP server (Streamable HTTP) |
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

## AI Workflow Commands (Phase 2)

Phase 2 added workflow-oriented commands that provide structured materials for LLM-assisted decision-making:

| Command | Description |
|---------|-------------|
| `issue triage-materials <KEY>` | Collect structured triage materials (attributes, history, similar-issue stats) for an issue |
| `digest weekly -k <PROJECT>` | Aggregate weekly activity: completed, started, and blocked issues |
| `digest daily -k <PROJECT>` | Aggregate daily activity snapshot |

### Design principle

logvalet provides **deterministic materials**. LLM judgment (priority suggestions, comment drafts, etc.) is handled by Skills.

### Examples

```bash
# Get triage materials for an issue
logvalet issue triage-materials PROJ-123

# Weekly activity digest for a project
logvalet digest weekly -k PROJ

# Daily activity snapshot
logvalet digest daily -k PROJ
```

## AI Intelligence Commands (Phase 3)

Phase 3 added intelligence-oriented commands that provide structured materials for LLM-assisted decision-making, anomaly detection, and risk assessment:

| Command | Description |
|---------|-------------|
| `issue timeline <KEY>` | Fetch chronological comment and update history for an issue (decision log materials) |
| `activity stats` | Aggregate activity statistics by type, actor, hour, and day-of-week patterns |

### Design principle

logvalet provides **deterministic materials**. LLM judgment (decision extraction, anomaly interpretation, risk assessment) is handled by Skills.

### Examples

```bash
# Get issue timeline for decision log extraction
logvalet issue timeline PROJ-123

# Get timeline for a specific period
logvalet issue timeline PROJ-123 --since 2026-01-01 --until 2026-03-31

# Get activity stats for a project
logvalet activity stats --scope project -k PROJ

# Get activity stats with extended time range
logvalet activity stats --scope project -k PROJ --since 2026-01-01T00:00:00Z --until 2026-03-31T23:59:59Z --top-n 10
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

# Star a pull request (the --pr-id alias is still accepted for backward compatibility)
logvalet star add --pull-request-id pr456

# Star a pull request comment
logvalet star add --pr-comment-id prcomment789
```

## Watchings

Manage issue watchings — track issues you care about even when you're not the assignee:

```bash
# List your watchings ("me" resolves to authenticated user)
logvalet watching list me

# Count your watchings
logvalet watching count me

# Get watching detail
logvalet watching get 2997876

# Add a watching for an issue
logvalet watching add PROJ-123 --note "Tracking dependency"

# Update watching note
logvalet watching update 2997876 --note "Updated note"

# Delete a watching
logvalet watching delete 2997876

# Mark as read
logvalet watching mark-as-read 2997876
```

## MCP Server

logvalet can run as a Model Context Protocol (MCP) server, exposing all its operations as tools for Claude Desktop and Claude Code:

```bash
# Start MCP server (default: 127.0.0.1:8080)
logvalet mcp

# Specify custom host and port
logvalet mcp --host 0.0.0.0 --port 9000
```

The MCP server exposes **56 tools** covering essentially every operation available in the CLI. For every CLI subcommand there is an equivalent MCP tool that accepts the same options (parameter names are converted to `snake_case` and typed as JSON Schema).

Representative tools by area:

- **Issue**: `logvalet_issue_{get,list,create,update,context,stale,timeline,triage_materials}`, `logvalet_issue_comment_{list,add,update}`, `logvalet_issue_attachment_{list,get,download,delete}`
- **Project**: `logvalet_project_{get,list,blockers,health}`, `logvalet_user_workload`
- **Digest**: `logvalet_digest`, `logvalet_digest_unified`, `logvalet_digest_{weekly,daily}`, `logvalet_space_digest`, `logvalet_activity_digest`, `logvalet_document_digest`
- **Document**: `logvalet_document_{get,list,tree,create}`
- **Meta**: `logvalet_meta_{statuses,categories,issue_types,version,custom_field}`
- **User / Team**: `logvalet_user_{me,list,get,activity}`, `logvalet_team_{list,get,project}`
- **Space / Shared File**: `logvalet_space_{info,disk_usage}`, `logvalet_shared_file_{list,get,download}`
- **Star / Watching**: `logvalet_star_add`, `logvalet_watching_{list,count,get,add,update,delete,mark_as_read}`
- **Activity**: `logvalet_activity_{list,stats}`
- **Composite**: `logvalet_my_tasks`

Configure the MCP server in your Claude Desktop config or Claude Code settings to use logvalet as a tool.

### Binary Download Size Limit

The binary download tools `logvalet_issue_attachment_download` and `logvalet_shared_file_download` return the file contents as a base64-encoded string inside the JSON response. To keep MCP responses manageable and to avoid client-side truncation:

- **Maximum size: 20 MB**. The limit is enforced at the Backlog HTTP client layer — if the response `Content-Length` exceeds 20 MB the request fails fast with an explicit error and no bytes are buffered.
- For files larger than 20 MB, use the CLI instead: `logvalet issue attachment download <KEY> <ID> --output <path>` or `logvalet shared-file download --project <PROJECT> <FILE-ID> --output <path>`.

### MCP ツールの annotation 分類

logvalet MCP サーバーは全 56 ツールに [MCP ToolAnnotations](https://spec.modelcontextprotocol.io/specification/2025-03-26/server/tools/#tool-annotations) を付与しています。
Claude Desktop / Claude Code はこのヒントを参照してツールの自動実行可否や確認ダイアログの表示を決定します。

| カテゴリ | 件数 | 対象ツール例 | 挙動 |
|---|---|---|---|
| Read-only | 45 | `*_list`, `*_get`, `*_stats`, `*_health`, `*_digest`, `*_download` 等 | 確認ダイアログなしで自動実行 |
| Write 非冪等 | 3 | `issue_create`, `issue_comment_add`, `document_create` | 通常の書き込み確認 |
| Write 冪等 | 6 | `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` | 通常の書き込み確認 |
| Destructive | 2 | `watching_delete`, `issue_attachment_delete` | 強い確認ダイアログを表示 |

> **注意**: annotations はクライアントへの**ヒント**であり、サーバー側のアクセス制御ではありません。
> annotation を変更した場合、Claude Desktop/Code のコネクタを一度切断して再接続することで新しい設定が反映されます。
> セキュリティはバックエンドの API キーまたは OAuth スコープで担保されます。

### Breaking Changes in v0.16.0

v0.16.0 unifies MCP tool parameter naming and typing with the CLI. MCP clients that used the old names must update their invocations.

| ID | Change | Affected tools | Before | After |
|----|--------|----------------|--------|-------|
| C1 | Pagination parameter unified to `count` | `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`, `logvalet_shared_file_list` | `limit: 50` | `count: 50` |
| C2 | `user_id` now string-only (`"me"` or numeric string) | `logvalet_watching_list` | `user_id: 12345` (number) | `user_id: "12345"` / `user_id: "me"` |
| C3 | `project_id` → `project_key` | `logvalet_document_list` | `project_id: 9999` (number) | `project_key: "PROJ"` (string) |
| C4 | CLI flag rename (backward-compatible alias) | `logvalet star add` | `--pr-id <id>` | `--pull-request-id <id>` (old `--pr-id` kept as alias) |

> **Migration note**: MCP clients send parameter names as JSON keys. Because the MCP framework silently ignores unknown parameters, sending the old names will not raise an explicit error — the parameter will simply be dropped. Update integration code before upgrading to v0.16.0.

### Supported Modes

logvalet supports four operating modes combining CLI / MCP, the Backlog authentication method (API key vs OAuth), and MCP client authentication (OIDC via `idproxy`). Two combinations are **not** available — see notes below.

| # | Client | Backlog auth | Client auth (OIDC) | Status |
|---|--------|--------------|--------------------|--------|
| 1 | CLI | API key | — | ✅ supported |
| 2 | CLI | OAuth | — | ❌ not implemented (CLI OAuth login command is not wired; only manual `tokens.json` editing works today) |
| 3 | MCP | API key | none | ✅ supported |
| 4 | MCP | API key | OIDC | ✅ supported |
| 5 | MCP | OAuth | none | ❌ not supported by design — OAuth is per-user and userID can only be resolved from the OIDC subject. Running `logvalet mcp` with `--backlog-client-id` but without `--auth` fails fast |
| 6 | MCP | OAuth | OIDC | ✅ supported |

Examples below show each supported mode twice: (A) environment variables only, (B) CLI flags only (with the minimum env vars when flags are not available).

#### Mode 1: CLI + API key

(A) Environment variables:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

logvalet issue get EXAMPLE-1
```

(B) CLI flags:

```bash
logvalet --api-key=your-api-key-here --space=example-space issue get EXAMPLE-1
```

#### Mode 3: MCP + API key (no client authentication)

(A) Environment variables:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

logvalet mcp
```

(B) CLI flags:

```bash
logvalet mcp --api-key=your-api-key-here --space=example-space
```

#### Mode 4: MCP + API key + OIDC (idproxy)

(A) Environment variables:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

export LOGVALET_MCP_AUTH=true
export LOGVALET_MCP_EXTERNAL_URL=https://mcp.example.com
export LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0
export LOGVALET_MCP_OIDC_CLIENT_ID=your-oidc-client-id-here
export LOGVALET_MCP_OIDC_CLIENT_SECRET=your-oidc-client-secret-here
export LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32)
export LOGVALET_MCP_ALLOWED_DOMAINS=example.com
export LOGVALET_MCP_REFRESH_TOKEN_TTL=720h  # MCP OAuth refresh token TTL (default: 30d)

logvalet mcp
```

(B) CLI flags:

```bash
logvalet mcp \
  --api-key=your-api-key-here \
  --space=example-space \
  --auth \
  --external-url=https://mcp.example.com \
  --oidc-issuer=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0 \
  --oidc-client-id=your-oidc-client-id-here \
  --oidc-client-secret=your-oidc-client-secret-here \
  --cookie-secret=$(openssl rand -hex 32) \
  --allowed-domains=example.com
```

#### Mode 6: MCP + Backlog OAuth + OIDC

Backlog OAuth settings can be configured via CLI flags or the corresponding `LOGVALET_MCP_*` environment variables.

(A) Environment variables:

```bash
# Backlog space
export LOGVALET_SPACE=example-space

# OIDC (idproxy)
export LOGVALET_MCP_AUTH=true
export LOGVALET_MCP_EXTERNAL_URL=https://mcp.example.com
export LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0
export LOGVALET_MCP_OIDC_CLIENT_ID=your-oidc-client-id-here
export LOGVALET_MCP_OIDC_CLIENT_SECRET=your-oidc-client-secret-here
export LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32)
export LOGVALET_MCP_ALLOWED_DOMAINS=example.com

# Backlog OAuth
export LOGVALET_MCP_BACKLOG_CLIENT_ID=your-backlog-oauth-client-id-here
export LOGVALET_MCP_BACKLOG_CLIENT_SECRET=your-backlog-oauth-client-secret-here
export LOGVALET_MCP_BACKLOG_REDIRECT_URL=https://mcp.example.com/oauth/backlog/callback
export LOGVALET_MCP_OAUTH_STATE_SECRET=$(openssl rand -hex 32)

# Token store (DynamoDB recommended for Lambda)
export LOGVALET_MCP_TOKEN_STORE=dynamodb
export LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
export LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1

# OAuth refresh token TTL
export LOGVALET_MCP_REFRESH_TOKEN_TTL=720h  # MCP OAuth refresh token TTL (default: 30d)

logvalet mcp
```

(B) CLI flags (all settings via flags):

```bash
logvalet mcp \
  --space=example-space \
  --auth \
  --external-url=https://mcp.example.com \
  --oidc-issuer=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0 \
  --oidc-client-id=your-oidc-client-id-here \
  --oidc-client-secret=your-oidc-client-secret-here \
  --cookie-secret=$(openssl rand -hex 32) \
  --allowed-domains=example.com \
  --backlog-client-id=your-backlog-oauth-client-id-here \
  --backlog-client-secret=your-backlog-oauth-client-secret-here \
  --backlog-redirect-url=https://mcp.example.com/oauth/backlog/callback \
  --oauth-state-secret=$(openssl rand -hex 32) \
  --token-store=dynamodb \
  --token-store-dynamodb-table=logvalet-oauth-tokens \
  --token-store-dynamodb-region=ap-northeast-1
```

See the **Backlog OAuth (Per-User)** subsection below for token store details and the first-time connection flow.

### Authentication (Optional)

Enable OIDC/OAuth 2.1 authentication for remote deployments:

```bash
logvalet mcp --auth \
  --external-url https://logvalet.example.com \
  --oidc-issuer https://accounts.google.com \
  --oidc-client-id YOUR_CLIENT_ID \
  --cookie-secret $(openssl rand -hex 32)
```

All auth flags can also be set via environment variables (e.g. `LOGVALET_MCP_AUTH=true`). See [AgentCore Deployment Guide](docs/agentcore-deployment.md) for details.

When auth is enabled:
- `/mcp` requires a Bearer token (OAuth 2.1 with PKCE)
- `/healthz` is always accessible without authentication
- OAuth endpoints (`/register`, `/authorize`, `/token`, `/.well-known/*`) are handled automatically

### Backlog OAuth (Per-User)

For remote MCP deployments, logvalet can additionally use **Backlog OAuth 2.0** so that every Backlog API call runs with the **calling user's Backlog permissions**. This is layered on top of the optional OIDC authentication described above.

- **Authentication (AuthN)**: `idproxy` verifies the user via OIDC (Entra ID, Google, etc.)
- **Authorization (AuthZ)**: a per-user Backlog OAuth access token is used for Backlog API calls

Both layers are independent: the OIDC token is never used for Backlog API calls, and the Backlog token is never used for MCP authentication.

Backlog OAuth mode activates only when **both** of the following are set:
1. `--auth` (or `LOGVALET_MCP_AUTH=true`) is enabled
2. `--backlog-client-id` (or `LOGVALET_MCP_BACKLOG_CLIENT_ID`) is set

#### Token Store

Backlog tokens are persisted via a pluggable token store. Pick one based on your deployment:

| Store | Recommended for | Notes |
|-------|----------------|-------|
| `memory` | local dev / single-instance Lambda | default; tokens are lost on restart |
| `sqlite` | self-hosted server / local CLI | pure-Go (`modernc.org/sqlite`), no CGO required |
| `dynamodb` | Lambda / multi-instance remote MCP | no VPC required, AWS-managed durability |

#### First-Time Connection Flow (Claude Desktop / Claude Code)

When connecting via Claude Desktop or Claude Code, logvalet chains the OIDC login and Backlog OAuth consent into a single seamless browser flow:

1. Claude Desktop / Claude Code opens `/authorize?...` (MCP Authorization spec)
2. No session: idproxy redirects to OIDC login (Entra ID, etc.)
3. OIDC login completes; browser returns to `/authorize?...` with session cookie
4. **BacklogAuthorizeGate** detects: session OK but Backlog not connected
   - 302 to `/oauth/backlog/authorize?continue=%2Fauthorize%3F...`
5. logvalet redirects to Backlog consent screen
6. User approves on Backlog; Backlog redirects to `/oauth/backlog/callback`
7. logvalet exchanges code, saves token, reads `state.continue`
   - 302 to `/authorize?...` (back to original MCP authorize URL)
8. BacklogAuthorizeGate: session OK + Backlog connected → pass-through
9. idproxy issues authorization code; Claude Desktop / Claude Code receives it via localhost redirect
10. Claude Desktop / Claude Code exchanges code for JWT; all subsequent `POST /mcp` calls succeed

From the user's perspective: **OIDC login → Backlog consent → Claude Desktop connected** — no manual URL copying required.

#### First-Time Connection Flow (browser / manual)

1. User invokes a Backlog tool in Claude — logvalet returns `provider_not_connected` with a link to the connection URL.
2. User opens `GET /oauth/backlog/authorize` in the browser — logvalet redirects to the Backlog consent screen.
3. User approves on Backlog — Backlog redirects back to `/oauth/backlog/callback`.
4. logvalet exchanges the code, stores the token keyed by the user's OIDC subject, and returns `{"status":"connected"}`.
5. Subsequent tool calls automatically use the stored token for that user.
6. The user can check state with `GET /oauth/backlog/status` or revoke with `DELETE /oauth/backlog/disconnect`.

#### `continue` Parameter and Security

`GET /oauth/backlog/authorize` accepts an optional `?continue=<path>` query parameter.
When present, `/oauth/backlog/callback` redirects to that path after storing the token instead of returning the JSON success response.

Security constraints (enforced in both authorize and callback — double defence):

- `continue` must be a **relative path starting with `/authorize`** (e.g. `/authorize?client_id=...`)
- Absolute URLs (`https://...`), protocol-relative URLs (`//...`), and backslashes are rejected with `400 invalid_request`
- Any other path prefix (e.g. `/`, `/mcp`) is also rejected
- If a tampered `continue` value is found in the callback state, logvalet falls back to the JSON success response instead of redirecting

#### Flags and Environment Variables

All Backlog OAuth settings can be configured via CLI flags or environment variables (no config file required):

| Flag | Environment Variable | Required | Default | Description |
|------|---------------------|----------|---------|-------------|
| `--backlog-client-id` | `LOGVALET_MCP_BACKLOG_CLIENT_ID` | yes | — | Backlog OAuth client ID |
| `--backlog-client-secret` | `LOGVALET_MCP_BACKLOG_CLIENT_SECRET` | yes | — | Backlog OAuth client secret |
| `--backlog-redirect-url` | `LOGVALET_MCP_BACKLOG_REDIRECT_URL` | yes | — | OAuth callback URL (`https://<your-host>/oauth/backlog/callback`) |
| `--oauth-state-secret` | `LOGVALET_MCP_OAUTH_STATE_SECRET` | yes | — | HMAC-SHA256 signing key for state JWT (hex, 64+ chars) |
| `--token-store` | `LOGVALET_MCP_TOKEN_STORE` | no | `memory` | `memory` / `sqlite` / `dynamodb` |
| `--token-store-sqlite-path` | `LOGVALET_MCP_TOKEN_STORE_SQLITE_PATH` | sqlite only | `./logvalet.db` | SQLite DB file path |
| `--token-store-dynamodb-table` | `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE` | dynamodb only | — | DynamoDB table name |
| `--token-store-dynamodb-region` | `LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION` | dynamodb only | — | AWS region for the DynamoDB table |

#### Example

```bash
# Base idproxy (OIDC) config — see "Authentication (Optional)" above
export LOGVALET_MCP_AUTH=true
export LOGVALET_MCP_EXTERNAL_URL=https://mcp.example.com
export LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0
export LOGVALET_MCP_OIDC_CLIENT_ID=your-oidc-client-id-here
export LOGVALET_MCP_OIDC_CLIENT_SECRET=your-oidc-client-secret-here
export LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32)
export LOGVALET_MCP_ALLOWED_DOMAINS=example.com

# Token store (DynamoDB recommended for Lambda)
export LOGVALET_MCP_TOKEN_STORE=dynamodb
export LOGVALET_MCP_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
export LOGVALET_MCP_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1

# Backlog OAuth client (created in your Backlog space)
export LOGVALET_MCP_BACKLOG_CLIENT_ID=your-backlog-oauth-client-id-here
export LOGVALET_MCP_BACKLOG_CLIENT_SECRET=your-backlog-oauth-client-secret-here
export LOGVALET_MCP_BACKLOG_REDIRECT_URL=https://mcp.example.com/oauth/backlog/callback
export LOGVALET_MCP_OAUTH_STATE_SECRET=$(openssl rand -hex 32)

logvalet mcp --auth
```

On startup you will see something like:

```
logvalet MCP server (auth + OAuth) listening on 127.0.0.1:8080/mcp
  OAuth routes: /oauth/backlog/{authorize,callback,status,disconnect}
```

### Docker / AgentCore Deployment

```bash
# Build
docker build -t logvalet .

# Run (no auth)
docker run -p 8080:8080 \
  -e LOGVALET_API_KEY=your-api-key \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet

# Run (with auth)
docker run -p 8080:8080 \
  -e LOGVALET_MCP_AUTH=true \
  -e LOGVALET_MCP_EXTERNAL_URL=https://logvalet.example.com \
  -e LOGVALET_MCP_OIDC_ISSUER=https://accounts.google.com \
  -e LOGVALET_MCP_OIDC_CLIENT_ID=your-client-id \
  -e LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32) \
  -e LOGVALET_API_KEY=your-api-key \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet
```

For AWS Bedrock AgentCore Runtime deployment, see [docs/agentcore-deployment.md](docs/agentcore-deployment.md).

### Lambda Function URL (lambroll)

Deploy logvalet as a Lambda Function URL using [lambroll](https://github.com/fujiwara/lambroll) and [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter).
See [examples/lambroll/](examples/lambroll/) for setup instructions.

### Backlog OAuth 自動誘導

認証 (`--auth`) と Backlog OAuth (`--backlog-client-id` 等) を有効にしてデプロイすると、
ブラウザで `$LOGVALET_MCP_EXTERNAL_URL` を開いた際に以下のフローが自動実行されます:

1. EntraID 等の OIDC プロバイダでログイン
2. Backlog トークン未保存の場合 `/oauth/backlog/authorize` へ自動リダイレクト
3. Backlog 同意画面 → コールバック → 完了画面

MCP クライアントが未接続状態でツールを呼ぶと、レスポンスの `_meta.authorization_url` に
Backlog 認可 URL が含まれるため、クライアント側でユーザーに提示できます。

### Task Runner (mise)

```bash
mise run build              # Build binary
mise run test               # Run all tests
mise run test:integration   # Run integration tests
mise run vet                # Run go vet
mise run lint               # Run vet + test
mise run mcp:start          # Start MCP server (local)
mise run mcp:start-auth     # Start MCP server with auth
mise run docker:build       # Build Docker image
```

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
| `logvalet:logvalet` | PM meta-model hub: all skills overview, workflows, and getting started guide |
| `logvalet:report` | Report generation and analysis (with project health integration) |
| `logvalet:my-week` | Weekly summary and task management (with stale/overdue signals) |
| `logvalet:my-next` | Next-up task and priority management (with workload context) |
| `logvalet:issue-create` | Issue creation workflow with templates |
| `logvalet:health` | Project health check: stale issues, blockers, and user workload |
| `logvalet:context` | Full issue context: details, comments, and analysis signals |
| `logvalet:triage` | Issue triage workflow: LLM-assisted priority/assignee suggestions using triage-materials |
| `logvalet:draft` | Draft issue comments using issue context and conversation history |
| `logvalet:digest-periodic` | Generate weekly/daily digest summaries with LLM highlights |
| `logvalet:spec-to-issues` | Decompose a spec document into Backlog issues (SKILL-only, no CLI needed) |
| `logvalet:decisions` | Extract and summarize decision logs from issue timeline history |
| `logvalet:intelligence` | Analyze activity statistics to detect anomalies, biases, and risks |
| `logvalet:risk` | Generate integrated risk assessment and recommended actions for a project |

After installation, your coding agent will automatically know how to use logvalet commands for Backlog operations.

## License

MIT
