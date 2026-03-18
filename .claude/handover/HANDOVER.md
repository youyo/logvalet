# HANDOVER.md

> 生成日時: 2026-03-18 14:30
> プロジェクト: logvalet
> ブランチ: main

## 今回やったこと

- M13: GlobalFlags 完全実装
  - `internal/cli/global_flags.go`: 欠落していた 6 フラグを追加
    - `--api-key` / `LOGVALET_API_KEY`
    - `--access-token` / `LOGVALET_ACCESS_TOKEN`
    - `--base-url` / `LOGVALET_BASE_URL`
    - `--space` / `-s` / `LOGVALET_SPACE`
    - `--config` / `-c` / `LOGVALET_CONFIG`
    - `--no-color` / `LOGVALET_NO_COLOR`
  - `GlobalFlags.Validate()`: --api-key/--access-token の排他バリデーション（spec §17 Validation rules）
  - `internal/cli/runner.go`: `buildRunContext()` を修正
    - `config.ResolveConfigPath()` で --config / LOGVALET_CONFIG に対応
    - `config.OverrideFlags` に Pretty/Space/BaseURL/Verbose/NoColor/ConfigPath を全て渡す
    - `credentials.CredentialFlags` に APIKey/AccessToken を渡す
    - `boolPtr()` ヘルパーで bool → *bool 変換
  - `internal/cli/auth.go`: AuthLoginCmd の独自 --api-key フラグを削除し GlobalFlags に統合
  - `internal/cli/issue.go`: IssueDigestCmd.Comments の `-c` 短縮フラグを削除（GlobalFlags.Config の `-c` と衝突回避）
  - テスト: 新フラグの CLI パース / 環境変数オーバーライド / 排他バリデーション
- `plans/logvalet-m13-global-flags.md`: M13 詳細計画ファイルを作成
- `plans/logvalet-roadmap-v2.md`: M13 完了マーク + Current Focus を M14 に更新
- コミット: `326cc71`

## 決定事項

- **AuthLoginCmd の --api-key は GlobalFlags に統合**: 元々 AuthLoginCmd が独自に `--api-key` フラグを持っていたが、GlobalFlags に同名フラグを追加したため Kong で重複エラーが発生。auth login は GlobalFlags.APIKey を使うように変更
- **IssueDigestCmd.Comments の `-c` 短縮フラグを削除**: GlobalFlags.Config が `-c` を使うため衝突。サブコマンドのフラグは長い名前 `--comments` のみとした
- **bool → *bool 変換は boolPtr() ヘルパーで統一**: Kong の bool フラグは未指定=false、指定=true。config.OverrideFlags の *bool（nil=未指定）との変換が必要。Kong が env タグを先に処理するため、boolPtr で常に渡しても問題なし

## 捨てた選択肢と理由

- **buildRunContextWith() DI リファクタリング**: テスタビリティ向上のため検討したが、GlobalFlags のパーステスト + config/credentials の既存単体テストで十分カバーされるため取りやめ
- **bool フラグの negatable タグ**: Kong の `negatable` タグで `--no-pretty` のような否定形を実現できるが、spec の定義とずれるため不採用

## ハマりどころ

- Kong のグローバルフラグ（CLI struct の埋め込み）とサブコマンドのフラグが同名だと `duplicate flag` エラーになる
- 環境変数が前のテストから漏れて Validate() が意図しないエラーを出す問題 → `t.Setenv` で各テストの先頭で認証系 env をクリア

## 学び

- Kong は struct に `Validate() error` メソッドがあると Parse 後に自動で呼び出す
- `type:"path"` タグは Kong の補完ヒントのみで、ファイル存在チェックは行わない
- グローバルフラグの短縮名はサブコマンド全体で一意である必要がある

## 次にやること（優先度順）

- [ ] M14: JSON エラーエンベロープ (§9) — ErrorEnvelope/WarningEnvelope 型定義 + main.go 修正（中）
- [ ] M15: `logvalet config init` + `logvalet configure` エイリアス — 対話型セットアップ（中）
- [ ] M16: `logvalet version` コマンド実装（低・optional）
- [ ] `auth whoami` の user フィールドが null の原因調査（低）
- [ ] GitHub Secrets に `APP_ID` と `APP_PRIVATE_KEY` を設定（Homebrew tap 更新用）（高）
- [ ] `git tag v0.1.0 && git push origin v0.1.0` でリリースタグ → GoReleaser 自動実行（高）

## 関連ファイル

- `internal/cli/global_flags.go` — 6 フラグ追加 + Validate()
- `internal/cli/global_flags_test.go` — 新フラグテスト
- `internal/cli/runner.go` — buildRunContext() 修正
- `internal/cli/auth.go` — AuthLoginCmd の --api-key 削除
- `internal/cli/issue.go` — IssueDigestCmd.Comments の -c 削除
- `plans/logvalet-m13-global-flags.md` — M13 詳細計画
- `plans/logvalet-roadmap-v2.md` — M13 完了マーク
