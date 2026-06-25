# Roadmap: logvalet Reliability Upgrade（30 → 100 点）

## Meta

| 項目 | 値 |
|------|---|
| ゴール | logvalet を「動くプロダクト」から「プロダクション-grade」へ引き上げる。5 つの溝を埋め、実運用信頼性を 30 点 → 100 点にする |
| 成功基準 | 末尾「100 点チェックリスト」の全項目を満たす。CI ゲートが壊れたコミットを阻止し、**読み取り系（GET）Backlog API 一時障害に自動リトライで耐え**（書き込み系 POST/PUT は重複リスクでリトライ対象外、別途べき等性設計で対応）、Lambda 障害を CloudWatch で 3 分以内に特定できること |
| 制約 | 既存 CLI/MCP インターフェース（exit code・JSON エンベロープ・56 MCP ツール）を保護。TDD 厳守。マルチスペース・partial-failure 設計を維持 |
| 対象リポジトリ | `github.com/youyo/logvalet`（本体）＋ `github.com/heptagon-inc/logvalet-mcp`（Lambda デプロイ） |
| 調査根拠 | 2026-06-15 に 6 観点（アーキテクチャ / テスト・CI / HTTP リトライ / MCP 可観測性 / バージョン同期 / ページネーション境界）で実施した監査。実測カバレッジ 68.8%、テスト 1729 件 |
| 作成日 | 2026-06-15 |
| 最終更新 | 2026-06-15 |
| ステータス | 未着手 |

> **注**: 本ロードマップは詳細計画（milestone 別 m{NN}-*.md）を生成しない（ユーザー指示）。各マイルストーンの着手時に `/devflow:plan` で詳細化する。

---

## 現状の総評 — なぜ「30 点」なのか

logvalet は機能面では成熟している（1729 テスト、56 MCP ツール、15 スキル、マルチスペース、typed errors、partial-failure 設計）。しかし「動くプロダクト」と「プロダクション-grade」の間に **5 つの決定的な溝** がある。これが 30 点の正体。

| 溝 | 現状の姿 | 影響 |
|----|---------|------|
| **A. 本番堅牢性の欠如** | リトライ・タイムアウト・キャンセル・panic recovery 未実装。`Retryable()` フラグはあるのに消費者がいない | Backlog API の一時障害/429 で即エラー終了。Ctrl-C も効かない |
| **B. 運用可観測性の欠如** | 構造化ログ・メトリクス・トレース・リクエスト ID なし（OAuth ログだけ丁寧） | Lambda 障害時に「どのユーザーのどのツールが、なぜ失敗したか」を再構築できない |
| **C. 技術債による保守性低下** | CLI/MCP 重複（resolve 10 関数・`fetchAllIssues` 2 箇所・`extractProjectKey` 3 箇所）、1371 行 God File、digest 層の `[]interface{}` 乱用 | バグ修正が片手落ちになり、ページネーションバグが再発した（S4412/S4415） |
| **D. サプライチェーンの弱さ** | sha256 検証なし、2 リポ間の手動バージョン同期、Lambda ロールバック不可 | silent drift の温床。緊急時に即時切り替えできない |
| **E. テスト・品質保証の空洞** | backlog 読み取り系 HTTPClient が HTTP レベルで未テスト、cli Run が 43.2%、digest は table-driven test 0 件・golden test 1 件 | URL/クエリ組み立ての regression が検知できない |

**重要な構造**: **溝 C と溝 E は連鎖する**（重複コード × 未テスト = 片手落ちバグの再生産）。過去のバグ再発はこの連鎖が原因。したがって Phase 2 では「重複を消す前にテストで保護する」順序を守る。

**良いニュース**: `go test -race` は現在クリーン（並行バグは検出されていない）。設計基盤（typed errors・エラーエンベロープ・dependabot+pinact）は 50-55 点レベルで固まっている。

---

## Current Focus

- **マイルストーン**: M1（信頼性・可観測性・CI 基盤）— 未着手
- **直近の完了**: 6 観点の監査完了（2026-06-15）
- **次のアクション**: M1 の詳細計画を `/devflow:plan` で生成 → P0-α（CI ゲート新設）から着手

---

## Progress

### M1: 信頼性・可観測性・CI 基盤
> 目標: 外部 API 障害に耐え、障害をログから追跡できる「本番で眠れる」状態。壊れたコードが main に入るのを防ぐ。最も体感に効く半日〜数日の施策群。

