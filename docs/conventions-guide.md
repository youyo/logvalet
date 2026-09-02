# 運用規約（conventions）導入ガイド

logvalet の conventions は、Linear の「曖昧さを許さない構造」を Backlog の語彙に翻訳した
運用規約を、宣言的なファイルから Backlog プロジェクトへ冪等に適用し、その規約を
AI に読ませるための機能です。

ツールの機能ではなく、**制約と、その制約を自分たちの言葉で埋める作業**に価値があります。
`conventions.yaml` は値の設定ファイルに見えて、実際に書いているのは
組織の優先順位に対する態度です。

---

## 何が手に入るか

1. **Project に紐づかない Issue が「溜まり」として可視化される**
   `logvalet project health` が案件に属さない課題を `ambiguities` に挙げます。
2. **Lead の空欄がむき出しになる**
   担当者を決めていない案件は、apply が親課題を作らずスキップし、理由を表示します。
3. **優先度が組織の言葉で定義される**
   「低」が何を意味するのかを規約に書き、クローズ候補の検知に使います。
4. **規約がそのまま AI への指示書になる**
   `logvalet_project_conventions` MCP ツールが規約と用語集を返すので、
   AI エージェントは組織のルールを前提に動けます。

---

## 用語

| 用語 | 意味 | Backlog 上の実体 |
|---|---|---|
| conventions | 組織の運用規約。Linear の制約を Backlog の語彙に翻訳し、各項目の意味を自分たちの言葉で書いたもの | 規約課題（入力は `conventions.yaml`） |
| Initiative | 数か月規模の重点テーマ。並び順が優先度。案件は必ずいずれかに属する | なし（conventions 内のリスト） |
| 案件（engagement） | 数週間規模の取り組み。Linear の Project に相当 | カテゴリ + 種別「案件」の親課題 |
| Lead | 案件の責任者。1 人だけ | 案件親課題の担当者 |
| 案件親課題 | 案件のヘッダーとなる課題。Lead・期間・状態・Context & Goals を持つ | 種別「案件」の課題 |
| 規約課題 | 規約の正本。説明欄に運用ガイドと YAML を持ち、変更履歴とコメントで規約の議論を残す | 種別「規約」の課題（プロジェクトに 1 件） |
| 曖昧さ（ambiguity） | 規約に照らして決まっていないこと。案件不明の課題、Lead 不在の案件など | `project health` の `ambiguities` |

同じ用語集は `logvalet project conventions show --project KEY` と
MCP ツール `logvalet_project_conventions` も返します。人と AI が同じ定義を見るためです。

---

## Linear との対応付け

| Linear | logvalet / Backlog | 備考 |
|---|---|---|
| Team | なし | 横断視点は既存の fan-out / multi-space で補う |
| Project | 案件 = カテゴリ + 種別「案件」の親課題 | 親課題が Lead・期間・状態・説明テンプレートを持つ |
| Issue | 課題 | 案件カテゴリをちょうど 1 つ持ち、案件親課題の子課題にする |
| Initiative | `conventions.yaml` 内の順序付きリスト | Backlog 側に横断プロジェクトは作らない |
| Priority (4 段階) | 高・中・低（固定） | 段階は足さず、3 段階の意味を組織の言葉で定義する |

背景と却下案は [ADR 0005](adr/0005-conventions-source-of-truth-in-rule-issue.md) と
[ADR 0006](adr/0006-linear-project-mapping-to-category-and-parent-issue.md) を参照してください。

---

## 導入手順

### 新規プロジェクトに導入する

```bash
# 1. スケルトンを生成する
logvalet project conventions init --out conventions.yaml

# 2. 自分たちの言葉で埋める（後述「埋めるべき項目」）
$EDITOR conventions.yaml

# 3. 検証する
logvalet project conventions validate --file conventions.yaml

# 4. 差分を確認する（書き込みは一切しない）
logvalet project apply --file conventions.yaml --dry-run

# 5. 適用する（プロジェクトごと作る場合は --create）
logvalet project apply --file conventions.yaml --create
```

`--create` はスペース管理者権限が必要です。

### 既存プロジェクトに導入する

既存のカテゴリ・課題種別を壊さずに導入できます。apply は名前で照合する
冪等な差分適用なので、規約に書いていない既存リソースには触れません。

```bash
# 1. まず現状を棚卸しする
logvalet project health EXISTING_PROJ

# 2. 既存プロジェクトを起点にスケルトンを作る
#    既存のカテゴリが engagements[] の候補に、課題種別が issue_types[] に入る
logvalet project conventions init --from-project EXISTING_PROJ --out conventions.yaml

# 3. Lead と Initiative を埋める（自動生成では埋められない項目）
$EDITOR conventions.yaml

# 4. 差分を確認してから適用する
logvalet project apply --file conventions.yaml --project EXISTING_PROJ --dry-run
logvalet project apply --file conventions.yaml --project EXISTING_PROJ
```

`--from-project` は全案件の Initiative を仮の `未分類` に置きます。
これは自動では決められない項目なので、必ず自分たちのテーマに割り当て直してください。

