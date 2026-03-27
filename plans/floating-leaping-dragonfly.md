# Plan: Backlog 操作スキル群の新規作成

## Context

logvalet CLI は Backlog の読み書き機能を豊富に持つが、現状スキルは汎用的な `logvalet` スキル1つのみ。
よく使うオペレーションを専用スキルとして登録し、ワンコマンドで即実行できるようにする。

## ネームスペース

`backlog:*` の **2階層** を採用（`devflow:plan`, `slack:standup` と同じパターン）。
3階層（`backlog:issue:create`）は Claude Code の標準パターンから外れるため避ける。

## ディレクトリ構成

```
skills/backlog/
  issue-create/SKILL.md   → /backlog:issue-create
  my-week/SKILL.md        → /backlog:my-week
  my-next/SKILL.md        → /backlog:my-next
  report/SKILL.md         → /backlog:report
```

`settings.json` の変更は不要（`skills/` ディレクトリは自動検出）。

---

## Skill 1: `backlog:issue-create`

**目的:** Backlog 課題をインタラクティブに作成。不足情報は AskUserQuestion で補完。

**トリガー:** 「課題作成」「issue作成」「チケット作成」「タスク登録」「backlogに課題を作って」「バックログに登録」「新しいissue」「create issue」「file a ticket」「make a task」「backlog.com に課題追加」「*.backlog.com でタスク作成」

**ワークフロー:**

1. `project-key` 未指定 → `lv project list -f md` でプロジェクト一覧を提示、選択を促す
2. `summary` 未指定 → AskUserQuestion で質問
3. オプション項目（description, assignee, priority, due-date, issue-type, category, milestone, start-date）をまとめて1回の質問で聞く（スキップ可）
4. メタデータ確認が必要な場合 → `lv meta category PROJ`, `lv meta version PROJ` 等を実行
5. **必ず `--dry-run` で先にプレビュー**を表示
6. ユーザー確認後、`--dry-run` なしで実行
7. 作成された課題キーと URL を表示

**コマンドテンプレート:**
```bash
lv issue create --project-key PROJ --summary "..." \
  [--description "..."] [--assignee USER_ID] [--priority normal] \
  [--due-date YYYY-MM-DD] [--issue-type "タスク"] \
  [--category "カテゴリ名"] [--milestone "マイルストーン名"] \
  --dry-run
```

---

## Skill 2: `backlog:my-week`

**目的:** 今週自分がやるべき課題一覧（期限切れ含む、プロジェクト横断）。

**トリガー:** 「今週のタスク」「my week」「今週やること」「今週の予定」「backlogの今週分」「バックログで今週」「weekly tasks」「this week's issues」「今週やるべきこと」「backlog.com の今週のタスク」「今週何やる」「今週の課題」

**ワークフロー:**

1. 2コマンドを並列実行:
   ```bash
   lv issue list --assignee me --status not-closed --due-date overdue --sort dueDate --order asc -f md
   lv issue list --assignee me --status not-closed --due-date this-week --sort dueDate --order asc -f md
   ```
2. 結果を「⚠ 期限切れ」「📅 今週」の2セクションに整理
3. 課題キーで重複排除
4. サマリー行: 「期限切れ X 件、今週 Y 件」

**ユーザー操作不要** — 表示のみ。

---

## Skill 3: `backlog:my-next`

**目的:** 直近数日の課題一覧（週を跨ぐ、期限切れ含む、プロジェクト横断）。

**トリガー:** 「直近のタスク」「upcoming tasks」「次にやること」「次やること」「明日以降のタスク」「backlogの直近」「バックログで直近」「coming up」「what's next」「近々のタスク」「backlog.com の予定」「今日明日のタスク」「数日以内の課題」

**my-week との違い:** 週に縛られず、今日から4営業日先までを対象。金曜なら翌週火曜まで。

**ワークフロー:**

1. 日付計算（曜日→カレンダー日数のオフセット）:

   | 曜日 | +N日 | 終了日の例（3/27木起点） |
   |------|------|----------------------|
   | 月   | +4   | 金                    |
   | 火   | +6   | 翌月                  |
   | 水   | +6   | 翌火                  |
   | 木   | +6   | 翌水                  |
   | 金   | +6   | 翌木                  |
   | 土   | +5   | 翌木                  |
   | 日   | +4   | 翌木                  |

   ```bash
   DOW=$(date +%u)
   case $DOW in
     1) OFFSET=4 ;; 2|3|4|5) OFFSET=6 ;; 6) OFFSET=5 ;; 7) OFFSET=4 ;;
   esac
   END_DATE=$(date -v+${OFFSET}d +%Y-%m-%d)  # macOS
   TODAY=$(date +%Y-%m-%d)
   ```

