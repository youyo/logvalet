# logvalet Phase 1〜3 実装ロードマップ生成 指示書

## この文書の目的

この文書は、`logvalet` リポジトリ上で動作する coding-agent に対して、  
**Phase 1 から Phase 3 までの実装を完遂するためのロードマップを生成させるための、超詳細な指示書**である。

agent はこの文書を起点として、以下の順で作業を進めること。

1. 現状把握
2. ギャップ分析
3. ロードマップ作成
4. 実装プラン作成
5. 詳細設計
6. 実装
7. テスト
8. ドキュメント更新

この文書の役割は、そのうち **ロードマップ作成フェーズを高品質に行わせること** にある。  
ただし、後続のプラン・設計・実装にそのまま接続できるよう、必要な背景・制約・判断基準・完了条件まで具体的に定義する。

---

## 背景

弊社ではタスク管理ツールとして Backlog を利用している。  
一方で Linear には Claude 連携や AI フレンドリーな統合体験が存在し、自然言語での課題参照・更新・文脈理解・進捗整理などが比較的やりやすい。

`logvalet` は、この差分を埋めるために作られた **LLM-first な Backlog CLI** であり、以下の方向性を持つ。

- Backlog API の単純ラッパーではなく、AI が使いやすい粒度に抽象化する
- JSON を中心に、安定して機械利用しやすい出力を返す
- Claude Code / Codex / その他 coding-agent / MCP client から利用しやすい構造にする
- 人間向けの Markdown 表示はビューとして提供しつつ、内部の正規表現は JSON に統一する
- 将来的に Backlog を AI ネイティブな PM / Issue 管理基盤へ拡張する

現時点で `logvalet` には、issue / comment / project / user / activity / document / MCP など、AI接続層としての基礎機能がかなり揃っている。  
したがって今後の主戦場は、単なる接続ではなく **AI ネイティブな高レベル機能の追加** である。

---

## 本開発の最終目的

`logvalet` を、単なる Backlog CLI ではなく、  
**Backlog を Linear に遜色ない AI 体験へ引き上げるための中核レイヤ** に進化させること。

最終的に達成したい状態は次の通り。

### 到達状態

- Claude / Codex / 各種 coding-agent が `logvalet` を介して Backlog を自然言語で高精度に扱える
- 基本 CRUD だけでなく、AI が意思決定しやすい「判断素材コマンド」が揃っている
- issue 単位だけでなく、project / assignee / team / activity レベルで状況理解できる
- 週次・日次 digest、停滞検知、ブロッカー抽出、コメント草案作成などが可能
- Backlog に不足しがちな PM 的視点や intelligence を `logvalet` が外付けで補完する
- JSON を正としながら、人間向け Markdown 表示も自然に提供できる
- CLI と MCP が同じドメインモデル・同じユースケース・同じ出力スキーマを共有する

---

## 絶対に守るべき基本方針

agent は、ロードマップ作成時にも以後のプラン・設計・実装時にも、以下を守ること。

### 1. `logvalet` を中心に拡張する
新規ツールを増やして責務分散しないこと。  
別の補助CLIや別名の新サービスを生やすのではなく、既存の `logvalet` に対して機能追加・構造整理・内部モジュール化で対応すること。

### 2. JSON を正本とする
内部表現、CLI の安定出力、MCP の応答は JSON を正本とすること。  
Markdown は人間向けレンダリングであり、正本にしてはならない。

### 3. API の写経ではなく、AI が使いやすい粒度にする
単に Backlog API のエンドポイントを 1:1 で CLI 化するのでは不十分。  
`stale issues` `project blockers` `issue context` のように、**AI が直接判断に使える単位** に抽象化すること。

### 4. 1コマンド1意思決定を意識する
AI が高品質に使える CLI は、1 回の呼び出しで必要十分な判断材料が返る。  
情報が分散しすぎて複数回の往復が前提になる設計を避けること。

### 5. ノイズを減らす
LLM が使う情報にはノイズが少ないほどよい。  
巨大な生 description、全コメント、冗長な API フィールドを無差別に返さず、要約・抽出・正規化・件数制限を使うこと。

