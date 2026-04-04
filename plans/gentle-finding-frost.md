# Plan: ロードマップ v2 に Watch 統合マイルストーンを追加 + スキル更新

## Context

ユーザーの要件:
> 「watchしている課題も対応すべき、もしくは注意すべき課題になる。自分が担当じゃなくても。」

Backlog API には watching エンドポイントが7つ存在するが、logvalet には未実装。
前回の計画で M17（Watch CLI コマンド）をロードマップに追加済み。

**今回の追加要件**: M17 の Watch API 実装に加え、既存スキルを「ウォッチ課題も考慮するように」更新する。
これは「自分に関係する課題 ＝ 担当課題 + ウォッチ課題」というメンタルモデルの拡張。

**核心となるインサイト**: ウォッチしている課題は「自分が担当ではないが、進捗や状態が自分の仕事・判断に影響する課題」。
レビュー待ち、依存先の課題、チーム横断で関心のある課題など。これを無視すると、担当課題だけを見て「自分のやることは把握できている」と錯覚するリスクがある。

---

## 変更対象

### 1. ロードマップ更新
- `plans/logvalet-roadmap-v2.md` — M17 の内容を拡充し、スキル統合タスクを追加

### 2. スキル SKILL.md 更新（6ファイル）

| スキル | 変更箇所 | 影響度 |
|--------|---------|--------|
| `skills/my-week/SKILL.md` | description + instructions（ウォッチ課題セクション追加） | **大** |
| `skills/my-next/SKILL.md` | description + instructions（同上） | **大** |
| `skills/health/SKILL.md` | instructions（ウォッチ課題の停滞をシグナルに追加） | 中 |
| `skills/risk/SKILL.md` | instructions（ウォッチ課題をリスク材料に追加） | 中 |
| `skills/triage/SKILL.md` | instructions（ウォッチ情報を担当者提案のシグナルに追加） | 小 |
| `skills/context/SKILL.md` | instructions（ウォッチ情報をコンテキスト出力に追加） | 小 |

---

## 詳細設計

### M17 拡充: Watch コマンド + スキル統合

**前提**: M17 の Watch CLI コマンド（`lv watching list/add/delete/...`）が実装されている状態でスキルが活用する。
→ ロードマップの M17 を「Watch CLI + スキル統合」として拡充する。

### スキル変更詳細

#### A. `my-week` / `my-next`（最重要 — description + instructions 両方変更）

##### description 変更

**現状の問題**: description が "assigned to me" のみに言及しており、ウォッチ課題を求めるプロンプトではトリガーされない。

skill-creator の "pushy" description 原則に従い、ウォッチ課題に関するトリガーフレーズを追加する。

**my-week description 変更案:**
```yaml
description: >
  Show this week's Backlog issues assigned to me AND issues I'm watching
  across all projects, including overdue items from previous weeks — the weekly planning view.
  Watched issues matter because they affect your work even when you're not the assignee:
  dependency blockers, reviews you're waiting on, cross-team items you need to track.
  TRIGGER when: user says "今週のタスク", "my week", "今週やること", "今週の予定",
  "backlogの今週分", "weekly tasks", "this week's issues",
  "今週やるべきこと", "今週何やる", "今週の課題",
  "今週のバックログ", "weekly plan", "week overview",
  "今週の計画", "月曜から金曜のタスク", "今週のスケジュール",
  "weekly planning", "what's on my plate this week", "今週の見通し",
  "ウォッチしてる課題", "watching issues", "気にしてる課題",
  "注視してる課題", "watched tasks this week".
  DO NOT TRIGGER when: user wants only the next 1-2 days (use my-next)
  or wants team-wide workload (use health).
```

**my-next description 変更案:**
```yaml
description: >
  Show near-term Backlog issues assigned to me AND issues I'm watching:
  next few business days across all projects, including overdue items —
  helps answer "what should I work on next?" and "what should I keep an eye on?"
  Watched issues are included because they represent work you care about even
  without being the assignee — blocked dependencies, pending reviews, cross-team items.
  TRIGGER when: user says "直近のタスク", "upcoming tasks", "次にやること", "次やること",
  "明日以降のタスク", "backlogの直近", "coming up", "what's next",
  "明日何やる", "次の予定", "今日と明日のタスク", "直近の課題",
  "next tasks", "upcoming issues", "what should I do next",
  "明日の予定", "次のアクション", "今日やること", "today's tasks",
  "直近やるべきこと", "tomorrow's tasks", "近日中のタスク",
  "ウォッチしてるやつどうなった", "watched issues status".
  DO NOT TRIGGER when: user wants a full week overview (use my-week)
  or wants a project-wide task list (use logvalet CLI directly).
```

##### instructions 変更

**Why（なぜウォッチ課題を含めるか — スキル内で説明する）:**
> ウォッチしている課題は「自分が担当ではないが、進捗や状態変化が自分の仕事に影響する課題」。
> 例: レビュー待ちの課題、自分のタスクが依存している他チームの課題、将来引き受ける可能性がある課題。
> 担当課題だけでは「自分の仕事の全体像」が見えない。ウォッチ課題を並べて表示することで、
> 「やるべきこと」と「気にかけるべきこと」の両方を一覧できる。

**Workflow 変更:**

Step 1 に並列コマンドを1つ追加:
```bash
lv watching list --user-id me -f md
```

Step 2 の出力フォーマットを3セクションに拡張:
```
## ⚠ 期限切れ (N件)
<overdue assigned issues>

## 📅 今週 (N件)  ← my-next では「直近 〜 END_DATE」
<this week assigned issues>

## 👁 ウォッチ中 (N件)
<watched issues — 自分担当ではないが注視している課題>
- 担当課題と重複するものは除外（担当セクションで既に表示済み）
- ステータスが not-closed のもののみ
- 期限切れのウォッチ課題は ⚠ マーク付与
- 最終更新日が7日以上前のものは「停滞」シグナル付与

---
担当: X件 / ウォッチ: Y件 / 合計: Z件
```

