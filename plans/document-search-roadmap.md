# Roadmap: Backlog ドキュメント検索（document-search）

## Meta
| 項目 | 値 |
|------|---|
| ゴール | Backlog ドキュメントのキーワード検索を CLI / MCP / Skill の全層に追加し、溜まった知見を LLM から探せるようにする |
| 成功基準 | `lv document search <keyword>` と `logvalet_document_search` が**単一スペース内・全プロジェクト横断**でヒット文書をスニペット付き単一 digest で返す。複数スペース指定時は既存 fan-out（スペース別結果配列）に委譲。`go test ./...` / `go vet ./...` がパスし、実 API スモークが通る。`logvalet:document-search` スキルが両配置先で発火する |
| 制約 | TDD 必須（Red→Green→Refactor）/ Backlog API テストはモックのみ / 既存 `ListDocuments` インターフェースは変更しない / JSON は snake_case / マルチスペースは既存 `RegisterWithSpaces` の挙動を踏襲（新規集約層は作らない） |
| 対象リポジトリ | `youyo/logvalet`（CLI・MCP 実装）、`youyo/claude-plugins`（プラグイン版スキル）、`heptagon-inc/logvalet-mcp`（Lambda 配布・バージョン追従のみ） |
| 作成日 | 2026-06-11 |
| 最終更新 | 2026-06-11 01:45 |
| ステータス | 進行中 |

## インタビュー結果（確定事項）

| # | 論点 | 決定 |
|---|------|------|
| 1 | 検索スコープ | スペース内を横断検索（projectId は任意の絞り込みフィルタ） |
| 2 | 出力粒度 | **スニペット抜粋つき digest をデフォルト**。`detail` で切替（snippet / meta / full） |
| 3 | スキル粒度 | **検索特化の単機能スキル** `logvalet:document-search` |
| 4 | 検索ロジック | **Backlog の keyword 検索をそのまま利用**（client-side 再ランキングなし、MVP） |
| 5 | スキル配置 | `logvalet/skills/` と `claude-plugins/plugins/logvalet/skills/` の**両方にミラー配置** |
| 6 | 横断スコープの確定 | **単一スペース内・全プロジェクト横断**（projectId 省略）→ 単一 digest。複数 Backlog スペース指定時は既存 fan-out（スペース別結果配列）に委ねる。新規の複数スペース集約層は作らない |
| 7 | 取得上限 | **1 コールで最大 100 件（count=100 既定）**。ちょうど 100 件返った場合は「さらにヒットの可能性あり・`--offset` でページング可能」を明示（silent な切り捨てはしない） |

## 技術前提（探索で確定）

- Backlog API: `GET /api/v2/documents` は `keyword`(任意) / `projectId[]`(任意・複数可) / `sort`(created\|updated) / `order`(asc\|desc) / `offset`(**必須**) / `count`(1-100, 既定20) をサポート。
- **レスポンスは各ドキュメントに本文 `plain` を含む**（`internal/domain/domain.go:111`）→ スニペット抽出は取得済み `plain` から client-side で切り出すだけで**追加 API コール不要**。
- 既存 `ListDocuments(ctx, projectID int, opt)`（`http_client.go:566`）は projectID 必須・keyword 非対応 → 横断検索には新規メソッドが必要。
- マルチスペース: `RegisterWithSpaces`（`internal/mcp/tools.go`）は `all_spaces`/`spaces` 指定時に各スペースで独立実行し**スペース別配列**を返す。本機能はこの既存挙動をそのまま継承（単一 digest はあくまで 1 スペース内の話）。
- 既存 `ListWikis` は `keyword` をサポート（`http_client.go` 付近）するが**プロジェクト必須**。document 検索は projectId 任意にするため、検索体験が wiki と非対称になる点は許容（AD8）。

## Open Verifications（実 API で要確認・M1 着手前〜M5 スモーク）

| # | 確認事項 | 影響するマイルストーン | 既定の前提 |
|---|----------|----------------------|-----------|
| V1 | `GET /documents` で `projectId[]` を**完全省略**した時、スペース内全プロジェクトを横断して返すか（エラー/空でないか） | M1, M3 | 横断して返ると仮定。違えば M3 で「全プロジェクト ID を列挙して projectId[] に詰める」フォールバックへ |
| V2 | keyword マッチのスコープ（本文のみ / タイトル・タグも含むか、AND/OR/部分一致の挙動） | M2（snippet 設計） | 本文を含む全文検索と仮定。タイトルのみマッチ時は snippet がリード抜粋にフォールバック |
| V3 | ゴミ箱（TrashTree）文書・閲覧権限のない文書が結果に混じるか | M2, M5 | active のみ・権限内のみと仮定。混じる場合は statusId 等でフィルタ検討 |
| V4 | `count` に 101 以上を渡した時の挙動（400 か自動丸めか） | M1, M3 | 400 になり得ると仮定し M3 で 100 にクランプ |

## Current Focus
- **マイルストーン**: M1 — backlog client `SearchDocuments`
- **直近の完了**: ロードマップ改訂（devils-advocate 批評の反映：スコープ確定・ページング方針・検証項目追加）
- **次のアクション**: V1/V2 を実 API で軽くスモーク → M1 を `/devflow:implement` で実装（TDD）

## Progress

### M1: backlog client — SearchDocuments
- [ ] `SearchDocumentsOptions`（Keyword/ProjectIDs/Sort/Order/Offset/Count）を options.go に追加
- [ ] `SearchDocuments(ctx, opt) ([]domain.Document, error)` を Client interface に追加
- [ ] http_client.go に実装（`GET /api/v2/documents`、projectId[] 任意・複数、offset 必須）
- [ ] mock_client.go に `SearchDocumentsFunc` を追加（静的アサートで interface 充足確認）
- [ ] options / http_client のテスト（クエリ構築 + count 境界値 0/1/100）
- 詳細: plans/document-search-m01-backlog-client.md

