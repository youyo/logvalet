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

	// goreleaser ldflags で埋め込まれたバージョン情報を注入
	root.GlobalFlags.Version = version.Version
	root.GlobalFlags.Commit = version.Commit
	root.GlobalFlags.Date = version.Date

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