**除外ロジック**: 担当課題の issue key セットとウォッチ課題を比較し、重複を除外。
ウォッチ課題には担当者名を表示する（「誰がやっているか」が分かるように）。

#### B. `health`（instructions 変更）

**Why**: プロジェクト健全性を評価する際、ウォッチされている課題が停滞していることは
「多くの人が気にしているのに進んでいない」という強いシグナル。

**変更内容**: Step 4 のサマリー出力に「ウォッチ数の多い停滞課題」セクションを追加。
`lv watching list` の結果と `lv issue stale` を突合し、ウォッチされている停滞課題を抽出。

#### C. `risk`（instructions 変更）

**Why**: 自分がウォッチしている課題にリスクがある場合、そのリスクは自分の仕事にも波及する。
プロジェクト全体のリスク評価に加え、「あなたがウォッチしている課題のリスク」を個別に提示する。

**変更内容**: Step 3 に `lv watching list --user-id me -f json` を追加。
Step 5 のレポートに「あなたのウォッチ課題に関連するリスク」セクションを追加。

#### D. `triage`（instructions 変更）

**Why**: 課題をウォッチしている人は、その課題に関心や知識がある可能性が高い。
担当者を提案する際の参考情報として活用できる。

**変更内容**: Step 3 の分析に「この課題のウォッチ情報」を補助シグナルとして追加。
ただし Backlog API の制約で「特定課題のウォッチャー一覧」を直接取得できない可能性があるため、
利用可能なデータに応じて柔軟に対応する旨を注記。

#### E. `context`（instructions 変更）

**Why**: 課題のコンテキストを把握する際に「誰がこの課題を注視しているか」は
ステークホルダーの特定やエスカレーション先の判断に役立つ。

**変更内容**: Step 4 の出力に「ウォッチ情報」フィールドを追加（取得可能な範囲で）。

---

## skill-creator ベストプラクティスの適用

### 1. Description の最適化（トリガリング精度）

skill-creator は description を "pushy" に書くことを推奨。
「ウォッチ」「watching」「気にしてる課題」等のトリガーフレーズを追加し、
ユーザーがウォッチ関連の質問をした時に正しいスキルがトリガーされるようにする。

**変更するスキルの description:**
- `my-week`: ウォッチ課題トリガーフレーズ追加
- `my-next`: 同上

**変更しないスキルの description:**
- `health`, `risk`, `triage`, `context`: ウォッチはこれらのスキルの主要トリガーではなく、
  内部的に活用する補助データ。description にウォッチを追加するとノイズになる。

### 2. Instructions の "Why" 説明

skill-creator は「ALWAYS/NEVER の ALL CAPS」よりも「なぜそうするか」の説明を推奨。
各スキルのウォッチ関連セクションに、なぜウォッチ課題を考慮するかの理由を記載する。

### 3. Progressive Disclosure

ウォッチ関連の instructions は Optional セクションとして追加し、
M17（Watch CLI）が実装されるまでは graceful degradation する旨を明記。
スキル本体が500行を超えないよう注意。

### 4. テストケース設計

スキル更新後、以下のテストプロンプトで検証:

| # | テストプロンプト | 期待動作 |
|---|----------------|---------|
| 1 | 「今週のタスク見せて」 | my-week がトリガー → 担当課題 + ウォッチ課題の2セクション表示 |
| 2 | 「ウォッチしてる課題どうなった？」 | my-week or my-next がトリガー → ウォッチ課題セクション表示 |
| 3 | 「次にやることは？」 | my-next がトリガー → 担当 + ウォッチの直近課題表示 |
| 4 | 「プロジェクトの健康状態を見たい」 | health がトリガー → ウォッチされている停滞課題もシグナルに含む |
| 5 | 「このプロジェクトのリスクは？」 | risk がトリガー → ウォッチ課題関連リスクセクション含む |

---

## 変更手順

1. `plans/logvalet-roadmap-v2.md` の M17 を拡充（Watch CLI + スキル統合タスクを追加）
2. `skills/my-week/SKILL.md` — description + instructions 更新
3. `skills/my-next/SKILL.md` — description + instructions 更新
4. `skills/health/SKILL.md` — instructions にウォッチシグナル追加
5. `skills/risk/SKILL.md` — instructions にウォッチ材料追加
6. `skills/triage/SKILL.md` — instructions にウォッチ情報追加
7. `skills/context/SKILL.md` — instructions にウォッチ情報追加

---

## 検証

- 各 SKILL.md の Markdown が正しくレンダリングされること
- description がウォッチ関連トリガーフレーズを含むこと（my-week, my-next）
- instructions が「なぜウォッチ課題を含めるか」を説明していること
- ウォッチセクションが Optional として記載され、M17 未実装時に graceful に動作すること
- スキル本体が500行を超えないこと
- テストプロンプトで期待通りのスキルがトリガーされること
- ロードマップのチェックリストが一貫した粒度であること

---

## 注意点

- **M17 の Watch CLI が未実装の間**: スキルの instructions に「Watch CLI 実装後に有効化」の注記を入れ、Watch セクションは Optional として記載
- **API 制約**: Backlog Watching API は `GET /api/v2/users/:userId/watchings` でユーザーのウォッチ一覧を取得できるが、「特定の課題をウォッチしているユーザー一覧」を直接取得する API は存在しない可能性 → context/triage での活用方法は API 制約に応じて調整
- **description の肥大化防止**: ウォッチトリガーを追加するのは my-week と my-next のみ。他のスキルは instructions 内で対応し description はそのまま
