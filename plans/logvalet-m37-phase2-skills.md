# M37: Phase 2 SKILL 作成 — 詳細計画

## 概要

Phase 2 で完成した `issue triage-materials`（M34）と `digest weekly/daily`（M36）を材料源とし、LLM 判断を含む新規スキル4つを作成する。また既存スキル2つ（`logvalet`, `logvalet-issue-create`）を更新する。

## 設計原則

- **logvalet = 材料提供（deterministic）、SKILL = LLM 判断** の分離を厳守
- スキルは `skills/` ディレクトリに配置（vercel-labs/skills フォーマット）
- 各スキルは YAML フロントマター（name, description, TRIGGER）+ ワークフロー手順で構成
- 既存スキル（logvalet-health, logvalet-context 等）のフォーマットに倣う

---

## 新規スキル一覧

| スキル | 材料源 | LLM 判断内容 |
|--------|--------|-------------|
| `logvalet-triage` | `lv issue triage-materials ISSUE_KEY` | priority/assignee/category の提案 |
| `logvalet-draft` | `lv issue context ISSUE_KEY` + `lv issue get ISSUE_KEY` | コメント下書き生成 |
| `logvalet-digest-periodic` | `lv digest weekly/daily -k PROJECT` | サマリー・ハイライト抽出 |
| `logvalet-spec-to-issues` | — (SKILL 完結) | spec.md → 課題リスト分解 |

## 既存スキル更新

| スキル | 変更内容 |
|--------|---------|
| `logvalet` | `issue triage-materials`, `digest weekly`, `digest daily` コマンドのドキュメント追加 |
| `logvalet-issue-create` | spec-to-issues ワークフローへの誘導、バッチ課題作成パターン追加 |

---

## 実装詳細

### 1. logvalet-triage

**ディレクトリ:** `skills/logvalet-triage/SKILL.md`

**トリガー:** "トリアージ", "triage", "優先度を決めて", "アサイン提案", "課題を振り分けて", "誰に担当させる"

**ワークフロー:**
1. Issue key の取得（必須）
2. `lv issue triage-materials ISSUE_KEY -f json` で材料取得
3. LLM が以下を判断・提案:
   - priority: 現在の優先度が適切か、変更提案があれば理由付きで提示
   - assignee: 候補者とその根拠（workload_signals があれば参照）
   - category: 課題のカテゴリ分類提案
   - 関連課題との関係性の要約
4. 提案を構造化して出力（実際の更新は確認後）
5. ユーザーが承認した項目のみ `lv issue update` で適用

**出力フォーマット:**
```
## トリアージ提案 — ISSUE_KEY

### 現状
- ステータス: X | 優先度: Y | 担当者: Z

### 提案
| 項目 | 現在 | 提案 | 根拠 |
|------|------|------|------|
| 優先度 | 中 | 高 | 期限超過・高影響度 |
| 担当者 | 未設定 | UserName | 関連スキルあり、負荷余裕あり |
| カテゴリ | — | バグ | 再現手順・エラーログあり |

### 関連課題
- PROJ-100: 類似バグ（解決済み）
- PROJ-200: 同一コンポーネント（進行中）

---
適用しますか？ [優先度/担当者/カテゴリ/全て/キャンセル]
```

### 2. logvalet-draft

**ディレクトリ:** `skills/logvalet-draft/SKILL.md`

**トリガー:** "コメント下書き", "draft comment", "コメントを書いて", "返信を作って", "コメントを作成", "comment draft"

**ワークフロー:**
1. Issue key の取得（必須）
2. 下書きの目的を確認（例: 進捗報告、確認依頼、解決通知、質問応答）
3. 並列で材料取得:
   - `lv issue context ISSUE_KEY -f json`（詳細＋コメント履歴）
4. LLM がコンテキストに基づいてコメントを下書き:
   - 過去のコメントトーンに合わせる
   - 課題の状態（stale, overdue 等）を反映
   - 目的に応じた文体・内容
5. 下書きをユーザーに提示し編集依頼
6. ユーザー確認後に `lv issue comment add ISSUE_KEY --content-file /tmp/draft.md` で投稿

**出力フォーマット:**
```
## コメント下書き — ISSUE_KEY

> コンテキスト: 最終更新 N日前、担当者: UserName

---

<下書き内容>

---
このコメントを投稿しますか？[はい/編集/キャンセル]
```

### 3. logvalet-digest-periodic

**ディレクトリ:** `skills/logvalet-digest-periodic/SKILL.md`

**トリガー:** "週次ダイジェスト", "日次ダイジェスト", "weekly digest", "daily digest", "今週のまとめ", "今日のまとめ", "週報ダイジェスト", "日報ダイジェスト", "定期レポート"

**ワークフロー:**
1. 対象プロジェクトと期間（weekly/daily）を特定
2. `lv digest weekly -k PROJECT_KEY -f json` または `lv digest daily -k PROJECT_KEY -f json` を実行
3. LLM が以下を生成:
   - 期間のハイライト（完了課題、新規課題、注目すべき動き）
   - リスク・懸念事項（期限超過、停滞課題等）
   - 次のアクション候補
