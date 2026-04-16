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

### ダイジェストの取得

```bash
# 単一課題
logvalet digest --issue PROJ-123

# プロジェクト + ユーザーの今月の活動
logvalet digest --project PROJ --user me --since this-month

# チームの今週の活動
logvalet digest --team 173843 --since this-week
```

### ショートエイリアス

```bash
lv digest --issue PROJ-123
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
| `digest` | 課題・プロジェクト・ユーザー・チーム・スペースのダイジェスト生成 |
| `issue get <KEY>` | 課題の取得 |
| `issue list` | 課題一覧（フィルタ付き） |
| `issue create` | 課題の作成 |
| `issue update <KEY>` | 課題の更新 |
| `issue comment list <KEY>` | コメント一覧 |
| `issue comment add <KEY>` | コメントの追加 |
| `issue comment update <KEY> <ID>` | コメントの更新 |
| `issue attachment list <KEY>` | 添付ファイル一覧 |
| `issue attachment get <KEY> <ID>` | 添付ファイル情報の取得 |
| `issue attachment download <KEY> <ID>` | 添付ファイルのダウンロード |
| `issue attachment delete <KEY> <ID>` | 添付ファイルの削除 |
| `issue context <KEY>` | 課題の判断材料を一括取得（詳細・コメント・分析シグナル） |
| `issue stale` | プロジェクトの停滞課題を検出 |
| `project get <KEY>` | プロジェクトの取得 |
| `project list` | プロジェクト一覧 |
| `project blockers <KEY>` | プロジェクトのブロッカー検出（停滞・未アサイン・期限超過） |
| `project health <KEY>` | プロジェクト健全性の統合ビュー |
| `user workload <KEY>` | ユーザー負荷状況の分析 |
| `activity list` | アクティビティ一覧 |
| `user list` | スペースユーザー一覧 |
| `user get <ID>` | ユーザーの取得 |
| `user activity <ID>` | ユーザーアクティビティ |
| `document get <ID>` | ドキュメントの取得 |
| `document list` | プロジェクト内ドキュメント一覧 |
| `document tree` | ドキュメントツリー |
| `document create` | ドキュメントの作成 |
| `meta status <KEY>` | プロジェクトステータス一覧 |
| `meta category <KEY>` | プロジェクトカテゴリ一覧 |
| `meta version <KEY>` | プロジェクトバージョン一覧 |
| `meta custom-field <KEY>` | カスタムフィールド一覧 |
| `team list` | チーム一覧 |
| `team project <KEY>` | プロジェクトのチーム一覧 |
| `space info` | スペース情報の表示 |
| `space disk-usage` | ディスク使用量の表示 |
| `shared-file list` | プロジェクトの共有ファイル一覧 |
| `shared-file get <FILE-ID>` | 共有ファイル情報の取得 |
| `shared-file download <FILE-ID>` | 共有ファイルのダウンロード |
| `star add` | スター追加（課題、コメント、Wiki等） |
| `watching list <USER-ID>` | ウォッチ一覧取得（`me` 対応） |
| `watching count <USER-ID>` | ウォッチ件数取得 |
| `watching get <WATCHING-ID>` | ウォッチ詳細取得 |
| `watching add <ISSUE-ID-OR-KEY>` | ウォッチ追加 |
| `watching update <WATCHING-ID>` | ウォッチのメモ更新 |
| `watching delete <WATCHING-ID>` | ウォッチ削除 |
| `watching mark-as-read <WATCHING-ID>` | ウォッチ既読化 |
| `mcp` | MCP サーバー起動（Streamable HTTP） |

## AI 分析コマンド

Phase 1 で、プロジェクトの洞察と意思決定支援のための AI 指向分析コマンドが追加されました:

| コマンド | 説明 |
|---------|------|
| `issue context <KEY>` | 課題の判断材料を一括取得（詳細・コメント・分析シグナル） |
| `issue stale -k <PROJECT>` | N日以上更新のない停滞課題を検出 |
| `project blockers <PROJECT>` | ブロッカー検出（停滞高優先度・未アサイン・期限超過） |
| `user workload <PROJECT>` | ユーザーごとの未完了課題数・期限超過分布を分析 |
| `project health <PROJECT>` | 停滞検出・ブロッカー・負荷を統合した健全性ビュー |

### 利用例

```bash
# 課題のコンテキストを一括取得
logvalet issue context PROJ-123

