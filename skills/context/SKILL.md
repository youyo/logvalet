---
name: logvalet:context
description: >
  Fetch full context for a Backlog issue in one shot: issue details, comments, status signals,
  overdue/stale detection, and LLM hints — everything needed to understand and act on an issue.
  TRIGGER when: user provides a Backlog issue key (e.g. PROJ-123, ESU2_S2-32),
  a Backlog URL (*.backlog.com/view/*, *.backlog.com/alias/*),
  or says "課題の詳細", "この課題について", "issue context", "課題を理解したい",
  "issue を調べて", "この issue の状況", "課題の背景", "課題の情報をまとめて",
  "PROJ-123 について教えて", "バックログの課題の詳細", "issue summary",
  "課題コンテキスト", "backlog issue context", "チケットの状態",
  "このチケットについて", "課題の全体像", "what's the status of this issue".
  DO NOT TRIGGER when: user wants to create/update an issue (use issue-create),
  wants a triage recommendation (use triage), or wants a comment draft (use draft).
---

# logvalet-context

Backlog 課題の判断材料（詳細・コメント・関連情報）を一括取得する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-context` when you need to:

- understand the full context of a Backlog issue before taking action
- gather comments, related metadata, and analysis signals for a single issue
- prepare a summary of an issue for a PR description, standup update, or report
- decide whether an issue needs escalation, reassignment, or status change

`logvalet-context` is more issue-focused than `digest --issue`. Use it when a single issue is the center of attention.

---

## Workflow

### Step 1: Identify issue key

If the user provides a Backlog issue key (e.g., `PROJ-123`, a URL like `*.backlog.com/view/PROJ-123`), extract and use it directly.

If the key is embedded in a URL:

```
https://myspace.backlog.com/view/PROJ-123  →  PROJ-123
```

If no key is provided, ask the user for it.

### Step 2: Fetch issue context

```bash
lv issue context ISSUE_KEY
```

This returns a rich context object including:
- issue details (summary, description, status, priority, assignee, dates)
- comment history
- related metadata (project, category, milestone, version)
- analysis signals (stale indicator, overdue flag, blocker hints)

### Step 3: Optionally fetch digest for broader context

If the user wants broader project context around the issue, also run:

```bash
lv digest --issue ISSUE_KEY --since 30d
```

Run in parallel with Step 2 if both are needed.

### Step 4: Format output

Present the context in a human-readable summary:

```
## PROJ-123: 課題タイトル

**ステータス:** 処理中 | **優先度:** 高 | **担当者:** UserName
**期限:** YYYY-MM-DD | **最終更新:** YYYY-MM-DD (N日前)

### 説明
<issue description>

### コメント履歴 (N件)
- [YYYY-MM-DD] UserName: "コメント内容..."
- [YYYY-MM-DD] UserName: "コメント内容..."

### シグナル
- ⚠ N日間更新なし（停滞の可能性）  ← if stale
- ⚠ 期限超過 N日              ← if overdue
- ✅ 最近更新あり               ← if recently active

### メタデータ
- プロジェクト: PROJECT_NAME (PROJECT_KEY)
- カテゴリ: CategoryName
- マイルストーン: MilestoneName

### ウォッチ情報
- ウォッチ中: はい / いいえ（自分がこの課題をウォッチしているか）
```

### Step 5: Suggest next actions

Based on the context, suggest possible actions without executing them:

- If stale: "停滞しています。担当者への確認コメントを追加しますか？"
- If overdue: "期限を過ぎています。ステータス更新または期限変更を検討してください。"
- If no assignee: "担当者が設定されていません。`logvalet issue update PROJ-123 --assignee USER` で設定できます。"
- If not watching and the issue is relevant to the user's work: "この課題をウォッチしますか？ `logvalet watching add PROJ-123` でウォッチできます。"

Always ask before writing.

---

## Notes

- `issue context` returns all signals in one call — prefer it over combining `issue get` + `issue comment list` manually
- The output is optimized for LLM consumption and agent reasoning
- If the issue key resolves to a 404, check for typos or confirm the project key is correct with `lv project list`
- Use `--format md` for human-readable output, default JSON for agent pipelines
- ウォッチ情報は `lv watching list --user-id me -f json` から取得し、対象課題キーが含まれているか確認する
- 課題のコンテキストを把握する際に「自分がウォッチしているか」は関心度の指標になる
- Watch CLI（M17）が未実装の場合、ウォッチ関連の表示はスキップする

---

## Anti-patterns

- Do not call `issue get` and `issue comment list` separately when `issue context` gives you everything
- Do not update an issue based on context analysis without user confirmation
- Do not guess the issue key from a partial description — always confirm
