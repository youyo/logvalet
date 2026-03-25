# 批評レポート: M17 — 日付フィルタ強化 + Mermaid gantt 出力

---

## 問題点リスト

### 1. 🔴 **[致命的]** Backlog API v2 が `startDateSince`/`startDateUntil` パラメータをサポートしているか未検証
**根拠と影響:**
- 計画の「前提」セクションでは「Backlog API v2 ドキュメントで対応確認済み」と記載されているが、実際のコード検索結果では既に `dueDateSince`/`dueDateUntil` が実装されている一方、`startDateSince`/`startDateUntil` は一切見当たらない
- Backlog API v2 公式ドキュメント（spec §18.2 参照）を直接確認していない状態での実装計画は、**API互換性の致命的な破綻**を招く
- 仮にこのパラメータが未対応の場合、全機能が機能しない状態でリリースされる可能性がある
- リスク評価表では「高」として記載されているが、対策が「ドキュメントで対応確認済み」という文言のみで、実証的な検証が不足

**推奨対応:** 実装前に Backlog API v2 公式ドキュメントから `startDate*` フィルタの存在を確認するテストケース（`TestBacklogAPI_startDateSupport` など）を追加する。API 実装なしで commit すべきではない。

---

### 2. 🟡 **[重大]** `omitempty` 削除は breaking change だが、外部ツール/スクリプトへの影響範囲が不明確
**根拠と影響:**
- JSON スキーマとして `"startDate": null` が出力されるようになるが、外部コンシューマー（JSON パーサー、Terraform プロバイダー、AIエージェント等）が `null` を明示的に処理していない場合、パースエラーまたは無視される可能性がある
- CHANGELOG に「breaking change」と記載とあるが、セマンティックバージョニングの定義（major.minor.patch）が不明確
- 現在のバージョンが `v0.1.11` であり、次が `v0.2.0` のようだが、このマイナーバージョン上げだけでは SemVer 的に breaking change を適切に表現していない（`v1.0.0` 以上の major version が必要な可能性）
- パーザーテスト（ユーザー向けドキュメント例、複数パーサーの動作確認）を実装フェーズで検証する計画がない

**推奨対応:**
- `null` 出力による既知の互換性問題をドキュメント化し、移行ガイドを作成
- テストに `TestIssueJSON_withNullDates_JSONParserCompatibility` を追加して、一般的な JSON パーサー（jq, Python json, Node.js等）での動作を確認

---

### 3. 🟡 **[重大]** `resolveStartDate()` が `resolveDueDate()` とほぼ同じ実装になる計画だが、コード重複が許容されていない
**根拠と影響:**
- 計画では「resolveStartDate と resolveDueDate のコード重複は許容」とリスク評価では「低」と判定されているが、これは他の open issues/design guideline と矛盾する可能性がある
- DRY 原則や logvalet の「minimal hidden state」ポリシーに反する可能性がある
- 両関数の微細な差（`overdue` 非対応）が将来の保守で不具合の温床になる
  - 例：6ヶ月後に `--start-date overdue` を実装したい場合、重複コードを修正する手間が2倍
  - テストの重複も増加

**推奨対応:**
- 共通化関数 `parseRelativeDateRange(input string, includeOverdue bool)` を先に実装
- `resolveDueDate()` と `resolveStartDate()` はこれを呼び出すラッパーに統一

---

### 4. 🟡 **[重大]** Mermaid gantt 出力の nil 日付フォールバック戦略が曖昧で、実装時の解釈ズレがある
**根拠と影響:**
- 計画では以下のように記載されているが、挙動が一貫性を欠く：
  - `startDate nil → Created を代用`
  - `dueDate nil → startDate + 1日`
  - `Created も nil → 行スキップ`
  
  **問題1**: `startDate` も `Created` も nil の場合、gantt に何も出力されない → ユーザーは課題が存在しないと誤解する可能性
  
  **問題2**: `dueDate nil, startDate有` の場合、`startDate + 1日` で duration=1日のみの gantt タスクになるが、これは実際の期限がないタスクの表現として不適切（終了日が不確実）
  
  **問題3**: `Created` が nil で `startDate` も nil の場合のスキップは、**サイレント・エラー**（ユーザーに警告なく行を消失）で、デバッグが困難
  
- テスト M4 (`TestMermaidGantt_bothNil`) では「Created, Created+1日」と記載されているが、計画の「Created も nil → 行スキップ」と矛盾

**推奨対応:**
- gantt 出力時、nil 日付の場合は明示的にコメント行またはデフォルト値（例：`unknown date`）を出力
- テストケースを拡張：
  - `TestMermaidGantt_allNil` — startDate, dueDate, Created 全て nil → コメント行またはエラーメッセージ
  - `TestMermaidGantt_onlyStartDate` — startDate のみ有効 → duration 決定ロジックを明文化

---

