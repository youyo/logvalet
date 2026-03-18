# ドキュメント更新: README.md & SKILL.md を最新実装に同期

## Context

M17-M19 実装完了後、README.md と skills/SKILL.md に古い記述が残っている。
Quick Start 手順が `auth login` のみで `config init` を案内していない、
go install パスが間違っている、auth の認証モード説明が不正確 等。

## 変更対象ファイル

- `README.md`
- `skills/SKILL.md`
- `docs/specs/logvalet_SKILL.md`（skills/SKILL.md と同期）

## 変更一覧

### 1. README.md — Quick Start を `config init` ベースに変更

**現状** (line 27-45):
```markdown
## Quick Start
### Authenticate
logvalet auth login --profile work
### Get an issue digest
logvalet issue digest PROJ-123
```

**変更後**:
```markdown
## Quick Start

### 1. Setup

logvalet config init --init-profile work --init-space myspace --init-api-key YOUR_API_KEY

Or interactively:

logvalet config init

### 2. Verify

logvalet auth whoami

### 3. Get an issue digest

logvalet issue digest PROJ-123
```

### 2. README.md — go install パス修正

**現状** (line 18):
```
go install github.com/youyo/logvalet/cmd/lv@latest
```

**変更後**:
```
go install github.com/youyo/logvalet/cmd/logvalet@latest
```

### 3. README.md — Configuration セクションに config init の案内を追加

**現状** (line 47-59): config.toml と tokens.json のパスだけ記載

**変更後**: 先頭に `logvalet config init` でセットアップする旨を追加

### 4. SKILL.md — Auth and configuration の認証モード説明を修正

**現状** (line 111-117):
```
Primary auth mode:
- OAuth with localhost callback
Secondary auth mode:
- API key override
```

**変更後**:
```
Primary auth mode:
- API key (via `config init --init-api-key` or `auth login`)
Secondary auth mode:
- OAuth (access token via `--access-token` flag or env var)
```

理由: 現在の `auth login` は API キーのみ対応。OAuth は credentials パッケージに実装済みだが CLI フローは未接続。

### 5. SKILL.md — "Explicitly unsupported in MVP" の表現を更新

**現状** (line 77):
```
Explicitly unsupported in MVP:
```

**変更後**:
```
Explicitly unsupported:
```

「MVP」は開発完了済みのため不適切。

### 6. SKILL.md — issue list の --project フラグ説明

**現状** (line 404-413): `--project PROJ` と記載
**確認**: CLI 実装では `--project-key` / `-k` フラグ。

実装コードを確認 (`internal/cli/issue.go:43`):
```go
ProjectKey []string `short:"k" help:"プロジェクトキー"`
```

Kong のデフォルトでは構造体フィールド名がケバブケースになるため `--project-key` が正式名。
SKILL.md の `--project PROJ` は不正確 → `--project-key PROJ` に修正。

### 7. docs/specs/logvalet_SKILL.md の同期

skills/SKILL.md の変更後、同じ内容をコピー。

## 検証方法

- 3ファイルの diff を確認
- skills/SKILL.md と docs/specs/logvalet_SKILL.md が一致することを確認
