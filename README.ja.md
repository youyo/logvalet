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

### セットアップ

```bash
logvalet configure --init-profile work --init-space myspace --init-api-key YOUR_API_KEY
```

### 確認

```bash
logvalet user me
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
| `configure` | 対話的な設定初期化 |
| `completion bash/zsh/fish` | シェル補完スクリプト生成 |
| `search <KEYWORD>` | 課題・ドキュメント・Wiki をキーワードで横断検索 |
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

# 親課題で絞り込み（複数指定可）
logvalet issue list --parent-issue-id 123 --parent-issue-id 456
```

| フラグ | 指定値 | 説明 |
|--------|--------|------|
| `--assignee` | `me`、ユーザーID、ユーザー名、またはチーム名 | 担当者で絞り込み。チーム名（部分一致可）を指定するとチームメンバー全員の課題を表示 |
| `--status` | `open`、`not-closed`、ステータス名（カンマ区切り可）、ステータスID | ステータスで絞り込み。`open` は完了以外。`not-closed` も完了以外（プロジェクトキー不要）。名前/`open` は `-k` 必須 |
| `--due-date` | `today`、`overdue`、`this-week`、`this-month`、`YYYY-MM-DD`、`YYYY-MM-DD:YYYY-MM-DD` | 期限日で絞り込み。日付範囲は開端記法に対応（`:YYYY-MM-DD` または `YYYY-MM-DD:`） |
| `--start-date` | `today`、`this-week`、`this-month`、`YYYY-MM-DD`、`YYYY-MM-DD:YYYY-MM-DD` | 開始日で絞り込み。日付範囲は開端記法に対応。`--due-date` との同時指定可（AND 結合）。 |
| `--parent-issue-id` | 課題ID | 親課題IDで絞り込み。複数指定可。 |
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

# プルリクエストにスターを追加（後方互換のため --pr-id も alias として受け付けます）
logvalet star add --pull-request-id pr456

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

logvalet は Model Context Protocol (MCP) サーバーとして実行できます。まず次の2コマンドから用途に合う方を選びます:

