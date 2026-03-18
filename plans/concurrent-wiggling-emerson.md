# Shell Completion バグ修正

## Context

Tab 補完が壊れている。2つの根本原因:

1. **`--completion-bash` バックエンド未実装**: 生成される zsh 補完スクリプトが `logvalet --completion-bash ...` を呼ぶが、このフラグのハンドラが存在しない → `unknown flag --completion-bash` エラー
2. **`lv` エイリアス未定義**: `--short` で `compdef _logvalet lv` を出力するが、`lv` コマンド自体が定義されていない → `command not found: lv`

## 変更対象ファイル

- `cmd/logvalet/main.go` — `--completion-bash` インターセプター追加
- `internal/cli/completion.go` — zsh スクリプトに `alias lv=logvalet` 追加、bash/fish テンプレート削除
- `internal/cli/completion_test.go` — テスト更新
- `internal/cli/root.go` — Bash/Fish サブコマンド削除

## 変更一覧

### 1. `cmd/logvalet/main.go` — `--completion-bash` ハンドラ追加

Kong の `Parse()` 前に `os.Args` をチェックし、`--completion-bash` がある場合は Kong モデルを歩いて補完候補を返す。

```go
// run() 内、parser 作成後・Parse 呼び出し前に追加:
if handleCompletionBash(parser, os.Args[1:]) {
    return app.ExitSuccess
}
```

```go
// handleCompletionBash は --completion-bash フラグを処理する。
// 補完スクリプトから呼ばれ、利用可能なサブコマンドを stdout に出力する。
func handleCompletionBash(k *kong.Kong, args []string) bool {
    idx := -1
    for i, a := range args {
        if a == "--completion-bash" {
            idx = i
            break
        }
    }
    if idx < 0 {
        return false
    }

    // --completion-bash の後の引数がユーザーが入力中のコマンド列
    partial := args[idx+1:]

    // Kong モデルのコマンドツリーを歩く
    node := k.Model.Node
    for _, word := range partial {
        if word == "" {
            continue
        }
        found := false
        for _, child := range node.Children {
            if child.Name == word || contains(child.Aliases, word) {
                node = child
                found = true
                break
            }
        }
        if !found {
            break
        }
    }

    // 子コマンド名を出力
    for _, child := range node.Children {
        if !child.Hidden {
            fmt.Println(child.Name)
        }
    }
    return true
}

func contains(ss []string, s string) bool {
    for _, v := range ss {
        if v == s {
            return true
        }
    }
    return false
}
```

### 2. `internal/cli/completion.go` — zsh のみに簡素化 + エイリアス追加

- bash/fish テンプレートを削除（ドキュメントと同期: zsh のみサポート）
- zsh テンプレート: `--completion-bash` を使った動的補完（現状維持）
- `--short` 時: `alias lv=logvalet` を `compdef` の前に追加
- `completionAlias()` 関数を zsh のみに簡素化

**zsh alias 出力 (--short 時)**:
```zsh
# lv alias
alias lv=logvalet
compdef _logvalet lv
```

### 3. `internal/cli/root.go` — Bash/Fish サブコマンド削除

```go
type CompletionCmd struct {
    Zsh  ZshCompletionCmd  `cmd:"" help:"zsh 用の補完スクリプトを出力する"`
}
```

Bash/Fish を CLI から削除（ドキュメントで zsh のみに変更済み）。

### 4. `internal/cli/completion_test.go` — テスト更新

- bash/fish テストを削除
- zsh テスト: `alias lv=logvalet` が含まれることを検証
- `--completion-bash` テンプレートが含まれることを検証

### 5. `cmd/logvalet/main_test.go` — completion ハンドラのテスト追加

- `handleCompletionBash` がトップレベルコマンド名を返すことを検証
- `--completion-bash auth` でサブコマンド（login, logout, whoami）を返すことを検証
- `--completion-bash` がない場合は false を返すことを検証

## 検証方法

```bash
# テスト実行
go test ./...

# ビルド
go build -o logvalet ./cmd/logvalet/

# --completion-bash ハンドラの動作確認
./logvalet --completion-bash ""
# → auth, config, configure, completion, issue, project, ...

./logvalet --completion-bash auth
# → login, logout, whoami

# zsh 補完スクリプト確認
./logvalet completion zsh --short
# → alias lv=logvalet と compdef _logvalet lv が含まれること

# 実際の tab 補完テスト
eval "$(./logvalet completion zsh --short)"
logvalet <TAB>  # → サブコマンド一覧が表示される
lv <TAB>        # → lv コマンドが動作し、サブコマンド一覧が表示される
```
