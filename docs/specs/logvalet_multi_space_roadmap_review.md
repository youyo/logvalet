# logvalet multi-space ロードマップ批評レポート

devils-advocate による依存関係・粒度・テスト戦略の脆弱性分析

---

## サマリー

| 深刻度   | 件数 |
|----------|------|
| CRITICAL | 3    |
| HIGH     | 5    |
| MEDIUM   | 4    |
| LOW      | 2    |
| **合計** | **14** |

---

## CRITICAL（このまま実装に入ると誤った成果物が生まれる）

### RC1: MS08 の依存関係が誤り — 並列化計画が崩壊する

**問題:**
マイルストーン依存表では `MS08（SpaceAwareClientFactory）→ MS04（Resolver）` と記載されているが、SpaceAwareClientFactory の実装を見ると `(ctx, SpaceRegistration) → backlog.Client` を生成するだけであり、Resolver を呼び出さない。Resolver は「どのスペースを対象にするか」を決めるものであり、Client を作るファクトリとは責務が異なる。

MS08 の実際の依存は:
- `MS01`（SpaceRegistration 型の定義）
- `internal/auth.TokenManager`（既存）
- `credentials.Resolver`（既存）

MS04 への依存は不要。

**影響:**
この誤りにより、並列化計画の `MS04 + MS06 完了後 → MS07 + MS08 が並列可能` というグラフが正しくない。MS08 は MS01 完了後すぐに着手でき、MS04 完了を待つ必要がない。並列化の実現可能なウィンドウが狭まる（実際はもっと早く着手できるのに待ちが発生する）。

**推奨対処:**
依存表を `MS08 → MS01` に修正する。並列化ダイアグラムも `MS01 完了後: MS02, MS03, MS05, MS06, MS08 が並列可能` に更新する。

---

### RC2: MS06 が過負荷で TDD サイクルが回せない

**問題:**
MS06（DynamoDB SpaceStore + Nonce Store）には実質4つの独立した作業が詰め込まれている:
1. DynamoDB SpaceStore 実装（`internal/space/dynamodbstore.go`）
2. NonceStore **interface** 定義（`internal/auth/nonce.go`）
3. NonceStore DynamoDB 実装（同上）
4. NonceStore SQLite 実装（同上）— MS05 の SQLite SpaceStore とは別パッケージに置かれているが、テーブルは同じ SQLite ファイルを共有する可能性がある

NonceStore interface は MS10（MultiSpaceOAuthHandler）が依存する。しかし interface の定義だけで一つのマイルストーンになり得る（MS01 で他の interface を定義したように）。また NonceStore SQLite 実装は概念的に MS05 の SQLite SpaceStore と対になるべきものだが、MS06 に入っている。

この過負荷により:
- MS06 の Red→Green→Refactor サイクルが4タスクにまたがり複雑
- MS10 の実装者が MS06 の NonceStore interface の完成を待つ構造になるが、interface のみ先に定義できる

**推奨対処:**
MS06 を分割する:
- `MS06a`: NonceStore interface 定義（`internal/auth/nonce.go` の interface と error 定数のみ）→ MS01 完了後すぐ着手可能
- `MS06b`: DynamoDB SpaceStore + NonceStore DynamoDB 実装（`internal/space/dynamodbstore.go`）
- MS05 に NonceStore SQLite 実装を追加する（SQLite を扱う同一マイルストーンとして）

---

### RC3: 並列化ダイアグラムと依存表が矛盾している（2箇所）

**問題1:**
依存表: `MS07 → MS05, MS06`
ダイアグラム: `MS04 + MS06 完了後 → MS07`（MS05 が消えている）

MS07 は SQLite Store の誤設定 validation のため MS05 も必要なはずだが、ダイアグラムでは MS04（Resolver）と MS06（DynamoDB）が前提になっている。MS07 は Resolver とは無関係なので `MS04 + MS06` という前提も誤り。正しくは `MS05 + MS06` が完了後（両ストアに validation が適用できる状態）。

**問題2:**
依存表: `MS14 → MS09, MS10`
ダイアグラム: `MS14 ← MS09 完了後着手可能`（MS10 が省略）

MS14 の `logvalet_space_connect_url` は MultiSpaceOAuthHandler（MS10）が実装する authorize URL 生成を使う。MS10 完了なしに MS14 を実装すると `connect_url` tool が shell out またはスタブになり、一部 tools が動かない状態でマイルストーンが「完了」してしまう。

