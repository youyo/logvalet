# M12: Release pipeline & distribution — 詳細計画

## メタ
| 項目 | 値 |
|------|---|
| マイルストーン | M12 |
| タイトル | Release pipeline & distribution |
| 作業ディレクトリ | /Users/youyo/src/github.com/youyo/logvaret |
| 前提 | M11 完了（コミット 47e0a1a）、全テストパス |
| 作成日 | 2026-03-13 |

## 実装対象

### 1. `.goreleaser.yaml`

スペック §21 に基づく GoReleaser 設定。

**方針:**
- ccmix の `.goreleaser.yaml` を参考にしつつ、スペックの設定を適用
- GoReleaser v2 形式（`version: 2`）
- `lv` というバイナリ名ではなく、スペック通り `logvalet` をメインバイナリとする
  - 理由: Homebrew formula は `logvalet` として配布、`lv` エイリアスはシェル設定で対応
- Windows 対応を追加（スペックは darwin/linux のみだが、ロードマップには windows/amd64 記載あり）
  - → スペック §21 に従い darwin/linux のみにする（スペック優先）

**設定内容:**
```yaml
version: 2
project_name: logvalet

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - id: logvalet
    main: ./cmd/lv/main.go
    binary: logvalet
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w
      - -X github.com/youyo/logvalet/internal/version.Version={{ .Version }}
      - -X github.com/youyo/logvalet/internal/version.Commit={{ .Commit }}
      - -X github.com/youyo/logvalet/internal/version.Date={{ .Date }}

archives:
  - id: default
    formats: [tar.gz]
    name_template: ...

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  use: github

brews:
  - name: logvalet
    repository:
      owner: youyo
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/youyo/logvalet"
    description: "LLM-first Backlog CLI with digest-oriented output"
    license: "MIT"
    install: |
      bin.install "logvalet"
    test: |
      system "#{bin}/logvalet", "--help"

release:
  github:
    owner: youyo
    name: logvalet
```

### 2. `.github/workflows/release.yml`

スペック §22 に基づく GitHub Actions ワークフロー。

**設定内容:**
- トリガー: `push` → `tags: ["v*"]`
- `permissions: contents: write`
- steps:
  1. `actions/checkout@v4` (fetch-depth: 0)
  2. `actions/setup-go@v5` (go-version-file: go.mod)
  3. `tibdex/github-app-token@v2` (GitHub App token 生成)
  4. `goreleaser/goreleaser-action@v6`

**必要な Secrets:**
- `APP_ID` — GitHub App ID
- `APP_PRIVATE_KEY` — GitHub App 秘密鍵

### 3. `README.md`（英語）

**構成:**
- バッジ（GitHub release、Go version）
- 概要（1段落）
- インストール方法
  - Homebrew: `brew install youyo/tap/logvalet`
  - go install: `go install github.com/youyo/logvalet/cmd/lv@latest`
- クイックスタート
  - auth login
  - issue digest
- コマンド一覧（テーブル形式、概要レベル）
- 設定ファイルパス
- ライセンス

### 4. `README.ja.md`（日本語）

README.md の日本語版。内容は同一、言語のみ日本語。

### 5. `skills/SKILL.md`

`docs/specs/logvalet_SKILL.md` をそのままコピー。
スキル定義ファイルはスペックで完成しているため、調整不要。

## 実装ステップ

| # | ステップ | 説明 | リスク |
|---|---------|------|--------|
| 1 | `.github/workflows/` ディレクトリ作成 | - | 低 |
| 2 | `.goreleaser.yaml` 作成 | スペック §21 をベースに ccmix 参考 | 低 |
| 3 | `.github/workflows/release.yml` 作成 | スペック §22 をそのまま適用 | 低 |
| 4 | `README.md` 作成 | 英語でインストール・使い方・コマンド一覧 | 低 |
| 5 | `README.ja.md` 作成 | README.md の日本語版 | 低 |
| 6 | `skills/` ディレクトリ作成 | - | 低 |
| 7 | `skills/SKILL.md` 作成 | docs/specs/logvalet_SKILL.md をコピー | 低 |
| 8 | `go build ./...` で動作確認 | ビルドが引き続き通ることを確認 | 低 |
| 9 | `go test ./...` で全テストパス確認 | - | 低 |
| 10 | git commit | feat(release) コミット | 低 |

## リスク評価

| リスク | 影響 | 対策 |
|--------|------|------|
| GoReleaser v2 の YAML 構文変更 | ビルド失敗 | ccmix の動作実績ある設定を参考に適用 |
| Homebrew tap token 環境変数名の不一致 | リリース失敗 | スペック §21 と ccmix 両方確認し統一 |
| `logvalet` vs `lv` バイナリ名の混乱 | ユーザー混乱 | README に lv alias は brew 不要と明記 |

## 検証基準

- [ ] `go build ./...` が通る
- [ ] `go test ./...` が通る
- [ ] `.goreleaser.yaml` の YAML 文法が正しい（goreleaser check で確認、またはツールで確認）
- [ ] `.github/workflows/release.yml` の YAML 文法が正しい
- [ ] `skills/SKILL.md` が存在する
- [ ] `README.md` と `README.ja.md` が存在する

## 完了基準

- 全ファイルが正しく配置されている
- ビルドが通る
- コミットが作成されている