### M2: digest — document search digest + スニペット抽出
- [ ] `DocumentSearchBuilder` を digest 層に追加（[]Document → 単一 DigestEnvelope）
- [ ] スニペット抽出（keyword 周辺 ±N **rune**、ケースインセンシティブ、ヒットなし時はリード抜粋）
  - [ ] **マルチバイト安全**: `[]rune` ベースで切り出し（`[]byte` 禁止）
  - [ ] **複数語 keyword**: 最初にマッチした語をアンカーに切り出し（アンカー決定ルールを明記）
  - [ ] **plain のノイズ**: V2/plain 実体確認の上、必要ならマークダウン記法の軽い除去
- [ ] verbosity モード（snippet=既定 / meta=本文除外 / full=本文全文）。snippet・meta では full `plain` を**返さない**（トークン抑制）
- [ ] 取得件数サマリー（`total_returned`、`possibly_more`（=100件ちょうどで true））を digest に含める
- [ ] golden test（**単一スペース固定**でデコード結果を検証。fan-out 非決定性は対象外）
- 詳細: 着手時に生成（遅延生成）

### M3: CLI + MCP 配線
- [ ] `lv document search <keyword> [--project KEY ...] [--sort] [--order] [--count] [--offset] [--detail snippet|meta|full]`
- [ ] `logvalet_document_search` ツールを `RegisterWithSpaces` で登録（read-only・マルチスペース継承）
- [ ] project_key → projectID 解決（任意の絞り込み・複数可）
- [ ] **count 既定 100・上限 100 にクランプ**（V4 対策）
- [ ] V1 が偽だった場合のフォールバック（全プロジェクト列挙）は V1 確認後に要否判断
- [ ] CLI / MCP テスト
- 詳細: 着手時に生成（遅延生成）

### M4: Skill — logvalet:document-search（2箇所配置）
- [ ] `skills/document-search/SKILL.md`（logvalet repo）
- [ ] `claude-plugins/plugins/logvalet/skills/document-search/SKILL.md`
- [ ] plugin.json の version バンプ
- 詳細: 着手時に生成（遅延生成）

### M5: E2E 検証 & リリース
- [ ] `go test ./...` / `go vet ./...` グリーン
- [ ] 実 Backlog API スモーク（keyword 横断検索 + V1/V2/V3 の最終確認）
- [ ] CHANGELOG 追記・version バンプ・タグ・リリース CI 監視
- [ ] logvalet-mcp の mise.toml バージョン追従（タグプッシュでデプロイ）
- 詳細: 着手時に生成（遅延生成）

## Blockers
なし

## Architecture Decisions
| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| AD1 | 既存 `ListDocuments` を変更せず新規 `SearchDocuments` メソッドを追加 | projectId 任意・keyword・複数プロジェクト・sort/order が必要で別シグネチャになるため。既存インターフェースを制約として保護（Scope Management） | 2026-06-11 |
| AD2 | スニペット抽出はレスポンスの `plain` から client-side で実施 | 検索レスポンスが本文 `plain` を含むため追加 API コール不要。コスト懸念を構造的に解消 | 2026-06-11 |
| AD3 | 出力 verbosity を `detail`(snippet/meta/full) で切替、既定は snippet。snippet/meta では full plain を返さない | snippet が情報量とトークンコストのバランス最良。巨大本文によるトークン爆発を防ぐ | 2026-06-11 |
| AD4 | MVP は Backlog サーバーサイド keyword 検索に委譲（client 再ランキングなし） | シンプル・高速。再ランキングは将来の拡張余地として保留。挙動は V2 で確認 | 2026-06-11 |
| AD5 | CLI と MCP を 1 マイルストーン（M3）に統合 | 両者とも digest 上の薄い配線層。先例 `plans/logvalet-m24-stale-cli-mcp.md`（実在）に倣う | 2026-06-11 |
| AD6 | 「スペース全体横断」＝**単一スペース内・全プロジェクト横断**と定義。複数スペースは既存 fan-out に委譲し新規集約層は作らない | 既存 `RegisterWithSpaces` がスペース別配列を返す構造と、単一 digest 要件の矛盾を回避。全ツールと一貫した挙動を維持 | 2026-06-11 |
| AD7 | 取得は 1 コール count=100 既定。100 件返却時は `possibly_more=true` を明示し silent な切り捨てをしない | ユーザー選択。追加コール無しで実用上十分な件数を確保しつつ取りこぼしを可視化 | 2026-06-11 |
| AD8 | document 検索は projectId 任意（wiki 検索はプロジェクト必須）。非対称を許容 | 「知見を横断で探す」ユースケースを優先。wiki との統一は将来検討 | 2026-06-11 |

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-06-11 01:30 | 作成 | ロードマップ初版作成。インタビュー4問＋API探索で検索スコープ・出力粒度・スキル粒度・検索ロジックを確定。`plain` がリストレスポンスに含まれることを確認しスニペット抽出のコストゼロを根拠に AD2/AD3 を決定 |
| 2026-06-11 01:45 | 改訂 | devils-advocate 批評を反映。「スペース全体横断」を単一スペース内・全プロジェクト横断に確定（AD6、fan-out 矛盾の回避）。取得上限を count=100・possibly_more 明示に確定（AD7）。keyword スコープ等の実 API 検証項目 V1-V4 を追加。M2 に snippet のマルチバイト/複数語/ノイズ対策と verbosity でのトークン抑制を明記。ゴミ箱/権限/レート制限/トークン爆発をリスク・検証へ反映。AD5 先例の実在を確認（批評の誤判定を訂正） |