2. 2コマンドを並列実行:
   ```bash
   lv issue list --assignee me --status not-closed --due-date overdue --sort dueDate --order asc -f md
   lv issue list --assignee me --status not-closed --due-date ${TODAY}:${END_DATE} --sort dueDate --order asc -f md
   ```
3. 「⚠ 期限切れ」「📅 直近（〜END_DATE）」の2セクションに整理
4. 課題キーで重複排除

---

## Skill 4: `backlog:report`

**目的:** 対象ユーザー/チーム/期間の活動レポートを指定フォーマットで生成。

**トリガー:** 「レポート作成」「月次レポート」「活動レポート」「チームレポート」「backlogレポート」「バックログのレポート」「activity report」「monthly report」「team report」「backlog.com のレポート」「先月のレポート」「今月のレポート」「メンバーの活動」「チームの活動まとめ」「KPTレポート」「ふりかえりレポート」

**パラメータ収集:**
- 対象メンバー（ユーザー名/ID、複数可）またはチーム名
- 期間: `this-month` / `last-month` / 日付範囲 `YYYY-MM-DD:YYYY-MM-DD`
- プロジェクトフィルタ（オプション）

未指定項目は AskUserQuestion で補完。

**データ取得:**

各メンバーについて:
```bash
lv user activity USER_ID --since START --until END --limit 1000
```

アクティビティ type マッピング（整数→カテゴリ）:
| type ID | レポートカテゴリ |
|---------|----------------|
| 1       | 課題作成        |
| 2, 14   | 課題更新        |
| 3       | コメント        |
| その他   | その他          |

**出力テンプレート:**

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
| **合計** | **X**  | **Y**    | **Z**    | **W**  | **T**|

### 📈 日別アクティビティ推移

- MM月第1週 (DD-DD): N件
- MM月第2週 (DD-DD): N件
- ...

### 🎯 主要なアクティビティ

#### 📊 プロジェクト別アクティビティ数
1. **プロジェクト名** - N件
2. ...

#### 🔥 活発だった課題
1. **PROJ-123: 課題タイトル** (N件のアクティビティ)
2. ...

## ふりかえり

### Fact (事実 / やったこと)

- プロジェクト名1 (PROJ1_KEY)
  - PROJ1_KEY-101 課題名
  - PROJ1_KEY-102 課題名

### Keep (よかったこと、継続すること）
- 記載する

### Problem（問題点、うまくいかなかったこと）
- 記載する

### Try (改善点、Next Action）
- 記載する
```

**集計ロジック（SKILL.md に記載）:**
1. `user activity` の JSON 配列から type でグルーピング → 統計テーブル
2. `created` の日付で週ごとに集約 → 推移セクション
3. `content.key` のプロジェクトプレフィックスで集約 → プロジェクト別
4. `content.key` ごとにアクティビティ数をカウント → 活発な課題
5. Fact セクション: 各メンバーが関わった課題をプロジェクト別にリスト化
6. Keep/Problem/Try: プレースホルダーとして出力

---

## 実装順序

| # | スキル | 複雑度 | 理由 |
|---|--------|--------|------|
| 1 | `backlog:my-week` | 低 | 固定コマンド2本、ユーザー操作なし |
| 2 | `backlog:my-next` | 低〜中 | my-week + 日付計算 |
| 3 | `backlog:issue-create` | 中 | インタラクティブフロー |
| 4 | `backlog:report` | 高 | 複数データソース集計 + テンプレート |

## 対象ファイル

- `skills/backlog/my-week/SKILL.md` (新規)
- `skills/backlog/my-next/SKILL.md` (新規)
- `skills/backlog/issue-create/SKILL.md` (新規)
- `skills/backlog/report/SKILL.md` (新規)
- `skills/logvalet/SKILL.md` (変更なし、参照のみ)

## 検証方法

各スキルの検証:
1. `/backlog:my-week` を実行 → 期限切れ + 今週の課題が正しく表示されるか
2. `/backlog:my-next` を実行 → 日付計算が正しく、週跨ぎが機能するか
3. `/backlog:issue-create` を実行 → AskUserQuestion で不足情報を聞き、dry-run → 実行の流れが動くか
4. `/backlog:report` を実行 → テンプレート通りのフォーマットで出力されるか

全スキルが `skills/` ディレクトリの自動検出で Claude Code に認識されることを確認。
