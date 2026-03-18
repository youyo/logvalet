# Roadmap v2: logvalet

## Meta
| 項目 | 値 |
|------|---|
| ゴール | Backlog 向け LLM-first CLI の MVP 完成 + スペック完全準拠 |
| 成功基準 | 全 digest コマンドが安定 JSON スキーマで動作し、Homebrew tap 経由でインストール可能。スペック §9 エラーエンベロープ対応。GlobalFlags 完全実装 |
| 制約 | Go 1.26.1 / Kong CLI / TDD 必須 / モックベーステスト / コード品質優先 |
| 対象リポジトリ | github.com/youyo/logvalet |
| 作成日 | 2026-03-18 |
| 最終更新 | 2026-03-18 |
| ステータス | 進行中 |
| 前バージョン | plans/logvalet-roadmap.md (v1, 完了) |

## Current Focus
- **マイルストーン**: M13 — GlobalFlags 完全実装
- **直近の完了**: M12 — Release pipeline & distribution
- **次のアクション**: M13 の詳細計画を確定し実装を開始する

## 完了済みマイルストーン (v1)

### M01: Project scaffold & CLI foundation ✅
- [x] go.mod 初期化 (github.com/youyo/logvalet)
- [x] ディレクトリ構造作成 (spec §16)
- [x] cmd/logvalet/main.go — Kong エントリポイント
- [x] GlobalFlags / DigestFlags / ListFlags / WriteFlags (spec §17.1-17.2)
- [x] Root CLI struct — 全コマンドプレースホルダー (spec §17.3)
- [x] Version package — ldflags 注入 (spec §23)
- [x] Exit code 定義 (spec §8)
- [x] JSON renderer (デフォルト出力)
- [x] Completion commands — bash/zsh/fish + --short (spec §6)

### M02: Config system ✅
- [x] config.toml スキーマ定義・ローダー (spec §5)
- [x] Profile 解決ロジック
- [x] 環境変数オーバーライド
- [x] Boolean env parsing (1/true/yes/on)
- [x] 設定値優先順位 (CLI flags > env > config > defaults)

### M03: Credential system & auth commands ✅
- [x] tokens.json スキーマ・ストア (spec §5)
- [x] Credential resolver (優先順位付き)
- [x] API key サポート
- [x] auth login / logout / whoami / list (spec §5)

### M04: Backlog API client core ✅
- [x] Client interface 定義 (spec §18.1 — 全メソッド)
- [x] HTTP transport + auth header injection
- [x] Request/Response option types (spec §18.2-18.3)
- [x] Typed error handling (spec §18.4 — ErrNotFound 等)
- [x] Exit code マッピング

### M05: Domain models & full rendering ✅
- [x] Domain types: issue, project, activity, user, document, team, space (spec §11-12)
- [x] Renderer interface (spec §20)
- [x] JSON renderer (pretty-print 対応)
- [x] YAML renderer
- [x] Markdown renderer
- [x] Text renderer

### M06: Issue read & digest ✅
- [x] issue get コマンド (spec §14.1)
- [x] issue list コマンド (spec §14.2)
- [x] IssueDigestBuilder (spec §19)
- [x] issue digest コマンド (spec §14.3, §13.1)
- [x] Golden tests — digest JSON 出力

### M07: Project & meta commands ✅
- [x] project get / list (spec §14.9-14.10)
- [x] ProjectDigestBuilder
- [x] project digest (spec §14.11, §13.2)
- [x] meta status / category / version / custom-field (spec §14.23-14.26)

### M08: Issue write operations ✅
- [x] issue create (spec §14.4)
- [x] issue update (spec §14.5)
- [x] issue comment list / add / update (spec §14.6-14.8)
- [x] 排他フラグバリデーション (--content vs --content-file)
- [x] --dry-run サポート

### M09: Document commands ✅
- [x] document get / list / tree (spec §14.18-14.20)
- [x] DocumentDigestBuilder (spec §13.5)
- [x] document digest (spec §14.21)
- [x] document create (spec §14.22)