# 7日以上更新のない停滞課題を検出
logvalet issue stale -k PROJ --days 7

# コメントを含むブロッカー検出
logvalet project blockers PROJ --days 14 --include-comments

# 完了済みステータスを除いたユーザー負荷分析
logvalet user workload PROJ --exclude-status "完了,却下"

# プロジェクト健全性の統合レポート
logvalet project health PROJ --days 7
```

## AI ワークフローコマンド（Phase 2）

Phase 2 で、LLM 支援の意思決定に向けた構造化された材料を提供するワークフロー向けコマンドが追加されました:

| コマンド | 説明 |
|---------|------|
| `issue triage-materials <KEY>` | 課題のトリアージ材料を構造化して取得（属性・履歴・類似課題統計） |
| `digest weekly -k <PROJECT>` | 週次活動集約（完了・開始・ブロック中の課題） |
| `digest daily -k <PROJECT>` | 日次活動スナップショット |

### 設計方針

logvalet は **deterministic な材料** を提供します。LLM による判断（優先度提案・コメント下書き等）は SKILL 側が担います。

### 利用例

```bash
# 課題のトリアージ材料を取得
logvalet issue triage-materials PROJ-123

# プロジェクトの週次活動ダイジェスト
logvalet digest weekly -k PROJ

# 日次活動スナップショット
logvalet digest daily -k PROJ
```

## AI インテリジェンスコマンド（Phase 3）

Phase 3 で、LLM 支援の意思決定・異常検知・リスク評価に向けた構造化された材料を提供するインテリジェンス向けコマンドが追加されました:

| コマンド | 説明 |
|---------|------|
| `issue timeline <KEY>` | 課題のコメント・更新履歴を時系列で取得（意思決定ログの材料） |
| `activity stats` | アクティビティ統計（タイプ別・アクター別・時間帯別・パターン）を集計 |

### 設計方針

logvalet は **deterministic な材料** を提供します。LLM による判断（意思決定の抽出・異常の解釈・リスク評価）は SKILL 側が担います。

### 利用例

```bash
# 意思決定ログ抽出用に課題のタイムラインを取得
logvalet issue timeline PROJ-123

# 特定期間のタイムライン取得
logvalet issue timeline PROJ-123 --since 2026-01-01 --until 2026-03-31

# プロジェクトのアクティビティ統計を取得
logvalet activity stats --scope project -k PROJ

# 期間指定・上位件数指定でアクティビティ統計を取得
logvalet activity stats --scope project -k PROJ --since 2026-01-01T00:00:00Z --until 2026-03-31T23:59:59Z --top-n 10
```

---

## グローバルフラグ

```text
--profile, -p <name>     使用するプロファイル
--format, -f <format>    出力フォーマット: json（デフォルト）, yaml, md, gantt
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

# チームメンバーの課題を一覧（チーム名または部分一致で指定）
logvalet issue list --assignee "ヘプタゴン" --status not-closed --due-date this-week

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

# 開始日で絞り込み（今月開始の課題）
logvalet issue list --start-date this-month

# 開始日の範囲で絞り込み
logvalet issue list --start-date 2026-03-01:2026-03-31

