package main

import (
	"fmt"
	"os"

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
			if child.Name == word {
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
