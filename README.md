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
go install github.com/youyo/logvalet/cmd/lv@latest
```

The installed binary is named `logvalet`. You can alias it to `lv` in your shell:

```bash
alias lv=logvalet
```

## Quick Start

### Authenticate

```bash
logvalet auth login --profile work
```

### Get an issue digest

```bash
logvalet issue digest PROJ-123
```

### Short alias

```bash
lv issue digest PROJ-123
```

## Configuration

Configuration file:

```text
~/.config/logvalet/config.toml
```

Token store:

```text
~/.config/logvalet/tokens.json
```

## Shell Completion

### zsh

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

### bash

```bash
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion bash --short)"
fi
```

### fish

```fish
if type -q logvalet
    logvalet completion fish --short | source
end
```

## Commands

| Command | Description |
|---------|-------------|
| `auth login` | Authenticate via OAuth |
| `auth logout` | Remove stored credentials |
| `auth whoami` | Show current identity |
| `auth list` | List configured profiles |
| `completion bash/zsh/fish` | Generate shell completion scripts |
| `issue get <KEY>` | Get a single issue |
| `issue list` | List issues with filters |
| `issue digest <KEY>` | Get issue with context digest |
| `issue create` | Create a new issue |
| `issue update <KEY>` | Update an existing issue |
| `issue comment list <KEY>` | List issue comments |
| `issue comment add <KEY>` | Add a comment to an issue |
| `issue comment update <KEY> <ID>` | Update a comment |
| `project get <KEY>` | Get a single project |
| `project list` | List all projects |
| `project digest <KEY>` | Get project with context digest |
| `activity list` | List activity events |
| `activity digest` | Get activity digest for a time window |
| `user list` | List space users |
| `user get <ID>` | Get a single user |
| `user activity <ID>` | Get user activity |
| `user digest <ID>` | Get user activity digest |
| `document get <ID>` | Get a single document |
| `document list` | List documents in a project |
| `document tree` | Get document tree |
| `document digest <ID>` | Get document with context digest |
| `document create` | Create a new document |
| `meta status <KEY>` | List project statuses |
| `meta category <KEY>` | List project categories |
| `meta version <KEY>` | List project versions |
| `meta custom-field <KEY>` | List project custom fields |
| `team list` | List all teams |
| `team project <KEY>` | List teams for a project |
| `team digest <ID>` | Get team with context digest |
| `space info` | Show space information |
| `space disk-usage` | Show disk usage |
| `space digest` | Get space overview digest |

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
lv issue create --project PROJ --summary "Fix bug" --issue-type "Bug" --dry-run
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
```

## License

MIT
