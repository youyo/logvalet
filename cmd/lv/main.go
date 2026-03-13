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
	var root cli.CLI

	// goreleaser ldflags で埋め込まれたバージョン情報を注入
	root.GlobalFlags.Version = version.Version
	root.GlobalFlags.Commit = version.Commit
	root.GlobalFlags.Date = version.Date

	ctx := kong.Parse(&root,
		kong.Name("logvalet"),
		kong.Description("Backlog 向け LLM-first CLI — digest-oriented structured output"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: false}),
		kong.Vars{
			"version": version.String(),
		},
	)

	if err := ctx.Run(&root.GlobalFlags); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(app.ExitGenericError)
	}
}