- [ ] **P0-α: `ci.yml` 新設** — push/PR で `gofmt -l`=0 / `go vet` / `go test -race -coverprofile` / `golangci-lint run` / カバレッジ閾値（現状 68.8%）。現状 CI は pinact・release の 2 つのみで、テスト・lint・vet が PR/push で一切走らない（`.github/workflows/`）
- [ ] **P0-β: `gofmt -w .` で 48 ファイル一括修正** — 実測 48 ファイル違反（`http_client.go`/`domain.go`/`issue.go`/`tool_categories.go` 等の本線含む）
- [ ] **P0-γ: Go 1.26.2+ 更新** — govulncheck が 11 件の脆弱性検出（GO-2026-4866 crypto/x509 認証バイパス等、OAuth の x509.Verify で実経路あり）。1.26.2 で修正済み。mise + `go.mod` bump
- [ ] **P0-δ: CLI の `context` 伝播（残コマンド監査ベース）** — `signal.NotifyContext` で SIGINT 即時中断。※MCP エンドポイント（`mcp.go:362`/`mcp_stdio.go:45`）は**既対応済み**（Codex 指摘 #3 で是正）。未対応の通常コマンド（`issue.go:43`/`watching.go`7 箇所/`user.go`4 箇所 等、`context.Background()` 使用）のみを検証済みインベントリとして対象化。Ctrl-C で 30s 待たされる現状を解消
- [ ] **P0-ε: `activity_stats.go` にページ上限追加** — `analysis/activity_stats.go:191` に安全弁なし（無限ループリスク）。`timeline.go` の `MaxActivityPages` パターンに統一
- [ ] **P1-1: リトライレイヤー（べき等 GET 系のみ）** — `internal/backlog/retry.go` 新設。**⚠ `do()` の包括ラップは禁止**: `do()` は `CreateIssue`/`UpdateIssue`/`AddIssueComment`/`CreateDocument` 等の非べき等ミューテーションに共用され、サーバー受領後のタイムアウト/5xx でリトライすると**二重課題・二重コメント**が生じる（Codex 指摘 #1）。**リトライ対象はべき等操作（GET 系）の allowlist に制限**。429 は `Retry-After` ヘッダ尊重、5xx/ネットワーク一時エラーは指数バックオフ（ジッタ付き・最大 3 回）。書き込み系は Idempotency-Key 等のべき等性保護を先行検討してから拡張
- [ ] **P1-4: MCP ツール呼び出しのアクセスログ + リクエスト ID 伝播** — `tools.go` の `AddTool` ラッパで tool_name/duration/error_type/space を構造化ログ出力。現状ツール失敗が CloudWatch に痕跡を残さない
- [ ] **P0-3 補足: 構造化 JSON ロガー設定** — `slog.SetDefault(slog.NewJSONHandler(...))`。現状 `slog.NewJSONHandler` が本番コードでゼロ件。CloudWatch Insights 検索可能化
- [ ] **P0-4 補足: panic recovery middleware** — `mcp.go` の `topHandler` をラップ。`recover()` がプロジェクト全体でゼロ件。Lambda クラッシュ防止 + 構造化スタック
- [ ] **govulncheck 定期 CI** — 週次実行で新規脆弱性を Issue 化（P0-γ の継続的担保）
- [ ] **P1-9: クロスリポ契約テスト（テナント分離含む）**（Codex 指摘 #2/#3）— `logvalet` ↔ `logvalet-mcp` 間の互換性（exit code・JSON エンベロープ・56 MCP ツール名・リリースバージョン）+ **テナント分離（userID/space/baseURL 伝播・cross-user/cross-space リークのネガティブケース）** を検証する契約テストを新設。M2 リファクタが MCP コンシューマーを壊さないこと、かつマルチスペースでリクエストが誤ルーティング/データ漏洩しないことを M1 段階で担保。※ロールバックスモークは機構（P2-3）前提のため M3 で同時実施
- 詳細: 着手時に `/devflow:plan` で生成

### M2: 保守性とテストの回復
> 目標: バグ修正が 1 箇所で済み、CI が品質を自動担保する状態。**重複を消す前にテストで保護する**順序を厳守（溝 C×E 連鎖の断絶）。