# --start-date と --due-date を同時指定（AND 条件）
logvalet issue list --start-date this-month --due-date this-month
```

| フラグ | 指定値 | 説明 |
|--------|--------|------|
| `--assignee` | `me`、ユーザーID、ユーザー名、またはチーム名 | 担当者で絞り込み。チーム名（部分一致可）を指定するとチームメンバー全員の課題を表示 |
| `--status` | `open`、`not-closed`、ステータス名（カンマ区切り可）、ステータスID | ステータスで絞り込み。`open` は完了以外。`not-closed` も完了以外（プロジェクトキー不要）。名前/`open` は `-k` 必須 |
| `--due-date` | `today`、`overdue`、`this-week`、`this-month`、`YYYY-MM-DD`、`YYYY-MM-DD:YYYY-MM-DD` | 期限日で絞り込み。日付範囲は開端記法に対応（`:YYYY-MM-DD` または `YYYY-MM-DD:`） |
| `--start-date` | `today`、`this-week`、`this-month`、`YYYY-MM-DD`、`YYYY-MM-DD:YYYY-MM-DD` | 開始日で絞り込み。日付範囲は開端記法に対応。`--due-date` との同時指定可（AND 結合）。 |
| `--sort` | `dueDate`、`created`、`updated`、`priority`、`status`、`assignee` | 結果のソート対象フィールド |
| `--order` | `asc`、`desc` | ソート順序。デフォルト: `desc` |

注: `--due-date` または `--start-date` 指定時は自動ページング機能で全件取得されます（上限10,000件）。

## ダイジェストコマンド

`digest` コマンドは、期間指定で Backlog データの安定した構造化サマリーを生成します。プロジェクト・ユーザー・チーム・課題でフィルタ可能で、LLM エージェント向けに最適化されたコンパクト機械可読形式で出力されます。

### 利用例

```bash
# 単一課題のコンテキスト付きダイジェスト
logvalet digest --issue PROJ-123

# プロジェクト + ユーザーの今月の実績
logvalet digest --project HEP_ISSUES --user "石澤直人" --since this-month

# 複数プロジェクト + 複数ユーザー（AND 条件）
logvalet digest --project HEP_ISSUES --project TAISEI --user "石澤" --user "須合" --since this-month

# チームの今週の実績
logvalet digest --team 173843 --since this-week

# スペース全体の今月ダイジェスト
logvalet digest --since this-month

# カスタム期間
logvalet digest --project PROJ --user me --since 2026-03-01 --until 2026-03-31
```

### フラグ

| フラグ | 指定値 | 説明 |
|--------|--------|------|
| `--issue` | 課題キー（例: `PROJ-123`） | 単一課題のダイジェスト。複数指定可。 |
| `--project` | プロジェクトキー（例: `HEP_ISSUES`） | プロジェクトで絞り込み。複数指定可。 |
| `--user` | `me`、ユーザーID、またはユーザー名 | ユーザーの活動で絞り込み。複数指定可。 |
| `--team` | チームID | チームメンバーの活動で絞り込み。複数指定可。 |
| `--since` | `today`、`this-week`、`this-month`、`YYYY-MM-DD` | 期間開始（必須）。課題は `updatedSince` でフィルタ。 |
| `--until` | `today`、`this-week`、`this-month`、`YYYY-MM-DD` | 期間終了（オプション）。課題は `updatedUntil` でフィルタ。 |
| `--start-date` | `today`、`this-week`、`this-month`、`YYYY-MM-DD` | 課題の開始日（スケジュール）で絞り込み。`--since`/`--until` とは独立。 |
| `--due-date` | `today`、`this-week`、`this-month`、`YYYY-MM-DD` | 課題の期限日（スケジュール）で絞り込み。`--since`/`--until` とは独立。 |

### 補足

- フィルタを指定しない場合、スペース全体の期間別サマリーを生成します
- 複数の `--project`・`--user`・`--team`・`--issue` フラグは AND 条件で結合されます
- `--since`/`--until` は更新日時（`updatedSince`/`updatedUntil`）で絞り込みます
- `--start-date`/`--due-date` はスケジュール日付で絞り込み、更新日ウィンドウとは独立して動作します
- ダイジェスト出力には概要統計・主要課題・アクティビティパターンが含まれます

### スケジュール日付フィルタを使ったダイジェスト

```bash
# 今月開始の課題のダイジェスト
logvalet digest --project PROJ --since this-month --start-date this-month

# 今週が期限の課題のダイジェスト
logvalet digest --project PROJ --since this-month --due-date this-week

