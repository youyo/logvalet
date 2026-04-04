# Plan: release スキル作成（push → bump → tag → CI 監視）

## Context

今回の会話で実施した「push → version bump → tag → CI 監視」のワークフローをスキル化する。

**既存スキルとのギャップ:**
- `smart-push`: push + tag あり、CI 監視なし
- `auto-merge`: CI 監視あり、PR フロー必須（direct-to-main 不対応）

今回のワークフローは **sole maintainer が main に直接 push してリリースする** パターンで、既存スキルではカバーできない。

## スキル仕様

### 名前: `release`

### ワークフロー（今回の会話から抽出）

1. **Push**: `git push origin main`（未プッシュのコミットがあれば）
2. **Version bump**:
   - 変更内容から bump 種別を判定（feat → minor, fix → patch）
   - `.claude-plugin/plugin.json` の version を更新
   - `git commit` でバージョンバンプコミット
3. **Tag**: `git tag vX.Y.Z && git push origin vX.Y.Z`
4. **CI 監視**: `gh run list` → `gh run view` → 完了まで polling → 結果報告

### トリガー
- `/release`
- `リリースして`
- `push,bump up,ci監視`
- `タグ打ってリリース`
- `バージョン上げてpush`

### バージョンバンプのロジック

git log から最新タグ以降のコミットを分析:
- `feat` コミットあり → **minor** bump (0.6.3 → 0.7.0)
- `fix`/`chore`/`docs` のみ → **patch** bump (0.6.3 → 0.6.4)
- ユーザー指定があればそれを優先

### バージョンファイル検出

以下のファイルからバージョンを検出・更新:
- `.claude-plugin/plugin.json` の `"version"` フィールド
- `package.json` の `"version"` フィールド
- その他（Go は ldflags なのでソースファイル不要、タグのみ）

### CI 監視ロジック

```bash
# 最新の run を取得
gh run list --limit 1 --json databaseId,status,conclusion

# polling（30秒間隔、最大10分）
gh run view <RUN_ID>

# ジョブ詳細
gh run view --job <JOB_ID>
```

成功時: 完了報告
失敗時: エラーログ表示 + 対応提案

## 配置場所

`~/.claude/skills/release/SKILL.md`
（プロジェクト固有ではなく汎用スキル）

## 変更対象ファイル

- `~/.claude/skills/release/SKILL.md` — 新規作成

## 検証

テストプロンプト:
1. 「リリースして」— feat コミットありの場合、minor bump + tag + CI 監視
2. 「v0.7.1 でリリース」— 指定バージョンで bump + tag + CI 監視
3. 「push して CI 見て」— bump なしで push + CI 監視のみ