### M10: Activity & user commands ✅
- [x] activity list (spec §14.12)
- [x] ActivityDigestBuilder (spec §13.3)
- [x] activity digest (spec §14.13)
- [x] user list / get (spec §14.14-14.15)
- [x] user activity (spec §14.16)
- [x] UserDigestBuilder (spec §13.4)
- [x] user digest (spec §14.17)

### M11: Team & space commands ✅
- [x] team list / project (spec §14.27-14.28)
- [x] TeamDigestBuilder (spec §13.6)
- [x] team digest (spec §14.29)
- [x] space info / disk-usage (spec §14.30-14.31)
- [x] SpaceDigestBuilder (spec §13.7)
- [x] space digest (spec §14.32)

### M12: Release pipeline & distribution ✅
- [x] .goreleaser.yaml (spec §21)
- [x] .github/workflows/release.yml (spec §22)

## 新マイルストーン

### M13: GlobalFlags 完全実装
- [ ] `internal/cli/global_flags.go`: 欠落6フラグを Kong struct に追加
  - `--api-key` / `LOGVALET_API_KEY`
  - `--access-token` / `LOGVALET_ACCESS_TOKEN`
  - `--base-url` / `LOGVALET_BASE_URL`
  - `--space` / `-s` / `LOGVALET_SPACE`
  - `--config` / `-c` / `LOGVALET_CONFIG`
  - `--no-color` / `LOGVALET_NO_COLOR`
- [ ] `internal/cli/runner.go`: `buildRunContext()` 修正
  - `CredentialFlags{}` → GlobalFlags から api-key/access-token を渡す
  - `config.OverrideFlags` に Space/BaseURL/NoColor/ConfigPath を渡す
- [ ] テスト: GlobalFlags → OverrideFlags → RunContext の結合テスト

### M14: JSON エラーエンベロープ (§9)
- [ ] `internal/domain/` に ErrorEnvelope/WarningEnvelope 型定義
- [ ] `cmd/logvalet/main.go` のエラーハンドリング修正
  - エラー時に JSON エンベロープを stdout に出力
  - exit code との整合性
- [ ] テスト: 各 exit code に対応するエラーの JSON 出力確認

### M15: `logvalet config init` コマンド（対話型セットアップ）
- [ ] `internal/cli/config.go`: `ConfigInitCmd` 追加（`aws configure` 相当）
  - 対話プロンプトで profile 名、space 名、base_url を入力
  - `~/.config/logvalet/config.toml` を生成・追記
  - 既存プロファイルがある場合は上書き確認
- [ ] `logvalet configure` をトップレベルエイリアスとして追加（Kong `cmd:"configure"` + `hidden` or alias）
- [ ] `auth login` との連携: config init → auth login の導線
- [ ] `internal/config/writer.go`: config.toml の書き出しロジック
- [ ] テスト: 対話入力のモック + 生成された config.toml の検証

### M16 (optional): `logvalet version` コマンド
- [ ] `internal/cli/root.go`: Version コマンド追加
- [ ] `--version` グローバルフラグ対応

## Blockers
なし

## Architecture Decisions
| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | TDD 必須 (Red→Green→Refactor) | CLAUDE.md ルール + コード品質優先の方針 | 2026-03-13 |
| 2 | Backlog API テストはモックのみ | interface ベースで Client を定義し、テストではモック実装を使用 | 2026-03-13 |
| 3 | 12 マイルストーン分割 | スペックの 5 フェーズをより細かい粒度に分割し、各マイルストーンを独立してデリバリー可能にする | 2026-03-13 |
| 4 | OAuth は API key 認証に変更 | Backlog の OAuth2 は client_id/secret の事前登録が必要で CLI 組み込みが複雑。API key 認証に絞る | 2026-03-13 |
| 5 | roadmap v2 で残差分を管理 | v1 の M01-M12 完了後、スペックとの差分を新 M13-M16 で管理 | 2026-03-18 |
| 6 | `logvalet config init` で初期セットアップの障壁を下げる | config.toml を手書きするハードルが高い。`aws configure` のような対話型セットアップが必要 | 2026-03-18 |

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-03-18 | 作成 | roadmap v2 作成。v1 M01-M12 完了を反映、新 M13-M15 を追加 |
| 2026-03-18 | 追加 | M15 `logvalet config init` を追加、旧 M15 version を M16 に繰り下げ |