# スケジュール日付のみで絞り込み（更新日ウィンドウ不要）
logvalet digest --project PROJ --start-date 2026-03-01 --due-date 2026-03-31
```

## 出力

デフォルト出力は JSON です。`--format` で変更できます:

| フォーマット | 説明 |
|------------|------|
| `json` | 機械可読 JSON（デフォルト） |
| `yaml` | YAML 出力 |
| `md` | リッチ Markdown — 配列はテーブル形式、単体オブジェクトはキー・値リスト形式 |
| `gantt` | Issue 専用 Gantt テーブル — 日付列・経過/残り日数・Backlog URL 付き |

```bash
# Markdown テーブル出力（汎用）
lv issue list --due-date this-month --format md

# YAML 出力
lv issue get PROJ-123 --format yaml
```

### Gantt フォーマット

`--format gantt` を `issue list` と組み合わせると、日付付き Gantt テーブルを生成します。各行に課題キー・件名・開始日/期限日・経過日数・残り日数・Backlog の直接 URL が表示されます。開始日または期限日が設定されていない課題はスキップされ、stderr に警告が出力されます。

```bash
# 今月が期限の課題を Gantt テーブルで表示
logvalet issue list --due-date this-month --format gantt

# プロジェクトで絞り込んだ Gantt テーブル
logvalet issue list -k PROJ --start-date this-month --format gantt
```

## 添付ファイル

課題の添付ファイルを管理します:

```bash
# 課題の添付ファイル一覧を表示
logvalet issue attachment list PROJ-123

# 添付ファイル情報を取得
logvalet issue attachment get PROJ-123 12345

# 添付ファイルをダウンロード
logvalet issue attachment download PROJ-123 12345 --output ./file.pdf

# 添付ファイルを削除（--dry-run で確認）
logvalet issue attachment delete PROJ-123 12345 --dry-run
logvalet issue attachment delete PROJ-123 12345
```

## 共有ファイル

プロジェクト内の共有ファイルを管理します:

```bash
# プロジェクトの共有ファイル一覧を表示
logvalet shared-file list --project PROJ

# 特定ディレクトリ内のファイルを一覧
logvalet shared-file list --project PROJ --path "/docs/technical"

# 共有ファイル情報を取得
logvalet shared-file get --project PROJ abc123def

# 共有ファイルをダウンロード
logvalet shared-file download --project PROJ abc123def --output ./file.pdf
```

## スター

課題・コメント・Wiki・プルリクエストにスターを追加します:

```bash
# 課題にスターを追加
logvalet star add --issue-id 12345

# コメントにスターを追加
logvalet star add --comment-id 67890

# Wiki ページにスターを追加
logvalet star add --wiki-id wiki123

# プルリクエストにスターを追加
logvalet star add --pr-id pr456

# プルリクエストコメントにスターを追加
logvalet star add --pr-comment-id prcomment789
```

## ウォッチ

課題のウォッチを管理 — 担当ではなくても気にかけている課題を追跡:

```bash
# 自分のウォッチ一覧（"me" は認証ユーザーに自動解決）
logvalet watching list me

# ウォッチ件数
logvalet watching count me

# ウォッチ詳細
logvalet watching get 2997876

# 課題をウォッチに追加
logvalet watching add PROJ-123 --note "依存先の課題"

# ウォッチのメモを更新
logvalet watching update 2997876 --note "更新されたメモ"

# ウォッチを削除
logvalet watching delete 2997876

# 既読にする
logvalet watching mark-as-read 2997876
```

## MCP サーバー

logvalet は Model Context Protocol (MCP) サーバーとして実行でき、すべての操作を Claude Desktop や Claude Code のツールとして公開できます:

```bash
# MCP サーバーを起動（デフォルト: 127.0.0.1:8080）
logvalet mcp