4. 人が読みやすい Markdown で出力

**出力フォーマット:**
```
## 週次ダイジェスト — PROJECT_KEY (YYYY/MM/DD - YYYY/MM/DD)

### ハイライト
- 完了: N件（PROJ-XXX, PROJ-YYY）
- 新規開始: N件
- 注目: PROJ-ZZZ が期限に近づいています

### リスク・懸念
- N件が停滞中（7日以上更新なし）
- 期限超過: N件

### 次のアクション
- PROJ-ZZZ のステータス確認
- UserName の過負荷解消のためのタスク再配分検討

---
詳細を確認しますか？ 特定の課題に絞りますか？
```

### 4. logvalet-spec-to-issues

**ディレクトリ:** `skills/logvalet-spec-to-issues/SKILL.md`

**トリガー:** "specから課題作成", "仕様から課題を作って", "spec to issues", "仕様書を課題に分解", "要件を課題にして", "spec 分解", "仕様分解"

**ワークフロー:**
1. spec ファイルのパスまたは内容を取得
2. プロジェクトキー、デフォルト設定（優先度、担当者等）を確認
3. LLM が spec を解析し課題リストを生成:
   - 適切な粒度（1課題 = 1〜3日の作業量）
   - タイトル・説明・優先度・依存関係
   - 親子構造（エピック → タスク）の提案
4. 課題リストをユーザーに提示し確認・編集依頼
5. ユーザー承認後、`lv issue create` で順次作成（dry-run → 実行）

**出力フォーマット:**
```
## Spec → Issues 分解結果 — PROJECT_KEY

> spec: ./spec.md | 課題数: N件

### 課題リスト

1. **[高] API エンドポイント設計**
   - 説明: spec §3 の認証エンドポイントを実装
   - 推定工数: 2日
   - 依存: なし

2. **[中] データベーススキーマ設計**
   - 説明: spec §4 のテーブル定義に従いマイグレーション作成
   - 推定工数: 1日
   - 依存: なし

3. **[高] フロントエンド認証フロー**
   - 説明: spec §5 のログイン/ログアウト UI
   - 推定工数: 3日
   - 依存: 課題1

---
N件の課題を作成します。[全て作成/選択/キャンセル]
```

---

## 既存スキル更新詳細

### logvalet/SKILL.md 更新

**追加セクション（`## issue triage-materials` として）:**

```
## issue triage-materials

課題のトリアージ（優先度・担当者・カテゴリ決定）に必要な材料を一括収集する。

logvalet issue triage-materials ISSUE_KEY

トリアージ判断材料:
- 課題属性（優先度・種別・担当者・期限）
- 課題の更新履歴・コメント履歴
- 類似課題の統計情報
- 担当者の負荷シグナル
- ブロッカーシグナル

```

**追加セクション（`## digest weekly` / `## digest daily` として）:**

```
## digest weekly / digest daily

週次・日次の活動集約サマリーを生成する。

logvalet digest weekly -k PROJECT_KEY
logvalet digest daily -k PROJECT_KEY [--date YYYY-MM-DD]

完了課題・新規開始・ステータス変更・コメント活動を期間ベースで集約する。
```

**`Minimal command set` への追記:**
- `logvalet issue triage-materials PROJ-123`
- `logvalet digest weekly -k PROJ`
- `logvalet digest daily -k PROJ`

### logvalet-issue-create/SKILL.md 更新

**追加セクション（`## Alternative: Spec-to-Issues workflow` として）:**

複数課題を一度に作成する場合の `logvalet-spec-to-issues` スキルへの誘導を追加。

---

## ファイル作成・更新リスト

| 操作 | ファイル |
|------|---------|
| 新規作成 | `skills/logvalet-triage/SKILL.md` |
| 新規作成 | `skills/logvalet-draft/SKILL.md` |
| 新規作成 | `skills/logvalet-digest-periodic/SKILL.md` |
| 新規作成 | `skills/logvalet-spec-to-issues/SKILL.md` |
| 更新 | `skills/logvalet/SKILL.md` |
| 更新 | `skills/logvalet-issue-create/SKILL.md` |

---

## 完了条件

- [ ] `logvalet-triage` スキル: triage-materials を材料に LLM が priority/assignee/category を提案
- [ ] `logvalet-draft` スキル: issue context を材料に LLM がコメント下書きを生成
- [ ] `logvalet-digest-periodic` スキル: digest weekly/daily を材料にサマリー生成
- [ ] `logvalet-spec-to-issues` スキル: spec.md から課題リストを分解・作成
- [ ] `logvalet` スキル更新: Phase 2 コマンド追記
- [ ] `logvalet-issue-create` スキル更新: spec-to-issues 誘導追記
- [ ] `go test ./...` が引き続きパス（スキルはドキュメントのみ、コード変更なし）