**推奨対処:**
依存表をダイアグラムの正確な情報源とし、両者を一致させる。MS07 の前提を `MS05, MS06` に修正。MS14 の前提を `MS09, MS10` に統一（ダイアグラム側に MS10 を追加）。

---

## HIGH

### RH1: MS11 に Kong `--spaces foo --spaces bar` 二重渡しのテストケースが存在しない

**問題:**
MS11 のバリデーション一覧には `--spaces ""`, `--spaces ","` 等の空/不正入力は列挙されているが、`--spaces foo --spaces bar` のように同一フラグを2回渡すケースが含まれていない。

`GlobalFlags.Spaces` を `string` 型にすると、Kong が同一フラグを2回受け取った場合は後勝ちになり `foo` が無視されてサイレントに `bar` だけが対象になる。ユーザーへのエラーも出ない。

**影響:** ユーザーが `--spaces foo --spaces bar` を試みると `bar` だけが実行され、`foo` が無視されることにユーザーは気づかない。

**推奨対処:** MS11 のテストケースに `--spaces foo --spaces bar` → エラー（または Kong が弾く旨を仕様化）を追加する。または `Spaces []string` 型に変更して各要素を comma-split + flatten する実装を選び、どちらかを明記する。

---

### RH2: `partial_failure` exit code 衝突の解決が MS16 に先送りされている

**問題:**
リスク表に `partial_failure exit code 2 の既存定義衝突（H6）` が記録されており、「MS16 で対処」とある。しかし exit code の定数定義は `internal/app/exit_code.go` 等のドメイン定数に属するものであり、MS01（Space domain model & errors）の段階で確立すべきだ。

MS16 まで放置すると、MS12〜MS15 の実装者がどの exit code を使えばよいか不明確なまま実装を進め、後から全コマンドの exit code を修正する手戻りが発生する。

**推奨対処:** MS01 の完了条件に「`partial_failure` 用 exit code の定義（既存 exit code 体系との衝突解消）」を追加する。

---

### RH3: `SpaceStatus=disabled` 時の verify 挙動がどのマイルストーンにも定義されていない

**問題:**
前回 spec 批評（`docs/specs/logvalet_multi_space_spec_review.md` M3）で `SpaceStatusDisabled` の verify 挙動が未定義と指摘済みだが、本ロードマップのどのマイルストーンにも「disabled 時の挙動定義・テスト」が含まれていない。

MS12（lv spaces verify）の完了条件に disabled ケースが含まれておらず、disabled space を登録する CLI コマンドも存在しない。

**影響:** 実装者が `SpaceStatusDisabled` を使わないか、あるいは verify で disabled space の扱いが undefined behavior になる。

**推奨対処:** MS01 か MS04 の段階で「disabled space は Resolver から除外する（enabled のみ返す）」「verify は disabled space をスキップして結果に含めない（または `{"ok": false, "error_code": "disabled"}` を返す）」を明記し、MS12 の verify テストケースに加える。

---

### RH4: MS14 での MCP 65 ツールへの引数追加実装量が過小評価されている

**問題:**
MS14 の「既存 tools への引数追加（MVP スコープのみ）」セクションには `spaces` と `all_spaces` 引数を追加するとあるが、MVP 対象（spec §13.1 のマトリクス）だけで 17 ツール × 2 引数 = 34 スキーマ変更が必要。さらに各ツールのハンドラに `SpaceResolver → ExecuteAcrossSpaces` を呼び出す処理を追加する必要がある。

MS14 の対象ファイルには「各 `tools_*.go`」とのみ記載されており、具体的な変更対象ファイルと変更量の見積もりがない。

既存 65 ツールが個別の関数として実装されているなら、共通の `spaces/all_spaces` 解決ロジックを手動で 17 箇所に埋め込むことは実装ミスの温床になる。

**推奨対処:** MS14 の前に「MCP tool 共通 space 解決ミドルウェア」を実装するサブタスクを定義し、各 tool が共通関数経由で fan-out するパターンを確立する。または MCP tool wrapper 関数（`withSpaces(toolFn)`）を `internal/mcp/tools.go` に追加するアプローチを仕様化する。

---

### RH5: NonceStore の配置がパッケージ的に曖昧で将来の混乱を招く

**問題:**
アーキテクチャ図では NonceStore interface を `internal/auth/nonce.go` に置いている。しかし DynamoDB の nonce レコードは `logvalet-spaces` テーブル（`internal/space/dynamodbstore.go`）に `PK=USER#uid, SK=NONCE#<nonce>` として格納される。