### 6. 安定スキーマを優先する
同じコマンドは同じ構造を返し、`ok/error/code/message/data` のような形を揃えること。  
人間が読みやすいが毎回形が変わる出力より、AI が確実に読める安定構造を優先すること。

### 7. CLI と MCP の二重実装を避ける
ユースケース・ドメインモデル・レスポンス構造はできる限り共通化すること。  
CLI と MCP で別ロジックを持たないこと。

### 8. 実装可能性と漸進性を重視する
壮大だが着手不能なロードマップは不要。  
段階的に価値が出て、各フェーズの終わりで「使える」と言える計画を作ること。

---

## coding-agent に求める成果物

今回、agent にまず作らせたいのは **ロードマップ** である。  
ただし、単なる大雑把な機能一覧ではなく、後続工程に直結するレベルの具体度が必要である。

### ロードマップに必ず含めること

- フェーズ分割
- 各フェーズの目的
- 各フェーズで実装する機能一覧
- 依存関係
- 先行して整備すべき内部基盤
- CLI コマンド案
- MCP への反映方針
- JSON スキーマ整備の要否
- テスト戦略の観点
- ドキュメント更新対象
- 各マイルストーンの完了条件
- リスクとその低減策
- 後続のプラン作成に必要な論点

---

## 作業開始時に最初にやること

agent はロードマップ作成前に、必ずリポジトリの現状を確認すること。  
推測だけでロードマップを書いてはいけない。

### 具体的な確認対象

1. 既存コマンド体系
   - `issue`
   - `comment`
   - `project`
   - `activity`
   - `user`
   - `document`
   - `team`
   - `space`
   - `digest`
   - `mcp`
   - その他サブコマンド

2. 出力フォーマット実装
   - JSON
   - Markdown
   - YAML
   - Gantt 等

3. ドメイン層・APIクライアント層・コマンド層の分離状況

4. Backlog API との接続方式
   - auth
   - config
   - endpoint resolution
   - error handling
   - retry
   - rate limit 考慮

5. MCP サーバ実装の現状
   - どの機能が tool 化されているか
   - CLI と共通化されているか
   - schema が安定しているか

6. テストコードの有無と配置
   - unit test
   - integration test
   - golden test
   - snapshot test
   - mock strategy

7. README / SKILL / ドキュメント構造

8. 既存の digest 機能の粒度と内容

9. 既存で高レベルに近いコマンドがどこまであるか

---

## ロードマップ作成時の基本姿勢

agent は「ゼロから構想する」のではなく、  
**既存の logvalet を最大限活かしながら、どこに高レベル機能を積むか** を考えること。

重要なのは以下。

- すでにあるものは作り直さない
- 既存コマンドを拡張して済むなら新コマンドを乱立しない
- ただし意味のある操作単位になるなら、新コマンド追加をためらわない
- Phase 1 で価値が出る順に並べる
- Phase 2 でワークフロー化する
- Phase 3 で intelligence を載せる

---

# 実装対象フェーズ定義

以下を実装対象とする。  
ロードマップは必ずこの 3 Phase をカバーし、Phase 3 まで完了させる前提で作成すること。

---

## Phase 1: AIネイティブ操作層

### フェーズ目的

AI が Backlog の issue / project / user / activity を扱う際、  
複数の低レベルコマンドを組み合わせなくても、**一発で判断材料を得られるようにする**。

### このフェーズの本質

- 「取得」から「解釈可能な材料提供」へ進化させる
- coding-agent が思考しやすい JSON 構造を揃える
- CLI / MCP 双方から同じ意味操作を使えるようにする

### Phase 1 候補機能

#### 1. issue context
単一 issue を AI が扱うための総合コンテキスト取得。

含めるべき要素の例：

- issue 基本情報
- status / assignee / priority / category / version / milestone
- recent comments
- recent updates
- related issue 候補
- blocker 候補
- next action hint の素地となる材料
- optional summary

想定コマンド例：