**過去課題の親子付け替え（既存課題を案件親課題にぶら下げ直す）は行いません。**
規約は導入時点から先の運用に効かせ、過去は `project health` で可視化するだけに留めます。

---

## 埋めるべき項目

`init` が出すスケルトンには全項目にコメントが付いています。
特に次の 3 つは、埋める作業そのものが目的です。

### `priority` — 優先度の意味

Backlog は 高・中・低 の 3 段階固定です。段階を足すことはできないので、
**それぞれが何を意味するのかを案件との相対で定義します。**

```yaml
priority:
  high: "契約・SLA 上、他案件を止めてでも対応する"
  normal: "案件と同じ優先度。必ず担当者を割り当て、担当者が責任を持つ"
  low: "案件より劣後し、実行は保証されない。進むには担当者の熱量か 20% ルール的な仕組みが必要"
```

「低」を「あとでやる」と定義すると溜まり続けます。「実行は保証されない」と
書き切れるかどうかが、この項目の本題です。

### `initiatives` — 重点テーマ

数か月規模のテーマを、**並び順を優先度として**列挙します。
横断テーマと顧客テーマのどちらが上かを明示せざるを得ません。

定常業務も「運用保守」のように明示して置きます。ただし、
案件の所属を決めきれないときの逃げ場にはしないでください。

### `engagements[].lead` — 案件の責任者

1 人だけです。空欄のまま `validate` を通すと warning が出て、
`apply` は**その案件の親課題を作らずスキップ**します（カテゴリは作ります）。

決めていない案件は始めない、という規約をツール側で担保しています。

---

## 日々の運用

### 課題を起票する

```bash
# 案件名 1 つで、案件カテゴリと案件親課題の両方が設定される
logvalet issue create --project-key PROJ --summary "ログ基盤の移行" --engagement "顧客A 基盤更改"
```

案件カテゴリが 0 個や 2 個以上になると stderr に警告が出ますが、
書き込みも exit code も変わりません。規約は運用を止めるためのものではないからです。

MCP からも同じことができます（`logvalet_issue_create` の `engagement` パラメータ）。

### 曖昧さを見る

```bash
logvalet project health PROJ
```

`ambiguities` に次が挙がります。

| 種類 | 意味 |
|---|---|
| `no_engagement` | 案件カテゴリを持たない課題（Linear でいう「溜まり」） |
| `multiple_engagements` | 案件カテゴリを 2 つ以上持つ課題 |
| `missing_parent_issue` | 親課題のない案件カテゴリ |
| `missing_lead` | 担当者のいない案件親課題 |
| `missing_due_date` | 期限のない案件親課題 |
| `unknown_initiative` | Initiative に紐づかない案件 |
| `close_candidate` | `close_policy.low_untouched_days` を超えて放置された低優先度課題 |

曖昧さは `health_score` の減点要因にもなります（1 件 2 点、上限 20 点）。

### 規約を変える

規約の正本は Backlog 上の**規約課題**（種別「規約」の課題 1 件）です。
`conventions.yaml` は apply の入力にすぎません。

変更方法は 2 つあります。

1. `conventions.yaml` を直して `logvalet project apply` を実行する
2. Backlog 上で規約課題の説明欄を直接編集する

どちらでも変更履歴は Backlog の更新履歴に残り、議論は課題のコメントで行えます。
読み出し側（`conventions show`、MCP、`--engagement` の解決）は常に規約課題を見ます。

```bash
# 規約課題から現在の規約を読み出す
logvalet project conventions show --project PROJ
```

規約が未導入のプロジェクトでは `adopted: false` を返すだけで、エラーにはなりません。

---

## CI で使う

`validate --strict` は warning も失敗扱いにします。
Lead 空欄を許さない運用にしたい場合に使ってください。

```bash
logvalet project conventions validate --file conventions.yaml --strict
```

exit code は、error があれば 2、warning のみなら `--strict` 指定時だけ 2 です。

---

## 制約

Backlog API と logvalet の設計上、次の制約があります。

- **優先度はカスタマイズできない**（高・中・低 の 3 段階固定）
- **カスタム状態は最大 8 個**まで。追加・更新にはスペース管理者権限が必要
- **色は Backlog の allowlist からしか選べない**。`validate` が事前に弾きます
  - 状態: `#ea2c00` `#e87758` `#e07b9a` `#868cb7` `#3b9dbd` `#4caf93` `#b0be3c` `#eda62a` `#f42858` `#393939`
  - 課題種別: `#e30000` `#990000` `#934981` `#814fbc` `#2779ca` `#007e9a` `#7ea800` `#ff9200` `#ff3265` `#666665`
- **「案件カテゴリをちょうど 1 つ」は API では強制できない**（カテゴリは複数選択可）。
  `project health` の曖昧さ検知で補います
- **apply はトランザクションではない**。途中で失敗すると成功分は残ります（exit 8）。
  もう一度 apply すれば成功済みは `unchanged` になり、残りが適用されます
- **同名のカテゴリ・課題種別・状態が 2 件以上ある**と、どれを更新すべきか決まらないため
  apply は書き込み前にエラーで止まります
- **apply は MCP に出していません**。一括更新には人の承認を挟む方針です
