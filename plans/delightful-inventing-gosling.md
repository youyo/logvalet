# Plan: logvalet roadmap-v2 作成 + スペック更新

## Context

logvalet の実装はロードマップ v1 の全12マイルストーンをほぼ完了しているが、
ロードマップ自体は「未着手」のまま。スペックと実装の間に乖離がある。

このプランでは:
1. スペックを現状の実装に合わせて更新する
2. 残差分を新マイルストーンとして roadmap-v2 にまとめる
3. 完了済みマイルストーンの進捗を反映する
4. 参照系コマンドで実際の Backlog API と動作確認する

## 差分分析結果

### 重大な差分

| # | 項目 | スペック | 現状 | 影響 |
|---|------|---------|------|------|
| 1 | GlobalFlags 欠落 | 10フラグ定義 | 4フラグのみ Kong に登録 | `--api-key`, `--access-token`, `--base-url`, `--space`, `--config`, `--no-color` が CLI フラグとして使えない |
| 2 | buildRunContext の CredentialFlags | `--api-key`/`--access-token` を渡す | `CredentialFlags{}` 空で渡している | CLI フラグ経由の認証情報上書きが機能しない |
| 3 | JSON エラーエンベロープ (§9) | `{"schema_version":"1","error":{...}}` | `fmt.Fprintf(stderr, "エラー: %v")` | エラー時の出力が機械可読でない |
| 4 | OAuth フロー (§5) | OAuth + API key 両対応 | API key のみ（意図的決定） | スペック更新が必要 |

### 軽微な差分

| # | 項目 | 現状 |
|---|------|------|
| 5 | エントリポイント | `cmd/logvalet/` (スペックは `cmd/lv/`)。機能上問題なし |
| 6 | `lv version` コマンド | 未実装（スペックでも MVP 外 OK と明記） |
| 7 | DigestFlags/ListFlags/WriteFlags | スペック§17.2 と若干異なる（Comments フラグなし等） |

### 実装は完了しているがロードマップで反映されていないもの

- M01-M12 全て実装済み → ロードマップのチェックボックスが全て未チェック

## 実行内容

### Part 1: スペック更新 (docs/specs/logvalet_full_design_spec_with_architecture.md)

#### 変更箇所

1. **§5 Authentication**: OAuth localhost callback を「将来対応予定」に格下げ
   - auth login: "login using OAuth" → "login using API key"
   - `--api-key` フラグの説明を明記
   - tokens.json: oauth エントリを「オプション（将来対応）」に
2. **§16 Directory**: `cmd/lv/` → `cmd/logvalet/`
3. **§21 .goreleaser.yaml**: `./cmd/lv/main.go` → `./cmd/logvalet/main.go`

### Part 2: roadmap-v2 作成 (plans/logvalet-roadmap-v2.md)

#### 完了済みセクション
- M01-M12: 全て完了マーク付きで記載

#### 新マイルストーン

##### M13: GlobalFlags 完全実装
- `internal/cli/global_flags.go`: 欠落6フラグを Kong struct に追加
  - `--api-key` / `LOGVALET_API_KEY`
  - `--access-token` / `LOGVALET_ACCESS_TOKEN`
  - `--base-url` / `LOGVALET_BASE_URL`
  - `--space` / `-s` / `LOGVALET_SPACE`
  - `--config` / `-c` / `LOGVALET_CONFIG`
  - `--no-color` / `LOGVALET_NO_COLOR`
- `internal/cli/runner.go`: `buildRunContext()` 修正
  - `CredentialFlags{}` → GlobalFlags から api-key/access-token を渡す
  - `config.OverrideFlags` に Space/BaseURL/NoColor/ConfigPath を渡す
- テスト: GlobalFlags → OverrideFlags → RunContext の結合テスト

##### M14: JSON エラーエンベロープ (§9)
- `internal/domain/` に ErrorEnvelope/WarningEnvelope 型定義
- `cmd/logvalet/main.go` のエラーハンドリング修正
  - エラー時に JSON エンベロープを stdout に出力
  - exit code との整合性
- テスト: 各 exit code に対応するエラーの JSON 出力確認

##### M15 (optional): `lv version` コマンド
- `internal/cli/root.go`: Version コマンド追加
- `--version` グローバルフラグ対応

### Part 3: 既存ロードマップ更新
- `plans/logvalet-roadmap.md`: ステータスを「完了 → roadmap-v2 に移行」に更新

### Part 4: 実 API 動作確認（参照系のみ）
- `LOGVALET_API_KEY` 環境変数を使用
- テスト対象:
  - `logvalet auth whoami --profile <profile>`
  - `logvalet space info --profile <profile>`
  - `logvalet space disk-usage --profile <profile>`
  - `logvalet project list --profile <profile>`
  - `logvalet user list --profile <profile>`
- **更新系は絶対に実行しない**

## 対象ファイル
- `plans/logvalet-roadmap-v2.md` — 新規作成
- `docs/specs/logvalet_full_design_spec_with_architecture.md` — スペック更新
- `plans/logvalet-roadmap.md` — ステータス更新

## 検証方法
1. スペック diff 確認
2. roadmap-v2 のマイルストーン整合性確認
3. `go build` でビルド成功確認
4. 参照系コマンドの実 API 動作確認（Part 4）