- `logvalet issue context PROJ-123`
- `logvalet issue context PROJ-123 --comments 20`
- `logvalet issue context PROJ-123 --compact`

#### 2. project digest 強化
既存 digest がある場合は活用し、足りない視点を強化する。

追加候補：

- stale issues
- overdue
- blocked candidates
- workload imbalance
- recently completed
- newly opened
- risk summary

#### 3. project blockers
project 全体の進行阻害要因を抽出。

ロジック候補：

- 長期間更新がない open issue
- due date 超過
- unresolved dependency 的な状態
- コメント停止
- assignee 未設定
- 重要ラベル付きで停滞

#### 4. stale issues
停滞課題抽出専用コマンド。

必要観点：

- 何日以上更新なしを stale とするか
- status ごとに閾値を変えるか
- recently commented だが status 未更新の扱い
- `waiting` や `on hold` のような status は別扱いにするか

#### 5. assignee workload / user workload
担当者視点の負荷状況を可視化。

観点例：

- open count
- overdue count
- due soon
- priority distribution
- project distribution
- stale assigned issues
- blocked assigned issues

### Phase 1 の完了条件

- AI が issue 単位・project 単位・user 単位で状況理解できる
- 少ないコマンド数で判断材料が揃う
- JSON スキーマが安定している
- Markdown 表示も最低限見やすい
- CLI と MCP に反映方針がある
- テスト可能な deterministic な出力になっている

---

## Phase 2: AIワークフロー層

### フェーズ目的

Phase 1 で得られる判断材料をもとに、  
**AI が実際の業務フローに入り込める操作** を追加する。

### このフェーズの本質

- 読むだけではなく、動けるようにする
- 人間の代わりに下書きや整理を行えるようにする
- Backlog 上の運用を一段抽象化する

### Phase 2 候補機能

#### 6. draft comment
Issue に対するコメント草案生成補助。

用途例：

- 進捗報告
- 確認依頼
- ブロッカー共有
- 作業開始報告
- 作業完了報告
- レビュー依頼
- フォローアップ

重要要件：

- LLM に丸投げせず、構造化入力から下書きを組み立てる余地を持つ
- intent 指定可能
- issue context との連携前提
- Markdown 出力も相性が良いが、内部は JSON とする

想定コマンド例：

- `logvalet issue draft-comment PROJ-123 --intent progress`
- `logvalet issue draft-comment PROJ-123 --intent unblock`
- `logvalet issue draft-comment PROJ-123 --intent review`

#### 7. issue triage
新規課題や未整理課題の仕分け支援。

含めたい観点：

- priority suggestion
- assignee suggestion
- label suggestion
- category suggestion
- urgency signal
- missing information detection

#### 8. spec to issues
仕様文書や設計メモから issue 群を生成する支援。

前提：

- いきなり自動作成でもよいが、まずは preview モードが望ましい
- issue summary / description / hierarchy / labels 等の下書きを返す
- 実ファイル入力や stdin 対応を考慮する

#### 9. weekly digest / daily digest
定期進捗要約生成。

対象粒度候補：

- project
- user
- team
- milestone 相当の単位
- saved filter 相当

出力観点：

- completed
- started
- in progress
- blocked
- overdue
- watch items
- notable decisions

### Phase 2 の完了条件

- AI が「読める」だけでなく「下書きできる」
- 実運用で使える定期 digest が出せる
- triage や spec 分解で、Backlog に情報投入する支援ができる
- 人間の作業時間を削減できることが明確

---

## Phase 3: Intelligence / 差別化層

### フェーズ目的

Backlog に標準では不足しがちな PM / intelligence 的価値を `logvalet` が補完し、  
単なる管理ツール補助を超えて **意思決定支援ツール** に進化させる。

### このフェーズの本質

- Backlog を超える価値を追加する
- 活動履歴やコメント履歴から知見を抽出する
- プロジェクト管理の質を引き上げる

### Phase 3 候補機能

#### 10. decision log extraction
コメントや更新履歴から意思決定ログを抽出。

対象例：

