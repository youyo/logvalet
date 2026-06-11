---
name: logvalet:document-search
description: >
  Search Backlog documents by keyword within a space: runs lv document search and presents
  results as a snippet-based digest — title, project, and a relevant excerpt from each document.
  TRIGGER when: user says "ドキュメント検索", "document search", "資料を探して",
  "ドキュメントを探して", "Backlog ドキュメント検索", "wiki 検索", "ドキュメントを検索",
  "document を検索", "search documents", "find documents", "資料検索",
  "ドキュメントから探して", "document を探したい", "ドキュメントで調べて",
  "search backlog docs", "find in documents", "ドキュメント一覧から探す",
  "キーワードでドキュメントを検索", "keyword document search",
  "ドキュメントの中を検索", "資料を見つけて".
  DO NOT TRIGGER when: user wants to get a specific document by ID (use lv document get),
  wants to list all documents in a project (use lv document list),
  or wants to search issues/wiki (use the respective commands).
---

# logvalet:document-search

Backlog ドキュメントをキーワードで横断検索し、スニペット付き digest を返す。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet:document-search` when you need to:

- find Backlog documents containing a specific keyword
- locate design documents, meeting notes, or knowledge base articles
- search across all projects in a space for relevant documentation
- answer "is there a document about X?" questions

---

## Workflow

### Step 1: Identify the keyword

Extract the search keyword from the user's request. If multiple keywords are mentioned, combine them or use the most specific one.

Optionally identify project key(s) if the user wants to narrow the search to specific projects.

### Step 2: Search documents

```bash
lv document search "<keyword>" [--project KEY ...] [--detail snippet|meta|full] [-f json]
```

| Option | Default | Description |
|--------|---------|-------------|
| `<keyword>` | required | Search term |
| `--project KEY` | (all projects) | Filter by project key (can repeat) |
| `--detail` | `snippet` | `snippet`: excerpt only / `meta`: title+metadata / `full`: full body |
| `--count` | 100 | Max results (1-100) |
| `--offset` | 0 | Pagination offset |
| `--sort` | (API default) | `created` \| `updated` |
| `--order` | (API default) | `asc` \| `desc` |

**Default behaviour**: searches all projects within the current space. Pass `--project` to narrow.

### Step 3: Present results

Present the digest to the user:

- Show `total_returned` and note if `possibly_more=true` (more results may exist)
- For each document: title, project, and the `snippet` excerpt
- If `possibly_more=true`, suggest using `--offset` to paginate

**Example output format:**
```
Found 5 document(s) matching "OAuth". (possibly more — use --offset to paginate)

1. [認証フロー設計書] (PROJ) — "...OAuth 2.0 の認可コードフローを採用。クライアント ID は..."
2. [OAuth 実装メモ] (INFRA) — "...BacklogのOAuthエンドポイント: /oauth2/token を使用..."
```

### Step 4: Handle edge cases

- **No results**: Suggest broadening the keyword or removing project filters
- **possibly_more=true**: Offer to fetch the next page with `--offset N`
- **detail=full requested**: Warn that full body may be large; recommend `snippet` first

---

## MCP tool (alternative to CLI)

```
logvalet_document_search
  keyword:      string   (required)
  project_keys: string[] (optional)
  sort:         string   (optional) — created | updated
  order:        string   (optional) — asc | desc
  count:        number   (optional, default 100)
  offset:       number   (optional, default 0)
  detail:       string   (optional, default "snippet") — snippet | meta | full
```

---

## Tips

- **Cross-project search**: Omit `--project` to search all projects in the current space (default)
- **Pagination**: If `possibly_more=true`, use the `next_offset` value from the response as `--offset N` to get the next page. Example: if `next_offset=50`, run `lv document search "keyword" --count 50 --offset 50`
- **Keyword scope**: Backlog searches document content (title and body). Use specific technical terms for best results
- **Multi-space**: When multiple Backlog spaces are connected, results are per-space (fan-out). Each space returns its own digest
