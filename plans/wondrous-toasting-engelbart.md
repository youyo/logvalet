# Fix: SKILL.md name フィールドに logvalet: プレフィックスを追加

## Context

プラグイン形式に移行した際（bc1ee7a）、各スキルの SKILL.md `name` フィールドからプレフィックスを除去した（`logvalet-xxx` → `xxx`）。
しかし、ローカルプロジェクトの `skills/` から直接読み込む場合、`name` フィールドがそのまま使われるため、`logvalet:` プレフィックスが付かない。
devflow プラグインは `name: devflow:cycle` のように明示的にプレフィックスを含めており、これが正しいパターン。

## 修正内容

全14スキルの `SKILL.md` の `name` フロントマターを変更:

| ディレクトリ | 現在の name | 修正後の name |
|---|---|---|
| skills/context/ | context | logvalet:context |
| skills/decisions/ | decisions | logvalet:decisions |
| skills/digest-periodic/ | digest-periodic | logvalet:digest-periodic |
| skills/draft/ | draft | logvalet:draft |
| skills/health/ | health | logvalet:health |
| skills/intelligence/ | intelligence | logvalet:intelligence |
| skills/issue-create/ | issue-create | logvalet:issue-create |
| skills/logvalet/ | logvalet | logvalet（変更なし） |
| skills/my-next/ | my-next | logvalet:my-next |
| skills/my-week/ | my-week | logvalet:my-week |
| skills/report/ | report | logvalet:report |
| skills/risk/ | risk | logvalet:risk |
| skills/spec-to-issues/ | spec-to-issues | logvalet:spec-to-issues |
| skills/triage/ | triage | logvalet:triage |

## 対象ファイル

- `skills/*/SKILL.md` — 13ファイルの `name:` 行を変更（`skills/logvalet/` は変更なし）

## 検証

- 変更後、Claude Code を再起動してスキル一覧に `logvalet:` プレフィックスが表示されることを確認