これは `internal/auth` パッケージが `internal/space` の DynamoDB テーブルスキーマを知る必要があるか、あるいは `dynamodbstore.go` が NonceStore を兼ねることを意味する。どちらも循環依存またはパッケージ責務の侵食リスクがある。

さらに SQLite NonceStore（MS06 に記載）は論理的には MS05 の SQLite SpaceStore と同じファイルに実装すべきだが、`internal/auth/nonce.go` に書くと `internal/space` の SQLite ファイルとは別の DB ファイルを使うことになる可能性がある。

**推奨対処:** NonceStore interface は `internal/space` パッケージに定義する（SpaceStore が nonce を管理するという設計）か、`Store.ConsumeNonce/StoreNonce` として Store interface に組み込む。MS06 着手前にパッケージ配置を決定し、MS01 または MS06a の完了条件として記載する。

---

## MEDIUM

### RM1: MS07 は MS05/MS06 完了後まで待つ必要がない

**問題:**
ValidateSpaceStoreConfig の実装（`internal/space/config.go`）は SpaceStore の実装に依存しない。storeType 文字列と isMCPRemote bool を受け取るだけの純粋な validation 関数であり、MS01 完了後（型定数が揃えば）すぐに実装できる。

MS07 を MS05/MS06 の後に置くことで、起動時 validation の実装が遅れる。

**推奨対処:** MS07 の依存を `MS01`（または `MS01, MS05, MS06` として、実装は MS01 後に可能・テストは MS05/06 後に完全確認）に変更する。

---

### RM2: Resolver の fallback 5（legacy profile）は MS04 で実装可能だが config.Resolve との結合が未定義

**問題:**
Resolver fallback 5（既存 config/profile から backward compatibility fallback）は `config.Resolve` の結果を使う。しかし `Resolver` は `internal/space` パッケージにあり、`config.Resolve` は `internal/config` パッケージにある。

MS04 の詳細には fallback 5 が `func (r *Resolver) resolveFromLegacyProfile(ctx, resolvedCfg *config.ResolvedConfig)` として示されているが、Resolver struct がどのように `config.ResolvedConfig` を受け取るか（コンストラクタ引数 or context）が未記載。

remote MCP では `config.ResolvedConfig` が存在しないため、fallback 5 は CLI 専用かつオプションになるが、Resolver がこれを切り替える方法が明記されていない。

**推奨対処:** MS04 の完了条件に「Resolver が CLI モードでは fallback 5 を使い、MCP モードでは使わないことを切り替えられるオプション（e.g. `WithLegacyProfileFallback(cfg *config.ResolvedConfig)` オプション）」を追加する。

---

### RM3: MS10 が3つの独立した責務を持ち TDD が難しい

**問題（advisor 指摘と一致）:**
MS10 は以下3つの独立した実装を含む:
1. StateClaims struct の拡張（`internal/auth/state.go`）
2. MultiSpaceOAuthHandler の実装（新規ファイル）
3. NonceStore との接続・callback 処理順序の実装

StateClaims 拡張は既存 state.go に触れる変更で後方互換テストが必要。MultiSpaceOAuthHandler は HTTP handler であり、State + Nonce + TokenStore + SpaceRegistry の4つのモックが必要。これらを1マイルストーンで TDD する場合、テスト記述が膨大になる。

**推奨対処（RC2 と合わせた分割案）:**
- `MS10a`: StateClaims 拡張 + GenerateStateWithSpaceInfo 関数（`internal/auth/state.go` のみ）
- `MS10b`: MultiSpaceOAuthHandler + NonceStore 接続・callback 処理順序実装

これにより MS10a は MS06a（NonceStore interface）完了後すぐに着手でき、MS10b は MS06b と MS10a 完了後に着手できる。

---

### RM4: BC テスト（MS15）で Resolver fallback 5 が「テスト可能か」が不明確

**問題:**
BC6 のテストケース「SpaceRegistry が空でも profile があれば動く（Resolver fallback 5）」は、Resolver が `config.ResolvedConfig` を受け取れる状態でないとテストできない。MS04 で fallback 5 の実装と config との結合が未解決のまま（RM2 参照）では、MS15 段階でも BC6 テストが書けない可能性がある。

**推奨対処:** RM2 で提案した `WithLegacyProfileFallback` オプションを MS04 で実装し、MS15 の BC6 テストが `MemoryStore + WithLegacyProfileFallback` で実行できることを確認する。

---

## LOW

