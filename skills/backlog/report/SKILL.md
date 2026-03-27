---
name: backlog:report
description: >
  Generate a Backlog activity report for specified users, teams, or projects over a time period.
  TRIGGER when: user says "レポート作成", "月次レポート", "活動レポート", "チームレポート",
  "backlogレポート", "バックログのレポート", "activity report", "monthly report",
  "team report", "backlog.com のレポート", "先月のレポート", "今月のレポート",
  "メンバーの活動", "チームの活動まとめ", "KPTレポート", "ふりかえりレポート",
  "backlog レポート", "バックログ レポート", "活動報告", "月報", "週報".
---

# backlog:report

対象ユーザー/チーム/期間の Backlog 活動レポートを指定フォーマットで生成する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## Workflow

### Step 1: Gather parameters

Determine from the user's request:

| パラメータ | 必須 | 例 |
|-----------|------|---|
| 対象メンバー | ○ | ユーザー名、ユーザーID、またはチーム名 |
| 期間 | ○ | `this-month`, `last-month`, `2026-02-01:2026-02-28` |
| プロジェクトフィルタ | △ | プロジェクトキー（省略時は全プロジェクト） |

If any required parameter is missing, ask using AskUserQuestion in a single prompt.

### Step 2: Resolve members

If a **team name** is given, get member list:

```bash
lv team list -f json
```

Parse the JSON to extract member IDs for the specified team.

If **user names** are given, resolve to IDs:

```bash
lv user list -f json
```

### Step 3: Resolve period dates

| 指定 | since | until |
|------|-------|-------|
| `this-month` | 今月1日 (YYYY-MM-01) | 今日 (YYYY-MM-DD) |
| `last-month` | 先月1日 | 先月末日 |
| `YYYY-MM-DD:YYYY-MM-DD` | そのまま使用 | そのまま使用 |

### Step 4: Fetch activity data

For **each member**, fetch activities:

```bash
lv user activity USER_ID --since YYYY-MM-DD --until YYYY-MM-DD --limit 1000 -f json
```

Run multiple members **in parallel** (multiple Bash tool calls in one message).

### Step 5: Aggregate data

The `user activity` command returns a JSON array of raw Backlog activities. Each activity has:

```json
{
  "id": 914382588,
  "type": 3,
  "created": "2026-03-23T07:24:56Z",
  "createdUser": {"id": 1537084, "name": "User Name"},
  "content": {
    "id": 141884975,
    "key_id": 1158,
    "summary": "Issue title"
  }
}
```

**Activity type mapping:**

| type ID | レポートカテゴリ |
|---------|----------------|
| 1 | 課題作成 |
| 2, 14 | 課題更新 |
| 3 | コメント |
| その他 (4-13, 15-21) | その他 |

**Aggregation steps:**

1. **統計テーブル:** type でグルーピング → メンバーごとにカウント
2. **週別推移:** `created` の日付を週ごとに集約
3. **プロジェクト別:** `content.key_id` からプロジェクトキーのプレフィックス部分で集約（`content` に `key` フィールドがあればそのプレフィックスを使用）
4. **活発な課題:** issue key ごとにアクティビティ数をカウント、上位をランキング
5. **Fact セクション:** 各メンバーが関わった課題をプロジェクト別にリスト化

### Step 6: Generate report

**必ず以下のテンプレートに従って出力する:**

```markdown
# YYYY年MM月 Backlogレポート

> 生成日時: YYYY-MM-DD HH:MM:SS JST

## 📊 サマリー

### 対象期間
YYYY年MM月DD日 〜 YYYY年MM月DD日

### 対象メンバー
- **メンバー名1** (*userKey1)
- **メンバー名2** (*userKey2)

### アクティビティ統計

| メンバー | 課題作成 | コメント | 課題更新 | その他 | 合計 |
|---------|---------|----------|----------|--------|------|
| Name1   | X       | Y        | Z        | W      | T    |
| Name2   | X       | Y        | Z        | W      | T    |
| **合計** | **X**  | **Y**    | **Z**    | **W**  | **T**|

### 📈 日別アクティビティ推移

- MM月第1週 (DD-DD): N件
- MM月第2週 (DD-DD): N件
- MM月第3週 (DD-DD): N件
- MM月第4週 (DD-DD): N件

### 🎯 主要なアクティビティ

#### 📊 プロジェクト別アクティビティ数
1. **プロジェクト名** - N件
2. **プロジェクト名** - N件

#### 🔥 活発だった課題
1. **PROJ-123: 課題タイトル** (N件のアクティビティ)
2. **PROJ-456: 課題タイトル** (N件のアクティビティ)


## ふりかえり

### Fact (事実 / やったこと)

- プロジェクト名1 (PROJ1_KEY)
  - PROJ1_KEY-101 課題名
  - PROJ1_KEY-102 課題名

- プロジェクト名2 (PROJ2_KEY)
  - PROJ2_KEY-201 課題名

### Keep (よかったこと、継続すること）

- 記載する

### Problem（問題点、うまくいかなかったこと）

- 記載する

### Try (改善点、Next Action）

- 記載する
```

---

## Important rules

- **テンプレート厳守** — 上記のフォーマットに必ず従う。セクションの順序、見出し、テーブル構造を変えない
- **Fact セクションは自動生成** — 各メンバーが関わった課題をプロジェクト別にリスト化
- **Keep/Problem/Try はプレースホルダー** — 「記載する」と表示。ユーザーが後で記入する
- **活動がゼロのメンバーも表示** — 統計テーブルに全メンバーを含め、0件でも行を表示
- **並列実行推奨** — 複数メンバーの activity 取得は並列 Bash で効率化
- **プロジェクト名の解決** — issue key のプレフィックス（例: `PROJ` from `PROJ-123`）がわかる場合、`lv project get PROJ -f json` でプロジェクト名を取得

## Anti-patterns

- レポートフォーマットを勝手にアレンジしない
- Keep/Problem/Try を AI が推測で埋めない（プレースホルダーのまま出力する）
- メンバーごとに1回ずつ質問しない — パラメータは一括で聞く
