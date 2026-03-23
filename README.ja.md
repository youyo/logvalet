# logvalet

**logvalet** は [Backlog](https://backlog.com/) 向けの LLM-first CLI ツールです。

薄い API ラッパーではありません。主な目的は、Backlog のデータを Claude Code・Codex などのコーディングエージェントが利用しやすい**安定した・コンパクトな・機械可読なダイジェスト JSON** に変換することです。

## インストール

### Homebrew

```bash
brew install youyo/tap/logvalet
```

### go install

```bash
go install github.com/youyo/logvalet/cmd/lv@latest
```

インストールされるバイナリ名は `logvalet` です。シェルで `lv` エイリアスを設定することを推奨します:

```bash
alias lv=logvalet
```

## クイックスタート

### 認証

```bash
logvalet auth login --profile work
```

### Issue ダイジェストの取得

```bash
logvalet issue digest PROJ-123
```

### ショートエイリアス

```bash
lv issue digest PROJ-123
```

## 設定

設定ファイル:

```text
~/.config/logvalet/config.toml
```

トークンストア:

```text
~/.config/logvalet/tokens.json
```

## シェル補完

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

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `auth login` | OAuth 認証 |
| `auth logout` | 認証情報の削除 |
| `auth whoami` | 現在のアイデンティティを表示 |
| `auth list` | 設定済みプロファイル一覧 |
| `completion bash/zsh/fish` | シェル補完スクリプト生成 |
| `issue get <KEY>` | Issue の取得 |
| `issue list` | Issue 一覧（フィルタ付き） |
| `issue digest <KEY>` | コンテキスト付き Issue ダイジェスト |
| `issue create` | Issue の作成 |
| `issue update <KEY>` | Issue の更新 |
| `issue comment list <KEY>` | コメント一覧 |
| `issue comment add <KEY>` | コメントの追加 |
| `issue comment update <KEY> <ID>` | コメントの更新 |
| `project get <KEY>` | プロジェクトの取得 |
| `project list` | プロジェクト一覧 |
| `project digest <KEY>` | コンテキスト付きプロジェクトダイジェスト |
| `activity list` | アクティビティ一覧 |
| `activity digest` | 期間指定のアクティビティダイジェスト |
| `user list` | スペースユーザー一覧 |
| `user get <ID>` | ユーザーの取得 |
| `user activity <ID>` | ユーザーアクティビティ |
| `user digest <ID>` | ユーザーアクティビティダイジェスト |
| `document get <ID>` | ドキュメントの取得 |
| `document list` | プロジェクト内ドキュメント一覧 |
| `document tree` | ドキュメントツリー |
| `document digest <ID>` | コンテキスト付きドキュメントダイジェスト |
| `document create` | ドキュメントの作成 |
| `meta status <KEY>` | プロジェクトステータス一覧 |
| `meta category <KEY>` | プロジェクトカテゴリ一覧 |
| `meta version <KEY>` | プロジェクトバージョン一覧 |
| `meta custom-field <KEY>` | カスタムフィールド一覧 |
| `team list` | チーム一覧 |
| `team project <KEY>` | プロジェクトのチーム一覧 |
| `team digest <ID>` | コンテキスト付きチームダイジェスト |
| `space info` | スペース情報の表示 |
| `space disk-usage` | ディスク使用量の表示 |
| `space digest` | スペース概要ダイジェスト |

## グローバルフラグ

```text
--profile, -p <name>     使用するプロファイル
--format, -f <format>    出力フォーマット: json（デフォルト）, md, text, yaml
--pretty                 JSON の整形出力
--config, -c <path>      設定ファイルパス
--api-key <key>          Backlog API キー
--access-token <token>   OAuth アクセストークン
--base-url <url>         Backlog ベース URL
--space, -s <space>      スペースキー
--verbose, -v            詳細出力
--no-color               カラー出力を無効化
```

## 課題のフィルタリング

担当者・ステータス・期限日で課題を絞り込みます:

```bash
# 自分の未完了課題を一覧
logvalet issue list --assignee me --status open -k PROJECT_KEY

# 特定ユーザーの課題を一覧
logvalet issue list --assignee "田中太郎" -k PROJECT_KEY

# 期限超過の課題を確認
logvalet issue list --assignee me --due-date overdue -k PROJECT_KEY

# 今日が期限の課題を確認
logvalet issue list --assignee me --due-date today -k PROJECT_KEY

# ステータス名で絞り込み
logvalet issue list --status "未対応,処理中" -k PROJECT_KEY

# ステータスIDで絞り込み
logvalet issue list --status 1

# 全体の完了以外の課題を一覧（プロジェクトキー不要）
logvalet issue list --status not-closed

# 今月が期限の課題を一覧
logvalet issue list --due-date this-month

# 今週が期限の課題を期限順に表示
logvalet issue list --due-date this-week --sort dueDate --order asc

# 特定期間の課題を一覧
logvalet issue list --due-date 2026-03-01:2026-03-31

# 指定日以降が期限の課題を一覧
logvalet issue list --due-date 2026-03-20:

# 指定日までが期限の課題を一覧
logvalet issue list --due-date :2026-03-31

# 複合条件：自分の完了以外の課題を期限順に表示
logvalet issue list --assignee me --status not-closed --sort dueDate --order asc
```

| フラグ | 指定値 | 説明 |
|--------|--------|------|
| `--assignee` | `me`、ユーザーID、またはユーザー名 | 担当者で絞り込み |
| `--status` | `open`、`not-closed`、ステータス名（カンマ区切り可）、ステータスID | ステータスで絞り込み。`open` は完了以外。`not-closed` も完了以外（プロジェクトキー不要）。名前/`open` は `-k` 必須 |
| `--due-date` | `today`、`overdue`、`this-week`、`this-month`、`YYYY-MM-DD`、`YYYY-MM-DD:YYYY-MM-DD` | 期限日で絞り込み。日付範囲は開端記法に対応（`:YYYY-MM-DD` または `YYYY-MM-DD:`） |
| `--sort` | `dueDate`、`created`、`updated`、`priority`、`status`、`assignee` | 結果のソート対象フィールド |
| `--order` | `asc`、`desc` | ソート順序。デフォルト: `desc` |

注: `--due-date` 指定時は自動ページング機能で全件取得されます（上限10,000件）。

## 出力

デフォルト出力は JSON です。`--format` で変更できます:

```bash
lv issue digest PROJ-123 --format md
lv issue digest PROJ-123 --format yaml
lv issue digest PROJ-123 --format text
```

## 安全性

書き込み操作は `--dry-run` でリクエストペイロードを確認してから実行できます:

```bash
lv issue create --project PROJ --summary "バグ修正" --issue-type "Bug" --dry-run
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
```

## ライセンス

MIT