- 方針変更
- スコープ調整
- 採用・不採用の理由
- 一時保留判断
- 依存解消判断
- リリース判断

重要論点：

- 単なる要約ではなく「決定」と「根拠」を分けられるか
- confidence や source references をどう扱うか
- LLM 利用前提でも deterministic さをどう担保するか

#### 11. roadmap assistance
project / issue 群から実行ロードマップ・マイルストーン案を組み立てる。

注意点：

- Backlog に initiative / milestone / roadmap の概念が弱いため、外付け的に表現する可能性がある
- 既存の version / milestone 情報を活用できるなら活用する
- まずは「案」を出す機能として実装してよい

#### 12. activity intelligence / anomaly detection
活動の偏りや異常、停滞や集中を検出する。

例：

- 更新が特定メンバーに偏っている
- 重要課題の更新が止まっている
- 直近で大量の期限超過が発生
- 特定 project にだけ activity が集中
- 予兆的に燃えそうな状況

#### 13. risk summary
project のリスクを構造化してまとめる。

入力候補：

- overdue
- blockers
- stale issues
- assignee imbalance
- unresolved questions
- dependency concentration

### Phase 3 の完了条件

- PM / TL / manager 視点で有用な intelligence が得られる
- Backlog の標準機能以上の価値が明確
- 「Linear と比べて羨ましい」ではなく、「logvalet があるから Backlog でも十分戦える」と言える水準に近づく

---

# ロードマップ作成時の技術観点

agent はロードマップに、以下の技術的観点も必ず織り込むこと。

## 1. コマンド設計方針
- 新規サブコマンドにするか
- 既存コマンド配下に追加するか
- alias を設けるか
- `digest` と `context` の責務分離をどうするか

## 2. JSON スキーマ方針
- response envelope を統一するか
- `ok/error/data/meta` を採用するか
- compact モードと full モードを分けるか
- human-readable fields と machine fields の分離をどうするか

## 3. Markdown レンダリング方針
- JSON を元にレンダリングすること
- Markdown を正本にしないこと
- headings / bullets / status summaries のテンプレートをどう共通化するか

## 4. テスト方針
- JSON snapshot / golden test
- command integration test
- formatter test
- MCP tool response test
- rate limit / API error / not found の異常系 test

## 5. MCP 反映方針
- Phase 1 のコマンドは極力 MCP でも利用可能にする
- tool schema を CLI の JSON スキーマと整合させる
- どの段階で公開するか、実験機能扱いにするかを検討する

## 6. パフォーマンス観点
- comment 取得件数
- recent activity の集約コスト
- N+1 API 呼び出し
- キャッシュ導入の要否
- compact mode による token 削減

## 7. 認証・設定観点
- 既存 config の拡張で済むか
- 新しい feature toggle が必要か
- LLM 補助機能用の provider 設定が必要か
- それともまずは純粋な deterministic 集約に留めるか

---

# ロードマップに必ず盛り込むべき具体項目

agent は最終的なロードマップに、最低でも以下の章立てを含めること。

1. 現状サマリ
2. 目標アーキテクチャ
3. ギャップ一覧
4. フェーズ戦略
5. マイルストーン一覧
6. 各マイルストーンの対象機能
7. 内部基盤整備項目
8. コマンド追加・変更案
9. MCP 反映方針
10. テスト戦略
11. ドキュメント更新計画
12. リスクと対策
13. フェーズ完了条件
14. 次に作成すべき実装プランへの引き継ぎ事項

---

# ロードマップの粒度に関する要求

以下のような粗いロードマップは不可。

- Phase 1: context 作る
- Phase 2: workflow 作る
- Phase 3: intelligence 作る

この程度では後続工程に使えない。  
最低限、以下の粒度で書くこと。

### 必須粒度

- なぜその順番なのか
- どの内部共通部品が先に必要か
- CLI と MCP のどちらを先に対応するか
- 既存コマンドをどう活かすか
- どの feature を experimental とするか
- どの機能を Phase 2 に送るか
- どこで output schema を固定するか
- どこでドキュメントを差し込むか
- どの段階で README に露出させるか

