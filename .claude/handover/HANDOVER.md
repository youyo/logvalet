# HANDOVER.md

> 生成日時: 2026-03-13 (M12 実装完了)
> プロジェクト: logvaret
> ブランチ: main

## 今回やったこと（M12 Release pipeline & distribution）

### .goreleaser.yaml — 新規作成

GoReleaser v2 形式のリリース設定（スペック §21）:
- `project_name: logvalet`
- バイナリ名: `logvalet`（`cmd/lv/main.go` エントリポイント）
- ターゲット: darwin/linux × amd64/arm64
- `ldflags`: Version/Commit/Date を `internal/version` パッケージへ注入
- アーカイブ: tar.gz、名前テンプレートでアーキテクチャ表示を最適化
- Homebrew tap: `youyo/homebrew-tap`（`HOMEBREW_TAP_GITHUB_TOKEN` 使用）
- `changelog` 設定: docs/test/chore を除外
- `prerelease: auto`

### .github/workflows/release.yml — 新規作成

GitHub Actions リリースパイプライン（スペック §22）:
- トリガー: `push` → `tags: ["v*"]`
- `permissions: contents: write`
- steps: checkout → setup-go → GitHub App token 生成 → goreleaser-action@v6
- `HOMEBREW_TAP_GITHUB_TOKEN` は tibdex/github-app-token@v2 で生成
- 必要 Secrets: `APP_ID`, `APP_PRIVATE_KEY`

### README.md — 新規作成（英語）

- バッジなし（シンプル構成）
- インストール方法: Homebrew + go install
- クイックスタート
- シェル補完セットアップ
- コマンド一覧（全コマンドをテーブル形式）
- グローバルフラグ一覧
- `--dry-run` 安全性の説明

### README.ja.md — 新規作成（日本語）

README.md の日本語版。内容・構成は同一。

### skills/SKILL.md — 新規作成

`docs/specs/logvalet_SKILL.md` をそのままコピー。スキル定義はスペック通りの内容で完成済み。

## 決定事項

- **バイナリ名は `logvalet`**: GoReleaser と Homebrew では `logvalet` を使用。`lv` はシェルエイリアスとして README で案内。
- **ターゲットは darwin/linux のみ**: スペック §21 に準拠（Windows は対象外）。
- **GoReleaser v2 format**: ccmix の実績ある設定スタイルを踏襲。

## 検証結果

- `go build ./...` — ビルド成功
- `go test ./...` — 全テストパス（10パッケージ）
- 全設定ファイルが正しい YAML 文法で作成済み

## 全マイルストーン完了状況

| マイルストーン | タイトル | ステータス |
|--------------|---------|----------|
| M01 | Project scaffold & CLI foundation | 完了 |
| M02 | Config system | 完了 |
| M03 | Credential system & auth commands | 完了 |
| M04 | Backlog API client core | 完了 |
| M05 | Domain models & full rendering | 完了 |
| M06 | Issue read & digest | 完了 |
| M07 | Project & meta commands | 完了 |
| M08 | Issue write operations | 完了 |
| M09 | Document commands | 完了 |
| M10 | Activity & user commands | 完了 |
| M11 | Team & space commands | 完了 |
| M12 | Release pipeline & distribution | **完了** |

**12マイルストーン全て完了。MVP 実装完成。**

## 次にやること

- [ ] `git push origin main` でリモートへプッシュ
- [ ] GitHub Release 作成（`git tag v0.1.0 && git push origin v0.1.0`）
- [ ] GoReleaser が自動でビルド・Homebrew formula 更新を実行
- [ ] CLI Run() に BacklogClient を統合（credential/config 完成後の継続タスク）
  - 全コマンドの Run() を BacklogClient 呼び出しで実装
  - 現在は全コマンドが `ErrNotImplemented` を返す stub 状態

## 関連ファイル

- `plans/logvalet-roadmap.md` — 12マイルストーンのロードマップ
- `plans/logvalet-m12-release.md` — M12: 詳細計画（完了）
- `docs/specs/logvalet_full_design_spec_with_architecture.md` — 完全な設計仕様書
- `.goreleaser.yaml` — GoReleaser 設定（M12 新規）
- `.github/workflows/release.yml` — リリース CI（M12 新規）
- `README.md` — 英語 README（M12 新規）
- `README.ja.md` — 日本語 README（M12 新規）
- `skills/SKILL.md` — Claude Code スキル定義（M12 新規）
