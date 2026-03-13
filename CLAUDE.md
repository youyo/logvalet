# CLAUDE.md

## プロジェクト概要

**logvalet** — Backlog 向け LLM-first CLI。thin API wrapper ではなく、digest-oriented structured output を生成するツール。

- プロダクト名: `logvalet`
- ショートエイリアス: `lv`
- モジュールパス: `github.com/youyo/logvalet`
- リポジトリ: `github.com/youyo/logvaret`

## 技術スタック

- Go 1.26.1（mise で管理）
- CLI フレームワーク: [Kong](https://github.com/alecthomas/kong)
- リリース: GoReleaser + GitHub Actions
- 配布: GitHub Releases + Homebrew tap (`youyo/homebrew-tap`)

## ビルド & テスト

```bash
# ビルド
go build -o lv ./cmd/lv/

# テスト
go test ./...

# Lint
go vet ./...
```

## ディレクトリ構造

```
cmd/lv/          — エントリポイント
internal/
  app/           — アプリケーション共通（exit code 等）
  backlog/       — Backlog API クライアント
  cli/           — Kong コマンド定義
  config/        — config.toml ローダー
  credentials/   — tokens.json / OAuth
  digest/        — Digest ビルダー
  domain/        — ドメインモデル
  render/        — 出力フォーマッタ（JSON/YAML/MD/Text）
  version/       — バージョン情報（GoReleaser ldflags）
  util/          — 汎用ヘルパー
docs/specs/      — 設計仕様書
plans/           — ロードマップ & マイルストーン計画
skills/          — Claude Code スキル定義
```

## 設計仕様

詳細な設計仕様は以下を参照:
- `docs/specs/logvalet_full_design_spec_with_architecture.md` — 完全な設計仕様書
- `docs/specs/logvalet_SKILL.md` — スキル定義

## ロードマップ

`plans/logvalet-roadmap.md` に 12 マイルストーンのロードマップがある。
各マイルストーンの詳細計画は `plans/logvalet-m{NN}-{slug}.md`。

## 開発ルール

### TDD 必須
Red → Green → Refactor サイクルを厳守する。テストを先に書く。

### テスト方針
- Backlog API テストは **モックのみ**（interface ベースの Client でモック実装を使用）
- Golden test を digest 出力に活用
- `go test ./...` が常にパスする状態を維持

### コーディング規約
- JSON キーは `snake_case`
- Go struct フィールドは `CamelCase` + 明示的 JSON タグ
- stdout は機械可読な結果のみ（JSON デフォルト）
- stderr は警告・ログ・診断用
- エラーは JSON エンベロープ形式で stdout に出力（spec §9）

### Exit codes
```
0  success
1  generic error
2  argument / validation error
3  authentication error
4  permission error
5  resource not found
6  API error
7  digest generation failed
10 configuration error
```

### コミット
- Conventional Commits 形式
- メッセージは日本語
- plans/*.md を含むコミットには `Plan:` フッターを追加