# カスタムホストとポート指定
logvalet mcp --host 0.0.0.0 --port 9000
```

MCP サーバーは以下を含む 31 個以上のツールを提供します:
- `logvalet_issue_get`, `logvalet_issue_list`, `logvalet_issue_create`
- `logvalet_project_get`, `logvalet_project_list`
- `logvalet_digest`
- `logvalet_shared_file_list`, `logvalet_shared_file_download`
- `logvalet_star_add`
- `logvalet_issue_context` — 課題コンテキスト分析
- `logvalet_issue_stale` — 停滞課題検出
- `logvalet_project_blockers` — ブロッカー検出
- `logvalet_user_workload` — ユーザー負荷分析
- `logvalet_project_health` — プロジェクト健全性統合ビュー
- `logvalet_issue_triage_materials` — トリアージ材料収集
- `logvalet_digest_weekly`, `logvalet_digest_daily` — 定期活動ダイジェスト
- `logvalet_issue_timeline` — 課題コメント・更新履歴（時系列）
- `logvalet_activity_stats` — アクティビティ統計・パターン分析
- その他多数...

Claude Desktop の設定または Claude Code のスキル設定で MCP サーバーを設定し、logvalet をツールとして使用できます。

### サポートされる動作モード

logvalet は CLI / MCP × Backlog 認証方式（API key / OAuth）× MCP クライアント認証（OIDC via `idproxy`）の組み合わせで 4 パターンをサポートします。2 つの組み合わせは **未対応** です（下表参照）。

| # | クライアント | Backlog 認証 | クライアント認証 (OIDC) | 状態 |
|---|------------|-------------|------------------------|------|
| 1 | CLI | API key | — | ✅ サポート |
| 2 | CLI | OAuth | — | ❌ 未実装（CLI 向け OAuth ログインコマンドは未配線。現状は `tokens.json` を手動編集した場合のみ動作） |
| 3 | MCP | API key | なし | ✅ サポート |
| 4 | MCP | API key | OIDC | ✅ サポート |
| 5 | MCP | OAuth | なし | ❌ 非サポート（設計上） — OAuth は per-user で、userID の取得元が OIDC subject のみ。`logvalet mcp` を `LOGVALET_BACKLOG_CLIENT_ID` 設定 + `--auth` 無しで起動すると fast-fail エラーになります |
| 6 | MCP | OAuth | OIDC | ✅ サポート |

以下の例ではサポートされる 4 モードについて、(A) 環境変数のみ・(B) CLI 引数のみ（フラグに対応しない設定は必要最小限の環境変数）の 2 通りで記載しています。

#### Mode 1: CLI + API key

(A) 環境変数:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

logvalet auth login
logvalet issue get EXAMPLE-1
```

(B) CLI 引数:

```bash
logvalet --api-key=your-api-key-here --space=example-space auth login
logvalet --space=example-space issue get EXAMPLE-1
```

#### Mode 3: MCP + API key（クライアント認証なし）

(A) 環境変数:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

logvalet mcp
```

(B) CLI 引数:

```bash
logvalet mcp --api-key=your-api-key-here --space=example-space
```

#### Mode 4: MCP + API key + OIDC (idproxy)

(A) 環境変数:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

export LOGVALET_MCP_AUTH=true
export LOGVALET_MCP_EXTERNAL_URL=https://mcp.example.com
export LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0
export LOGVALET_MCP_OIDC_CLIENT_ID=your-oidc-client-id-here
export LOGVALET_MCP_OIDC_CLIENT_SECRET=your-oidc-client-secret-here
export LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32)
export LOGVALET_MCP_ALLOWED_DOMAINS=example.com

logvalet mcp
```

(B) CLI 引数:

```bash
logvalet mcp \
  --api-key=your-api-key-here \
  --space=example-space \
  --auth \
  --external-url=https://mcp.example.com \
  --oidc-issuer=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0 \
  --oidc-client-id=your-oidc-client-id-here \
  --oidc-client-secret=your-oidc-client-secret-here \
  --cookie-secret=$(openssl rand -hex 32) \
  --allowed-domains=example.com
```

#### Mode 6: MCP + Backlog OAuth + OIDC

Backlog OAuth 関連の設定（`LOGVALET_BACKLOG_*`, `LOGVALET_OAUTH_STATE_SECRET`, `LOGVALET_TOKEN_STORE*`）は **環境変数のみ** で設定します（CLI フラグは用意していません）。

(A) 環境変数:

