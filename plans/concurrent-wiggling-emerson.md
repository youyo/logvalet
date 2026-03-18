# Shell Completion セクション整理

## Context

README.md と SKILL.md の Shell Completion セクションに bash/fish の手順が含まれているが、
zsh のみで十分。README には eval の手順を記載する。

## 変更対象ファイル

- `README.md` — Shell Completion セクション (line 61-85)
- `skills/SKILL.md` — completion セクション (line 354-388)
- `docs/specs/logvalet_SKILL.md` — skills/SKILL.md と同期

## 変更一覧

### 1. README.md — Shell Completion を zsh のみに

**現状** (line 61-85): zsh / bash / fish の3セクション

**変更後**:
```markdown
## Shell Completion

Add this to `.zshrc`:

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

This enables completion for both `logvalet` and `lv`.
```

### 2. SKILL.md — completion セクションを zsh のみに

**現状** (line 354-388): zsh / bash / fish の3セクション + `--short` 説明

**変更後**:
```markdown
## completion

Completion is generated dynamically and should be loaded with `eval`.

### zsh

Put this in `.zshrc`:

```zsh
if command -v logvalet >/dev/null 2>&1; then
  eval "$(logvalet completion zsh --short)"
fi
```

`--short` enables completion for both `logvalet` and `lv`.
```

### 3. docs/specs/logvalet_SKILL.md の同期

skills/SKILL.md と同じ内容にコピー。

## 検証方法

- 3ファイルの diff を確認
- skills/SKILL.md と docs/specs/logvalet_SKILL.md が一致することを確認