### 5. 🟡 **[重大]** Mermaid gantt の section 分け（projectKey ベース）が複数プロジェクト環境で スケーラビリティ問題を招く
**根拠と影響:**
- `--start-date this-week --project-key P1 --project-key P2 --project-key P3` で 300 課題が返った場合、gantt は3つの section に分割されるが、Mermaid の上限確認がない
- 1 section 内の課題数が多い場合、gantt テキストが肥大化し、Web ブラウザでの描画がタイムアウトする可能性
- 計画では「出力例」で 2 課題のシンプルケースのみ記載されており、実装時に大規模データの動作を検証する計画がない

**推奨対応:**
- Mermaid gantt の出力上限を定義：
  - 例：「100課題以上の場合は JSON フォールバック」と明記
  - テスト `TestMermaidGantt_largeDataset` を追加（500課題でのパフォーマンス確認）

---

### 6. 🟠 **[重大]** `--start-date` と `--due-date` の併用時の AND 結合について、API 仕様が明記されていない
**根拠と影響:**
- 計画では「テストで確認」と記載されているが、実装前に確認すべき仕様
- 例：`--start-date 2026-03-01:2026-03-31 --due-date 2026-03-15:2026-03-31` を指定した場合、API はどのセマンティクスで結合するか？
  - 通常の REST API では AND 結合（両条件を満たす課題）だが、Backlog API では保証されていない可能性
- 計画では「API は AND 結合。テストで確認」とされているが、テスト S11（HTTP Client テスト）にはこれが明示されていない

**推奨対応:**
- テストケース S11 を以下のように拡張：
  ```
  S11a: TestHTTPClient_startDateAndDueDateAND
    入力: StartDateSince=3/1, StartDateUntil=3/31, DueDateSince=3/15, DueDateUntil=3/31
    期待: クエリに 4 パラメータ全て含まれ、API が AND で処理（課題の startDate が 3/1-3/31 && dueDate が 3/15-3/31）
  ```

---

### 7. 🟠 **[中大]** Mermaid gantt の summary から コロン除去は、課題の実際のサマリーを改変するため、追跡性が失われる
**根拠と影響:**
- テスト M7 では「summary に `:` 含む → `:` 除去で正常出力」と記載されているが、**出力ファイル名やガントチャートでの課題追跡が困難になる**
- 例：サマリー「`CND-7 対応状況: KB反映、sandbox2`」が「`CND-7 対応状況 KB反映、sandbox2`」に変換されると、ユーザーが Backlog で検索しても該当課題が見つからない（スペースが埋まるため）
- より深刻には、複数課題のコロンが同じ形式で削除された場合、どの課題のどの部分が削除されたか特定不可能

**推奨対応:**
- Mermaid gantt の制限を理由にサマリーを改変するべきではない
- 代替案：
  1. コロンをエスケープ文字で置換（例：`:`→`-`、または `:`→`​（ゼロ幅スペース）`）
  2. Mermaid 出力時に issue key のみを表示し、summary は別行で出力
  3. gantt テキスト形式ではなく JSON フォーマットで出力

---

### 8. 🟠 **[中大]** 自動ページング条件 `if c.DueDate != "" || c.StartDate != ""` の AND/OR 論理が不明確
**根拠と影響:**
- 計画の IssueListCmd.Run() で「`opt.DueDateSince` / `DueDateUntil` 設定」「自動ページング条件：`if c.DueDate != "" || c.StartDate != ""`」と記載されているが、実装コード (line 100-104) では `if c.DueDate != ""` のみで `c.StartDate` は無視されている
- 新機能として `--start-date` を追加しても、自動ページング条件に含めないと、**フィルタ結果が 100 課題以上の場合、切り詰められる**
- 計画と実装に齟齬がある

