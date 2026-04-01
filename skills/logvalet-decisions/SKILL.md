---
name: logvalet-decisions
description: >
  Extract and summarize decision logs from a Backlog issue or project timeline.
  TRIGGER when: user says "意思決定", "decision log", "決定履歴", "なぜこうなったか",
  "経緯", "決定の背景", "decision history", "どうして変更された", "意思決定ログ",
  "何が決まった", "承認経緯", "設計の経緯", "why was this decided".
---

# logvalet-decisions

`lv issue timeline` を材料に、LLM がコメント・更新履歴から意思決定を抽出・要約する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-decisions` when you need to:

- understand why a certain approach was chosen
- trace the history of key decisions in an issue or project
- identify who was involved in key decisions and when
- summarize decision points for stakeholders or retrospectives
- explain the rationale behind current issue state

---

## Workflow

### Step 1: Identify issue key or project

If the user provides a Backlog issue key (e.g., `PROJ-123`) or project key, extract and use it directly.

If no key is provided, ask for it.

### Step 2: Fetch timeline data

For a single issue:

```bash
lv issue timeline ISSUE_KEY -f json
```

For a richer view, optionally include more activity:

```bash
lv issue timeline ISSUE_KEY --max-comments 0 --max-activity-pages 10 -f json
```

To narrow to a specific period:

```bash
lv issue timeline ISSUE_KEY --since YYYY-MM-DD --until YYYY-MM-DD -f json
```

The timeline returns a chronological sequence of:

- comments (author, timestamp, content)
- update events (field changed, old value → new value, actor, timestamp)

### Step 3: Identify decision signals

Scan the timeline for content that indicates decisions were made:

**Comment-based signals:**
- Explicit decision phrases: "決定しました", "〜にします", "承認", "合意", "decided to", "agreed to", "resolved to"
- Rationale phrases: "なぜなら", "理由は", "because", "the reason is"
- Alternatives discussion: "〜か〜か", "option A vs B", "検討した結果"
- Status transitions with explanation

**Update-based signals:**
- Priority changes (especially escalation)
- Assignee changes with significant workload implication
- Category or milestone changes
- Due date extensions with context

### Step 4: Extract and structure decisions

For each identified decision, extract:

- **When:** timestamp of the decision event
- **Who:** actor(s) involved (comment author or update actor)
- **What:** what was decided (concise summary)
- **Why:** rationale if mentioned
- **Context:** linked alternative options or prior discussion

### Step 5: Present decision log

Present the decision log in chronological order:

```
## 意思決定ログ — ISSUE_KEY

> タイムライン期間: FROM〜TO / 意思決定数: N件

---

### 1. [YYYY-MM-DD] <決定内容の1行サマリー>

- **判断者:** UserName
- **決定:** <具体的な決定内容>
- **理由:** <根拠・背景（記載がある場合）>
- **証拠:** <元のコメント or 更新イベント（要約）>

---

### 2. [YYYY-MM-DD] <決定内容の1行サマリー>

...

---

**サマリー:**
- 主要な意思決定: N件
- 主な判断者: UserA, UserB
- 重要な転換点: <あれば記載>
```

---

## Notes

- `issue timeline` が材料の唯一の権威ソース — 追加の `issue get` / `comment list` 呼び出しは不要
- 明確な根拠が記載されていない決定は「根拠: 記録なし」と明示する
- コメントが多い場合は最初に `--since` で期間を絞ることを検討する
- `--no-include-updates` フラグでコメントのみに絞ることもできる

---

## Anti-patterns

- Do not invent rationale that is not in the timeline — if rationale is missing, say so
- Do not skip the issue identification step — always confirm the key
- Do not confuse routine status updates with actual decisions — focus on changes that involved judgment or discussion
- Do not summarize all comments; focus only on those containing decision signals