- [ ] **P0-7: backlog 読み取り系 HTTPClient の httptest テスト追加** — `http_client.go:217-826`（ListUsers/GetUser/ListIssueComments/GetDocument/ListProjects 等 20+ メソッド）が HTTP レベルで未テスト。URL/クエリ/デコードの regression が検知不可。**リファクタリング前の保護網として最優先**
- [ ] **P1-6: cli の `listIssues`/`createIssue`/`updateIssue` を DI 可能にして単体テスト化** — Run 本体を純粋関数に抽出。cli カバレッジ 43.2% → 70%+。`config_cmd.go` の stdin Prompter も interface 化
- [ ] **P1-2: CLI/MCP 共通サービス層** — `internal/app/service/` + `resolve/` 新設。resolve 10 関数・`fetchAllIssues`・`extractProjectKey` の重複を一本化。片手落ちバグを構造的に防止
- [ ] **P1-3: digest 層の型安全化** — `[]interface{}` → `[]domain.NormalizedActivity`。`unified.go:100,231,494`・`activity.go:175` の型アサーションを排除
- [ ] **P1-7: digest の table-driven test 化 + golden test 拡充** — 現在 table-driven test 0 件・golden test 1 ファイルのみ。JSON シリアライズ regression 保護
- [ ] **P0-6 拡充: `.golangci.yml` 新設** — errcheck/staticcheck/gocyclo/unparam/misspell 有効化。M1 の CI ゲートに組み込み
- [ ] **P2-1: `http_client.go`(1371 行) リソース別解体 + interface グループ化** — 50 メソッドを 8-10 ファイルへ。`Client` interface(277 行) を機能別 sub-interface に分割しモック軽量化
- [ ] **maxID セマンティクス明記 + 境界テスト** — `activity_filter.go:111` の `maxID - 1`。Backlog API の `maxId` が inclusive/exclusive かを実機検証しコメント明記。片手落ちバグ再発候補の封印
- 詳細: 着手時に `/devflow:plan` で生成

### M3: 運用・サプライチェーン完成
> 目標: 緊急時に 1 コマンドで戻せ、障害をメトリクスで察知できる状態。

- [ ] **P0-5: MCP デプロイに `checksums.txt` 検証** — `deploy.yml:36` が curl のみで `sha256sum -c` なし。サプライチェーン弱点。5 分で実装可能
- [ ] **P1-5: `LOGVALET_VERSION` を GitHub API で動的解決** — 2 リポ間の手動同期ミス（silent drift）を撲滅。logvalet-mcp の `mise.toml` bump 忘れを防止
- [ ] **P1-8: MCP typed error 変換** — `tools_issue.go` 等が `fmt.Errorf` で typed error を素通り。`BacklogError`（exit code/retryable）を構造化 MCP エラーレスポンスに変換
- [ ] **P2-3: Lambda バージョニング + エイリアス + ロールバックスモーク** — `function.json` に `Publish: true` で即時ロールバック可能化。**機構完成後に**ロールバックスモーク（旧バージョン復帰の自動検証）を同時実施（機構前のスモークは偽りの信頼を生むため Codex 再指摘 #2 で順序是正）
- [ ] **P2-2: X-Ray 有効化 + CloudWatch EMF メトリクス** — `function.json` に `TracingConfig: Active`。ツール成功率/レイテンシのアラート設定
- [ ] **README 腐敗の修正** — logvalet-mcp README の古い既定値（`0.15.1`/`git tag v0.15.1`）を mise.toml 参照に修正
- [ ] **孤立タグクリーンアップ手順の docs 化** — GoReleaser before.hooks の `go test` 失敗時の復旧手順
- [ ] **P2-5: MockClient のエラー注入体系化** — `ErrorFor(method) error` 追加。現状 Func 未設定時 `ErrNotFound` 固定
- 詳細: 着手時に `/devflow:plan` で生成

---

## Blockers

なし。全施策は既存コードの制約内で実行可能。M2 のリファクタリングは P0-7（HTTPClient テスト）を先に行うことで安全網を確保する必要がある点のみ留意。

---

