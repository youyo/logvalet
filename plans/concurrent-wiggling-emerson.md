# npx skills add コマンド修正 + スキルディレクトリ構造修正

## Context

README.md の「Claude Code Skill」セクションで `npx @anthropic-ai/claude-code skills add` を使っているが、これは誤り。
正しくは [vercel-labs/skills](https://github.com/vercel-labs/skills) の `npx skills add` コマンドを使う。
logvalet のスキルは Claude Code 限定ではないため、一般的な使い方と `-a claude-code` 指定の両方を記載する。

また、vercel-labs/skills の規約ではスキルは `skills/<skill-name>/SKILL.md` というサブディレクトリ構造が必要。
現在の `skills/SKILL.md`（直置き）を `skills/logvalet/SKILL.md` に移動する。

## 変更対象ファイル

- `skills/SKILL.md` → `skills/logvalet/SKILL.md` に移動（git mv）
- `README.md` — 「Claude Code Skill」→「Skill」に改名、npx コマンド修正

## 変更内容

### 1. スキルファイル移動

```bash
git mv skills/SKILL.md skills/logvalet/SKILL.md
```

### 2. `README.md` (L164–180)

**Before:**
```markdown
## Claude Code Skill

logvalet includes a Claude Code skill that teaches Claude how to use logvalet commands effectively.

### Install via npx

```bash
npx @anthropic-ai/claude-code skills add /path/to/logvalet/skills
```

### Install from GitHub

```bash
npx @anthropic-ai/claude-code skills add https://github.com/youyo/logvalet/tree/main/skills
```

After installation, Claude Code will automatically know how to use logvalet commands for Backlog operations.
```

**After:**
```markdown
## Skill

logvalet includes an agent skill that teaches AI coding agents how to use logvalet commands effectively.

### Install (all supported agents)

```bash
npx skills add https://github.com/youyo/logvalet
```

### Install (Claude Code only)

```bash
npx skills add https://github.com/youyo/logvalet -a claude-code
```

After installation, your coding agent will automatically know how to use logvalet commands for Backlog operations.
```

## 検証方法

- `go test ./...` でビルド破壊がないことを確認（ドキュメントのみの変更なので念のため）
- README.md の該当セクションを目視確認
- `skills/logvalet/SKILL.md` が存在することを確認
