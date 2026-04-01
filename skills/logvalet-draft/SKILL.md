---
name: logvalet-draft
description: >
  Draft a Backlog issue comment based on issue context: progress update, inquiry, or resolution notice.
  TRIGGER when: user says "コメント下書き", "draft comment", "コメントを書いて",
  "返信を作って", "コメントを作成", "comment draft", "コメント草稿",
  "コメントを下書きして", "issue comment を書いて", "バックログにコメント",
  "進捗報告コメント", "確認依頼コメント", "解決通知コメント".
---

# logvalet-draft

`lv issue context` を材料に、LLM がコンテキストに沿ったコメント下書きを生成する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-draft` when you need to:

- write a progress update comment for an in-progress issue
- draft a confirmation request to a stakeholder
- compose a resolution notice when closing an issue
- reply to a question in the issue comment thread
- write any issue comment where context (history, tone, status) matters

---

## Workflow

### Step 1: Identify issue key

If the user provides a Backlog issue key (e.g., `PROJ-123`, a URL like `*.backlog.com/view/PROJ-123`), extract and use it directly.

If no key is provided, ask for it.

### Step 2: Clarify comment purpose

Ask the user (if not already clear) what kind of comment they want to write:

- 進捗報告 (progress update)
- 確認依頼 (confirmation request)
- 解決通知 (resolution notice)
- 質問応答 (reply to a question)
- その他 (other — let the user describe)

This helps the LLM calibrate tone and content.

### Step 3: Fetch issue context

```bash
lv issue context ISSUE_KEY -f json
```

This returns:
- issue details (summary, description, status, priority, assignee, dates)
- comment history (tone, participants, recent discussion)
- analysis signals (stale indicator, overdue flag, blocker hints)

### Step 4: Draft the comment

Using the issue context, generate a comment that:

- matches the tone of past comments in the thread (formal/casual, Japanese/English)
- reflects the current issue state (e.g., stale → acknowledge delay; overdue → address deadline)
- fits the stated purpose (progress update, inquiry, etc.)
- is concise and actionable

### Step 5: Present draft for review

```
## コメント下書き — ISSUE_KEY

> コンテキスト: 最終更新 N日前、担当者: UserName、ステータス: X

---

<下書き内容>

---
このコメントを投稿しますか？[はい/編集/キャンセル]
```

If the user wants to edit, revise and re-present.

### Step 6: Post the comment

After user confirmation, write the draft to a temp file and post:

```bash
lv issue comment add ISSUE_KEY --content-file /tmp/draft_ISSUE_KEY.md --dry-run
lv issue comment add ISSUE_KEY --content-file /tmp/draft_ISSUE_KEY.md
```

Always run `--dry-run` first before posting.

---

## Notes

- `issue context` provides comment history — use it to match the thread's existing tone and avoid repeating information already stated
- If the issue has no previous comments, default to a polite, professional Japanese tone
- The draft should reference specific issue details (e.g., deadline, assignee name) to feel contextual, not generic
- For long drafts, write to a temp file (`/tmp/draft_ISSUE_KEY.md`) rather than passing via `--content`

---

## Anti-patterns

- Do not post a comment without showing the draft and confirming with the user first
- Do not use a generic template without incorporating issue-specific context
- Do not guess the issue key — always confirm
- Do not skip `--dry-run` before posting