## Architecture Decisions

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | 5 つの溝（A 堅牢性 / B 可観測性 / C 技術債 / D サプライチェーン / E テスト空洞）を改善対象とする | 6 観点監査で実測ベースに特定。100 点到達を阻む構造的課題の完全集合 | 2026-06-15 |
| 2 | 溝 C（重複）と溝 E（テスト空洞）は連鎖して扱う | 過去の片手落ちバグ（S4412/S4415）は「重複 × 未テスト」で再生産された。順序: 保護→統合 | 2026-06-15 |
| 3 | M1（CI 基盤 + P0 群）を最優先 | 既存コードへのリスク極小・半日で体感品質が跳ね上がる・以降の Phase の安全網になる | 2026-06-15 |
| 4 | M2 では「リファクタリング前にテストで保護」順序を厳守 | 重複を消す前に保護網（P0-7）を敷かないと、また片手落ちを生む | 2026-06-15 |
| 5 | 詳細計画（m{NN}-*.md）は着手時に遅延生成 | ユーザー指示「プラン不要」。ロードマップ層で合意後、各マイルストーン着手時に `/devflow:plan` | 2026-06-15 |
| 6 | リトライはべき等 GET 系のみに制限 | Codex #1: `do()` は非べき等ミューテーション（CreateIssue 等）に共用されるため、包括ラップは重複書き込みを生む。allowlist 方式を採用 | 2026-06-15 |
| 7 | クロスリポ契約ゲートを M1 で確立 | Codex #2: バージョン同期/ロールバックが M3 だと、M2 リファクタが CLI/Lambda 不一致を出荷しうる。契約テストで M2 リファクタを M1 段階から保護 | 2026-06-15 |
| 8 | P0-δ は残コマンド監査ベースに再スコープ | Codex #3: MCP エンドポイントは既に `signal.NotifyContext` 対応済み。過大評価を是正し、未対応コマンドのみを対象化 | 2026-06-15 |
| 9 | 成功基準を読み取り系リトライに整合 | Codex 再指摘 #1: 書き込み系は重複リスクでリトライ不可。「全 API レジリエンス」の過剰主張を撤回し GET 系に現実化 | 2026-06-15 |
| 10 | ロールバックスモークは機構と同時（M3）| Codex 再指摘 #2: 機構不存在時のスモークは偽りの信頼。検証は検証対象より後に配置 | 2026-06-15 |
| 11 | 契約ゲートにテナント分離を含む | Codex 再指摘 #3: マルチスペース環境で最もコストの高い境界。cross-user/cross-space リークをネガティブケースで保護 | 2026-06-15 |

---

## 100 点チェックリスト（成功基準）

### 堅牢性（溝 A）
- [ ] Ctrl-C で即座に中断し、長時間コマンドにタイムアウトがある（P0-δ）
- [ ] Backlog API の**読み取り系（GET）**が 429/5xx を返しても自動リトライで吸収する（P1-1）。書き込み系（POST/PUT）は重複を防ぐためリトライ対象外とし、べき等性保護で別途対応
- [ ] panic が起きても Lambda がクラッシュせず、構造化スタックが残る（P0-4）

### 可観測性（溝 B）
- [ ] CloudWatch Logs で「ユーザー X のツール Y がリクエスト Z で失敗」を検索できる（P0-3/P1-4）
- [ ] ツール別の成功率/レイテンシがメトリクスで追跡できる（P2-2）

### 保守性（溝 C）
- [ ] resolve / ページネーション / extractProjectKey が単一実装（P1-2）
- [ ] digest 層に `[]interface{}` が無く、IDE の型追従が効く（P1-3）
- [ ] `http_client.go` が 1371 行の God File ではなく、リソース別に分割されている（P2-1）

### サプライチェーン（溝 D）
- [ ] MCP デプロイがバイナリの完全性を検証する（P0-5）
- [ ] logvalet リリース時に MCP が自動で追従（P1-5）
- [ ] Lambda 障害時にエイリアス切り替えで即時復旧（P2-3）

### テスト・CI（溝 E）
- [ ] backlog 読み取り系 API の URL/クエリ/デコードが httptest で保護（P0-7）
- [ ] cli の主要 Run ロジックが DI で単体テスト可能（P1-6）
- [ ] digest 出力の JSON regression を golden test が検知（P1-7）
- [ ] CI が gofmt/vet/lint/race/カバレッジで品質を自動担保（P0-α/P0-6）
- [ ] govulncheck が 0 件（P0-γ）

---

## Changelog

| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-06-15 | 作成 | 6 観点監査（アーキテクチャ / テスト・CI / HTTP リトライ / MCP 可観測性 / バージョン同期 / ページネーション境界）の結果から、5 つの溝を埋める 3 マイルストーン構成でロードマップ初版作成。詳細計画は遅延生成（ユーザー指示） |
| 2026-06-15 | 修正 | Codex adversarial review（needs-attention）の 3 指摘を反映: ①P1-1 をべき等 GET 系 allowlist に制限（非べき等ミューテーションの重複書き込み防止） ②P1-9 クロスリポ契約テスト + ロールバックスモークを M1 に追加（M2 リファクタ前の安全網） ③P0-δ を残コマンド監査ベースに再スコープ（MCP エンドポイント既対応を除外） |
| 2026-06-15 | 修正（2巡目）| Codex 再レビュー（needs-attention）を反映: ①成功基準を読み取り系リトライに整合（書き込み系の過剰主張を撤回、#1）②ロールバックスモークを M1→M3 に移動し P2-3 機構と同時に（順序是正、#2）③契約テストにテナント分離（userID/space/baseURL + cross-user/cross-space ネガティブケース）を追加（#3） |