```bash
# Backlog space
export LOGVALET_SPACE=example-space

# OIDC (idproxy)
export LOGVALET_MCP_AUTH=true
export LOGVALET_MCP_EXTERNAL_URL=https://mcp.example.com
export LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0
export LOGVALET_MCP_OIDC_CLIENT_ID=your-oidc-client-id-here
export LOGVALET_MCP_OIDC_CLIENT_SECRET=your-oidc-client-secret-here
export LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32)
export LOGVALET_MCP_ALLOWED_DOMAINS=example.com

# Backlog OAuth
export LOGVALET_BACKLOG_CLIENT_ID=your-backlog-oauth-client-id-here
export LOGVALET_BACKLOG_CLIENT_SECRET=your-backlog-oauth-client-secret-here
export LOGVALET_BACKLOG_REDIRECT_URL=https://mcp.example.com/oauth/backlog/callback
export LOGVALET_OAUTH_STATE_SECRET=$(openssl rand -hex 32)

# Token store（Lambda では DynamoDB 推奨）
export LOGVALET_TOKEN_STORE=dynamodb
export LOGVALET_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
export LOGVALET_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1

logvalet mcp
```

(B) CLI 引数（OIDC はフラグ、Backlog OAuth は環境変数のまま）:

```bash
# Backlog OAuth 関連は CLI フラグ無し。環境変数で設定する必要があります
export LOGVALET_BACKLOG_CLIENT_ID=your-backlog-oauth-client-id-here
export LOGVALET_BACKLOG_CLIENT_SECRET=your-backlog-oauth-client-secret-here
export LOGVALET_BACKLOG_REDIRECT_URL=https://mcp.example.com/oauth/backlog/callback
export LOGVALET_OAUTH_STATE_SECRET=$(openssl rand -hex 32)
export LOGVALET_TOKEN_STORE=dynamodb
export LOGVALET_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
export LOGVALET_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1

logvalet mcp \
  --space=example-space \
  --auth \
  --external-url=https://mcp.example.com \
  --oidc-issuer=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0 \
  --oidc-client-id=your-oidc-client-id-here \
  --oidc-client-secret=your-oidc-client-secret-here \
  --cookie-secret=$(openssl rand -hex 32) \
  --allowed-domains=example.com
```

Token Store の詳細や初回接続フローは後述の **Backlog OAuth（ユーザーごとの認可）** を参照してください。

### 認証（オプション）

リモートデプロイ向けに OIDC/OAuth 2.1 認証を有効化できます:

```bash
logvalet mcp --auth \
  --external-url https://logvalet.example.com \
  --oidc-issuer https://accounts.google.com \
  --oidc-client-id YOUR_CLIENT_ID \
  --cookie-secret $(openssl rand -hex 32)
```

すべての認証フラグは環境変数でも設定可能です（例: `LOGVALET_MCP_AUTH=true`）。詳細は [AgentCore デプロイガイド](docs/agentcore-deployment.md) を参照してください。

認証有効時:
- `/mcp` は Bearer トークンが必要（OAuth 2.1 + PKCE）
- `/healthz` は常に認証なしでアクセス可能
- OAuth エンドポイント（`/register`, `/authorize`, `/token`, `/.well-known/*`）は自動的に処理されます

### Backlog OAuth（ユーザーごとの認可）

リモート MCP 構成では **Backlog OAuth 2.0** を上乗せして、各 Backlog API 呼び出しを **呼び出しユーザー自身の Backlog 権限** で実行できます。上記の OIDC 認証とは責務を分離しています。

- **認証 (AuthN)**: `idproxy` が OIDC (Entra ID / Google 等) でユーザーを確認
- **認可 (AuthZ)**: ユーザーごとの Backlog OAuth access token で Backlog API を呼び出す

両者は独立しています。OIDC トークンが Backlog API に流用されることはなく、Backlog トークンが MCP 認証に使われることもありません。

Backlog OAuth モードは次の **両方** が設定されている場合のみ有効化されます:
1. `--auth`（または `LOGVALET_MCP_AUTH=true`）が有効
2. `LOGVALET_BACKLOG_CLIENT_ID` が設定されている

#### Token Store

Backlog のトークンはプラガブルな Token Store に保存します。用途に応じて選択してください:

