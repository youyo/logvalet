# Plan: logvalet スキル自動発動の改善

## Context

ユーザーが `/devflow:plan` で Backlog URL（`https://heptagon.backlog.com/view/ESU2_S2-32#comment-707972390`）を貼っても、`logvalet` スキルが自動発動しない。
原因は SKILL.md の `description` フィールドに Backlog URL や課題キーなどのトリガーキーワードが含まれていないため、Claude Code のスキルマッチングが発火しない。

## 変更対象

- `skills/logvalet/SKILL.md` — description フィールドの強化

## 変更内容

### description を以下に変更

**現在:**
```
description: Use logvalet (lv) to read, summarize, and safely update Backlog with LLM-friendly JSON digests.
```

**変更後:**
```
description: >
  Use logvalet (lv) to read, summarize, and safely update Backlog with LLM-friendly JSON digests.
  TRIGGER when: user provides a Backlog URL (*.backlog.com/view/*, *.backlog.com/alias/*),
  mentions a Backlog issue key (e.g. PROJ-123, ESU2_S2-32),
  or asks about Backlog issues, projects, activities, documents, or users.
```

## 検証方法

1. 変更後に `/devflow:plan` + Backlog URL を含むメッセージで logvalet スキルが発動するか確認
2. Backlog 課題キー（`ESU2_S2-32` など）を含むメッセージでも発動するか確認