---

# 実装優先順位の考え方

agent は優先順位をつける際、次の軸で判断すること。

## 優先度が高いもの
- 既存構造を活かして実装しやすい
- AI の使い勝手が大きく上がる
- 他機能の土台になる
- deterministic に作りやすい
- テストしやすい
- CLI / MCP 両方で価値がある

## 優先度を下げるもの
- LLM 依存が強く deterministic にしにくい
- 効果が抽象的で測りにくい
- 既存モデルに無理がある
- API コストや呼び出し量が重い
- まず土台整備が必要

---

# ロードマップ作成時の具体的な期待

agent は以下を満たすロードマップを作ること。

### 期待1
読んだ人が、そのまま次に「では実装プランを作ろう」と進めることができる。

### 期待2
`logvalet` の maintainer が読んで、既存コードにどう足すかを想像できる。

### 期待3
フェーズごとに価値があり、途中段階でも使える。

### 期待4
Linear を真似るだけではなく、Backlog + logvalet の文脈で現実的に強い構成になっている。

### 期待5
出力は過度に抽象的でなく、工程管理に使える具体性がある。

---

# 禁止事項

以下は禁止。

- リポジトリ確認なしの憶測ベース設計
- 大量の新規コンセプト導入
- 既存機能の無視
- JSON ではなく Markdown を正本にする提案
- CLI と MCP の仕様乖離を前提にする設計
- 「AI がよしなにやる」で済ませる曖昧な設計
- 実装順序の理由が書かれていないロードマップ
- テスト観点がないロードマップ
- 完了条件が曖昧なフェーズ定義

---

# 期待する最終アウトプット形式

agent は最終的に、Markdown 形式でロードマップを出力すること。  
見出し構造は明確にし、最低でも以下を含めること。

- タイトル
- 概要
- 現状評価
- 目標
- フェーズ一覧
- マイルストーン
- 機能一覧
- 技術基盤整備
- リスク
- 完了条件
- 次工程への引き継ぎ

必要であれば表を使ってよいが、表だけに依存せず、文章で十分に説明すること。

---

# 実行指示

以下を実行せよ。

1. まずリポジトリを確認し、現状機能と構造を把握する
2. 本文書の要求と比較してギャップを整理する
3. Phase 1〜3 を完遂するためのロードマップを Markdown で作成する
4. ロードマップは、後続の実装プラン作成へ直接接続できるレベルの具体度にする
5. 必要に応じて、既存コマンドの拡張案と新規コマンド案を明示する
6. 必要に応じて、内部共通モジュール化やスキーマ整理を先行マイルストーンとして含める
7. 最後に「このロードマップの次に作るべき実装プランの章立て案」まで添える

---

# 補足: 出力形式に関する設計方針

`logvalet` の正本出力は JSON を維持すること。  
Markdown はあくまで人間向けレンダリングであり、CLI 表示や README 例示、Slack 投稿テンプレート等に使うものとする。

ロードマップ上でも、以下の前提を崩さないこと。

- 内部表現は JSON
- LLM 連携も JSON 中心
- Markdown は render layer
- 可能なら compact / full / md のような複数表現を整理する
- スキーマ安定性を優先する

---

# 補足: 特に優先してほしい候補機能

もし優先順位付けに迷った場合、以下を優先すること。

1. issue context
2. project blockers
3. stale issues
4. assignee workload
5. draft comment
6. weekly digest
7. triage
8. decision log extraction
9. risk summary
10. roadmap assistance

この順番は、価値・実装現実性・他機能の土台性を考慮している。  
ただし、実際のリポジトリ構造を見て、よりよい順序があるなら理由付きで変更してよい。

---

# 最後に

このタスクで求めているのは、単なるアイデア列挙ではない。  
`logvalet` を **Backlog 向け AI ネイティブ基盤として本気で完成させるための、実行可能なロードマップ** である。

抽象論ではなく、現状コードベースに接続可能で、  
次にプラン化・設計・実装へ移れるレベルの具体性を持ったロードマップを作成せよ。