- ローカルクライアント: `logvalet mcp-stdio` は MCP 認証なしで、選択した CLI の Backlog 資格情報をそのまま使用します。共通フラグ `--profile`、`--api-key`、`--space` に対応します。
- リモート HTTP: `logvalet mcp` は Streamable HTTP を提供し、`--auth-mode=none|apikey` を使います。明示的な space store の設定は [認証](#認証) を参照してください。

```bash
# ローカル MCP（stdio、MCP 認証なし・CLI 資格情報を使用）
logvalet mcp-stdio --profile default

# リモート MCP（HTTP）
logvalet mcp --auth-mode=apikey --auth-api-key=YOUR_GATEWAY_KEY

# カスタムホストとポート指定
logvalet mcp --host 0.0.0.0 --port 9000
```

MCP サーバーは **72 個のツール** を提供し、CLI の全サブコマンドに対応する MCP ツールが存在します。CLI と同等のオプションをサポートしており、パラメータ名は `snake_case` に変換されて JSON Schema として型付けされます。

領域別の代表的なツール:

- **課題**: `logvalet_issue_{get,list,create,update,context,stale,timeline,triage_materials}`, `logvalet_issue_comment_{list,add,update}`, `logvalet_issue_attachment_{list,get,download,delete}`
- **プロジェクト**: `logvalet_project_{get,list,blockers,health}`, `logvalet_user_workload`
- **ダイジェスト**: `logvalet_digest`, `logvalet_digest_unified`, `logvalet_digest_{weekly,daily}`, `logvalet_space_digest`, `logvalet_activity_digest`, `logvalet_document_digest`
- **ドキュメント**: `logvalet_document_{get,list,tree,create}`
- **メタ情報**: `logvalet_meta_{statuses,categories,issue_types,version,custom_field}`
- **ユーザー / チーム**: `logvalet_user_{me,list,get,activity}`, `logvalet_team_{list,get,project}`
- **スペース / 共有ファイル**: `logvalet_space_{info,disk_usage}`, `logvalet_shared_file_{list,get,download}`
- **スター / ウォッチ**: `logvalet_star_add`, `logvalet_watching_{list,count,get,add,update,delete,mark_as_read}`
- **アクティビティ**: `logvalet_activity_{list,stats}`
- **複合ツール**: `logvalet_my_tasks`

Claude Desktop の設定または Claude Code のスキル設定で MCP サーバーを設定し、logvalet をツールとして使用できます。

### バイナリダウンロードのサイズ上限

バイナリダウンロード系ツール `logvalet_issue_attachment_download` と `logvalet_shared_file_download` は、ファイル内容を base64 エンコードした文字列として JSON レスポンスに含めて返します。MCP レスポンスが肥大化しクライアント側で打ち切られるのを防ぐため、以下の上限を設けています:

- **最大サイズ: 20 MB**。制限は Backlog HTTP クライアント層で適用され、レスポンスの `Content-Length` が 20 MB を超える場合は 1 バイトも読み込む前にエラーで早期失敗します。
- 20 MB を超えるファイルは CLI を利用してください: `logvalet issue attachment download <KEY> <ID> --output <path>` または `logvalet shared-file download --project <PROJECT> <FILE-ID> --output <path>`

### MCP ツールの annotation 分類

logvalet MCP サーバーは全 72 ツールに [MCP ToolAnnotations](https://spec.modelcontextprotocol.io/specification/2025-03-26/server/tools/#tool-annotations) を付与しています。
Claude Desktop / Claude Code はこのヒントを参照してツールの自動実行可否や確認ダイアログの表示を決定します。

| カテゴリ | 件数 | 対象ツール例 | 挙動 |
|---|---|---|---|
| Read-only | 45 | `*_list`, `*_get`, `*_stats`, `*_health`, `*_digest`, `*_download` 等 | 確認ダイアログなしで自動実行 |
| Write 非冪等 | 3 | `issue_create`, `issue_comment_add`, `document_create` | 通常の書き込み確認 |
| Write 冪等 | 6 | `issue_update`, `issue_comment_update`, `star_add`, `watching_add/update/mark_as_read` | 通常の書き込み確認 |
| Destructive | 2 | `watching_delete`, `issue_attachment_delete` | 強い確認ダイアログを表示 |

> **注意**: annotations はクライアントへの**ヒント**であり、サーバー側のアクセス制御ではありません。
> annotation を変更した場合、Claude Desktop/Code のコネクタを一度切断して再接続することで新しい設定が反映されます。
> セキュリティはバックエンドの API キーまたは OAuth スコープで担保されます。

### stdio トランスポート（Claude Desktop 向け）

`logvalet mcp-stdio` は stdio トランスポートで MCP サーバーを起動します。MCP 認証はなく、選択した CLI の Backlog 資格情報をそのまま使います。共通フラグの `--profile`、`--api-key`、`--space` は stdio でも有効です。HTTP サーバーを立てずにローカル MCP クライアントと通信できるため、Claude Desktop との統合に適しています。

**方法 1: `configure` でプロファイルを事前設定する（推奨）**

```bash
# 一度だけ実行してプロファイルを作成
logvalet configure --init-profile default --init-space YOUR_SPACE --init-api-key YOUR_API_KEY

# Claude Desktop が自動起動するので手動実行は不要
logvalet mcp-stdio
logvalet mcp-stdio --profile default
```

Claude Desktop の設定例 (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "logvalet": {
      "command": "/absolute/path/to/logvalet",
      "args": ["mcp-stdio", "--profile", "default"]
    }
  }
}
```

**方法 2: 環境変数で直接 API キーを渡す（configure 不要）**

```bash
LOGVALET_API_KEY=your-api-key LOGVALET_SPACE=your-space logvalet mcp-stdio
```

Claude Desktop の設定例（`env` キーでシークレットを安全に渡す）:

```json
{
  "mcpServers": {
    "logvalet": {
      "command": "/absolute/path/to/logvalet",
      "args": ["mcp-stdio"],
      "env": {
        "LOGVALET_API_KEY": "your-api-key",
        "LOGVALET_SPACE": "your-space"
      }
    }
  }
}
```

**方法 3: フラグで直接指定**

```bash
logvalet mcp-stdio --api-key YOUR_API_KEY --space YOUR_SPACE
```

Claude Desktop の設定例:

```json
{
  "mcpServers": {
    "logvalet": {
      "command": "/absolute/path/to/logvalet",
      "args": ["mcp-stdio", "--api-key", "YOUR_API_KEY", "--space", "YOUR_SPACE"]
    }
  }
}
```

> **セキュリティ注意**
>
> stdio モードでは MCP クライアントが選択されたプロファイルのトークンをそのまま使って Backlog API を呼び出します。
> 次のいずれも守ってください:
>
> - 専用プロファイルを作成し、必要最小限の権限を持つ Backlog API キーを設定する
> - 信頼できる MCP クライアントのみ起動コマンドに登録する
> - チーム共有マシンでは利用しない
>
> **注意**: stdio モードでは `logvalet_issue_attachment_upload` の `file_paths` パラメータは使用できません。代わりに `file_content_base64` を使用してください。

### v0.16.0 の破壊的変更

v0.16.0 では MCP ツールのパラメータ命名・型を CLI と揃えるための破壊的変更が含まれます。旧パラメータ名を使っている MCP クライアントは呼び出しを更新してください。

| ID | 変更内容 | 対象ツール | 変更前 | 変更後 |
|----|---------|----------|-------|-------|
| C1 | ページネーションを `count` に統一 | `logvalet_issue_list`, `logvalet_issue_comment_list`, `logvalet_document_list`, `logvalet_shared_file_list` | `limit: 50` | `count: 50` |
| C2 | `user_id` を文字列型に統一（`"me"` または数値文字列） | `logvalet_watching_list` | `user_id: 12345`（数値） | `user_id: "12345"` / `user_id: "me"` |
| C3 | `project_id` → `project_key` | `logvalet_document_list` | `project_id: 9999`（数値） | `project_key: "PROJ"`（文字列） |
| C4 | CLI フラグ改名（後方互換 alias あり） | `logvalet star add` | `--pr-id <id>` | `--pull-request-id <id>`（旧 `--pr-id` は alias として維持） |

> **移行上の注意**: MCP クライアントはパラメータ名を JSON キーとして送信します。MCP フレームワークは未知のパラメータを暗黙的に無視するため、旧パラメータ名を送っても明示的エラーにはならず、単にパラメータが欠落した呼び出しとして扱われます。v0.16.0 へ上げる前に統合コードを更新してください。

### サポートされる動作モード

logvalet は CLI/stdio ではローカルの API key 認証を使用します。リモート HTTP MCP の認証は AgentCore Gateway に委譲され、`none` または `apikey` のいずれかです。

| # | クライアント | Backlog 認証 | Gateway 認証 | 状態 |
|---|------------|-------------|-------------|------|
| 1 | CLI | API key | — | ✅ サポート |
| 2 | MCP stdio | API key | — | ✅ サポート |
| 3 | MCP HTTP | Gateway passthrough | none | ✅ サポート |
| 4 | MCP HTTP | Gateway passthrough | apikey | ✅ サポート |

以下の例では各モードについて、(A) 環境変数のみ・(B) CLI 引数のみ（フラグに対応しない設定は必要最小限の環境変数）の 2 通りで記載しています。

#### Mode 1: CLI + API key

(A) 環境変数:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

logvalet issue get EXAMPLE-1
```

(B) CLI 引数:

```bash
logvalet --api-key=your-api-key-here --space=example-space issue get EXAMPLE-1
```

#### Mode 2: MCP stdio + API key

(A) 環境変数:

```bash
export LOGVALET_API_KEY=your-api-key-here
export LOGVALET_SPACE=example-space

logvalet mcp-stdio
```

(B) CLI 引数:

```bash
logvalet mcp-stdio --api-key=your-api-key-here --space=example-space
```

これらはローカル CLI/stdio 用の認証情報です。リモート HTTP は下記 Mode 4 で説明する AgentCore Gateway passthrough 契約を使用します。
`LOGVALET_API_KEY` / `--api-key` はこれらローカル CLI/stdio モード専用です。

#### Mode 3: MCP HTTP + AgentCore Gateway (none)

(A) 環境変数:

```bash
export LOGVALET_SPACE=example-space
export LOGVALET_MCP_AUTH_MODE=none
export LOGVALET_SPACE_STORE_TYPE=sqlite

logvalet mcp
```

(B) CLI 引数:

```bash
logvalet mcp \
  --space=example-space \
  --auth-mode=none
```

Mode 3・4 は HTTP モードであり、明示的な SpaceStore を使用します。CLI/stdio 用のローカル Backlog 認証情報は使用しません。Mode 3 は信頼済みの `none` Gateway モードを許可し、Mode 4 はさらに共有 Gateway API key を検証します。

#### Mode 4: MCP HTTP + AgentCore Gateway

(A) 環境変数:

```bash
export LOGVALET_SPACE=example-space

export LOGVALET_MCP_AUTH_MODE=apikey
export LOGVALET_MCP_API_KEY=shared-gateway-key
export LOGVALET_SPACE_STORE_TYPE=sqlite

logvalet mcp
```

(B) CLI 引数:

```bash
logvalet mcp \
  --space=example-space \
  --auth-mode=apikey \
  --auth-api-key=shared-gateway-key
```

リモート HTTP は AgentCore Gateway passthrough 経由で Backlog `Authorization: Bearer` 認証情報を受け取ります。

#### リモート HTTP MCP 契約

Gateway がエンドユーザー認証を担います。HTTP サーバーは `none` または `apikey` で設定し、後者では `X-Logvalet-Api-Key` を送信し、`X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` を送信する場合があります。Backlog 認証情報は Bearer passthrough です。HTTP モードは明示的な space store を必須とし `memory` を拒否します。ローカルトークンストレージは CLI/stdio 専用です（`sqlite` または `tokens.json`）。詳細は [gateway-request-contract.md](docs/specs/gateway-request-contract.md) を参照してください。

### 認証

リモート HTTP MCP は AgentCore Gateway 配下で `none` または `apikey` を使用します。Gateway 共有キーは `X-Logvalet-Api-Key`、identity メタデータは `X-Logvalet-Identity-Issuer` / `X-Logvalet-Identity-Subject` です。Backlog 認証情報は Bearer 認証情報として passthrough されます。これは直交する2軸です。`auth-mode` は MCP Gateway 認証を制御し、Backlog Bearer passthrough は HTTP で常時有効な固定動作であり、`auth-mode` の選択肢ではありません。[MCP サーバー](#mcp-サーバー)も参照してください。

### Backlog 認証情報

旧来のリモートブラウザコールバックとユーザーごとの OAuth 手順は廃止されました。リモート HTTP は AgentCore Gateway から Backlog Bearer passthrough を受け取り、[docs/specs/gateway-request-contract.md](docs/specs/gateway-request-contract.md) に記載のリクエスト契約を使用します。

リモート HTTP では `none` または `apikey` と明示的な space store を設定してください。`memory` は無効です。CLI と `mcp-stdio` はローカル認証情報のみを使用します（`sqlite` または `tokens.json`）。デプロイ詳細は [AgentCore デプロイガイド](docs/agentcore-deployment.md) を参照してください。

### Space store 環境変数

HTTP モードでは `memory` を使用できず、`LOGVALET_SPACE_STORE_TYPE` の明示指定が必要です。`sqlite` と `dynamodb` の両方に対応し、ローカル利用時の既定値は `sqlite` です。

| 変数 | 既定値 | 説明 |
|---|---|---|
| `LOGVALET_SPACE_STORE_TYPE` | `sqlite` | `sqlite` または `dynamodb`。HTTP では明示指定必須で `memory` は拒否 |
| `LOGVALET_SPACE_STORE_PATH` | プラットフォーム既定 | SQLite データベースのパス |
| `LOGVALET_SPACE_STORE_DYNAMODB_TABLE` | — | `dynamodb` 使用時のテーブル名 |
| `LOGVALET_SPACE_STORE_DYNAMODB_REGION` | — | `dynamodb` 使用時の AWS リージョン |

```bash
# ローカル CLI / stdio
logvalet configure --init-profile default --init-space YOUR_SPACE --init-api-key YOUR_API_KEY
logvalet mcp-stdio --profile default

# AgentCore Gateway 配下のリモート HTTP
export LOGVALET_MCP_AUTH_MODE=apikey
export LOGVALET_MCP_API_KEY=shared-gateway-key
export LOGVALET_SPACE_STORE_TYPE=sqlite
logvalet mcp
```

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

## Claude Code スキル

logvalet の Claude Code スキル（PM ワークフロー・プロジェクト健全性・リスク分析・アクティビティ分析・課題トリアージ・定期ダイジェストをカバーする 14 スキル）は [`youyo/claude-plugins`](https://github.com/youyo/claude-plugins) マーケットプレース経由で配布しています。

### インストール

```bash
# Claude Code 内で
/plugin marketplace add youyo/claude-plugins
/plugin install logvalet@youyo
```

CLI バイナリは別途インストールが必要です（上記「インストール」セクション参照）。

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

### 旧インストールからの移行

`npx skills add youyo/logvalet` や `/plugin install logvalet@<旧>` で導入していた場合は、先に旧版をアンインストールしてから上記の `claude-plugins` マーケットプレース経由で再インストールしてください。スキルは本リポジトリから直接配布しなくなりました — 本リポジトリは CLI バイナリと MCP サーバー実装に集中します。

## ライセンス

MIT
