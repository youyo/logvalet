package cli

import (
	"fmt"
	"os"
)

// completionScript は指定されたシェル・コマンド名・エイリアスの補完スクリプトを生成する。
func completionScript(shell, name string, short bool) string {
	alias := ""
	if short {
		alias = completionAlias(shell, "lv", name)
	}

	switch shell {
	case "zsh":
		return fmt.Sprintf(`# %s completion for zsh
# To enable: eval "$(%s completion zsh)"
_%s() {
  local -a completions
  completions=($(${words[1]} --completion-bash "${words[@]:1}"))
  compadd -- $completions
}
compdef _%s %s
%s`, name, name, name, name, name, alias)

	case "bash":
		return fmt.Sprintf(`# %s completion for bash
# To enable: eval "$(%s completion bash)"
_%s() {
  local completions
  completions=$(${COMP_WORDS[0]} --completion-bash "${COMP_WORDS[@]:1}")
  COMPREPLY=( $(compgen -W "$completions" -- "${COMP_WORDS[COMP_CWORD]}") )
}
complete -F _%s %s
%s`, name, name, name, name, name, alias)

	case "fish":
		return fmt.Sprintf(`# %s completion for fish
# To enable: %s completion fish | source
complete -c %s -f -a '(%s --completion-bash (commandline -opc)[2..])'
%s`, name, name, name, name, alias)

	default:
		return fmt.Sprintf("# completion not supported for shell: %s\n", shell)
	}
}

// completionAlias はエイリアス用の補完スクリプトを返す。
func completionAlias(shell, alias, original string) string {
	switch shell {
	case "zsh":
		return fmt.Sprintf("# lv alias\ncompdef _%s %s\n", original, alias)
	case "bash":
		return fmt.Sprintf("# lv alias\ncomplete -F _%s %s\n", original, alias)
	case "fish":
		return fmt.Sprintf("# lv alias\ncomplete -c %s -f -a '(%s --completion-bash (commandline -opc)[2..])'\n", alias, original)
	default:
		return ""
	}
}

// GenerateCompletion は指定されたシェル・コマンド名・エイリアスの補完スクリプト文字列を返す。
// テスト・実装の両方から利用できる公開ヘルパー。
func GenerateCompletion(shell, name string, short bool) string {
	return completionScript(shell, name, short)
}

// CompletionCmd は completion コマンドのルート。
type CompletionCmd struct {
	Bash BashCompletionCmd `cmd:"" help:"bash 用の補完スクリプトを出力する"`
	Zsh  ZshCompletionCmd  `cmd:"" help:"zsh 用の補完スクリプトを出力する"`
	Fish FishCompletionCmd `cmd:"" help:"fish 用の補完スクリプトを出力する"`
}

// BashCompletionCmd は bash 用の補完スクリプトを出力する。
type BashCompletionCmd struct {
	Short bool `help:"lv エイリアスの補完も出力する"`
}

// Run は Kong から呼ばれる実行メソッド。os.Stdout に出力する。
func (c *BashCompletionCmd) Run(g *GlobalFlags) error {
	_, err := fmt.Fprint(os.Stdout, completionScript("bash", "logvalet", c.Short))
	return err
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

// FishCompletionCmd は fish 用の補完スクリプトを出力する。
type FishCompletionCmd struct {
	Short bool `help:"lv エイリアスの補完も出力する"`
}

// Run は Kong から呼ばれる実行メソッド。
func (c *FishCompletionCmd) Run(g *GlobalFlags) error {
	_, err := fmt.Fprint(os.Stdout, completionScript("fish", "logvalet", c.Short))
	return err
}