**推奨対応:**
- 計画ファイルの 56 行目を修正：
  ```
  - 自動ページング条件: `if c.DueDate != "" || c.StartDate != "" {`
  ```
  に修正し、実装時には `if c.DueDate != "" || c.StartDate != ""` で両フィルタをチェック

---

### 9. 🔵 **[軽微]** MermaidGanttRenderer の JSON フォールバックが渡される `data` の型チェックのみで、スキーマ検証がない
**根拔と影響:**
- 計画では「非Issue データは JSON フォールバック」と記載されているが、`[]domain.Issue` 以外のスライス（例：`[]string`, `[]domain.User`）が渡された場合、単に JSON で出力するのみ
- これは「エラーハンドリング不十分」：ユーザーがうっかり wrong format で renderer を呼び出した場合、警告なく JSON が出力される
- 本来は stderr に `"warning: data type mismatch. expected []domain.Issue, got ..."` を出力すべき

**推奨対応:**
- MermaidGanttRenderer.Render() に type assertion error ログを追加
- テスト M5 を拡張して、非Issue データ時に stderr に警告が出ていることを確認

---

### 10. 🔵 **[軽微]** テスト設計では `TestNewRenderer_mermaid` (M8) が存在するが、`render.go` の NewRenderer() に `"mermaid"` case を追加した後のテストパスを確認する計画がない
**根拠と影響:**
- 計画では「変更ファイル：`internal/render/render.go` (18-31行)」で case 追加とあるが、既存コードを見ると render.go は 32 行で終わっている
- 新しい case を追加すると、出力フォーマット一覧のエラーメッセージも修正が必要だが、計画では言及されていない
- test coverage の確認がない

**推奨対応:**
- render.go の全行数確認と、case 追加による行数の再計算
- NewRenderer() の error message テストケースを追加

---

## 前提の脆弱性

| 仮定 | 成立条件の脆弱性 |
|---|---|
| 「Backlog API v2 が `startDateSince`/`startDateUntil` をサポート」 | **未検証**。公式ドキュメントを参照していない。API実装がない可能性が高い（DueDateSince/Until はあるが StartDate* は検索に引っかからない）。 |
| 「omitempty 削除は breaking change だがセマンティックバージョニングで対応」 | バージョン番号の定義が不明確（v0.2.0 が major か minor か）。外部ツールの互換性テストが実装段階にない。 |
| 「resolveStartDate の重複コードは許容」 | 将来の要件追加（overdue サポート等）時に技術債が増加。DRY 原則との整合性が曖昧。 |
| 「自動ページングを `if c.DueDate != ""`で判定」 | 実装計画では `--start-date` の追加後も条件が更新されていない可能性（計画56行と実装100行の乖離）。 |
| 「Mermaid gantt 出力は nil 日付を自動フォールバック」 | フォールバック戦略（Created代用、skip、default値）が統一されていない。テストM4と計画の記載に矛盾。 |

---

## 見落とされたリスク

1. **Backlog API 仕様確認なしでの実装開始** — startDateSince/Until が未対応の場合、全機能が失敗。実装完了後に判明すると大きな手戻り。

2. **JSON スキーマの breaking change 対策不足** — 外部ユーザーのツール・スクリプト破壊。回避ロジック（enum フィールド追加）の検討なし。

3. **重複コード増加による保守性低下** — resolveStartDate と resolveDueDate が完全複製で、同一バグが2か所に存在する可能性。

4. **Mermaid gantt の nil フォールバック戦略の曖昧性** — ユーザーが「なぜ課題が表示されないのか」とデバッグ困難。サイレント・エラー。

5. **大規模データ（500+ 課題）への対応不明** — gantt テキスト肥大化、ブラウザ描画タイムアウトの可能性を検証していない。

6. **Summary コロン削除による課題追跡困難** — Backlog での検索が失敗し、ユーザーが課題を特定できない。サマリー改変は避けるべき。

7. **AND/OR 論理の実装ズレ** — 自動ページング条件が `--start-date` を含まず、100+ 課題フィルタが切り詰められる。

8. **Backlog API の AND 結合セマンティクス未確認** — `--start-date` と `--due-date` 同時指定時に API がどう処理するか未検証。

---

## ⚠️ 最も危険な1点

**Backlog API v2 が `startDateSince`/`startDateUntil` パラメータをサポートしているか実装前に確認していない**

**理由:**
- 機能1 (--start-date フラグ追加) はこのパラメータが存在することを全前提としており、API 実装がない場合、CLI フラグは機能しない
- 計画では「Backlog API v2 ドキュメントで対応確認済み」と記載されているが、コード内には`startDateSince`/`startDateUntil`が一切見当たらず、既存の `dueDateSince`/`dueDateUntil` のみが実装されている
- 実装開始後に API 非対応が判明すると、**全機能が使用不可**になり、計画全体が無効化される
- 最小限の回避として、実装前に以下を検証すべき：
  1. Backlog API v2 公式ドキュメント https://developer.nulab.com/ja/docs/backlog/ で `startDate` フィルタを検索
  2. API テストエンドポイントで実際に `startDateSince`, `startDateUntil` パラメータを送信し、クエリが受け付けられるか確認
  3. Backlog スペース（heptagon.backlog.com）で `GET /api/v2/issues?startDateSince=...&startDateUntil=...` を実行

---

## 追加の指摘

### パフォーマンスリスク
- 自動ページングで「全件取得」するロジックは、Backlog スペースに 10,000+ 課題がある場合、API 呼び出しが数百回に及ぶ可能性
- rate limit（Backlog API は通常 1分300回）を超える危険
- 計画に maxPages や timeout の記載がない

### ドキュメント更新の不十分性
- README.md に使用例を追加すると計画に記載されているが、「どのような例を追加するか」が明記されていない
- SKILL.md の更新スコープが曖昧

### テストの実行順序
- 計画では「Red → Green → Refactor」と記載されているが、テストファイルの新規作成（resolve_test.go, mermaid_test.go 等）の詳細が計画に含まれていない
- Golden test の活用については言及されていない

---