| Store | 推奨用途 | 補足 |
|-------|---------|------|
| `memory` | ローカル開発 / 単一インスタンス Lambda | デフォルト。プロセス再起動で消失 |
| `sqlite` | セルフホスト / ローカル CLI | pure-Go（`modernc.org/sqlite`）、CGO 不要 |
| `dynamodb` | Lambda / マルチインスタンス | VPC 不要、AWS マネージド |

#### 初回接続フロー

1. Claude 上で Backlog ツールを呼び出す → 未接続なら logvalet が `provider_not_connected` と接続 URL を返す。
2. ブラウザで `GET /oauth/backlog/authorize` にアクセス → logvalet が Backlog の同意画面へリダイレクト。
3. Backlog 上で同意 → Backlog が `/oauth/backlog/callback` にリダイレクト。
4. logvalet が code を交換し、OIDC subject をキーに token を保存して `{"status":"connected"}` を返す。
5. 以降、そのユーザーのツール呼び出しは保存済みトークンで自動実行される。
6. `GET /oauth/backlog/status` で状態確認、`DELETE /oauth/backlog/disconnect` で切断が可能。

#### 環境変数

OAuth 設定はすべて環境変数で行います（設定ファイル不要）:

| 環境変数 | 必須 | デフォルト | 説明 |
|---------|------|-----------|------|
| `LOGVALET_BACKLOG_CLIENT_ID` | Yes | — | Backlog OAuth クライアント ID |
| `LOGVALET_BACKLOG_CLIENT_SECRET` | Yes | — | Backlog OAuth クライアントシークレット |
| `LOGVALET_BACKLOG_REDIRECT_URL` | Yes | — | OAuth コールバック URL（`https://<ホスト>/oauth/backlog/callback`） |
| `LOGVALET_OAUTH_STATE_SECRET` | Yes | — | state JWT の HMAC-SHA256 署名鍵（hex、64 文字以上） |
| `LOGVALET_TOKEN_STORE` | No | `memory` | `memory` / `sqlite` / `dynamodb` |
| `LOGVALET_TOKEN_STORE_SQLITE_PATH` | sqlite 時 | `./logvalet.db` | SQLite DB のパス |
| `LOGVALET_TOKEN_STORE_DYNAMODB_TABLE` | dynamodb 時 | — | DynamoDB テーブル名 |
| `LOGVALET_TOKEN_STORE_DYNAMODB_REGION` | dynamodb 時 | — | DynamoDB のリージョン |

#### 起動例

```bash
# idproxy (OIDC) 設定 — 上記「認証（オプション）」参照
export LOGVALET_MCP_AUTH=true
export LOGVALET_MCP_EXTERNAL_URL=https://mcp.example.com
export LOGVALET_MCP_OIDC_ISSUER=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0
export LOGVALET_MCP_OIDC_CLIENT_ID=your-oidc-client-id-here
export LOGVALET_MCP_OIDC_CLIENT_SECRET=your-oidc-client-secret-here
export LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32)
export LOGVALET_MCP_ALLOWED_DOMAINS=example.com

# Token Store（Lambda では DynamoDB 推奨）
export LOGVALET_TOKEN_STORE=dynamodb
export LOGVALET_TOKEN_STORE_DYNAMODB_TABLE=logvalet-oauth-tokens
export LOGVALET_TOKEN_STORE_DYNAMODB_REGION=ap-northeast-1

# Backlog OAuth クライアント（Backlog スペースで作成）
export LOGVALET_BACKLOG_CLIENT_ID=your-backlog-oauth-client-id-here
export LOGVALET_BACKLOG_CLIENT_SECRET=your-backlog-oauth-client-secret-here
export LOGVALET_BACKLOG_REDIRECT_URL=https://mcp.example.com/oauth/backlog/callback
export LOGVALET_OAUTH_STATE_SECRET=$(openssl rand -hex 32)

logvalet mcp --auth
```

起動すると次のようなログが出力されます:

```
logvalet MCP server (auth + OAuth) listening on 127.0.0.1:8080/mcp
  OAuth routes: /oauth/backlog/{authorize,callback,status,disconnect}
```

