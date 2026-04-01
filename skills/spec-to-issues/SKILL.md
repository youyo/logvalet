---
name: spec-to-issues
description: >
  Decompose a specification document into Backlog issues: the LLM analyzes a spec/requirements
  file, breaks it into appropriately-scoped tasks, and creates them in Backlog one by one
  with dry-run preview before each submission.
  TRIGGER when: user says "specから課題作成", "仕様から課題を作って", "spec to issues",
  "仕様書を課題に分解", "要件を課題にして", "spec 分解", "仕様分解",
  "仕様書からチケットを切って", "spec からタスクを作成", "要件定義を課題に",
  "仕様をバックログに登録", "spec を issue に変換",
  "設計書から課題を起こして", "PRD から課題を作成", "タスク分解",
  "break down spec into issues", "create issues from requirements",
  "一括課題作成", "bulk issue creation from spec".
  DO NOT TRIGGER when: user wants to create a single issue manually (use issue-create)
  or wants to write a spec first (use an external spec tool).
---

# logvalet-spec-to-issues

spec ファイルを解析し、適切な粒度の課題リストに分解してから Backlog に順次登録する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-spec-to-issues` when you need to:

- break down a feature spec or requirements document into actionable Backlog issues
- bulk-create a structured set of tasks from a design document
- convert a markdown spec file into a hierarchical epic → task structure
- onboard a new feature by registering all required tasks at once

---

## Workflow

### Step 1: Obtain spec content

Ask the user for one of:
- a file path (e.g., `./docs/spec.md`, `./requirements.md`)
- pasted spec text directly in the conversation

Read the spec file if a path is provided.

### Step 2: Confirm project and defaults

Ask the user (in a single question):

- `project-key` — target Backlog project
- default `assignee` (optional — can be left unset for all issues)
- default `priority` (optional — default: "中")
- default `issue-type` (optional — default: "タスク")
- whether to use parent/child structure (epic → task) if the spec has clear phases or sections

If project key is unknown, list available projects:

```bash
lv project list -f md
```

### Step 3: Decompose spec into issue list

Analyze the spec and generate a proposed issue list. Guidelines:

- **Granularity:** Each issue should represent 1–3 days of work
- **Title:** Clear, actionable, specific (not "Implement feature X" but "X の API エンドポイントを実装")
- **Description:** Reference the relevant section of the spec (e.g., "spec §3 の認証フロー")
- **Priority:** Infer from criticality, dependencies, and user-stated defaults
- **Dependencies:** Note which issues depend on others
- **Parent/child:** Group related sub-tasks under a parent issue when a section is large

### Step 4: Present issue list for review

Show the proposed issue list before creating anything:

```
## Spec → Issues 分解結果 — PROJECT_KEY

> spec: ./spec.md | 課題数: N件

### 課題リスト

1. **[高] API エンドポイント設計**
   - 説明: spec §3 の認証エンドポイントを実装
   - 推定工数: 2日
   - 依存: なし

2. **[中] データベーススキーマ設計**
   - 説明: spec §4 のテーブル定義に従いマイグレーション作成
   - 推定工数: 1日
   - 依存: なし

3. **[高] フロントエンド認証フロー**
   - 説明: spec §5 のログイン/ログアウト UI
   - 推定工数: 3日
   - 依存: 課題1

---
N件の課題を作成します。[全て作成/選択/キャンセル]
```

Allow the user to:
- approve all
- select specific issues to create
- edit titles, priorities, or descriptions before creation
- cancel

### Step 5: Dry-run preview

For each issue to be created, run `--dry-run` first:

```bash
lv issue create \
  --project-key PROJ \
  --summary "API エンドポイント設計" \
  --description "spec §3 の認証エンドポイントを実装" \
  --priority "高" \
  --issue-type "タスク" \
  --dry-run
```

Show the dry-run summary to the user and confirm before proceeding to actual creation.

### Step 6: Create issues sequentially

Create each confirmed issue one by one:

```bash
lv issue create \
  --project-key PROJ \
  --summary "API エンドポイント設計" \
  --description "spec §3 の認証エンドポイントを実装" \
  --priority "高" \
  --issue-type "タスク"
```

For child issues, use `--parent-issue-id` after the parent is created:

```bash
lv issue create \
  --project-key PROJ \
  --summary "子タスクのタイトル" \
  --parent-issue-id PARENT_ISSUE_ID
```

### Step 7: Report results

After all issues are created, show a summary:

```
## 作成完了 — N件の課題を登録しました

| # | 課題キー | タイトル |
|---|----------|----------|
| 1 | PROJ-101 | API エンドポイント設計 |
| 2 | PROJ-102 | データベーススキーマ設計 |
| 3 | PROJ-103 | フロントエンド認証フロー |
```

---

## Notes

- This skill is SKILL-only — there is no dedicated `logvalet` command for spec decomposition; the LLM does the analysis
- Granularity matters: too coarse (1 issue for entire feature) and it's untrackable; too fine (1 issue per function) and it's noise
- Reference spec sections in descriptions so issues stay traceable to requirements
- Creation order: create parent issues before children so `--parent-issue-id` is available
- If the spec is very long (>500 lines), consider processing it in sections and asking the user to confirm between sections

---

## Anti-patterns

- Do not create issues without showing the list and getting user confirmation first
- Do not skip `--dry-run` before bulk creation
- Do not create issues with generic titles like "実装する" — always be specific
- Do not create more than ~20 issues in a single session without checking with the user
- Do not guess the project key — always confirm
