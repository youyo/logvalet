package cli

import (
	"fmt"
	"os"
)

// completionScript は zsh 用の補完スクリプトを生成する。
func completionScript(shell, name string, short bool) string {
	switch shell {
	case "zsh":
		alias := ""
		if short {
			alias = fmt.Sprintf("\n# lv alias\nalias lv=%s\ncompdef _%s lv\n", name, name)
		}
		return fmt.Sprintf(`# %s completion for zsh
# To enable: eval "$(%s completion zsh)"
_%s() {
  local -a completions
  completions=($(${words[1]} --completion-bash ${words[@]:1}))
  compadd -- $completions
}
compdef _%s %s
%s`, name, name, name, name, name, alias)

	default:
		return fmt.Sprintf("# completion not supported for shell: %s\n", shell)
	}
}

// GenerateCompletion は指定されたシェル・コマンド名・エイリアスの補完スクリプト文字列を返す。
// テスト・実装の両方から利用できる公開ヘルパー。
func GenerateCompletion(shell, name string, short bool) string {
	return completionScript(shell, name, short)
}

// CompletionCmd は completion コマンドのルート。
type CompletionCmd struct {
	Zsh ZshCompletionCmd `cmd:"" help:"zsh 用の補完スクリプトを出力する"`
}

// ZshCompletionCmd は zsh 用の補完スクリプトを出力する。
type ZshCompletionCmd struct {
	Short bool `help:"lv エイリアスの補完も出力する"`
}

// Run は Kong から呼ばれる実行メソッド。
func (c *ZshCompletionCmd) Run(g *GlobalFlags) error {
	_, err := fmt.Fprint(os.Stdout, completionScript("zsh", "logvalet", c.Short))
	return err
}