### RL1: 「並列実行可能な組み合わせ」ダイアグラムが prose と依存表の第三の情報源になっている

**問題:**
ロードマップには依存情報が (1) 依存表、(2) prose（「実装優先順位の根拠」）、(3) ダイアグラムの3箇所に分散しており、RC3 で示した通り矛盾が生じている。単一の情報源（依存表）を正とし、prose とダイアグラムは派生として扱うべき。

**推奨対処:** ダイアグラムを削除し、依存表をの単一 source of truth にする。または prose の順序説明はダイアグラムへの参照のみにして重複を避ける。

---

### RL2: localstack セットアップ手順が未記載（CI 実行方法が不明）

**問題:**
MS06 の完了条件に「localstack を使用」とあるが、ローカル開発・CI での localstack 起動方法（Docker Compose、`go test` 前のセットアップスクリプト等）が未記載。

**推奨対処:** MS06 の完了条件に「localstack セットアップ手順が `docs/development.md` または Makefile に記載されている」を追加する。

---

## team-lead 指摘で有効な指摘 / 無効な指摘の整理

### 有効（上記レポートで採用）
- MS08 → MS04 依存は誤り（RC1）
- MS10 の3責務詰め込み（RC2/RM3）
- localstack 未記載（RL2）
- Kong `--spaces foo --spaces bar` テストなし（RH1）
- MS12 PREF 更新詳細なし（RH3 として `disabled` 問題と統合）
- exit code 配置ミス（RH2）
- disabled verify 未定義（RH3）
- MS14 実装量過小評価（RH4）

### 無効（以下の理由で却下）

**「Resolver fallback 5 BC テストが MS15 に含まれていない」→ 却下:**
MS15 の BC6 明示的に「SpaceRegistry が空でも profile があれば動く（Resolver fallback 5）」として記載されている。ただし RM2/RM4 で示した通り、fallback 5 の「テスト可能な実装」が MS04 で保証されているかは別問題。

**「lv spaces import-profiles が MVP に必要」→ 却下:**
Resolver fallback 5（MS04）と BC9 テスト（MS15）が、既存 profile ユーザーの移行パスを自動的にカバーする。spec §22 も「MVP ではやらないこと」として明示除外済み。import-profiles コマンドは UX 改善であり、機能的なブロッカーではない。

---

## GPT vs 自分の差分サマリー

| 指摘カテゴリ | GPT | 自分 |
|---|---|---|
| Store 間の並列化リスク（仕様確定前着手） | 言及あり（抽象的） | RL1 として「情報源の矛盾」に具体化 |
| 依存関係の具体的誤り（MS08→MS04） | 言及なし | RC1 で具体的に指摘 |
| 並列化ダイアグラムと依存表の矛盾 | 言及なし | RC3 で2箇所の具体的矛盾を指摘 |
| MS06 過負荷 | 言及なし | RC2 で NonceStore 分割案付きで指摘 |
| E2E テストが遅い | 指摘あり | MS16 の詰め込み問題として RM3 で扱う |
| 横断操作の partial failure テスト | 指摘あり | MS09 のテストケースに含まれている（既存対応済み） |
| MCP/CLI 責務分離 | 言及あり（抽象的） | RH4 の共通ミドルウェア不足として具体化 |
| NonceStore パッケージ配置 | 言及なし | RH5 で循環依存リスクとして指摘 |
| exit code 配置ミス | 言及なし | RH2 で MS16 先送り問題を指摘 |

GPT は抽象的なリスクカテゴリを列挙する傾向があったが、具体的なパッケージ配置・依存関係の誤り・ダイアグラム矛盾等の実装レベルの指摘はなかった。

---

## 総評

ロードマップ全体の構造と優先順位の考え方は正しい。CRITICAL・HIGH の多くは「依存関係の記述ミス」と「1マイルストーンへの詰め込み過ぎ」に起因しており、設計の方向性に問題があるわけではない。

修正の優先度:
1. **RC1**（MS08 依存修正）: 並列化計画の前提が変わる。今すぐ修正すべき。
2. **RC2 + RC3**（MS06 分割・ダイアグラム修正）: マイルストーン実装前に依存表を確定すべき。
3. **RH1**（Kong 二重渡し）: MS11 実装前にテストケースを追加すべき。
4. **RH2**（exit code）: MS01 着手時に解決すべき。
5. **RH5**（NonceStore パッケージ配置）: MS06 着手前に決定すべき。

---

*Reviewed by: devils-advocate — devflow:team Phase 5 on 2026-05-20*
