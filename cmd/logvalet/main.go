package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/cli"
	"github.com/youyo/logvalet/internal/version"
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

// run はメイン処理を実行し、exit code を返す。
// テスタビリティのためにos.Exit から分離。
func run() int {
	var root cli.CLI

	// Kong パーサーを構築
	parser, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Description("Backlog 向け LLM-first CLI — digest-oriented structured output"),
		kong.ConfigureHelp(kong.HelpOptions{Compact: false}),
		kong.Vars{
			"version": version.String(),
		},
		// カスタム Exit: ヘルプ表示 (code=0) はそのまま終了。
		// エラー (code!=0) は JSON エンベロープで処理するため exit しない。
		kong.Exit(func(code int) {
			if code == 0 {
				os.Exit(0)
			}
		}),
	)
	if err != nil {
		return app.HandleError(os.Stdout, err, app.ExitGenericError)
	}

	// --completion-bash フラグを Parse 前にインターセプト
	if handleCompletionBash(parser, os.Args[1:]) {
		return app.ExitSuccess
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		// パースエラー（引数不正、未知のサブコマンド等）
		// usage を stderr に出力してから JSON エンベロープを stdout に出力
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		return app.HandleError(os.Stdout, err, app.ExitArgumentError)
	}

	if err := ctx.Run(&root.GlobalFlags); err != nil {
		// コマンド実行エラー → JSON エラーエンベロープを stdout に出力
		return app.HandleError(os.Stdout, err, app.ExitGenericError)
	}

	return app.ExitSuccess
}

// collectCompletions は --completion-bash 以降の部分入力を解析し、
// 補完候補のスライスを返す。--completion-bash がない場合は nil, false を返す。
func collectCompletions(k *kong.Kong, args []string) ([]string, bool) {
	idx := -1
	for i, a := range args {
		if a == "--completion-bash" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, false
	}

	// --completion-bash の後の引数がユーザーが入力中のコマンド列
	partial := args[idx+1:]

	// Kong モデルのコマンドツリーを歩きながら使用済みフラグを収集する
	node := k.Model.Node
	usedFlags := make(map[string]bool)
	prefix := ""
	endsWithFlag := false

	// 最後の word が "--" で始まる場合は入力中のプレフィクスとして扱い、
	// それ以前の word のみをループ処理対象とする
	loopPartial := partial
	if len(partial) > 0 {
		last := partial[len(partial)-1]
		if last != "" && strings.HasPrefix(last, "--") {
			prefix = last
			loopPartial = partial[:len(partial)-1]
		}
	}

	for _, word := range loopPartial {
		if word == "" {
			continue
		}
		if strings.HasPrefix(word, "--") {
			// フラグとして記録（値が確定しているとみなす）
			usedFlags[word] = true
			endsWithFlag = true
		} else {
			endsWithFlag = false
			// サブコマンドにマッチするか試みる
			found := false
			for _, child := range node.Children {
				if child.Name == word {
					node = child
					found = true
					break
				}
			}
			if !found {
				// マッチしなければプレフィクスとして保存
				prefix = word
			}
		}
	}

	var completions []string

	// フラグ候補を収集（AllFlags は親フラグも含む）
	flagGroups := node.AllFlags(true)
	for _, group := range flagGroups {
		for _, flag := range group {
			if flag.Hidden {
				continue
			}
			candidate := "--" + flag.Name
			if usedFlags[candidate] {
				continue
			}
			if prefix != "" && !strings.HasPrefix(candidate, prefix) {
				continue
			}
			completions = append(completions, candidate)
		}
	}

	// サブコマンド候補を収集（フラグのプレフィクス入力中でなければ）
	if !endsWithFlag && !strings.HasPrefix(prefix, "--") {
		for _, child := range node.Children {
			if child.Hidden {
				continue
			}
			if prefix != "" && !strings.HasPrefix(child.Name, prefix) {
				continue
			}
			completions = append(completions, child.Name)
		}
	}

	return completions, true
}

// handleCompletionBash は --completion-bash フラグを処理する。
// 補完スクリプトから呼ばれ、利用可能なサブコマンドとフラグを stdout に出力する。
func handleCompletionBash(k *kong.Kong, args []string) bool {
	completions, ok := collectCompletions(k, args)
	if !ok {
		return false
	}
	for _, c := range completions {
		fmt.Println(c)
	}
	return true
}