### Docker / AgentCore デプロイ

```bash
# ビルド
docker build -t logvalet .

# 実行（認証なし）
docker run -p 8080:8080 \
  -e LOGVALET_API_KEY=your-api-key \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet

# 実行（認証あり）
docker run -p 8080:8080 \
  -e LOGVALET_MCP_AUTH=true \
  -e LOGVALET_MCP_EXTERNAL_URL=https://logvalet.example.com \
  -e LOGVALET_MCP_OIDC_ISSUER=https://accounts.google.com \
  -e LOGVALET_MCP_OIDC_CLIENT_ID=your-client-id \
  -e LOGVALET_MCP_COOKIE_SECRET=$(openssl rand -hex 32) \
  -e LOGVALET_API_KEY=your-api-key \
  -e LOGVALET_BASE_URL=https://your-space.backlog.com \
  logvalet
```

AWS Bedrock AgentCore Runtime へのデプロイ方法は [docs/agentcore-deployment.md](docs/agentcore-deployment.md) を参照してください。

### Lambda Function URL (lambroll)

[lambroll](https://github.com/fujiwara/lambroll) と [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter) を使用して logvalet を Lambda Function URL にデプロイできます。
セットアップ手順は [examples/lambroll/](examples/lambroll/) を参照してください。

### タスクランナー（mise）

```bash
mise run build              # バイナリビルド
mise run test               # 全テスト実行
mise run test:integration   # 統合テスト実行
mise run vet                # go vet 実行
mise run lint               # vet + test 実行
mise run mcp:start          # MCP サーバー起動（ローカル）
mise run mcp:start-auth     # MCP サーバー起動（認証あり）
mise run docker:build       # Docker イメージビルド
```

## 安全性

書き込み操作は `--dry-run` でリクエストペイロードを確認してから実行できます:

```bash
lv issue create --project PROJ --summary "バグ修正" --issue-type "Bug" --dry-run
lv issue comment add PROJ-123 --content-file ./comment.md --dry-run
lv issue attachment delete PROJ-123 12345 --dry-run
```

## スキル

logvalet には、AI コーディングエージェントに logvalet コマンドの効果的な使用方法を教えるエージェントスキルが含まれます。

### インストール（全サポートエージェント）

```bash
npx skills add youyo/logvalet
```

### インストール（Claude Code のみ）

```bash
npx skills add youyo/logvalet -a claude-code
```

### 利用可能なスキル

| スキル | 説明 |
|--------|------|
| `logvalet:logvalet` | PM メタモデルハブ：全スキル一覧・ワークフロー・はじめかたガイド |
| `logvalet:report` | レポート生成・分析（プロジェクト健全性統合対応） |
| `logvalet:my-week` | 週次サマリーとタスク管理（停滞・期限超過シグナル対応） |
| `logvalet:my-next` | 次のタスク・優先順位管理（負荷状況コンテキスト対応） |
| `logvalet:issue-create` | 課題作成ワークフロー（テンプレート付き） |
| `logvalet:health` | プロジェクト健全性チェック（停滞課題・ブロッカー・ユーザー負荷） |
| `logvalet:context` | 課題コンテキスト分析（詳細・コメント・分析シグナル） |
| `logvalet:triage` | トリアージワークフロー：triage-materials をもとに LLM が優先度・担当者を提案 |
| `logvalet:draft` | 課題コンテキストをもとに LLM がコメント下書きを生成 |
| `logvalet:digest-periodic` | 週次・日次ダイジェストの LLM サマリー生成 |
| `logvalet:spec-to-issues` | spec.md を Backlog 課題に分解（SKILL 完結、CLI 不要） |
| `logvalet:decisions` | 課題タイムライン履歴から意思決定ログを抽出・要約 |
| `logvalet:intelligence` | アクティビティ統計を分析して偏り・異常・リスクを検出 |
| `logvalet:risk` | プロジェクトの統合リスク評価・推奨アクションを生成 |

インストール後、コーディングエージェントは Backlog 操作用の logvalet コマンドを自動的に認識します。

## ライセンス

MIT
