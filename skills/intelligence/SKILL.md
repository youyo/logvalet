---
name: logvalet:intelligence
description: >
  Analyze Backlog activity patterns: detect anomalies, concentration bias, peak hours,
  and unusual trends — the LLM interprets activity statistics and project health data
  to surface risks that numbers alone don't reveal.
  TRIGGER when: user says "アクティビティ分析", "activity intelligence", "異常検知",
  "偏り", "アクティビティの傾向", "activity patterns", "最近の動きに異常は",
  "チームの活動パターン", "作業の偏り", "ピーク時間帯", "activity anomaly",
  "特定の人に集中してない？", "活動量の分析", "誰が何をしてるか",
  "concentration analysis", "activity trends", "稼働分析".
  DO NOT TRIGGER when: user wants a simple activity list (use logvalet CLI directly),
  wants a periodic digest (use digest-periodic), or wants risk recommendations (use risk).
  Workflow: Combines activity stats + project health for holistic analysis.
---

# logvalet-intelligence

`lv activity stats` と `lv project health` を材料に、LLM がアクティビティパターンの偏り・異常を解釈してリスク評価を生成する。

> For full logvalet CLI documentation, see the `logvalet` skill.

---

## When to use this skill

Use `logvalet-intelligence` when you need to:

- detect unusual spikes or drops in project activity
- identify team members with disproportionate contribution (over- or under-active)
- surface hidden risks that raw stats do not make explicit
- evaluate team health from an activity perspective
- support retrospectives with data-driven insights

---

## Workflow

### Step 1: Identify project

If the user provides a project key, use it directly.

If not provided, list available projects:

```bash
lv project list -f md
```

Then ask the user to select one.

### Step 2: Determine scope and period

Ask in a single question (if not already specified):

- `--scope`: 集計スコープ (`project` / `user` / `space`, デフォルト: `project`)
- `--since` / `--until`: 集計期間（ISO 8601 形式）
- `--top-n`: 上位表示数（デフォルト: 5）

If the user wants a quick overview, use defaults.

### Step 3: Fetch materials in parallel

Run both commands **in parallel**:

```bash
lv activity stats --scope project -k PROJECT_KEY --since YYYY-MM-DDT00:00:00Z --until YYYY-MM-DDT23:59:59Z --top-n 10 -f json
```

```bash
lv project health PROJECT_KEY -f json
```

The activity stats output includes:

- `total_count`: 期間内総アクティビティ数
- `by_type`: アクティビティタイプ別内訳
- `by_actor`: アクター（ユーザー）別内訳
- `by_hour`: 時間帯別分布
- `by_day_of_week`: 曜日別分布
- `top_active_actors`: 最も活発なアクター上位 N 件
- `top_active_types`: 最も多いタイプ上位 N 件

### Step 4: Analyze patterns and detect anomalies

Using the stats and health data, reason about:

**Activity volume:**
- Is total activity count unusually high or low for the period?
- Are there unexpected spikes or gaps?

**Actor distribution:**
- Is activity heavily concentrated on a few members? (Gini-like inequality)
- Are any team members completely absent from recent activity?
- Is there an unexpectedly high single-actor share (>60%)?

**Type distribution:**
- Is there an unusual imbalance between issue creation vs. resolution?
- Are comment activities disproportionately high (discussion-heavy, no resolution)?
- Are there unexpected activity types?

**Cross-signal correlation:**
- Compare `activity stats` with `project health` blockers/stale issues
- If stale issues are high but activity is also high → activity may not be on the right issues
- If activity is low but no blockers → team may be in a quiet phase (confirm intentionality)

### Step 5: Present intelligence report

```
## アクティビティインテリジェンスレポート — PROJECT_KEY

> 分析期間: FROM〜TO / 総アクティビティ: N件 / 生成日時: YYYY-MM-DD

---

### アクティビティ概要

| 指標 | 値 |
|------|-----|
| 総アクティビティ数 | N |
| アクター数 | N |
| 最も多いタイプ | TypeName (N%) |
| 最も活発なメンバー | UserName (N件, N%) |

---

### 異常・偏り検出

#### 偏り指標
- **アクター集中度:** <低/中/高> — 上位1名が全体の N% を占める
- **タイプバランス:** <問題なし / 課題作成偏重 / コメント偏重 / ...>

#### 検出された異常
1. **<異常の名称>**
   - 観測値: <具体的な数値>
   - 解釈: <何が起きている可能性があるか>
   - リスク: <低/中/高>

2. ...（なければ「異常なし」）

---

### ヘルス相関分析

<activity stats と project health の相関から導かれる洞察>

---

### 推奨アクション

1. <最優先のアクション>
2. <次に重要なアクション>
3. ...（最大5件）

---

**サマリー:**
- 全体リスクレベル: <低/中/高>
- 主な懸念事項: <1-2行>
```

### Step 6: No writes

This is a read-only skill. No issue updates are performed.

If the user wants to act on findings, switch to the `logvalet` skill for updates, or `logvalet-triage` for issue triage.

---

## Notes

- `activity stats` と `project health` は**並列実行**で取得する（API 呼び出し最小化）
- アクター集中度の高低目安: 上位1名が全体の >60% → 高、30-60% → 中、<30% → 低
- `--top-n 10` を推奨（デフォルト5では偏りが見えにくい場合がある）
- 期間を指定しない場合、Backlog API の直近アクティビティ（最大100件）が返る

---

## Anti-patterns

- Do not auto-update issues based on intelligence findings — always present findings first
- Do not interpret low activity as a problem without context (planned quiet phase, holidays, etc.)
- Do not skip the project identification step — never guess the project key
- Do not run both commands sequentially when they can run in parallel
